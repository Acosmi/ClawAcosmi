# 工具-技能绑定架构（Tool-Skill Binding）

> **版本**: V1 | **更新**: 2026-03-01 | **状态**: Active
> **模块**: `backend/internal/agents/skills/` + `backend/internal/agents/runner/`

---

## 一、工具-技能绑定概述

**核心问题**: 工具（`ToolDef`）和技能（`SKILL.md`）完全解耦。LLM 必须先调用 `search_skills` → `lookup_skill` 才能获取工具使用指南，浪费 token 且经常被跳过。

**解决方案**: 双路绑定 + 描述增强。技能通过 frontmatter `tools:` 字段声明关联的工具名，系统在加载时构建索引，并将技能描述追加到工具 Description 末尾。LLM 读工具定义时自动获得技能指引，无需额外搜索。

```
┌──────────────────────────────────────────────────────────────┐
│  SKILL.md                                                    │
│  ---                                                         │
│  name: exec                                                  │
│  description: "Exec tool usage, stdin modes..."              │
│  tools: bash                    ← 绑定声明                    │
│  ---                                                         │
└───────────────────────┬──────────────────────────────────────┘
                        │
                        ▼
┌──────────────────────────────────────────────────────────────┐
│  loadSkillsFromDir()                                         │
│  解析 frontmatter → SkillEntry.Metadata.Tools = ["bash"]     │
└───────────────────────┬──────────────────────────────────────┘
                        │
                        ▼
┌──────────────────────────────────────────────────────────────┐
│  ResolveToolSkillBindings(entries)                            │
│  构建映射: map["bash"] = "Exec tool usage, stdin modes..."   │
└───────────────────────┬──────────────────────────────────────┘
                        │
                        ▼
┌──────────────────────────────────────────────────────────────┐
│  buildToolDefinitions()                                      │
│  tools[i].Description += " [Skill: Exec tool usage...]"      │
│                                                              │
│  LLM 看到:                                                   │
│  bash: "Execute a bash command... [Skill: Exec tool usage,   │
│         stdin modes, and TTY support]"                        │
└──────────────────────────────────────────────────────────────┘
```

---

## 二、当前绑定清单

| 工具名 | 绑定技能 | SKILL.md 路径 | 技能描述 |
|--------|---------|--------------|---------|
| `bash` | exec | `docs/skills/tools/exec/SKILL.md` | Exec tool usage, stdin modes, and TTY support |
| `browser` | browser | `docs/skills/tools/browser/SKILL.md` | Integrated browser control service + action commands |
| `send_media` | send-media | `docs/skills/tools/send-media/SKILL.md` | 发送文件/媒体到远程频道（飞书/Discord/Telegram/WhatsApp） |
| `web_search` | web | `docs/skills/tools/web/SKILL.md` | Web search + fetch tools (Brave Search API, Perplexity direct/OpenRouter) |
| `spawn_coder_agent` | coder | `docs/skills/tools/coder/SKILL.md` | Programming sub-agent: 9-layer fuzzy edit, file tools, sandboxed bash |
| `spawn_argus_agent` | argus-visual | `docs/skills/tools/argus-visual/SKILL.md` | Argus visual sub-agent: screen perception + UI automation via argus_* tools |
| `search_skills` | skills | `docs/skills/tools/skills/SKILL.md` | 技能系统：加载路径、优先级、门控规则与配置 |
| `lookup_skill` | skills | `docs/skills/tools/skills/SKILL.md` | （同上，一个技能绑定两个工具） |
| `write_file` | apply-patch | `docs/skills/tools/apply-patch/SKILL.md` | Apply multi-file patches with the apply_patch tool |

**未绑定工具**（无对应技能或动态注册）:
- `read_file`、`list_dir` — 基础文件操作，无需额外指引
- `argus_*` — 动态前缀工具，由 `ArgusBridge.AgentTools()` 运行时注册
- `remote_*` — 远程 MCP 工具，由 `RemoteMCPBridge.AgentRemoteTools()` 运行时注册

---

## 三、如何新增工具-技能绑定

### 场景 A：已有技能，新增绑定

在 SKILL.md 的 frontmatter `---` 块内添加 `tools:` 行即可。

**示例**: 为已有的 `my-tool` 技能绑定工具 `my_tool_name`：

```yaml
---
name: my-tool
description: "My tool usage guide"
tools: my_tool_name
---
# 技能正文...
```

**多工具绑定**（逗号分隔）:

```yaml
---
name: skills
description: 技能系统：加载路径、优先级、门控规则与配置
tools: search_skills, lookup_skill
---
```

无需修改任何 Go 代码。下次 Gateway 启动或 LLM 调用时自动生效。

### 场景 B：新建技能并绑定工具

1. 创建技能目录和 SKILL.md:

```bash
mkdir -p docs/skills/tools/my-new-tool
```

2. 编写 SKILL.md（含 `tools:` 字段）:

```yaml
---
name: my-new-tool
description: "工具使用指南摘要（建议 ≤120 字符，超出会被截断）"
tools: tool_name_1, tool_name_2
---
# 工具使用指南正文

## 何时使用
...

## 参数说明
...

## 注意事项
...
```

