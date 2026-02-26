# Phase 5D 频道适配器移植 — 任务清单

> 上下文：[phase5d-bootstrap.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase5d-bootstrap.md)
> 审计：[phase5-channels-deep-audit.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase5-channels-deep-audit.md)

## Phase 5A 延迟项修复（2026-02-14）

- [x] **P3-D4** `normalizePreviewRole` + `isToolCallMessage` → `session_utils_fs.go`
- [x] **P4-DRIFT5** `DowngradeOpenAIReasoningBlocks` 完整实现 → `helpers.go`
- [x] **P2-D6** `validateAction` 确认内联等价 → `hooks_mapping.go`
- [x] **P2-D3+D4** session key 路由模块 → **[NEW]** `internal/routing/session_key.go` (340L, 18 函数)
- 剩余 8 项延迟 → 见 [deferred-items.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-items.md)

## 已完成（本轮）

- [x] **5D.1** plugins/ 通用辅助（10 文件）
- [x] **5D.2** Onboarding 引导适配器（7 文件）
- [x] **5D.3** Actions 消息动作（3 文件）

## 待移植

### 5D.4 WhatsApp SDK (80L + web/ 5696L) ✅ 已完成

- [x] `whatsapp/normalize.ts` (80L) → WA 号码规范化
- [x] `web/accounts.ts` → WA 账户解析
- [x] `web/outbound.ts` → WA 发送（含轮询）
- [x] `web/gateway.ts` → WA 网关连接管理（骨架，待 Phase 6）
- [x] `web/heartbeat.ts` → WA 心跳监控
- [x] `web/` 其余文件 (~40 文件) → 网关全量逻辑（14 Go 文件）
- [x] 隐藏依赖审计 → 8 项发现，5 项已修复（H1/H4/H5/H6/H7），2 项延迟 → [审计报告](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/whatsapp-hidden-dep-audit.md)
- [x] 编译验证：`go build ./...` 通过（14 Go 文件）
- **延迟桩**：WA-A ~ WA-F (Phase 6) → 详见 [deferred-items.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-items.md)

### 5D.5 Signal SDK (14 文件 2567L) ✅ 已完成

- [x] `signal/accounts.ts` → Signal 账户解析 (`accounts.go`)
- [x] `signal/send.ts` + `send-reactions.ts` → Signal 发送/反应 (`send.go`, `send_reactions.go`)
- [x] `signal/reaction-level.ts` → 反应级别策略 (`reaction_level.go`)
- [x] `signal/` 其余文件 → identity/client/format/daemon/probe/sse/monitor/event_handler
- [x] 隐藏依赖审计修复 5 项（H1 endpoint / H2 text-style / M1 username / M2 HTTP201 / M3 timeout）→ [审计报告](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase5d-signal-audit.md)
- [x] 编译验证：`go build ./...` 通过（14 Go 文件）
- **延迟桩**：SIG-A ~ SIG-C (Phase 6) → 详见 [deferred-items.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-items.md)

### 5D.6 iMessage SDK (12 文件 1697L) ✅ 已完成

- [x] `imessage/targets.ts` → handle/target 规范化 (`targets.go`)
- [x] `imessage/accounts.ts` → 多账户解析 (`accounts.go`)
- [x] `imessage/client.ts` → JSON-RPC over stdio (`client.go`)
- [x] `imessage/probe.ts` → CLI 可用性探测 (`probe.go`)
- [x] `imessage/send.ts` → iMessage 发送 (`send.go`)
- [x] `imessage/monitor/` → 监控骨架 + Phase 6 桩 (`monitor_types.go`, `monitor.go`)
- [x] 隐藏依赖审计 → 11 项发现，5 项已修复（H1/H4/H8/H10/H11） → [审计报告](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase5d-imessage-audit.md)
- [x] **全局审计（第二轮）** → 10 项新发现（G1-G10），类型 26/26 字段零遗漏，前序 H1-H11 全部确认 → [全局审计报告](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase5d-imessage-global-audit.md)
- [x] 编译验证：`go build ./...` 通过（8 Go 文件）
- **延迟桩**：IM-A ~ IM-E (Phase 6/7) → 详见 [deferred-items.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-items.md)

