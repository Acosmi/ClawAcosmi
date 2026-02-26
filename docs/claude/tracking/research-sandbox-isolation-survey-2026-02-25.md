---
document_type: Tracking
status: In Progress
created: 2026-02-25
last_updated: 2026-02-25
audit_report: Pending
skill5_verified: true
---

# Research Survey: Open-Source Sandbox & Container Isolation Implementations

## Purpose

Identify best practices, architecture patterns, and lessons learned from leading
open-source sandbox and container isolation projects. Inform oa-sandbox design
decisions for multi-platform (Linux, macOS, Windows) process isolation.

---

## Online Verification Log

### Chromium Sandbox Architecture
- **Query**: Chromium sandbox architecture design Linux seccomp namespaces macOS Seatbelt Windows
- **Source**: https://chromium.googlesource.com/chromium/src/+/HEAD/docs/design/sandbox.md
- **Key finding**: Chromium uses a broker/target multi-process model with platform-specific Layer-1 (semantics/namespace) and Layer-2 (attack surface reduction/seccomp-BPF) sandboxing.
- **Verified date**: 2026-02-25

### Firecracker Micro-VM
- **Query**: Firecracker microvm architecture jailer cold start design Rust KVM
- **Source**: https://github.com/firecracker-microvm/firecracker
- **Key finding**: Jailer sets up privileged resources (cgroups, chroot, namespaces), drops privileges, then exec()s into Firecracker. <125ms cold start with <5MB overhead per VM.
- **Verified date**: 2026-02-25

### gVisor User-Space Kernel
- **Query**: gVisor architecture Sentry Gofer seccomp ptrace syscall interception
- **Source**: https://gvisor.dev/docs/architecture_guide/security/
- **Key finding**: Sentry reimplements Linux syscall API in Go, reducing host kernel exposure from ~350 syscalls to 68. Systrap platform uses SECCOMP_RET_TRAP for interception.
- **Verified date**: 2026-02-25

### Bubblewrap (bwrap)
- **Query**: Bubblewrap bwrap unprivileged sandboxing namespace creation fallback
- **Source**: https://github.com/containers/bubblewrap
- **Key finding**: Creates empty mount namespace on tmpfs invisible to host. Uses PR_SET_NO_NEW_PRIVS. Supports graceful fallback with --unshare-*-try flags.
- **Verified date**: 2026-02-25

### Claude Code Sandbox Runtime (Anthropic)
- **Query**: Claude Code CLI sandbox implementation Anthropic
- **Source**: https://github.com/anthropic-experimental/sandbox-runtime
- **Key finding**: Uses native OS primitives (sandbox-exec on macOS, bubblewrap+seccomp on Linux) plus proxy-based network filtering. No container required.
- **Verified date**: 2026-02-25

### Deno Permission Model
- **Query**: Deno permission security model implementation
- **Source**: https://docs.deno.com/runtime/fundamentals/security/
- **Key finding**: Deny-by-default with fine-grained --allow-* flags. Permission checks in Rust ops layer. Known gap: subprocesses/FFI can bypass sandbox.
- **Verified date**: 2026-02-25

### Rust Sandbox Crates
- **Query**: Rust sandbox crate library crates.io landlock extrasafe gaol
- **Source**: https://lib.rs/crates/extrasafe, https://crates.io/crates/landlock, https://github.com/servo/gaol
- **Key finding**: extrasafe provides deny-by-default seccomp+landlock with RuleSet trait. gaol offers cross-platform whitelist profiles. hakoniwa combines namespaces+cgroups+landlock+seccomp.
- **Verified date**: 2026-02-25

---

## 1. Chromium Sandbox (Google)

### Architecture Overview

Chromium implements a **multi-process broker/target model**:

- **Broker process** (browser): Privileged controller that spawns sandboxed
  child processes and mediates access to resources via IPC.
- **Target processes** (renderers, GPU, plugins): Run with maximum restrictions.
  Cannot directly access filesystem, network, or display.

The sandbox operates in two layers:

| Layer | Purpose | Linux | macOS | Windows |
|-------|---------|-------|-------|---------|
| Layer-1 (Semantics) | Prevent access to most resources | Namespace sandbox (user/PID/net/mount) replacing legacy setuid sandbox | Seatbelt profiles (sandbox-exec) | Restricted tokens + Job objects + alternate desktop |
| Layer-2 (Attack Surface) | Reduce kernel attack surface | seccomp-BPF syscall filtering | (integrated into Seatbelt) | Integrity levels (untrusted/low) |

