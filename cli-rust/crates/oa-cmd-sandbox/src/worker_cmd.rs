//! `sandbox worker-start` command: start a persistent sandbox Worker process.
//!
//! The Worker reads command requests from stdin (JSON-Lines), executes them
//! inside the sandbox, and writes responses to stdout.
//!
//! **Sandbox application** follows a two-path design:
//! - **Launcher path**: The launcher applies sandbox via `pre_exec` before exec.
//!   Sets `OA_SANDBOX_APPLIED=1` so the Worker skips self-sandboxing.
//! - **Go bridge path**: Go directly spawns `worker-start` without a launcher.
//!   The Worker self-sandboxes before entering the event loop.
//!
//! # Usage (internal — called by the launcher or Go bridge, not directly by users)
//!
//! ```text
//! oa sandbox worker-start --workspace /path [--timeout 120] [--security-level sandbox] [--idle-timeout 300]
//! ```

use std::collections::HashMap;
use std::path::PathBuf;

use anyhow::{Context, Result};
use tracing::info;

use oa_sandbox::config::{BackendPreference, OutputFormat, ResourceLimits, SandboxConfig, SecurityLevel};
use oa_sandbox::worker::{WorkerConfig, run_event_loop};

/// Options for the `sandbox worker-start` subcommand.
#[derive(Debug, Clone)]
pub struct WorkerStartOptions {
    /// Workspace directory for sandboxed commands.
    pub workspace: PathBuf,
    /// Default timeout in seconds for commands without per-request timeout.
    pub timeout: u64,
    /// Security level: "deny" (L0), "allowlist" (L1), "sandboxed" (L2). Legacy aliases: "sandbox", "full".
    pub security_level: String,
    /// Idle timeout in seconds. Worker exits if no request arrives within this duration.
    /// 0 = no idle timeout (default).
    pub idle_timeout: u64,
}

/// Parse a security level string to [`SecurityLevel`].
fn parse_security_level(s: &str) -> Result<SecurityLevel> {
    match s {
        "deny" => Ok(SecurityLevel::L0Deny),
        "allowlist" | "sandbox" => Ok(SecurityLevel::L1Allowlist),
        "sandboxed" | "full" => Ok(SecurityLevel::L2Sandboxed),
        other => anyhow::bail!("unknown security level: {other:?} (expected deny/allowlist/sandboxed)"),
    }
}

/// Run the Worker event loop with self-sandboxing.
///
/// If `OA_SANDBOX_APPLIED=1` is set (launcher path), the sandbox is already
/// active and self-sandboxing is skipped. Otherwise (Go bridge path), the
/// Worker applies the sandbox to itself before entering the event loop.
///
/// Self-sandboxing is irreversible and inherited by all child processes.
pub fn sandbox_worker_start_command(opts: &WorkerStartOptions) -> Result<()> {
    // 1. Parse security level
    let security_level = parse_security_level(&opts.security_level)
        .context("parsing worker security level")?;

    // 2. Self-sandbox (unless launcher already applied it)
    let already_sandboxed = std::env::var("OA_SANDBOX_APPLIED").as_deref() == Ok("1");

    if already_sandboxed {
        info!(
            security = %opts.security_level,
            "sandbox already applied by launcher, skipping self-sandbox"
        );
    } else {
        // Build SandboxConfig for self-sandboxing.
        // command/args are irrelevant for self-sandbox (we sandbox the current process).
        let sandbox_config = SandboxConfig {
            security_level,
            command: "self-sandbox-placeholder".into(), // not used by apply_sandbox_to_self
            args: vec![],
            workspace: opts.workspace.clone(),
            mounts: vec![],
            resource_limits: ResourceLimits::default(),
            network_policy: None,
            env_vars: HashMap::new(),
            format: OutputFormat::Json,
            backend: BackendPreference::Native,
        };

        oa_sandbox::apply_sandbox_to_self(&sandbox_config)
            .context("worker self-sandbox failed")?;

        info!(
            security = %opts.security_level,
            workspace = %opts.workspace.display(),
            "worker self-sandbox applied"
        );
    }

    // 3. Run event loop (already sandboxed at this point)
    let config = WorkerConfig {
        workspace: opts.workspace.clone(),
        default_timeout_secs: opts.timeout,
        idle_timeout_secs: opts.idle_timeout,
    };

    run_event_loop(&config).context("worker event loop failed")
}
