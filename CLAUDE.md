# OpenAcosmi — oa-sandbox Agent Directives

> This file is auto-loaded by Claude Code. It defines mandatory execution rules for
> the oa-sandbox development agent. Full specifications are in `docs/SKILL-*.md`.

## Project Overview

Polyglot monorepo: Rust CLI (`cli-rust/`) + Go backend (`backend/`) + frontend (`ui/`).
Primary focus module: **oa-sandbox** (sandbox/container isolation subsystem).

---

## Skill 1: Workspace Governance

> Prevent token waste. Scope all operations surgically. Full spec: `docs/SKILL-1-WORKSPACE-GOVERNANCE.md`

### Allowed Paths

| Scope | Paths |
|---|---|
| Primary | `cli-rust/crates/oa-cmd-sandbox/`, `cli-rust/crates/oa-runtime/` |
| Supporting | `cli-rust/crates/oa-types/`, `cli-rust/crates/oa-config/`, `cli-rust/crates/oa-infra/` |
| Docs | `docs/`, `docs/claude/` |
| Context | `Dockerfile.sandbox*`, `docker-compose.yml` |

Anything outside requires **explicit user authorization**.

### Forbidden at Project Root

- `find .` / `tree` / `ls -R` (recursive listing)
- Unbounded `grep -r` / `rg` without path constraint
- `Read` on files >500 lines without `offset`/`limit`
- Glob `**/*` at root level

### Exploration Protocol

1. Ask first if target path is uncertain
2. Single-level `ls` only
3. Targeted `Read` with offset/limit for large files
4. Document reason for cross-module access in tracking doc

---

## Skill 2: DocOps (Document-Driven Operations)

> Every task is traceable from plan to archive. Full spec: `docs/SKILL-2-DOCOPS.md`

### Directory Structure

```
docs/claude/
  tracking/     # Active task decomposition & progress
  deferred/     # TODOs, blockers, gotchas, tech debt
  audit/        # Code-level review reports (Skill 4)
  archive/      # Completed & audited items (read-only)
```

### Mandatory Document Header

Every `docs/claude/**/*.md` file must start with:

```yaml
---
document_type: Tracking | Deferred | Audit | Archive
status: Draft | In Progress | Auditing | Archived
created: YYYY-MM-DD
last_updated: YYYY-MM-DD
audit_report: Pending | <path to audit report>
skill5_verified: true | false
---
```

### Task Lifecycle

```
[Plan] → [Track] → [Execute] → [Audit] → [Archive]
```

- **Plan**: Decompose into checkbox items in `tracking/`. Trigger Skill 5 verification.
- **Track**: Check off `[x]` as steps complete.
- **Defer**: Immediately log blockers/TODOs/gotchas to `deferred/`. Never leave them only in conversation.
- **Audit**: Trigger Skill 4 before closing.
- **Archive**: Requires completed audit report (Skill 4 gate). No exceptions.

### Archive Gate

**Hard stop** — no item moves to `archive/` without:
- All checkboxes `[x]`
- Audit report in `audit/` covering every code change
- Audit link populated in document header
- Status set to `Archived`

---

## Skill 3: Rust Sandbox Coding Standard

> Zero-panic, resource-safe, security-critical code. Full spec: `docs/SKILL-3-RUST-SANDBOX-CODING.md`

### 3.1 Zero-Panic Policy

Production code (non-test) **must never panic**:

| Banned | Use Instead |
|---|---|
| `.unwrap()` | `.context("...")?` / `.map_err(...)?` |
| `.expect()` | `.ok_or_else(\|\| Error::...)?` |
| `panic!()` / `unreachable!()` / `todo!()` | `return Err(...)` |
| `array[i]` on untrusted input | `.get(i).ok_or(...)?` |

Exceptions: `#[cfg(test)]`, truly infallible static init (with `// SAFETY:` comment).

### 3.2 Error Propagation

- `thiserror` for library errors (typed, matchable)
- `anyhow` for CLI/application errors (contextual)
- Every `?` in public functions needs `.context()` or typed error
- Preserve underlying `io::Error` via `#[source]`
- Include identifiers (PID, path, fd) in error messages

### 3.3 Unsafe Boundary Control

