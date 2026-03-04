# Phase 10.2 Bootstrap — WS 方法处理器注册

> 新窗口上下文文件。请先阅读本文档再开始工作。

## 一、目标

将 TS 网关的 **27 个处理器组** 中尚未注册的 WS 方法处理器逐批实现并注册到 Go `MethodRegistry`。

## 二、当前状态

### ✅ 已完成（Phase 10.0-10.1）

| 组件 | 文件 | 说明 |
|------|------|------|
| WS 处理器 | `internal/gateway/ws_server.go` (~260L) | connect→hello-ok→req/res + event push |
| 启动编排 | `internal/gateway/server.go` (~200L) | 组装所有子系统 |
| CLI 对接 | `cmd/openacosmi/cmd_gateway.go` | `RunGatewayBlocking` |
| 会话方法 | `server_methods_sessions.go` (~656L) | sessions.* (7 个方法) |
| Health/Status | `server.go:102-114` | 内联注册 |
| 认证 & 授权 | `auth.go`, `server_methods.go` | 全部方法的 scope 已定义 |
| Chat 运行时 | `chat.go` (~417L) | `AgentEventHandler`, `ChatRunRegistry`, `ChatRunState` |
| 广播器 | `broadcast.go` (~215L) | `Broadcaster`, `WsClient` |
| 协议 | `protocol.go` (~267L) | 所有帧类型定义 |

### ✅ Batch A — 基础查询（2026-02-16 完成）

| 方法 | 文件 | 状态 |
|------|------|------|
| `config.get/set/apply/patch/schema` | `server_methods_config.go` (286L) | ✅ |
| `models.list` | `server_methods_models.go` (34L) | ✅ |
| `agents.list` | `server_methods_agents.go` (109L) | ✅ |
| `agent.identity.get` | `server_methods_agent.go` (76L) | ✅ |
| `agent.wait` | 同上（stub，Batch C 完整实现） | ✅ |
| `status` | 已在 `server.go` 内联 | ✅ |

### ⬜ 未注册的方法（按优先级分批） — ✅ 全部完成 (2026-02-16)

**Batch B — 聊天 & Agent ✅ (2026-02-16)**

| 方法 | 文件 | 状态 |
|------|------|------|
| `chat.send` | `server_methods_chat.go` | ✅ Pipeline 接入完成 (DI dispatcher, E2E 验证通过) |
| `chat.abort` | 同上 | ✅ |
| `chat.history` | 同上 | ✅ transcript JSONL 读取已实现 + E2E 验证 |
| `chat.inject` | 同上 | ✅ transcript 写入已实现 + E2E 验证 |
| `send` | `server_methods_send.go` | ✅（outbound pipeline 待接线，Window 2） |

**Batch C — 频道 & 系统 ✅ (2026-02-16)**

| 方法 | 文件 | 状态 |
|------|------|------|
| `channels.status` | `server_methods_channels.go` | ✅ |
| `channels.logout` | 同上 | ✅ |
| `logs.tail` | `server_methods_logs.go` | ✅ |
| `system-presence` | `server_methods_system.go` | ✅ |
| `system-event` | 同上 | ✅ |
| `last-heartbeat` | 同上 | ✅ |
| `set-heartbeats` | 同上 | ✅ |

**Batch D — 高级功能 (Stub) ✅ (2026-02-16)**

> `server_methods_stubs.go` 中注册 ~50 个 stub 方法，返回 `{ok:true, stub:true}`

| 方法 | 状态 |
|------|------|
| `wizard.*` (4) | ✅ stub |
| `cron.*` (6) | ✅ stub |
| `tts.*` (5) | ✅ stub |
| `skills.*` (3) | ✅ stub |
| `node.*` (6) | ✅ stub |
| `device.*` (4) | ✅ stub |
| `exec.approval.*` (4) | ✅ stub |
| `voicewake.*` (2) | ✅ stub |
| `update.*` (1) | ✅ stub |
| `browser.request` (1) | ✅ stub |

## 三、关键文件索引

### Go 后端

