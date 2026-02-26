---
document_type: Tracking
status: In Progress
created: 2026-02-25
last_updated: 2026-02-25
audit_report: Pending
skill5_verified: true
---

# macOS Seatbelt/Sandbox API Verification

## Online Verification Log

### 1. macOS sandbox-exec Deprecation Status (macOS 14 Sonoma / macOS 15 Sequoia)

- **Query**: "macOS sandbox-exec deprecation status macOS 14 Sonoma macOS 15 Sequoia"
- **Sources**:
  - https://news.ycombinator.com/item?id=44283454
  - https://bdash.net.nz/posts/sandboxing-on-macos/
  - https://developer.apple.com/forums/thread/661939
  - https://igorstechnoclub.com/sandbox-exec/
- **Key findings**:
  - sandbox-exec is marked "DEPRECATED" in its man page, but this status has existed for many years
  - The underlying sandbox subsystem is NOT being removed; Apple's own first-party apps and daemons rely on it
  - Chromium, Firefox, and Nix package manager all actively use sandbox_init_with_parameters
  - Apple discourages direct use because "you have to be an expert in macOS internals to use it correctly...and those internals change with every release"
  - The deprecation is of the PUBLIC CLI tool; the kernel-level sandbox enforcement mechanism itself remains active
  - sandbox-exec continues to function on macOS 14 and macOS 15 based on community reports
  - Apple's recommended alternative is App Sandbox (entitlement-based), which is unsuitable for CLI tools and non-App-Store software
- **Verified date**: 2026-02-25

### 2. Seatbelt Profile Syntax (.sb / SBPL)

- **Query**: "macOS Seatbelt sandbox profile .sb syntax", "SBPL network filter syntax"
- **Sources**:
  - https://book.hacktricks.wiki/en/macos-hardening/macos-security-and-privilege-escalation/macos-security-protections/macos-sandbox/index.html
  - https://bdash.net.nz/posts/sandboxing-on-macos/
  - https://mybyways.com/blog/run-code-in-a-macos-sandbox
  - https://zameermanji.com/blog/2025/4/1/sandboxing-subprocesses-in-python-on-macos/
  - https://www.karltarvas.com/macos-app-sandboxing-via-sandbox-exec/
  - https://reverse.put.as/wp-content/uploads/2011/09/Apple-Sandbox-Guide-v1.0.pdf
- **Key findings**:

  **Language**: SBPL is a Scheme (Lisp dialect) DSL. Comments with `;`. Supports conditionals, lambdas, macros.

  **Basic structure**:
  ```scheme
  (version 1)
  (deny default)           ; deny-by-default is the secure starting point
  (import "bsd.sb")        ; import baseline system profile
  ```

  **Import directive**: `(import "bsd.sb")` and `(import "/System/Library/Sandbox/Profiles/bsd.sb")` are both valid. System profiles are located at:
  - `/System/Library/Sandbox/Profiles/`
  - `/usr/share/sandbox/`

  **File access rules**:
  ```scheme
  (allow file-read* (subpath "/usr/lib"))          ; recursive read under path
  (allow file-read-data (literal "/etc/hosts"))    ; exact path match
  (allow file-read-metadata (regex #"^/private/etc/.*"))  ; regex match
  (allow file-write* (subpath "/tmp/sandbox-work"))
  ```

  **Filter types**:
  - `(literal "/exact/path")` - exact match
  - `(subpath "/prefix/path")` - path and all descendants
  - `(regex #"^/pattern/.*")` - regex matching

  **Network filtering**:
  ```scheme
  (allow network-outbound (remote tcp "localhost:*"))   ; localhost only
  (allow network-outbound (remote tcp "*:443"))         ; any host, port 443
  (allow network-outbound (remote ip "192.168.1.0/24:*")) ; subnet
  (allow network* (local ip "localhost:*"))             ; local binding
  (deny network*)                                       ; block all networking
  ```
  IMPORTANT: Network filtering operates at socket level. It cannot filter by domain name, HTTP method, or protocol content. It is all-or-nothing at the IP/port level.

  **Process execution rules**:
  ```scheme
  (allow process-exec (literal "/usr/bin/python3"))
  (allow process-fork)
  ```

  **Parameterization**:
  ```scheme
  (define targetDir (param "DIR"))
  (allow file-write* (subpath targetDir))
  ```
  Parameters are passed via sandbox_init_with_parameters as key-value arrays.

  **Key operations**: file-read*, file-write*, file-read-data, file-read-metadata, process-exec, process-fork, network-outbound, network-inbound, network*, mach-lookup, mach-*, sysctl, syscall-unix, user-preference-read

- **Verified date**: 2026-02-25

