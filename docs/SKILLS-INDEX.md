# oa-sandbox Agent Skills Index

> Core execution directives for the oa-sandbox development agent.

## Skills Overview

| # | Skill | Purpose | Interlock |
|---|---|---|---|
| 1 | [Workspace Governance](./SKILL-1-WORKSPACE-GOVERNANCE.md) | Prevent token waste, scope all operations to sandbox modules | — |
| 2 | [DocOps](./SKILL-2-DOCOPS.md) | Document-driven task lifecycle with traceability | Gate → Skill 4 |
| 3 | [Rust Sandbox Coding Standard](./SKILL-3-RUST-SANDBOX-CODING.md) | Zero-panic, resource-safe, security-critical coding rules | Input → Skill 4 |
| 4 | [Code Audit](./SKILL-4-CODE-AUDIT.md) | Line-level security & correctness review | Gate → Skill 2 archive |
| 5 | [Online Verification](./SKILL-5-ONLINE-VERIFICATION.md) | Verify OS/API assumptions against authoritative sources before coding | Gate → implementation |

## Skill Dependency Graph

```
  Skill 5 (Verify)
       │
       ▼
  Skill 2 (Track)  ──────────────────┐
       │                              │
       ▼                              ▼
  Skill 3 (Code)  ───►  Skill 4 (Audit)
                              │
                              ▼
                     Skill 2 (Archive)
```

## Workspace Directory Layout

```
docs/
  SKILLS-INDEX.md              ← You are here
  SKILL-1-WORKSPACE-GOVERNANCE.md
  SKILL-2-DOCOPS.md
  SKILL-3-RUST-SANDBOX-CODING.md
  SKILL-4-CODE-AUDIT.md
  SKILL-5-ONLINE-VERIFICATION.md
  claude/
    tracking/                  ← Active task decomposition
    deferred/                  ← TODOs, blockers, tech debt
    audit/                     ← Code review reports
    archive/                   ← Completed & audited items
```

## Quick Reference: When Does Each Skill Activate?

| Trigger | Skills Activated |
|---|---|
| New task received | 5 → 2 → 1 |
| Writing sandbox code | 3, 1 |
| Task complete, ready to close | 4 → 2 (archive gate) |
| Encountered blocker or TODO | 2 (deferred) |
| Cross-module exploration needed | 1 (boundary check) |
| OS API design decision | 5 (mandatory verify) |
