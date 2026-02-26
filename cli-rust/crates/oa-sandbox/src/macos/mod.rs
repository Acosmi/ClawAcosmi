//! macOS sandbox backend.
//!
//! Uses Apple's Seatbelt (Sandbox) framework via `sandbox_init_with_parameters` FFI
//! to apply SBPL (Sandbox Profile Language) policies to child processes.
//!
//! # Execution flow
//!
//! ```text
//! [Generate SBPL profile]
//!        │
//!        ▼
//! [Build Command + pre_exec closure]
//!        │
//!        ▼
//! [spawn()] ─── fork() ──┬── Child: apply_sandbox() → exec(command)
//!                         │
//!                         └── Parent: timeout thread + wait_with_output()
//!                                     │
//!                                     ▼
//!                              [SandboxOutput]
//! ```
//!
//! # Key decisions
//!
//! - Uses `sandbox_init_with_parameters` FFI (like Chromium), not `sandbox-exec` CLI
//! - Base profile imports `bsd.sb` (not `system.sb` — too permissive)
//! - Workspace path injected via SBPL parameters for safe path interpolation
//! - Timeout via background thread + `SIGKILL` (simple, reliable)

pub mod ffi;
pub mod seatbelt;

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
use crate::platform::MacosCapabilities;

/// macOS Seatbelt sandbox runner.
pub struct MacosRunner {
    capabilities: MacosCapabilities,
}

impl MacosRunner {
    /// Create a new macOS runner with the detected capabilities.
    #[must_use]
    pub const fn new(capabilities: MacosCapabilities) -> Self {
        Self { capabilities }
    }
}

impl SandboxRunner for MacosRunner {
    fn name(&self) -> &'static str {
        "macos-seatbelt"
    }

    fn available(&self) -> bool {
        self.capabilities.has_seatbelt
    }

    #[allow(clippy::too_many_lines)]
    fn run(&self, config: &SandboxConfig) -> Result<SandboxOutput, SandboxError> {
        let start = Instant::now();

        // 1. Generate SBPL profile
        let profile = seatbelt::generate_profile(config)?;
        debug!(
            profile_len = profile.sbpl.len(),
            param_count = profile.params.len(),
            "generated SBPL profile"
        );

        // 2. Pre-build FFI arguments (before fork)
        let params_refs: Vec<(&str, &str)> = profile
            .params
            .iter()
            .map(|(k, v)| (k.as_str(), v.as_str()))
            .collect();
        let sandbox_args = ffi::SandboxArgs::new(&profile.sbpl, &params_refs)?;

        // 3. Build Command with sandbox pre_exec
        let mut cmd = Command::new(&config.command);
        cmd.args(&config.args)
            .current_dir(&config.workspace)
            .stdout(Stdio::piped())
            .stderr(Stdio::piped());

        // Set environment variables
        for (key, value) in &config.env_vars {
            cmd.env(key, value);
        }
        // Ensure TMPDIR is passed for temp directory access in profile
        if !config.env_vars.contains_key("TMPDIR") {
            if let Ok(tmpdir) = std::env::var("TMPDIR") {
                cmd.env("TMPDIR", &tmpdir);
            }
        }

        // Apply sandbox in pre_exec (runs in child after fork, before exec).
        // SAFETY:
        // - `sandbox_args` is pre-built with no allocations needed in the closure.
        // - `sandbox_init_with_parameters` is a system call wrapper, safe to call post-fork.
        // - The closure moves `sandbox_args` into the child context.
        // - After `apply()`, the sandbox is irreversible and inherited by exec'd process.
        unsafe {
            cmd.pre_exec(move || sandbox_args.apply());
        }

        // 4. Spawn child process
        info!(command = %config.command, "spawning sandboxed process");
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

        // 5. Set up timeout killer thread
        let done = Arc::new(AtomicBool::new(false));
        let timed_out = Arc::new(AtomicBool::new(false));

        let timeout_handle = config.resource_limits.timeout_secs.map(|secs| {
            let done = done.clone();
            let timed_out = timed_out.clone();
            std::thread::spawn(move || {
                // Poll in small increments so the thread exits promptly when done.
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
                    // If the process already exited, kill() returns ESRCH (harmless).
                    if let Ok(pid) = libc::pid_t::try_from(child_pid) {
                        unsafe {
                            libc::kill(pid, libc::SIGKILL);
                        }
                    }
                }
            })
        });

        // 6. Wait for child + collect stdout/stderr
        // wait_with_output() reads pipes to completion, preventing deadlock from
        // full pipe buffers, then waits for the child to exit.
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

        // 7. Check for timeout
        if timed_out.load(Ordering::SeqCst) {
            let timeout_secs = config.resource_limits.timeout_secs.unwrap_or(0);
            info!(
                pid = child_pid,
                timeout_secs, duration_ms, "process timed out"
            );
            return Err(SandboxError::Timeout { timeout_secs });
        }

        // 8. Build output
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
