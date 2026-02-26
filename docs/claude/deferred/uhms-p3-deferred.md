---
document_type: Deferred
status: In Progress
created: 2026-02-26
last_updated: 2026-02-26
audit_report: docs/claude/audit/audit-2026-02-26-uhms-p3-runner-gateway.md
skill5_verified: true
---

# UHMS P3 Deferred Items

来源: 审计报告 `audit-2026-02-26-uhms-p3-runner-gateway.md`

## ~~D-01: Bridge Adapter 快速路径优化 (F-04)~~ ✅ FIXED

**修复**: adapter 层加快速 byte-length 估算，低于阈值跳过双向转换。

## ~~D-02: L0→L1 升级 Token 估算 (F-05)~~ ✅ FIXED

**修复**: 加载 L1 后实测 token，预留 20% budget，仅确认不超 budget 才升级。

## ~~D-03: 记忆提取日志级别 (F-06)~~ ✅ FIXED

**修复**: safeGo 包装 + Debug→Warn 升级 + panic recovery。

## D-04: context_middleware.go 实现

**描述**: P3 计划中的 `context_middleware.go` 尚未创建为独立文件，压缩逻辑直接在 `manager.go:CompressIfNeeded` 中实现。可考虑后续提取为独立中间件。
**优先级**: LOW — 当前实现功能完整。

## D-05: server_methods_memory.go RPC 实现

**描述**: P3 计划中的 `memory.uhms.status/search/compress/commit` RPC 尚未实现。需要新建 `server_methods_memory.go`。
**优先级**: MEDIUM — P4 阶段前置依赖。
