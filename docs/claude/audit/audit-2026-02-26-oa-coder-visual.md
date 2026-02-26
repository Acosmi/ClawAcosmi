---
document_type: Audit
status: In Progress
created: 2026-02-26
last_updated: 2026-02-26
audit_report: self
skill5_verified: true
---

# Audit Report: oa-coder Visual Integration (P1-P5)

## Scope

Full Skill 4 granular code-level audit of the oa-coder visual integration, covering
5 phases of changes across Go backend, TypeScript frontend, Rust conditional compilation,
CSS, and a new skill file.

| Phase | Area | Files | Approx LOC |
|-------|------|-------|------------|
| P1 | Go Backend — Confirmation Manager | 8 files | ~260 |
| P2 | Frontend — Rich Tool Cards | 5 files | ~265 |
| P3 | Frontend — Confirmation Flow | 6 files | ~200 |
| P4 | Rust — Feature-Gated Extraction | 3 files | ~27 |
| P5 | Skill File | 1 file | ~60 |

### Build Verification (pre-audit, user-provided)

- `go build ./...` PASS
- `go test ./internal/agents/runner/...` PASS
- `go test ./internal/gateway/...` PASS
- `cargo check -p oa-coder` PASS
- `cargo check -p oa-coder --no-default-features` PASS
- `cargo test -p oa-coder` 40/40 PASS
- TypeScript: 0 new errors

---

## Findings

### F-01 [INFO] — Timer vs context timeout redundancy in `RequestConfirmation`

**Location:** `backend/internal/agents/runner/coder_confirmation.go:96-114`

**Analysis:** `RequestConfirmation` creates both a `time.NewTimer(m.timeout)` and accepts
a parent `ctx` with its own deadline (from `AttemptRunner`). The `select` on lines 100-114
correctly handles both, with `defer timer.Stop()` on line 97. If ctx expires first, the
timer is cleaned up by the defer. If the timer fires first, ctx is not cancelled but the
function returns. No resource leak in either path.

**Risk:** INFO — No bug, but the two timeout sources could interact unexpectedly. If
the parent `ctx` has a shorter deadline than `m.timeout` (60s default), the context wins
and auto-denies. This is correct behavior for the abort/cancel case.

**Recommendation:** No action required. Document in the function comment that ctx.Done
takes precedence over the internal timer when the parent deadline is shorter.

---

### F-02 [LOW] — `ResolveConfirmation` does not delete pending entry

**Location:** `backend/internal/agents/runner/coder_confirmation.go:135-160`

**Analysis:** `ResolveConfirmation` reads the channel from `m.pending[id]` under the lock
(line 141) and writes the decision (line 150), but does NOT delete the entry from
`m.pending`. The entry is only deleted in `RequestConfirmation` on line 118 after the
select returns.

This means between the time `ResolveConfirmation` sends the decision and
`RequestConfirmation` wakes up and deletes the entry, a second call to
`ResolveConfirmation` with the same `id` will find the channel, attempt to write, and
hit the `default` case (line 155) silently. This is correct and safe.

However, if `RequestConfirmation` panics between the select and the delete (unlikely,
since there is no panic path there), the entry would leak in `m.pending`. Given the
zero-panic policy, this is acceptable.

**Risk:** LOW — Double-call is safely handled by the buffered channel + default case.
No entry leak under normal operation.

**Recommendation:** No action required.

---

### F-03 [LOW] — Pending channel leak if `broadcast` panics

**Location:** `backend/internal/agents/runner/coder_confirmation.go:86-88`

**Analysis:** If `m.broadcast("coder.confirm.requested", req)` panics (e.g., a bug in
the broadcast callback), the goroutine executing `RequestConfirmation` will unwind.
The channel was already added to `m.pending` on line 82 but the cleanup on line 117-119
will not execute, leaving an orphan entry.

In practice, `m.broadcast` is `bc.Broadcast(event, payload, nil)` from boot.go:202,
which calls `json.Marshal` + `c.Send`. `json.Marshal` does not panic for valid Go values.
`c.Send` is a WebSocket write. Neither should panic.