### 5D.7 Telegram SDK (40 文件 7929L) ✅ 已完成

- [x] 叶子工具：`targets.go`, `caption.go`, `voice.go`, `network.go` (merged 3 TS)
- [x] 账户/Token：`accounts.go`, `token.go`, `reaction_level.go`, `sent_message_cache.go`
- [x] 格式/网络：`format.go`, `http_client.go` (merged fetch+proxy), `download.go`, `api_logging.go`
- [x] 发送核心：`send.go` (800L→500L)
- [x] Bot 辅助：`bot_types.go`, `bot_helpers.go`, `inline_buttons.go`, `update_offset_store.go`, `group_migration.go`
- [x] 功能模块：`sticker_cache.go`, `probe.go`, `model_buttons.go`, `audit.go`, `bot_access.go`
- [x] 流/监控：`draft_chunking.go`, `draft_stream.go`, `webhook.go` (merged webhook-set), `monitor.go`
- [x] Bot 核心：`bot.go`, `bot_updates.go`, `bot_message_context.go`, `bot_delivery.go`, `bot_message.go`
- [x] Bot 分发：`bot_message_dispatch.go`, `bot_native_commands.go`, `bot_handlers.go`
- [x] 编译验证：`go build ./...` 通过（35 Go 文件）
- [x] 架构文档：`docs/gouji/telegram.md`
- [x] 隐藏依赖审计 → 5 项发现，4 项已修复（HD-1/HD-2/HD-3/HD-4），1 项延迟（HD-5 resolveTableMode）
- **延迟桩**：TG-1 ~ TG-5 (Phase 6) + TG-HD5 (Phase 6 config) → 详见 [deferred-items.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-items.md)

### 5D.8 Slack SDK (43 文件 5809L) ✅ 已完成

- [x] Batch 1 叶子工具：`types.go`, `token.go`, `client.go`, `targets.go`, `threading.go`
- [x] Batch 2 账户/格式：`accounts.go`, `format.go`, `scopes.go`, `probe.go`
- [x] Batch 3 动作/解析：`actions.go`, `resolve_channels.go`, `resolve_users.go`
- [x] Batch 4 辅助：`channel_migration.go`, `directory_live.go`, `threading_tool_context.go`, `http_registry.go`, `send.go`
- [x] Batch 5 监控工具：`monitor_types.go`, `allow_list.go`, `monitor_auth.go`, `monitor_policy.go`, `monitor_commands.go`, `monitor_channel_config.go`
- [x] Batch 6-10 监控核心（Phase 6 桩）：14 个 monitor_*.go 文件
- [x] 编译验证：`go build ./...` 通过（37 Go 文件，3591L）
- [x] 隐藏依赖审计 → 7 类 19 项（9✅ + 10⚠️），2 项 `panic` 已修复 → [审计报告](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase5d-slack-audit.md)
- **延迟桩**：SLK-A ~ SLK-I (Phase 6) + SLK-P7-A/B (Phase 7) → 详见 [deferred-items.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-items.md)

### 5D.9 Discord SDK (44 文件 8733L) ✅ 已完成

- [x] Batch 1 叶子工具：`token.go`, `pluralkit.go`, `api.go`, `gateway_logging.go`, `account_id.go`
- [x] Batch 2 账户/目标：`accounts.go`, `targets.go`, `audit.go`, `probe.go`
- [x] Batch 3 解析/分块：`directory_live.go`, `resolve_channels.go`, `resolve_users.go`, `chunk.go`
- [x] Batch 4-5 Send 层：`send_types.go`, `send_shared.go`, `send_permissions.go`, `send_messages.go`, `send_reactions.go`, `send_channels.go`, `send_guild.go`
- [x] Batch 6 Monitor 工具：`monitor_format.go`, `monitor_sender_identity.go`, `monitor_allow_list.go`, `monitor_message_utils.go`, `monitor_threading.go`
- [x] 审计铺砌 Batch A：新建 `send_media.go`（媒体下载+multipart），`send_shared.go` +FormatReactionEmoji/SendDiscordText 增强/SendDiscordMedia
- [x] 审计铺砌 Batch B：新建 `send_emojis_stickers.go`（emoji/sticker 上传），`send_guild.go` +SendMessageDiscord media/embeds/chunkMode + ParseAndResolveRecipient + NormalizeDiscordPollInput
- [x] 隐藏依赖审计 → 4 项 P0 全部修复 + 6 项 P1 中 4 项修复 / 2 项延迟 + 3 项 P2 记录 → [审计报告](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase5d-discord-audit.md)
- [x] 编译验证：`go build ./...` + `go vet ./...` 通过（27 Go 文件）
- **延迟桩**：DIS-A ~ DIS-F (Phase 6/7) → 详见 [deferred-items.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-items.md)

