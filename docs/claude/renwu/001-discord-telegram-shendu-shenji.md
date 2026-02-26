> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# 任务 001：Discord & Telegram 模块深度审计

## 任务概述
对 TS→Go 重构的 Discord 和 Telegram 模块进行深度审计，验证业务逻辑 100% 无损继承。

---

## Discord 模块审计清单

### 核心模块（已映射 39 对）
- [x] `api.ts` ↔ `api.go` — 已审计，发现 3 个差异（见延迟项）
- [x] `accounts.ts` ↔ `accounts.go` — 已审计
- [ ] `audit.ts` ↔ `audit.go`
- [ ] `chunk.ts` ↔ `chunk.go`
- [ ] `directory-live.ts` ↔ `directory_live.go`
- [ ] `gateway-logging.ts` ↔ `gateway_logging.go`
- [ ] `pluralkit.ts` ↔ `pluralkit.go`
- [ ] `probe.ts` ↔ `probe.go`
- [ ] `resolve-channels.ts` ↔ `resolve_channels.go`
- [ ] `resolve-users.ts` ↔ `resolve_users.go`
- [ ] `send.channels.ts` ↔ `send_channels.go`
- [ ] `send.emojis-stickers.ts` ↔ `send_emojis_stickers.go`
- [ ] `send.guild.ts` ↔ `send_guild.go`
- [ ] `send.messages.ts` ↔ `send_messages.go`
- [ ] `send.permissions.ts` ↔ `send_permissions.go`
- [ ] `send.reactions.ts` ↔ `send_reactions.go`
- [ ] `send.shared.ts` ↔ `send_shared.go`
- [ ] `send.types.ts` ↔ `send_types.go`
- [ ] `targets.ts` ↔ `targets.go`
- [ ] `token.ts` ↔ `token.go`
- [ ] `monitor/allow-list.ts` ↔ `monitor_allow_list.go`
- [ ] `monitor/exec-approvals.ts` ↔ `monitor_exec_approvals.go`
- [ ] `monitor/format.ts` ↔ `monitor_format.go`
- [ ] `monitor/gateway-registry.ts` ↔ `monitor_gateway_registry.go`
- [ ] `monitor/listeners.ts` ↔ `monitor_listeners.go`
- [ ] `monitor/message-handler.ts` ↔ `monitor_message_dispatch.go`
- [ ] `monitor/message-handler.preflight.ts` ↔ `monitor_message_preflight.go`
- [ ] `monitor/message-handler.preflight.types.ts` ↔ `monitor_message_preflight_types.go`
- [ ] `monitor/message-handler.process.ts` ↔ `monitor_message_process.go`
- [ ] `monitor/message-utils.ts` ↔ `monitor_message_utils.go`
- [ ] `monitor/native-command.ts` ↔ `monitor_native_command.go`
- [ ] `monitor/presence-cache.ts` ↔ `monitor_presence_cache.go`
- [ ] `monitor/provider.ts` ↔ `monitor_provider.go`
- [ ] `monitor/reply-context.ts` ↔ `monitor_reply_context.go`
- [ ] `monitor/reply-delivery.ts` ↔ `monitor_reply_delivery.go`
- [ ] `monitor/sender-identity.ts` ↔ `monitor_sender_identity.go`
- [ ] `monitor/system-events.ts` ↔ `monitor_system_events.go`
- [ ] `monitor/threading.ts` ↔ `monitor_threading.go`
- [ ] `monitor/typing.ts` ↔ `monitor_typing.go`

### 未映射 TS 文件（待评估）
- [ ] `index.ts` — Barrel export（可能无需 Go 对应）
- [ ] `monitor.ts` — 顶层编排
- [ ] `monitor.gateway.ts` — Gateway 连接管理
- [ ] `send.ts` — Send 主编排器
- [ ] `send.outbound.ts` — 出站消息逻辑
- [ ] `extensions/discord/index.ts` — 插件入口
- [ ] `extensions/discord/src/channel.ts` — 插件 channel
- [ ] `extensions/discord/src/runtime.ts` — 插件运行时
- [ ] `actions/discord/handle-action.ts` — Action handler
- [ ] `actions/discord/handle-action.guild-admin.ts` — Guild admin

---

## Telegram 模块审计清单

### 核心模块（已映射 35 对）
- [ ] `accounts.ts` ↔ `accounts.go`
- [ ] `api-logging.ts` ↔ `api_logging.go`
- [ ] `audit.ts` ↔ `audit.go`
- [ ] `bot.ts` ↔ `bot.go`
- [ ] `bot-access.ts` ↔ `bot_access.go`
- [ ] `bot-handlers.ts` ↔ `bot_handlers.go`
- [ ] `bot-message.ts` ↔ `bot_message.go`
- [ ] `bot-message-context.ts` ↔ `bot_message_context.go`
- [ ] `bot-message-dispatch.ts` ↔ `bot_message_dispatch.go`
- [ ] `bot-native-commands.ts` ↔ `bot_native_commands.go`
- [ ] `bot-updates.ts` ↔ `bot_updates.go`
- [ ] `caption.ts` ↔ `caption.go`
- [ ] `download.ts` ↔ `download.go`
- [ ] `draft-chunking.ts` ↔ `draft_chunking.go`
- [ ] `draft-stream.ts` ↔ `draft_stream.go`
- [ ] `format.ts` ↔ `format.go`
- [ ] `group-migration.ts` ↔ `group_migration.go`
- [ ] `inline-buttons.ts` ↔ `inline_buttons.go`
- [ ] `model-buttons.ts` ↔ `model_buttons.go`
- [ ] `monitor.ts` ↔ `monitor.go`
- [ ] `network-config.ts` + `network-errors.ts` ↔ `network.go`
- [ ] `probe.ts` ↔ `probe.go`
- [ ] `reaction-level.ts` ↔ `reaction_level.go`
- [ ] `send.ts` ↔ `send.go`
- [ ] `sent-message-cache.ts` ↔ `sent_message_cache.go`
- [ ] `sticker-cache.ts` ↔ `sticker_cache.go`
- [ ] `targets.ts` ↔ `targets.go`
- [ ] `token.ts` ↔ `token.go`
- [ ] `update-offset-store.ts` ↔ `update_offset_store.go`
- [ ] `voice.ts` ↔ `voice.go`
- [ ] `webhook.ts` ↔ `webhook.go`
- [ ] `bot/delivery.ts` ↔ `bot_delivery.go`
- [ ] `bot/helpers.ts` ↔ `bot_helpers.go`
- [ ] `bot/types.ts` ↔ `bot_types.go`

### 未映射 TS 文件（待评估）
- [ ] `index.ts` — Barrel export
- [ ] `allowed-updates.ts` — 允许的 update 类型
- [ ] `fetch.ts` — HTTP fetch 封装
- [ ] `proxy.ts` — 代理配置
- [ ] `webhook-set.ts` — Webhook 设置
- [ ] `extensions/telegram/index.ts` — 插件入口
- [ ] `extensions/telegram/src/channel.ts` — 插件 channel
- [ ] `extensions/telegram/src/runtime.ts` — 插件运行时
