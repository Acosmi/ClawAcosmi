---
document_type: Tracking
status: Archived
created: 2026-02-26
last_updated: 2026-02-26
audit_report: docs/claude/audit/audit-2026-02-26-vfs-async-llm-summary.md
skill5_verified: true
---

# Plan: writeVFS 异步 LLM 生成真正的 L0/L1 摘要

## Context

当前 `writeVFS()` 用 `truncate(content, 200)` / `truncate(content, 2000)` 生成 L0/L1——只是简单截断，不是语义摘要。
本次实现 Write-Then-Upgrade 模式：同步截断写入 → 异步 LLM 覆写真正摘要。

## Online Verification Log

### Write-Then-Upgrade 异步覆写模式
- **Query**: async write-then-upgrade eventual consistency pattern
- **Source**: 项目内部 `session_committer.go:generateArchiveSummary` + `safeGo` 模式
- **Key finding**: 已有 `safeGo` panic-safe goroutine 包装 + `generateArchiveSummary` LLM L0/L1 生成模式，可直接复用
- **Verified date**: 2026-02-26

---

## 修改清单

### 1. `backend/internal/memory/uhms/vfs.go` — 新增 WriteL0L1

- [x] 新增 `WriteL0L1(userID, mem, l0, l1)` 方法
- [x] 仅覆写 `l0.txt` + `l1.txt`，不动 `l2.txt` / `meta.json`
- [x] 加 `mu.Lock()` 保护并发写入

### 2. `backend/internal/memory/uhms/manager.go` — 异步 LLM 升级

- [x] `writeVFS()` Phase 1: 同步截断写入（不变）
- [x] `writeVFS()` Phase 2: `m.llm != nil` 时 `safeGo` 触发异步升级
- [x] 值副本 `memCopy := *mem` 避免异步竞态
- [x] 新增 `upgradeVFSSummary()`: 调用 LLM → `WriteL0L1` 覆写
- [x] 新增 `generateMemorySummary()`: L0 prompt (~50 词) + L1 prompt (~100-300 词)
- [x] 降级策略: L0 失败 → 返回空串保留截断; L1 失败 → 返回 L0 + 截断 L1

### 3. 验证

- [x] `go build ./...` 编译通过
- [x] 现有行为不变: `m.llm == nil` 时无异步触发

## 不修改的文件

- `session_committer.go` — `generateArchiveSummary()` 独立，不变
- `interfaces.go` — `LLMProvider` 接口不变
- `config.go` — 无新增配置项（`llm != nil` 即开关）
- 前端 — 读取端已正常（`ReadL0/L1/L2` 按 level 返回文件内容）