> [!NOTE]
> 5D.4-5D.9 均为独立频道 SDK，建议每个频道一个独立对话。

## 验证

- [ ] 编译验证: `go build ./...`
- [ ] 单元测试: `go test -race ./internal/channels/...`
- [ ] 契约完整性: contracts 接口覆盖审查

## 待对接桩汇总（Phase 6 Gateway 集成）

> 按 `/refactor` 工作流步骤 3 隐藏依赖审计 7 类格式归类。
> 各频道完成后在对应章节追加。

### Signal — `event_handler.go` (5 处 TODO)

| # | 桩函数 | 隐藏依赖类别 | 职责 | Go 等价方案 |
|---|--------|-------------|------|------------|
| 1 | `dispatchInboundMessage` | 事件总线/回调链 | 入站消息 → resolveAgentRoute → recordInboundSession → deliverReplies | Gateway 消息分发管线 |
| 2 | `enqueueSystemEvent` | 事件总线/回调链 | 反应通知 → sessionKey 路由 → 事件入队 | Gateway 事件队列 |
| 3 | `upsertChannelPairingRequest` | 全局状态/单例 | 配对请求管理（pairing 模式下新发送者注册） | `channels.PairingRegistry` |
| 4 | `fetchAttachment` | 协议/消息格式 | signal-cli REST attachment 下载 + 媒体保存 | `signal.Client.FetchAttachment` + 媒体存储层 |
| 5 | `sendReadReceiptSignal` | 协议/消息格式 | signal-cli REST 发送已读回执 | `signal.Client.SendReceipt` |

### WhatsApp — 5 处桩函数 + 2 处隐藏遗漏

| # | 桩函数 | 文件:行 | 隐藏依赖类别 | 现状 | P6 Go 等价方案 |
|---|--------|---------|-------------|------|---------------|
| 1 | `LoginWeb()` | `login.go:36` | npm 包黑盒行为 (Baileys WebSocket) | 仅检查 creds.json，无连接→返回静态消息 | whatsmeow WebSocket 握手 + QR 码扫描 |
| 2 | `StartWebLoginWithQR()` | `login_qr.go:70` | npm 包黑盒行为 (Baileys QR) | 状态管理完整，但无 QR 数据→返回静态消息 | whatsmeow QR DataURL 生成 + 推送 |
| 3 | `StartAutoReply()` | `auto_reply.go:44` | 事件总线/回调链 | `return nil`（空操作） | Gateway 消息分发管线 + Agent 引擎路由 |
| 4 | `ActiveWebListener` 接口 | `active_listener.go:24` | npm 包黑盒行为 (Baileys) | 接口已定义，无实现类型 | whatsmeow 适配器实现该接口 |
| 5 | 媒体优化管线（缺失） | `media.go:14` | 协议/消息格式 | 仅有加载+MIME 检测，无 HEIC→JPEG/PNG 优化 | `goheif`/`imaging` + `pngquant` CLI |
| 6 | **Markdown 表格转换** 🔴 | `outbound.go:45` | 协议/消息格式 | TS 有 `convertMarkdownTables` + 配置驱动，Go 完全缺失 | 移植 `markdown/tables.go` |
| 7 | auth-store 辅助 + 日志 | `auth_store.go` | 全局状态 + 可观测 | 缺 `WA_WEB_AUTH_DIR`、`logWebSelfId`、`pickWebChannel`、结构化日志 | Phase 6 日志基础设施 |

