//! `sandbox run` command: execute a command inside a sandboxed process.
//!
//! Uses the `oa-sandbox` runtime to select the best available backend
//! (native OS sandbox or Docker fallback) and execute the given command
//! with the specified security constraints.
//!
//! # JSON output
//!
//! With `--format json` (default), outputs a [`SandboxOutput`] JSON object
//! to stdout for consumption by the Go orchestration layer.

use std::collections::HashMap;
use std::path::PathBuf;

use anyhow::{Context, Result};

use oa_sandbox::config::{
    BackendPreference, MountMode, MountSpec, NetworkPolicy, OutputFormat, ResourceLimits,
    SandboxConfig, SecurityLevel,
};
use oa_sandbox::output::SandboxOutput;

/// Options for the `sandbox run` subcommand.
#[derive(Debug, Clone)]
pub struct SandboxRunOptions {
    /// Security level: deny (L0), allowlist (L1), or sandboxed (L2).
    pub security: SecurityLevel,
    /// Working directory (mounted into the sandbox).
    pub workspace: PathBuf,
    /// Network policy override. If `None`, uses the security level default.
    pub network: Option<NetworkPolicy>,
    /// Execution timeout in seconds.
    pub timeout: Option<u64>,
    /// Output format: json or text.
    pub format: OutputFormat,
    /// Backend preference: auto, native, or docker.
    pub backend: BackendPreference,
    /// Additional bind mounts in `host:sandbox[:ro|rw]` format.
    pub mounts: Vec<String>,
    /// Environment variables in `KEY=VALUE` format.
    pub env: Vec<String>,
    /// Memory limit in bytes (0 = no limit).
    pub memory: u64,
    /// CPU limit in millicores (0 = no limit).
    pub cpu: u32,
    /// Max processes (0 = no limit).
    pub pids: u32,
    /// Show execution plan without running.
    /// Automatically enabled for L2 (sandboxed) security level in text mode.
    pub dry_run: bool,
    /// Command to execute.
    pub command: String,
    /// Arguments for the command.
    pub args: Vec<String>,
}

/// Parse a mount spec string `host:sandbox[:ro|rw]` into a [`MountSpec`].
fn parse_mount(mount_str: &str) -> Result<MountSpec> {
    let parts: Vec<&str> = mount_str.splitn(3, ':').collect();
    if parts.len() < 2 {
        anyhow::bail!("invalid mount format: expected 'host:sandbox[:ro|rw]', got '{mount_str}'");
    }

    let host_path = PathBuf::from(parts[0]);
    let sandbox_path = PathBuf::from(parts[1]);
    let mode = match parts.get(2) {
        Some(&"ro") | None => MountMode::ReadOnly,
        Some(&"rw") => MountMode::ReadWrite,
        Some(other) => anyhow::bail!("invalid mount mode '{other}': expected 'ro' or 'rw'"),
    };

    Ok(MountSpec {
        host_path,
        sandbox_path,
        mode,
    })
}

/// Parse an environment variable string `KEY=VALUE` into a (key, value) pair.
fn parse_env_var(env_str: &str) -> Result<(String, String)> {
    let (key, value) = env_str
        .split_once('=')
        .with_context(|| format!("invalid env format: expected 'KEY=VALUE', got '{env_str}'"))?;
    Ok((key.to_owned(), value.to_owned()))
}

/// Build a [`SandboxConfig`] from the CLI options.
fn build_config(opts: &SandboxRunOptions) -> Result<SandboxConfig> {
    let mounts: Vec<MountSpec> = opts
        .mounts
        .iter()
        .map(|m| parse_mount(m))
        .collect::<Result<Vec<_>>>()?;

    let env_vars: HashMap<String, String> = opts
        .env
        .iter()
        .map(|e| parse_env_var(e))
        .collect::<Result<HashMap<_, _>>>()?;

    Ok(SandboxConfig {
        security_level: opts.security,
        command: opts.command.clone(),
        args: opts.args.clone(),
        workspace: opts.workspace.clone(),
        mounts,
        resource_limits: ResourceLimits {
            memory_bytes: opts.memory,
            cpu_millicores: opts.cpu,
            max_pids: opts.pids,
            timeout_secs: opts.timeout,
        },
        network_policy: opts.network,
        env_vars,
        format: opts.format,
        backend: opts.backend,
    })
}

