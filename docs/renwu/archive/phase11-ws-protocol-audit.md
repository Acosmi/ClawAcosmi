# 模块 E: WS 协议 — 重构健康度审计报告

> 审计日期: 2026-02-17
> 审计方法: `/refactor` 六步循环法 + 隐藏依赖审计

---

## 一、文件映射

| TS 文件 | 行数 | Go 文件 | 行数 | 覆盖度 |
|---------|------|---------|------|--------|
| `ws-connection.ts` | 267 | `ws_server.go` | 325 | ⚠️ 部分 |
| `message-handler.ts` | 1009 | `ws_server.go` (内联) | — | ❌ 大量缺失 |
| `ws-types.ts` | 11 | `protocol.go` | 267 | ✅ 完整 |
| `protocol/schema/frames.ts` | 165 | `protocol.go` | — | ✅ 完整 |
| `ws-log.ts` | 450 | — | — | ❌ 未移植 |
| `ws-logging.ts` | 14 | — | — | ❌ 未移植 |
| `server-constants.ts` | 35 | `broadcast.go` (常量) | — | ⚠️ 值不一致 |
| — | — | `ws.go` | 292 | ✅ Go 独有客户端 |
| — | — | `ws_test.go` | 151 | ✅ 测试 |

---

## 二、隐藏依赖审计 (7 项)

| # | 类别 | 结果 | 说明 |
|---|------|------|------|
| 1 | npm 包黑盒行为 | ⚠️ | TS 使用 `ws` npm 包: 自动管理 TCP backpressure、per-message deflate 协商。Go `gorilla/websocket` 无 per-message deflate |
| 2 | 全局状态/单例 | ⚠️ | TS: `wsInflightCompact` / `wsInflightOptimized` 全局 Map (ws-log.ts)。Go: 无等价日志状态 |
| 3 | 事件总线/回调链 | ⚠️ | TS: `socket.on("message")` 异步事件驱动 + `socket.once("close/error")`。Go: 同步 `for` 循环读取，行为等价 |
| 4 | 环境变量依赖 | ⚠️ | TS: `OPENACOSMI_VERSION` / `GIT_COMMIT` / `OPENACOSMI_TEST_HANDSHAKE_TIMEOUT_MS`。Go: 未读取这些环境变量 |
| 5 | 文件系统约定 | ✅ | WS 协议层无文件系统依赖 |
| 6 | 协议/消息格式约定 | ❌ | **关键偏差** — 详见第三节 |
| 7 | 错误处理约定 | ⚠️ | TS 使用 `ErrorCodes.INVALID_REQUEST` + `truncateCloseReason()`。Go 使用 `ErrCodeBadRequest`，close reason 不截断 |

---

## 三、关键行为偏差

### 3.1 连接握手协议 (P0 ⚠️)

**TS 服务端流程:**

1. 服务端发送 `connect.challenge` 事件 (含 nonce)
2. 客户端发送 `{type:"req", method:"connect", params: ConnectParams}`
3. 服务端回复 `{type:"res", id:X, ok:true, payload: HelloOk}`

**Go 服务端流程:**

1. ~~无 connect.challenge~~
2. 客户端发送 `{type:"connect", ...params}`
3. 服务端直接回复 `HelloOk`

**前端 (gateway.ts) 当前行为:**

- 已改为发送 `{type:"connect"}` 格式 (L275)
- 等待 `connect.challenge` 事件获取 nonce，750ms 超时后也会发送

**结论:** 前端已与 Go 服务端对齐，但与 TS 服务端不兼容。此为**有意变更**。

### 3.2 MaxPayloadBytes 不一致 (P0 ❌)

| 参数 | TS 值 | Go 值 | 差异 |
|------|-------|-------|------|
| `MaxPayloadBytes` | 512 KB | **25 MB** | **50 倍** |
| `MaxBufferedBytes` | 1.5 MB | 1.5 MB | ✅ 一致 |
| `TickIntervalMs` | 30,000 | 30,000 | ✅ 一致 |

Go 端 `MaxPayloadBytes = 25MB` 过大，可能导致:

- 内存滥用风险（恶意客户端发送大帧）
- hello-ok 中 `policy.maxPayload` 向客户端声明了过大限额

### 3.3 缺失功能清单

