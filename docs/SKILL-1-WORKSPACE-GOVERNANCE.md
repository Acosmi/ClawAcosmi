# Skill 1: Precision Workspace Governance

> Prevent token waste and context pollution in a monorepo environment.

## Rationale

This codebase is a polyglot monorepo (Rust + Go + frontend). Unrestricted scanning
floods the context window with irrelevant code, degrades response quality, and wastes
tokens. All exploration must be surgically scoped.

## Rules

### 1.1 Workspace Boundary Lock

| Scope Level | Allowed Paths | Notes |
|---|---|---|
| **Primary** | `cli-rust/crates/oa-cmd-sandbox/`, `cli-rust/crates/oa-runtime/` | Core sandbox implementation |
| **Supporting** | `cli-rust/crates/oa-types/`, `cli-rust/crates/oa-config/`, `cli-rust/crates/oa-infra/` | Shared types & infrastructure |
| **Docs** | `docs/`, `docs/claude/` | Documentation & tracking |
| **Context** | `Dockerfile.sandbox*`, `docker-compose.yml` | Container-level config |

Anything outside these paths requires **explicit user authorization** or a stated
dependency reason before access.

### 1.2 Prohibited Operations

The following are **strictly forbidden** at the project root (`/`):

- `find .` / `tree` / `ls -R` (recursive listing)
- Unbounded `grep -r` / `rg` without path constraint
- `cat` or `Read` on files > 500 lines without `offset`/`limit`
- Glob patterns like `**/*` at root level

### 1.3 Exploration Protocol

When information outside the primary workspace is needed:

1. **Ask first** — confirm the target path with the user if uncertain.
2. **Single-level `ls`** — explore one directory layer at a time.
3. **Targeted read** — use `Read` with `offset`/`limit` for large files; read only the
   relevant section.
4. **Document the reason** — if cross-module access is necessary, note it in the current
   tracking document.

### 1.4 Context Budget Discipline

- Prefer `Grep` with `head_limit` over open-ended searches.
- When reading Cargo.toml / go.mod for dependency info, read only the `[dependencies]`
  section.
- Summarize findings inline rather than dumping raw file contents into conversation.