**Risk:** LOW — Extremely unlikely panic path; Go convention discourages panics in
library code. The `Broadcaster.broadcastInternal` silently returns on marshal error.

**Recommendation:** Consider wrapping the broadcast call in a `defer/recover` guard for
defense in depth, or accept the risk given the zero-panic contract.

---

### F-04 [INFO] — `truncatePreview` uses `[]rune` conversion — correct Unicode handling

**Location:** `backend/internal/agents/runner/coder_confirmation.go:210-216`

**Analysis:** `truncatePreview` converts to `[]rune` before slicing, preserving multi-byte
character boundaries. This is correct and prevents invalid UTF-8 in the preview payload.
The `"..."` suffix is pure ASCII. No data integrity issue.

**Risk:** INFO — Correct implementation.

**Recommendation:** None.

---

### F-05 [LOW] — `extractCoderPreview` returns nil on parse error, no logging

**Location:** `backend/internal/agents/runner/coder_confirmation.go:173-177`

**Analysis:** If `json.Unmarshal(args, &parsed)` fails, the function returns nil silently.
The `CoderConfirmationRequest.Preview` field will be nil, and the frontend will receive
a request with no preview data. The user can still allow/deny but with no context.

**Risk:** LOW — Degraded UX on malformed args (unlikely from a functioning Coder bridge).

**Recommendation:** Add a `slog.Debug` log on parse failure for diagnostic visibility.

---

### F-06 [INFO] — `isCoderConfirmable` toolName matching

**Location:** `backend/internal/agents/runner/coder_confirmation.go:163-170`

**Analysis:** The function checks against `mcpToolName` (after stripping `coder_` prefix
in `executeCoderTool`). The MCP tool names from oa-coder are `edit`, `write`, `bash`,
`read`, `grep`, `glob`. The confirmable set `{edit, write, bash}` is the correct subset
of mutating operations. `read`, `grep`, `glob` are read-only and correctly excluded.

**Risk:** INFO — Correct design.

**Recommendation:** None.

---

### F-07 [INFO] — `CoderConfirmation` nil check in `executeCoderTool`

**Location:** `backend/internal/agents/runner/tool_executor.go:689`

**Analysis:** The confirmation interception is guarded by
`if params.CoderConfirmation != nil && isCoderConfirmable(mcpToolName)`. When
`CoderConfirmation` is nil (the default when coder bridge is not available or when the
manager is not initialized), the code falls through directly to `AgentCallTool`.
This preserves backward compatibility — zero behavior change when the feature is not
configured.

**Risk:** INFO — Correct nil-safe design.

**Recommendation:** None.

---

### F-08 [INFO] — `CoderConfirmation` wiring in `attempt_runner.go`

**Location:** `backend/internal/agents/runner/attempt_runner.go:96,268,331`

**Analysis:** The `CoderConfirmation` field is added to `EmbeddedAttemptRunner` (line 96)
and passed through to both `ToolExecParams` instances — the primary tool loop (line 268)
and the retry-after-approval path (line 331). Both are consistent.

**Risk:** INFO — Correct, consistent wiring.

**Recommendation:** None.

---

### F-09 [INFO] — `coder.confirm.resolve` registered in `approvalMethods`

**Location:** `backend/internal/gateway/server_methods.go:168`

**Analysis:** The method `"coder.confirm.resolve"` is in the `approvalMethods` set, which
requires the `operator.approvals` scope. This is the correct authorization level — same
as `exec.approval.resolve` and `security.escalation.resolve`. Only clients with the
approvals scope can send confirmation decisions.

**Risk:** INFO — Correct authorization design.

**Recommendation:** None.

---

### F-10 [INFO] — Broadcast scope guards for coder events

**Location:** `backend/internal/gateway/broadcast.go:52-53`

**Analysis:** Two new entries are added to `eventScopeGuards`:
- `"coder.confirm.requested"` requires `scopeApprovals`
- `"coder.confirm.resolved"` requires `scopeApprovals`

This ensures only clients with the `operator.approvals` scope receive coder confirmation
broadcasts. This prevents unauthorized clients from seeing tool call previews (which may
contain file paths and code content).

**Risk:** INFO — Correct scope guard design.

**Recommendation:** None.

