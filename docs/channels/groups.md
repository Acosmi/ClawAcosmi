---
summary: "跨平台群聊行为（WhatsApp/Telegram/Discord/Slack/Signal/iMessage/Teams/飞书）"
read_when:
  - 修改群聊行为或提及门控
title: "群组（Groups）"
---

# 群组

> [!IMPORTANT]
> **架构状态**：群组消息处理由 **Go Gateway**（`backend/internal/channels/group_mentions.go`、`mention_gating.go`）实现。
> 群策略和白名单在 Go Gateway 配置中定义。

OpenAcosmi 在所有平台上统一处理群聊：WhatsApp、Telegram、Discord、Slack、Signal、iMessage、Microsoft Teams、飞书。

## 入门简介（2 分钟）

OpenAcosmi "活"在你自己的消息账户上。没有单独的 WhatsApp bot 用户。
只要**你**在某个群里，OpenAcosmi 就能看到该群并在其中回复。

默认行为：

- 群组受限（`groupPolicy: "allowlist"`）。
- 回复需要 @提及，除非你显式禁用提及门控。

概括：白名单中的发送者可以通过 @提及 来触发 OpenAcosmi。

> 要点速览
>
> - **DM 访问** 由 `*.allowFrom` 控制。
> - **群组访问** 由 `*.groupPolicy` + 白名单（`*.groups`、`*.groupAllowFrom`）控制。
> - **回复触发** 由提及门控（`requireMention`、`/activation`）控制。

快速流程（群消息处理过程）：

```
groupPolicy? disabled -> 丢弃
groupPolicy? allowlist -> 群组在白名单中? 否 -> 丢弃
requireMention? yes -> 被提及? 否 -> 仅存储为上下文
否则 -> 回复
```

如果你想要...

| 目标 | 配置方式 |
|------|---------|
| 允许所有群组但仅在 @提及时回复 | `groups: { "*": { requireMention: true } }` |
| 禁用所有群组回复 | `groupPolicy: "disabled"` |
| 仅特定群组 | `groups: { "<group-id>": { ... } }`（无 `"*"` 键） |
| 仅你自己可在群中触发 | `groupPolicy: "allowlist"`, `groupAllowFrom: ["+1555..."]` |

## 会话键

- 群组会话使用 `agent:<agentId>:<channel>:group:<id>` 会话键（房间/频道使用 `agent:<agentId>:<channel>:channel:<id>`）。
- Telegram 论坛主题在群组 id 后追加 `:topic:<threadId>`，使每个主题拥有独立会话。
- 私聊使用主会话（或按发送者隔离，如已配置）。
- 心跳检测跳过群组会话。

## 模式：个人 DM + 公共群组（单 agent）

可行 — 如果你的"个人"流量是 **DM** 而"公共"流量是**群组**，此模式效果良好。

原因：在单 agent 模式下，DM 通常落在**主**会话键（`agent:main:main`），而群组始终使用**非主**会话键（`agent:main:<channel>:group:<id>`）。启用沙箱模式 `mode: "non-main"` 后，群组会话在 Docker 中运行，而主 DM 会话留在宿主机。

两种执行姿态：

- **DM**：完整工具（宿主机）
- **群组**：沙箱 + 受限工具（Docker）

> 如需完全隔离的工作区/角色（"个人"和"公共"绝不混合），使用第二个 agent + bindings。参见 [Multi-Agent Routing](/concepts/multi-agent)。

示例（DM 在宿主机，群组沙箱化 + 仅消息工具）：

```json5
{
  agents: {
    defaults: {
      sandbox: {
        mode: "non-main",
        scope: "session",
        workspaceAccess: "none",
      },
    },
  },
  tools: {
    sandbox: {
      tools: {
        allow: ["group:messaging", "group:sessions"],
        deny: ["group:runtime", "group:fs", "group:ui", "nodes", "cron", "gateway"],
      },
    },
  },
}
```

想要"群组仅能看到文件夹 X"而非"无宿主机访问"？保持 `workspaceAccess: "none"` 并仅挂载白名单路径到沙箱：

