//! Docker fallback backend.
//!
//! When native OS sandbox backends are unavailable, falls back to executing
//! commands inside a Docker container via the `docker` CLI.
//!
//! This backend maps [`SandboxConfig`] to `docker run` arguments and produces
//! the same [`SandboxOutput`] JSON contract.
//!
//! # Docker argument mapping
//!
//! | SandboxConfig field   | Docker argument                                       |
//! |-----------------------|-------------------------------------------------------|
//! | `security_level`      | `--cap-drop=ALL`, `--read-only`, `--security-opt`     |
//! | `network_policy`      | `--network none` / `--network bridge`                 |
//! | `mounts`              | `-v host:container:ro` / `-v host:container:rw`       |
//! | `resource_limits`     | `--memory`, `--cpus`, `--pids-limit`                  |
//! | `env_vars`            | `-e KEY=VALUE`                                        |
//! | `workspace`           | `-v workspace:/workspace:rw` + `-w /workspace`        |

use std::process::Command;
use std::time::Instant;

use tracing::debug;

use crate::SandboxRunner;
use crate::config::{MountMode, NetworkPolicy, SandboxConfig, SecurityLevel};
use crate::error::SandboxError;
use crate::output::SandboxOutput;

/// Default Docker image for the fallback backend.
/// Matches the Go layer's `DefaultDockerConfig().DockerImage`.
const DEFAULT_IMAGE: &str = "alpine:3.19";

/// Default container working directory.
const CONTAINER_WORKDIR: &str = "/workspace";

/// Default tmpfs size in MB for read-only root mode.
const DEFAULT_TMPFS_SIZE_MB: u32 = 64;

/// Default PID limit when not specified.
const DEFAULT_PIDS_LIMIT: u32 = 256;

/// Docker CLI fallback runner.
///
/// Used when no native sandbox is available. Requires `docker` in PATH.
pub struct DockerFallbackRunner {
    /// Docker image to use. Defaults to [`DEFAULT_IMAGE`].
    image: String,
}

impl DockerFallbackRunner {
    /// Create a new Docker fallback runner with the default image.
    #[must_use]
    pub fn new() -> Self {
        Self {
            image: DEFAULT_IMAGE.to_owned(),
        }
    }

    /// Create a Docker fallback runner with a custom image.
    #[must_use]
    pub const fn with_image(image: String) -> Self {
        Self { image }
    }

    /// Build `docker run` arguments from a [`SandboxConfig`].
    fn build_docker_args(&self, config: &SandboxConfig) -> Vec<String> {
        let mut args = vec!["run".to_owned(), "--rm".to_owned()];

        // ── Security hardening ───────────────────────────────────────
        args.extend_from_slice(&[
            "--cap-drop=ALL".to_owned(),
            "--security-opt".to_owned(),
            "no-new-privileges:true".to_owned(),
        ]);

        // Read-only root for L0 (deny)
        let read_only = config.security_level == SecurityLevel::L0Deny;
        if read_only {
            args.push("--read-only".to_owned());
            args.push(format!(
                "--tmpfs=/tmp:rw,noexec,nosuid,size={DEFAULT_TMPFS_SIZE_MB}m"
            ));
        }

        // ── Network policy ───────────────────────────────────────────
        match config.effective_network_policy() {
            NetworkPolicy::None => args.push("--network=none".to_owned()),
            NetworkPolicy::Restricted | NetworkPolicy::Host => {
                // Docker bridge provides basic network; true restricted mode
                // (blocking localhost/LAN) would need iptables rules inside the container.
                // For "host" we still use bridge for isolation.
                args.push("--network=bridge".to_owned());
            }
        }

        // ── Resource limits ──────────────────────────────────────────
        let limits = &config.resource_limits;

        if limits.memory_bytes > 0 {
            args.push(format!("--memory={}b", limits.memory_bytes));
        }

        if limits.cpu_millicores > 0 {
            let cpus = f64::from(limits.cpu_millicores) / 1000.0;
            args.push(format!("--cpus={cpus:.2}"));
        }

        let pids = if limits.max_pids > 0 {
            limits.max_pids
        } else {
            DEFAULT_PIDS_LIMIT
        };
        args.extend_from_slice(&["--pids-limit".to_owned(), pids.to_string()]);

        // ── Workspace mount ──────────────────────────────────────────
        let workspace_mode = match config.security_level {
            SecurityLevel::L0Deny => "ro",
            SecurityLevel::L1Sandbox | SecurityLevel::L2Full => "rw",
        };
        args.extend_from_slice(&[
            "-v".to_owned(),
            format!(
                "{}:{CONTAINER_WORKDIR}:{workspace_mode}",
                config.workspace.display()
            ),
        ]);
        args.extend_from_slice(&["-w".to_owned(), CONTAINER_WORKDIR.to_owned()]);

        // ── Additional mounts ────────────────────────────────────────
        for mount in &config.mounts {
            let mode = match mount.mode {
                MountMode::ReadOnly => "ro",
                MountMode::ReadWrite => "rw",
            };
            args.extend_from_slice(&[
                "-v".to_owned(),
                format!(
                    "{}:{}:{mode}",
                    mount.host_path.display(),
                    mount.sandbox_path.display()
                ),
            ]);
        }

        // ── Environment variables ────────────────────────────────────
        for (key, value) in &config.env_vars {
            args.extend_from_slice(&["-e".to_owned(), format!("{key}={value}")]);
        }

        // ── Image + command ──────────────────────────────────────────
        args.push(self.image.clone());
        args.push(config.command.clone());
        args.extend(config.args.iter().cloned());

        args
    }
}

