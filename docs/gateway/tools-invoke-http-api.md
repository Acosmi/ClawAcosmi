---
summary: "通过 Gateway HTTP 端点直接调用单个工具"
read_when:
  - 不运行完整 Agent 回合直接调用工具
  - 构建需要工具策略强制执行的自动化
title: "Tools Invoke API"
---

# Tools Invoke (HTTP)

> [!IMPORTANT]
> **架构状态**：此端点由 **Go Gateway**（`backend/internal/gateway/tools_invoke_http.go`）实现。

OpenAcosmi Gateway 暴露一个简单的 HTTP 端点用于直接调用单个工具。始终启用，但受 Gateway 认证和工具策略门控。

- `POST /tools/invoke`
- 与 Gateway 同端口（WS + HTTP 复用）：`http://<gateway-host>:<port>/tools/invoke`

默认最大请求体为 2 MB。

## 认证

使用 Gateway 认证配置，发送 Bearer token：

- `Authorization: Bearer <token>`

说明：

- `gateway.auth.mode="token"` 时使用 `gateway.auth.token`。
- `gateway.auth.mode="password"` 时使用 `gateway.auth.password`。

## 请求体

```json
{
  "tool": "sessions_list",
  "action": "json",
  "args": {},
  "sessionKey": "main",
  "dryRun": false
}
```

字段：

- `tool`（string，必需）：要调用的工具名。
- `action`（string，可选）：工具 schema 支持时映射到 args。
- `args`（object，可选）：工具特定参数。
- `sessionKey`（string，可选）：目标会话键。省略或 `"main"` 时使用配置的主会话键。
- `dryRun`（boolean，可选）：预留，目前忽略。

## 策略 + 路由行为

工具可用性通过与 Gateway Agent 相同的策略链过滤：

- `tools.profile` / `tools.byProvider.profile`
- `tools.allow` / `tools.byProvider.allow`
- `agents.<id>.tools.allow`
- 群组策略（session key 映射到群组时）

工具不在策略白名单中时返回 **404**。

## 响应

- `200` → `{ ok: true, result }`
- `400` → `{ ok: false, error: { type, message } }`（无效请求或工具错误）
- `401` → 未认证
- `404` → 工具不可用
- `405` → 方法不允许

## 示例

```bash
curl -sS http://127.0.0.1:19001/tools/invoke \
  -H 'Authorization: Bearer YOUR_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{
    "tool": "sessions_list",
    "action": "json",
    "args": {}
  }'
```