#### Linux Specifics
- **Namespace sandbox**: Creates new network, PID, mount, and user namespaces.
  Replaces the older setuid sandbox (SUID binary that called chroot).
- **seccomp-BPF**: Per-process BPF programs compiled to filter syscalls. Kernel
  can raise SIGSYS to allow userland emulation or IPC brokering of denied calls.
- **Zygote process**: Pre-forked template process that renderers clone from,
  enabling faster startup while inheriting sandbox constraints.

#### macOS Specifics
- **Seatbelt sandbox profiles**: Declarative policy files controlling file,
  network, IPC, and Mach port access.
- **Warm-up phase elimination**: As of 2025, Chromium rewrote profiles to sandbox
  child processes from the very start, eliminating the previous unsandboxed
  warm-up phase that acquired system resources before entering the sandbox.

#### Windows Specifics
- **Restricted tokens**: Derived from user token with removed privileges, SIDs
  set to deny-only, and restricted SIDs added.
- **Job objects**: Hard process quota of 1 (no child spawning), enforced limits
  on global resources without traditional security descriptors.
- **Alternate desktop**: Third desktop object with strict security descriptor
  isolates target processes from user desktop message attacks.
- **Integrity levels**: Untrusted/low integrity prevents write-up to higher
  integrity objects.
- **Interception (hooks)**: Windows API calls transparently forwarded via
  sandbox IPC to broker for policy-checked execution.

### Key Design Patterns

1. **Broker/Target separation**: All resource acquisition happens in the broker.
   Targets receive only pre-opened handles/FDs via IPC.
2. **Policy-as-code**: Sandbox restrictions are programmatic (not config files),
   allowing fine-grained per-process-type policies.
3. **Layered defense**: Two independent layers so a bypass of one still leaves
   the other intact.
4. **Platform abstraction with platform-specific backends**: Common conceptual
   model (broker + restricted target) with entirely different OS mechanisms.
5. **Deny-by-default**: Targets start with zero access; everything is explicitly
   granted through the broker.

### Relevance to oa-sandbox

- The **broker/target IPC model** is directly applicable. Our sandbox should have
  a privileged orchestrator that pre-opens resources and passes FDs/handles.
- The **two-layer approach** (namespace + seccomp) should be our default on Linux.
- The **Seatbelt profile approach** for macOS is the proven path -- Anthropic's
  Claude Code uses the same mechanism.
- On Windows, **restricted tokens + Job objects** is the established pattern.
- The **Zygote/pre-fork model** could accelerate repeated sandbox creation.

---

## 2. Firecracker (AWS)

### Architecture Overview

Firecracker is a **virtual machine monitor (VMM)** written in Rust that uses
Linux KVM to create microVMs. It originated from Chromium OS's crosvm.

Core architecture:
- **Three threads**: API thread (REST), VMM thread (device emulation), vCPU thread(s)
- **Minimal device model**: Only 5 emulated devices (virtio-net, virtio-block,
  virtio-vsock, serial console, minimal keyboard controller)
- **~50,000 lines of Rust** (96% reduction vs QEMU)

### The Jailer Component

The jailer implements a **setup-then-drop-privileges** pattern:

```
[Root] --> Create cgroup dirs --> Create chroot dir --> Copy binary
       --> Set up /dev nodes --> Apply cgroup limits
       --> Enter namespaces (PID, net, mount) --> chroot
       --> Apply seccomp filter (level 2: whitelist + parameter filtering)
       --> Drop to unprivileged user --> exec() Firecracker
```

After exec(), Firecracker can ONLY access resources that were explicitly placed
in the chroot or passed as file descriptors.

Seccomp levels:
- Level 0: Disabled
- Level 1: Whitelist syscall numbers only
- Level 2: Whitelist syscalls + trusted parameter values (recommended)

### Fast Cold Start Design

Achieving <125ms cold starts through:
1. **Minimal device emulation**: 5 devices vs QEMU's dozens
2. **No BIOS/UEFI**: Direct kernel boot
3. **Stripped guest kernel**: Minimal config matching the 5 devices
4. **Pre-built rootfs**: No package installation at boot
5. **Memory overhead <5MB per VM**: Enables thousands on one host

### Key Design Patterns

