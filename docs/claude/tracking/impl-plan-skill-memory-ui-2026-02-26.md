---
document_type: Tracking
status: Archived
created: 2026-02-26
last_updated: 2026-02-26
audit_report: docs/claude/audit/audit-2026-02-26-skill-import-memory-ui.md
skill5_verified: true
---

# Plan: 技能文档 L0/L1/L2 分级存储 + 记忆管理 UI 入口

## Context

UHMS 记忆系统的核心价值是分级加载省 token：L0（~100 tokens 摘要）→ L1（~2K 概述）→ L2（全文）。
当前技能文档（SKILL.md）以全文注入 prompt，无分级机制。
本计划实现两件事：
1. **后端**: 技能文档 → UHMS 批量迁移 RPC，自动生成 L0/L1/L2
2. **前端**: 将 Chat 页面脑袋图标改为记忆管理入口 + 创建记忆管理页面

---

## Part A: 后端 — 技能迁移 RPC

### 修改文件

| 文件 | 变更 |
|------|------|
| `internal/gateway/server_methods_memory.go` | 新增 `memory.import.skills` handler |
| `internal/gateway/server_methods.go` | writeMethods 新增 `memory.import.skills` |
| `internal/memory/uhms/manager.go` | 新增 `ImportSkill()` 方法 |

### A1: manager.go — ImportSkill 方法

```go
func (m *DefaultManager) ImportSkill(ctx context.Context, userID, skillName, description, fullContent string) (*Memory, bool, error)
```

- 先用 `SearchByFTS5(userID, skillName, 1)` 检查是否已导入
- 已存在且内容相同 → 跳过，返回 (existing, false, nil)
- 已存在但内容变化 → 更新 (update store + rewrite VFS)
- 不存在 → 调用 `AddMemory(ctx, userID, fullContent, MemTypeProcedural, CatSkill)`
- 返回 (memory, isNew, error)

**L0/L1/L2 自动生成**（复用现有 `writeVFS`）：
- L0 = truncate(fullContent, 200) — 技能摘要
- L1 = truncate(fullContent, 2000) — 技能概述
- L2 = fullContent — 完整 SKILL.md

### A2: server_methods_memory.go — memory.import.skills handler

```go
func handleMemoryImportSkills(ctx *MethodHandlerContext)
```

- params: `{ userId? }`
- 流程:
  1. `skills.LoadSkillEntries(cfg)` 加载所有技能
  2. 遍历 skillEntries，对每个: `mgr.ImportSkill(userID, skill.Name, skill.Description, skill.Content)`
  3. 统计 imported / skipped / updated / failed
- response: `{ imported, skipped, updated, failed, total, skills: [{name, id, status}] }`

### A3: 授权注册

writeMethods 新增 `"memory.import.skills"`

---

## Part B: 前端 — 记忆管理 UI

### 修改文件

| 文件 | 变更 |
|------|------|
| `ui/src/ui/navigation.ts` | 新增 `"memory"` Tab + 路由 + 图标 |
| `ui/src/ui/icons.ts` | 新增 `memoryChip` 图标（芯片/记忆棒风格） |
| `ui/src/ui/app-render.helpers.ts` | 脑袋图标 → 导航到 /memory |
| `ui/src/ui/controllers/memory.ts` | **新建** — RPC 调用逻辑 |
| `ui/src/ui/views/memory.ts` | **新建** — 记忆管理页面渲染 |
| `ui/src/ui/app-view-state.ts` | 新增 memory 相关状态属性 |
| `ui/src/ui/app.ts` | 新增 @state() 属性 |
| `ui/src/ui/app-render.ts` | 新增 memory tab 渲染分支 |
| `ui/src/ui/locales/en.ts` | 新增英文文案 |
| `ui/src/ui/locales/zh.ts` | 新增中文文案 |

### B1: navigation.ts — 新增 memory Tab

