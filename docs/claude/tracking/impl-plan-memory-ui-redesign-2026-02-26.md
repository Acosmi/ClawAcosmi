---
document_type: Tracking
status: Auditing
created: 2026-02-26
last_updated: 2026-02-26
audit_report: docs/claude/audit/audit-2026-02-26-memory-ui-redesign.md
skill5_verified: true
---

# Memory Management UI Redesign

## Online Verification Log

### Lit html template patterns
- **Query**: lit html template rendering patterns
- **Source**: https://lit.dev/docs/templates/overview/
- **Key finding**: Lit uses tagged template literals with `html` and `nothing` sentinel. Existing codebase patterns confirmed.
- **Verified date**: 2026-02-26

## Task Checklist

### Phase 1: Go backend stats endpoint (~80 LOC)
- [x] Add `AggregateStats` struct to `uhms/store.go`
- [x] Implement `Store.AggregateStats()` with 3 SQL queries
- [x] Add `handleMemoryStats` handler in `server_methods_memory.go`
- [x] Register `memory.stats` RPC in `MemoryHandlers()`

### Phase 2: Frontend data layer (~65 LOC)
- [x] Add `MemoryStats` + `MemorySearchResult` types to `controllers/memory.ts`
- [x] Add `loadMemoryStats()` + `searchMemories()` controller functions
- [x] Extend `MemoryState` with stats/search fields
- [x] Add 4 `@state()` fields in `app.ts`
- [x] Update `renderMemory()` call in `app-render.ts` with new props

### Phase 3: i18n (~70 LOC)
- [x] Add ~35 new keys to `locales/zh.ts`
- [x] Add ~35 new keys to `locales/en.ts`

### Phase 4: Page rewrite memory.ts (~850 LOC)
- [x] Remove `renderFlowingBanner()` (lines 82-176)
- [x] Remove Sessions card (lines 416-472)
- [x] Remove `renderMemoryTypeCapsule()` (lines 179-201)
- [x] Remove static banner capsule
- [x] Rewrite Card 1: Status Overview (6 stats + LLM config + import)
- [x] Add Card 2: Distribution & Health (type bar + category pills + decay bars)
- [x] Enhance Card 3: Memory List (search + pills + importance bar + decay dot)
- [x] Enhance Card 4: Memory Detail (pills + progress bars + L0/L1/L2 tabs)
- [x] Update `MemoryProps` type
- [x] Preserve all existing functionality (filter/pagination/delete/LLM config)

### Phase 5: Header capsules update (~10 LOC)
- [x] Update `renderMemoryTypeCapsules()` to show dynamic stats

## Post-Implementation
- [x] `go build ./...` passes
- [x] TypeScript compiles without new errors (0 new errors introduced)
- [x] Audit (Skill 4) — PASS, 5 findings all fixed
- [ ] Archive (awaiting用户确认)