1. **Privilege escalation then permanent drop**: Jailer acquires privileged
   resources, then irrevocably drops to unprivileged before exec().
2. **Defense in depth**: KVM hardware isolation + jailer (cgroups + chroot +
   namespaces + seccomp) + minimal attack surface.
3. **Minimal attack surface by design**: Remove features, not just restrict them.
   Fewer devices = fewer bugs = fewer exploits.
4. **Rust for safety-critical VMM code**: Memory safety eliminates buffer
   overflows in the VMM itself.
5. **Separate jailer binary**: Security setup code is isolated from runtime code,
   making each independently auditable.

### Relevance to oa-sandbox

- The **jailer pattern** (privileged setup -> drop privileges -> exec target) is
  directly applicable to our sandbox launcher.
- **Seccomp level 2** (parameter filtering) provides stronger guarantees than
  simple syscall whitelisting.
- The **minimal attack surface principle** -- strip features we do not need.
- The **separate setup binary** pattern keeps our sandbox entry point auditable.
- Rust's ownership model for resource management is validated at AWS scale.

---

## 3. gVisor (Google)

### Architecture Overview

gVisor implements a **user-space kernel (Sentry)** that intercepts and handles
application syscalls without passing most of them to the host kernel.

```
Application --> [syscall] --> Sentry (user-space kernel, Go)
                                |
                                +--> [68 host syscalls via seccomp-filtered path]
                                |
                                +--> Gofer (file proxy, separate process)
                                       |
                                       +--> Host filesystem
```

Key components:
- **Sentry**: Reimplements ~350 Linux syscalls in Go. Runs as a user-space
  process. Applies its own seccomp-BPF filter allowing only 68 host syscalls.
- **Gofer**: Separate process that provides file system access. Communicates
  with Sentry via 9P protocol. Can be further restricted.
- **Systrap platform** (default since mid-2023): Uses SECCOMP_RET_TRAP to
  intercept syscalls via SIGSYS signal, replacing the slower ptrace approach.

### Syscall Filtering Approach

gVisor's seccomp-BPF optimizations (as of 2024):
- Converted linear syscall number search to **binary search tree** in BPF
- Per-process filter generation based on actual configuration
- Dangerous syscalls (execve, open, socket) completely blocked from host

### Defense-in-Depth Model ("Rule of Two")

To escape gVisor, an attacker must simultaneously exploit:
1. The Sentry (Go, memory-safe, independent implementation)
2. The host kernel (via only 68 allowed syscalls)

These do not share code, so a single vulnerability is insufficient.

Additional layers:
- pivot_root away from host filesystem
- Memory-safe Go eliminates buffer overflows in the Sentry
- Gofer runs as separate process with its own restrictions

### Key Design Patterns

1. **Syscall API re-implementation**: Intercept at the syscall boundary and
   handle in user space. Dramatically reduces host kernel exposure.
2. **Rule of Two**: Require exploiting two independent codebases for escape.
3. **Separate file proxy (Gofer)**: Filesystem access mediated by a distinct
   process with minimal privileges.
4. **Binary search tree BPF**: Optimize seccomp filter performance for large
   rule sets.
5. **Platform abstraction**: Swappable syscall interception backends (Systrap,
   ptrace, KVM) behind a common interface.

### Relevance to oa-sandbox

- The **68-syscall reduction** provides a concrete target for our seccomp profiles.
  We should analyze which syscalls our sandboxed processes actually need.
- The **Gofer pattern** (separate file proxy process) is worth considering for
  filesystem mediation in high-security scenarios.
- The **Rule of Two** principle: our sandbox should layer independent mechanisms
  so no single bypass compromises everything.
- BPF binary search tree optimization is relevant if we generate complex
  seccomp filters.
- The evolution from ptrace to Systrap shows SECCOMP_RET_TRAP is the
  performant choice for syscall interception.

---

## 4. Bubblewrap (bwrap)

### Architecture Overview

Bubblewrap is a **minimal, unprivileged sandboxing tool** used by Flatpak.
Written in C, ~3000 lines. Creates isolated environments using Linux namespaces
without requiring root or setuid binaries (when unprivileged user namespaces
are available).

Core mechanism:
1. Create new mount namespace with empty tmpfs root
2. Bind-mount only explicitly specified paths from host
3. Optionally create PID, network, UTS, IPC, user namespaces
4. Apply PR_SET_NO_NEW_PRIVS (blocks setuid escalation)
5. Optionally load seccomp-BPF filter from file descriptor
6. Run trivial pid1 inside container to reap children

