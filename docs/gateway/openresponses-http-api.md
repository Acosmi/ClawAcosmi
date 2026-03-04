---
summary: "通过 Gateway 暴露 OpenResponses 兼容的 /v1/responses HTTP 端点"
read_when:
  - 集成使用 OpenResponses API 的客户端
  - 需要基于 item 的输入、客户端工具调用或 SSE 事件
title: "OpenResponses API"
---

# OpenResponses API (HTTP)

> [!IMPORTANT]
> **架构状态**：此端点由 **Go Gateway**（`backend/internal/gateway/openresponses_http.go`）实现。

OpenAcosmi Gateway 提供 OpenResponses 兼容的 `POST /v1/responses` 端点。

**默认禁用**，需先在配置中启用。

- `POST /v1/responses`
- 与 Gateway 同端口：`http://<gateway-host>:<port>/v1/responses`

## 认证

使用 Gateway 认证配置，发送 Bearer token：`Authorization: Bearer <token>`

## 选择 Agent

与 Chat Completions 相同：`model: "openacosmi:<agentId>"` 或 `x-openacosmi-agent-id` header。

## 启用

```json5
{
  gateway: {
    http: {
      endpoints: {
        responses: { enabled: true },
      },
    },
  },
}
```

## 请求格式

支持 OpenResponses API 基于 item 的输入：

- `input`：字符串或 item 对象数组
- `instructions`：合并到系统提示
- `tools`：客户端工具定义（function）
- `tool_choice`：过滤或要求客户端工具
- `stream`：启用 SSE 流式
- `max_output_tokens`：输出限制

### Item 类型

- `message`（角色：`system`、`developer`、`user`、`assistant`）
- `function_call_output`（返回工具结果）
- `input_image`（支持 base64 或 URL）
- `input_file`（支持 base64 或 URL；PDF 解析）

### 文件 + 图片限制

```json5
{
  gateway: {
    http: {
      endpoints: {
        responses: {
          enabled: true,
          maxBodyBytes: 20000000,
          files: { maxBytes: 5242880, maxChars: 200000 },
          images: { maxBytes: 10485760 },
        },
      },
    },
  },
}
```

## 流式（SSE）

`stream: true` 时返回 SSE 事件：

`response.created` → `response.in_progress` → `response.output_text.delta` → `response.completed`

## 示例

```bash
curl -sS http://127.0.0.1:19001/v1/responses \
  -H 'Authorization: Bearer YOUR_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "openacosmi",
    "input": "hi"
  }'
```
