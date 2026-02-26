---
document_type: Archive
status: Archived
created: 2026-02-26
last_updated: 2026-02-26
audit_report: docs/claude/audit/audit-2026-02-26-uhms-frontend-rpc.md
skill5_verified: true
---

# UHMS 记忆系统前端 RPC 扩展

## 概述

补全前端缺失的 6 个 RPC 方法，修复现有 3 个方法的授权注册缺失。

## 修改清单

| 文件 | 变更 | 状态 |
|------|------|------|
| `internal/memory/uhms/manager.go` | +4 公开方法 (GetMemory/DeleteMemory/ListMemories/ReadVFSContent) | ✅ |
| `internal/gateway/server_methods_memory.go` | **新建** 6 个 RPC handler + memoryToMap helper | ✅ |
| `internal/gateway/server_methods.go` | readMethods/writeMethods 授权集注册 (新增 6 + 修复 3) | ✅ |
| `internal/gateway/server.go` | +1 行 `registry.RegisterAll(MemoryHandlers())` | ✅ |

## 任务检查

- [x] Step 1: manager.go — GetMemory, DeleteMemory, ListMemories, ReadVFSContent
- [x] Step 2: server_methods_memory.go — 6 handler + memoryToMap
- [x] Step 3: server_methods.go — 授权集 (read: status/search/list/get, write: add/delete/compress/commit/decay.run)
- [x] Step 4: server.go — MemoryHandlers() 注册
- [x] `go build ./internal/memory/uhms/` — PASS
- [x] `go build ./internal/gateway/` — PASS
- [x] `go vet ./internal/memory/uhms/ ./internal/gateway/` — PASS
- [x] 审计 (Skill 4) — PASS, 3 INFO findings

## 安全保证

- 不修改 server_methods_uhms.go (现有 3 handler 不变)
- 不修改 Manager 接口 (仅 DefaultManager 具体类型加方法)
- 不修改 boot.go / attempt_runner.go
- 所有 handler 首先检查 mgr == nil
- DeleteMemory 校验 userID 所有权
- VFS/Vector 删除为 best-effort
- limit 上限 200 防滥用

## Online Verification Log

### VFS ReadL0/L1/L2 签名
- **Source**: `internal/memory/uhms/vfs.go:134-148`
- **Key finding**: ReadL0/L1/L2 均接受 (userID, memoryType, category, memoryID) 4 参数
- **Verified date**: 2026-02-26

### VectorIndex.Delete 签名
- **Source**: `internal/memory/uhms/interfaces.go:56`
- **Key finding**: `Delete(ctx, collection, id string) error` — 需传 collection + id
- **Verified date**: 2026-02-26

### Store.ListMemories + CountMemories
- **Source**: `internal/memory/uhms/store.go:260,354`
- **Key finding**: ListMemories 返回 `([]Memory, error)`, CountMemories 返回 `(int64, error)`
- **Verified date**: 2026-02-26