---

### F-11 [LOW] — `CoderConfirmMgr` wiring in `ws_server.go`

**Location:** `backend/internal/gateway/ws_server.go:475`

**Analysis:** `CoderConfirmMgr: cfg.State.CoderConfirmMgr()` is wired into the
`GatewayMethodContext` for each WebSocket connection. `CoderConfirmMgr()` simply returns
the pointer from `GatewayState`, which may be nil if coder bridge was not initialized.
The RPC handler in `server.go:506` checks for nil before calling `ResolveConfirmation`.

The `WsServerConfig` struct does NOT include `CoderConfirmMgr` directly — it's accessed
through `cfg.State`, which is the `GatewayState`. This is consistent with how
`EscalationMgr`, `TaskPresetMgr`, and other optional subsystems are accessed.

**Risk:** LOW — No issue, but the access pattern differs from `RemoteMCPBridge` which IS
on `WsServerConfig` directly. Minor inconsistency, not a bug.

**Recommendation:** Accept the inconsistency; it follows the established pattern for
subsystems owned by `GatewayState`.

---

### F-12 [INFO] — `CoderConfirmMgr` initialization in `boot.go`

**Location:** `backend/internal/gateway/boot.go:200-206`

**Analysis:** The confirmation manager is created only when `s.coderBridge != nil`,
using the broadcaster's `Broadcast` method as the callback. The 60-second timeout is
hardcoded but matches the `NewCoderConfirmationManager` default guard on line 55-57
of `coder_confirmation.go`.

The broadcast callback captures `bc` (the broadcaster) by closure. Since `bc` is created
earlier in `NewGatewayState` and is never replaced, this is safe.

**Risk:** INFO — Correct lifecycle management.

**Recommendation:** None.

---

### F-13 [INFO] — RPC handler input validation in `server.go`

**Location:** `backend/internal/gateway/server.go:499-515`

**Analysis:** The RPC handler for `coder.confirm.resolve`:
1. Extracts `id` and `decision` from params (string type assertion, defaults to "")
2. Validates both are non-empty
3. Checks `CoderConfirmMgr` is non-nil
4. Delegates to `ResolveConfirmation(id, decision)`
5. `ResolveConfirmation` validates `decision` is "allow" or "deny"

The validation chain is complete. Invalid `decision` values are rejected by
`ResolveConfirmation` before the channel write.

**Risk:** INFO — Correct input validation.

**Recommendation:** None.

---

### F-14 [INFO] — `server.go` `CoderConfirmation` injection into AttemptRunner

**Location:** `backend/internal/gateway/server.go:667`

**Analysis:** `CoderConfirmation: state.CoderConfirmMgr()` is set on the
`EmbeddedAttemptRunner`. When nil (coder bridge not available), the attempt runner
skips all confirmation logic per F-07.

**Risk:** INFO — Correct wiring.

**Recommendation:** None.

---

### F-15 [INFO] — Frontend `isCoderTool` detection

**Location:** `ui/src/ui/chat/coder-tool-cards.ts:12-14`

**Analysis:** `isCoderTool` checks `name.startsWith("coder_")`, matching the Go backend
prefix convention (`"coder_" + t.Name` in `attempt_runner.go:589`). The early return in
`tool-cards.ts:55-58` delegates to `renderCoderCard`, which returns `null` for unknown
coder sub-tools (e.g., `coder_grep`, `coder_glob`, `coder_read`), falling through to the
default renderer. This is correct — only `edit`, `write`, `bash` get enhanced cards.

**Risk:** INFO — Correct dispatch logic.

**Recommendation:** None.

---

### F-16 [INFO] — No XSS risk in lit `html` template literals

**Location:** `ui/src/ui/chat/coder-tool-cards.ts`, `ui/src/ui/views/coder-confirm.ts`

**Analysis:** All user-controlled data (file paths, code content, commands) is interpolated
via lit's `html` tagged template literals, which auto-escape HTML. There is no use of
`.innerHTML`, `unsafeHTML`, or raw string concatenation into DOM. The diff preview lines
and file paths are rendered via `${variable}` inside `html\`...\``, which is safe.

