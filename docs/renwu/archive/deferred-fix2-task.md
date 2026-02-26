# 延迟待办修复任务清单 Phase 2 (deferred-fix2)

> 来源：2026-02-16 扩大审计发现（缺口 A/B/E）
> 上下文：[deferred-fix2-bootstrap.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-fix2-bootstrap.md)
> 审计来源：[deferred-items.md](file:///Users/fushihua/Desktop/Claude-Acosmi/docs/renwu/deferred-items.md) 末尾 3 章节
> 最后更新：2026-02-16

---

## Batch DF2-A：Telegram Phase 6 集成（P1，高优先级） ✅

> 19 处 TODO — 已全部完成（SOCKS5 可选项保留）

### DF2-A1: Bot 消息处理管线 ✅

- [x] `bot_message_dispatch.go` — 接入 agent dispatch pipeline
- [x] `bot_message.go` — 实现 `dispatchTelegramMessage`
- [x] `bot_handlers.go` — 完整 bot handler 集成
- [x] `bot_message_context.go` — 完整上下文构建（routing/session/agent 模块）
- [x] `bot_native_commands.go` — reset/model 命令实现（接入 session 管理器）
- [x] `monitor_deps.go` — [NEW] TelegramMonitorDeps DI 接口

### DF2-A2: Monitor + Webhook 集成 ✅

- [x] `monitor.go` — update 分发到 bot handler
- [x] `webhook.go` — bot handler 集成 + secret token 验证

### DF2-A3: 媒体发送 + 投递 ✅

- [x] `send.go` — 媒体发送（download → MIME → multipart upload）
- [x] `bot_delivery.go` — 媒体上传集成

### DF2-A4: Draft Stream + 辅助 ✅

- [x] `draft_stream.go` — sendMessage + editMessageText 降级策略
- [x] `sticker_cache.go` — `DescribeStickerImage`（DI 回调 + 缓存）
- [ ] `http_client.go` — SOCKS5 代理支持（可选，保留 TODO）

### DF2-A5: 验证 ✅

- [x] `go build ./...` + `go vet ./...`
- [x] `go test -race ./internal/channels/telegram/...`
- [x] 更新 `deferred-items.md` 缺口 A 状态为 ✅

---

## Batch DF2-B：Bridge Actions 实现（P2，中优先级）✅ 已完成

> 12 处桩 — 影响 messaging tool 的频道动作执行 → **已全部实现**

### DF2-B1: Telegram Bridge Actions

- [x] `bridge/telegram_actions.go` — 实现 messaging/reaction/poll/pin/admin/sticker 6 个动作组（16 case）
- [x] 对接 `internal/channels/telegram/send.go` API（DI 接口 `TelegramActionDeps`）

### DF2-B2: Slack Bridge Actions

- [x] `bridge/slack_actions.go` — 实现 messaging/reactions/pin/memberInfo/emojiList（11 case）
- [x] 对接 `internal/channels/slack/client.go` API（DI 接口 `SlackActionDeps`）

### DF2-B3: Discord Bridge Actions

- [x] `bridge/discord_actions.go` + 4 子文件 — 实现 messaging/guild/moderation/presence（37 case）
- [x] 对接 `internal/channels/discord/send_*.go` API（DI 接口 `DiscordActionDeps`）

### DF2-B4: 验证

- [x] `go test -race ./internal/channels/bridge/...` — 12 tests PASS
- [x] 更新 `deferred-items.md` 缺口 B 状态为 ✅

---

## Batch DF2-C：内部模块残留骨架（P2-P3，中优先级） ✅

> 7 处骨架/桩 — 已全部完成

### DF2-C1: Memory Manager 搜索/同步

- [x] `internal/memory/manager.go` — `Search` 接入向量搜索 + 关键词搜索
- [x] `internal/memory/manager.go` — `Sync` 接入文件扫描 + embedding
- [x] 已有 `memory` 包测试通过（`go test -race`）

### DF2-C2: Cron Agent Runner

- [x] `internal/cron/timer.go` — 错误信息更新 + DI 注释
- [x] `AgentExecutor` DI 已在 `isolated_agent.go` 中实现

### DF2-C3: Outbound Deliver 完善

- [x] `internal/outbound/deliver.go` — ChannelOutboundAdapter 插件加载
- [x] 文本分块集成（TextChunker DI）
- [x] Signal 格式化 + 频道媒体限制 + AbortSignal 检查

### DF2-C4: FollowupRunner 类型安全

- [x] `followup_runner.go` — `Config`→`*types.OpenAcosmiConfig`、`SkillsSnapshot`→`*session.SessionSkillSnapshot`、`ExecOverrides`→`*ExecOverrides`

### DF2-C5: 辅助模块（低优先级）

- [x] `memory/qmd_manager.go` — qmd 子进程调用实现
- [x] `browser/extension_relay.go` — CDP 双向 WebSocket 转发
- [x] `discord/monitor_native_command.go` — session 重置 / 模型切换 DI 接入

### DF2-C6: 验证

- [x] `go build ./...` + `go vet ./...` — 通过
- [x] `go test -race ./internal/memory/... ./internal/outbound/...` — 通过
- [x] 文档已更新

---

## 文档更新（每批次完成后）

- [ ] 更新 `deferred-items.md` 对应章节状态
- [ ] 更新 `refactor-plan-full.md` 相关章节
