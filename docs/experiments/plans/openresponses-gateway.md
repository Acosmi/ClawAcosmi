---
summary: "计划：添加 OpenResponses /v1/responses 端点并清晰弃用 Chat Completions"
owner: "openacosmi"
status: "draft"
last_updated: "2026-01-19"
title: "OpenResponses 网关计划"
arch: go-gateway
---

# OpenResponses 网关集成计划

> [!IMPORTANT]
> **架构状态**：本计划中所有网关端点和 Schema 验证均在 **Go Gateway**（`backend/internal/gateway/`）实现。
> 原文档引用的 TypeScript/Zod 方案已更新为 Go struct + 验证方式。

## 背景

OpenAcosmi Gateway 当前在 `/v1/chat/completions` 暴露了一个最小的 OpenAI 兼容 Chat Completions 端点（参见 [OpenAI Chat Completions](/gateway/openai-http-api)）。

Open Responses 是基于 OpenAI Responses API 的开放推理标准。它专为 Agent 工作流设计，使用基于 item 的输入和语义化流式事件。OpenResponses 规范定义的是 `/v1/responses`，而非 `/v1/chat/completions`。

## 目标

- 添加符合 OpenResponses 语义的 `/v1/responses` 端点。
- 将 Chat Completions 保留为兼容层，便于禁用和最终移除。
- 使用独立、可复用的 Go struct 标准化验证和解析。

## 非目标

- 首次实现不追求 OpenResponses 完整功能（images、files、hosted tools）。
- 不替换内部 Agent 执行逻辑或工具编排。
- 首阶段不改变现有 `/v1/chat/completions` 行为。

## 调研摘要

来源：OpenResponses OpenAPI、OpenResponses 规范网站和 Hugging Face 博客文章。

关键要点：

- `POST /v1/responses` 接受 `CreateResponseBody` 字段，如 `model`、`input`（字符串或 `ItemParam[]`）、`instructions`、`tools`、`tool_choice`、`stream`、`max_output_tokens` 和 `max_tool_calls`。
- `ItemParam` 是以下类型的区分联合体：
  - `message` items（角色 `system`、`developer`、`user`、`assistant`）
  - `function_call` 和 `function_call_output`
  - `reasoning`
  - `item_reference`
- 成功响应返回 `ResponseResource`，包含 `object: "response"`、`status` 和 `output` items。
- 流式传输使用语义事件：
  - `response.created`、`response.in_progress`、`response.completed`、`response.failed`
  - `response.output_item.added`、`response.output_item.done`
  - `response.content_part.added`、`response.content_part.done`
  - `response.output_text.delta`、`response.output_text.done`
- 规范要求：
  - `Content-Type: text/event-stream`
  - `event:` 必须与 JSON `type` 字段匹配
  - 终止事件必须是字面量 `[DONE]`
- Reasoning items 可暴露 `content`、`encrypted_content` 和 `summary`。
- HF 示例在请求中包含 `OpenResponses-Version: latest` 头（可选）。

## 方案架构

- 添加 `backend/internal/gateway/openresponses_schema.go`：仅包含 Go struct 定义和验证逻辑（不依赖 Gateway 其他模块）。
- 添加 `backend/internal/gateway/openresponses_http.go`：实现 `/v1/responses` HTTP handler。
- 保留 `backend/internal/gateway/openai_http.go` 作为旧版兼容适配器。
- 添加配置 `gateway.http.endpoints.responses.enabled`（默认 `false`）。
- `gateway.http.endpoints.chatCompletions.enabled` 独立控制；允许两个端点分别开关。
- Chat Completions 启用时在启动日志中发出旧版警告。

## Chat Completions 弃用路径

- 保持严格模块边界：responses 和 chat completions 之间不共享 Schema 类型。
- 通过配置使 Chat Completions 可选，以便无需修改代码即可禁用。
- `/v1/responses` 稳定后更新文档，将 Chat Completions 标记为旧版。
- 可选后续步骤：将 Chat Completions 请求映射到 Responses handler，简化移除路径。

## 第一阶段支持范围

- 接受 `input` 为字符串或含 message 角色和 `function_call_output` 的 `ItemParam[]`。
- 从输入中提取 system 和 developer 消息到 `extraSystemPrompt`。
- 使用最近的 `user` 或 `function_call_output` 作为 Agent 运行的当前消息。
- 对不支持的 content part（image/file）返回 `invalid_request_error`。
- 返回包含 `output_text` 内容的单个 assistant 消息。
- `usage` 返回零值，直到 token 统计接入。

## 验证策略（无 SDK）

- 在 Go 中实现支持子集的 struct 定义和验证：
  - `CreateResponseBody`
  - `ItemParam` + message content part 联合类型
  - `ResponseResource`
  - Gateway 使用的流式事件结构
- 将 struct 定义保持在单独的隔离模块中，避免漂移并支持未来代码生成。

## 流式实现（第一阶段）

- SSE 行包含 `event:` 和 `data:`。
- 必需序列（最小可行）：
  - `response.created`
  - `response.output_item.added`
  - `response.content_part.added`
  - `response.output_text.delta`（按需重复）
  - `response.output_text.done`
  - `response.content_part.done`
  - `response.completed`
  - `[DONE]`

## 测试与验证计划

- 为 `/v1/responses` 添加端到端测试覆盖（Go 测试）：
  - 需要认证
  - 非流式响应结构
  - 流式事件排序和 `[DONE]`
  - 使用 headers 和 `user` 的会话路由
- 保留 `backend/internal/gateway/openai_http_test.go` 不变。
- 手动测试：对 `/v1/responses` 发送 `stream: true` 的 curl 请求，验证事件排序和终止 `[DONE]`。

## 文档更新（后续）

- 为 `/v1/responses` 添加新文档页面，包含用法和示例。
- 更新 `/gateway/openai-http-api` 添加旧版说明并指向 `/v1/responses`。