> 对应 `deferred-items.md` 中的 WA-A ~ WA-F 条目。

### 核心层 — 插件注册桩 (3 处)

| # | 文件 | 位置 | 隐藏依赖类别 | 职责 | Go 等价方案 |
|---|------|------|-------------|------|------------|
| 1 | `dock.go` | L160 | 全局状态/单例 | 插件频道动态追加到 `AllDocks` | `PluginRegistry` 启动时注入 |
| 2 | `message_actions.go` | L28 | 全局状态/单例 | 动态扩展频道可用动作列表 | `PluginRegistry` 注册回调 |
| 3 | `message_actions.go` | L34 | 全局状态/单例 | 动态解析频道消息动作处理器 | `PluginRegistry` 查询接口 |

### iMessage — `monitor.go` / `send.go` (6 处 TODO)

| # | 桩函数 | 隐藏依赖类别 | 职责 | Go 等价方案 |
|---|--------|-------------|------|------------|
| 1 | `handleInboundMessage` 内 `dispatchInbound` | 事件总线/回调链 | 入站消息分发管线 | Gateway 消息分发 |
| 2 | `handleInboundMessage` 内 `upsertPairingRequest` | 全局状态/单例 | 配对请求管理 | `channels.PairingRegistry` |
| 3 | `handleInboundMessage` 内 `resolveAgentRoute` | 事件总线/回调链 | Agent 路由解析 | Phase 4 Agent 引擎 |
| 4 | `resolveAttachment` | 协议/消息格式 | 媒体下载+存储 | `web/media` + 媒体存储层 |
| 5 | `convertMarkdownTables` | — | Markdown 表格转换 | `markdown/tables` 包 |
| 6 | `DeliverReplies` 分块 | — | 文本分块+表格转换 | `auto-reply/chunk` 包 |

### Telegram — 5 处桩函数 (Phase 6 集成)

| # | 桩函数 | 文件 | 隐藏依赖类别 | 职责 | Go 等价方案 |
|---|--------|------|-------------|------|------------|
| 1 | `DispatchTelegramMessage` | `bot_message_dispatch.go` | 事件总线/回调链 | 入站消息 → Agent 引擎路由 → 回复投递 | Gateway 消息分发管线 |
| 2 | `RegisterTelegramHandlers` (完整实现) | `bot_handlers.go` | 事件总线/回调链 | debouncer、媒体组聚合、回调按钮处理 | Channel Handler 注册体系 |
| 3 | `RegisterTelegramNativeCommands` (完整管线) | `bot_native_commands.go` | 事件总线/回调链 | /start, /help, /reset 完整处理（含 Agent 调用） | Gateway 命令管线 |
| 4 | `pollOnce` (更新分发) | `monitor.go` | npm 包黑盒行为 (grammy runner) | getUpdates 后分发到处理器 | Channel 更新分发管线 |
| 5 | `DescribeStickerImage` (Vision API) | `sticker_cache.go` | 协议/消息格式 | 贴纸图像描述→缓存 | Phase 7 媒体理解模块 |

### Slack — 9 处 Phase 6 桩 + 2 处 Phase 7 桩

