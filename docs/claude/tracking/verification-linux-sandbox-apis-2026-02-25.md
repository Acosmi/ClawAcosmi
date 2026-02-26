---
document_type: Tracking
status: In Progress
created: 2026-02-25
last_updated: 2026-02-25
audit_report: Pending
skill5_verified: true
---

# Linux Sandbox API Verification Report

Pre-implementation research for oa-sandbox OS-native isolation on Linux.

---

## Online Verification Log

### 1. Linux Namespaces + clone3 — Unprivileged User Namespaces

- **Query**: "unprivileged user namespaces linux 2024 2025 Ubuntu 24.04 Fedora desktop distributions"
- **Sources**:
  - https://man7.org/linux/man-pages/man7/user_namespaces.7.html
  - https://man7.org/linux/man-pages/man2/clone.2.html
  - https://ubuntu.com/blog/ubuntu-23-10-restricted-unprivileged-user-namespaces
  - https://github.com/lima-vm/lima/issues/2319
  - https://discussion.fedoraproject.org/t/confining-user-namespaces-with-selinux/142995
- **Key findings**:
  - `clone3()` was introduced in Linux 5.3. It accepts a `clone_args` struct with all namespace flags: `CLONE_NEWUSER`, `CLONE_NEWPID`, `CLONE_NEWNS`, `CLONE_NEWNET`, `CLONE_NEWIPC`, `CLONE_NEWUTS`. It also supports `set_tid` (since 5.5) for choosing PIDs in namespaces and `cgroup` fd (since 5.7) for placing child in a cgroup.
  - Since Linux 3.8, unprivileged processes can create user namespaces. All other namespace flags (PID, mount, net, IPC, UTS) require `CAP_SYS_ADMIN` unless combined with `CLONE_NEWUSER` (which grants full capabilities inside the new user namespace).
  - Writing UID/GID mappings requires `CAP_SETUID`/`CAP_SETGID` in the parent namespace, or the process must write its own mapping via `/proc/self/uid_map`. Mapping UID 0 requires `CAP_SETFCAP` since Linux 5.12.
  - **Ubuntu 24.04**: Restricts unprivileged user namespaces by default via AppArmor. Two sysctls control this: `kernel.apparmor_restrict_unprivileged_userns=1` and `kernel.apparmor_restrict_unprivileged_unconfined=1`. Unconfined processes cannot create user namespaces unless they have `CAP_SYS_ADMIN`. Applications needing user namespaces must have an AppArmor profile with the `userns,` rule. Workaround: set `kernel.apparmor_restrict_unprivileged_userns=0` in `/etc/sysctl.d/`.
  - **Fedora 40/41**: Does NOT restrict unprivileged user namespaces by default. There is community discussion about using SELinux to confine them (via secureblue project), but official Fedora policy has not adopted restrictions because Firefox, Flatpak, Podman, and toolbx all depend on unprivileged user namespaces.
  - **Rust crate**: `clone3` crate (v0.2.3) provides bindings. It is `unsafe` by nature. Feature flags like `linux_5-5`, `linux_5-7` enable newer clone_args fields.
- **Verified date**: 2026-02-25

### 2. Landlock LSM — API Versions and Access Rights

- **Query**: "landlock LSM API version Linux 6.x kernel access rights"
- **Sources**:
  - https://docs.kernel.org/userspace-api/landlock.html
  - https://man7.org/linux/man-pages/man7/landlock.7.html
  - https://landlock.io/rust-landlock/landlock/
