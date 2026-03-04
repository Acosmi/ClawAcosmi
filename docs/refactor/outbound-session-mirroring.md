---
title: 出站会话镜像重构（Issue #1520）
description: 跟踪出站会话镜像重构笔记、决策、测试和未决项。
status: active
arch: rust-cli+go-gateway
---

# 出站会话镜像重构（Issue #1520）

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - 出站会话路由由 **Go Gateway** 处理
> - 会话管理：`backend/internal/gateway/session_manager.go`
> - 消息发送方法：`backend/internal/gateway/server_methods_send.go`

## 状态

- 进行中。
- 核心 + 插件通道路由已更新为出站镜像。
- Gateway 发送现在在省略 sessionKey 时推导目标会话。

## 背景

出站发送被镜像到_当前_ agent 会话（工具会话键），而非目标通道会话。入站路由使用通道/对端会话键，因此出站响应落入了错误的会话，且首次联系目标通常缺少会话条目。

## 目标

- 将出站消息镜像到目标通道会话键。
- 出站时如会话缺失则创建会话条目。
- 保持线程/话题作用域与入站会话键对齐。
- 覆盖核心通道及内置扩展。

## 实现摘要

- 新的出站会话路由辅助函数：
  - `backend/internal/gateway/outbound_session.go`
  - `resolveOutboundSessionRoute` 使用 `buildAgentSessionKey`（dmScope + identityLinks）构建目标 sessionKey。
  - `ensureOutboundSessionEntry` 通过 `recordSessionMetaFromInbound` 写入最小 `MsgContext`。
- `runMessageAction`（发送）推导目标 sessionKey 并传递给 `executeSendAction` 进行镜像。
- `message-tool` 不再直接镜像；它只从当前会话键解析 agentId。
- 插件发送路径通过 `appendAssistantMessageToSessionTranscript` 使用推导的 sessionKey 进行镜像。
- Gateway 发送在未提供 sessionKey 时推导目标会话键（默认 agent），并确保会话条目。

## 线程/话题处理

- Slack：replyTo/threadId -> `resolveThreadSessionKeys`（后缀）。
- Discord：threadId/replyTo -> `resolveThreadSessionKeys` 使用 `useSuffix=false` 以匹配入站（线程通道 id 已作用为域）。
- Telegram：话题 ID 通过 `buildTelegramGroupPeerId` 映射到 `chatId:topic:<id>`。

## 涉及的扩展

- Matrix、MS Teams、Mattermost、BlueBubbles、Nextcloud Talk、Zalo、Zalo Personal、Nostr、Tlon。
- 备注：
  - Mattermost 目标现在去除 `@` 用于 DM 会话键路由。
  - Zalo Personal 对 1:1 目标使用 DM 对端类型（仅当存在 `group:` 时为群组）。
  - BlueBubbles 群组目标去除 `chat_*` 前缀以匹配入站会话键。
  - Slack 自动线程镜像不区分通道 id 大小写。
  - Gateway 发送在镜像前将提供的会话键转为小写。

## 决策

- **Gateway 发送会话推导**：如提供了 `sessionKey` 则使用它。如省略，则从目标 + 默认 agent 推导 sessionKey 并镜像。
- **会话条目创建**：始终使用 `recordSessionMetaFromInbound`，`Provider/From/To/ChatType/AccountId/Originating*` 与入站格式对齐。
- **目标规范化**：出站路由使用解析后的目标（post `resolveChannelTarget`）。
- **会话键大小写**：在写入和迁移时将会话键规范化为小写。

## 已添加/更新的测试

- `backend/internal/gateway/outbound_session_test.go`
  - Slack 线程会话键。
  - Telegram 话题会话键。
  - dmScope identityLinks 与 Discord。
- `backend/internal/agents/tools/message_tool_test.go`
  - 从会话键推导 agentId。
- `backend/internal/gateway/server_methods_send_test.go`
  - 省略时推导会话键并创建会话条目。

## 未决项 / 后续

- 语音通话插件使用自定义 `voice:<phone>` 会话键。出站映射未标准化；如 message-tool 需支持语音通话发送，需添加显式映射。
- 确认是否有外部插件使用超出内置集的非标准 `From/To` 格式。

## 涉及的文件

- `backend/internal/gateway/outbound_session.go`
- `backend/internal/gateway/outbound_send_service.go`
- `backend/internal/gateway/message_action_runner.go`
- `backend/internal/agents/tools/message_tool.go`
- `backend/internal/gateway/server_methods_send.go`
- 测试：
  - `backend/internal/gateway/outbound_session_test.go`
  - `backend/internal/agents/tools/message_tool_test.go`
  - `backend/internal/gateway/server_methods_send_test.go`
