---
summary: "使用量追踪的展示界面和凭证要求"
read_when:
  - 接入 Provider 使用量/配额界面
  - 需要解释使用量追踪行为或认证要求
title: "使用量追踪"
status: active
arch: go-gateway+rust-cli
---

# 使用量追踪

> [!NOTE]
> **架构状态**：使用量采集由 **Go Gateway** 处理。
> 展示由 **Rust CLI**（`openacosmi status --usage`）和 **Web UI** 提供。

## 功能说明

- 直接从 Provider 的使用量端点拉取使用量/配额数据。
- **不做估算成本**；仅展示 Provider 报告的窗口数据。

## 展示位置

- `/status`（聊天中）：带表情的状态卡片，含会话 token 数 + 估算成本（仅 API Key）。可用时展示**当前模型 Provider** 的使用量。
- `/usage off|tokens|full`（聊天中）：按回复的使用量注脚（OAuth 仅展示 token 数）。
- `/usage cost`（聊天中）：从 OpenAcosmi 会话日志聚合的本地成本摘要。
- CLI：`openacosmi status --usage` 打印完整的按 Provider 分解。
- CLI：`openacosmi channels list` 可打印使用量快照（`--no-usage` 跳过）。
- macOS 菜单栏："Usage" 区段（可用时）。

## Provider 与凭证

| Provider | 凭证类型 |
|----------|----------|
| Anthropic（Claude） | OAuth 令牌（auth profile） |
| GitHub Copilot | OAuth 令牌（auth profile） |
| Gemini CLI | OAuth 令牌（auth profile） |
| Antigravity | OAuth 令牌（auth profile） |
| OpenAI Codex | OAuth 令牌（auth profile，使用 accountId） |
| MiniMax | API Key（`MINIMAX_CODE_PLAN_KEY` 或 `MINIMAX_API_KEY`） |
| z.ai | API Key（环境变量/配置/auth store） |

无匹配 OAuth/API 凭证时使用量信息不显示。
