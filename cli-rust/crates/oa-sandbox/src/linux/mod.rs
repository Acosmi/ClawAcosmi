//! Linux sandbox backend.
//!
//! Provides process isolation using a layered approach:
//!
//! | Layer | Required? | What it does |
//! |-------|-----------|-------------|
//! | **Landlock LSM** | Yes | Filesystem access control (unprivileged) |
//! | **Seccomp-BPF** | Yes | Syscall filtering + network policy |
//! | **Namespaces** | Optional | User/Mount NS isolation (needs kernel support) |
//! | **Cgroups v2** | Optional | Memory/CPU/PID resource limits (needs systemd) |
//!
//! # Execution flow
//!
//! ```text
//! LinuxRunner::run(config)
//!        │
//!        ├── [Optional] Setup cgroup limits
//!        │
//!        ▼
//! Command::new(command)
//!   .pre_exec(|| {
//!       [Optional] apply_user_namespace()
//!       [Optional] apply_mount_namespace()
//!       [Optional] cgroup.add_self()
//!       apply_landlock_rules()    // filesystem isolation
//!       apply_seccomp_filter()    // syscall filtering (MUST BE LAST)
//!   })
//!   .spawn()
//!        │
//!        ▼
//! Timeout thread + wait_with_output() → SandboxOutput
//! ```
//!
//! # Degradation chain
//!
//! ```text
//! Namespace+Seccomp+Landlock  →  Landlock+Seccomp only  →  Docker fallback
//! (requires user ns)             (unprivileged)
//! ```

pub mod cgroup;
pub mod landlock;
pub mod namespace;
pub mod seccomp;

use std::os::unix::process::CommandExt;
use std::process::{Command, Stdio};
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};
use std::time::{Duration, Instant};

use tracing::{debug, info, warn};

use crate::SandboxRunner;
use crate::config::SandboxConfig;
use crate::error::SandboxError;
use crate::output::SandboxOutput;
use crate::platform::{LinuxCapabilities, SandboxBackend};

/// Linux sandbox runner.
///
/// Selected backend and detected capabilities are determined at construction time
/// by [`crate::platform::select_runner`].
pub struct LinuxRunner {
    backend: SandboxBackend,
    capabilities: LinuxCapabilities,
}

impl LinuxRunner {
    /// Create a new Linux runner with the given backend selection and capabilities.
    #[must_use]
    pub fn new(backend: SandboxBackend, capabilities: LinuxCapabilities) -> Self {
        Self {
            backend,
            capabilities,
        }
    }
}