### Unprivileged Namespace Creation

Two modes:
- **With unprivileged user namespaces** (preferred): No setuid needed. Uses
  CLONE_NEWUSER to gain namespace creation capability.
- **Setuid fallback**: When kernel disables unprivileged user namespaces, bwrap
  can be installed setuid root and creates namespaces directly.

### Graceful Fallback Mechanism

Bubblewrap provides `--unshare-*-try` variants for optional namespaces:
- `--unshare-user-try`: Create user namespace if possible, skip if not
- `--unshare-cgroup-try`: Create cgroup namespace if possible, skip if not

This allows the same command to work across kernels with different capabilities.

### Key Design Patterns

1. **Empty root by default**: Start with nothing, explicitly add what is needed.
   Whitelist approach to filesystem access.
2. **PR_SET_NO_NEW_PRIVS**: Simple, irrevocable privilege restriction.
3. **Graceful degradation**: `--try` flags for optional isolation layers.
4. **Trivial pid1**: Reaps zombie children inside the namespace, preventing
   zombie process accumulation.
5. **FD-passing for seccomp**: Pre-compiled BPF programs loaded via FD,
   separating filter generation from filter application.
6. **Minimal trusted code**: ~3000 lines of C, focused and auditable.

### Relevance to oa-sandbox

- The **empty-root + bind-mount whitelist** approach should be our default
  filesystem isolation model on Linux.
- **PR_SET_NO_NEW_PRIVS** is a mandatory baseline for all sandboxed processes.
- The **graceful fallback pattern** is critical: we must handle environments
  where user namespaces are disabled (corporate Linux, older kernels).
- The **trivial pid1** pattern solves zombie reaping -- directly relevant to
  our Skill 3 resource lifecycle requirements (every spawn needs a reap path).
- **FD-passing for seccomp filters** keeps filter generation separate from
  the sandbox entry point.
- Claude Code's sandbox already uses bwrap on Linux, validating this as a
  production-ready foundation.

---

## 5. Claude Code Sandbox Runtime (Anthropic)

### Architecture Overview

Anthropic's sandbox-runtime is a **lightweight sandboxing tool** that enforces
filesystem and network restrictions at the OS level without containers.

Two isolation boundaries:
1. **Filesystem isolation**: Restrict which directories can be read/written
2. **Network isolation**: Restrict which domains can be connected to

Platform backends:
- **macOS**: sandbox-exec with Seatbelt profiles
- **Linux**: Bubblewrap + seccomp-BPF

### Implementation Details

#### Filesystem
- Default: read/write access to CWD only
- Configurable allowWrite and denyRead lists
- Everything outside allowed paths is blocked at the OS level

#### Network
- **Proxy-based filtering**: All network traffic routed through local proxy
- HTTP/HTTPS via HTTP proxy, other TCP via SOCKS5 proxy
- Domain allowlist/denylist enforcement at proxy level
- **macOS**: Seatbelt profile allows only localhost:specific_port, proxy on that port
- **Linux**: Traffic routed via Unix domain socket to proxy
- Empty allowedDomains = no network access (deny by default)

#### seccomp on Linux
- Pre-built static apply-seccomp binaries for x64 and arm64
- Pre-generated BPF filters included (no runtime compilation needed)
- Blocks Unix domain socket creation at syscall level (preventing bypass
  of network proxy)

#### Cowork (Cloud Sandbox)
- macOS: Linux VM via VZVirtualMachine (Apple Virtualization framework)
- Inside VM: bubblewrap + seccomp for process-level isolation
- Two layers: VM boundary + process sandbox within VM

### Key Design Patterns

1. **Native OS primitives, no containers**: Avoids Docker/OCI overhead and
   complexity. Uses what the OS already provides.
2. **Proxy-based network control**: Instead of network namespaces alone, route
   traffic through an allow-listing proxy for domain-level filtering.
3. **Pre-compiled seccomp filters**: Ship pre-built BPF binaries, eliminating
   runtime dependencies on compilation tools.
4. **Block Unix sockets via seccomp**: Prevents sandboxed process from creating
   local IPC channels that could bypass the network proxy.
5. **Platform-native violation logging**: On macOS, taps into system sandbox
   violation log store for real-time monitoring.

### Relevance to oa-sandbox

