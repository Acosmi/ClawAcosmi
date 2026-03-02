//! SBPL (Sandbox Profile Language) profile generator.
//!
//! Generates Seatbelt profiles dynamically from [`SandboxConfig`], following
//! the Chromium model of parameterized per-use profiles.
//!
//! # Profile structure
//!
//! ```scheme
//! (version 1)
//! (deny default)
//! (import "bsd.sb")        ; minimal baseline (dynamic linker, locale, /dev/urandom)
//! ;; workspace, network, process, system paths, devices — generated per config
//! ```
//!
//! # Key design decisions
//!
//! - Base import is `bsd.sb` (NOT `system.sb` — too permissive)
//! - Workspace path is injected via `(param "WORKSPACE_DIR")` for safe escaping
//! - Process execution is broadly allowed (the sandbox restricts resources, not which
//!   binaries can run — that's the CLI layer's responsibility)
//! - Network deny rules take precedence over allow rules in SBPL

use std::fmt::Write as _;

use crate::config::{MountMode, NetworkPolicy, SandboxConfig, SecurityLevel};
use crate::error::SandboxError;

/// A generated sandbox profile with its parameter bindings.
pub struct SandboxProfile {
    /// The SBPL source string.
    pub sbpl: String,
    /// Key-value parameters referenced in the profile via `(param "KEY")`.
    pub params: Vec<(String, String)>,
}

/// Generate a Seatbelt SBPL profile from the given sandbox configuration.
///
/// The workspace directory is passed as a parameter (`WORKSPACE_DIR`) to avoid
/// SBPL injection via path names containing special characters.
pub fn generate_profile(config: &SandboxConfig) -> Result<SandboxProfile, SandboxError> {
    let mut sbpl = String::with_capacity(2048);
    let mut params: Vec<(String, String)> = Vec::new();

    // ── Header ─────────────────────────────────────────────────────────────
    sbpl.push_str("(version 1)\n");
    sbpl.push_str("(deny default)\n");
    sbpl.push_str("(import \"bsd.sb\")\n\n");

    // ── Workspace access ───────────────────────────────────────────────────
    // Canonicalize workspace path — on macOS, /var → /private/var is a symlink
    // and Seatbelt operates on resolved paths.
    let canonical_workspace =
        config
            .workspace
            .canonicalize()
            .map_err(|e| SandboxError::PathError {
                path: config.workspace.clone(),
                reason: format!("failed to canonicalize workspace: {e}"),
            })?;
    let workspace_str = canonical_workspace
        .to_str()
        .ok_or_else(|| SandboxError::PathError {
            path: config.workspace.clone(),
            reason: "workspace path is not valid UTF-8".into(),
        })?;
    params.push(("WORKSPACE_DIR".into(), workspace_str.into()));

    // TMPDIR parameter — used for per-user temp directory access.
    // Must canonicalize for the same symlink reason as workspace.
    let tmpdir_raw = std::env::var("TMPDIR").unwrap_or_else(|_| "/tmp".into());
    let tmpdir = std::path::Path::new(&tmpdir_raw)
        .canonicalize()
        .map(|p| p.to_string_lossy().into_owned())
        .unwrap_or(tmpdir_raw);
    params.push(("TMPDIR".into(), tmpdir));

    sbpl.push_str("; Workspace access\n");
    sbpl.push_str("(define workspace-dir (param \"WORKSPACE_DIR\"))\n");

    match config.security_level {
        SecurityLevel::L0Deny => {
            sbpl.push_str("(allow file-read* (subpath workspace-dir))\n");
        }
        SecurityLevel::L1Allowlist | SecurityLevel::L2Sandboxed => {
            sbpl.push_str("(allow file-read* file-write* (subpath workspace-dir))\n");
        }
    }
    sbpl.push('\n');

    // ── Additional mounts ──────────────────────────────────────────────────
    // F-04 fix: Use SBPL parameters for mount paths (consistent with workspace)
    // to prevent SBPL injection via specially crafted path names.
    if !config.mounts.is_empty() {
        sbpl.push_str("; Additional mounts\n");
        for (i, mount) in config.mounts.iter().enumerate() {
            let canonical_mount =
                mount
                    .host_path
                    .canonicalize()
                    .map_err(|e| SandboxError::PathError {
                        path: mount.host_path.clone(),
                        reason: format!("failed to canonicalize mount path: {e}"),
                    })?;
            let path = canonical_mount
                .to_str()
                .ok_or_else(|| SandboxError::PathError {
                    path: mount.host_path.clone(),
                    reason: "mount path is not valid UTF-8".into(),
                })?;
            let param_name = format!("MOUNT_{i}");
            params.push((param_name.clone(), path.into()));
            let _ = writeln!(sbpl, "(define mount-{i} (param \"{param_name}\"))");
            match mount.mode {
                MountMode::ReadOnly => {
                    let _ = writeln!(sbpl, "(allow file-read* (subpath mount-{i}))");
                }
                MountMode::ReadWrite => {
                    let _ = writeln!(sbpl, "(allow file-read* file-write* (subpath mount-{i}))");
                }
            }
        }
        sbpl.push('\n');
    }

    // ── Network policy ─────────────────────────────────────────────────────
    emit_network_rules(&mut sbpl, config.effective_network_policy());

    // ── Process execution ──────────────────────────────────────────────────
    emit_process_rules(&mut sbpl);

    // ── System paths ───────────────────────────────────────────────────────
    emit_system_paths(&mut sbpl);

    // ── Temp directories ───────────────────────────────────────────────────
    emit_temp_dirs(&mut sbpl);

    // ── Device access ──────────────────────────────────────────────────────
    emit_device_access(&mut sbpl);

    // ── Mach services ──────────────────────────────────────────────────────
    emit_mach_services(&mut sbpl);

    // ── Sysctl access ──────────────────────────────────────────────────────
    emit_sysctl_access(&mut sbpl);

    Ok(SandboxProfile { sbpl, params })
}

