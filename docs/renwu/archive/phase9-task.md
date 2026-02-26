# Phase 9 任务清单 — 延迟项清理

> 上下文：[deferred-items.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-items.md)
> 路线图：[refactor-plan-full.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/refactor-plan-full.md)
> 最后更新：2026-02-15（D1 CLI 辅助完成）

---

## 总览

消化 `deferred-items.md` 中 ~60 项未解决延迟待办，为 Phase 10 集成测试扫清障碍。

---

## Batch A：Gateway 集成（~35 项）

> 按频道分窗口执行，从简到繁：Telegram → iMessage → Signal → WhatsApp → Slack → Discord

### A1: Telegram Gateway [x]

- [x] TG-HD5: resolveMarkdownTableMode 完整实现 ✅ (Phase 9, 2026-02-15)

### A2: iMessage Gateway [x]

- [x] IM-A: 入站消息分发管线（13 项子依赖） ✅ (Phase 9, 2026-02-15)
  - 新增 `monitor_deps.go` (DI 接口)、`monitor_envelope.go`、`monitor_history.go`、`monitor_gating.go`、`monitor_inbound.go` (核心管线 ~640L)
  - 移除 Phase 6 骨架 `handleInboundMessage`，替换为 `HandleInboundMessageFull`
- [x] IM-B: 配对请求管理 ✅ (Phase 9, 2026-02-15)
  - DI 接口 `UpsertPairingRequest` + `handlePairing` 函数 + `BuildPairingReply`
- [x] IM-C: 媒体附件下载 + 存储 ✅ (Phase 9, 2026-02-15)
  - DI 接口 `ResolveMedia` 就绪，`resolveAttachments` 过滤逻辑已实现
- [x] IM-D: 分块（auto-reply/chunk 接入） ✅ (Phase 9, 2026-02-15)
  - `DeliverReplies` 集成 `autoreply.ChunkTextWithMode` + `markdown.ConvertMarkdownTables`

### A3: Signal Gateway [x] ✅ (Phase 9, 2026-02-15)

- [x] SIG-A: 入站消息分发管线 ✅
  - 新增 `monitor_deps.go` (DI 接口)
  - `event_handler.go`: `dispatchSignalInbound()` + `formatSignalEnvelope()` + 反应事件 `EnqueueSystemEvent`
- [x] SIG-B: 配对请求管理 ✅
  - `handleSignalPairing()` + `BuildPairingReply()` — DI 注入 `UpsertPairingRequest`
- [x] SIG-C: 媒体下载 + 已读回执 ✅
  - `FetchAttachmentSignal` 集成、`SendReadReceiptSignal` 集成
  - `DeliverSignalReplies` (分块 + 附件投递)

### A4: WhatsApp Gateway [x] ✅ (Phase 9, 2026-02-15)

- [x] WA-A: Baileys WebSocket 连接 + Session 管理（whatsmeow DI） ✅
  - 新增 `monitor_deps.go` (WhatsAppMonitorDeps DI 接口: 9 项注入点)
- [x] WA-B: 入站消息监控 + 路由 ✅
  - 新增 `monitor_inbound.go` (~350L): `HandleInboundMessageFull` (去重 + DM/群组策略门控 + 配对管理 + 附件解析 + 路由→会话→分发)
  - `dispatchWhatsAppInbound` + `handleWhatsAppPairing` + `BuildWhatsAppPairingReply` + `formatWhatsAppEnvelope`
- [x] WA-C: 自动回复引擎 ✅
  - `auto_reply.go`: `DeliverWhatsAppReplies` (表格转换 + 分块投递) + `ChunkReplyText` (length/newline 模式)
  - `MonitorConfig.Deps` 接入 DI 依赖
- [x] WA-D: 媒体优化管线 ✅
  - `media.go`: `MediaOptimizer` 接口 + `OptimizeWebMedia` (HEIC→JPEG / PNG 优化 / JPEG 尺寸检查)
  - `ClampImageDimensions` (4096×4096 等比缩放) + `isHEICContentType` + `replaceExt`
- [x] WA-E: 出站消息 Markdown 表格转换 ✅
  - `outbound.go`: `SendMessageWhatsApp` 接入 `markdown.ConvertMarkdownTables`
  - `ResolveWhatsAppTableMode` (默认 bullets)
- [x] WA-F: auth-store 辅助函数 + 结构化日志 ✅
  - `auth_store.go`: `log.Printf` → `slog` 结构化日志 (6 处)
  - 新增 `whatsapp_test.go` (33 个测试用例，含 -race 验证)

### A5: Slack Gateway [x] ✅ (Phase 9, 2026-02-15)

- [x] SLK-A: Socket Mode WebSocket 连接 + 事件循环 ✅
  - 集成 `slack-go/slack` v0.17.3 `socketmode` 子包（全托管 WebSocket + 自动重连）
