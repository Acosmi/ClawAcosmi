---
summary: "研究笔记：工作区离线记忆系统（Markdown 为真相来源 + 派生索引）"
read_when:
  - 设计工作区记忆（~/.openacosmi/workspace）超越每日 Markdown 日志
  - 决策：独立 CLI 工具 vs 深度 OpenAcosmi 集成
  - 添加离线回忆 + 反思（retain/recall/reflect）
title: "工作区记忆研究"
status: draft
arch: rust-cli + go-gateway
---

# 工作区记忆 v2（离线）：研究笔记

> [!NOTE]
> **架构状态**：记忆系统为探索性设计。CLI 记忆命令计划在 **Rust CLI**（`cli-rust/`）实现，
> 后端存储和反思调度可由 **Go Gateway**（`backend/internal/`）提供支持。

目标：Clawd 风格的工作区（`agents.defaults.workspace`，默认 `~/.openacosmi/workspace`），其中"记忆"以每日一个 Markdown 文件（`memory/YYYY-MM-DD.md`）加少量稳定文件（如 `memory.md`、`SOUL.md`）的形式存储。

本文提出一种**离线优先**的记忆架构，保持 Markdown 作为规范的、可审查的真相来源，同时通过派生索引添加**结构化回忆**（搜索、实体摘要、置信度更新）。

## 为何要改变？

当前方案（每日一个文件）擅长：

- "仅追加"式记录
- 人工编辑
- 基于 git 的持久性 + 可审计性
- 低摩擦捕获（"直接写下来"）

不足之处：

- 高召回率检索（"我们对 X 做了什么决定？"、"上次尝试 Y 是什么时候？"）
- 以实体为中心的回答（"告诉我关于 Alice / The Castle / warelay"），无需重读大量文件
- 观点/偏好稳定性（及变化时的证据）
- 时间约束（"2025 年 11 月的情况？"）和冲突解决

## 设计目标

- **离线**：无需网络；可在笔记本/Castle 上运行；无云依赖。
- **可解释**：检索到的条目应可追溯（文件 + 位置），与推理分离。
- **低仪式感**：每日日志保持 Markdown，无需复杂 Schema。
- **增量式**：v1 仅用 FTS 即可发挥作用；语义/向量和图谱是可选升级。
- **Agent 友好**：使"在 token 预算内回忆"变得简单（返回小型事实包）。

## 北极星模型（Hindsight × Letta）

两个要融合的部分：

1. **Letta/MemGPT 风格控制循环**

- 保持一个小型"核心"始终在上下文中（persona + 关键用户事实）
- 其他所有内容在上下文外，通过工具检索
- 记忆写入是显式工具调用（append/replace/insert），持久化后在下一轮重新注入

1. **Hindsight 风格记忆基底**

- 区分观察到的 vs 相信的 vs 摘要的
- 支持 retain/recall/reflect
- 带置信度的观点，可随证据演化
- 实体感知检索 + 时间查询（即使没有完整知识图谱）

## 方案架构（Markdown 真相来源 + 派生索引）

### 规范存储（git 友好）

保持 `~/.openacosmi/workspace` 作为规范的人类可读记忆。

建议的工作区布局：

```
~/.openacosmi/workspace/
  memory.md                    # 小文件：持久事实 + 偏好（核心级）
  memory/
    YYYY-MM-DD.md              # 每日日志（追加；叙事体）
  bank/                        # "类型化"记忆页面（稳定、可审查）
    world.md                   # 关于世界的客观事实
    experience.md              # Agent 做了什么（第一人称）
    opinions.md                # 主观偏好/判断 + 置信度 + 证据指针
    entities/
      Peter.md
      The-Castle.md
      warelay.md
      ...
```

备注：

- **每日日志保持为每日日志**。无需转换为 JSON。
- `bank/` 文件是**精心策划的**，由反思任务生成，仍可手动编辑。
- `memory.md` 保持"小 + 核心级"：你希望 Clawd 在每次会话中都能看到的内容。

### 派生存储（机器回忆）

在工作区下添加派生索引（不一定纳入 git 跟踪）：

```
~/.openacosmi/workspace/.memory/index.sqlite
```

底层支持：

- SQLite Schema 用于事实 + 实体链接 + 观点元数据
- SQLite **FTS5** 用于词法回忆（快速、体积小、离线）
- 可选嵌入表用于语义回忆（仍为离线）

索引始终**可从 Markdown 重建**。

## Retain / Recall / Reflect（操作循环）

### Retain：将每日日志规范化为"事实"

Hindsight 的关键洞察：存储**叙事性、自包含的事实**，而非小片段。

`memory/YYYY-MM-DD.md` 的实践规则：

- 在一天结束时（或期间），添加 `## Retain` 部分，包含 2–5 条要点：
  - 叙事性（保留跨轮上下文）
  - 自包含（独立阅读有意义）
  - 标注类型 + 实体提及

示例：