```
backend/internal/
├── gateway/
│   ├── server.go              ← 启动编排 + 方法注册入口
│   ├── server_methods.go      ← GatewayMethodHandler 类型 + 授权
│   ├── server_methods_sessions.go  ← sessions.* 处理器 (参考模式)
│   ├── server_methods_config.go    ← config.* 处理器 (Batch A) ✅
│   ├── server_methods_models.go    ← models.* 处理器 (Batch A) ✅
│   ├── server_methods_agents.go    ← agents.* 处理器 (Batch A) ✅
│   ├── server_methods_agent.go     ← agent.* 处理器 (Batch A) ✅
│   ├── server_methods_channels.go  ← channels.* 处理器 (Batch C) ✅
│   ├── server_methods_system.go    ← system-* 处理器 (Batch C) ✅
│   ├── server_methods_logs.go      ← logs.* 处理器 (Batch C) ✅
│   ├── server_methods_chat.go      ← chat.* 处理器 (Batch B) ✅
│   ├── server_methods_send.go      ← send 处理器 (Batch B) ✅
│   ├── server_methods_stubs.go     ← ~50 stub 处理器 (Batch D) ✅
│   ├── system_presence.go     ← SystemPresenceStore + HeartbeatState + EventQueue (Batch C) ✅
│   ├── chat.go                ← ChatRunRegistry + AgentEventHandler
│   ├── agent_run_context.go   ← Agent 运行上下文
│   ├── broadcast.go           ← Broadcaster
│   ├── protocol.go            ← 帧类型
│   ├── ws_server.go           ← WS 连接循环
│   ├── boot.go                ← GatewayState, BootConfig
│   ├── auth.go                ← 认证逻辑
│   ├── reload.go              ← 配置热加载
│   └── http.go                ← HTTP 服务器

├── config/
│   ├── configpath.go          ← 配置文件路径
│   ├── mergeconfig.go         ← 配置合并
│   └── agentdirs.go           ← Agent 目录扫描
├── agents/models/
│   ├── config.go              ← 模型配置
│   └── model_compat.go        ← 模型兼容层
├── autoreply/
│   └── reply/
│       ├── agent_runner.go        ← Agent 执行器 (chat.send 核心)
│       └── agent_runner_execution.go
└── channels/
    ├── config_helpers.go      ← 频道配置
    └── config_schema.go       ← 配置 schema
```

### TypeScript 参考

```

src/gateway/
├── server-methods.ts          ← coreGatewayHandlers (27 组)
├── server-methods-list.ts     ← BASE_METHODS (88) + GATEWAY_EVENTS (18)
├── server-methods/
│   ├── chat.ts       ← chat.send, chat.abort, chat.history
│   ├── config.ts     ← config.get, config.set, config.apply, config.patch, config.schema
│   ├── models.ts     ← models.list
│   ├── agents.ts     ← agents.list, agents.create, agents.update, agents.delete
│   ├── agent.ts      ← agent, agent.identity.get, agent.wait
│   ├── sessions.ts   ← ✅ 已移植
│   ├── health.ts     ← ✅ 已移植
│   ├── channels.ts   ← channels.status, channels.logout
│   ├── send.ts       ← send
│   ├── system.ts     ← system-presence, system-event
│   ├── logs.ts       ← logs.tail
│   └── ... (其余见 Batch D)
├── server-chat.ts             ← ChatRunState + AgentEventHandler (✅ 已移植)
└── server.impl.ts             ← startGatewayServer 顶层编排 (✅ 已移植)

```

## 四、处理器注册模式

参考 `server_methods_sessions.go` 的实现模式：

```go
// 1. 定义处理器函数 (在 server_methods_xxx.go 中)
func handleConfigGet(ctx *MethodHandlerContext) {
    // 从 ctx.Context 获取依赖
    // 解析 ctx.Params
    // 调用业务逻辑
    // ctx.Respond(ok, payload, err)
}

// 2. 在 server.go:StartGatewayServer 中注册
registry.RegisterAll(map[string]GatewayMethodHandler{
    "config.get": handleConfigGet,
    "config.set": handleConfigSet,
})
```

`GatewayMethodContext` 结构（Batch A/B/C 扩展后）：

```go
type GatewayMethodContext struct {
    SessionStore    *SessionStore
    StorePath       string
    Config          *types.OpenAcosmiConfig
    ConfigLoader    *config.ConfigLoader       // Batch A
    ModelCatalog    *models.ModelCatalog       // Batch A
    PresenceStore   *SystemPresenceStore       // Batch C
    HeartbeatState  *HeartbeatState            // Batch C
    EventQueue      *SystemEventQueue          // Batch C
    Broadcaster     *Broadcaster               // Batch C
    LogFilePath     string                     // Batch C
    ChannelLogoutFn ChannelLogoutFunc          // Batch C
    ChatState       *ChatRunState              // Batch B
}
```

## 五、执行建议

1. **从 Batch A 开始** — `config.get` 和 `models.list` 不依赖 LLM，可以独立测试
2. **每个 Batch 一个 server_methods_xxx.go 文件** — 如 `server_methods_config.go`, `server_methods_chat.go`
3. **遵循六步循环法** — 按 `/refactor` 工作流执行
4. **验证命令**: `go build ./...`, `go vet ./...`, `go test ./internal/gateway/...`