The sidebar content uses markdown strings (e.g., line 109 of `coder-tool-cards.ts`:
`` `## Coder Write\n\n**File:** \`${filePath}\`...` ``). These are passed to
`onOpenSidebar` which renders them through a markdown renderer (not raw HTML), so
backtick-injection in filePath would appear as literal text, not code.

**Risk:** INFO — No XSS vulnerability.

**Recommendation:** None.

---

### F-17 [LOW] — Frontend `setTimeout` for auto-expiry cleanup

**Location:** `ui/src/ui/app-gateway.ts:359-362`

**Analysis:** When a `coder.confirm.requested` event arrives, a `setTimeout` is scheduled
to remove the entry after `expiresAtMs - Date.now() + 500` milliseconds. This mirrors the
existing `exec.approval.requested` pattern (lines 328-331).

If many confirmation requests arrive in rapid succession, each gets its own timer. These
timers cannot be cancelled individually (no timer ID is stored). However:
1. The `removeCoderConfirm` function is idempotent (filters by id)
2. The queue is pruned on every `addCoderConfirm` call via `pruneQueue`
3. In practice, coder tool calls are sequential (one at a time)

**Risk:** LOW — Timer accumulation is bounded by confirmation request rate, which is
inherently limited by LLM tool call frequency.

**Recommendation:** Accept current design; it matches the proven exec-approval pattern.

---

### F-18 [INFO] — Frontend `coderConfirmQueue` initialization and nil-safety

**Location:** `ui/src/ui/app-gateway.ts:140,357,361,370`

**Analysis:** `coderConfirmQueue` is initialized to `[]` in `connectGateway` (line 140)
and on the `GatewayHost` type (line 72). The event handlers use `?? []` defensively
(lines 357, 361, 370). The `@state()` decorator in `app.ts:173` initializes it to `[]`.

**Risk:** INFO — Properly initialized, defensive null coalescing.

**Recommendation:** None.

---

### F-19 [INFO] — `renderCoderConfirmPrompt` timer display accuracy

**Location:** `ui/src/ui/views/coder-confirm.ts:19-20`

**Analysis:** The remaining time is computed as `active.expiresAtMs - Date.now()` at render
time. This is a snapshot — it does not auto-update. The user sees the remaining time at
the moment the component renders. If the component re-renders (e.g., due to other state
changes), the timer updates.

The `formatRemaining` function handles negative values via `Math.max(0, ms)`, showing
"0s" for expired entries. The 500ms padding in the `setTimeout` (F-17) ensures the entry
is removed shortly after expiry.

**Risk:** INFO — The timer is static per render. For a 60s timeout, this is acceptable
UX. A live countdown would require `setInterval` + `requestUpdate`.

**Recommendation:** Consider adding a `setInterval` for live countdown in a future
enhancement. Current behavior is functional.

---

### F-20 [INFO] — `handleCoderConfirmDecision` in `app.ts`

**Location:** `ui/src/ui/app.ts:556-564`

**Analysis:** The method sends `coder.confirm.resolve` RPC with `{id, decision}` and
immediately removes the entry from the local queue via `removeCoderConfirm`. If the RPC
fails (catch block), the error is logged to console but the entry is NOT removed — the
`removeCoderConfirm` call is BEFORE the catch, inside the try block.

Wait — re-reading: the `removeCoderConfirm` on line 560 is inside the `try` block, after
the `await`. So if the `request` throws, the removal does NOT happen, and the entry
stays in the queue until the auto-expiry timer fires (F-17). This is correct behavior:
the user can retry if the RPC fails.

**Risk:** INFO — Correct error handling.

**Recommendation:** None.

---

### F-21 [INFO] — `AppViewState` type includes `coderConfirmQueue` and handler

**Location:** `ui/src/ui/app-view-state.ts:95,270`

**Analysis:** `coderConfirmQueue: CoderConfirmRequest[]` (line 95) and
`handleCoderConfirmDecision: (id: string, decision: "allow" | "deny") => Promise<void>`
(line 270) are both added to the `AppViewState` type, making them available to all view
components. The type-level `decision` parameter is constrained to `"allow" | "deny"`,
matching the Go backend validation.

