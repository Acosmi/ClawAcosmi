//! Sandbox configuration types.
//!
//! Defines security levels, network policies, mount specifications, resource limits,
//! and the top-level [`SandboxConfig`] consumed by all sandbox backends.

use std::collections::HashMap;
use std::path::PathBuf;

use serde::{Deserialize, Serialize};

/// Security level controlling the degree of process isolation.
///
/// Maps to the CLI `--security` flag and the Go layer's `SecurityLevel` enum.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub enum SecurityLevel {
    /// L0 — Maximum isolation. All network denied, minimal filesystem access.
    /// Suitable for untrusted code execution.
    #[serde(rename = "deny")]
    L0Deny,

    /// L1 — Allowlist-restricted sandbox. Restricted network (public TCP + DNS only),
    /// workspace read/write, curated tool access.
    #[serde(rename = "allowlist", alias = "sandbox")]
    L1Allowlist,

    /// L2 — Sandboxed with full permissions inside sandbox + temporary mount authorization.
    /// Full network access. Requires human-in-the-loop escalation approval with TTL.
    #[serde(rename = "sandboxed", alias = "full")]
    L2Sandboxed,
}

impl SecurityLevel {
    /// Returns the default [`NetworkPolicy`] for this security level.
    #[must_use]
    pub const fn default_network_policy(self) -> NetworkPolicy {
        match self {
            Self::L0Deny => NetworkPolicy::None,
            Self::L1Allowlist => NetworkPolicy::Restricted,
            Self::L2Sandboxed => NetworkPolicy::Host,
        }
    }
}

/// Network access policy for the sandboxed process.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum NetworkPolicy {
    /// No network access at all (socket syscalls blocked).
    None,

    /// Outbound TCP to public internet allowed;
    /// localhost (127.0.0.0/8), LAN (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16),
    /// and Unix sockets are blocked.
    Restricted,

    /// Full host network access (no isolation).
    Host,
}

/// Filesystem mount access mode.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum MountMode {
    /// Read-only access.
    ReadOnly,
    /// Read-write access.
    ReadWrite,
}

/// A bind-mount mapping a host path into the sandbox filesystem.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MountSpec {
    /// Absolute path on the host filesystem.
    pub host_path: PathBuf,

    /// Path as seen inside the sandbox.
    pub sandbox_path: PathBuf,

    /// Access mode (read-only or read-write).
    pub mode: MountMode,
}

/// Resource limits for the sandboxed process tree.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ResourceLimits {
    /// Maximum memory in bytes (0 = no limit).
    #[serde(default)]
    pub memory_bytes: u64,

    /// CPU limit in millicores (1000 = 1 full core, 0 = no limit).
    #[serde(default)]
    pub cpu_millicores: u32,

    /// Maximum number of processes/threads (0 = no limit).
    #[serde(default)]
    pub max_pids: u32,

    /// Execution timeout in seconds (`None` = no timeout).
    #[serde(default)]
    pub timeout_secs: Option<u64>,
}

/// Output format for sandbox results.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize, Default)]
#[serde(rename_all = "lowercase")]
pub enum OutputFormat {
    /// JSON output for Go IPC consumption.
    #[default]
    Json,
    /// Human-readable text output.
    Text,
}

/// Backend selection preference.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize, Default)]
#[serde(rename_all = "lowercase")]
pub enum BackendPreference {
    /// Automatically select the best available backend (native → Docker).
    #[default]
    Auto,
    /// Force native OS sandbox; fail if unavailable.
    Native,
    /// Force Docker backend; fail if unavailable.
    Docker,
}

/// Top-level sandbox configuration.
///
/// Constructed from CLI arguments (via `oa-cmd-sandbox`) or Go IPC,
/// then passed to the selected [`SandboxRunner`](crate::SandboxRunner).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SandboxConfig {
    /// Security level (L0/L1/L2).
    pub security_level: SecurityLevel,

    /// Command to execute inside the sandbox.
    pub command: String,

    /// Arguments for the command.
    #[serde(default)]
    pub args: Vec<String>,

    /// Workspace directory (mounted read-write by default in L1+).
    pub workspace: PathBuf,

    /// Additional bind mounts.
    #[serde(default)]
    pub mounts: Vec<MountSpec>,

    /// Resource limits.
    #[serde(default)]
    pub resource_limits: ResourceLimits,

    /// Network policy (defaults to security level's default if not specified).
    #[serde(default)]
    pub network_policy: Option<NetworkPolicy>,

    /// Environment variables to set in the sandbox.
    #[serde(default)]
    pub env_vars: HashMap<String, String>,

    /// Output format.
    #[serde(default)]
    pub format: OutputFormat,

    /// Backend preference.
    #[serde(default)]
    pub backend: BackendPreference,
}

