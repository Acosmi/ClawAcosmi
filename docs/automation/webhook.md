---
summary: "Webhook 入口：外部唤醒和隔离 Agent 运行"
read_when:
  - 添加或修改 webhook 端点
  - 将外部系统接入 OpenAcosmi
title: "Webhooks"
---

# Webhooks

> [!IMPORTANT]
> **架构状态**：Webhook 系统由 **Go Gateway** 原生实现。
> 核心代码：`backend/internal/gateway/hooks.go`（端点、Token 验证、Payload 处理）、
> `backend/internal/gateway/hooks_mapping.go`（映射匹配与模板渲染）。

Gateway 可暴露一个小型 HTTP webhook 端点，供外部系统触发 Agent 运行。

## 启用

```json5
{
  hooks: {
    enabled: true,
    token: "shared-secret",
    path: "/hooks",
  },
}
```

说明：

- `hooks.token` 在 `hooks.enabled=true` 时为必填项。
- `hooks.path` 默认为 `/hooks`。

## 认证

每个请求必须携带 hook token。推荐使用 Header 方式：

- `Authorization: Bearer <token>`（推荐）
- `x-openacosmi-token: <token>`
- `?token=<token>`（已废弃；会记录警告日志，将在未来主版本中移除）

## 端点

### `POST /hooks/wake`

Payload：

```json
{ "text": "系统事件描述", "mode": "now" }
```

- `text` **必填**（string）：事件描述（例如"收到新邮件"）。
- `mode` 可选（`now` | `next-heartbeat`）：是否触发立即心跳（默认 `now`）或等待下次定期检查。

效果：

- 为**主会话**入队一个 system event
- 如果 `mode=now`，触发立即心跳

### `POST /hooks/agent`

Payload：

```json
{
  "message": "执行此任务",
  "name": "Email",
  "sessionKey": "hook:email:msg-123",
  "wakeMode": "now",
  "deliver": true,
  "channel": "last",
  "to": "+15551234567",
  "model": "openai/gpt-5.2-mini",
  "thinking": "low",
  "timeoutSeconds": 120
}
```

- `message` **必填**（string）：Agent 处理的提示词或消息。
- `name` 可选（string）：Hook 的可读名称（如"GitHub"），用作会话摘要前缀。
- `sessionKey` 可选（string）：Agent 会话的标识 key。默认为随机 `hook:<uuid>`。使用一致的 key 可在同一 hook 上下文中实现多轮对话。
- `wakeMode` 可选（`now` | `next-heartbeat`）：是否触发立即心跳（默认 `now`）。
- `deliver` 可选（boolean）：如为 `true`，Agent 响应将发送到消息渠道。默认 `true`。仅为心跳确认的响应会自动跳过。
- `channel` 可选（string）：投递渠道。可选值：`last`、`whatsapp`、`telegram`、`discord`、`slack`、`mattermost`（插件）、`signal`、`imessage`、`msteams`。默认 `last`。
- `to` 可选（string）：渠道接收方标识（如 WhatsApp/Signal 的手机号、Telegram 的 chat ID、Discord/Slack/Mattermost 的 channel ID、MS Teams 的 conversation ID）。默认使用主会话的上次接收方。
- `model` 可选（string）：模型覆盖（如 `anthropic/claude-3-5-sonnet` 或别名）。如有模型限制列表则必须在其中。
- `thinking` 可选（string）：思考级别覆盖（如 `low`、`medium`、`high`）。
- `timeoutSeconds` 可选（number）：Agent 运行的最大时长（秒）。

效果：

- 运行**隔离** agent turn（独立 session key）
- 始终向**主会话**发布摘要
- 如果 `wakeMode=now`，触发立即心跳

### `POST /hooks/<name>`（映射）

自定义 hook 名称通过 `hooks.mappings` 解析（见配置）。映射可将任意 payload 转换为 `wake` 或 `agent` 动作，支持模板或代码转换。

映射选项概要：

- `hooks.presets: ["gmail"]` 启用内置 Gmail 映射。
- `hooks.mappings` 允许在配置中定义 `match`、`action` 和模板。
- `hooks.transformsDir` + `transform.module` 加载自定义转换模块。
- 使用 `match.source` 保留通用入口端点（基于 payload 路由）。
- 设置 `deliver: true` + `channel`/`to` 可将回复路由到聊天界面（`channel` 默认 `last`，回退到 WhatsApp）。
- `allowUnsafeExternalContent: true` 为该 hook 禁用外部内容安全包装（危险；仅用于受信内部源）。
- `openacosmi webhooks gmail setup` 为 `openacosmi webhooks gmail run` 写入 `hooks.gmail` 配置。详见 [Gmail Pub/Sub](/automation/gmail-pubsub)。

## 响应码

- `200`：`/hooks/wake` 成功
- `202`：`/hooks/agent` 异步运行已启动
- `401`：认证失败
- `400`：Payload 无效
- `413`：Payload 过大

## 示例

```bash
curl -X POST http://127.0.0.1:18789/hooks/wake \
  -H 'Authorization: Bearer SECRET' \
  -H 'Content-Type: application/json' \
  -d '{"text":"收到新邮件","mode":"now"}'
```

```bash
curl -X POST http://127.0.0.1:18789/hooks/agent \
  -H 'x-openacosmi-token: SECRET' \
  -H 'Content-Type: application/json' \
  -d '{"message":"总结收件箱","name":"Email","wakeMode":"next-heartbeat"}'
```

### 使用不同模型

在 agent payload（或映射）中添加 `model` 来覆盖该次运行的模型：

```bash
curl -X POST http://127.0.0.1:18789/hooks/agent \
  -H 'x-openacosmi-token: SECRET' \
  -H 'Content-Type: application/json' \
  -d '{"message":"总结收件箱","name":"Email","model":"openai/gpt-5.2-mini"}'
```

如果启用了 `agents.defaults.models` 限制，确保覆盖的模型在列表中。

```bash
curl -X POST http://127.0.0.1:18789/hooks/gmail \
  -H 'Authorization: Bearer SECRET' \
  -H 'Content-Type: application/json' \
  -d '{"source":"gmail","messages":[{"from":"Ada","subject":"Hello","snippet":"Hi"}]}'
```

## 安全建议

- 将 hook 端点限制在 loopback、tailnet 或受信反向代理之后。
- 使用专用 hook token；不要复用 Gateway 认证 token。
- 避免在 webhook 日志中包含敏感原始 payload。
- Hook payload 默认被视为不可信，并用安全边界包装。如必须为特定 hook 禁用此行为，在该 hook 映射中设置 `allowUnsafeExternalContent: true`（危险）。