**Risk:** INFO — Correct type definition.

**Recommendation:** None.

---

### F-22 [INFO] — `renderCoderConfirmPrompt` placement in `app-render.ts`

**Location:** `ui/src/ui/app-render.ts:1352`

**Analysis:** `renderCoderConfirmPrompt(state)` is rendered after `renderExecApprovalPrompt`
and before `renderGatewayUrlConfirmation`. This means both exec approval and coder
confirmation popups can appear simultaneously. Given they serve different purposes
(exec approval for bash commands, coder confirmation for coder sub-agent operations),
this is acceptable.

**Risk:** INFO — Correct render ordering.

**Recommendation:** None.

---

### F-23 [INFO] — Rust `#[cfg(feature = "sandbox")]` conditional compilation

**Location:** `cli-rust/crates/oa-coder/src/tools/bash.rs:60-69`

**Analysis:** The `execute` function uses `#[cfg(feature = "sandbox")]` to conditionally
compile the sandboxed path. When the `sandbox` feature is disabled:
1. The `execute_sandboxed` function is not compiled (line 155: `#[cfg(feature = "sandbox")]`)
2. The `sandboxed = true` code path falls through to `execute_direct` with a warning log
3. `oa-sandbox` dependency is not linked

The `#[cfg(not(feature = "sandbox"))]` block (lines 65-68) provides the fallback path.
The two `#[cfg]` blocks within the same `if sandboxed` branch ensure exactly one is
compiled, avoiding dead code warnings.

**Risk:** INFO — Correct conditional compilation pattern.

**Recommendation:** None.

---

### F-24 [INFO] — `oa-coder` Cargo.toml feature gating

**Location:** `cli-rust/crates/oa-coder/Cargo.toml:34,42-44`

**Analysis:**
- `oa-sandbox = { workspace = true, optional = true }` (line 34)
- `default = ["sandbox"]` (line 43) — sandbox enabled by default
- `sandbox = ["dep:oa-sandbox"]` (line 44) — feature activates the dependency

`oa-cmd-coder/Cargo.toml` (line 10): `oa-coder = { workspace = true, features = ["sandbox"] }`
explicitly enables sandbox for the CLI binary.

Build verification confirms both `cargo check -p oa-coder` (with sandbox) and
`cargo check -p oa-coder --no-default-features` (without sandbox) pass.

**Risk:** INFO — Correct feature flag design.

**Recommendation:** None.

---

### F-25 [INFO] — CSS variable fallbacks

**Location:** `ui/src/styles/chat/coder-cards.css`

**Analysis:** All CSS custom properties include fallback values:
- `var(--color-accent, #6366f1)` (line 6)
- `var(--font-mono, monospace)` (line 11)
- `var(--color-text-secondary, #9ca3af)` (line 12)
- `var(--color-bg-secondary, #1e1e2e)` (line 24)
- `var(--color-warning, #f59e0b)` (line 65)
- etc.

All fallbacks are sensible defaults for a dark theme. No missing fallback.

**Risk:** INFO — Correct CSS variable usage.

**Recommendation:** None.

---

### F-26 [INFO] — `tool-display.json` coder entries

**Location:** `ui/src/ui/tool-display.json`

**Analysis:** Six entries added for `coder_edit`, `coder_write`, `coder_read`,
`coder_bash`, `coder_grep`, `coder_glob`. Each has `icon`, `title`, and `detailKeys`.
The icons reference existing icon names (`penLine`, `edit`, `fileText`, `wrench`,
`search`, `folder`). The `detailKeys` correctly extract the primary parameter
(`filePath` for file tools, `command` for bash, `pattern` for search).

**Risk:** INFO — Correct configuration.

**Recommendation:** None.

---

### F-27 [INFO] — Skill file content

**Location:** `docs/skills/tools/coder/SKILL.md`

**Analysis:** The skill file provides a concise reference for the coder sub-agent,
documenting all 6 tools, best practices, edit engine layers, and security constraints.
The YAML frontmatter includes `name`, `description`, and `metadata` fields. The emoji
metadata is enclosed in a JSON string within a YAML literal block, which is the
established convention for skill files.

**Risk:** INFO — Well-structured skill documentation.

