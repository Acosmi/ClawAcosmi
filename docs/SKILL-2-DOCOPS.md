# Skill 2: Document-Driven Operations (DocOps)

> Every task must be traceable from inception through execution to archival.

## Rationale

Undocumented decisions evaporate between sessions. DocOps ensures continuity, prevents
repeated work, and creates an auditable trail of all changes to the sandbox subsystem.

## Directory Structure

```
docs/claude/
  tracking/     # Active task decomposition & progress
  deferred/     # Parking lot: TODOs, blockers, gotchas, tech debt
  audit/        # Code-level review reports (Skill 4 output)
  archive/      # Completed & audited items (read-only)
```

## Rules

### 2.1 Mandatory Document Header

Every Markdown file created under `docs/claude/` **must** begin with this YAML
front-matter block:

```yaml
---
document_type: Tracking | Deferred | Audit | Archive
status: Draft | In Progress | Auditing | Archived
created: YYYY-MM-DD
last_updated: YYYY-MM-DD
audit_report: Pending | <relative path to docs/claude/audit/report.md>
skill5_verified: true | false  # Was online verification performed?
---
```

### 2.2 Task Lifecycle

```
 [Plan] ──► [Track] ──► [Execute] ──► [Audit] ──► [Archive]
   │           │            │             │            │
   │     Create tracking   Update        Generate     Move to
   │     doc with [ ]      checkboxes    audit        archive/
   │     task items        to [x]        report       with link
   │
   └─ Skill 5: Verify plan against authoritative sources
```

**Phase details:**

| Phase | Action | Output Location |
|---|---|---|
| Plan | Decompose task into granular checkbox items | `tracking/` |
| Track | Check off `[x]` as each step completes | `tracking/` (in-place update) |
| Defer | Log blockers, future TODOs, gotchas immediately | `deferred/` |
| Audit | Line-level code review before closing | `audit/` (via Skill 4) |
| Archive | Move completed + audited items | `archive/` |

### 2.3 Deferred Items Policy

When encountering any of the following during execution, **immediately** create or
append to a file in `deferred/`:

- A bug or edge case discovered but not in current scope
- A TODO or FIXME left in code
- A design concern or tech debt observation
- A dependency upgrade or API deprecation notice
- Platform-specific behavior that needs future testing

**Never** leave deferred items only in conversation context — they will be lost.

### 2.4 Archive Gate (Interlock with Skill 4)

**No item may be moved to `archive/` without a corresponding audit report.**

Archive checklist:
- [ ] All tracking checkboxes are `[x]`
- [ ] Audit report exists in `audit/` and covers every code change
- [ ] Audit report link is populated in the document header
- [ ] Document status is updated to `Archived`
- [ ] Original tracking doc header updated with audit link

Violation of this gate is a **hard stop** — ask the user before proceeding.