/// Escape a string for embedding in SBPL quoted strings.
/// Handles backslashes and double quotes.
#[cfg(test)]
fn escape_sbpl_string(s: &str) -> String {
    s.replace('\\', "\\\\").replace('"', "\\\"")
}

/// Emit network rules based on the effective network policy.
fn emit_network_rules(sbpl: &mut String, policy: NetworkPolicy) {
    sbpl.push_str("; Network policy\n");
    match policy {
        NetworkPolicy::None => {
            // (deny default) already blocks everything; explicit for clarity
            sbpl.push_str("(deny network*)\n");
        }
        NetworkPolicy::Restricted => {
            // Allow outbound TCP to public internet
            sbpl.push_str("(allow network-outbound (remote tcp))\n");
            // DNS resolution (UDP port 53)
            sbpl.push_str("(allow network-outbound (remote udp \"*:53\"))\n");
            // Deny localhost — SBPL only supports `*` or `localhost` as host
            // (CIDR ranges like 127.0.0.0/8 are NOT supported by Seatbelt)
            sbpl.push_str("(deny network-outbound (remote tcp \"localhost:*\"))\n");
            // Block Unix domain sockets to prevent proxy bypass
            // (ref: Claude Code sandbox-runtime approach)
            sbpl.push_str("(deny network* (local unix-socket))\n");
            // NOTE: LAN addresses (10.x, 172.16.x, 192.168.x) cannot be blocked
            // via SBPL alone — Seatbelt network filters operate on hostname/port,
            // not on IP ranges. Full LAN blocking requires Network Extension or
            // a proxy-based approach (Phase 6 enhancement).
        }
        NetworkPolicy::Host => {
            sbpl.push_str("(allow network*)\n");
        }
    }
    sbpl.push('\n');
}

/// Emit process execution rules.
///
/// We allow broad process execution because the sandbox's job is resource isolation,
/// not binary whitelisting. The CLI layer controls which commands are allowed.
fn emit_process_rules(sbpl: &mut String) {
    sbpl.push_str("; Process execution\n");
    sbpl.push_str("(allow process-exec)\n");
    sbpl.push_str("(allow process-fork)\n");
    sbpl.push_str("(allow signal (target self))\n\n");
}

/// Emit system path read access for dynamic linking and interpreter support.
///
/// `bsd.sb` covers basic system libraries, but interpreters (Python, Node, Ruby)
/// and Homebrew tools need additional paths.
fn emit_system_paths(sbpl: &mut String) {
    sbpl.push_str("; System paths for dynamic linking and interpreter support\n");
    let read_paths = [
        "/usr/lib",
        "/usr/share",
        "/usr/local",
        "/opt/homebrew",
        "/System/Library",
        "/Library/Frameworks",
        "/Library/Apple",
        "/private/var/db", // dyld shared cache metadata
    ];
    for path in &read_paths {
        let _ = writeln!(sbpl, "(allow file-read* (subpath \"{path}\"))");
    }

    // Executable search paths
    let exec_paths = ["/bin", "/usr/bin", "/usr/local/bin", "/opt/homebrew/bin"];
    for path in &exec_paths {
        let _ = writeln!(sbpl, "(allow file-read* (subpath \"{path}\"))");
    }
    sbpl.push('\n');
}

/// Emit temp directory access.
fn emit_temp_dirs(sbpl: &mut String) {
    sbpl.push_str("; Temporary directories\n");
    sbpl.push_str("(allow file-read* file-write* (subpath \"/tmp\"))\n");
    sbpl.push_str("(allow file-read* file-write* (subpath \"/private/tmp\"))\n");
    // macOS per-user temp dir
    sbpl.push_str("(allow file-read* file-write* (subpath (param \"TMPDIR\")))\n");
    sbpl.push('\n');
}

