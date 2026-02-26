---
document_type: Audit
status: Final
created: 2026-02-26
last_updated: 2026-02-26
---

# Audit: writeVFS 异步 LLM 生成 L0/L1 摘要

## Scope

### 修改文件 (2 文件)

- `internal/memory/uhms/manager.go` — 修改 `writeVFS()` + 新增 `upgradeVFSSummary()` + `generateMemorySummary()`
- `internal/memory/uhms/vfs.go` — 新增 `WriteL0L1()` 方法

### 审计覆盖

- 安全性 (路径遍历, prompt 注入, 信息泄露)
- 资源安全 (goroutine 生命周期, 内存持有, 文件句柄)
- 并发安全 (竞态, 锁语义, 闭包捕获)
- 正确性 (降级路径, 边界条件, 一致性模型)

---

## Findings

### F-01 [INFO] LLM 输出无长度边界保护

**Location**: `manager.go:756,781`

**Analysis**: `generateMemorySummary` 中 `strings.TrimSpace(l0Result)` / `strings.TrimSpace(l1Result)` 直接写入文件，不做长度截断。如果 LLM 不遵守 prompt 指令返回超长文本（如 L0 返回 500 词而非 50 词），会导致 L0 失去"极短摘要"的分级意义，`BuildContextBlock` 中 L0 token 估算偏差。

**Risk**: LOW — prompt 指令通常被遵守；即使超长，仅影响 token 估算精度，不影响功能正确性。

**Recommendation**: 可选加安全网 `l0 = truncate(l0, 300)` / `l1 = truncate(l1, 3000)`。当前可接受。

---

### F-02 [INFO] 短内容仍触发 LLM 调用

**Location**: `manager.go:707-712`

**Analysis**: 当 `fullContent` 短于 200 字符时，`truncate(content, 200)` 返回原文，L0 已是完整内容。此时异步 LLM 调用生成的"摘要"可能与原文几乎相同，浪费一次 API 调用。

**Risk**: INFO — 仅影响成本效率，不影响正确性。短内容记忆数量通常较少。

**Recommendation**: 可选加快速路径 `if len([]rune(fullContent)) <= 200 { return }` 跳过异步。

---

### F-03 [INFO] 异步 LLM 调用无 context 超时

**Location**: `manager.go:711`

**Analysis**: `context.Background()` 无超时限制。如果 LLM provider 挂起，goroutine 永远阻塞。但这与现有模式一致：`indexVector`（manager.go:179）、`memory_extraction`（manager.go:348）均用 `context.Background()`。

**Risk**: INFO — 一致性选择。LLM provider 自身通常有 HTTP 超时（30-60s）。

**Recommendation**: 全局改进项：未来统一为 `context.WithTimeout(context.Background(), 60*time.Second)`。

---

### F-04 [INFO] Delete 和 async upgrade 的竞态窗口

**Location**: `manager.go:710-712` vs `manager.go:492`

**Analysis**: 时间线：`AddMemory` → `writeVFS` Phase 1 → safeGo 启动 → 用户立刻调用 `DeleteMemory` → VFS 目录被删除 → safeGo 中 `WriteL0L1` 尝试写已删除目录的文件 → `writeFile` 返回 error → `slog.Warn` 记录。

**Risk**: INFO — 错误被正确捕获和日志记录，不会 panic 或数据损坏。日志 Warn 是预期行为。

---

### F-05 [INFO] `memCopy := *mem` 浅拷贝足够性

**Location**: `manager.go:709`

**Analysis**: `Memory` struct 包含 `string` 字段（ID, UserID, Content, VFSPath）和值类型（MemoryType, MemoryCategory, float64, time.Time）。Go 中 string 是不可变引用，浅拷贝即安全。`upgradeVFSSummary` 仅读取 `mem.ID`/`mem.MemoryType`/`mem.Category`，不写入。竞态安全。

**Risk**: INFO — 确认正确。

---

## Correctness Analysis

### 降级路径覆盖

| 场景 | 行为 | 正确性 |
|---|---|---|
| `m.llm == nil` | 无异步触发，纯截断 | ✅ 与原行为一致 |
| L0 LLM 失败 | 返回 `("", "")` → `upgradeVFSSummary` 直接 return | ✅ 保留截断版 |
| L1 LLM 失败 | 返回 `(l0, truncate(content, 2000))` → 覆写 L0 + 截断 L1 | ✅ 部分升级 |
| WriteL0L1 失败 | slog.Warn，保留 Phase 1 截断版 | ✅ 优雅降级 |
| safeGo panic | panic recovery + Error 日志 + stack trace | ✅ 不崩进程 |

### 并发安全

| 访问 | 保护 | 正确性 |
|---|---|---|
| Phase 1 `WriteMemory` | `vfs.mu.Lock()` | ✅ |
| Phase 2 `WriteL0L1` | `vfs.mu.Lock()` | ✅ |
| 闭包捕获 `mem` | `memCopy := *mem` 值副本 | ✅ |
| 闭包捕获 `userID`, `fullContent` | Go string 不可变 | ✅ |
| `m.llm` 读取 | 初始化后不变（`NewManager` 设置） | ✅ |

### 调用链完整性

```
AddMemory (manager.go:175) ──→ writeVFS()
ImportSkill (manager.go:602) ──→ writeVFS()
  ├── Phase 1: vfs.WriteMemory() [同步]
  └── Phase 2: safeGo → upgradeVFSSummary() → generateMemorySummary() → vfs.WriteL0L1() [异步]
```

两个入口均受益，无遗漏。

---

## Verdict

**PASS** — 5 findings (0 CRITICAL, 0 HIGH, 0 MEDIUM, 1 LOW, 4 INFO)

代码正确实现 Write-Then-Upgrade 异步覆写模式。降级路径完备，并发安全，与现有 `safeGo` / `generateArchiveSummary` 模式一致。所有 findings 均为可选优化项，不影响功能正确性。
