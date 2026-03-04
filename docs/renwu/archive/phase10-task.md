# Phase 10 任务清单 — 集成与验证

> 上下文：[phase10-bootstrap.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase10-bootstrap.md)
> 路线图：[refactor-plan-full.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/refactor-plan-full.md)
> 前置：Phase 9 ✅ (2026-02-16 审计通过)
> 最后更新：2026-02-17（审计更新 — 5 项代码已完成标记修正 + 3 项 §10.4 已完成标记修正）

---

## 总览

验证 Go 后端可完整替代 Node.js 后端，与前端 UI 和各频道网关兼容运行。

> **核心进展 (2026-02-16)**:
>
> - ✅ `llmclient` 实现：Anthropic/OpenAI/Ollama 流式客户端 (10/10 PASS)
> - ✅ `AttemptRunner` 实现：工具循环、Token 管理、API 调用
> - ✅ Agent Pipeline 接入：Gateway → Runner → LLM 全链路打通

---

## 10.0 Go 网关启动编排 ✅

> 新增: `server.go`, `ws_server.go`, `server_test.go`
> 修改: `cmd_gateway.go`

- [x] `ws_server.go` — 服务端 WebSocket (connect→hello-ok→req/res)
- [x] `server.go` — 网关编排 (State + Auth + Registry + HTTP mux)
- [x] `cmd_gateway.go` — CLI 对接 `RunGatewayBlocking`
- [x] `server_test.go` — 6 tests PASS

## 10.1 前端 ↔ Go 网关集成测试 ✅

> 架构说明：前端通过 WebSocket 方法通信，不使用 REST API

### HTTP 端点

- [x] `/health` → `{"status":"ok","phase":"ready"}`
- [x] `/ws` → WebSocket 升级成功

### WebSocket 方法（已注册）

- [x] `health` — 服务状态
- [x] `status` — 精简状态 (phase, version, clients)
- [x] `sessions.list` — 会话列表
- [x] `sessions.preview` — 会话预览（已注册，待端到端验证）
- [x] `sessions.resolve` — 会话解析（已注册）
- [x] `sessions.patch` — 会话更新（已注册）
- [x] `sessions.reset` — 会话重置（已注册）
- [x] `sessions.delete` — 会话删除（已注册）
- [x] `sessions.compact` — 会话压缩（已注册）

### 认证与事件

- [x] Token 认证握手 → hello-ok (protocol=3)
- [x] `presence.changed` 事件推送

### 已注册的 WS 方法 — Batch A ✅ (2026-02-16)

- [x] `config.get` / `config.set` / `config.apply` / `config.patch` / `config.schema` — 配置读写
- [x] `models.list` — 模型列表
- [x] `agents.list` — Agent 列表
- [x] `agent.identity.get` — Agent 身份
- [x] `agent.wait` — Agent 等待（stub）

### 已注册的 WS 方法 — Batch C/D/B ✅ (2026-02-16)

- [x] `chat.send` — 聊天消息发送（✅ 已接入 autoreply 管线，使用 stub DI）
- [x] `chat.abort` — 中断聊天
- [x] `chat.history` — 聊天历史（✅ 已接入 transcript JSONL 读取）
- [x] `chat.inject` — 注入消息（✅ 已接入 transcript JSONL 写入）
- [x] `send` — 频道消息发送（outbound pipeline deferred Window 2）
- [x] `channels.status` — 频道状态
- [x] `channels.logout` — 频道断开
- [x] `logs.tail` — 实时日志
- [x] `system-presence` — 系统在线状态
- [x] `system-event` — 系统事件
- [x] `last-heartbeat` — 最后心跳
- [x] `set-heartbeats` — 设置心跳
- [x] ~50 Batch D 方法 (stub) — wizard/cron/tts/skills/node/device/exec.approval 等

## 10.2 WS 方法处理器 + 端到端功能测试

### Batch A — 基础查询 ✅ (2026-02-16)

> 新增 4 个文件: `server_methods_config.go`, `server_methods_models.go`, `server_methods_agents.go`, `server_methods_agent.go`
> 扩展 `GatewayMethodContext` + `WsServerConfig` 增加 `ConfigLoader` + `ModelCatalog`
> 12 新测试 PASS

### Batch C — 频道 & 系统 ✅ (2026-02-16)

> 新增 4 个文件: `system_presence.go`, `server_methods_system.go`, `server_methods_channels.go`, `server_methods_logs.go`
> 扩展 `GatewayMethodContext`: +`PresenceStore` +`HeartbeatState` +`EventQueue` +`Broadcaster` +`LogFilePath` +`ChannelLogoutFn`
> 注册 6 个方法: `channels.status`, `channels.logout`, `logs.tail`, `system-presence`, `system-event`, `last-heartbeat`, `set-heartbeats`

### Batch D — 高级功能 Stubs ✅ (2026-02-16)

> 新增 1 个文件: `server_methods_stubs.go`
> 注册 ~50 个 stub 方法 (wizard/cron/tts/skills/node/device 等)

### Batch B — 聊天 & 发送 ✅ (2026-02-16)