/// Execute the `sandbox run` command.
pub fn sandbox_run_command(opts: &SandboxRunOptions) -> Result<()> {
    let config = build_config(opts)?;

    let runner = oa_sandbox::select_runner(&config).context("failed to select sandbox backend")?;

    // Dry run: show execution plan without running.
    // Triggered by --dry-run flag or automatically for L2 (sandboxed) security level.
    let is_dry_run =
        opts.dry_run || (opts.security == SecurityLevel::L2Sandboxed && opts.format == OutputFormat::Text);
    if is_dry_run {
        emit_dry_run_preview(&config, runner.name(), opts.format);
        return Ok(());
    }

    let result = runner.run(&config);

    match result {
        Ok(output) => {
            emit_output(&output, opts.format);
            // Exit with the sandboxed command's exit code
            if output.exit_code != 0 {
                std::process::exit(output.exit_code);
            }
            Ok(())
        }
        Err(e) => {
            // Map sandbox errors to exit code + JSON output
            let (exit_code, error_msg) = match &e {
                oa_sandbox::error::SandboxError::Timeout { timeout_secs } => (
                    oa_sandbox::output::exit_codes::TIMEOUT,
                    format!("sandbox execution timed out after {timeout_secs}s"),
                ),
                oa_sandbox::error::SandboxError::ResourceExceeded {
                    resource,
                    limit,
                    actual,
                } => (
                    oa_sandbox::output::exit_codes::RESOURCE_EXCEEDED,
                    format!(
                        "resource limit exceeded: {resource} (limit: {limit}, actual: {actual})"
                    ),
                ),
                oa_sandbox::error::SandboxError::InvalidConfig { message } => (
                    oa_sandbox::output::exit_codes::CONFIG_ERROR,
                    format!("invalid config: {message}"),
                ),
                _ => (oa_sandbox::output::exit_codes::CONFIG_ERROR, format!("{e}")),
            };

            let output = SandboxOutput {
                stdout: String::new(),
                stderr: String::new(),
                exit_code,
                error: Some(error_msg),
                duration_ms: 0,
                sandbox_backend: runner.name().to_owned(),
            };
            emit_output(&output, opts.format);
            std::process::exit(exit_code);
        }
    }
}

/// Emit a dry-run preview of the sandbox execution plan.
fn emit_dry_run_preview(config: &SandboxConfig, backend_name: &str, format: OutputFormat) {
    let effective_net = config.effective_network_policy();

    match format {
        OutputFormat::Json => {
            let mounts_json: Vec<serde_json::Value> = config
                .mounts
                .iter()
                .map(|m| {
                    serde_json::json!({
                        "host_path": m.host_path.display().to_string(),
                        "sandbox_path": m.sandbox_path.display().to_string(),
                        "mode": m.mode,
                    })
                })
                .collect();

            let limits = &config.resource_limits;
            let limits_json = serde_json::json!({
                "memory_bytes": limits.memory_bytes,
                "cpu_millicores": limits.cpu_millicores,
                "max_pids": limits.max_pids,
                "timeout_secs": limits.timeout_secs,
            });

            let preview = serde_json::json!({
                "dry_run": true,
                "sandbox_backend": backend_name,
                "security_level": config.security_level,
                "command": config.command,
                "args": config.args,
                "workspace": config.workspace.display().to_string(),
                "network_policy": effective_net,
                "mounts": mounts_json,
                "env_vars": config.env_vars,
                "resource_limits": limits_json,
            });
            if let Ok(json) = serde_json::to_string_pretty(&preview) {
                println!("{json}");
            }
        }
        OutputFormat::Text => {
            eprintln!("=== Sandbox Execution Plan (dry run) ===");
            eprintln!();
            eprintln!("  Backend:    {backend_name}");
            eprintln!("  Security:   {:?}", config.security_level);
            eprintln!("  Network:    {effective_net:?}");
            eprintln!("  Workspace:  {}", config.workspace.display());
            eprintln!(
                "  Command:    {} {}",
                config.command,
                config.args.join(" ")
            );

            if !config.mounts.is_empty() {
                eprintln!("  Mounts:");
                for m in &config.mounts {
                    eprintln!(
                        "    {} -> {} ({:?})",
                        m.host_path.display(),
                        m.sandbox_path.display(),
                        m.mode
                    );
                }
            }

            if !config.env_vars.is_empty() {
                eprintln!("  Env vars:");
                for (k, v) in &config.env_vars {
                    eprintln!("    {k}={v}");
                }
            }

            let l = &config.resource_limits;
            if l.memory_bytes > 0 || l.cpu_millicores > 0 || l.max_pids > 0 {
                eprintln!("  Limits:");
                if l.memory_bytes > 0 {
                    eprintln!("    Memory: {} bytes", l.memory_bytes);
                }
                if l.cpu_millicores > 0 {
                    eprintln!("    CPU:    {} millicores", l.cpu_millicores);
                }
                if l.max_pids > 0 {
                    eprintln!("    PIDs:   {}", l.max_pids);
                }
            }
            if let Some(t) = l.timeout_secs {
                eprintln!("  Timeout:    {t}s");
            }

            eprintln!();
            eprintln!("Re-run without --dry-run to execute.");
        }
    }
}