- This is the **most directly relevant reference implementation** for our design.
- The **proxy-based network filtering** approach is pragmatic and battle-tested
  in production with Claude Code.
- **Pre-compiled seccomp filters** avoid a class of deployment problems.
- The **no-container philosophy** aligns with our goal of lightweight isolation.
- The **seccomp block on Unix sockets** is a subtle but critical detail for
  preventing network proxy bypass -- we must implement this.
- The **Cowork two-layer model** (VM + process sandbox) shows how to scale
  isolation strength based on trust level.

---

## 6. Deno Runtime

### Architecture Overview

Deno implements a **permission-based security model** in Rust, built on V8
isolates and Tokio async runtime.

Default state: **deny everything**. Scripts cannot access:
- Filesystem (read or write)
- Network (connections or listeners)
- Environment variables
- Subprocess spawning
- FFI (foreign function interface)
- High-resolution time

Permissions granted via CLI flags with optional scope:
- `--allow-read=/specific/path` (path-scoped filesystem read)
- `--allow-net=api.example.com:443` (host+port-scoped network)
- `--allow-env=HOME,PATH` (variable-scoped env access)
- `--allow-run=git,npm` (binary-scoped subprocess)

### Internal Implementation

- **Ops layer**: Rust functions bridging V8 JavaScript to OS operations.
  Every op that touches a restricted resource calls into the permission system.
- **deno_permissions crate**: Central permission state management, allowlist
  parsing, user prompts, and runtime checks.
- **Tokio runtime**: Single-threaded event loop with I/O threadpool for
  async operations.
- **Permission broker**: Optional external process (via Unix socket / named pipe)
  for centralized permission decisions.

### Known Limitations

- **Subprocess escape**: Spawned processes and FFI libraries can access system
  resources regardless of Deno's permission flags, effectively bypassing the
  sandbox.
- **Coarse granularity for some operations**: Network permissions are host-level,
  not path-level (cannot restrict to specific HTTP endpoints).

### Key Design Patterns

1. **Deny-by-default with explicit grants**: The most secure default posture.
   Users must consciously opt in to each capability.
2. **Scope narrowing**: Permissions can be restricted to specific paths, hosts,
   ports, env vars, and binaries.
3. **Runtime permission prompts**: Interactive mode can prompt the user at
   first access rather than requiring all permissions upfront.
4. **Centralized permission crate**: Single module responsible for all
   permission logic, making it auditable.
5. **External permission broker**: Delegation pattern for policy-driven
   permission decisions.

### Relevance to oa-sandbox

- The **deny-by-default + explicit grant** model should be our API design
  principle. Sandbox profiles should start empty and require explicit allowances.
- **Scope narrowing** (path-level, host-level) provides the right granularity
  for our sandbox configuration.
- The **subprocess escape problem** is a critical lesson: we MUST ensure our
  sandbox restrictions survive subprocess spawning. seccomp + namespaces
  handle this where Deno's in-process checks cannot.
- The **permission broker pattern** is interesting for centralized policy
  management in multi-sandbox deployments.
- Deno validates that Rust is an excellent language for building security-
  critical runtime infrastructure.

---

## 7. Rust Sandbox Crates & Libraries

### extrasafe

- **Scope**: seccomp-BPF + Landlock + user namespaces
- **API**: Deny-by-default. Enable capabilities via `RuleSet` trait.
  Built-in rule sets: `SystemIO`, `Networking`, `Threads`, etc.
- **Landlock integration**: `SystemIO::nothing().allow_read_path("/data")`
  for fine-grained filesystem access (requires Linux 5.19+)
- **Maturity**: Active development, well-documented, good API ergonomics.
- **Limitation**: Linux only. Landlock requires kernel 5.19+.

### landlock (crate)

- **Scope**: Pure Landlock LSM bindings
- **API**: Safe Rust abstraction over Landlock system calls
- **Use case**: Filesystem access restriction as a security layer
  additional to system-wide access controls (e.g., complement seccomp)
- **Maturity**: Official Landlock project crate. Production-quality.
- **Limitation**: Linux 5.13+ required, ABI versioning adds complexity.

### gaol (Servo)

- **Scope**: Cross-platform sandbox (Linux, macOS, FreeBSD)
- **API**: Whitelist-based profiles describing allowed operations.
  Multi-process model (profile in parent, restrictions on child).
