> 归档前请完成复核审计，具体审计方法查看复核审计技能。

# Telegram 模块：TS → Go 文件映射表

## 概述
- **TS 源文件**: 43 个（非 test）
- **Go 源文件**: 36 个（非 test）
- **已映射配对**: 35 对
- **TS 未映射**: 8 个
- **Go 独有**: 2 个

## 已映射文件对

| # | TS 源文件 | Go 对应文件 | 审计状态 |
|---|---|---|---|
| 1 | `accounts.ts` | `accounts.go` | 待审计 |
| 2 | `api-logging.ts` | `api_logging.go` | 待审计 |
| 3 | `audit.ts` | `audit.go` | 待审计 |
| 4 | `bot.ts` | `bot.go` | 待审计 |
| 5 | `bot-access.ts` | `bot_access.go` | 待审计 |
| 6 | `bot-handlers.ts` | `bot_handlers.go` | 待审计 |
| 7 | `bot-message.ts` | `bot_message.go` | 待审计 |
| 8 | `bot-message-context.ts` | `bot_message_context.go` | 待审计 |
| 9 | `bot-message-dispatch.ts` | `bot_message_dispatch.go` | 待审计 |
| 10 | `bot-native-commands.ts` | `bot_native_commands.go` | 待审计 |
| 11 | `bot-updates.ts` | `bot_updates.go` | 待审计 |
| 12 | `caption.ts` | `caption.go` | 待审计 |
| 13 | `download.ts` | `download.go` | 待审计 |
| 14 | `draft-chunking.ts` | `draft_chunking.go` | 待审计 |
| 15 | `draft-stream.ts` | `draft_stream.go` | 待审计 |
| 16 | `format.ts` | `format.go` | 待审计 |
| 17 | `group-migration.ts` | `group_migration.go` | 待审计 |
| 18 | `inline-buttons.ts` | `inline_buttons.go` | 待审计 |
| 19 | `model-buttons.ts` | `model_buttons.go` | 待审计 |
| 20 | `monitor.ts` | `monitor.go` | 待审计 |
| 21 | `network-config.ts` + `network-errors.ts` | `network.go`（合并） | 待审计 |
| 22 | `probe.ts` | `probe.go` | 待审计 |
| 23 | `reaction-level.ts` | `reaction_level.go` | 待审计 |
| 24 | `send.ts` | `send.go` | 待审计 |
| 25 | `sent-message-cache.ts` | `sent_message_cache.go` | 待审计 |
| 26 | `sticker-cache.ts` | `sticker_cache.go` | 待审计 |
| 27 | `targets.ts` | `targets.go` | 待审计 |
| 28 | `token.ts` | `token.go` | 待审计 |
| 29 | `update-offset-store.ts` | `update_offset_store.go` | 待审计 |
| 30 | `voice.ts` | `voice.go` | 待审计 |
| 31 | `webhook.ts` | `webhook.go` | 待审计 |
| 32 | `bot/delivery.ts` | `bot_delivery.go` | 待审计 |
| 33 | `bot/helpers.ts` | `bot_helpers.go` | 待审计 |
| 34 | `bot/types.ts` | `bot_types.go` | 待审计 |

## 未映射 TS 文件

| # | TS 文件 | 说明 | 评估 |
|---|---|---|---|
| A | `index.ts` | Barrel export 入口 | Go 无需等价 |
| B | `allowed-updates.ts` | 允许的 update 类型定义 | 可能合入 bot.go 或 bot_updates.go |
| C | `fetch.ts` | HTTP fetch 封装 | 可能对应 http_client.go |
| D | `proxy.ts` | 代理配置 | 可能合入 http_client.go |
| E | `webhook-set.ts` | Webhook 设置 | 可能合入 webhook.go |
| F | `extensions/telegram/index.ts` | 插件入口 | 待评估 |
| G | `extensions/telegram/src/channel.ts` | 插件 channel | 待评估 |
| H | `extensions/telegram/src/runtime.ts` | 插件运行时 | 待评估 |

## Go 独有文件

| 文件 | 说明 |
|---|---|
| `http_client.go` | HTTP 客户端封装（可能对应 fetch.ts + proxy.ts） |
| `monitor_deps.go` | 依赖注入接口文件 |