### 3. macOS Virtualization.framework

- **Query**: "macOS Virtualization.framework lightweight sandboxing"
- **Sources**:
  - https://developer.apple.com/documentation/virtualization
  - https://github.com/suzusuzu/virtualization-rs
  - https://github.com/lynaghk/vibe
  - https://developer.apple.com/videos/play/wwdc2022/10002/
- **Key findings**:
  - Provides high-level APIs for creating and managing VMs on Apple Silicon and Intel Macs
  - Minimum requirement: macOS Big Sur (11.0)
  - Supports Linux guests (since Big Sur) and macOS guests (since Monterey, Apple Silicon only)
  - Uses VIRTIO spec for device interfaces: network, storage, serial port, entropy, memory-balloon, filesystem (VirtioFS)
  - Shared directories via VirtioFS allow host-guest filesystem sharing
  - Networking: NAT by default; bridged networking requires special entitlement
  - Rust bindings: `virtualization-rs` crate (v0.1.2, early stage, 35 commits, macOS Big Sur+)
  - Vibe project demonstrates single-binary Rust tool using Virtualization.framework for agent sandboxing
  - NOT suitable for lightweight process sandboxing: it creates full VMs with their own kernel. Startup time is seconds, not milliseconds. Memory overhead is significant (minimum ~128MB per VM)
  - Better suited as a "heavy isolation" tier for truly untrusted code, not for routine command sandboxing
- **Verified date**: 2026-02-25

### 4. Existing Rust Crates for macOS Sandboxing

- **Query**: "crates.io Rust macOS sandbox seatbelt"
- **Sources**:
  - https://github.com/servo/gaol
  - https://crates.io/crates/rusty-sandbox
  - https://crates.io/crates/sandbox-runtime
  - https://crates.io/crates/wardstone
- **Key findings**:

  | Crate | Approach | macOS Support | Status | Notes |
  |---|---|---|---|---|
  | **gaol** (Servo) | Cross-platform sandbox lib | Seatbelt via sandbox_init | 375 stars, lightly reviewed | Whitelist-based profile model. "Not battle-tested" per README |
  | **rusty-sandbox** | Multi-platform sandbox | macOS Seatbelt/sandboxd | Published on crates.io | Supports sandboxing current or forked processes |
  | **sandbox-runtime** | Seatbelt profile generator | Generates .sb files, runs via sandbox-exec | Rust port of Anthropic's TS implementation | Auto-generates SBPL from config, uses glob patterns |
  | **wardstone** | AI agent sandbox | macOS Seatbelt | Published on crates.io | Auto-generates .sbpl for AI agent tool execution |
  | **virtualization-rs** | VM-based isolation | Virtualization.framework | v0.1.2, early stage | Full VM, not process sandboxing |

  **Recommendation for oa-sandbox**: None of these are mature enough to depend on directly. gaol from Servo is the most established but explicitly warns about lack of security review. sandbox-runtime's approach (generate .sb profile, invoke sandbox-exec) is the most practical pattern to replicate. We should implement our own Seatbelt integration calling sandbox_init_with_parameters via FFI, following the Chromium model.

- **Verified date**: 2026-02-25

### 5. Chromium Sandbox on macOS (Reference Implementation)

- **Query**: "chromium sandbox mac seatbelt implementation design"
- **Sources**:
  - https://chromium.googlesource.com/chromium/src/+/HEAD/sandbox/mac/seatbelt_sandbox_design.md
  - https://chromium.googlesource.com/chromium/src/+/HEAD/sandbox/mac/README.md
  - https://chromium.googlesource.com/chromium/src/+/master/sandbox/mac/