impl Default for DockerFallbackRunner {
    fn default() -> Self {
        Self::new()
    }
}

impl SandboxRunner for DockerFallbackRunner {
    fn name(&self) -> &'static str {
        "docker-fallback"
    }

    fn available(&self) -> bool {
        which::which("docker").is_ok()
    }

    fn run(&self, config: &SandboxConfig) -> Result<SandboxOutput, SandboxError> {
        if !self.available() {
            return Err(SandboxError::PlatformNotSupported {
                platform: "docker".into(),
                reason: "docker CLI not found in PATH".into(),
            });
        }

        let args = self.build_docker_args(config);
        debug!(args = ?args, "docker run arguments");

        let start = Instant::now();

        let mut child = Command::new("docker")
            .args(&args)
            .stdout(std::process::Stdio::piped())
            .stderr(std::process::Stdio::piped())
            .spawn()
            .map_err(|e| SandboxError::Io {
                context: "failed to spawn docker process".into(),
                source: e,
            })?;

        // ── Timeout handling ─────────────────────────────────────────
        // Read stdout/stderr in background threads, then poll for exit
        // with timeout. This avoids the kill-from-thread race.
        let stdout_pipe = child.stdout.take();
        let stderr_pipe = child.stderr.take();

        let stdout_handle = std::thread::spawn(move || {
            stdout_pipe.map_or_else(String::new, |mut pipe| {
                let mut buf = String::new();
                let _ = std::io::Read::read_to_string(&mut pipe, &mut buf);
                buf
            })
        });
        let stderr_handle = std::thread::spawn(move || {
            stderr_pipe.map_or_else(String::new, |mut pipe| {
                let mut buf = String::new();
                let _ = std::io::Read::read_to_string(&mut pipe, &mut buf);
                buf
            })
        });

        let timeout_secs = config.resource_limits.timeout_secs;

        let exit_status = if let Some(secs) = timeout_secs {
            let deadline = Instant::now() + std::time::Duration::from_secs(secs);
            loop {
                match child.try_wait() {
                    Ok(Some(status)) => break Some(status),
                    Ok(None) => {
                        if Instant::now() >= deadline {
                            let _ = child.kill();
                            let _ = child.wait();
                            break None; // timed out
                        }
                        std::thread::sleep(std::time::Duration::from_millis(50));
                    }
                    Err(_) => break None,
                }
            }
        } else {
            child.wait().ok()
        };

        let stdout_content = stdout_handle.join().unwrap_or_default();
        let stderr_content = stderr_handle.join().unwrap_or_default();

        let duration = start.elapsed();

        let Some(exit_status) = exit_status else {
            return Err(SandboxError::Timeout {
                timeout_secs: timeout_secs.unwrap_or(0),
            });
        };

        let exit_code = exit_status.code().unwrap_or(1);

        // Docker exit code 125 = Docker daemon error, 126 = command cannot be invoked,
        // 127 = command not found in container
        if exit_code == 127 {
            return Err(SandboxError::CommandNotFound {
                command: config.command.clone(),
            });
        }

        Ok(SandboxOutput {
            stdout: stdout_content,
            stderr: stderr_content,
            exit_code,
            error: None,
            duration_ms: u64::try_from(duration.as_millis()).unwrap_or(u64::MAX),
            sandbox_backend: self.name().to_owned(),
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::{BackendPreference, OutputFormat, ResourceLimits};
    use std::collections::HashMap;

    fn make_config(command: &str, args: &[&str], level: SecurityLevel) -> SandboxConfig {
        SandboxConfig {
            security_level: level,
            command: command.into(),
            args: args.iter().map(|s| (*s).into()).collect(),
            workspace: std::env::temp_dir(),
            mounts: vec![],
            resource_limits: ResourceLimits::default(),
            network_policy: None,
            env_vars: HashMap::new(),
            format: OutputFormat::Json,
            backend: BackendPreference::Docker,
        }
    }

    #[test]
    fn docker_args_l0_deny() {
        let runner = DockerFallbackRunner::new();
        let config = make_config("echo", &["hello"], SecurityLevel::L0Deny);
        let args = runner.build_docker_args(&config);

        assert!(args.contains(&"--read-only".to_owned()));
        assert!(args.contains(&"--network=none".to_owned()));
        assert!(args.contains(&"--cap-drop=ALL".to_owned()));

        // Workspace should be read-only
        let workspace_mount = args.iter().find(|a| a.contains("/workspace:ro"));
        assert!(workspace_mount.is_some(), "workspace should be ro in L0");
    }

    #[test]
    fn docker_args_l1_sandbox() {
        let runner = DockerFallbackRunner::new();
        let config = make_config("echo", &["hello"], SecurityLevel::L1Sandbox);
        let args = runner.build_docker_args(&config);

        // L1 should NOT be read-only root
        assert!(!args.contains(&"--read-only".to_owned()));
        // L1 default network is Restricted → bridge
        assert!(args.contains(&"--network=bridge".to_owned()));
        // Workspace should be read-write
        let workspace_mount = args.iter().find(|a| a.contains("/workspace:rw"));
        assert!(workspace_mount.is_some(), "workspace should be rw in L1");
    }

    #[test]
    fn docker_args_memory_limit() {
        let runner = DockerFallbackRunner::new();
        let mut config = make_config("echo", &["hello"], SecurityLevel::L1Sandbox);
        config.resource_limits.memory_bytes = 256 * 1024 * 1024; // 256MB
        let args = runner.build_docker_args(&config);

        let mem_arg = args.iter().find(|a| a.starts_with("--memory="));
        assert!(mem_arg.is_some());
        assert_eq!(mem_arg.map(String::as_str), Some("--memory=268435456b"));
    }

    #[test]
    fn docker_args_env_vars() {
        let runner = DockerFallbackRunner::new();
        let mut config = make_config("echo", &["hello"], SecurityLevel::L1Sandbox);
        config.env_vars.insert("FOO".into(), "bar".into());
        let args = runner.build_docker_args(&config);

        let env_idx = args.iter().position(|a| a == "-e");
        assert!(env_idx.is_some());
        let env_val = &args[env_idx.map(|i| i + 1).unwrap_or(0)];
        assert_eq!(env_val, "FOO=bar");
    }

    #[test]
    fn docker_args_custom_network() {
        let runner = DockerFallbackRunner::new();
        let mut config = make_config("echo", &["hello"], SecurityLevel::L1Sandbox);
        config.network_policy = Some(NetworkPolicy::None);
        let args = runner.build_docker_args(&config);

        assert!(args.contains(&"--network=none".to_owned()));
    }

    #[test]
    fn custom_image() {
        let runner = DockerFallbackRunner::with_image("ubuntu:24.04".into());
        let config = make_config("echo", &["hello"], SecurityLevel::L1Sandbox);
        let args = runner.build_docker_args(&config);

        assert!(args.contains(&"ubuntu:24.04".to_owned()));
        assert!(!args.contains(&DEFAULT_IMAGE.to_owned()));
    }
}