3. 验证绑定生效: 重启 Gateway 后检查日志 `toolBindings` 计数。

### 场景 C：工具名与技能名不同

这是绑定系统的核心价值。`tools:` 字段值必须是 **工具的实际注册名**（即 `buildToolDefinitions()` 中的 `Name` 字段），而非技能目录名。

常见映射:

| 技能目录名 | 工具注册名 | 原因 |
|-----------|-----------|------|
| `exec` | `bash` | 技能叫 exec，工具叫 bash |
| `send-media` | `send_media` | 连字符 vs 下划线 |
| `coder` | `spawn_coder_agent` | 名称完全不同 |
| `argus-visual` | `spawn_argus_agent` | 名称完全不同 |

### 场景 D：同一工具被多个技能绑定

**先到先得**: `ResolveToolSkillBindings()` 对同一工具名只保留第一个绑定的技能描述。技能加载顺序由 `LoadSkillEntries()` 的 5 级优先级决定（workspace > managed > bundled > extraDirs > docs/skills）。

---

## 四、实现细节

### 4.1 frontmatter 解析

`tools:` 字段支持两种声明位置:

1. **直接 frontmatter**（推荐）: YAML frontmatter 的顶级字段

```yaml
---
name: exec
tools: bash
---
```

2. **metadata JSON 内**: `metadata` 字段的 `openacosmi.tools` 数组

```yaml
---
name: my-skill
metadata: |
  { "openacosmi": { "tools": ["tool_a", "tool_b"] } }
---
```

**优先级**: 直接 frontmatter `tools:` 优先于 metadata JSON 内的 `tools`。

### 4.2 核心函数

| 函数 | 文件 | 职责 |
|------|------|------|
| `loadSkillsFromDir()` | `workspace_skills.go` | 解析 SKILL.md frontmatter，填充 `SkillEntry.Metadata.Tools` |
| `ResolveToolSkillBindings()` | `workspace_skills.go` | 从 `[]SkillEntry` 构建 `map[string]string`（toolName → description） |
| `buildSystemPrompt()` | `attempt_runner.go` | 调用 `ResolveToolSkillBindings()` 填充 `r.toolBindings` |
| `buildToolDefinitions()` | `attempt_runner.go` | 遍历工具列表，命中绑定的追加 `[Skill: ...]` 后缀 |

### 4.3 描述截断

`ResolveToolSkillBindings()` 对技能描述做 120 字符截断:

```go
if len(desc) > 120 {
    desc = desc[:117] + "..."
}
```

目的: 控制工具 Description 增长，避免 token 浪费。每个绑定约增加 30-50 tokens。

### 4.4 Boot 模式 vs 文件扫描模式

工具绑定在**两种模式**下都会构建:

| 模式 | 技能索引 | 工具绑定 |
|------|---------|---------|
| Boot 模式（VFS） | 不扫描文件，LLM 用 `search_skills` 按需检索 | 仍从文件加载 entries 构建绑定（静态，<5ms） |
| 文件扫描模式 | 全量扫描，prompt 放紧凑索引 | 复用已加载 entries 构建绑定（零额外 I/O） |

### 4.5 注入格式

工具 Description 末尾追加:

```
[Skill: <技能描述>]
```

示例:

```
原始: "Execute a bash command in the workspace. Use for system operations..."
注入后: "Execute a bash command in the workspace. Use for system operations... [Skill: Exec tool usage, stdin modes, and TTY support]"
```

### 4.6 与意图过滤的关系

`filterToolsByIntent()` 按工具 **Name** 过滤，不受 Description 追加影响。绑定描述是纯信息性的，不改变工具的可用性或权限。

---

## 五、Token 预算分析

| 项目 | Token 估算 |
|------|-----------|
| 每个工具追加描述 | ~30-50 tokens |
| 当前 9 个工具有绑定 | ~300-450 total |
| 省去的 `search_skills` 调用 | -200~500 tokens/次 |
| 省去的 `lookup_skill` 调用 | -100~300 tokens/次 |
| **净影响** | 基本持平或净节省（首轮即省 1-2 次技能搜索） |

---

## 六、测试覆盖

| 测试函数 | 文件 | 验证内容 |
|---------|------|---------|
| `TestResolveToolSkillBindings` | `workspace_skills_test.go` | 基础绑定解析、多工具绑定、空描述跳过 |
| `TestResolveToolSkillBindings_TruncatesLongDescription` | 同上 | 120 字符截断 + 尾部 `...` |
| `TestResolveToolSkillBindings_FirstWins` | 同上 | 同名工具先到先得 |
| `TestLoadSkillsFromDir_ParsesToolsFromFrontmatter` | 同上 | 文件系统端到端: frontmatter → Metadata.Tools |

---

## 七、相关文档

| 文档 | 路径 | 关联 |
|------|------|------|
| 技能系统架构 | `docs/architecture/skill-system-architecture.md` | §3.5 工具绑定段落 |
| 工具总览 | `docs/tools/index.md` | 工具定义与配置 |
| 五维联动架构 | `docs/claude/goujia/arch-agent-execution-v2.md` | 意图过滤与工具注入 |