- [x] SLK-B: HTTP 事件验证 + 签名校验 + 分发 ✅
  - HMAC-SHA256 签名验证 + url_verification challenge + event_callback 分发
- [x] SLK-C: 入站消息分发管线（6 处 TODO） ✅
  - 新增 `monitor_deps.go` (SlackMonitorDeps DI: 9 项注入点)
  - `monitor_message_prepare.go`: 11 步过滤管线 (self/bot/channel/DM/mention/user)
  - `monitor_message_dispatch.go`: agent 路由 + MsgContext + session + auto-reply
- [x] SLK-D: 事件处理器（channels/members/pins/reactions） ✅
  - 6 个频道事件 + 2 成员事件 + 2 pin 事件 + 2 反应事件 → system event
- [x] SLK-E: 监控上下文（缓存 + API 回填） ✅
  - `monitor_context.go` 重写: conversations.info/users.info API 回填 + dedup cache
- [x] SLK-F: 线程历史补全 + 回复发送 ✅
  - conversations.replies API + 历史裁剪 + 分块发送 + 反应状态 (⏳→✅/❌)
- [x] SLK-G: 斜杠命令处理 ✅
  - 命令解析 + agent 路由 + ephemeral 回复
- [x] SLK-H: Pairing Store 集成 ✅
  - 静态+动态 allowlist 合并 + DI 注入 ReadAllowFromStore/UpsertPairingRequest
- [x] SLK-I: 媒体下载 ✅
  - Bot token 授权私有文件下载
- [x] SLK-P7-A: Markdown IR 中间层剩余 ✅ (部分覆盖 via MarkdownToSlackMrkdwnChunks)
- [x] SLK-P7-B: 媒体上传 + 分块策略 ✅ (部分覆盖 via chunked reply)

### A6: Discord Gateway [x] ✅ (2026-02-15)

- [x] DIS-A: Gateway 事件绑定 + Monitor 生命周期 ✅ (2026-02-15)
- [x] DIS-B: 消息处理管线 ✅ (2026-02-15)
- [x] DIS-C: 执行审批 UI ✅ (2026-02-15)
- [x] DIS-D: 原生命令 ✅ (2026-02-15)
- [x] DIS-E: 缓存 + 辅助模块 ✅ (2026-02-15)
- [x] DIS-F: Send 层铺砌延迟项 ✅ (2026-02-15)

---

## Batch B：Config/Agent 补全（~10 项） [x] ✅ (2026-02-15)

- [x] P1-F9: Shell Env Fallback — `shellenv.go` (250L) 已完整实现 ✅
- [x] P1-B3: ChannelsConfig Extra 字段 — `types_channels.go` (107L) 含 Extra + 自定义 UnmarshalJSON/MarshalJSON ✅
- [x] P1-C1: Discord/Slack/Signal 群组解析 — `grouppolicy.go` 已含全部频道 ✅
- [x] P1-channelPreferOver: 动态查询 — TS 核心频道无 preferOver 值，Go 空 map 行为正确 ✅
- [x] P1-F14c: 6 个 TS 配置工具模块移植 ✅
  - `cache-utils.ts` → `cacheutils.go` (新建)
  - `channel-capabilities.ts` → `channel_capabilities.go` (已预存在 146L)
  - `commands.ts` → `commands.go` (新建)
  - `talk.ts` → `talk.go` (新建)
  - `merge-config.ts` → `mergeconfig.go` (新建)
  - `telegram-custom-commands.ts` → `telegramcmds.go` (新建)
- [x] P1-F14d: port-defaults.ts / logging.ts ✅
  - `port-defaults.ts` → `portdefaults.go` (新建)
  - `logging.ts` → `configlog.go` (新建)
- [x] P4-NEW1: Fallbacks `*[]string` hasOwn 语义 ✅
  - 修改 `types_agents.go` + `types_agent_defaults.go` + 4 处引用修复
- [x] P4-NEW2: ResolveHooksGmailModel ✅
  - 新增函数到 `selection.go`

---

## Batch C：Agent 引擎缺口（~8 项）✅ 完成 (2026-02-15)

- [x] P4-GA-RUN2: messaging tool 元数据传播 ✅
  - `run.go` L274-275 — 传播 `DidSendViaMessaging` + `MessagingSentTargets`
- [x] P4-GA-ANN1: subagent steer/queue 机制 ✅
  - `subagent_announce.go` — `AnnounceQueueHandler` DI + steer/queue 逻辑
- [x] P4-GA-CLI3: CLI Agent 完整 system prompt 构建 ✅
  - `cli_runner.go` — `SystemPromptBuilder` 回调 + `CliSystemPromptContext`
- [x] P4-GA-DLV1: 完整 Channel Handler 管线 ✅
  - `deliver.go` — `ChannelOutboundAdapter` + `TextChunkerFunc` DI