- `Tab` 类型新增 `| "memory"`
- `TAB_PATHS` 新增 `memory: "/memory"`
- `iconForTab` 新增 `case "memory": return "memoryChip"`
- `getTabGroups` 中 agent 分组新增 `"memory"`: `["agents", "skills", "memory", "nodes"]`

### B2: icons.ts — 新增记忆图标

使用 database/chip 风格的 SVG（三层堆叠表示 L0/L1/L2），醒目且直观。

### B3: app-render.helpers.ts — 脑袋图标改造

将 Chat 控制栏中的 brain toggle 改为：
- 图标换成 `icons.memoryChip`
- 点击事件改为 `state.tab = "memory"` 导航到记忆页面
- title 改为 "记忆管理" / "Memory Management"

### B4: controllers/memory.ts — 控制器

```typescript
export type MemoryState = {
  client: GatewayBrowserClient | null;
  connected: boolean;
  memoryLoading: boolean;
  memoryList: MemoryItem[] | null;
  memoryTotal: number;
  memoryError: string | null;
  memoryDetail: MemoryDetail | null;
  memoryStatus: MemoryStatus | null;
  memoryImporting: boolean;
  memoryImportResult: ImportResult | null;
};
```

函数:
- `loadMemoryStatus(state)` → `memory.uhms.status`
- `loadMemoryList(state, opts?)` → `memory.list`
- `loadMemoryDetail(state, id, level)` → `memory.get`
- `deleteMemory(state, id)` → `memory.delete`
- `importSkills(state)` → `memory.import.skills`

### B5: views/memory.ts — 记忆管理页面

布局（3 个 card）:

**Card 1: 状态概览**
- UHMS 启用状态、记忆总数、磁盘占用、向量模式
- "导入技能" 按钮 + 导入结果显示

**Card 2: 记忆列表**
- 分页表格: ID(短)、内容摘要、类型、分类、重要性、访问次数、创建时间
- 类型/分类筛选下拉
- 点击行展开详情
- 删除按钮

**Card 3: 记忆详情** (点击列表项展开)
- 元数据面板
- L0 / L1 / L2 三个 tab 切换查看内容
- 渐进加载: 默认显示 L0，点击 L1/L2 按需加载

---

## Online Verification Log

### LitElement SPA 状态管理模式
- **Query**: LitElement lit-html state management pattern page navigation SPA 2025
- **Source**: https://lit.dev/docs/components/properties/
- **Key finding**: @state() 装饰器 + reactive properties 是 Lit 标准模式；本项目已使用此模式（app.ts 100+ @state 属性）
- **Verified date**: 2026-02-26

### 分级记忆系统 LLM 优化
- **Query**: hierarchical memory system L0 L1 L2 tiered loading LLM context optimization
- **Sources**:
  - H-MEM (arxiv 2507.22925): 多级语义抽象，按需加载
  - MemGPT (arxiv 2310.08560): OS 启发的虚拟上下文管理
  - RGMem: L0 微观证据 → L1 多尺度演化 → L2 规模感知检索
- **Key finding**: 分级加载是 2025 LLM 记忆系统的主流模式，L0/L1/L2 设计与学术最佳实践一致
- **Verified date**: 2026-02-26

---

## 验证步骤

1. `cd backend && go build ./internal/...` — 编译通过
2. `cd backend && go vet ./internal/...` — 静态检查
3. 启动 gateway，用 Go 测试脚本验证:
   - `memory.import.skills` → 应返回 imported > 0
   - `memory.list` 过滤 `type=procedural, category=skill` → 应返回导入的技能
   - `memory.get` 取一条 → level=0/1/2 内容长度应递增
4. `cd ui && npm run build` — 前端编译
5. 浏览器打开 UI:
   - Chat 页面脑袋图标 → 应跳转到 /memory
   - 记忆管理页面 → 状态卡片显示
   - 点击"导入技能" → 技能列表出现
   - 点击记忆 → L0/L1/L2 内容展示
