---
document_type: Audit
status: Completed
created: 2026-02-27
last_updated: 2026-02-27
tracking_doc: docs/claude/tracking/impl-plan-global-boot-skill-vfs-2026-02-27.md
verdict: PASS
---

# Audit: Global Boot + VFS Tiered Skill Loading

**Scope**: Code changes implementing the Global Boot + VFS tiered skill distribution feature.
**Auditor**: Claude Code (Sonnet 4.6)
**Date**: 2026-02-27

---

## Files Audited

| File | Change Type |
|------|-------------|
| `backend/internal/memory/uhms/vfs.go` | Extended (guard + 2 new methods) |
| `backend/internal/memory/uhms/types.go` | Extended (2 new types) |
| `backend/internal/memory/uhms/interfaces.go` | Extended (4 new interface methods) |
| `backend/internal/memory/uhms/manager.go` | Extended (5 new methods + helpers) |
| `backend/internal/agents/skills/skill_distributor.go` | Extended (SkillDistributor struct) |
| `backend/internal/gateway/server_methods_uhms.go` | Extended (skills.distribution.status RPC) |

---

## Security Findings

### F1 — `_system` Guard: Exact Match Only (Low, Pre-existing)
**Location**: `vfs.go:53-55`, `vfs.go:102-104`

The new guard `if userID == "_system"` prevents the exact string, but does not prevent `../` path traversal (e.g., `userID = "../_system"`). `filepath.Join` normalizes `..` segments at the OS layer, so a crafted `userID` could resolve outside `vfsRoot`. However, this is pre-existing behavior in `WriteMemory`/`WriteArchive` — the new guard only hardens one specific vector. `userID` originates from authenticated agent sessions in current callers.

**Risk**: Low (pre-existing pattern, controlled input source).
**Recommendation**: Add `strings.Contains(userID, "..")` check to all VFS write path-building methods in a follow-up hardening pass.

### F8 — `ReadByVFSPath` No Path Validation (Low)
**Location**: `vfs.go:204-217`, `manager.go:988-999`

`ReadSystemL0/L1/L2` delegate to `vfs.ReadByVFSPath(vfsPath, level)` which calls `filepath.Join(v.root, vfsPath)` with no `..` validation. A `vfsPath` containing `../` sequences could escape `vfsRoot`. **Currently mitigated**: all `vfsPath` values are constructed by system internals (`filepath.Join("_system", namespace, category, id)`) and never accepted from user input.

**Risk**: Low (currently mitigated; becomes Medium if user-supplied vfsPath is ever accepted).
**Recommendation**: Add path escaping check in `ReadByVFSPath`: verify `filepath.Clean(filepath.Join(v.root, vfsPath))` starts with `v.root`.

---

## Correctness Findings

### F5 — Context Cancellation Ignored in `SearchSystem` Fallback (Medium) ✅ FIXED
**Location**: `manager.go:918-934`

**Before fix**: When `SearchSystemEntries` failed for any reason — including `context.Canceled` or `context.DeadlineExceeded` — the code fell through to the VFS meta.json scan. This wasted CPU on cancelled requests and violated Go context conventions (cancelled context should propagate, not trigger fallback work).

**Fix applied**: Added `if ctx.Err() != nil { return nil, ctx.Err() }` guard before the VFS fallback path. Only non-cancellation errors trigger the scan.

**Risk**: Medium (unnecessary resource usage on cancellation, unexpected behavior).
**Status**: Fixed in this audit.

### F3 — `SkillDistributeResult.Updated` Never Set (Minor)
**Location**: `skill_distributor.go:25`, `skill_distributor.go:92-113`

The `Updated int` field was added to `SkillDistributeResult` to distinguish "first-time indexed" from "re-indexed after change". However, `DistributeSkills` only increments `Indexed` or `Skipped` — it has no "updated" path. `Updated` always stays 0.

**Risk**: Minor (misleading field, no behavioral bug for current callers).
**Recommendation**: Either remove `Updated` and use `Indexed` for both cases, or add tracking in `distributeOneSkill` to distinguish first-write vs overwrite. Logged as deferred.

---

## Resource Safety

### F4 — Per-Request `NewBootManager` Allocation (Minor)
**Location**: `server_methods_uhms.go:414-422`

`handleSkillsDistributionStatus` calls `uhms.NewBootManager(bp)` on every WebSocket RPC call. `BootManager` has an internal `sync.Once` cache, but since a new instance is created per request, the cache provides no benefit. Each call parses `boot.json` from disk.

**Risk**: Minor (disk I/O per status query; typically low-frequency call).
**Recommendation**: Cache a `BootManager` instance in `GatewayState` or inside `DefaultManager` (alongside `BootFilePath`). Deferred.

---

## Code Cleanliness

### F2 — `relativeSystemPath` Unused (Minor)
**Location**: `vfs.go:568-570`

Method `relativeSystemPath(namespace, category, id string)` is defined on `LocalVFS` but not called by any new or existing code. The distributor builds paths directly via `filepath.Join("_system", "skills", category, name)`.

**Risk**: None (dead code; Go compiler won't complain since it's a method, not a function).
**Recommendation**: Remove or use in a future refactor to unify path construction.

### F6 — Empty Query Match-All in `keywordScore` (Low)
**Location**: `manager.go:1058-1073`

`strings.Contains(s, "")` returns `true` in Go. An empty `query` passed to `SearchSystem` → `keywordScore` would give all entries a score of ≥ 3.0, returning all skills. Current callers never pass empty query (Boot mode uses task text; distribute status uses hardcoded collection query).

**Risk**: Low (only affects `searchSystemVFSFallback`; Qdrant path is unaffected).
**Recommendation**: Early-return `0.0` in `keywordScore` when query is empty, or add non-empty validation in `SearchSystem`.

---

## Concurrency & Mutex Analysis

All new VFS methods (`SystemEntryHash`) correctly acquire `mu.RLock` for read operations. `WriteSystemEntry` (pre-existing) uses `mu.Lock` for write. No double-lock or lock-across-await issues observed.

`DefaultManager.SearchSystem` and helper methods use `m.mu.RLock` at the `SearchSystemEntries` layer (pre-existing). New helper methods (`searchSystemVFSFallback`, `payloadHitsToSystemHits`) don't hold locks directly — they delegate to VFS methods that manage their own mutex. No deadlock risk.

---

## Interface Correctness

- `SearchSystem`, `ReadSystemL0`, `ReadSystemL1`, `ReadSystemL2` added to `Manager` interface and fully implemented on `DefaultManager`. ✅
- `SystemDistributionStatus` is on `DefaultManager` (concrete type) only — this is correct since `ctx.Context.UHMSManager` is `*uhms.DefaultManager` per `server_methods.go:115`. ✅
- `SkillDistributor.Distribute` two-pass design (VFS write → payload index) is correct: `DistributeSkills` receives `nil` VectorIndex (skips old upsert path), then `manager.IndexSystemEntry` routes through `UpsertPayload` with correct zero-vector dimension. ✅

---

## Verdict

**PASS** (with one fix applied)

F5 (context cancellation) was fixed during this audit. All other findings are Low/Minor severity with no behavioral correctness bugs in primary code paths. The implementation is functionally complete and safe for the current call patterns.

**Deferred items**: F1 (path traversal hardening), F2 (dead code), F3 (Updated field tracking), F4 (BootManager caching), F6 (empty query guard), F8 (ReadByVFSPath validation) — logged for follow-up.
