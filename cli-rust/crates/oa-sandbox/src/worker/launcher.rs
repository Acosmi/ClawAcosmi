//! Worker process launcher.
//!
//! Spawns a persistent sandbox Worker process and returns a [`WorkerHandle`]
//! for sending commands over stdin/stdout IPC.
//!
//! The launcher applies platform-specific sandbox constraints (Seatbelt/Landlock/
//! Seccomp/Job Object) before exec, so the Worker runs already-sandboxed.
//! All subsequent fork+exec'd commands inherit the sandbox.

use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};

use tracing::{debug, info};

use crate::config::{NetworkPolicy, SandboxConfig, SecurityLevel};
use crate::error::SandboxError;

use super::handle::WorkerHandle;

/// Configuration for launching a Worker process.
#[derive(Debug, Clone)]
pub struct WorkerLaunchConfig {
    /// Security level for the sandbox.
    pub security_level: SecurityLevel,

    /// Workspace directory (mounted read-write in the sandbox).
    pub workspace: PathBuf,

    /// Network policy override (defaults to security level's default).
    pub network_policy: Option<NetworkPolicy>,

    /// Additional bind mounts.
    pub mounts: Vec<crate::config::MountSpec>,

    /// Default timeout in seconds for commands executed by the Worker.
    pub default_timeout_secs: u64,

    /// Idle timeout in seconds. Worker exits if no request arrives within this duration.
    /// 0 = no idle timeout.
    pub idle_timeout_secs: u64,
}

impl Default for WorkerLaunchConfig {
    fn default() -> Self {
        Self {
            security_level: SecurityLevel::L1Sandbox,
            workspace: std::env::temp_dir(),
            network_policy: None,
            mounts: vec![],
            default_timeout_secs: 120,
            idle_timeout_secs: 0,
        }
    }
}

/// Launch a persistent sandbox Worker process.
///
/// The Worker process runs inside the platform's native sandbox with constraints
/// matching the specified security level. Commands sent via the returned
/// [`WorkerHandle`] are executed inside the sandbox.
///
/// # Platform behavior
///
/// - **macOS**: Applies Seatbelt (SBPL) profile via `pre_exec` before exec.
/// - **Linux**: Applies Landlock + Seccomp via `pre_exec` (Phase 2).
/// - **Windows**: Binds to Job Object + Restricted Token (Phase 2).
///
/// # Errors
///
/// Returns `SandboxError` if:
/// - The CLI binary cannot be found
/// - Sandbox setup fails (invalid config, platform not supported)
/// - Process spawn fails
pub fn launch_worker(config: &WorkerLaunchConfig) -> Result<WorkerHandle, SandboxError> {
    let cli_binary = find_cli_binary()?;

    info!(
        binary = %cli_binary.display(),
        workspace = %config.workspace.display(),
        security = ?config.security_level,
        "launching worker process"
    );

    let mut cmd = Command::new(&cli_binary);
    cmd.arg("sandbox")
        .arg("worker-start")
        .arg("--workspace")
        .arg(&config.workspace)
        .arg("--timeout")
        .arg(config.default_timeout_secs.to_string())
        .arg("--security-level")
        .arg(security_level_to_str(config.security_level))
        .arg("--idle-timeout")
        .arg(config.idle_timeout_secs.to_string())
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::inherit()); // Worker logs flow to parent's stderr

    // Apply platform-specific sandbox
    #[cfg(target_os = "macos")]
    apply_macos_sandbox(&mut cmd, config)?;

    #[cfg(target_os = "linux")]
    apply_linux_sandbox(&mut cmd, config)?;

    #[cfg(target_os = "windows")]
    apply_windows_sandbox(&mut cmd, config)?;

    let child = cmd.spawn().map_err(|e| SandboxError::Io {
        context: format!(
            "spawning worker process '{}'",
            cli_binary.display()
        ),
        source: e,
    })?;

    let pid = child.id();
    debug!(pid, "worker process spawned");

    WorkerHandle::new(child).map_err(|e| SandboxError::Io {
        context: "creating worker handle from spawned process".into(),
        source: e,
    })
}

/// Launch a Worker process **without** sandbox constraints.
///
/// Useful for testing the Worker event loop and IPC without platform-specific
/// sandbox overhead. Should not be used in production.
pub fn launch_worker_unsandboxed(config: &WorkerLaunchConfig) -> Result<WorkerHandle, SandboxError> {
    let cli_binary = find_cli_binary()?;

    info!(
        binary = %cli_binary.display(),
        workspace = %config.workspace.display(),
        "launching unsandboxed worker process (testing only)"
    );

    let mut cmd = Command::new(&cli_binary);
    cmd.arg("sandbox")
        .arg("worker-start")
        .arg("--workspace")
        .arg(&config.workspace)
        .arg("--timeout")
        .arg(config.default_timeout_secs.to_string())
        .arg("--security-level")
        .arg(security_level_to_str(config.security_level))
        .arg("--idle-timeout")
        .arg(config.idle_timeout_secs.to_string())
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::inherit());

    let child = cmd.spawn().map_err(|e| SandboxError::Io {
        context: format!(
            "spawning unsandboxed worker process '{}'",
            cli_binary.display()
        ),
        source: e,
    })?;

    let pid = child.id();
    debug!(pid, "unsandboxed worker process spawned");

    WorkerHandle::new(child).map_err(|e| SandboxError::Io {
        context: "creating worker handle from spawned process".into(),
        source: e,
    })
}

