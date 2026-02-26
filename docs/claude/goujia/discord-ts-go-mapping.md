> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# Discord 模块：TS → Go 文件映射表

## 概述
- **TS 源文件**: 49 个（非 test）
- **Go 源文件**: 43 个（非 test）
- **已映射配对**: 39 对
- **TS 未映射**: 10 个
- **Go 独有**: 4 个

## 已映射文件对

| # | TS 源文件 | Go 对应文件 | 审计状态 |
|---|---|---|---|
| 1 | `src/discord/accounts.ts` | `accounts.go` | 已审计 |
| 2 | `src/discord/api.ts` | `api.go` | 已审计 |
| 3 | `src/discord/audit.ts` | `audit.go` | 待审计 |
| 4 | `src/discord/chunk.ts` | `chunk.go` | 待审计 |
| 5 | `src/discord/directory-live.ts` | `directory_live.go` | 待审计 |
| 6 | `src/discord/gateway-logging.ts` | `gateway_logging.go` | 待审计 |
| 7 | `src/discord/pluralkit.ts` | `pluralkit.go` | 待审计 |
| 8 | `src/discord/probe.ts` | `probe.go` | 待审计 |
| 9 | `src/discord/resolve-channels.ts` | `resolve_channels.go` | 待审计 |
| 10 | `src/discord/resolve-users.ts` | `resolve_users.go` | 待审计 |
| 11 | `src/discord/send.channels.ts` | `send_channels.go` | 待审计 |
| 12 | `src/discord/send.emojis-stickers.ts` | `send_emojis_stickers.go` | 待审计 |
| 13 | `src/discord/send.guild.ts` | `send_guild.go` | 待审计 |
| 14 | `src/discord/send.messages.ts` | `send_messages.go` | 待审计 |
| 15 | `src/discord/send.permissions.ts` | `send_permissions.go` | 待审计 |
| 16 | `src/discord/send.reactions.ts` | `send_reactions.go` | 待审计 |
| 17 | `src/discord/send.shared.ts` | `send_shared.go` | 待审计 |
| 18 | `src/discord/send.types.ts` | `send_types.go` | 待审计 |
| 19 | `src/discord/targets.ts` | `targets.go` | 待审计 |
| 20 | `src/discord/token.ts` | `token.go` | 待审计 |
| 21 | `monitor/allow-list.ts` | `monitor_allow_list.go` | 待审计 |
| 22 | `monitor/exec-approvals.ts` | `monitor_exec_approvals.go` | 待审计 |
| 23 | `monitor/format.ts` | `monitor_format.go` | 待审计 |
| 24 | `monitor/gateway-registry.ts` | `monitor_gateway_registry.go` | 待审计 |
| 25 | `monitor/listeners.ts` | `monitor_listeners.go` | 待审计 |
| 26 | `monitor/message-handler.ts` | `monitor_message_dispatch.go` | 待审计 |
| 27 | `monitor/message-handler.preflight.ts` | `monitor_message_preflight.go` | 待审计 |
| 28 | `monitor/message-handler.preflight.types.ts` | `monitor_message_preflight_types.go` | 待审计 |
| 29 | `monitor/message-handler.process.ts` | `monitor_message_process.go` | 待审计 |
| 30 | `monitor/message-utils.ts` | `monitor_message_utils.go` | 待审计 |
| 31 | `monitor/native-command.ts` | `monitor_native_command.go` | 待审计 |
| 32 | `monitor/presence-cache.ts` | `monitor_presence_cache.go` | 待审计 |
| 33 | `monitor/provider.ts` | `monitor_provider.go` | 待审计 |
| 34 | `monitor/reply-context.ts` | `monitor_reply_context.go` | 待审计 |
| 35 | `monitor/reply-delivery.ts` | `monitor_reply_delivery.go` | 待审计 |
| 36 | `monitor/sender-identity.ts` | `monitor_sender_identity.go` | 待审计 |
| 37 | `monitor/system-events.ts` | `monitor_system_events.go` | 待审计 |
| 38 | `monitor/threading.ts` | `monitor_threading.go` | 待审计 |
| 39 | `monitor/typing.ts` | `monitor_typing.go` | 待审计 |

## 未映射 TS 文件

| # | TS 文件 | 说明 | 评估 |
|---|---|---|---|
| A | `src/discord/index.ts` | Barrel export 入口 | Go 无需等价 |
| B | `src/discord/monitor.ts` | 顶层 monitor 编排 | 可能已拆入 monitor_deps.go + monitor_provider.go |
| C | `src/discord/monitor.gateway.ts` | Gateway 连接管理 | 可能已拆入 monitor_gateway_registry.go |
| D | `src/discord/send.ts` | Send 主编排器 | 需确认逻辑去向 |
| E | `src/discord/send.outbound.ts` | 出站消息逻辑 | 可能合入 send_media.go |
| F | `extensions/discord/index.ts` | 插件入口 | 待评估 |
| G | `extensions/discord/src/channel.ts` | 插件 channel 定义 | 待评估 |
| H | `extensions/discord/src/runtime.ts` | 插件运行时 | 待评估 |
| I | `actions/handle-action.ts` | Action handler | 待评估 |
| J | `actions/handle-action.guild-admin.ts` | Guild admin action | 待评估 |

## Go 独有文件

| 文件 | 说明 |
|---|---|
| `account_id.go` | 账户 ID 辅助函数（从 TS routing/session-key.js 提取） |
| `monitor_deps.go` | 依赖注入接口文件 |
| `send_media.go` | 媒体发送（可能从 send.outbound.ts 提取） |
| `webhook_verify.go` | Webhook 验签 |