```
## Retain
- W @Peter: 目前在马拉喀什（2025年11月27日–12月1日），参加 Andy 的生日。
- B @warelay: 我通过将 connection.update handler 包装在 try/catch 中修复了 Baileys WS 崩溃（见 memory/2025-11-27.md）。
- O(c=0.95) @Peter: 在 WhatsApp 上偏好简洁回复（<1500 字符）；长内容放入文件。
```

最小化解析：

- 类型前缀：`W`（世界）、`B`（经历/传记）、`O`（观点）、`S`（观察/摘要；通常生成的）
- 实体：`@Peter`、`@warelay` 等（slug 映射到 `bank/entities/*.md`）
- 观点置信度：`O(c=0.0..1.0)` 可选

如果不想让作者思考这些：反思任务可从日志的其余部分推断这些要点，但有显式的 `## Retain` 部分是最简单的"质量把关"。

### Recall：对派生索引的查询

Recall 应支持：

- **词法**："查找精确术语/名称/命令"（FTS5）
- **实体**："告诉我关于 X"（实体页面 + 实体关联事实）
- **时间**："11月27日前后发生了什么" / "上周以来"
- **观点**："Peter 偏好什么？"（含置信度 + 证据）

返回格式应 Agent 友好并标注来源：

- `kind`（`world|experience|opinion|observation`）
- `timestamp`（来源日期，或提取的时间范围）
- `entities`（`["Peter","warelay"]`）
- `content`（叙事性事实）
- `source`（`memory/2025-11-27.md#L12` 等）

### Reflect：生成稳定页面 + 更新信念

反思是定时任务（每日或心跳 `ultrathink`），执行：

- 从近期事实更新 `bank/entities/*.md`（实体摘要）
- 基于强化/矛盾更新 `bank/opinions.md` 的置信度
- 可选地提议编辑 `memory.md`（"核心级"持久事实）

观点演化（简单、可解释）：

- 每个观点包含：
  - 陈述
  - 置信度 `c ∈ [0,1]`
  - last_updated
  - 证据链接（支持 + 矛盾事实 ID）
- 当新事实到达时：
  - 通过实体重叠 + 相似度找到候选观点（先 FTS，后嵌入）
  - 通过小增量更新置信度；大幅跳跃需要强矛盾 + 重复证据

## CLI 集成：独立 vs 深度集成

建议：**深度集成到 OpenAcosmi**，但保持可分离的核心库。

### 为何集成到 OpenAcosmi？

- OpenAcosmi 已知道：
  - 工作区路径（`agents.defaults.workspace`）
  - 会话模型 + 心跳
  - 日志 + 故障排查模式
- 你希望 Agent 本身调用工具：
  - `openacosmi memory recall "…" --k 25 --since 30d`（Rust CLI 命令）
  - `openacosmi memory reflect --since 7d`（Rust CLI 命令）

### 为何仍要分离库？

- 保持记忆逻辑可测试，无需 Gateway/运行时
- 可从其他上下文复用（本地脚本、未来桌面应用等）

方案概要：
记忆工具计划作为 Rust CLI 层的小型命令 + 库，但这仅为探索性设计。Go Gateway 负责后端反思任务调度和存储管理。

## "S-Collide" / SuCo：何时使用（研究）

如果 "S-Collide" 指 **SuCo（Subspace Collision）**：这是一种 ANN 检索方法，通过在子空间中使用学习的/结构化的碰撞来实现强召回率/延迟权衡（论文：arXiv 2411.14754，2024）。

`~/.openacosmi/workspace` 的实用建议：

- **不要一开始**就使用 SuCo。
- 从 SQLite FTS + （可选的）简单嵌入开始；你会立即获得大部分用户体验收益。
- 仅在以下情况下考虑 SuCo/HNSW/ScaNN 级解决方案：
  - 语料库很大（数万/数十万块）
  - 暴力嵌入搜索变得太慢
  - 词法搜索明显成为召回质量瓶颈

离线友好的替代方案（按复杂度递增）：

- SQLite FTS5 + 元数据过滤（零 ML）
- 嵌入 + 暴力搜索（如果块数少，效果出奇地好）
- HNSW 索引（常见、稳健；需要库绑定）
- SuCo（研究级别；如果有可嵌入的可靠实现则很有吸引力）

待讨论：

- 在你的机器（笔记本 + 台式机）上，"个人助手记忆"的**最佳**离线嵌入模型是什么？
  - 如果已有 Ollama：用本地模型嵌入；否则在工具链中附带一个小型嵌入模型。

## 最小可用试点

如果你想要一个最小但仍有用的版本：

- 添加 `bank/` 实体页面和每日日志中的 `## Retain` 部分。
- 使用 SQLite FTS 进行带引用的回忆（路径 + 行号）。
- 仅在召回质量或规模需要时添加嵌入。

## 参考

- Letta / MemGPT 概念："核心记忆块" + "归档记忆" + 工具驱动的自编辑记忆。
- Hindsight 技术报告："retain / recall / reflect"，四网络记忆，叙事事实提取，观点置信度演化。
- SuCo：arXiv 2411.14754（2024）："Subspace Collision" 近似最近邻检索。