/// Convert a [`SecurityLevel`] to its CLI argument string.
fn security_level_to_str(level: SecurityLevel) -> &'static str {
    match level {
        SecurityLevel::L0Deny => "deny",
        SecurityLevel::L1Sandbox => "sandbox",
        SecurityLevel::L2Full => "full",
    }
}

// ── Platform-specific sandbox setup ─────────────────────────────────────

/// Apply macOS Seatbelt sandbox to the Worker command (pre_exec).
#[cfg(target_os = "macos")]
fn apply_macos_sandbox(
    cmd: &mut Command,
    config: &WorkerLaunchConfig,
) -> Result<(), SandboxError> {
    use std::os::unix::process::CommandExt;
    use crate::macos::seatbelt;

    // Build a SandboxConfig for profile generation
    let sandbox_config = SandboxConfig {
        security_level: config.security_level,
        command: String::new(), // Not used for profile generation
        args: vec![],
        workspace: config.workspace.clone(),
        mounts: config.mounts.clone(),
        resource_limits: crate::config::ResourceLimits::default(),
        network_policy: config.network_policy,
        env_vars: std::collections::HashMap::new(),
        format: crate::config::OutputFormat::Json,
        backend: crate::config::BackendPreference::Native,
    };

    let profile = seatbelt::generate_profile(&sandbox_config)?;
    let params_refs: Vec<(&str, &str)> = profile
        .params
        .iter()
        .map(|(k, v)| (k.as_str(), v.as_str()))
        .collect();
    let sandbox_args = crate::macos::ffi::SandboxArgs::new(&profile.sbpl, &params_refs)?;

    debug!(
        profile_len = profile.sbpl.len(),
        param_count = profile.params.len(),
        "applying seatbelt sandbox to worker"
    );

    // SAFETY:
    // - `sandbox_args` is pre-built with all allocations complete before fork.
    // - `SandboxArgs::apply()` calls sandbox_init_with_parameters, safe post-fork.
    // - The sandbox is irreversible and inherited by the exec'd worker process.
    // - Verified by Skill 5: fork+exec'd children inherit Seatbelt constraints.
    unsafe {
        cmd.pre_exec(move || sandbox_args.apply());
    }

    Ok(())
}

/// Apply Linux Landlock + Seccomp sandbox to the Worker command (pre_exec).
#[cfg(target_os = "linux")]
fn apply_linux_sandbox(
    _cmd: &mut Command,
    _config: &WorkerLaunchConfig,
) -> Result<(), SandboxError> {
    // Phase 2: Implement Landlock + Seccomp pre_exec setup
    // For now, launch without Linux-specific sandbox
    // (the Worker is still useful for IPC latency reduction)
    tracing::warn!("Linux Worker sandbox not yet implemented — launching without Landlock/Seccomp");
    Ok(())
}

/// Apply Windows Job Object + Restricted Token sandbox to the Worker command.
#[cfg(target_os = "windows")]
fn apply_windows_sandbox(
    _cmd: &mut Command,
    _config: &WorkerLaunchConfig,
) -> Result<(), SandboxError> {
    // Phase 2: Implement Job Object + Restricted Token setup
    // Windows Worker needs CREATE_SUSPENDED → AssignToJob → ResumeThread pattern
    // For now, launch without Windows-specific sandbox
    tracing::warn!("Windows Worker sandbox not yet implemented — launching without Job Object");
    Ok(())
}

// ── Binary discovery ────────────────────────────────────────────────────

/// Find the CLI binary path for launching the Worker.
///
/// Discovery order:
/// 1. `OA_CLI_BINARY` environment variable (for testing/override)
/// 2. Current executable path (when running as the CLI)
/// 3. `which oa` fallback
fn find_cli_binary() -> Result<PathBuf, SandboxError> {
    // 1. Environment variable override
    if let Ok(path) = std::env::var("OA_CLI_BINARY") {
        let p = PathBuf::from(&path);
        if p.exists() {
            return Ok(p);
        }
        return Err(SandboxError::InvalidConfig {
            message: format!("OA_CLI_BINARY={path} does not exist"),
        });
    }

    // 2. Current executable (skip if path contains "(deleted)" — Linux upgrade edge case)
    if let Ok(exe) = std::env::current_exe() {
        let exe_str = exe.to_string_lossy();
        if exe.exists() && !exe_str.contains("(deleted)") {
            return Ok(exe);
        }
    }

    // 3. which fallback
    if let Ok(path) = which::which("oa") {
        return Ok(path);
    }

    Err(SandboxError::InvalidConfig {
        message: "cannot find CLI binary for Worker launcher: set OA_CLI_BINARY or ensure 'oa' is in PATH".into(),
    })
}

/// Check if a path is a valid executable.
#[allow(dead_code)]
fn is_executable(path: &Path) -> bool {
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        path.metadata()
            .map(|m| m.permissions().mode() & 0o111 != 0)
            .unwrap_or(false)
    }
    #[cfg(not(unix))]
    {
        path.exists()
    }
}