/// Emit device file access.
fn emit_device_access(sbpl: &mut String) {
    sbpl.push_str("; Device access\n");
    let devices = [
        "/dev/null",
        "/dev/zero",
        "/dev/urandom",
        "/dev/random",
        "/dev/stdin",
        "/dev/stdout",
        "/dev/stderr",
        "/dev/fd",
        "/dev/tty",
        "/dev/dtracehelper",
    ];
    for dev in &devices {
        let _ = writeln!(sbpl, "(allow file-read* file-write* (literal \"{dev}\"))");
    }
    // /dev/fd/* needs subpath access for file descriptor operations
    sbpl.push_str("(allow file-read* file-write* (subpath \"/dev/fd\"))\n\n");
}

/// Emit Mach service lookups required by many macOS programs.
fn emit_mach_services(sbpl: &mut String) {
    sbpl.push_str("; Required Mach services\n");
    let services = [
        "com.apple.system.logger",
        "com.apple.system.notification_center",
        "com.apple.CoreServices.coreservicesd",
        "com.apple.SecurityServer",
        "com.apple.system.opendirectoryd.libinfo",
    ];
    for svc in &services {
        let _ = writeln!(sbpl, "(allow mach-lookup (global-name \"{svc}\"))");
    }
    sbpl.push('\n');
}

/// Emit sysctl read access needed by runtime introspection.
fn emit_sysctl_access(sbpl: &mut String) {
    sbpl.push_str("; Sysctl access (runtime introspection)\n");
    sbpl.push_str("(allow sysctl-read)\n\n");
}

#[cfg(test)]
#[allow(clippy::unwrap_used)]
mod tests {
    use super::*;
    use crate::config::{BackendPreference, OutputFormat, ResourceLimits};

    /// Create a test config using a real temp directory (required for canonicalization).
    fn test_config(
        security_level: SecurityLevel,
        network: Option<NetworkPolicy>,
    ) -> (SandboxConfig, tempfile::TempDir) {
        #[allow(clippy::expect_used)]
        let tmpdir = tempfile::tempdir().expect("failed to create temp dir");
        let config = SandboxConfig {
            security_level,
            command: "/usr/bin/echo".into(),
            args: vec!["hello".into()],
            workspace: tmpdir.path().to_path_buf(),
            mounts: vec![],
            resource_limits: ResourceLimits::default(),
            network_policy: network,
            env_vars: std::collections::HashMap::new(),
            format: OutputFormat::Json,
            backend: BackendPreference::Native,
        };
        (config, tmpdir)
    }

    #[test]
    fn profile_l0_deny_has_readonly_workspace() {
        let (config, _td) = test_config(SecurityLevel::L0Deny, None);
        let profile = generate_profile(&config).unwrap();
        // Workspace is read-only (no file-write* on workspace-dir)
        assert!(
            profile
                .sbpl
                .contains("(allow file-read* (subpath workspace-dir))")
        );
        assert!(
            !profile
                .sbpl
                .contains("(allow file-read* file-write* (subpath workspace-dir))")
        );
        // L0 default network is None
        assert!(profile.sbpl.contains("(deny network*)"));
    }

    #[test]
    fn profile_l1_sandbox_has_readwrite_workspace() {
        let (config, _td) = test_config(SecurityLevel::L1Allowlist, None);
        let profile = generate_profile(&config).unwrap();
        assert!(profile.sbpl.contains("file-write*"));
        // L1 default network is Restricted
        assert!(
            profile
                .sbpl
                .contains("(allow network-outbound (remote tcp))")
        );
        assert!(
            profile
                .sbpl
                .contains("(deny network-outbound (remote tcp \"localhost:*\"))")
        );
    }

    #[test]
    fn profile_host_network_allows_all() {
        let (config, _td) = test_config(SecurityLevel::L2Sandboxed, Some(NetworkPolicy::Host));
        let profile = generate_profile(&config).unwrap();
        assert!(profile.sbpl.contains("(allow network*)"));
    }

    #[test]
    fn profile_has_required_header() {
        let (config, _td) = test_config(SecurityLevel::L1Allowlist, None);
        let profile = generate_profile(&config).unwrap();
        assert!(
            profile
                .sbpl
                .starts_with("(version 1)\n(deny default)\n(import \"bsd.sb\")\n")
        );
    }

    #[test]
    fn profile_params_contain_workspace() {
        let (config, td) = test_config(SecurityLevel::L1Allowlist, None);
        let profile = generate_profile(&config).unwrap();
        // Workspace param should be the canonicalized path
        let canonical = td.path().canonicalize().unwrap();
        let canonical_str = canonical.to_str().unwrap();
        assert!(
            profile
                .params
                .iter()
                .any(|(k, v)| k == "WORKSPACE_DIR" && v == canonical_str)
        );
    }

    #[test]
    fn escape_handles_special_chars() {
        assert_eq!(
            escape_sbpl_string(r#"path\with"quotes"#),
            r#"path\\with\"quotes"#
        );
    }
}
