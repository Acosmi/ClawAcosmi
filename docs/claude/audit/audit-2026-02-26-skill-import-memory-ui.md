---
document_type: Audit
status: Final
created: 2026-02-26
last_updated: 2026-02-26
---

# Audit: 技能导入 RPC + 记忆管理 UI

## Scope

### Part A: 后端 (3 文件)

- `internal/memory/uhms/manager.go` — 新增 `ImportSkill()` 方法 + `ImportSkillResult` 类型
- `internal/gateway/server_methods_memory.go` — 新增 `handleMemoryImportSkills` handler
- `internal/gateway/server_methods.go` — writeMethods 新增 `"memory.import.skills"` 授权

### Part B: 前端 (10 文件)

- `ui/src/ui/navigation.ts` — 新增 `"memory"` Tab + 路由 + 图标
- `ui/src/ui/icons.ts` — 新增 `memoryChip` 图标
- `ui/src/ui/app-render.helpers.ts` — brain 图标替换为 memoryChip + 导航
- `ui/src/ui/controllers/memory.ts` — **新建** 5 个控制器函数
- `ui/src/ui/views/memory.ts` — **新建** 3 卡片记忆管理页面
- `ui/src/ui/app-view-state.ts` — 新增 15 个 memory 状态属性
- `ui/src/ui/app.ts` — 新增 15 个 `@state()` 属性
- `ui/src/ui/app-render.ts` — 新增 memory tab 渲染分支
- `ui/src/ui/app-settings.ts` — `refreshActiveTab()` 新增 memory 标签加载
- `ui/src/ui/locales/en.ts` + `zh.ts` — 新增 35 条国际化文案

---

## Findings

### F-01 [LOW] controllers/memory.ts 有未使用的 `t` import — **已修复**

**Location**: `controllers/memory.ts:2`

**Analysis**: 导入了 `import { t } from "../i18n.ts"` 但控制器中未使用。纯逻辑层不需要 i18n。

**Resolution**: 已删除未使用的 import。✅

---

### F-02 [LOW] app-render.helpers.ts 使用 `as any` 类型强转 — **已修复**

**Location**: `app-render.helpers.ts:163`

**Analysis**: `state.setTab("memory" as any)` 使用了 `as any` 绕过类型检查。
但 `Tab` 类型已正确扩展包含 `"memory"`，`setTab(tab: Tab)` 签名无需强转。

**Resolution**: 已移除 `as any`，直接用 `state.setTab("memory")`。✅

---

### F-03 [INFO] ImportSkill FTS5 匹配可能误命中非技能记忆

**Location**: `manager.go:577`

**Analysis**: `SearchByFTS5(userID, skillName, 10)` 用 skillName 作为搜索词，
可能匹配到非 procedural/skill 类型的记忆（例如内容中偶然包含技能名的 episodic 记忆）。
但代码在 L582 过滤 `mem.MemoryType != MemTypeProcedural || mem.Category != CatSkill`，
只处理精确类型匹配。多余结果被跳过不影响正确性。

**Risk**: LOW — FTS5 是宽松匹配但后续有类型过滤，功能正确。性能影响可忽略（limit=10）。

**Recommendation**: 无需修改。如需精确，可在 FTS5 查询中加 `memory_type:procedural` 条件。

---

### F-04 [INFO] handleMemoryImportSkills 同步遍历可能耗时较长

**Location**: `server_methods_memory.go:362-394`

**Analysis**: 对所有技能条目（69+）逐个调用 `mgr.ImportSkill()`，每次涉及 FTS5 查询 +
可能的 VFS 读写。在首次导入时，69 个技能每个需创建 SQLite 行 + 写 3 个 VFS 文件。

**Risk**: LOW — WebSocket 请求无 HTTP 超时限制。首次导入可能需要 2-5 秒，
但只运行一次（后续调用大部分 skipped）。Gateway WebSocket 层无消息超时保护。

**Recommendation**: 当前可接受。如需优化，可加 batch 写入或异步流式返回。

---

### F-05 [INFO] 前端 confirm() 可能被浏览器拦截

**Location**: `views/memory.ts:205`

**Analysis**: 删除确认使用原生 `confirm()` 对话框。部分浏览器/嵌入场景可能阻止弹窗。

**Risk**: LOW — 与其他页面（如 sessions）的删除确认模式一致。

**Recommendation**: 无需修改。后续可统一迁移到自定义 `showConfirmDialog` 组件。

---

### F-06 [INFO] VFS content 在 `<pre>` 中渲染可能导致超长内容

**Location**: `views/memory.ts:300`

**Analysis**: L2 全文可能很长（完整 SKILL.md），直接在 `<pre>` 中渲染不做截断。
Lit 的 `${...}` 在 text node 中自动 HTML 转义，无 XSS 风险。

**Risk**: LOW — 纯 UI 体验问题，不影响安全。

**Recommendation**: 可选加 `max-height + overflow-y: auto` CSS 限制显示高度。

---

### F-07 [INFO] brain icon 替换移除了 chatShowThinking toggle 功能

**Location**: `app-render.helpers.ts:160-168`

**Analysis**: 原 brain 图标既显示 thinking 状态又切换 `chatShowThinking`。
替换后该功能入口消失。用户无法在 Chat 页面直接切换 thinking 显示。

**Risk**: LOW — Thinking toggle 仍可通过 sessions 页面或配置修改。
用户明确要求将此处改为记忆管理入口。

**Recommendation**: 接受。如需恢复 thinking toggle，可在 Chat 控制栏另增一个按钮。

---

## Security Checklist

- [x] 输入校验: skillName/fullContent 非空检查 ✅
- [x] 授权: `memory.import.skills` 注册到 writeMethods ✅
- [x] nil-safe: handler 首行检查 `mgr == nil` + `loader == nil` ✅
- [x] XSS 防护: Lit html`` 模板自动转义文本插值 ✅
- [x] 所有权: ImportSkill 不涉及跨用户操作，复用 AddMemory 内部逻辑 ✅
- [x] 配置加载: 复用 `ConfigLoader.LoadConfig()` 标准路径 ✅
- [x] 技能加载: 复用 `skills.LoadSkillEntries()` 与 `skills.status` 一致 ✅

## Resource Safety Checklist

- [x] 无文件句柄泄漏（VFS 写入由 writeVFS 内部管理）
- [x] 无 goroutine 泄漏（向量索引异步写用 safeGo）
- [x] context.Background() 与现有 handler 一致
- [x] 前端无内存泄漏（controller 函数无 setInterval/addEventListener）

## Compilation Verification

- [x] `go build ./internal/...` — 通过
- [x] `go vet ./internal/...` — 通过
- [x] `cd ui && npm run build` — 通过 (141 modules, 700.70 kB JS)

## Verdict

**PASS** — 2 个 LOW 发现已修复 (F-01, F-02)，5 个 INFO 发现均不阻塞。
代码质量与现有 `server_methods_memory.go` / `controllers/sessions.ts` / `views/sessions.ts` 保持一致。
