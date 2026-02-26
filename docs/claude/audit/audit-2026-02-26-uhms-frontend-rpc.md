---
document_type: Audit
status: Final
created: 2026-02-26
last_updated: 2026-02-26
---

# Audit: UHMS 前端 RPC 扩展

## Scope

- `internal/memory/uhms/manager.go` — 4 新增方法 (GetMemory/DeleteMemory/ListMemories/ReadVFSContent)
- `internal/gateway/server_methods_memory.go` — 新文件, 6 handler + memoryToMap helper
- `internal/gateway/server_methods.go` — readMethods/writeMethods 授权集扩展
- `internal/gateway/server.go` — +1 行 MemoryHandlers() 注册

## Findings

### F-01 [INFO] memory.get 无 userID 所有权校验

**Location**: `server_methods_memory.go:108`, `manager.go:452`

**Analysis**: `handleMemoryGet` 通过 `id` 直接调用 `mgr.GetMemory(id)`，不传入也不校验 `userID`。
理论上知道 memory ID 的客户端可读取任何用户的记忆。

**Risk**: LOW — Memory ID 为 128-bit 随机 hex (32 chars)，不可猜测。与现有 `store.GetMemory(id)`
签名一致。单用户/受信任前端场景可接受。

**Recommendation**: 多租户场景需在 GetMemory 中增加 userID 过滤。当前不阻塞。

---

### F-02 [INFO] memory.compress/commit 消息数组无长度上限

**Location**: `server_methods_memory.go:176,250`

**Analysis**: `handleMemoryCompress` 和 `handleMemoryCommit` 接受任意长度的 messages/transcript 数组。

**Risk**: LOW — `CompressIfNeeded` 内部 `summarizeMessages` 有 60K chars 截断保护;
`CommitSession` 委托 `session_committer.go` 同样有内部限制。JSON 反序列化由 gateway
WebSocket 层的消息大小限制保护。

**Recommendation**: 可选添加 `len(rawMessages) > 10000` 上限作为额外防线。当前不阻塞。

---

### F-03 [INFO] ListMemories list+count 非原子

**Location**: `manager.go:523-531`

**Analysis**: `ListMemories` 先查 `store.ListMemories` 再查 `store.CountMemories`，
两次 SQLite 查询之间可能有插入/删除，导致 total 与返回列表微小不一致。

**Risk**: LOW — 只读 API，分页显示场景可接受。

**Recommendation**: 无需修改。如需严格一致，可用单个 SQL 子查询。

---

## Security Checklist

- [x] 输入校验: id/userID/sessionKey 非空检查 ✅
- [x] 所有权: DeleteMemory 校验 `mem.UserID == userID` ✅
- [x] 授权: 6 新方法 + 3 现有方法均已注册到 readMethods/writeMethods ✅
- [x] nil-safe: 所有 handler 首行检查 `mgr == nil` ✅
- [x] 参数上限: limit 最大 200, level 0-2 范围校验 ✅
- [x] best-effort: VFS/Vector 删除失败不阻塞主流程 ✅
- [x] safeGo: 异步 vector delete 使用 safeGo 包装 ✅

## Resource Safety Checklist

- [x] 无文件句柄泄漏
- [x] 无 goroutine 泄漏 (safeGo 带 panic recovery)
- [x] context.Background() 与现有 UHMS handler 一致

## Verdict

**PASS** — 3 个 INFO 级别发现，均不阻塞。代码质量与现有 `server_methods_uhms.go` 保持一致。
