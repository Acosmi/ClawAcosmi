---
summary: "Agent 运行时（Go Gateway 嵌入式 pi-agent）、工作区契约和会话引导"
read_when:
  - 修改 Agent 运行时、工作区引导或会话行为
title: "Agent 运行时"
status: active
arch: go-gateway
---

# Agent 运行时 🤖

> [!IMPORTANT]
> **架构状态**：Agent 运行时由 **Go Gateway** 实现（`backend/internal/agents/`）。
> Rust CLI 通过 WebSocket RPC 调用 `agent` 方法触发运行，不直接执行 Agent 逻辑。

OpenAcosmi 运行一个嵌入式 Agent 运行时。

## 工作区（必需）

OpenAcosmi 使用单个 Agent 工作区目录（`agents.defaults.workspace`）作为 Agent 的**唯一**工作目录（`cwd`），用于工具和上下文。

推荐：使用 `openacosmi setup` 创建 `~/.openacosmi/openacosmi.json`（如缺失）并初始化工作区文件。

完整工作区布局 + 备份指南：[Agent 工作区](/concepts/agent-workspace)

如果启用了 `agents.defaults.sandbox`，非主会话可使用 `agents.defaults.sandbox.workspaceRoot` 下的按会话工作区（参见 [Gateway 配置](/gateway/configuration)）。

## Bootstrap 文件（注入）

在 `agents.defaults.workspace` 内，OpenAcosmi 期望这些用户可编辑的文件：

- `AGENTS.md` — 操作指令 + "记忆"
- `SOUL.md` — 人格、边界、语气
- `TOOLS.md` — 用户维护的工具注释（如 `imsg`、`sag`、约定）
- `BOOTSTRAP.md` — 一次性首次运行仪式（完成后删除）
- `IDENTITY.md` — Agent 名称/风格/表情
- `USER.md` — 用户档案 + 偏好称呼

在新会话的第一轮，OpenAcosmi 将这些文件内容直接注入 Agent 上下文。

空文件将被跳过。大文件会被裁剪和截断并标记，以保持提示词精简（读取文件获取完整内容）。

缺失的文件会注入一行"缺失文件"标记（`openacosmi setup` 可创建安全的默认模板）。

`BOOTSTRAP.md` 仅在**全新工作区**（无其他 bootstrap 文件存在）时创建。完成仪式后删除它，后续重启不会重新创建。

要完全禁用 bootstrap 文件创建（用于预填充的工作区），设置：

```json5
{ agent: { skipBootstrap: true } }
```

## 内置工具

核心工具（read/exec/edit/write 及相关系统工具）始终可用，受工具策略约束。`apply_patch` 为可选，由 `tools.exec.applyPatch` 控制。`TOOLS.md` **不**控制哪些工具存在；它仅是使用指导。

## Skills

OpenAcosmi 从三个位置加载 Skills（名称冲突时工作区优先）：

- 捆绑（随安装包提供）
- 托管/本地：`~/.openacosmi/skills`
- 工作区：`<workspace>/skills`

Skills 可通过配置/环境变量控制（参见 [Gateway 配置](/gateway/configuration) 中的 `skills`）。

## 会话

会话转录以 JSONL 格式存储在：

- `~/.openacosmi/agents/<agentId>/sessions/<SessionId>.jsonl`

会话 ID 是稳定的，由 OpenAcosmi 选择。

## 流式期间引导（Steer）

当队列模式为 `steer` 时，入站消息被注入到当前运行中。队列在**每次工具调用后**检查；如有排队消息，当前助手消息的剩余工具调用将被跳过（以"因排队用户消息而跳过"的错误工具结果标记），然后排队的用户消息在下一次助手响应前注入。

当队列模式为 `followup` 或 `collect` 时，入站消息保持到当前轮结束，然后以排队载荷开始新的 Agent 轮次。参见 [队列](/concepts/queue)。

Block streaming 在完成的助手块可用时发送；默认**关闭**（`agents.defaults.blockStreamingDefault: "off"`）。
通过 `agents.defaults.blockStreamingBreak` 调整边界（`text_end` vs `message_end`；默认 `text_end`）。
通过 `agents.defaults.blockStreamingChunk` 控制软块分块（默认 800–1200 字符；优先段落断行，然后换行，最后句子）。
通过 `agents.defaults.blockStreamingCoalesce` 合并流式块以减少单行刷屏（基于空闲的合并后发送）。非 Telegram 通道需要显式 `*.blockStreaming: true` 以启用 block 回复。
详情：[流式 + 分块](/concepts/streaming)。

## 模型引用

配置中的模型引用（例如 `agents.defaults.model` 和 `agents.defaults.models`）通过**第一个** `/` 分割解析。

- 配置模型时使用 `provider/model`。
- 如果模型 ID 本身包含 `/`（OpenRouter 风格），包含 provider 前缀（示例：`openrouter/moonshotai/kimi-k2`）。
- 省略 provider 时，OpenAcosmi 将输入视为别名或**默认 provider** 的模型（仅在模型 ID 中无 `/` 时有效）。

## 配置（最小化）

至少设置：

- `agents.defaults.workspace`
- `channels.whatsapp.allowFrom`（强烈推荐）

## 代码位置

| 组件 | 位置 |
|------|------|
| Agent 运行器 | `backend/internal/agents/runner/` |
| Agent 作用域 | `backend/internal/agents/scope/` |
| Rust CLI agent 命令 | `cli-rust/crates/oa-cmd-agent/` |
| 会话管理 | `backend/internal/sessions/` |

---

_下一步：[群组聊天](/channels/group-messages)_ 🦜
