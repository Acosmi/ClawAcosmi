# 技能系统架构文档

> **版本**: V1 | **更新**: 2026-02-25 | **状态**: In Progress
> **模块**: `backend/internal/agents/skills/` + `docs/skills/`

---

## 一、架构总览

OpenAcosmi 技能系统采用 **文件系统驱动 + 自动发现** 模式，将技能定义为 `SKILL.md` 文件，
通过多源扫描 → 去重 → 格式化 → 注入 LLM Prompt 的链路，让 Agent 感知并调用可用技能。

```
┌─────────────────────────────────────────────────────┐
│                  技能来源（5 级优先级）                │
│                                                     │
│  P1  .agent/skills/{name}/SKILL.md    (workspace)   │
│  P2  managedDir/{name}/SKILL.md       (managed)     │
│  P3  bundledDir/{name}/SKILL.md       (bundled)     │
│  P4  extraDirs/{name}/SKILL.md        (config)      │
│  P5  docs/skills/{cat}/{name}/SKILL.md (docs)  ← 新 │
│                                                     │
└──────────────────┬──────────────────────────────────┘
                   │
                   ▼
         ┌─────────────────┐
         │ LoadSkillEntries │ ← 全量扫描
         └────────┬────────┘
                  │
                  ▼
         ┌─────────────────┐
         │deduplicateEntries│ ← 同名先到先得
         └────────┬────────┘
                  │
                  ▼
         ┌─────────────────┐
         │filterSkillEntries│ ← 配置级禁用过滤
         └────────┬────────┘
                  │
                  ▼
    ┌──────────────────────────┐
    │ BuildWorkspaceSkillSnapshot │
    │  → formatSkillsForPrompt   │
    │  → SkillSnapshot.Prompt    │
    └─────────────┬──────────────┘
                  │
                  ▼
    ┌──────────────────────────┐
    │  BuildAgentSystemPrompt  │
    │  BuildParams.SkillsPrompt│ ← LLM 可见
    └──────────────────────────┘
```

---

## 二、目录结构

```
docs/skills/                          # 技能根目录
├── tools/                            # 工具类技能 (23)
│   ├── browser/SKILL.md
│   ├── exec/SKILL.md
│   ├── firecrawl/SKILL.md
│   └── ...
├── providers/                        # 供应商类技能 (20)
│   ├── anthropic/SKILL.md
│   ├── openai/SKILL.md
│   ├── ollama/SKILL.md
│   └── ...
├── general/                          # 通用类技能 (10)
│   ├── date-time/SKILL.md
│   ├── network/SKILL.md
│   ├── tts/SKILL.md
│   └── ...
└── official/                         # Claude 官方技能 (16)
    ├── pdf/SKILL.md
    ├── docx/SKILL.md
    ├── mcp-builder/SKILL.md
    └── ...
```

**总计**: 69 个技能（23 + 20 + 10 + 16）

### SKILL.md 格式

```yaml
---
name: browser
description: "Integrated browser control service + action commands"
---
# 技能正文内容（Markdown）
...
```

- `name:` — 技能唯一标识（同目录名）
- `description:` — 摘要，显示在 LLM prompt + 技能状态 API

---

## 三、后端实现

### 3.1 核心文件

| 文件 | 职责 |
|------|------|
| `skills/workspace_skills.go` | `LoadSkillEntries` 全量扫描 + `ResolveDocsSkillsDir` 自动发现 + `deduplicateEntries` 去重 |
| `skills/frontmatter.go` | Frontmatter 解析 + OpenAcosmi 元数据 + 调用策略 |
| `skills/bundled_dir.go` | Bundled skills 目录定位（多策略） |
| `runner/attempt_runner.go` | `buildSystemPrompt` 注入 `SkillsPrompt` 到 LLM |
| `gateway/server_methods_skills.go` | `skills.status` API + source="docs" 标识 |

### 3.2 自动发现算法 (`ResolveDocsSkillsDir`)

```
从 workspaceDir 向上遍历（最多 3 层）
  → 寻找 docs/skills/ 目录
  → 找到后扫描其一级子目录（tools/, providers/, general/, official/）
  → 对每个子目录调用 loadSkillsFromDir()
  → 每个 {name}/SKILL.md 产生一个 SkillEntry
```

### 3.3 优先级与去重

同名技能先到先得，顺序：

```
workspace (.agent/skills/)     — 优先级最高，开发者自定义覆盖
managed (managedDir)           — 平台管理
bundled (bundledDir)           — 内置捆绑
extraDirs (config)             — 配置扩展
docs/skills/ (自动发现)         — 优先级最低，不覆盖任何手动定义
```

### 3.4 SkillsPrompt 注入

```go
// attempt_runner.go → buildSystemPrompt()
snap := skills.BuildWorkspaceSkillSnapshot(...)
bp := prompt.BuildParams{
    SkillsPrompt: snap.Prompt,  // "Available skills:\n- browser: ...\n- exec: ..."
}
prompt.BuildAgentSystemPrompt(bp)  // → LLM system prompt 包含技能列表
```

### 3.5 状态 API

`skills.status` RPC 返回每个技能的 source 字段：

| source | 含义 |
|--------|------|
| `workspace` | .agent/skills/ 目录 |
| `bundled` | bundledDir 内置 |
| `docs` | docs/skills/ 自动发现 |

---

## 四、安全策略 — 信任边界模型

### 4.1 设计决策

**OpenAcosmi 不重复实现 4 重安全防护。**

技能安全审查由上游 **chat 系统（nexus-v4）** 的 4 层纵深防御承担：