- **Key findings**:
  - Landlock ABI version to kernel version mapping:
    | ABI | Kernel | New Features |
    |-----|--------|-------------|
    | 1   | 5.13   | Base FS rights: EXECUTE, WRITE_FILE, READ_FILE, READ_DIR, REMOVE_DIR, REMOVE_FILE, MAKE_CHAR, MAKE_DIR, MAKE_REG, MAKE_SOCK, MAKE_FIFO, MAKE_BLOCK, MAKE_SYM |
    | 2   | 5.19   | LANDLOCK_ACCESS_FS_REFER (file reparenting across dirs) |
    | 3   | 6.2    | LANDLOCK_ACCESS_FS_TRUNCATE |
    | 4   | 6.7    | Network: LANDLOCK_ACCESS_NET_BIND_TCP, LANDLOCK_ACCESS_NET_CONNECT_TCP |
    | 5   | 6.10   | LANDLOCK_ACCESS_FS_IOCTL_DEV (ioctl on block/char devices) |
    | 6   | 6.12   | IPC scoping: LANDLOCK_SCOPE_ABSTRACT_UNIX_SOCKET, LANDLOCK_SCOPE_SIGNAL |
  - Landlock is **unprivileged** — any process can sandbox itself. It is stackable (nested rulesets further restrict access). It uses three syscalls: `landlock_create_ruleset()`, `landlock_add_rule()`, `landlock_restrict_self()`.
  - The kernel docs recommend using ABI version detection rather than kernel version checks.
  - **Rust crate**: The `landlock` crate (https://crates.io/crates/landlock) provides a safe abstraction. It exposes `Ruleset`, `RulesetCreated`, `PathBeneath`, `NetPort`, `AccessFs`, `AccessNet`, and `Scope` enums. It supports a "best-effort" compatibility mode via the `Compatible` trait (graceful degradation on older kernels). Maintained by the Landlock project itself (Mickaël Salaün). Documentation states support for features as of Linux 5.19 (ABI 2+).
- **Verified date**: 2026-02-25

### 3. Seccomp-BPF in Rust

- **Query**: "seccomp rust crate libseccomp bindings crates.io"
- **Sources**:
  - https://crates.io/crates/libseccomp
  - https://github.com/libseccomp-rs/libseccomp-rs
  - https://docs.rs/libseccomp
- **Key findings**:
  - **`libseccomp` crate** (v0.4.0, released April 2025) is the most actively maintained option. It provides high-level safe Rust bindings over the C libseccomp library. MSRV is Rust 1.67. Requires system libseccomp >= 2.5.0. Licensed MIT/Apache-2.0. The companion `libseccomp-sys` crate provides raw FFI bindings.
  - Other crates exist but are less maintained: `seccomp` (higher-level wrapper), `libseccomp-rs` (mid-level), `seccomp-sys` (low-level), `scmp`.
  - libseccomp-rs supports: creating filter contexts, adding architectures, converting syscall names to numbers, defining conditional/unconditional rules, loading filters into the kernel.
  - **System dependency**: requires `libseccomp-dev` (or equivalent) installed on the build system. This is a C library dependency that complicates static linking and cross-compilation.
- **Verified date**: 2026-02-25

### 4. Cgroups v2 — Unprivileged Management

- **Query**: "cgroups v2 unprivileged user management memory CPU PIDs limits"
- **Sources**:
  - https://docs.kernel.org/admin-guide/cgroup-v2.html
  - https://systemd.io/CGROUP_DELEGATION/
  - https://man7.org/linux/man-pages/man7/cgroups.7.html
- **Key findings**:
  - Unprivileged cgroup management requires **delegation** — an administrator (or systemd) grants a user write access to `cgroup.procs`, `cgroup.threads`, and `cgroup.subtree_control` in a sub-tree.
  - **systemd delegation**: Set `Delegate=yes` (or specific controllers) on a scope/service unit. When `User=` is set, systemd `chown()`s the sub-tree to that user. `systemd-run --user --scope` can create a delegated scope for the current user session.
  - **`nsdelegate` mount option**: Automatically delegates cgroup management when a cgroup namespace is created. Writes to namespace-root files from inside the cgroup namespace are rejected (except files in `/sys/kernel/cgroup/delegate`).
  - **Resource limit files**:
    | File | Behavior |
    |------|----------|
    | `memory.max` | Hard limit; OOM killer invoked when exceeded |
    | `memory.high` | Throttle limit; heavy reclaim pressure, no OOM |
    | `cpu.max` | Bandwidth limit as `$MAX $PERIOD` (e.g., `50000 100000` = 50%) |
    | `pids.max` | Hard process count limit; `fork()`/`clone()` returns `-EAGAIN` when exceeded |
  - **No-internal-process constraint**: Non-root cgroups cannot have both processes and child cgroups with domain controllers enabled. Processes must be in leaf nodes. This means: create a child cgroup, move your process into it, then manage sibling cgroups for sandboxed processes.
  - **Delegable controllers**: cpu, cpuset, io, memory, pids, perf_event. The cpu and memory controllers are typically available to delegate.
- **Verified date**: 2026-02-25

### 5. sandbox-rs Crate

- **Query**: "sandbox-rs crate crates.io rust linux namespace sandboxing"
- **Sources**:
  - https://crates.io/crates/sandbox-rs
  - https://crates.io/keywords/sandbox
- **Key findings**:
  - **Yes, `sandbox-rs` does exist on crates.io**. It provides process isolation, resource limiting, and syscall filtering for Linux. It combines namespaces, cgroups, seccomp, and filesystem isolation.
  - **Critical limitation**: Requires root privileges. Running without root results in an error. This makes it unsuitable as a drop-in for our unprivileged sandbox goal.
  - Features: memory limiting, CPU usage restrictions, process timeouts, seccomp profile configuration.
  - The crate warns that sandbox escapes are possible through kernel vulnerabilities and recommends combining with AppArmor/SELinux for production use.
  - **Alternatives found**:
    - `ia-sandbox` — uses namespaces and cgroups
    - `rstrict` — uses Linux Landlock LSM (unprivileged)
    - `rusty-sandbox` — cross-platform sandbox
    - The `landlock` crate itself for unprivileged filesystem sandboxing
  - **Conclusion**: sandbox-rs is not suitable for our use case due to the root requirement. We should build our own composition using the `landlock`, `libseccomp`, and `clone3` crates directly.
- **Verified date**: 2026-02-25

### 6. PID 1 / Init Process — Zombie Reaping

- **Query**: "PID 1 init process namespace zombie reaping best practices linux"
- **Sources**:
  - https://www.marcusfolkesson.se/blog/pid1-in-containers/
  - https://academy.fpblock.com/rust/pid1/
  - https://github.com/krallin/tini
  - https://github.com/Yelp/dumb-init
  - https://blog.phusion.nl/2015/01/20/docker-and-the-pid-1-zombie-reaping-problem/
- **Key findings**:
  - When using `CLONE_NEWPID`, the first process in the namespace becomes PID 1. PID 1 has a special role: it must reap orphaned/zombie child processes by calling `waitpid()`.
  - **Signal behavior**: PID 1 in a namespace does NOT receive default signal handlers. Signals are only delivered if PID 1 has explicitly registered a handler. This means `SIGTERM` and `SIGINT` are ignored by default unless handled. If PID 1 exits, the kernel sends `SIGKILL` to all remaining processes in the namespace via `zap_pid_ns_processes()`.
  - **Zombie reaping pattern**: Install a `SIGCHLD` handler, then loop:
    ```
    while (waitpid(-1, &status, WNOHANG) > 0) { /* reap */ }
    ```
  - **Rust implementation approach** (from fpblock.com article): Use the `signal-hook` crate for signal registration. Implement a `Stream`-based async reaper: each `SIGCHLD` increments a counter, each `waitpid(-1, WNOHANG)` call decrements it. Use `libc::waitpid` with `WNOHANG` for non-blocking operation. The main loop uses `while let Some(()) = self.next().await { ... }`.
  - **Alternative**: For simplicity, a blocking approach works fine since PID 1 has nothing else to do: `loop { waitpid(-1, &mut status, 0); }` with SIGCHLD handler for the target child.
  - **Tini/dumb-init model**: Spawn a single child, forward all signals to it, reap all zombies. When the child exits, exit with its exit code. This is the proven pattern used by Docker (`--init` flag).
  - **Key gotcha**: Must forward signals to the child process group, not just the child PID, to ensure all descendants receive the signal.
- **Verified date**: 2026-02-25

---

## Design Implications and Gotchas

### Critical Corrections to Design Assumptions

1. **Ubuntu 24.04 blocks unprivileged user namespaces by default** via AppArmor. Our sandbox binary will need either: (a) an AppArmor profile with `userns,` permission, or (b) documentation telling users to set `kernel.apparmor_restrict_unprivileged_userns=0`, or (c) a fallback path that uses only Landlock+seccomp without namespaces. This is a significant portability concern.

2. **sandbox-rs requires root** — it cannot be our foundation. We must compose from lower-level primitives (`clone3` + `landlock` + `libseccomp`) ourselves.

3. **Cgroups v2 requires delegation** — an unprivileged process cannot simply create cgroups. It must either: (a) use `systemd-run --user --scope` to get a delegated subtree, or (b) rely on `nsdelegate` within a cgroup namespace. Without systemd cooperation, cgroup-based resource limits may not be available to unprivileged users.

4. **The no-internal-process constraint** in cgroups v2 means our sandbox manager must move itself to a child cgroup before creating sibling cgroups for sandboxed processes. This adds complexity to the cgroup setup logic.

5. **PID 1 signal handling is non-default** — we cannot assume SIGTERM will work. Our init process must explicitly register handlers for SIGTERM, SIGINT, SIGCHLD at minimum.

6. **libseccomp is a C library dependency** — this complicates static linking and cross-compilation. Consider whether a pure-Rust BPF assembler (writing seccomp-bpf programs directly) might be preferable for distribution simplicity, at the cost of more implementation effort.

### Recommended Architecture

```
oa-sandbox process
  |
  +-- [If available] clone3(CLONE_NEWUSER | CLONE_NEWPID | CLONE_NEWNS | ...)
  |     |
  |     +-- PID 1 init (zombie reaper + signal forwarder)
  |           |
  |           +-- Target process (execve)
  |                 |
  |                 +-- Landlock ruleset (filesystem + network)
  |                 +-- Seccomp-BPF filter (syscall allowlist)
  |
  +-- [Fallback if namespaces unavailable] Landlock + Seccomp only
  |
  +-- [If delegated] Cgroups v2 resource limits (memory, CPU, PIDs)
```

### Layer Availability Matrix

| Layer | Unprivileged? | Min Kernel | Ubuntu 24.04 | Fedora 40+ |
|-------|--------------|------------|--------------|------------|
| User namespaces | Yes* | 3.8 | Needs AppArmor profile | Yes |
| PID namespace | Via userns | 2.6.24 | Via userns | Via userns |
| Mount namespace | Via userns | 2.4.19 | Via userns | Via userns |
| Network namespace | Via userns | 2.6.29 | Via userns | Via userns |
| Landlock | Yes | 5.13 | Yes | Yes |
| Seccomp-BPF | Yes | 3.17 | Yes | Yes |
| Cgroups v2 | Via delegation | 4.15 | Via systemd | Via systemd |

*Ubuntu 24.04 requires AppArmor profile or sysctl override.

---

## Crate Selection Summary

| Purpose | Crate | Version | Status | Notes |
|---------|-------|---------|--------|-------|
| clone3 syscall | `clone3` | 0.2.3 | Maintained | Unsafe, needs careful wrapping |
| Landlock | `landlock` | latest | Actively maintained (by Landlock devs) | Best-effort compat mode |
| Seccomp | `libseccomp` | 0.4.0 | Active (April 2025) | Requires system libseccomp >= 2.5.0 |
| Signal handling | `signal-hook` | - | Well-maintained | For PID 1 SIGCHLD handling |
| Low-level Linux | `libc` / `nix` | - | Core ecosystem | For waitpid, mount, etc. |