**Recommendation:** None.

---

### F-28 [LOW] — `coder-confirm.ts` preview content not truncated for bash

**Location:** `ui/src/ui/views/coder-confirm.ts:86-89`

**Analysis:** `renderBashPreview` renders `preview.command` without truncation. The Go
backend's `extractCoderPreview` (line 201-203 of `coder_confirmation.go`) also does NOT
truncate bash commands (unlike `oldString`/`newString` which are truncated to 500 chars).

A very long bash command (e.g., inline base64 data) would render untruncated in the
confirmation dialog. The CSS `coder-command-mono` class has `overflow: hidden` and
`text-overflow: ellipsis` with `white-space: nowrap`, which visually truncates single-line
commands. Multi-line commands would overflow the container height.

**Risk:** LOW — Visual overflow only; no security issue. The `max-height` is not set on
`.coder-command-mono` unlike `.coder-diff-preview` which has `max-height: 96px`.

**Recommendation:** Add `max-height: 96px; overflow: hidden;` to `.coder-command-mono`
in the CSS, or truncate bash commands in `extractCoderPreview` to 500 chars like the
edit fields.

---

### F-29 [LOW] — `coder-tool-cards.ts` sidebar content includes raw file paths

**Location:** `ui/src/ui/chat/coder-tool-cards.ts:109,228`

**Analysis:** The sidebar markdown content includes file paths directly in backtick
templates: `` `${filePath}` ``. Since this is markdown rendered through the sidebar's
markdown processor, backticks prevent markdown injection. However, if a file path
contains literal backtick characters, it could break the markdown formatting.

**Risk:** LOW — Cosmetic issue only; file paths with backticks are extremely rare.
No security impact since the sidebar renderer escapes HTML.

**Recommendation:** Accept the risk; this is an edge case with no security implications.

---

### F-30 [INFO] — No circular imports

**Analysis:** The architecture uses function callbacks to decouple `runner` and `gateway`:
- `CoderConfirmBroadcastFunc` (line 37 of `coder_confirmation.go`) is a plain `func` type
- `boot.go` injects `bc.Broadcast` as the callback (line 202)
- `runner` package has NO import of `gateway`
- `gateway` imports `runner` only for the `CoderConfirmationManager` type and `runner.NewCoderConfirmationManager`

The frontend uses the controller pattern: `coder-confirmation.ts` is a pure data module
with no side effects. `app-gateway.ts` imports the parsers and queue helpers. `app.ts`
imports the type. `coder-confirm.ts` (view) imports only the `AppViewState` type.

**Risk:** INFO — Clean dependency architecture.

**Recommendation:** None.

---

## Summary

| Risk | Count | IDs |
|------|-------|-----|
| CRITICAL | 0 | — |
| HIGH | 0 | — |
| MEDIUM | 0 | — |
| LOW | 6 | F-02, F-03, F-05, F-17, F-28, F-29 |
| INFO | 24 | F-01, F-04, F-06-F-16, F-18-F-27, F-30 |

## Verdict: **PASS**

The oa-coder visual integration is well-designed with clean separation of concerns. All
30 findings are LOW or INFO severity. Key strengths:

1. **Backward compatibility**: CoderConfirmation=nil preserves existing behavior (F-07, F-14)
2. **Security**: Broadcast scope guards correctly restrict confirmation events (F-10),
   RPC authorization requires `operator.approvals` scope (F-09), input validation
   is complete (F-13)
3. **Resource safety**: Timer cleanup via `defer timer.Stop()` (F-01), buffered channel
   prevents double-write deadlock (F-02), auto-expiry via setTimeout (F-17)
4. **No XSS**: lit template literals auto-escape all interpolated values (F-16)
5. **Correct conditional compilation**: Rust `#[cfg(feature)]` pattern compiles cleanly
   both with and without sandbox (F-23, F-24)
6. **No circular imports**: func callback pattern decouples runner and gateway (F-30)

### Actionable items (optional improvements, not blockers)

- F-05: Add `slog.Debug` on preview parse failure
- F-28: Add `max-height` to `.coder-command-mono` or truncate bash commands in Go

No blockers for archive.