> 新增 2 个文件: `server_methods_chat.go`, `server_methods_send.go`
> 扩展 `GatewayMethodContext`: +`ChatState`
> 注册 5 个方法: `chat.send`, `chat.abort`, `chat.history`, `chat.inject`, `send`
> 新增 `server_methods_batch_cdb_test.go`: 27 测试 PASS
>
> **Agent Pipeline 接入** (2026-02-16):
>
> - 新增 `transcript.go` (270L): JSONL 读写 + 消息清理 + 字节裁剪
> - 新增 `dispatch_inbound.go` (130L): gateway → autoreply 桥接（DI 回调打破循环导入）
> - 修改 `server_methods_chat.go`: 3 个 TODO 替换为真实管线调用
> - 修改 `server_methods_send.go`: deferred 标注
> - ✅ `go build` + `go test` 全通过
>
> **Agent Runner 核心实现** (2026-02-16):
>
> - 新增 `llmclient/` 包 (5 文件): 统一 LLM HTTP 流式客户端 (Anthropic/OpenAI/Ollama) + 自动路由
> - 新增 `runner/attempt_runner.go` (370L): `EmbeddedAttemptRunner` 实现，含 Tool Loop、Prompt 构建、错误重试
> - 新增 `runner/tool_executor.go` (180L): 本地工具执行器 (bash/fs)
> - ✅ `llmclient` 单元测试 10/10 PASS (-race)

### 端到端验证

- [x] Agent 对话管线链路（Pipeline 层）：`chat.send` → `DispatchInboundMessage` → transcript 写入 → 完成回调 ✅
  - 6 个 E2E 测试全通过 (`server_methods_e2e_test.go`, 2.6s)
  - `PipelineDispatcher` DI 注入已验证（mock 返回回复→transcript 持久化）
  - `chat.history` 读取 transcript JSONL ✅
  - `chat.inject` 写入 transcript + messageId 返回 ✅
  - `chat.abort` 中断机制 ✅
- [ ] Agent 对话 UI 层：发送消息 → AI 回复 → 显示在 UI（需真实 LLM API + 前端连接验证）
- [x] 命令系统验证 (`/help`, `/status`, `/model`, `/compact`, `/reset`) — `commands_handler_*.go` 全部实现（端到端验证 deferred）
- [ ] 频道连通性 (Telegram, Discord, Slack, WhatsApp, Signal, iMessage) — 代码已完成，需真实账号端到端验证
- [x] 工具调用 (Bash, 浏览器, 文件) — `tool_executor.go` + `browser/client_actions.go` 实现完成（30+ 测试）
- [x] 记忆系统 (Embedding, 向量搜索, 上下文注入) — `internal/memory/` 5 个 provider + `SearchVector` + flush 决策完成

### Window 4 — Gateway → AttemptRunner 真实接线 (2026-02-16)

> 任务：将 Gateway 中的 stub `PipelineDispatcher` 替换为真实的 `AttemptRunner` 调用

#### 规划阶段 ✅

- [x] 分析 `server.go` — 确认依赖注入点和启动编排逻辑
- [x] 分析 `server_methods_chat.go` — 确认 stub dispatcher 调用位置
- [x] 分析 `dispatch_inbound.go` — 理解当前 DI 回调机制
- [x] 分析 `AttemptRunner` 实现 — 确认接口和初始化需求
- [x] 制定实现计划（`implementation_plan.md`）

#### 执行阶段 ✅

- [x] 在 `server.go` 中实例化 `EmbeddedAttemptRunner`（包含 `Config` 和 `AuthStore`）
- [x] 实现真实的 `PipelineDispatcher` 函数（调用 `autoreply/reply.GetReplyFromConfig`）
- [x] 更新 `GatewayMethodContext` 注入真实 `PipelineDispatcher`
- [x] 修改依赖注入链：`WsServerConfig` → `GatewayMethodContext` → `handleChatSend`
- [x] 修复编译错误：`ModelFallbackExecutor{RunnerDeps, Config}` 正确 DI 注入

#### 验证阶段 ✅

- [x] 编译验证：`cd backend && go build ./...` ✅
- [x] 单元测试：`go test -race ./internal/gateway/...` ✅ (4.5s)
- [x] 集成测试：6 个 E2E 测试全通过（mock dispatcher）
- [x] 准备真实 E2E 验证文档（API Key 配置需求）
- [x] 真实 LLM E2E 验证：DeepSeek API 调用成功（7/7 测试通过，回复 "Hello to you."）
  - 修复 `resolveAPIKey()` 支持 deepseek 等通用 provider
  - 修复 `resolveBaseURL()` 使用 `models.ResolveProviderBaseURL()`
- [x] 更新架构文档
- [x] 创建 walkthrough

#### SessionEntry 循环导入修复 ✅

- [x] `directive_persist_test.go` 移除 `gateway` 导入，9 处 `gateway.SessionEntry` → `SessionEntry`（使用包内已有类型别名）
- [x] `go vet ./...` ✅ 无循环导入
- [x] 更新 `deferred-items.md` P8W2-D5 记录

## 10.3 性能基准对比 ✅ (2026-02-16)

> 新增 3 文件: `bench_test.go` 增强 (+160L), `bench_memory_test.go` (130L), `bench_report_test.go` (270L)
> 17 benchmarks PASS + 3 内存快照测试 PASS + 报告生成器 PASS
> 测试平台: Apple M4 Max, darwin/arm64, Go 1.25.7

- [x] 内存占用 (Go idle ~800KB heap delta vs Node.js ~50-80MB)
- [x] 冷启动时间 (~102ms vs Node.js ~2-5s)
- [x] 聊天消息延迟 (P50=1.9ms / P95=3.0ms / P99=3.6ms)
- [x] 并发连接数 (WebSocket 全链路压测 PASS)

## 10.4 穿插可选

- [x] DIS-F-4: `loadWebMedia` 统一到 `pkg/media/` — Discord/WhatsApp 已委托 `pkgmedia.LoadWebMedia` ✅
- [x] SLK-P7-A: `chunkMarkdownIR` Slack/TG 调用方接入 — `slack/format.go` + `telegram/format.go` 已使用 `markdown.ChunkMarkdownIR` ✅
- [x] P7A-3: Security Audit 完整实现 — `audit.go` 400L + `audit_extra.go` 300L + 30 测试 ✅ + `cmd_security.go` CLI 入口已串联 ✅
