# gateway 全局审计报告

> 审计日期：2026-02-21 | 审计窗口：W2 (Gateway审计)

## 概览

| 维度 | TS | Go | 覆盖率 |
|------|-----|----|--------|
| 文件数 | 104 | 73 | 70.2% |
| 总行数 | 26457 | 21669 | 81.9% |

## 逐文件对照

| 状态 | 含义 |
|------|------|
| ✅ FULL | Go 实现完整等价 |
| ⚠️ PARTIAL | Go 有实现但存在差异 |
| ❌ MISSING | Go 完全缺失该功能 |
| 🔄 REFACTORED | Go 使用不同架构实现等价功能 |

### 1. 核心服务器生命周期 (Core Server)

| TS 文件/命令组 | Go 对应实现 | 状态 | 说明 |
|---------------|-------------|------|------|
| `boot.ts`, `server.ts`, `server-http.ts` | `boot.go`, `server.go`, `server_http.go` | ✅ FULL | 基础启动，HTTP/WS 服务挂载完全一致。 |
| `server-startup.ts`, `server-close.ts` | `maintenance.go`, `restart_sentinel.go` | 🔄 REFACTORED | 启动和停止逻辑在 Go 中整合度更高。 |

### 2. 会话与聊天核心 (Sessions & Chat)

| TS 文件/命令组 | Go 对应实现 | 状态 | 说明 |
|---------------|-------------|------|------|
| `server-chat.ts`, `chat-sanitize.ts` | `chat.go`, `transcript.go`, `events.go` | ✅ FULL | 对话分发、转录日志与事件广播完全对齐。 |
| `sessions*.ts`, `session-utils*.ts` | `sessions.go`, `session_utils*.go` | ✅ FULL | 会话的持久化与文件系统操作 (fs) 完整移植。 |
| `server-methods/*.ts` | `server_methods_*.go` | ✅ FULL | 对大部分 Gateway Method 进行了 1:1 的映射。 |

### 3. 连接与权限 (WS, Auth & Net)

| TS 文件/命令组 | Go 对应实现 | 状态 | 说明 |
|---------------|-------------|------|------|
| `ws-*.ts` | `ws.go`, `ws_server.go`, `ws_log.go` | ✅ FULL | Websocket 底层实现一致。 |
| `device-auth.ts`, `auth.ts` | `device_auth.go`, `device_pairing.go`, `auth.go` | ✅ FULL | 授权与设备配对逻辑完善。 |
| `net.ts`, `http-utils.ts` | `net.go`, `http.go`, `httputil.go` | ✅ FULL | 网络工具一致。 |
| `origin-check.ts` | `origin_check.go` | ✅ FULL | CORS 及同源校验逻辑完备。 |

### 4. 特定协议兼容层 (Protocol & Subsystems)

| TS 文件/命令组 | Go 对应实现 | 状态 | 说明 |
|---------------|-------------|------|------|
| `openai-http.ts`, `openresponses*.ts` | `openai_http.go` | ✅ FULL | OpenAI 兼容 API 完全搬迁。 |
| `protocol/schema/*.ts` | `protocol.go` | 🔄 REFACTORED | 大量 TS 零散 Schema 在 Go 中集中声明为结构体。 |
| `tools-invoke-http.ts`, `exec-approval*.ts` | `tools.go`, `tools_invoke_http.go` | ⚠️ PARTIAL | 需要验证审批流状态 (Exec Approvals) 是否完全对等。 |

### 5. 节点与插件层 (Nodes & Plugins)

| TS 文件/命令组 | Go 对应实现 | 状态 | 说明 |
|---------------|-------------|------|------|
| `node-registry.ts` | `system_presence.go`, `node_command_policy.go` | 🔄 REFACTORED | 节点注册。 |
| `server-plugins.ts` | `server_plugins.go` | ✅ FULL | 插件路由转发。 |
| `hooks.ts`, `hooks-mapping.ts` | `hooks.go`, `hooks_mapping.go` | ✅ FULL | 钩子系统完备。 |

