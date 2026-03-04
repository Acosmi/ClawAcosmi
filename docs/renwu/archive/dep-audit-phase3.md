# Phase 3 — 依赖隐式行为审计

> 最后更新：2026-02-12

## 一、npm 依赖清单

Phase 3 涉及 17 个 TS 源文件，所有 `import` 按来源分为三类：

### 1.1 Node.js 标准库

| 模块 | 使用文件 | 调用方式 |
|------|---------|---------|
| `node:http` | server-http, auth, hooks, http-common, http-utils, tools-invoke-http | `createServer`, `IncomingMessage`, `ServerResponse` |
| `node:https` | server-http | `createServer` (TLS) |
| `node:tls` | server-http | `TlsOptions` 类型 |
| `node:crypto` | client, http-utils, hooks, auth, server-node-events | `randomUUID`, `timingSafeEqual` |
| `node:net` | net | `net.isIP`, `net.createServer` (bind 测试) |
| `node:os` | net | `os.networkInterfaces` |
| `node:fs` | session-utils, boot | 同步/异步文件读写 |
| `node:fs/promises` | boot | `fs.readFile` |
| `node:path` | session-utils, hooks-mapping, boot | `path.join`, `path.resolve`, `path.extname` |
| `node:url` | hooks-mapping | `pathToFileURL` |

### 1.2 npm 第三方包

| 包名 | 使用文件 | 调用方式 |
|------|---------|---------|
| **`ws`** | server-http (type-only), client | `WebSocket` 构造器, `WebSocketServer` 类型, `ClientOptions`, `CertMeta` |
| **`chokidar`** | config-reload | `chokidar.watch` 文件系统监听 |

### 1.3 内部模块引用

Phase 3 文件引用了约 40+ 个内部模块，关键依赖方向：

```
server-http → auth, hooks, hooks-mapping, http-common, http-utils, net
           → config/config, agents/identity-avatar, canvas-host/*
           → slack/http, openai-http, openresponses-http, tools-invoke-http

client → infra/device-*, infra/tls/*, infra/ws, protocol/*
       → utils/message-channel, gateway/device-auth

auth → config/config, infra/tailscale, net

net → infra/tailnet

session-utils → config/sessions, config/paths, agents/*, routing/*

server-channels → channels/plugins/*, infra/outbound/*

config-reload → config/config, channels/plugins, plugins/runtime
```

---

## 二、逐包隐式行为扫描

### 2.1 `ws` (WebSocket)

| 维度 | TS 调用方式 | 实际内部行为 | Go 替代方案 | 差异 |
|------|-----------|------------|-----------|------|
| **连接管理** | `new WebSocket(url, opts)` | 自动 HTTP Upgrade 握手、处理 301/302 重定向 | `gorilla/websocket.Dialer.Dial` | gorilla 不处理重定向，需手动 |
| **帧分片** | `ws.send(data)` | 大消息自动分片、按需合并 | `gorilla/websocket.WriteMessage` | gorilla 默认发单帧，需配置 |
| **ping/pong 心跳** | 自动响应 ping | 库内置 auto-pong，可配超时 | 无自动心跳 | ⚠ **需手动实现** |
| **压缩协商** | `perMessageDeflate: true` | 自动 permessage-deflate 扩展 | `gorilla/websocket.EnableCompression` | 需手动配置，默认关闭 |
| **maxPayload** | `maxPayload: 25MB` | 超限自动关闭连接 | `SetReadLimit(25MB)` | API 不同但行为可对齐 |
| **背压控制** | `bufferedAmount` 属性 | 内部跟踪写缓冲字节 | 无直接等价属性 | ⚠ **需自行跟踪** |
| **TLS 指纹** | `checkServerIdentity` 回调 | 握手阶段验证证书 | `tls.Config.VerifyPeerCertificate` | 可对齐 |
| **rawData → string** | `rawDataToString(data)` | 处理 Buffer/ArrayBuffer/string | `string(msg)` | Go 天然 `[]byte` |
| **close 语义** | `ws.close(code, reason)` | 发送 close 帧后等待回复 | `WriteControl(CloseMessage...)` | 行为一致 |

**补偿措施**：

1. 实现 `PingPongManager` goroutine：每 30s 发 ping，60s 超时关闭
2. 封装 `BufferedWriter` 跟踪 `bufferedAmount`
3. 连接时显式设置 `ReadLimit(25 * 1024 * 1024)`
4. 编写行为探针测试：验证大消息、重连、超时

### 2.2 `chokidar` (文件监听)

| 维度 | TS 调用方式 | 实际内部行为 | Go 替代方案 | 差异 |
|------|-----------|------------|-----------|------|
| **监听机制** | `chokidar.watch(path, opts)` | 优先 `fs.watch`，fallback 轮询 | `fsnotify/fsnotify` | 跨平台行为类似 |
| **ignoreInitial** | `ignoreInitial: true` | 不触发已存在文件事件 | fsnotify 默认即此行为 | 无差异 |
| **awaitWriteFinish** | `stabilityThreshold: 200` | 等文件写入稳定后才触发 | 无内置 | ⚠ **需手动防抖** |
| **事件类型** | `add`, `change`, `unlink` | 文件新增、修改、删除 | `Create`, `Write`, `Remove` | 语义可映射 |
| **递归监听** | 默认递归 | 自动监听子目录 | macOS 上 fsnotify 支持递归 | 基本一致 |