- **Maturity**: Lightly reviewed, not battle-tested. Conceptually clean
  but under-maintained.
- **Limitation**: Not production-hardened. Limited active development.

### hakoniwa

- **Scope**: Namespaces + cgroups (v2) + Landlock + seccomp
- **API**: Container builder pattern -> Command execution
- **Features**: MNT namespace + pivot_root, network namespace + pasta
  (user-mode networking), setrlimit, systemd cgroup delegation.
- **Maturity**: Newer crate, covers comprehensive Linux isolation.
- **Limitation**: Linux only.

### sandbox-rs

- **Scope**: Namespaces + seccomp-BPF + cgroups v2 + filesystem isolation
- **API**: Unified interface combining multiple isolation mechanisms
- **Maturity**: Library + CLI tool. Newer project.

### sandbox-runtime (Rust port of Anthropic's)

- **Scope**: Rust port of Claude Code's TypeScript sandbox-runtime
- **Architecture**: Same as Anthropic's original but in Rust
- **Note**: Community port, not official Anthropic project.

### Assessment for oa-sandbox

| Crate | Cross-Platform | Maturity | API Quality | Recommendation |
|-------|---------------|----------|-------------|----------------|
| extrasafe | Linux only | Good | Excellent | Use as reference for seccomp+Landlock API design |
| landlock | Linux only | Excellent | Good | Use directly for filesystem restriction on Linux |
| gaol | Yes (limited) | Low | Good concepts | Reference for cross-platform abstraction ideas |
| hakoniwa | Linux only | Moderate | Good | Reference for namespace+cgroup integration |

**Recommendation**: Rather than depending on any single crate, oa-sandbox should
implement its own abstraction layer inspired by:
- extrasafe's `RuleSet` trait pattern for composable security policies
- landlock crate directly for filesystem access control
- gaol's cross-platform profile concept for the API surface
- hakoniwa's namespace+cgroup integration patterns

---

## Cross-Cutting Themes & Best Practices

### 1. Deny-by-Default is Universal

Every surveyed project starts from zero access and requires explicit grants.
This is not optional -- it is the fundamental design principle.

### 2. Layered Defense is Non-Negotiable

| Project | Layer 1 | Layer 2 | Layer 3 |
|---------|---------|---------|---------|
| Chromium | Namespaces/tokens | seccomp-BPF | IPC broker |
| Firecracker | KVM hardware | Jailer (cgroup+chroot+ns) | seccomp |
| gVisor | User-space kernel | seccomp on Sentry | Gofer file proxy |
| Bubblewrap | Namespaces | PR_SET_NO_NEW_PRIVS | seccomp (optional) |
| Claude Code | bwrap/Seatbelt | seccomp | Network proxy |
| Deno | V8 isolate | Permission checks | (subprocess gap) |

### 3. Privilege Drop Pattern

Firecracker's jailer and Chromium's broker both follow:
```
Acquire privileged resources -> Apply restrictions -> Drop privileges irrevocably -> exec target
```
This must be our sandbox launcher's architecture.

### 4. Platform Abstraction with Native Backends

Every mature project uses platform-specific mechanisms behind a common API:
- Linux: namespaces + seccomp + Landlock/cgroups
- macOS: Seatbelt (sandbox-exec)
- Windows: Restricted tokens + Job objects + integrity levels

Attempting a single cross-platform mechanism always fails. The abstraction
must be at the policy/API level, not the mechanism level.

### 5. Subprocess Isolation Must Be OS-Level

Deno's in-process permission model fails when subprocesses are spawned.
Lesson: **sandbox restrictions must be enforced by the kernel** (seccomp,
namespaces, Seatbelt) so they survive fork/exec.

### 6. Network Isolation Requires Proxy or Namespace

Two proven approaches:
- **Network namespace** (empty, loopback only): Complete isolation
- **Proxy-based filtering**: Domain-level granularity via HTTP/SOCKS5 proxy

Claude Code combines both: network namespace + proxy for selective access.

### 7. Zombie Process Prevention

Bubblewrap's trivial pid1 and Firecracker's jailer both address this.
Any sandbox creating PID namespaces must run an init process to reap children.

### 8. Seccomp Best Practices

- Use SECCOMP_RET_TRAP (not SECCOMP_RET_KILL) for debuggability
- Binary search tree in BPF for performance (gVisor)
- Parameter filtering (Firecracker level 2) for stronger guarantees
- Pre-compile BPF filters and ship as binaries (Claude Code)
- Block Unix socket creation when using network proxy (Claude Code)