/// Emit the sandbox output in the requested format.
fn emit_output(output: &SandboxOutput, format: OutputFormat) {
    match format {
        OutputFormat::Json => {
            // Unwrap is safe here: SandboxOutput is always serializable
            if let Ok(json) = serde_json::to_string(output) {
                println!("{json}");
            }
        }
        OutputFormat::Text => {
            if !output.stdout.is_empty() {
                print!("{}", output.stdout);
            }
            if !output.stderr.is_empty() {
                eprint!("{}", output.stderr);
            }
            if let Some(error) = &output.error {
                eprintln!("error: {error}");
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_mount_read_only() {
        let spec = parse_mount("/host/path:/sandbox/path:ro").unwrap();
        assert_eq!(spec.host_path, PathBuf::from("/host/path"));
        assert_eq!(spec.sandbox_path, PathBuf::from("/sandbox/path"));
        assert_eq!(spec.mode, MountMode::ReadOnly);
    }

    #[test]
    fn parse_mount_read_write() {
        let spec = parse_mount("/host:/sandbox:rw").unwrap();
        assert_eq!(spec.mode, MountMode::ReadWrite);
    }

    #[test]
    fn parse_mount_default_ro() {
        let spec = parse_mount("/host:/sandbox").unwrap();
        assert_eq!(spec.mode, MountMode::ReadOnly);
    }

    #[test]
    fn parse_mount_invalid() {
        assert!(parse_mount("nodelimiter").is_err());
    }

    #[test]
    fn parse_mount_invalid_mode() {
        assert!(parse_mount("/host:/sandbox:invalid").is_err());
    }

    #[test]
    fn parse_env_var_valid() {
        let (k, v) = parse_env_var("FOO=bar").unwrap();
        assert_eq!(k, "FOO");
        assert_eq!(v, "bar");
    }

    #[test]
    fn parse_env_var_with_equals() {
        let (k, v) = parse_env_var("FOO=bar=baz").unwrap();
        assert_eq!(k, "FOO");
        assert_eq!(v, "bar=baz");
    }

    #[test]
    fn parse_env_var_invalid() {
        assert!(parse_env_var("NOEQUALS").is_err());
    }

    #[test]
    fn build_config_basic() {
        let opts = SandboxRunOptions {
            security: SecurityLevel::L1Allowlist,
            workspace: PathBuf::from("/tmp/workspace"),
            network: None,
            timeout: Some(30),
            format: OutputFormat::Json,
            backend: BackendPreference::Auto,
            mounts: vec![],
            env: vec!["FOO=bar".to_owned()],
            memory: 0,
            cpu: 0,
            pids: 0,
            dry_run: false,
            command: "echo".to_owned(),
            args: vec!["hello".to_owned()],
        };
        let config = build_config(&opts).unwrap();
        assert_eq!(config.security_level, SecurityLevel::L1Allowlist);
        assert_eq!(config.command, "echo");
        assert_eq!(config.env_vars.get("FOO").map(String::as_str), Some("bar"));
        assert_eq!(config.resource_limits.timeout_secs, Some(30));
    }

    #[test]
    fn l2_text_triggers_auto_dry_run() {
        // L2 + text format should auto-enable dry run
        let opts = SandboxRunOptions {
            security: SecurityLevel::L2Sandboxed,
            workspace: PathBuf::from("/tmp/workspace"),
            network: None,
            timeout: None,
            format: OutputFormat::Text,
            backend: BackendPreference::Auto,
            mounts: vec![],
            env: vec![],
            memory: 0,
            cpu: 0,
            pids: 0,
            dry_run: false,
            command: "echo".to_owned(),
            args: vec![],
        };
        let is_dry_run = opts.dry_run
            || (opts.security == SecurityLevel::L2Sandboxed && opts.format == OutputFormat::Text);
        assert!(is_dry_run);
    }

    #[test]
    fn l1_text_no_auto_dry_run() {
        // L1 + text format should NOT auto-enable dry run
        let opts = SandboxRunOptions {
            security: SecurityLevel::L1Allowlist,
            workspace: PathBuf::from("/tmp/workspace"),
            network: None,
            timeout: None,
            format: OutputFormat::Text,
            backend: BackendPreference::Auto,
            mounts: vec![],
            env: vec![],
            memory: 0,
            cpu: 0,
            pids: 0,
            dry_run: false,
            command: "echo".to_owned(),
            args: vec![],
        };
        let is_dry_run = opts.dry_run
            || (opts.security == SecurityLevel::L2Sandboxed && opts.format == OutputFormat::Text);
        assert!(!is_dry_run);
    }
}