impl SandboxRunner for LinuxRunner {
    fn name(&self) -> &'static str {
        self.backend.name()
    }

    fn available(&self) -> bool {
        self.capabilities.has_seccomp && self.capabilities.landlock_abi_version > 0
    }

    #[allow(clippy::too_many_lines)]
    fn run(&self, config: &SandboxConfig) -> Result<SandboxOutput, SandboxError> {
        let start = Instant::now();
        let use_namespaces =
            self.backend == SandboxBackend::LinuxFull && self.capabilities.has_user_namespace;

        info!(
            backend = self.name(),
            use_namespaces,
            command = %config.command,
            "starting sandboxed execution"
        );

        // ── 1. Setup cgroup resource limits (before spawning) ─────────────
        let _cgroup_guard = cgroup::setup_cgroup_limits(config)?;

        // ── 2. Build command with sandbox pre_exec ────────────────────────
        let mut cmd = Command::new(&config.command);
        cmd.args(&config.args)
            .current_dir(&config.workspace)
            .stdout(Stdio::piped())
            .stderr(Stdio::piped());

        // Set environment variables
        for (key, value) in &config.env_vars {
            cmd.env(key, value);
        }

        // Clone config values needed in the pre_exec closure.
        // These are moved into the closure — no heap allocation post-fork.
        let config_clone = config.clone();
        let has_user_ns = use_namespaces;
        let cgroup_guard_ref = _cgroup_guard.as_ref();

        // Apply sandbox layers in pre_exec (runs in child after fork, before exec).
        //
        // SAFETY:
        // - All data needed by the closure is pre-built (no allocations post-fork).
        // - Landlock and Seccomp are self-restricting and safe to call post-fork.
        // - Namespace operations (unshare) are safe to call post-fork.
        // - Seccomp filter MUST be loaded last (it restricts which syscalls are available).
        //
        // F-22 audit fix: Use static error messages instead of format!() to minimize
        // heap allocations in the post-fork pre-exec context. While Rust's
        // std::io::Error::other() still allocates internally, avoiding format!()
        // removes runtime string formatting. A fully async-signal-safe approach
        // would require write(2) to stderr + _exit(2), but that loses error propagation.
        //
        // Order matters:
        // 1. User Namespace (optional) — must be first (provides caps for mount NS)
        // 2. Mount Namespace (optional) — uses caps from user NS
        // 3. Cgroup — move self into cgroup
        // 4. Landlock — filesystem access control
        // 5. Seccomp — syscall filtering (MUST BE LAST — restricts everything)
        unsafe {
            cmd.pre_exec(move || {
                // 1. User Namespace (optional enhancement)
                if has_user_ns {
                    namespace::apply_user_namespace()
                        .map_err(|_| std::io::Error::other("user namespace setup failed"))?;

                    // 2. Mount Namespace (requires user NS capabilities)
                    namespace::apply_mount_namespace(&config_clone)
                        .map_err(|_| std::io::Error::other("mount namespace setup failed"))?;
                }

                // 3. Move into cgroup (if set up)
                if let Some(guard) = cgroup_guard_ref {
                    guard
                        .add_self()
                        .map_err(|_| std::io::Error::other("cgroup add_self failed"))?;
                }

                // 4. Landlock — filesystem isolation
                landlock::apply_landlock_rules(&config_clone)
                    .map_err(|_| std::io::Error::other("landlock setup failed"))?;

                // 5. Seccomp — syscall filtering (MUST BE LAST)
                seccomp::apply_seccomp_filter(&config_clone)
                    .map_err(|_| std::io::Error::other("seccomp setup failed"))?;

                Ok(())
            });
        }

        // ── 3. Spawn child process ────────────────────────────────────────
        let child = cmd.spawn().map_err(|e| {
            if e.kind() == std::io::ErrorKind::NotFound {
                SandboxError::CommandNotFound {
                    command: config.command.clone(),
                }
            } else {
                SandboxError::Io {
                    context: format!("spawning sandboxed command '{}'", config.command),
                    source: e,
                }
            }
        })?;

        let child_pid = child.id();
        debug!(pid = child_pid, "sandboxed process spawned");

        // ── 4. Timeout killer thread ──────────────────────────────────────
        let done = Arc::new(AtomicBool::new(false));
        let timed_out = Arc::new(AtomicBool::new(false));

        let timeout_handle = config.resource_limits.timeout_secs.map(|secs| {
            let done = done.clone();
            let timed_out = timed_out.clone();
            std::thread::spawn(move || {
                // Poll in 50ms increments so the thread exits promptly when done
                let deadline = Instant::now() + Duration::from_secs(secs);
                while Instant::now() < deadline {
                    if done.load(Ordering::Relaxed) {
                        return;
                    }
                    std::thread::sleep(Duration::from_millis(50));
                }
                if !done.load(Ordering::SeqCst) {
                    warn!(
                        pid = child_pid,
                        timeout_secs = secs,
                        "killing timed out process"
                    );
                    timed_out.store(true, Ordering::SeqCst);
                    // SAFETY: child_pid is a valid PID of our child process.
                    // Sending SIGKILL is always safe for our own children.
                    // If already exited, kill() returns ESRCH (harmless).
                    if let Ok(pid) = libc::pid_t::try_from(child_pid) {
                        unsafe {
                            libc::kill(pid, libc::SIGKILL);
                        }
                    }
                }
            })
        });

        // ── 5. Wait for child + collect output ────────────────────────────
        let output = child.wait_with_output().map_err(|e| SandboxError::Io {
            context: format!("waiting for sandboxed process (pid {child_pid})"),
            source: e,
        })?;

        // Signal completion to timeout thread
        done.store(true, Ordering::SeqCst);
        if let Some(handle) = timeout_handle {
            let _ = handle.join();
        }

        let duration_ms = u64::try_from(start.elapsed().as_millis()).unwrap_or(u64::MAX);

        // ── 6. Check for timeout ──────────────────────────────────────────
        if timed_out.load(Ordering::SeqCst) {
            let timeout_secs = config.resource_limits.timeout_secs.unwrap_or(0);
            info!(
                pid = child_pid,
                timeout_secs, duration_ms, "process timed out"
            );
            return Err(SandboxError::Timeout { timeout_secs });
        }

        // ── 7. Build output ───────────────────────────────────────────────
        let exit_code = output.status.code().unwrap_or(-1);
        let stdout = String::from_utf8_lossy(&output.stdout).into_owned();
        let stderr = String::from_utf8_lossy(&output.stderr).into_owned();

        info!(
            pid = child_pid,
            exit_code,
            duration_ms,
            stdout_len = stdout.len(),
            stderr_len = stderr.len(),
            "sandboxed process completed"
        );

        Ok(SandboxOutput {
            stdout,
            stderr,
            exit_code,
            error: None,
            duration_ms,
            sandbox_backend: self.name().into(),
        })
    }
}