```json5
{
  agents: {
    defaults: {
      sandbox: {
        mode: "non-main",
        scope: "session",
        workspaceAccess: "none",
        docker: {
          binds: [
            "~/FriendsShared:/data:ro",
          ],
        },
      },
    },
  },
}
```

相关文档：

- 配置键和默认值：[Gateway 配置](/gateway/configuration#agentsdefaultssandbox)
- 调试工具被阻止的原因：[Sandbox vs Tool Policy vs Elevated](/gateway/sandbox-vs-tool-policy-vs-elevated)
- 绑定挂载详情：[沙箱](/gateway/sandboxing#custom-bind-mounts)

## 显示标签

- UI 标签使用 `displayName`（如可用），格式为 `<channel>:<token>`。
- `#room` 保留给房间/频道；群聊使用 `g-<slug>`（小写，空格 → `-`，保留 `#@+._-`）。

## 群策略

控制每个频道的群组/房间消息处理方式：

```json5
{
  channels: {
    whatsapp: {
      groupPolicy: "disabled", // "open" | "disabled" | "allowlist"
      groupAllowFrom: ["+15551234567"],
    },
    telegram: {
      groupPolicy: "disabled",
      groupAllowFrom: ["123456789", "@username"],
    },
    signal: {
      groupPolicy: "disabled",
      groupAllowFrom: ["+15551234567"],
    },
    imessage: {
      groupPolicy: "disabled",
      groupAllowFrom: ["chat_id:123"],
    },
    msteams: {
      groupPolicy: "disabled",
      groupAllowFrom: ["user@org.com"],
    },
    discord: {
      groupPolicy: "allowlist",
      guilds: {
        GUILD_ID: { channels: { help: { allow: true } } },
      },
    },
    slack: {
      groupPolicy: "allowlist",
      channels: { "#general": { allow: true } },
    },
    matrix: {
      groupPolicy: "allowlist",
      groupAllowFrom: ["@owner:example.org"],
      groups: {
        "!roomId:example.org": { allow: true },
        "#alias:example.org": { allow: true },
      },
    },
    feishu: {
      groupPolicy: "open", // 飞书默认 open
    },
  },
}
```

| 策略 | 行为 |
|------|------|
| `"open"` | 群组绕过白名单；提及门控仍然生效。 |
| `"disabled"` | 完全阻止所有群消息。 |
| `"allowlist"` | 仅允许匹配白名单的群组/房间。 |

备注：

- `groupPolicy` 与提及门控（需要 @提及）是分开的。
- WhatsApp/Telegram/Signal/iMessage/Microsoft Teams：使用 `groupAllowFrom`（回退到 `allowFrom`）。
- Discord：白名单使用 `channels.discord.guilds.<id>.channels`。
- Slack：白名单使用 `channels.slack.channels`。
- Matrix：白名单使用 `channels.matrix.groups`（房间 ID、别名或名称）。
- 群组 DM 单独控制（`channels.discord.dm.*`、`channels.slack.dm.*`）。
- Telegram 白名单可匹配用户 ID 或用户名；前缀不区分大小写。
- 默认 `groupPolicy: "allowlist"`；如果群白名单为空，群消息被阻止。

评估顺序（群消息处理的心理模型）：

1. `groupPolicy`（open/disabled/allowlist）
2. 群白名单（`*.groups`、`*.groupAllowFrom`、频道特定白名单）
3. 提及门控（`requireMention`、`/activation`）

## 提及门控（默认）

群消息默认需要 @提及，除非按群覆盖。默认值在每个子系统的 `*.groups."*"` 下。

回复 bot 消息算作隐式提及（当频道支持回复元数据时）。适用于 Telegram、WhatsApp、Slack、Discord 和 Microsoft Teams。

```json5
{
  channels: {
    whatsapp: {
      groups: {
        "*": { requireMention: true },
        "123@g.us": { requireMention: false },
      },
    },
    telegram: {
      groups: {
        "*": { requireMention: true },
        "123456789": { requireMention: false },
      },
    },
  },
  agents: {
    list: [
      {
        id: "main",
        groupChat: {
          mentionPatterns: ["@openacosmi", "openacosmi", "\\+15555550123"],
          historyLimit: 50,
        },
      },
    ],
  },
}
```

备注：

- `mentionPatterns` 是不区分大小写的正则表达式。
- 提供显式提及的平台仍然通过；模式匹配是回退方案。
- 每 agent 覆盖：`agents.list[].groupChat.mentionPatterns`。
- 提及门控仅在可检测提及时强制执行（原生提及或已配置 `mentionPatterns`）。
- Discord 默认在 `channels.discord.guilds."*"` 中（可按 guild/channel 覆盖）。
- 群历史上下文在频道间统一包装；使用 `messages.groupChat.historyLimit` 设置全局默认值。设置 `0` 禁用。

## 群组/频道工具限制（可选）

某些频道配置支持限制特定群组/房间/频道中可用的工具。

- `tools`：群组整体的允许/拒绝工具。
- `toolsBySender`：群组内按发送者覆盖（键为发送者 ID/用户名/邮箱/电话号码，取决于频道）。用 `"*"` 作通配符。

解析顺序（最具体的优先）：

1. 群组/频道 `toolsBySender` 匹配
2. 群组/频道 `tools`
3. 默认（`"*"`）`toolsBySender` 匹配
4. 默认（`"*"`）`tools`

示例（Telegram）：

```json5
{
  channels: {
    telegram: {
      groups: {
        "*": { tools: { deny: ["exec"] } },
        "-1001234567890": {
          tools: { deny: ["exec", "read", "write"] },
          toolsBySender: {
            "123456789": { alsoAllow: ["exec"] },
          },
        },
      },
    },
  },
}
```

备注：

- 群组/频道工具限制在全局/agent 工具策略之上应用（deny 始终优先）。
- 某些频道使用不同的嵌套结构（如 Discord `guilds.*.channels.*`、Slack `channels.*`、MS Teams `teams.*.channels.*`）。

## 群白名单

当配置了 `channels.whatsapp.groups`、`channels.telegram.groups` 或 `channels.imessage.groups` 时，键值充当群白名单。使用 `"*"` 允许所有群组同时设置默认提及行为。

常见意图（可直接复制使用）：

1. 禁用所有群回复

```json5
{
  channels: { whatsapp: { groupPolicy: "disabled" } },
}
```

1. 仅允许特定群组（WhatsApp）

```json5
{
  channels: {
    whatsapp: {
      groups: {
        "123@g.us": { requireMention: true },
        "456@g.us": { requireMention: false },
      },
    },
  },
}
```

1. 允许所有群组但需要提及（显式）

```json5
{
  channels: {
    whatsapp: {
      groups: { "*": { requireMention: true } },
    },
  },
}
```

1. 仅所有者可在群中触发（WhatsApp）

```json5
{
  channels: {
    whatsapp: {
      groupPolicy: "allowlist",
      groupAllowFrom: ["+15551234567"],
      groups: { "*": { requireMention: true } },
    },
  },
}
```

## 激活命令（仅所有者）

群所有者可切换每群激活模式：

- `/activation mention`
- `/activation always`

所有者由 `channels.whatsapp.allowFrom` 确定（未设置时使用 bot 自身的 E.164 号码）。以独立消息发送该命令。其他平台目前忽略 `/activation`。

## 上下文字段

群组入站载荷设置：

- `ChatType=group`
- `GroupSubject`（如已知）
- `GroupMembers`（如已知）
- `WasMentioned`（提及门控结果）
- Telegram 论坛主题还包括 `MessageThreadId` 和 `IsForum`。

Agent 系统提示在新群组会话的第一轮包含群组简介。它提醒模型像人类一样回复、避免 Markdown 表格、避免输入字面的 `\n` 序列。

## iMessage 特殊说明

- 路由或白名单中优先使用 `chat_id:<id>`。
- 列出聊天：`imsg chats --limit 20`。
- 群回复始终返回同一 `chat_id`。

## WhatsApp 特殊说明

WhatsApp 专属行为（历史注入、提及处理详情）参见 [群消息](/channels/group-messages)。