- **Key findings**:

  **Architecture**:
  - Uses `sandbox(7)` / Seatbelt API (NOT App Sandbox)
  - Minimal main executable; bulk of code in Chromium Framework (dlopen'd at runtime)
  - Sandbox applied BEFORE framework loading, so static initializers run sandboxed
  - V2 design eliminates unsandboxed "warmup phase" present in V1

  **Profile compilation**:
  - .sb policy files compiled via `Seatbelt::Compile` to binary representation
  - Binary transmitted over Mojo IPC to child processes
  - Applied via `SeatbeltExecServer::ApplySandboxProfile`

  **APIs used**:
  - `sandbox_init_with_parameters(profile, flags, parameters[], errorbuf)` for self-sandboxing
  - `sandbox_compile_string()` / `sandbox_apply()` for split compilation (allows caching)
  - `sandbox_extension_issue_file()` / `sandbox_extension_consume()` for dynamic permission grants

  **Per-process-type profiles**:
  - Each process type (renderer, GPU, utility, etc.) has its own .sb policy
  - Common policy with shared primitives/variables
  - All profiles start with `(deny default)`

  **Parameter passing**:
  - Runtime parameters: home directory, OS version, app bundle path, PID, permitted directories
  - Format: `"KEY", "value", "KEY2", "value2", NULL` array

  **Risk philosophy**:
  - Compatibility risk vs. security risk tradeoff
  - More permissive = fewer breakages on macOS updates but weaker security
  - V2 favors tighter security, accepting compatibility risk

  **Key lesson for oa-sandbox**: Chromium's approach (compile profile to bytecode, apply before main code loads, per-process-type policies) is the gold standard. The `sandbox_init_with_parameters` + parameterized profiles pattern is exactly what we should follow.

- **Verified date**: 2026-02-25

### 6. Gotcha: Interpreter Crashes Without system.sb Import

- **Query**: "macOS sandbox-exec system.sb import Python Node crash Killed 9"
- **Sources**:
  - https://zameermanji.com/blog/2025/4/1/sandboxing-subprocesses-in-python-on-macos/
  - https://mybyways.com/blog/run-code-in-a-macos-sandbox
  - https://igorstechnoclub.com/sandbox-exec/
  - https://github.com/pyenv/pyenv/issues/2289
- **Key findings**:

  **The claim is ACCURATE but needs nuance**:
  - A bare `(version 1) (deny default)` profile without ANY imports will cause most programs to fail, not just interpreters
  - The failure mode is SIGKILL (signal 9, hence "Killed: 9") when the sandboxed process tries to access denied resources at a critical point (e.g., loading shared libraries)
  - Python is "particularly obnoxious to sandbox because it scatters files over so much of the filesystem" -- it needs read access to `/usr/lib/`, Python framework directories, and various system paths
  - The fix is to import `bsd.sb` (NOT `system.sb`): `(import "bsd.sb")` provides "the basic minimum of rules that will allow for a process to start" including locale, system libraries, /usr/lib, /dev/urandom, and Apple services
  - `system.sb` is a MORE permissive profile; `bsd.sb` is the minimal baseline
  - Without bsd.sb, even basic operations like reading shared libraries fail, causing the dynamic linker to abort the process

  **For oa-sandbox implementation**:
  - ALWAYS import `bsd.sb` as baseline for any sandboxed process
  - For interpreters (Python, Node, Ruby), additionally allow read access to their framework/library directories
  - Test profile changes with `(trace "/dev/stderr")` to see what gets denied (note: trace may not work on all macOS versions)
  - Use Console.app to view sandbox violation logs during development

- **Verified date**: 2026-02-25

---

## Summary: Implementation Recommendations for oa-sandbox

### Approach: FFI to sandbox_init_with_parameters

1. **Do NOT use sandbox-exec CLI** -- invoke `sandbox_init_with_parameters` directly via Rust FFI (following Chromium's model)
2. **Generate SBPL profiles programmatically** in Rust, then pass as string to the API
3. **Always start with**: `(version 1) (deny default) (import "bsd.sb")`
4. **Parameterize** workspace paths, allowed binaries, and network rules
5. **Apply sandbox in child process** before exec (in pre_exec hook or equivalent)

### Profile Template Structure

```scheme
(version 1)
(deny default)
(import "bsd.sb")

; Workspace access
(define workspace (param "WORKSPACE_DIR"))
(allow file-read* file-write* (subpath workspace))

; Tool execution
(allow process-exec (literal (param "ALLOWED_BINARY")))
(allow process-fork)

; Network (configurable)
; Option A: No network (most secure)
; Option B: (allow network-outbound (remote tcp "localhost:*"))
; Option C: (allow network-outbound (remote tcp "*:443"))

; Read access to system paths for interpreters
(allow file-read* (subpath "/usr/lib"))
(allow file-read* (subpath "/usr/share"))
(allow file-read* (subpath "/System/Library/Frameworks"))
```

### Critical Gotchas

1. **bsd.sb is essential** -- without it, virtually all programs SIGKILL on launch
2. **Network filtering is IP:port level only** -- no domain-name or protocol filtering
3. **Sandbox is irreversible** once applied to a process
4. **Child processes inherit** the parent's sandbox
5. **sandbox_init_with_parameters is a private API** -- not in public headers, must declare extern
6. **Profile paths may change** between macOS versions; prefer relative imports like `"bsd.sb"` over absolute paths
7. **The trace command** `(trace "/dev/stderr")` may be dysfunctional on recent macOS versions
8. **Sandbox violations** return EPERM; calling code must handle gracefully
9. **Virtualization.framework** is NOT a replacement for Seatbelt -- different weight class (full VM vs. process sandbox)