- **Minimize scope**: wrap only the FFI call
- **`// SAFETY:` comment mandatory**: explain invariants, soundness, and failure modes
- **Encapsulate**: safe public API over internal `unsafe`; never expose `unsafe` publicly
- All `unsafe` blocks are high-priority Skill 4 audit targets

### 3.4 Resource Lifecycle

| Resource | Release | Failure Mode |
|---|---|---|
| Child process | `wait()` / `kill()+wait()` via `Drop` | Zombie process |
| File descriptor | `Drop` on `OwnedFd` | FD exhaustion |
| Temp directory | `Drop` on `TempDir` | Disk fill |
| Job Object (Win) | `CloseHandle` via Drop wrapper | Orphan processes |
| Network namespace | Process exit / explicit teardown | Stale namespace |

Every `Command::spawn()` needs a reap path. Use RAII (`Drop`, `scopeguard`) for cleanup.

### 3.5 Concurrency

- Prefer message passing (`mpsc`, `oneshot`) over `Arc<Mutex<T>>`
- Document lock ordering if shared state is necessary
- Never hold a lock across `.await`
- Document cancel-safety of `tokio::select!` branches

---

## Skill 4: Granular Code-Level Audit

> "LGTM" is not an audit. Full spec: `docs/SKILL-4-CODE-AUDIT.md`

### Banned Responses

- "Code looks good / LGTM"
- "Logic appears correct"
- "No obvious issues"
- Summarizing without analyzing correctness

### Audit Checklist

**Security:**
- Path traversal (`..`, symlinks, TOCTOU)
- Namespace/permission boundary correctness
- Privilege escalation vectors
- Input validation (CLI args, env vars, IPC, files)

**Resource Safety:**
- Error path cleanup (FDs, processes, temp files, locks)
- Panic path cleanup (`Drop` impls)
- Concurrency safety (lock ordering, `.await` + lock)
- Leak potential (zombies, FDs, temp files)

**Correctness:**
- Edge cases (empty, max-length, Unicode, null bytes, concurrency)
- Platform differences (macOS/Linux/Windows)
- Integer overflow
- Type safety (`as` casts → prefer `try_into()`)

### Report Format

Store in `docs/claude/audit/` as `audit-YYYY-MM-DD-<component>-<description>.md`.
Must include: scope, per-finding analysis with location/risk/recommendation, and verdict.

### Audit Triggers

1. Archive gate (Skill 2 interlock)
2. Final checkbox in tracking doc
3. New or modified `unsafe` code
4. Changes to namespace/sandbox/permission logic
5. Process lifecycle changes (spawn/kill/wait)
6. User request

### Post-Audit

- Save report → update tracking doc header with link → if FAIL: create fix items, block archive

---

## Skill 5: Pre-Planning Online Verification

> Verify before you build. Full spec: `docs/SKILL-5-ONLINE-VERIFICATION.md`

### When Mandatory

Before writing code involving: OS syscalls, platform security APIs (Seatbelt, namespaces,
Job Objects), kernel features (cgroups, eBPF, landlock), new crate adoption, container
runtime behavior, or filesystem semantics.

### Source Hierarchy

1. **Official docs**: Apple Developer, Microsoft Learn, man7.org
2. **Ecosystem docs**: docs.rs, crates.io, doc.rust-lang.org
3. **Source code**: kernel.org, GitHub (runc/containerd)
4. **Specs**: POSIX, OCI runtime spec, LSM docs
5. **Tech blogs**: LWN.net (cross-reference required)

Non-authoritative sources (SO, random blogs) need cross-reference with tier 1-4.

### Verification Record

Every planning doc must include:

```markdown
## Online Verification Log
### <Topic>
- **Query**: <search terms>
- **Source**: <URL>
- **Key finding**: <1-3 sentences>
- **Verified date**: YYYY-MM-DD
```

### Gate

`skill5_verified: false` in document header → **must not** proceed to implementation.

---

## Skill Activation Quick Reference

| Trigger | Skills |
|---|---|
| New task received | 5 → 2 → 1 |
| Writing sandbox code | 3, 1 |
| Task complete | 4 → 2 (archive gate) |
| Blocker / TODO found | 2 (deferred) |
| Cross-module access | 1 (boundary check) |
| OS/API design decision | 5 (mandatory verify) |

## Dependency Graph

```
Skill 5 (Verify) → Skill 2 (Track) → Skill 3 (Code) → Skill 4 (Audit) → Skill 2 (Archive)
```