---

## Recommended Architecture for oa-sandbox

Based on this survey, the following architecture emerges:

```
                    oa-sandbox Architecture

    [CLI / API] -- Policy Configuration (deny-by-default)
         |
    [Sandbox Launcher] (privileged setup phase)
         |
         +-- Linux: Create namespaces (user/PID/mount/net/IPC)
         |          Bind-mount allowed paths (empty root + whitelist)
         |          Apply cgroup limits
         |          Apply Landlock filesystem rules
         |          Load pre-compiled seccomp-BPF filter
         |          PR_SET_NO_NEW_PRIVS
         |          Drop privileges
         |          Start trivial pid1
         |          exec() target
         |
         +-- macOS: Generate Seatbelt profile
         |          Apply via sandbox-exec
         |          Optional: network proxy for domain filtering
         |
         +-- Windows: Create restricted token
                     Create Job object (process limit = 1)
                     Set untrusted integrity level
                     Create alternate desktop
                     Create process with restricted token
                     Optional: network proxy for domain filtering

    [Network Proxy] (optional, when domain-level filtering needed)
         |
         +-- HTTP proxy for HTTP/HTTPS traffic
         +-- SOCKS5 proxy for other TCP
         +-- Domain allowlist/denylist enforcement
         +-- Block Unix socket creation via seccomp (Linux)
```

### Priority Implementation Order

- [x] Research complete
- [ ] Linux namespace + seccomp baseline (highest priority, most mature patterns)
- [ ] macOS Seatbelt profile generation
- [ ] Landlock filesystem restriction (Linux 5.13+)
- [ ] Network proxy with domain filtering
- [ ] Windows restricted token + Job object
- [ ] Pre-compiled seccomp filter distribution
- [ ] Graceful degradation for restricted environments
- [ ] Cgroup resource limits

---

## Sources

- [Chromium Sandbox Design](https://chromium.googlesource.com/chromium/src/+/HEAD/docs/design/sandbox.md)
- [Chromium Linux Sandboxing](https://chromium.googlesource.com/chromium/src/+/0e94f26e8/docs/linux_sandboxing.md)
- [Chromium Mac Seatbelt Design](https://github.com/chromium/chromium/blob/main/sandbox/mac/seatbelt_sandbox_design.md)
- [Firecracker GitHub](https://github.com/firecracker-microvm/firecracker)
- [Firecracker Jailer Docs](https://github.com/firecracker-microvm/firecracker/blob/main/docs/jailer.md)
- [Firecracker Official Site](https://firecracker-microvm.github.io/)
- [gVisor Architecture Guide](https://gvisor.dev/docs/architecture_guide/security/)
- [gVisor Systrap Release](https://gvisor.dev/blog/2023/04/28/systrap-release/)
- [gVisor Seccomp Optimization](https://gvisor.dev/blog/2024/02/01/seccomp/)
- [gVisor Dangerzone Case Study](https://gvisor.dev/blog/2024/09/23/safe-ride-into-the-dangerzone/)
- [Bubblewrap GitHub](https://github.com/containers/bubblewrap)
- [Bubblewrap LWN Article](https://lwn.net/Articles/686113/)
- [Claude Code Sandboxing Engineering Blog](https://www.anthropic.com/engineering/claude-code-sandboxing)
- [Anthropic sandbox-runtime GitHub](https://github.com/anthropic-experimental/sandbox-runtime)
- [Claude Code Sandboxing Docs](https://code.claude.com/docs/en/sandboxing)
- [Deno Security & Permissions](https://docs.deno.com/runtime/fundamentals/security/)
- [Deno Internals - OPs](https://choubey.gitbook.io/internals-of-deno/architecture/ops)
- [Cage4Deno Paper](https://dl.acm.org/doi/fullHtml/10.1145/3579856.3595799)
- [extrasafe Crate](https://lib.rs/crates/extrasafe)
- [landlock Crate](https://crates.io/crates/landlock)
- [gaol (Servo)](https://github.com/servo/gaol)
- [hakoniwa Crate](https://github.com/souk4711/hakoniwa)
- [Palo Alto Sandboxed Container Overview](https://unit42.paloaltonetworks.com/making-containers-more-isolated-an-overview-of-sandboxed-container-technologies/)