| 层级 | 名称 | 位置 | 职责 |
|:----:|:----:|:----:|:-----|
| L1 | 输入防御 | nexus-v4 `skill_validator.go` | Prompt Injection 检测、XSS 扫描、URL/ZIP 安全、安全评分 |
| L2 | 模型约束 | nexus-v4 `skill_llm_filter.go` | 输出分类、敏感数据遮蔽、格式验证 |
| L3 | 沙箱桥接 | nexus-v4 `skill_sandbox_bridge.go` | SecurityLevel→SandboxPolicy 映射、隔离执行 |
| L4 | 运行时防御 | nexus-v4 `skill_runtime_guard.go` | 速率限制、异常检测、审计日志、人类审批 |

**全链路**: `请求 → [L1 输入] → [L4 速率/审批] → [L3 沙箱] → [L2 输出过滤] → [L4 审计] → 响应`

### 4.2 信任边界

```
┌──────────────────────────────────────────┐
│         nexus-v4 (Chat 系统)              │
│                                          │
│  用户提交技能 → L1 输入扫描               │
│              → L4 速率/异常检测            │
│              → L3 沙箱隔离执行             │
│              → L2 输出安全过滤             │
│              → L4 审计日志                │
│              → ✅ 审核通过                 │
│                    │                     │
└────────────────────┼─────────────────────┘
                     │ 已审核技能
                     ▼
┌──────────────────────────────────────────┐
│       OpenAcosmi (Agent 执行引擎)         │
│                                          │
│  docs/skills/ ← 已审核技能落盘            │
│  .agent/skills/ ← 开发者本地技能          │
│                                          │
│  LoadSkillEntries → SkillsPrompt → LLM  │
│                                          │
│  信任假设:                                │
│  - docs/skills/ 内容已通过 chat 端 4 重审核│
│  - .agent/skills/ 为开发者自信任内容       │
│  - bundled skills 为平台预审内容          │
└──────────────────────────────────────────┘
```

### 4.3 理由

| 考量 | 选择 |
|------|------|
| 避免重复建设 | 4 层防护代码只维护在 nexus-v4 一份 |
| 职责单一 | nexus-v4 = 审核网关, OpenAcosmi = 执行引擎 |
| 信任模型清晰 | 已审核 vs 开发者自信任 vs 平台预审 |
| 攻击面缩小 | OpenAcosmi 不直接暴露技能创建/上传接口 |
| 维护成本 | 安全规则升级只需改 nexus-v4 |

### 4.4 OpenAcosmi 侧的最小防护

虽不重复 4 层防护，OpenAcosmi 仍保留以下安全机制：

| 机制 | 位置 | 说明 |
|------|------|------|
| 执行安全级别 | `attempt_runner.go` | deny / allowlist / full 三级权限 |
| 命令规则 | `exec-approvals.json` | 白名单/黑名单命令过滤 |
| 权限审批 | `WaitForApproval` 回调 | 高危操作需用户确认 |
| 连续拒绝断路 | `maxConsecutivePermDeniedRounds` | 3 轮连续拒绝自动停止 |
| 沙箱隔离 | `oa-sandbox` (Rust) | 原生沙箱（Seatbelt/Landlock/AppContainer） |

---

## 五、技能管理操作

### 5.1 新增技能

```bash
# 在对应分类下创建目录 + SKILL.md
mkdir docs/skills/tools/my-new-tool
cat > docs/skills/tools/my-new-tool/SKILL.md << 'EOF'
---
name: my-new-tool
description: "My new tool description"
---
# 技能内容
...
EOF
```

重启 Gateway 或下次 LLM 调用时自动发现。

### 5.2 删除技能

```bash
rm -rf docs/skills/tools/my-new-tool
```

### 5.3 禁用技能（不删除）

在 `openacosmi.config.json` 中：

```json
{
  "skills": {
    "entries": {
      "my-new-tool": { "enabled": false }
    }
  }
}
```

### 5.4 覆盖 docs 技能

在 `.agent/skills/` 中创建同名技能，自动覆盖 docs/ 版本（优先级更高）。

---

## 六、Argus 视觉子智能体技能

**当前状态**: 通过代码硬编码注入（`argus.BuildArgusSkillEntries`），不走 SKILL.md 文件系统。

**迁移计划**: 见 `docs/claude/deferred/argus-skill-file-migration.md`

---

## 七、数据统计

| 指标 | 数值 |
|------|------|
| 技能总数 | 69 |
| 分类数 | 4 (tools / providers / general / official) |
| 后端修改文件 | 3 (workspace_skills.go / attempt_runner.go / server_methods_skills.go) |
| 新增函数 | 2 (ResolveDocsSkillsDir / deduplicateEntries) |
| 修改函数 | 3 (LoadSkillEntries / buildSystemPrompt / handleSkillsStatus) |
| 测试 | 5 pass (原有测试全通过) |

---

## 八、延迟待办

| ID | 描述 | 优先级 | 跟踪 |
|:--:|:-----|:------:|:----:|
| SK-1 | Argus 技能迁移至 docs/skills/argus/ | P3 | `deferred/argus-skill-file-migration.md` |
| SK-2 | chat 端审核通过后自动同步到 docs/skills/ | P2 | — |
| SK-3 | 技能热加载（文件 watcher，无需重启） | P3 | — |
| SK-4 | 前端技能管理页面适配 source="docs" 分组 | P2 | — |
| SK-5 | SKILL.md 渐进式加载（启动只读 frontmatter，调用时才读全文） | P3 | — |
