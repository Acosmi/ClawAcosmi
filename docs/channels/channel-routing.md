---
summary: "频道路由规则与会话键结构"
read_when:
  - 修改频道路由或收件箱行为
title: "频道路由（Channel Routing）"
---

# 频道与路由

> [!IMPORTANT]
> **架构状态**：频道路由由 **Go Gateway**（`backend/internal/channels/channel_match.go`）实现。
> 路由规则在 Go Gateway 配置中定义，Rust CLI 不参与运行时路由决策。

OpenAcosmi 将回复**确定性地路由回消息来源频道**。模型不会选择频道；路由完全由宿主配置控制。

## 关键术语

- **Channel**：频道标识 — `whatsapp`、`telegram`、`discord`、`slack`、`signal`、`imessage`、`feishu`、`dingtalk`、`wecom`、`webchat` 等。
- **AccountId**：每频道的账户实例（支持多账户时）。
- **AgentId**：隔离的工作区 + 会话存储（"大脑"）。
- **SessionKey**：会话上下文存储与并发控制的桶键。

## 会话键格式（示例）

私聊（DM）折叠到 agent 的**主会话**：

- `agent:<agentId>:<mainKey>`（默认：`agent:main:main`）

群组和频道按来源隔离：

- 群组：`agent:<agentId>:<channel>:group:<id>`
- 频道/房间：`agent:<agentId>:<channel>:channel:<id>`

线程（Threads）：

- Slack/Discord 线程追加 `:thread:<threadId>` 到基础键。
- Telegram 论坛主题嵌入 `:topic:<topicId>` 到群组键。

示例：

- `agent:main:telegram:group:-1001234567890:topic:42`
- `agent:main:discord:channel:123456:thread:987654`

## 路由规则（如何选择 agent）

路由为每条入站消息选择**一个 agent**：

1. **精确 peer 匹配**（`bindings` 中的 `peer.kind` + `peer.id`）。
2. **Guild 匹配**（Discord）通过 `guildId`。
3. **Team 匹配**（Slack）通过 `teamId`。
4. **Account 匹配**（频道上的 `accountId`）。
5. **Channel 匹配**（该频道的任意账户）。
6. **默认 agent**（`agents.list[].default`，否则取列表第一项，最终回退到 `main`）。

匹配到的 agent 决定使用哪个工作区和会话存储。

## 广播组（多 Agent 并行）

广播组允许对同一 peer 运行**多个 agent**（例如：WhatsApp 群组中，通过提及/激活门控后）。

配置：

```json5
{
  broadcast: {
    strategy: "parallel",
    "120363403215116621@g.us": ["alfred", "baerbel"],
    "+15555550123": ["support", "logger"],
  },
}
```

详见：[广播组](/channels/broadcast-groups)。

## 配置概览

- `agents.list`：命名的 agent 定义（工作区、模型等）。
- `bindings`：将入站频道/账户/peer 映射到 agent。

示例：

```json5
{
  agents: {
    list: [{ id: "support", name: "Support", workspace: "~/.openacosmi/workspace-support" }],
  },
  bindings: [
    { match: { channel: "slack", teamId: "T123" }, agentId: "support" },
    { match: { channel: "telegram", peer: { kind: "group", id: "-100123" } }, agentId: "support" },
  ],
}
```

## 会话存储

会话存储位于状态目录（默认 `~/.openacosmi`）下：

- `~/.openacosmi/agents/<agentId>/sessions/sessions.json`
- JSONL 转录文件与会话存储并列

可通过 `session.store` 和 `{agentId}` 模板覆盖存储路径。

## WebChat 行为

WebChat 附加到**选定的 agent**，默认使用该 agent 的主会话。因此，WebChat 可在一处查看该 agent 的跨频道上下文。

## 回复上下文

入站回复包含：

- `ReplyToId`、`ReplyToBody` 和 `ReplyToSender`（可用时）。
- 引用上下文以 `[Replying to ...]` 块追加到 `Body`。

此行为在所有频道中保持一致。