**补偿措施**：

1. 使用 `fsnotify` + 自定义防抖 goroutine (200ms stabilityThreshold)
2. 编写测试验证：文件添加/修改/删除事件、防抖合并

### 2.3 `node:http` / `node:https`

| 维度 | TS 行为 | Go 差异 | 补偿 |
|------|--------|---------|------|
| **Keep-Alive** | 默认开启连接池 | Go `net/http` 也默认开启 | 无需 |
| **超时** | 无默认超时 | Go Server 建议设置超时 | 显式设 `ReadTimeout`, `WriteTimeout` |
| **Transfer-Encoding** | 自动 chunked | Go 自动 chunked | 无需 |
| **请求体读取** | Stream 逐 chunk | `io.ReadAll` 或 `io.LimitReader` | 需限制 body 大小 |
| **SSE (text/event-stream)** | `res.write()` 可流式 | `Flusher` 接口 | 需 `http.Flusher` 断言 |
|  **Upgrade 处理** | `httpServer.on('upgrade')` | middleware/hijack | 使用 Fiber 或 gorilla 升级 |

**补偿措施**：

1. Fiber HTTP 框架封装 `Server.ReadTimeout = 30s, WriteTimeout = 120s`
2. SSE 使用 `c.Response().Flush()` 或直接 hijack

### 2.4 `node:crypto`

| 函数 | TS 行为 | Go 替代 | 差异 |
|------|--------|---------|------|
| `randomUUID()` | V4 UUID | `crypto/rand` + `google/uuid` | 已有替代，无差异 |
| `timingSafeEqual(a, b)` | 恒定时间比较 | `crypto/subtle.ConstantTimeCompare` | ⚠ **TS 版先检查长度，Go 版不同长度返回 0** |

**补偿措施**：

```go
func safeEqual(a, b string) bool {
    if len(a) != len(b) {
        return false
    }
    return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
```

### 2.5 `node:net`

| 函数 | 用途 | Go 替代 |
|------|------|---------|
| `net.isIP(str)` | 判断 IPv4/IPv6 | `net.ParseIP(str) != nil` |
| `net.createServer()` + `listen(0)` | 测试绑定 | `net.Listen("tcp", host+":0")` |

**无高危差异。**

### 2.6 `node:os`

| 函数 | 用途 | Go 替代 |
|------|------|---------|
| `os.networkInterfaces()` | 获取本机网卡 | `net.Interfaces()` |

**无高危差异。**

---

## 三、高危差异清单

| # | 风险等级 | 所在文件 | 差异描述 | 影响 |
|---|---------|---------|---------|------|
| 1 | 🔴 高危 | client.ts | `ws` 自动 ping/pong 心跳 Go 侧需手动实现 | 连接静默断开、无法检测 |
| 2 | 🔴 高危 | server-broadcast.ts | `ws.bufferedAmount` Go 无直接等价 | 慢消费者检测失败 |
| 3 | 🟡 中危 | auth.ts | `timingSafeEqual` 长度不等时行为差异 | 时序攻击风险（已有补偿） |
| 4 | 🟡 中危 | config-reload.ts | `chokidar.awaitWriteFinish` Go 无内置 | 频繁重载或漏事件 |
| 5 | 🟢 低危 | client.ts | `setTimeout().unref()` Go 无等价 | goroutine 生命周期不同，但 Go 天然支持后台运行 |
| 6 | 🟢 低危 | hooks-mapping.ts | `import(url)` 动态模块加载 | Go 不支持动态加载，需用 plugin 或配置化策略 |

---

## 四、验收标准

### 4.1 行为探针测试

| # | 测试名称 | 验证点 |
|---|---------|--------|
| 1 | `TestWsPingPong` | 手动心跳 30s 间隔 + 60s 超时关闭 |
| 2 | `TestWsBufferedAmount` | 自定义写缓冲跟踪器正确计数 |
| 3 | `TestWsMaxPayload` | 超 25MB 消息被拒绝 |
| 4 | `TestWsReconnect` | 指数退避重连 1s → 30s 上限 |
| 5 | `TestTimingSafeEqual` | 长度不等返回 false、恒定时间 |
| 6 | `TestFileWatcherDebounce` | 200ms 防抖合并多次写入为单次事件 |
| 7 | `TestSSEFlush` | SSE 响应逐行 flush |
| 8 | `TestBindHost` | ephemeral port 绑定测试 |

### 4.2 集成测试

| # | 测试名称 | 验证点 |
|---|---------|--------|
| 1 | `TestHookEndToEnd` | POST /hooks/wake + /hooks/agent 完整链路 |
| 2 | `TestAuthTokenFlow` | token/password/tailscale 三种模式 |
| 3 | `TestConfigReloadHybrid` | 配置变更触发热重载 vs 冷重启 |
