# Skill 4: Granular Code-Level Review Audit

> "LGTM" is not an audit. Every line must earn its place.

## Rationale

Sandbox code is a security boundary. A cursory review that misses a path traversal,
an unchecked error branch, or a race condition can have severe consequences. This skill
enforces rigorous, line-level analysis with documented output.

## Rules

### 4.1 Audit Depth Requirements

**Minimum depth: line-by-line for all changed or new code.**

The following superficial responses are **explicitly banned**:

- "Code looks good / LGTM"
- "Logic appears correct"
- "No obvious issues"
- Summarizing what the code does without analyzing correctness

Every audit must demonstrate **active reasoning** about:
- What each code path does under normal conditions
- What each code path does under error / edge-case conditions
- What assumptions are being made and whether they hold

### 4.2 Audit Checklist

Each audit report must address every applicable category:

#### Security
- [ ] **Path traversal**: Can user-controlled input escape intended directories?
      (Check for `..`, symlink following, TOCTOU on path checks)
- [ ] **Namespace/permission boundaries**: Are isolation barriers correctly applied?
      (Seatbelt profiles, Linux namespaces, ACLs, seccomp filters)
- [ ] **Privilege escalation**: Can a sandboxed process gain capabilities it shouldn't have?
- [ ] **Input validation**: Are all external inputs (CLI args, env vars, IPC messages,
      file contents) validated before use?

#### Resource Safety
- [ ] **Error path cleanup**: On every `Err` return, are resources (FDs, processes,
      temp files, locks) properly released?
- [ ] **Panic path cleanup**: If a panic occurs (despite Skill 3 rules), do `Drop`
      impls handle cleanup?
- [ ] **Concurrency safety**: Are shared resources protected? Is lock ordering consistent?
      Any `.await` while holding a lock?
- [ ] **Leak potential**: Can any code path leave behind zombie processes, open FDs,
      or temp files?

#### Correctness
- [ ] **Edge cases**: Empty input, maximum-length input, Unicode, null bytes,
      concurrent access
- [ ] **Platform differences**: Does the code behave correctly across target platforms
      (macOS/Linux/Windows)?
- [ ] **Integer overflow**: Any arithmetic on sizes, offsets, or counts that could overflow?
- [ ] **Type safety**: Are `as` casts safe? Prefer `try_into()` for fallible conversions.

### 4.3 Audit Report Format

Store in `docs/claude/audit/` with naming convention:
`audit-YYYY-MM-DD-<component>-<brief-description>.md`

```markdown
---
document_type: Audit
status: Complete
created: YYYY-MM-DD
last_updated: YYYY-MM-DD
scope: <list of files/modules reviewed>
verdict: Pass | Pass with Notes | Fail
---

# Audit Report: <Component> — <Brief Description>

## Scope
- Files reviewed: [list with line ranges]
- Commit/change reference: [description of what triggered this audit]

## Findings

### [PASS | WARN | FAIL] <Category>: <Brief Title>
- **Location**: `file.rs:LL-LL`
- **Analysis**: <Detailed reasoning about correctness/incorrectness>
- **Risk**: None | Low | Medium | High | Critical
- **Recommendation**: <Specific fix if applicable>

(Repeat for each finding)

## Summary
- Total findings: N (X pass, Y warnings, Z failures)
- Blocking issues: [list or "None"]
- Recommendation: [Approve / Approve with conditions / Reject]
```

### 4.4 Audit Triggers

An audit **must** be performed when:

1. Moving any item to `archive/` (Skill 2 interlock)
2. Completing a tracking document's final checkbox
3. Introducing or modifying `unsafe` code
4. Changing namespace/sandbox/permission enforcement logic
5. Modifying process lifecycle management (spawn, kill, wait)
6. User explicitly requests a review

### 4.5 Post-Audit Actions

After completing an audit:

1. Save report to `docs/claude/audit/`
2. Update the originating tracking document:
   - Set `audit_report` field to the report path
   - Set `status` to `Auditing` (or `Archived` if moving to archive)
3. If findings include `FAIL` items:
   - Create new tracking items for required fixes
   - Block the archive gate until fixes are resolved and re-audited
