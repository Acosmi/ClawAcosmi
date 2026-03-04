---
summary: "CLI 后端：通过本地 AI CLI 的纯文本回退"
read_when:
  - API 提供商故障时需要可靠的回退
  - 使用 Claude Code CLI 或其他本地 AI CLI
title: "CLI 后端"
---

# CLI 后端（回退运行时）

> [!IMPORTANT]
> **架构状态**：CLI 后端由 **Go Gateway**（`backend/internal/agents/runner/cli_backend.go`）管理。

OpenAcosmi 可运行**本地 AI CLI** 作为 API 提供商故障时的**纯文本回退**。

- **工具已禁用**（无工具调用）
- **文本进 → 文本出**
- **支持会话**（跟进回合保持连贯）
- **可传递图片**（如 CLI 支持路径）

## 快速开始

```bash
openacosmi agent --message "hi" --model claude-cli/opus-4.6
openacosmi agent --message "hi" --model codex-cli/gpt-5.3-codex
```

## 作为回退使用

```json5
{
  agents: {
    defaults: {
      model: {
        primary: "anthropic/claude-opus-4-6",
        fallbacks: ["claude-cli/opus-4.6"],
      },
    },
  },
}
```

## 配置

所有 CLI 后端在 `agents.defaults.cliBackends` 下：

```json5
{
  agents: {
    defaults: {
      cliBackends: {
        "claude-cli": { command: "/opt/homebrew/bin/claude" },
      },
    },
  },
}
```

内置默认：`claude-cli`（Claude Code CLI）和 `codex-cli`（Codex CLI）。

## 限制

- 无 OpenAcosmi 工具
- 无流式传输
- 结构化输出依赖 CLI 的 JSON 格式
