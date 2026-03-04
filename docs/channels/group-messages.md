---
summary: "WhatsApp 群消息处理的行为与配置（mentionPatterns 跨平台共享）"
read_when:
  - 修改群消息规则或提及配置
title: "群消息（Group Messages）"
---

# 群消息（WhatsApp 频道）

> [!IMPORTANT]
> **架构状态**：WhatsApp 群消息处理由 **Go Gateway**（`backend/internal/channels/whatsapp/`）实现。
> 提及门控由 `backend/internal/channels/group_mentions.go` 和 `mention_gating.go` 处理。

目标：让 OpenAcosmi 加入 WhatsApp 群组，仅在被提及（ping）时激活，并将群聊线程与个人 DM 会话隔离。

注意：`agents.list[].groupChat.mentionPatterns` 现在也被 Telegram/Discord/Slack/iMessage/飞书 使用；本文档专注于 WhatsApp 特有的行为。多 agent 场景中，按 agent 设置 `agents.list[].groupChat.mentionPatterns`（或使用 `messages.groupChat.mentionPatterns` 作为全局回退）。

## 已实现功能

- **激活模式**：`mention`（默认）或 `always`。`mention` 模式需要 ping（真实 WhatsApp @提及通过 `mentionedJids`、正则匹配、或正文中包含 bot 的 E.164 号码）。`always` 模式在每条消息时唤醒 agent，但仅在能提供有意义价值时才回复；否则返回静默令牌 `NO_REPLY`。默认值在配置 `channels.whatsapp.groups` 中设置，可通过 `/activation` 按群覆盖。当设置了 `channels.whatsapp.groups` 时，它同时充当群白名单（包含 `"*"` 以允许所有群）。
- **群策略**：`channels.whatsapp.groupPolicy` 控制是否接受群消息（`open|disabled|allowlist`）。`allowlist` 使用 `channels.whatsapp.groupAllowFrom`（回退：`channels.whatsapp.allowFrom`）。默认 `allowlist`（在添加发送者前被阻止）。
- **按群会话**：会话键格式为 `agent:<agentId>:whatsapp:group:<jid>`，因此 `/verbose on` 或 `/think high` 等命令（作为独立消息发送）仅作用于该群；个人 DM 状态不受影响。心跳检测跳过群线程。
- **上下文注入**：**待处理**群消息（默认 50 条）——即未触发运行的消息——以 `[Chat messages since your last reply - for context]` 前缀注入，触发行以 `[Current message - respond to this]` 标注。已在会话中的消息不会重复注入。
- **发送者标识**：每个群批次以 `[from: 发送者名称 (+E164)]` 结尾，让模型知道谁在说话。
- **阅后即焚/一次性查看**：在提取文本/提及前解包这些消息，因此其中的 ping 仍然触发。
- **群系统提示**：在群会话的第一轮（以及每次 `/activation` 更改模式时），在系统提示中注入简短介绍，如 `You are replying inside the WhatsApp group "<subject>". Group members: Alice (+44...), Bob (+43...), … Activation: trigger-only …`

## 配置示例（WhatsApp）

在 `~/.openacosmi/openacosmi.json` 中添加 `groupChat` 块，以便在 WhatsApp 去除正文中可视 `@` 时显示名称 ping 仍然有效：

```json5
{
  channels: {
    whatsapp: {
      groups: {
        "*": { requireMention: true },
      },
    },
  },
  agents: {
    list: [
      {
        id: "main",
        groupChat: {
          historyLimit: 50,
          mentionPatterns: ["@?openacosmi", "\\+?15555550123"],
        },
      },
    ],
  },
}
```

备注：

- 正则不区分大小写；覆盖了 `@openacosmi` 显示名称 ping 和带或不带 `+`/空格的原始号码。
- WhatsApp 在用户点击联系人时仍会通过 `mentionedJids` 发送标准提及，因此号码回退很少需要但是有用的安全网。

### 激活命令（仅所有者）

在群聊中使用命令：

- `/activation mention`
- `/activation always`

仅所有者号码（来自 `channels.whatsapp.allowFrom`，或未设置时使用 bot 自身的 E.164）可以更改此设置。在群中发送 `/status` 作为独立消息以查看当前激活模式。

## 使用方法

1. 将你的 WhatsApp 账户（运行 OpenAcosmi 的那个）添加到群组。
2. 发送 `@openacosmi …`（或包含号码）。仅白名单发送者可触发，除非设置 `groupPolicy: "open"`。
3. Agent 提示将包含近期群组上下文和尾部的 `[from: …]` 标记，以便其定位正确的发言者。
4. 会话级指令（`/verbose on`、`/think high`、`/new` 或 `/reset`、`/compact`）仅应用于该群的会话；作为独立消息发送以使其注册。你的个人 DM 会话保持独立。

## 测试 / 验证

- 手动冒烟测试：
  - 在群中发送 `@openacosmi` ping 并确认回复中引用了发送者名称。
  - 再发送一个 ping 并验证历史块已包含然后在下一轮清除。
- 检查 Gateway 日志（使用 `--verbose` 运行）查看 `inbound web message` 条目，显示 `from: <groupJid>` 和 `[from: …]` 后缀。

## 已知注意事项

- 心跳检测有意跳过群组以避免嘈杂广播。
- 回显抑制使用组合批次字符串；如果你发送两次相同文本且不含提及，仅第一次会获得回复。
- 会话存储条目将以 `agent:<agentId>:whatsapp:group:<jid>` 出现在会话存储中（默认 `~/.openacosmi/agents/<agentId>/sessions/sessions.json`）；缺少条目仅意味着该群尚未触发运行。
- 群中的打字指示器遵循 `agents.defaults.typingMode`（默认：未被提及时为 `message`）。
