//! `sandbox worker-start` command: start a persistent sandbox Worker process.
//!
//! The Worker reads command requests from stdin (JSON-Lines), executes them
//! inside the sandbox, and writes responses to stdout. The sandbox is applied
//! by the launcher before exec, so this code runs already-sandboxed.
//!
//! # Usage (internal — called by the launcher, not directly by users)
//!
//! ```text
//! oa sandbox worker-start --workspace /path [--timeout 120] [--security-level sandbox] [--idle-timeout 300]
//! ```

use std::path::PathBuf;

use anyhow::{Context, Result};

use oa_sandbox::worker::{WorkerConfig, run_event_loop};

/// Options for the `sandbox worker-start` subcommand.
#[derive(Debug, Clone)]
pub struct WorkerStartOptions {
    /// Workspace directory for sandboxed commands.
    pub workspace: PathBuf,
    /// Default timeout in seconds for commands without per-request timeout.
    pub timeout: u64,
    /// Security level: "deny" (L0), "sandbox" (L1), "full" (L2).
    /// Passed to the launcher for Seatbelt/Landlock profile selection.
    pub security_level: String,
    /// Idle timeout in seconds. Worker exits if no request arrives within this duration.
    /// 0 = no idle timeout (default).
    pub idle_timeout: u64,
}

/// Run the Worker event loop.
///
/// This is called after the process has been sandboxed by the launcher.
/// It blocks until stdin is closed, a shutdown command is received,
/// or the idle timeout is reached.
pub fn sandbox_worker_start_command(opts: &WorkerStartOptions) -> Result<()> {
    let config = WorkerConfig {
        workspace: opts.workspace.clone(),
        default_timeout_secs: opts.timeout,
        idle_timeout_secs: opts.idle_timeout,
    };

    run_event_loop(&config).context("worker event loop failed")
}
