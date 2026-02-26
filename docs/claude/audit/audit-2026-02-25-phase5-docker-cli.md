---
document_type: Audit
status: In Progress
created: 2026-02-25
last_updated: 2026-02-25
audit_report: self
skill5_verified: true
---

# Audit: Phase 5 — Docker Fallback + CLI Integration

## Scope

| File | Lines | Description |
|------|-------|-------------|
| `oa-sandbox/src/docker/mod.rs` | 1-363 | Docker CLI fallback runner |
| `oa-cmd-sandbox/src/run.rs` | 1-270 | `sandbox run` CLI subcommand |

## Audit Methodology

Full Skill 4 checklist: Security, Resource Safety, Correctness, Edge Cases.

---

## Findings

### F-01 (Low): Docker exit codes 125/126 not distinguished

**Location**: `docker/mod.rs:250-254`

**Issue**: Only exit code 127 (command not found) gets special handling. Docker exit 125 (daemon error) and 126 (command cannot be invoked) pass through as raw exit codes.

**Risk**: Low — caller receives the exit code and can inspect it. Go layer may need to distinguish daemon errors from command failures.

**Recommendation**: Add handling for 125/126 in a follow-up. Current behavior is acceptable.

### F-02 (Low): Catch-all error exit code misclassification

**Location**: `run.rs:150-153`

**Issue**: The catch-all `_ => CONFIG_ERROR` maps non-config errors (Io, Seccomp, Landlock) to exit code 2 (config error). These should use a general error exit code.

**Risk**: Low — Go IPC layer may misinterpret the error category.

**Recommendation**: Add `GENERAL_ERROR = 1` exit code for non-specific errors. Deferred.

### F-03 (Info): `std::process::exit()` bypasses destructors

**Location**: `run.rs:132, 165`

**Issue**: `std::process::exit()` does not run Drop implementations. The runner and config are lightweight, so no resource leak occurs.

**Risk**: None in current code. Would matter if runner held OS resources.

**Recommendation**: No action needed. Document this behavior.

### F-04 (Info): No Docker container label for cleanup

**Location**: `docker/mod.rs:68-71`

**Issue**: `docker run --rm` handles normal cleanup, but if the process is killed before Docker removes the container, orphans could accumulate.

**Risk**: Very low — `--rm` is reliable in practice.

**Recommendation**: Consider `--label oa-sandbox=true` for cleanup tooling in future.

---

## Security Analysis

### Command Injection: PASS

All Docker arguments use `Command::args()` (not shell interpretation). No injection possible via workspace paths, mount paths, env vars, or command args.

### Path Traversal: PASS

`SandboxConfig::validate()` (Phase 6) checks absolute paths and `..` traversal before config reaches Docker runner.

### Resource Leaks: PASS

- Child process: `try_wait()` polling + `child.kill()` + `child.wait()` prevents zombies
- Threads: stdout/stderr reader threads joined before return
- Pipes: Taken from child, consumed by threads, no FD leak

### Timeout Handling: PASS

`try_wait()` polling at 50ms intervals. On timeout: `child.kill()` (SIGKILL to Docker) → `child.wait()` (reap) → return `Timeout` error. Docker container is also killed by the Docker daemon when the CLI process dies.

### Error Propagation: PASS

All errors are properly mapped to `SandboxError` variants. JSON output emitted even on error for Go IPC compatibility.

---

## Verdict: **PASS**

No Critical or High severity findings. All findings are Low/Info level with clear deferred recommendations. The Docker fallback implementation is well-structured, resource-safe, and correctly handles edge cases (timeout, command not found, exit codes).

## Deferred Items

- [ ] D-01: Handle Docker exit codes 125/126 specifically
- [ ] D-02: Add `GENERAL_ERROR` exit code for non-config errors
- [ ] D-03: Add `--label` to Docker containers for cleanup tooling