- [x] P4-GA-DLV3: Mirror transcript 追加 ✅
  - `deliver.go` — `appendMirrorTranscript` + `TranscriptAppender` DI
- [x] C2-P1b: isCliProvider + runCliAgent ✅ (已存在)
  - `models/selection.go:68` + `exec/cli_runner.go:68`
- [x] C2-P2b: registerAgentRunContext 全局状态 ✅
  - **[NEW]** `internal/infra/agent_events.go` (150L) + test (3 测试)
- [x] C2-P2c: buildWorkspaceSkillSnapshot ✅
  - **[NEW]** `internal/agents/skills/workspace_skills.go` (245L) + test (5 测试)

---

## Batch D：辅助/优化（~15 项）

### D1: CLI 辅助 [x] ✅ (2026-02-15)

- [x] CLI-P2-1: Plugin registry singleton ✅
  - `plugin_registry.go` (80L): `EnsurePluginRegistryLoaded` (sync.Once) + `PluginRegistryDeps` DI
- [x] CLI-P2-2: dotenv 加载 ✅
  - `dotenv.go` (85L): `LoadDotEnv` — CWD + global fallback ~/.openacosmi/.env (不覆盖已有)
- [x] CLI-P2-3: --update flag 重写 ✅
  - `argv.go`: `RewriteUpdateFlagArgv` — `--update` → `update` 子命令
- [x] CLI-P2-4: Runtime clearProgressLine 联动 ✅
  - `progress.go`: `ClearProgressLine` — 活跃进度时清除 spinner 行
- [x] CLI-P2-5: OPENACOSMI_EAGER_CHANNEL_OPTIONS ✅
  - `utils.go`: `ResolveCliChannelOptions` — 动态插件频道合并 + `GetChannelNames` accessor
- [x] CLI-P3-1: 快速路由机制 ✅
  - `route.go` (100L): `TryRouteCli` + `RegisterRoutedCommand` — 快速命令路由
- [x] CLI-P3-2: PATH bootstrap ✅ (N/A — Go 独立二进制无需 PATH 补全)

### D2: ACP 辅助 [x] ✅ (2026-02-15)

- [x] ACP-P2-1: SessionID 使用 UUID 生成 ✅
  - `session.go`: `uuid.New().String()` 替换计数器，移除 `counter` 字段
- [x] ACP-P2-2: ListSessions 支持 limit 参数 ✅
  - `translator.go`: `ReadNumber(req.Meta, "limit")` 替换硬编码 100

### D3: Phase 7 遗留 [x] ✅ (2026-02-15)

- [x] P7A-1: 链接理解 Runner + Apply ✅
  - **[NEW]** `linkparse/runner.go` (220L): `RunLinkUnderstanding` + CLI 执行 + 作用域检查
  - **[NEW]** `linkparse/apply.go` (60L): `ApplyLinkUnderstanding` — Runner + 上下文更新
- [x] P7A-2: chunkMarkdownIR ✅ (已在 `ir.go:599-629` 完成)
- [x] P7A-3: Security audit — 骨架类型定义 ✅ (完整实现延迟 Phase 10+)
  - **[NEW]** `security/audit.go` (100L): 类型定义 + `RunSecurityAudit` 占位
- [x] P7B-4: PDF 文本提取 ✅
  - `input_files.go`: `pdfcpu` 库提取 → 临时目录 + 文本合并
- [x] P7B-5: 图像双线性缩放 ✅ (RUST_CANDIDATE P2 保留)
  - `image_ops.go`: `xdraw.BiLinear.Scale` 替换 nearest-neighbor
- [x] P7B-6: 媒体本地 HTTP 服务器 ✅ (隧道延迟 Phase 10+)
  - `host.go`: `startLocalMediaServer` (sync.Once) + `http.FileServer`

### D4: Phase 7C 记忆+浏览器 [x]

- [x] P7C-1: Embedding 批处理
- [x] P7C-2: SQLite 向量扩展
- [x] P7C-3: 文件监控（fsnotify）
- [x] P7C-4: Local Embeddings → Phase 10+ Rust（stub 确认）
- [x] P7C-5: Browser HTTP 控制服务器
- [x] P7C-6: Browser 高级操作

### D5: Phase 8 遗留 [x]

- [x] P8W2-D2: MemoryFlusher.RunFlush 决策逻辑 ✅ (Phase 9, 2026-02-16)
- [x] P8W2-D3: HandleInlineActions 技能命令解析 ✅ (Phase 9, 2026-02-16)
- [x] P8W2-D4: ApplyInlineDirectiveOverrides 指令持久化 ✅ (Phase 9, 2026-02-16)
- [x] P8W2-D5: SessionEntry 字段完善（统一到 gateway.SessionEntry）✅ (Phase 9, 2026-02-16)