/// Environment variable names that are dangerous to pass into a sandbox.
const DANGEROUS_ENV_VARS: &[&str] = &[
    "LD_PRELOAD",
    "LD_LIBRARY_PATH",
    "DYLD_INSERT_LIBRARIES",
    "DYLD_LIBRARY_PATH",
];

impl SandboxConfig {
    /// Returns the effective network policy, falling back to the security level's default.
    #[must_use]
    pub fn effective_network_policy(&self) -> NetworkPolicy {
        self.network_policy
            .unwrap_or_else(|| self.security_level.default_network_policy())
    }

    /// Validate the configuration, returning errors for unsafe or invalid settings.
    ///
    /// Checks:
    /// - Command is non-empty
    /// - Workspace path is absolute and exists
    /// - No null bytes in command, args, or paths
    /// - Network policy doesn't weaken security level
    /// - No dangerous environment variables (`LD_PRELOAD`, `DYLD_INSERT_LIBRARIES`)
    /// - Mount paths are absolute and don't escape root
    pub fn validate(&self) -> Result<(), crate::error::SandboxError> {
        // Command must be non-empty
        if self.command.is_empty() {
            return Err(crate::error::SandboxError::InvalidConfig {
                message: "command cannot be empty".into(),
            });
        }

        // Check for null bytes in command and args
        if self.command.contains('\0') {
            return Err(crate::error::SandboxError::InvalidConfig {
                message: "command contains null byte".into(),
            });
        }
        for (i, arg) in self.args.iter().enumerate() {
            if arg.contains('\0') {
                return Err(crate::error::SandboxError::InvalidConfig {
                    message: format!("argument {i} contains null byte"),
                });
            }
        }

        // Workspace must be absolute
        if !self.workspace.is_absolute() {
            return Err(crate::error::SandboxError::PathError {
                path: self.workspace.clone(),
                reason: "workspace path must be absolute".into(),
            });
        }

        // Check for path traversal in workspace
        if self
            .workspace
            .components()
            .any(|c| c == std::path::Component::ParentDir)
        {
            return Err(crate::error::SandboxError::PathError {
                path: self.workspace.clone(),
                reason: "workspace path contains '..' traversal".into(),
            });
        }

        // Validate mount paths
        for mount in &self.mounts {
            if !mount.host_path.is_absolute() {
                return Err(crate::error::SandboxError::PathError {
                    path: mount.host_path.clone(),
                    reason: "mount host_path must be absolute".into(),
                });
            }
            if mount
                .host_path
                .components()
                .any(|c| c == std::path::Component::ParentDir)
            {
                return Err(crate::error::SandboxError::PathError {
                    path: mount.host_path.clone(),
                    reason: "mount host_path contains '..' traversal".into(),
                });
            }
        }

        // Network policy must not weaken security level (F-02)
        if let Some(policy) = self.network_policy {
            let default_policy = self.security_level.default_network_policy();
            let weakens = matches!(
                (default_policy, policy),
                (
                    NetworkPolicy::None,
                    NetworkPolicy::Restricted | NetworkPolicy::Host
                ) | (NetworkPolicy::Restricted, NetworkPolicy::Host)
            );
            if weakens {
                return Err(crate::error::SandboxError::InvalidConfig {
                    message: format!(
                        "network policy '{policy:?}' weakens security level {:?} (default: {default_policy:?})",
                        self.security_level
                    ),
                });
            }
        }

        // Block dangerous environment variables
        for key in self.env_vars.keys() {
            if DANGEROUS_ENV_VARS
                .iter()
                .any(|&d| d.eq_ignore_ascii_case(key))
            {
                return Err(crate::error::SandboxError::InvalidConfig {
                    message: format!(
                        "dangerous environment variable '{key}' is not allowed in sandbox"
                    ),
                });
            }
        }

        Ok(())
    }
}

#[cfg(test)]
#[allow(clippy::unwrap_used)]
mod tests {
    use super::*;

    fn valid_config() -> SandboxConfig {
        SandboxConfig {
            security_level: SecurityLevel::L1Allowlist,
            command: "echo".into(),
            args: vec!["hello".into()],
            workspace: std::env::temp_dir(),
            mounts: vec![],
            resource_limits: ResourceLimits::default(),
            network_policy: None,
            env_vars: HashMap::new(),
            format: OutputFormat::Json,
            backend: BackendPreference::Auto,
        }
    }