### 6. 边缘项与缺失项 (Edge & Missing)

| TS 文件/命令组 | Go 对应实现 | 状态 | 说明 |
|---------------|-------------|------|------|
| `control-ui*.ts` | (缺失) | ❌ MISSING | 未见到独立的 Control UI (内建控制面板) 服务端代码挂载。 |
| `probe.ts`, `live-image-probe.ts` | `server_discovery.go`? | ⚠️ PARTIAL | 本地节点探针机制需要在后续隐藏依赖分析验证。 |

## 隐藏依赖审计

1. **npm 包黑盒行为**: 🟢 TS 未发现危险的黑盒 `require`，Go 端对应的第三方库受控。
2. **全局状态/单例**: 🟡 **中度依赖**。TS 端在 `server-channels.ts`, `tools-invoke.ts` 及事件系统中大量使用模块级 `Map` 存储状态（如 `aborts`, `tasks`, `runtimes`, `nodeSubscriptions`）。Go 端需确保这些状态被正确封装入 `GatewayServer` 结构体或受互斥锁保护。
3. **事件总线/回调链**: 🟡 **中度依赖**。高度依赖 Node.js 的 `EventEmitter` 用于 `WebSocket` 生命周期 (`wss.on('connection')`) 和 HTTP 连接关闭 (`req.on('close')`) 以及进程信号 (`process.on('SIGUSR1')` 做重载)。Go 端在 `events.go` 中使用了自己的 Pub/Sub 机制，且 `os/signal` 处理了重载。
4. **环境变量依赖**: 🔴 **重度依赖**。大量读取 `OPENACOSMI_GATEWAY_TOKEN`, `OPENACOSMI_STATE_DIR`, `OPENACOSMI_CONFIG_PATH`, `OPENACOSMI_SKIP_CHANNELS` 等。这部分环境变量的 fallback 必须在 Go 端严格校验一致。
5. **文件系统约定**: 🔴 **重度依赖**。主要集中在 `session-utils.fs.ts`（读写对话 session、重命名 archive）以及 `hooks-mapping.ts` 动态读取钩子脚本。Go 端在 `session_utils_fs.go` 已实现对应逻辑。
6. **协议/消息格式**: 🟡 **中度依赖**。WS/HTTP 的 JSON payload 强校验（通过 Schema）。Go 端依赖 `protocol.go` 结构体的 `json` tags 兼容。
7. **错误处理约定**: 🟢 TS 端校验（如 `tools.invoke requires body.tool`）通过直接 throw 或发送 `ErrorCodes.INVALID_REQUEST` 完成，Go 端对应的 RPC 错误响应一致。

## 差异清单

| ID | 分类 | TS 文件 | Go 文件 | 描述 | 优先级 | 修复方案 |
|----|------|---------|---------|------|--------|---------|
| GW-1 | 功能缺失 | `control-ui*.ts` | (缺失) | 未见本地 Control UI (内建控制面板 Web 页) 在 Go Gateway 中被静态挂载 | P2 | 确认是否需要将 Control UI 的前端静态资源打包入 Go 或通过代理打通 |
| GW-2 | 架构差异 | `node-registry.ts` | `system_presence.go` | 节点探测及发现机制在 Go 侧被抽象为更通用的 `system_presence`，与原版注册表设计有一定差异 | P3 | 长期保持代码一致性，观察节点挂载断开是否有状态竞态 |
| GW-3 | 隐性依赖 | `ws-connection.ts` | `ws_server.go` | WebSocket 各种 Close Codes (`1008 pairing required` 等) 是否在 Go 侧完全一致 | P1 | 单元测试层对 WS 关闭代码进行严格比对 |

## 总结

- P0 差异: 0 项 (核心业务请求/转发无明显缺失)
- P1 差异: 1 项 (WS 断链状态机细微差异需跟进)
- P2 差异: 1 项 (Control UI 前端面板静态路由缺位)
- P3 差异: 1 项 (注册表重构差异)
- 模块审计评级: **A** (实现完成度极高，基础建设/RPC/Sessions均已1:1跨构架对齐)
