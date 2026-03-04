# Skill 3: Rust Sandbox System-Level Coding Standard

> Safety-critical code demands zero-tolerance for runtime surprises.

## Rationale

The sandbox subsystem operates at the OS boundary — spawning processes, managing
namespaces, enforcing security policies. A panic or resource leak here doesn't just
crash a feature; it can orphan processes, leak file descriptors, or open security holes.

## Rules

### 3.1 Zero-Panic Policy

**Production code must never panic.** The following are banned in non-test code:

| Banned | Required Alternative |
|---|---|
| `.unwrap()` | `.map_err(...)? ` or `.context("...")?` |
| `.expect("...")` | `.ok_or_else(\|\| Error::...)? ` |
| `panic!()` | `return Err(...)` |
| `unreachable!()` | `return Err(Error::InternalBug("..."))` |
| `todo!()` | Compile-time `#[cfg]` gate or explicit `Err` |
| Array index `[i]` on untrusted input | `.get(i).ok_or(...)?` |

**Exception:** `unwrap()` is permitted in:
- Unit tests (`#[cfg(test)]`)
- Static initialization where failure is truly impossible (must have `// SAFETY:` comment)
- Builder patterns where the type system guarantees validity

### 3.2 Error Propagation Standard

```rust
// Use thiserror for library-facing errors (typed, matchable)
#[derive(Debug, thiserror::Error)]
pub enum SandboxError {
    #[error("namespace setup failed: {0}")]
    NamespaceSetup(#[source] std::io::Error),

    #[error("policy violation: process {pid} attempted {action}")]
    PolicyViolation { pid: u32, action: String },
}

// Use anyhow for application-level / CLI-facing errors (contextual)
fn setup_sandbox(config: &Config) -> anyhow::Result<Sandbox> {
    let ns = create_namespace(&config.ns_config)
        .context("failed to create sandbox namespace")?;
    // ...
}
```

**Error context requirements:**
- Every `?` propagation in a public function should have `.context()` or a typed error
- OS errors must preserve the underlying `io::Error` (via `#[source]` or `.source()`)
- Include relevant identifiers (PID, path, fd number) in error messages

### 3.3 Unsafe Boundary Control

```rust
// SAFETY: `dup2` is called with two valid file descriptors obtained from
// `pipe()` above. The old fd is closed after duplication. This is safe
// because we hold exclusive ownership of both descriptors in this scope.
unsafe {
    libc::dup2(read_fd, libc::STDIN_FILENO);
}
```

**Rules for `unsafe` blocks:**

1. **Minimize scope** — wrap only the exact FFI call, not surrounding safe logic.
2. **`// SAFETY:` comment is mandatory** — explain:
   - What invariants the caller guarantees
   - Why this specific usage is sound
   - What could go wrong if the invariants are violated
3. **Encapsulate** — expose a safe public API over internal `unsafe` calls. Never let
   `unsafe` leak into the public interface.
4. **Audit tag** — all `unsafe` blocks are high-priority targets in Skill 4 audits.

### 3.4 Resource Lifecycle & Leak Prevention

| Resource | Acquisition | Release Mechanism | Failure Mode |
|---|---|---|---|
| Child process | `Command::spawn()` | `wait()` / `kill()` + `wait()` | Zombie process |
| File descriptor | `open()` / `pipe()` | `Drop` on `OwnedFd` / explicit `close()` | FD exhaustion |
| Temp directory | `tempdir()` | `Drop` on `TempDir` | Disk fill |
| Job Object (Win) | `CreateJobObject` | `CloseHandle` via Drop wrapper | Orphan processes |
| Network namespace | `unshare(CLONE_NEWNET)` | Process exit / explicit teardown | Stale namespace |

**Mandatory patterns:**

```rust
// Wrap raw handles in RAII types
struct ProcessGuard {
    child: Child,
}

impl Drop for ProcessGuard {
    fn drop(&mut self) {
        // Best-effort kill + reap to prevent zombies
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}
```

- Every `Command::spawn()` must have a corresponding reap path (normal + error + panic).
- Use `scopeguard` or `Drop` impls for cleanup that must happen regardless of control flow.
- In `async` code, ensure spawned tasks are joined or aborted on parent cancellation.

### 3.5 Concurrency Safety

- Prefer message passing (`mpsc`, `oneshot`) over shared mutable state.
- If `Arc<Mutex<T>>` is necessary, document the lock ordering to prevent deadlocks.
- Never hold a lock across an `.await` point.
- Use `tokio::select!` with cancellation safety in mind — document which branches are
  cancel-safe.