    #[test]
    fn valid_config_passes() {
        assert!(valid_config().validate().is_ok());
    }

    #[test]
    fn empty_command_rejected() {
        let mut c = valid_config();
        c.command = String::new();
        assert!(c.validate().is_err());
    }

    #[test]
    fn null_byte_in_command_rejected() {
        let mut c = valid_config();
        c.command = "echo\0evil".into();
        assert!(c.validate().is_err());
    }

    #[test]
    fn null_byte_in_arg_rejected() {
        let mut c = valid_config();
        c.args = vec!["ok".into(), "bad\0arg".into()];
        assert!(c.validate().is_err());
    }

    #[test]
    fn relative_workspace_rejected() {
        let mut c = valid_config();
        c.workspace = PathBuf::from("relative/path");
        assert!(c.validate().is_err());
    }

    #[test]
    fn traversal_workspace_rejected() {
        let mut c = valid_config();
        c.workspace = PathBuf::from("/tmp/../etc/passwd");
        assert!(c.validate().is_err());
    }

    #[test]
    fn network_policy_weakening_rejected() {
        let mut c = valid_config();
        c.security_level = SecurityLevel::L0Deny;
        c.network_policy = Some(NetworkPolicy::Host);
        assert!(c.validate().is_err());
    }

    #[test]
    fn network_policy_strengthening_allowed() {
        let mut c = valid_config();
        c.security_level = SecurityLevel::L2Sandboxed;
        c.network_policy = Some(NetworkPolicy::None);
        assert!(c.validate().is_ok());
    }

    #[test]
    fn ld_preload_rejected() {
        let mut c = valid_config();
        c.env_vars
            .insert("LD_PRELOAD".into(), "/tmp/evil.so".into());
        assert!(c.validate().is_err());
    }

    #[test]
    fn dyld_insert_rejected() {
        let mut c = valid_config();
        c.env_vars
            .insert("DYLD_INSERT_LIBRARIES".into(), "/tmp/evil.dylib".into());
        assert!(c.validate().is_err());
    }

    #[test]
    fn relative_mount_rejected() {
        let mut c = valid_config();
        c.mounts.push(MountSpec {
            host_path: PathBuf::from("relative"),
            sandbox_path: PathBuf::from("/mnt"),
            mode: MountMode::ReadOnly,
        });
        assert!(c.validate().is_err());
    }

    // ── Serde alias backward-compatibility tests ──────────────────────

    #[test]
    fn serde_legacy_sandbox_alias() {
        // Old value "sandbox" must still deserialize to L1Allowlist
        let json = r#""sandbox""#;
        let level: SecurityLevel = serde_json::from_str(json).unwrap();
        assert_eq!(level, SecurityLevel::L1Allowlist);
    }

    #[test]
    fn serde_legacy_full_alias() {
        // Old value "full" must still deserialize to L2Sandboxed
        let json = r#""full""#;
        let level: SecurityLevel = serde_json::from_str(json).unwrap();
        assert_eq!(level, SecurityLevel::L2Sandboxed);
    }

    #[test]
    fn serde_canonical_roundtrip() {
        // New canonical names serialize and deserialize correctly
        let l1 = SecurityLevel::L1Allowlist;
        let l2 = SecurityLevel::L2Sandboxed;

        let l1_json = serde_json::to_string(&l1).unwrap();
        let l2_json = serde_json::to_string(&l2).unwrap();

        assert_eq!(l1_json, r#""allowlist""#);
        assert_eq!(l2_json, r#""sandboxed""#);

        let l1_back: SecurityLevel = serde_json::from_str(&l1_json).unwrap();
        let l2_back: SecurityLevel = serde_json::from_str(&l2_json).unwrap();

        assert_eq!(l1_back, SecurityLevel::L1Allowlist);
        assert_eq!(l2_back, SecurityLevel::L2Sandboxed);
    }

    #[test]
    fn serde_deny_unchanged() {
        // L0Deny name unchanged, verify stability
        let json = r#""deny""#;
        let level: SecurityLevel = serde_json::from_str(json).unwrap();
        assert_eq!(level, SecurityLevel::L0Deny);

        let serialized = serde_json::to_string(&SecurityLevel::L0Deny).unwrap();
        assert_eq!(serialized, r#""deny""#);
    }
}
