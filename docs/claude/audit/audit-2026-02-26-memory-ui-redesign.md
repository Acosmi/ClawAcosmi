---
document_type: Audit
status: Archived
created: 2026-02-26
last_updated: 2026-02-26
---

# Audit: Memory Management UI Redesign

## Scope

All code changes from `impl-plan-memory-ui-redesign-2026-02-26.md`:

| File | Type |
|------|------|
| `backend/internal/memory/uhms/store.go` | Go — AggregateStats |
| `backend/internal/memory/uhms/manager.go` | Go — proxy method |
| `backend/internal/gateway/server_methods_memory.go` | Go — RPC handler |
| `ui/src/ui/controllers/memory.ts` | TS — types + controllers |
| `ui/src/ui/views/memory.ts` | TS — full page rewrite |
| `ui/src/ui/app.ts` | TS — state fields |
| `ui/src/ui/app-view-state.ts` | TS — type interface |
| `ui/src/ui/app-render.ts` | TS — wiring + imports |
| `ui/src/ui/locales/zh.ts` | i18n — 38 keys |
| `ui/src/ui/locales/en.ts` | i18n — 38 keys |

## Findings

### F-01 — MEDIUM: `rows.Err()` not checked after iteration loops

**Location**: `store.go:391-400`, `store.go:408-417`

**Issue**: Two `for rows.Next()` loops iterate over GROUP BY results, then call `rows.Close()` but never check `rows.Err()`. If the database returns an error during iteration (e.g. I/O error mid-stream), it will be silently swallowed.

**Risk**: Silent data corruption — stats could return partial aggregates without signaling error.

**Recommendation**: Add `if err := rows.Err(); err != nil { return nil, err }` after each loop, before `rows.Close()`.

---

### F-02 — MEDIUM: Debounce timer reset on every Lit re-render

**Location**: `views/memory.ts:333-342`

**Issue**: `let searchTimer` is declared inside `renderMemory()`. Lit calls this function on every reactive update. Each call creates a new closure with a fresh `searchTimer = null`. The `clearTimeout(searchTimer)` on subsequent input events cannot reference the timer from a previous render cycle. Result: debounce does **not** work — every keystroke fires the search after 300ms, regardless of subsequent typing.

**Risk**: Excessive RPC calls on fast typing. Degrades UX with rapid flicker.

**Recommendation**: Move `searchTimer` to module scope (outside `renderMemory`), so it persists across renders.

---

### F-03 — MEDIUM: `formatRelativeTimestamp` called with Unix seconds, expects milliseconds

**Location**: `views/memory.ts:630`

**Issue**: `formatRelativeTimestamp(d.lastAccessedAt)` passes `lastAccessedAt` which is a Unix timestamp in **seconds** (from backend `m.LastAccessedAt.Unix()`). But `formatRelativeTimestamp` parameter is named `timestampMs` and does `Date.now() - timestampMs` — expects **milliseconds**.

**Result**: All "last accessed" times will appear as "50+ years ago" (off by factor 1000).

**Recommendation**: Pass `d.lastAccessedAt * 1000`.

---

### F-04 — LOW: Redundant SQL column in aggregate query

**Location**: `store.go:422,425`

**Issue**: Column 1 (`retPermanent`) and column 4 (`DecayHealth.Permanent`) compute the exact same SQL expression: `SUM(CASE WHEN retention_policy = 'permanent' THEN 1 ELSE 0 END)`. They always yield the same value.

**Risk**: No functional bug, but wasted CPU and confusing code.

**Recommendation**: Remove column 4, assign `stats.DecayHealth.Permanent = retPermanent` after Scan.

---

### F-05 — LOW: `defer rows.Close()` pattern not used

**Location**: `store.go:391-400`, `store.go:408-417`

**Issue**: Manual `rows.Close()` at end of loop. If `rows.Scan` panics (unlikely in production, possible in test), rows won't be closed. Existing codebase (e.g. `ListMemories`) uses `defer rows.Close()`.

**Recommendation**: Use `defer rows.Close()` for consistency with the rest of the file.

---

### F-06 — INFO: Removed `GatewaySessionRow` import + sessions props — backward compatible?

**Location**: `views/memory.ts` (removed), `app-render.ts:1119-1121` (removed)

**Issue**: The old `MemoryProps` included `sessions`, `sessionsLoading`, `onSessionsRefresh`, `basePath`. These are now removed. Any external consumer of `renderMemory()` would break.

**Risk**: None in practice — `renderMemory` is only consumed by `app-render.ts`. Verified by grep.

---

### F-07 — INFO: `formatRelativeTimestamp` also used in sessions page

**Location**: The old code used `formatRelativeTimestamp` for session `updatedAt` (also seconds). That code is now removed from memory.ts. No regression.

---

### F-08 — INFO: `common.loading` key assumed to exist

**Location**: `views/memory.ts:412`

**Issue**: Uses `t("common.loading")`. Verified — this key exists in both locale files.

---

## Fixes Applied

- **F-01**: Added `rows.Err()` check after both GROUP BY loops. Renamed second `rows` to `catRows` to avoid shadow.
- **F-02**: Moved `searchTimer` to module scope (`_searchDebounceTimer`) so it persists across Lit re-renders.
- **F-03**: Changed `formatRelativeTimestamp(d.lastAccessedAt)` to `formatRelativeTimestamp(d.lastAccessedAt * 1000)`.
- **F-04**: Removed redundant SQL column 4, assigned `DecayHealth.Permanent = retPermanent` after Scan.
- **F-05**: Changed manual `rows.Close()` to `defer rows.Close()` / `defer catRows.Close()`.

## Post-Fix Verification

- `go build ./...` — PASS
- `npx tsc --noEmit` — 0 new errors (all pre-existing)

## Verdict

**PASS** — All 5 actionable findings fixed and verified.

| ID | Severity | Status |
|----|----------|--------|
| F-01 | MEDIUM | FIXED |
| F-02 | MEDIUM | FIXED |
| F-03 | MEDIUM | FIXED |
| F-04 | LOW | FIXED |
| F-05 | LOW | FIXED |
| F-06 | INFO | ACCEPTED |
| F-07 | INFO | ACCEPTED |
| F-08 | INFO | ACCEPTED |