| 功能 | TS | Go | 优先级 |
|------|----|----|--------|
| connect.challenge nonce | ✅ | ❌ | P1 |
| 设备认证 (device auth) | ✅ (完整) | ❌ | P1 |
| Origin 检查 | ✅ | ❌ | P1 |
| 协议版本协商 | ✅ (min/max) | ❌ | P2 |
| Presence 跟踪 | ✅ | ❌ (仅广播) | P2 |
| 节点注册/注销 | ✅ | ❌ | P2 |
| Handshake 超时 | ✅ (10s timer) | ⚠️ (30s ReadDeadline) | P2 |
| WS 日志子系统 | ✅ (3 模式) | ❌ | P3 |
| 角色验证 | ✅ (operator/node) | ⚠️ (仅默认) | P2 |
| Close 原因记录 | ✅ (丰富上下文) | ❌ (仅 slog) | P3 |

---

## 四、心跳与重连对比

### 4.1 服务端心跳 (Go ws_server.go)

```
Ping 间隔: 30s (pingTicker)
ReadDeadline: 90s (3 倍 Ping 间隔)
PongHandler: 收到 Pong 后重置 ReadDeadline
```

与 TS 对比: TS 服务端**没有**主动发 Ping — 依靠客户端 Ping。
Go 服务端主动发 Ping ✅ 更健壮。

### 4.2 客户端心跳 (Go ws.go)

```
PingIntervalMs: 可配置 (测试用 5000)
PongTimeoutMs: 可配置 (测试用 10000)
```

### 4.3 前端重连 (UI gateway.ts)

```
初始延迟: 800ms
退避乘数: 1.7x
最大延迟: 15s
queueConnect 等待: 750ms (等待 connect.challenge)
```

### 4.4 Go 客户端重连 (ws.go)

```
reconnectDelay = min(2^(attempts-1) * 1s, MaxReconnectMs)
MaxReconnectMs: 可配置 (默认 30s)
```

---

## 五、WS 频繁断连根因分析

**现象:** `close 1006 (abnormal closure): unexpected EOF` 约 20s 断连

### 可能原因 (按概率排序)

1. **ReadDeadline 未重置 (P0)** — Go 服务端在握手阶段设置 `conn.SetReadDeadline(90s)`，但 Pong 是在 `wsConnectionLoop` 中设置。如果 `connect` 帧处理较慢（等待 auth 查询），90s 内未完成会超时关闭

2. **前端 queueConnect 750ms 延迟** — 前端 `open` 后等 750ms 再发 `sendConnect()`。如果 Go 端在这期间没收到任何帧，不会立即断开（90s 超时）。**非直接原因**

3. **Nginx/反向代理超时** — 如有 Nginx 反代，默认 `proxy_read_timeout 60s`。如果客户端连接后长时间无数据交互，代理会截断

4. **PingTicker 30s 与实际断连 ~20s 不匹配** — 如果断连发生在 20s，不是 Ping 超时导致。可能是网络层问题或代理层超时

### 建议排查步骤

1. 检查是否有反向代理，以及其 `read_timeout` 设置
2. 在 Go 服务端 `wsConnectionLoop` 开头添加带时间戳的 debug 日志
3. 确认 `connect` 帧到达时间与 `hello-ok` 发送时间间隔

---

## 六、错误码映射

| 用途 | TS 码 | Go 码 | 状态 |
|------|-------|-------|------|
| 客户端 connect 失败 | — | 4008 | ✅ 前端使用 |
| 协议违约 | 1008 | — | ❌ Go 未使用 |
| 协议不匹配 | 1002 | — | ❌ Go 未使用 |
| 异常关闭 | 1006 | 1006 | ✅ 一致 |
| 服务器重启 | 1012 | — | ❌ 未检查 |

---

## 七、修复建议

| # | 问题 | 优先级 | 建议 |
|---|------|--------|------|
| 1 | MaxPayloadBytes 50x 偏差 | P0 | Go 改为 `512 * 1024` |
| 2 | 缺少 connect.challenge | P1 | 实现 nonce 交换 (安全需要时) |
| 3 | 缺少设备认证 | P1 | 记入 deferred-items |
| 4 | 缺少 Origin 检查 | P1 | 记入 deferred-items |
| 5 | 协议版本协商 | P2 | 记入 deferred-items |
| 6 | Handshake 超时不匹配 | P2 | Go 添加 10s 握手计时器 |
| 7 | WS 日志系统 | P3 | 记入 deferred-items |

---

## 八、Go test 验证

```
go build  ./...   ✅ 通过
go vet   ./...    ✅ 通过
go test -race ./internal/gateway/...  ✅ 通过 (9.054s)
```
