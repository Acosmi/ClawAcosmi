---
summary: "通过 Gateway 暴露 OpenAI 兼容的 /v1/chat/completions HTTP 端点"
read_when:
  - 集成需要 OpenAI Chat Completions 的工具
title: "OpenAI Chat Completions API"
---

# OpenAI Chat Completions (HTTP)

> [!IMPORTANT]
> **架构状态**：此端点由 **Go Gateway**（`backend/internal/gateway/openai_http.go`）实现。

OpenAcosmi Gateway 提供 OpenAI 兼容的 Chat Completions 端点。

此端点**默认禁用**，需先在配置中启用。

- `POST /v1/chat/completions`
- 与 Gateway 同端口（WS + HTTP 复用）：`http://<gateway-host>:<port>/v1/chat/completions`

底层请求作为标准 Gateway Agent 运行执行（与 `openacosmi agent` 相同路径），因此路由/权限/配置与 Gateway 一致。

## 认证

使用 Gateway 认证配置，发送 Bearer token：

- `Authorization: Bearer <token>`

说明：

- `gateway.auth.mode="token"` 时使用 `gateway.auth.token`（或 `OPENACOSMI_GATEWAY_TOKEN`）。
- `gateway.auth.mode="password"` 时使用 `gateway.auth.password`（或 `OPENACOSMI_GATEWAY_PASSWORD`）。

## 选择 Agent

在 OpenAI `model` 字段中编码 agent ID：

- `model: "openacosmi:<agentId>"`（示例：`"openacosmi:main"`）
- `model: "agent:<agentId>"`（别名）

或通过 header 指定：

- `x-openacosmi-agent-id: <agentId>`（默认：`main`）

高级用法：

- `x-openacosmi-session-key: <sessionKey>` 完全控制会话路由。

## 启用端点

设置 `gateway.http.endpoints.chatCompletions.enabled` 为 `true`：

```json5
{
  gateway: {
    http: {
      endpoints: {
        chatCompletions: { enabled: true },
      },
    },
  },
}
```

## 会话行为

默认**无状态**（每次调用生成新的 session key）。

如果请求包含 OpenAI `user` 字符串，Gateway 从中派生稳定的 session key，重复调用可共享 Agent 会话。

## 流式（SSE）

设置 `stream: true` 接收 Server-Sent Events：

- `Content-Type: text/event-stream`
- 每个事件行：`data: <json>`
- 流结束：`data: [DONE]`

## 示例

```bash
curl -sS http://127.0.0.1:19001/v1/chat/completions \
  -H 'Authorization: Bearer YOUR_TOKEN' \
  -H 'Content-Type: application/json' \
  -H 'x-openacosmi-agent-id: main' \
  -d '{
    "model": "openacosmi",
    "messages": [{"role":"user","content":"hi"}]
  }'
```

流式：

```bash
curl -N http://127.0.0.1:19001/v1/chat/completions \
  -H 'Authorization: Bearer YOUR_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "openacosmi",
    "stream": true,
    "messages": [{"role":"user","content":"hi"}]
  }'
```
