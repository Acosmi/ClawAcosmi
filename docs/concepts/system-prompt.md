---
summary: "OpenAcosmi 系统提示词的组成部分及组装方式"
read_when:
  - 编辑系统提示词文本、工具列表或时间/心跳段落
  - 修改工作区 bootstrap 或 Skills 注入行为
title: "系统提示词"
status: active
arch: go-gateway
---

# 系统提示词

> [!IMPORTANT]
> **架构状态**：系统提示词由 **Go Gateway** 在每次 Agent 运行时构建（`backend/internal/agents/runner/`）。
> Rust CLI 不参与提示词组装。

OpenAcosmi 为每次 Agent 运行构建自定义系统提示词。提示词由 OpenAcosmi **拥有和管理**。

## 结构

提示词紧凑设计，使用固定段落：

- **工具列表（Tooling）**：当前工具列表 + 简短描述。
- **安全（Safety）**：简短的护栏提醒，防止权力寻求行为或绕过监督。
- **Skills**（如可用）：告诉模型如何按需加载 Skill 指令。
- **OpenAcosmi 自更新**：如何运行 `config.apply` 和 `update.run`。
- **工作区（Workspace）**：工作目录（`agents.defaults.workspace`）。
- **文档（Documentation）**：OpenAcosmi 文档的本地路径及何时查阅。
- **工作区文件（注入）**：标示 bootstrap 文件已包含在下方。
- **沙箱（Sandbox）**（启用时）：标示沙箱运行时、沙箱路径，以及是否可用提升执行。
- **当前日期与时间**：用户本地时间、时区和时间格式。
- **回复标签（Reply Tags）**：支持的 provider 的可选回复标签语法。
- **心跳（Heartbeats）**：心跳提示词和确认行为。
- **运行时（Runtime）**：主机、操作系统、模型、仓库根目录（检测时）、思考级别（一行）。
- **推理（Reasoning）**：当前可见性级别 + `/reasoning` 切换提示。

系统提示词中的安全护栏是**建议性的**。它们引导模型行为但不强制执行策略。使用工具策略、exec 审批、沙箱和通道白名单进行硬性强制；操作者可按设计禁用这些。

## 提示词模式

OpenAcosmi 可为子 Agent 渲染更小的系统提示词。运行时为每次运行设置 `promptMode`（非用户可见配置）：

- `full`（默认）：包含上述所有段落。
- `minimal`：用于子 Agent；省略 **Skills**、**记忆回忆**、**OpenAcosmi 自更新**、**模型别名**、**用户身份**、**回复标签**、**消息**、**静默回复**和**心跳**。工具列表、**安全**、工作区、沙箱、当前日期与时间（已知时）、运行时和注入上下文保持可用。
- `none`：仅返回基础身份行。

当 `promptMode=minimal` 时，额外注入的提示词标记为 **Subagent Context** 而非 **Group Chat Context**。

## 工作区 Bootstrap 注入

Bootstrap 文件被裁剪后附加在 **Project Context** 下，使模型无需显式读取即可看到身份和配置文件上下文：

- `AGENTS.md`
- `SOUL.md`
- `TOOLS.md`
- `IDENTITY.md`
- `USER.md`
- `HEARTBEAT.md`
- `BOOTSTRAP.md`（仅在全新工作区时）

大文件会被截断并附加标记。每文件最大大小由 `agents.defaults.bootstrapMaxChars`（默认：20000）控制。缺失文件注入一个简短的缺失文件标记。

内部 Hook 可通过 `agent:bootstrap` 拦截此步骤，变更或替换注入的 bootstrap 文件（例如替换 `SOUL.md` 为替代人格）。

要检查每个注入文件的贡献量（原始 vs 注入、截断情况，以及工具 schema 开销），使用 `/context list` 或 `/context detail`。参见 [上下文](/concepts/context)。

## 时间处理

系统提示词在已知用户时区时包含一个专门的**当前日期与时间**段落。为保持提示词缓存稳定，现在仅包含**时区**（无动态时钟或时间格式）。

Agent 需要当前时间时使用 `session_status`；状态卡包含时间戳。

配置：

- `agents.defaults.userTimezone`
- `agents.defaults.timeFormat`（`auto` | `12` | `24`）

参见 [日期与时间](/date-time)。

## Skills

当有合格 Skills 存在时，OpenAcosmi 注入紧凑的**可用 Skills 列表**，包含每个 Skill 的**文件路径**。提示词指示模型使用 `read` 在列出的位置（工作区、托管或捆绑）加载 `SKILL.md`。无合格 Skills 时省略 Skills 段落。

```
<available_skills>
  <skill>
    <name>...</name>
    <description>...</description>
    <location>...</location>
  </skill>
</available_skills>
```

这使基础提示词保持小巧，同时仍支持目标化的 Skill 使用。

## 文档

可用时，系统提示词包含一个**文档**段落，指向本地 OpenAcosmi 文档目录，并注明公共镜像、源代码仓库、社区 Discord 和 ClawHub 用于 Skills 发现。提示词指示模型优先查阅本地文档。