| # | 桩 ID | 桩函数/模块 | 文件 | 隐藏依赖类别 | 职责 | 状态 |
|---|-------|-----------|------|-------------|------|------|
| 1 | SLK-A | `monitorSlackSocket` | `monitor_provider.go:51` | #1 npm 包黑盒 + #3 事件总线 | Socket Mode WebSocket 连接+事件循环 | ⏳ Phase 6 |
| 2 | SLK-B | `monitorSlackHTTP` | `monitor_provider.go:64` | #6 协议/消息格式 | HTTP 事件验证+签名校验+分发 | ⏳ Phase 6 |
| 3 | SLK-C | 消息分发管线 | `monitor_events_messages.go` + `prepare.go` + `dispatch.go` | #3 事件总线 | 入站消息分发→Agent 路由→回复投递 | ⏳ Phase 6 |
| 4 | SLK-D | 事件处理器 (12 handler) | `monitor_events_*.go` | #2 全局状态 + #3 事件总线 | 频道/成员/pin/反应事件→缓存更新 | ⏳ Phase 6 |
| 5 | SLK-E | 监控上下文缓存 | `monitor_context.go` | #2 全局状态 | 频道/用户缓存预加载+API fallback | ⏳ Phase 6 |
| 6 | SLK-F | 线程历史+回复发送 | `monitor_thread_resolution.go` + `monitor_replies.go` | #6 协议 + #7 错误处理 | conversations.replies+状态反应+流式编辑 | ⏳ Phase 6 |
| 7 | SLK-G | 斜杠命令处理 | `monitor_slash.go` | #6 协议/消息格式 | slash command payload 解析+ephemeral 回复 | ⏳ Phase 6 |
| 8 | SLK-H | Pairing Store 集成 | `monitor_auth.go` | #2 全局状态 | pairing store 动态 allowFrom 读取 | ⏳ Phase 6 |
| 9 | SLK-I | 媒体下载 | `monitor_media.go` | #6 协议/消息格式 | bot token 认证文件下载+媒体存储 | ⏳ Phase 6 |
| 10 | SLK-P7-A | Markdown IR 中间层 | `format.go:86,93` | — | Markdown→IR→mrkdwn 转换管线 | ⏳ Phase 7 |
| 11 | SLK-P7-B | 媒体上传+分块 | `send.go:24` | — | files.uploadV2 + mode-aware 分块 | ⏳ Phase 7 |

> 详细 TS 参考行号和实现方案见 [deferred-items.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-items.md) 的「Phase 6 Slack Gateway 集成」章节。

### Discord — 13 个 monitor TS 文件延迟 (Phase 6) + 4 项 Send 层延迟 (DIS-F)

> 详细 7 类隐藏依赖检查和修复记录见 [审计报告](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/phase5d-discord-audit.md)。

| # | 延迟文件/函数 | 隐藏依赖类别 | 职责 | Go 等价方案 |
|---|-------------|-------------|------|------------|
| 1 | `monitor/listeners.ts` (322L) | 事件总线/回调链 | Gateway 事件绑定（消息/反应/typing） | Channel Handler 注册体系 |
| 2 | `monitor/provider.ts` (690L) | npm 包黑盒行为 (`@buape/carbon`) | Monitor 生命周期管理（start/stop/reconnect） | Gateway 集成层 |
| 3 | `monitor/message-handler.ts` (146L) | 事件总线/回调链 | 消息路由分发（preflight→process→reply） | Gateway 消息分发管线 |
| 4 | `monitor/message-handler.preflight.ts` (577L) | 全局状态/单例 + 环境变量 | 消息预检（allowlist、mention、bot 过滤） | allowlist + channel config |
| 5 | `monitor/message-handler.process.ts` (450L) | 事件总线/回调链 | 消息处理（转录构建→Agent 调用→回复投递） | Gateway 消息处理管线 |
| 6 | `monitor/exec-approvals.ts` (579L) | 事件总线/回调链 + 协议/消息格式 | 执行审批 UI（按钮交互→approve/deny） | Gateway 审批管线 |
| 7 | `monitor/gateway-registry.ts` (37L) | 全局状态/单例 | Gateway 客户端注册表 | Gateway 注册表 |
| 8 | `monitor/native-command.ts` (935L) | 事件总线/回调链 | 原生命令处理（/reset, /help 等） | Gateway 命令管线 |
| 9 | `monitor/presence-cache.ts` (61L) | 全局状态/单例 | 在线状态缓存 | `sync.Map` 缓存 |
| 10 | `monitor/reply-context.ts` (45L) | 全局状态/单例 | 回复上下文跟踪 | session context |
| 11 | `monitor/reply-delivery.ts` (81L) | 事件总线/回调链 | 回复发送（文本+媒体+分块） | Gateway 回复投递 |
| 12 | `monitor/system-events.ts` (55L) | 事件总线/回调链 | 系统事件发送（加入/离开通知） | Gateway 事件队列 |
| 13 | `monitor/typing.ts` (11L) | 协议/消息格式 | 输入指示器发送 | Discord REST API |
