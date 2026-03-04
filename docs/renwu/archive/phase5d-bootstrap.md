# Phase 5D Bootstrap — 频道 SDK 移植上下文

> 本文档为 5D.2-5D.9 各频道 SDK 移植窗口提供统一的启动上下文。

---

## 一、项目规则

- **技能规范**: `skills/acosmi-refactor/SKILL.md`
- **编码规范**: `skills/acosmi-refactor/references/coding-standards.md`
- **深度审计**: `docs/renwu/phase5-channels-deep-audit.md` — 四层架构 + 6 项隐藏依赖
- **语言**: 中文交互/文档，英文代码标识符
- **验证**: 每批文件写完后 `go build ./...`

---

## 二、Go channels 包现状 (31 文件 + routing 1 文件)

### `pkg/contracts/`（3 文件）

| 文件 | 内容 |
| ---- | ---- |
| `channel_types.go` | 15+ 核心类型（ChannelCapabilities, ChannelMeta 等） |
| `channel_adapters.go` | 14 个适配器接口（Outbound, Config, Gateway 等） |
| `channel_plugin.go` | ChannelPlugin 契约（23 个字段槽位） |

### `internal/channels/`（23 文件）

**核心基础** (5B): `registry.go`, `catalog.go`, `dock.go`, `channel_match.go`, `channels.go`

**共享工具** (5B): `group_mentions.go`, `mention_gating.go`, `ack_reactions.go`, `chat_type.go`, `conversation_label.go`, `logging.go`, `config_helpers.go`, `media_limits.go`, `config_schema.go`, `config_writes.go`, `message_actions.go`

**适配器辅助** (5D.1): `normalize.go`, `outbound.go`, `actions.go`, `status_issues.go`, `plugin_helpers.go`, `channel_status.go`, `pairing.go`, `setup_helpers.go`, `onboarding.go`, `directory_config.go`

### `internal/channels/bridge/`（5 文件）

工具桥接: `common.go`, `discord_actions.go`, `slack_actions.go`, `telegram_actions.go`, `whatsapp_actions.go`

### `internal/routing/`（1 文件，Phase 6 关键前置）

| 文件 | 内容 |
| ---- | ---- |
| `session_key.go` | 18 个 session key 路由函数（NormalizeAgentID, BuildAgentMainSessionKey, BuildAgentPeerSessionKey 等），对齐 TS `session-key.ts` + `session-key-utils.ts` |

---

## 三、关键频道常量

```go
// internal/channels/channels.go
const (
    ChannelDiscord   ChannelID = "discord"
    ChannelSlack     ChannelID = "slack"
    ChannelTelegram  ChannelID = "telegram"
    ChannelWhatsApp  ChannelID = "whatsapp"
    ChannelSignal    ChannelID = "signal"
    ChannelIMessage  ChannelID = "imessage"
    ChannelGoogleChat ChannelID = "googlechat"
)
```

---

## 四、各频道 SDK 概况

| 频道 | TS 文件数 | TS 行数 | Go 目标目录 | 核心文件 |
| ---- | ---- | ---- | ---- | ---- |
| WhatsApp | 1+43 | 80+5696 | `internal/channels/whatsapp/` + `internal/channels/web/` | normalize, gateway, heartbeat |
| Signal | 14 | 2567 | `internal/channels/signal/` | accounts, send, reactions |
| iMessage | 12 | 1697 | `internal/channels/imessage/` | targets, send |
| Telegram | 40 | 7929 | `internal/channels/telegram/` | accounts, format, send, inline-buttons |
| Slack | 43 | 5809 | `internal/channels/slack/` | accounts, targets, send, socket-mode |
| Discord | 44 | 8733 | `internal/channels/discord/` | accounts, targets, send, gateway |

---

## 五、新窗口启动模板

在新窗口中粘贴以下内容（替换 `[频道名]`）：

```
请阅读以下文件获取项目上下文：
1. `skills/acosmi-refactor/SKILL.md`
2. `docs/renwu/phase5d-bootstrap.md`
3. `docs/renwu/phase5-channels-deep-audit.md`
4. `docs/renwu/phase5d-task.md` — 任务清单，完成后请更新对应条目

当前任务：Phase 5D — 移植 [频道名] SDK 到 Go

请先列出 TS 文件清单，按依赖顺序规划移植，然后逐文件移植。
每次只创建 1-2 个文件，写完编译验证。
完成后更新 docs/renwu/phase5d-task.md 中对应子项为 [x]。
如有 TODO 桩函数（依赖 Gateway 层），记录到 phase5d-task.md 底部"待对接桩汇总"章节。
```
