---
summary: "上下文：模型看到什么、如何构建、如何检查"
read_when:
  - 需要了解 OpenAcosmi 中"上下文"的含义
  - 调试模型为何"知道"某些事（或忘记了）
  - 需要减少上下文开销（/context、/status、/compact）
title: "上下文"
status: active
arch: go-gateway
---

# 上下文

> [!NOTE]
> **架构状态**：上下文组装由 **Go Gateway** 在 Agent 运行时处理（`backend/internal/agents/runner/`）。

"上下文"是 **OpenAcosmi 在一次运行中发送给模型的所有内容**。受模型**上下文窗口**（token 限制）约束。

初学者心智模型：

- **系统提示词**（OpenAcosmi 构建）：规则、工具、Skills 列表、时间/运行时、注入的工作区文件。
- **对话历史**：该会话中你的消息 + 助手的消息。
- **工具调用/结果 + 附件**：命令输出、文件读取、图片/音频等。

上下文与"记忆"_不同_：记忆可存储到磁盘并稍后重新加载；上下文是模型当前窗口内的内容。

## 快速开始（检查上下文）

- `/status` → 快速查看"我的窗口有多满？" + 会话设置。
- `/context list` → 注入了什么 + 大致大小（按文件 + 总计）。
- `/context detail` → 更详细的分解：按文件、按工具 schema 大小、按 Skill 条目大小和系统提示词大小。
- `/usage tokens` → 在正常回复后附加按回复的用量注脚。
- `/compact` → 将旧历史总结为紧凑条目以释放窗口空间。

参见：[斜杠命令](/tools/slash-commands)、[Token 使用与成本](/reference/token-use)、[压缩](/concepts/compaction)。

## 什么计入上下文窗口

模型接收的一切都计数，包括：

- 系统提示词（所有段落）。
- 对话历史。
- 工具调用 + 工具结果。
- 附件/转录（图片/音频/文件）。
- 压缩摘要和剪枝产物。
- Provider "包装器"或隐藏头（不可见，但仍计数）。

## OpenAcosmi 如何构建系统提示词

系统提示词由 **OpenAcosmi 拥有**，每次运行时重新构建。包含：

- 工具列表 + 简短描述。
- Skills 列表（仅元数据；见下文）。
- 工作区位置。
- 时间（UTC + 转换后的用户时间，如已配置）。
- 运行时元数据（主机/操作系统/模型/思考）。
- **Project Context** 下注入的工作区 bootstrap 文件。

详细分解：[系统提示词](/concepts/system-prompt)。

## 注入的工作区文件（Project Context）

默认情况下，OpenAcosmi 注入一组固定的工作区文件（如存在）：

- `AGENTS.md`、`SOUL.md`、`TOOLS.md`、`IDENTITY.md`、`USER.md`、`HEARTBEAT.md`、`BOOTSTRAP.md`（仅首次运行）

大文件按文件使用 `agents.defaults.bootstrapMaxChars`（默认 `20000` 字符）截断。`/context` 显示**原始 vs 注入**大小及是否发生截断。

## Skills：注入 vs 按需加载

系统提示词包含紧凑的 **Skills 列表**（名称 + 描述 + 位置）。此列表有实际开销。

Skill 指令默认**不**包含。模型预期仅在**需要时** `read` Skill 的 `SKILL.md`。

## 工具：有两种开销

工具以两种方式影响上下文：

1. 系统提示词中的**工具列表文本**。
2. **工具 Schema**（JSON）。发送给模型以便调用工具。即使看不到纯文本形式也计入上下文。

`/context detail` 分解最大的工具 Schema 以便查看主要消耗者。

## 命令、指令和"内联快捷方式"

斜杠命令由 Gateway 处理。有几种不同行为：

- **独立命令**：仅包含 `/...` 的消息作为命令运行。
- **指令**：`/think`、`/verbose`、`/reasoning`、`/elevated`、`/model`、`/queue` 在模型看到消息前被剥离。
- **内联快捷方式**（仅限白名单发送者）：正常消息中的某些 `/...` 令牌可立即运行并在模型看到前被剥离。

详情：[斜杠命令](/tools/slash-commands)。

## 会话、压缩和剪枝（什么持久化）

持久化取决于机制：

- **正常历史**在会话转录中持久化，直到被策略压缩/剪枝。
- **压缩**将摘要持久化到转录中，保持近期消息不变。
- **剪枝**从运行的_内存中_提示词移除旧工具结果，但不重写转录。

文档：[会话](/concepts/session)、[压缩](/concepts/compaction)、[会话剪枝](/concepts/session-pruning)。
