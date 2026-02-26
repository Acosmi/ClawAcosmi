//! Landlock LSM filesystem isolation.
//!
//! Uses the `landlock` crate (official Rust bindings by Mickaël Salaün) to
//! restrict filesystem access for the sandboxed process. Landlock is unprivileged
//! and available since kernel 5.13 (ABI V1).
//!
//! # ABI compatibility
//!
//! We target ABI V4 (kernel 6.7, adds TCP network filtering) but use best-effort
//! mode: unsupported access rights are silently dropped on older kernels.
//!
//! # How it works
//!
//! 1. Declare handled access rights (anything not handled is unrestricted)
//! 2. Add allow-rules for specific paths/ports
//! 3. Call `restrict_self()` — irrevocable, inherited by exec'd processes

use std::path::Path;

use landlock::{
    ABI, Access, AccessFs, PathBeneath, PathFd, Ruleset, RulesetAttr, RulesetCreatedAttr,
    RulesetStatus,
};
use tracing::{debug, info, warn};

use crate::config::{MountMode, SandboxConfig, SecurityLevel};
use crate::error::SandboxError;

/// Maximum ABI version we generate rules for.
/// Best-effort compatibility silently drops unsupported rights on older kernels.
const TARGET_ABI: ABI = ABI::V4;

/// System paths allowed as read-only inside the sandbox.
const SYSTEM_READ_PATHS: &[&str] = &[
    "/usr",   // libraries, binaries, data
    "/bin",   // essential binaries (may → /usr/bin symlink)
    "/lib",   // essential shared libraries (may → /usr/lib symlink)
    "/lib64", // 64-bit shared libraries
    "/sbin",  // system binaries
    "/etc",   // configuration files
    "/run",   // runtime data (systemd, dbus sockets)
];

/// S-04 audit fix: Scoped /proc paths (read-only) instead of full /proc.
/// Full /proc exposes other processes' info — limit to own process + essential files.
const PROC_READ_PATHS: &[&str] = &[
    "/proc/self",        // own process info — needed by most programs
    "/proc/thread-self", // own thread info
];

/// S-04 audit fix: Individual /proc files needed by many programs.
const PROC_READ_FILES: &[&str] = &[
    "/proc/meminfo",
    "/proc/cpuinfo",
    "/proc/stat",
    "/proc/filesystems",
    "/proc/version",
    "/proc/loadavg",
    "/proc/uptime",
];

/// S-04 audit fix: Scoped /sys paths (read-only) instead of full /sys.
/// Full /sys exposes all kernel/hardware info — limit to essential paths.
const SYS_READ_PATHS: &[&str] = &[
    "/sys/devices/system/cpu", // CPU topology — needed by runtime detection
];

/// S-04 audit fix: Specific device files instead of full /dev.
/// Full /dev access exposes all devices; sandbox only needs common ones.
const DEV_PATHS: &[&str] = &[
    "/dev/null",
    "/dev/zero",
    "/dev/urandom",
    "/dev/random",
    "/dev/fd", // file descriptor directory
    "/dev/stdin",
    "/dev/stdout",
    "/dev/stderr",
    "/dev/tty",
    "/dev/shm", // shared memory — needed by some runtimes
];

/// Temp directories allowed as read-write.
const TEMP_PATHS: &[&str] = &["/tmp", "/var/tmp"];

/// Apply Landlock filesystem isolation rules to the current process.
///
/// After this call, the process (and any exec'd children) can only access
/// paths explicitly allowed by the rules. This is irrevocable.
///
/// # Rule mapping
///
/// | Config | Landlock rule |
/// |--------|---------------|
/// | Workspace (L0) | Read-only |
/// | Workspace (L1/L2) | Read-write |
/// | System paths | Read-only |
/// | Temp dirs | Read-write |
/// | Additional mounts | Per `MountMode` |
pub fn apply_landlock_rules(config: &SandboxConfig) -> Result<(), SandboxError> {
    let abi = TARGET_ABI;

    // Create ruleset — handles all filesystem access rights for this ABI.
    // Anything not explicitly allowed will be denied.
    let mut ruleset = Ruleset::default()
        .handle_access(AccessFs::from_all(abi))
        .map_err(|e| landlock_err("handle_access(fs)", e))?
        .create()
        .map_err(|e| landlock_err("create_ruleset", e))?;

    // ── Workspace access ──────────────────────────────────────────────────
    let workspace_access = match config.security_level {
        SecurityLevel::L0Deny => AccessFs::from_read(abi),
        SecurityLevel::L1Sandbox | SecurityLevel::L2Full => AccessFs::from_all(abi),
    };
    add_path_rule(
        &mut ruleset,
        &config.workspace,
        workspace_access,
        "workspace",
    )?;

    // ── System paths (read-only) ──────────────────────────────────────────
    for path in SYSTEM_READ_PATHS {
        if Path::new(path).exists() {
            add_path_rule(&mut ruleset, path, AccessFs::from_read(abi), path)?;
        }
    }

    // S-04 audit fix: Scoped /proc access (not full /proc)
    for path in PROC_READ_PATHS {
        if Path::new(path).exists() {
            add_path_rule(&mut ruleset, path, AccessFs::from_read(abi), path)?;
        }
    }
    for path in PROC_READ_FILES {
        if Path::new(path).exists() {
            add_path_rule(&mut ruleset, path, AccessFs::from_read(abi), path)?;
        }
    }

    // S-04 audit fix: Scoped /sys access (not full /sys)
    for path in SYS_READ_PATHS {
        if Path::new(path).exists() {
            add_path_rule(&mut ruleset, path, AccessFs::from_read(abi), path)?;
        }
    }

    // S-04 audit fix: Scoped /dev access (specific device files only)
    for path in DEV_PATHS {
        if Path::new(path).exists() {
            add_path_rule(&mut ruleset, path, AccessFs::from_all(abi), path)?;
        }
    }

    // ── Temp directories (read-write) ─────────────────────────────────────
    for path in TEMP_PATHS {
        if Path::new(path).exists() {
            add_path_rule(&mut ruleset, path, AccessFs::from_all(abi), path)?;
        }
    }
    // Per-user TMPDIR (e.g., /run/user/<uid>)
    if let Ok(tmpdir) = std::env::var("TMPDIR") {
        if Path::new(&tmpdir).exists() {
            add_path_rule(&mut ruleset, &tmpdir, AccessFs::from_all(abi), "TMPDIR")?;
        }
    }

    // ── Additional mounts ─────────────────────────────────────────────────
    for mount in &config.mounts {
        let access = match mount.mode {
            MountMode::ReadOnly => AccessFs::from_read(abi),
            MountMode::ReadWrite => AccessFs::from_all(abi),
        };
        let label = mount.host_path.display().to_string();
        add_path_rule(&mut ruleset, &mount.host_path, access, &label)?;
    }

    // ── Restrict self ─────────────────────────────────────────────────────
    let status = ruleset
        .restrict_self()
        .map_err(|e| landlock_err("restrict_self", e))?;

    match status.ruleset {
        RulesetStatus::FullyEnforced => {
            info!("landlock fully enforced");
        }
        RulesetStatus::PartiallyCompatible => {
            warn!("landlock partially compatible — some rules dropped (older kernel ABI)");
        }
        RulesetStatus::NotEnforced => {
            return Err(SandboxError::Landlock {
                operation: "restrict_self".into(),
                source: std::io::Error::other("landlock ruleset not enforced by kernel"),
            });
        }
    }

    Ok(())
}

/// Add a path-based allow rule to the Landlock ruleset.
fn add_path_rule<A: Into<landlock::BitFlags<AccessFs>>>(
    ruleset: &mut landlock::RulesetCreated,
    path: impl AsRef<Path>,
    access: A,
    label: &str,
) -> Result<(), SandboxError> {
    let path_ref = path.as_ref();
    let fd = PathFd::new(path_ref).map_err(|e| SandboxError::Landlock {
        operation: format!("open PathFd for {label}"),
        source: std::io::Error::other(e),
    })?;
    ruleset
        .add_rule(PathBeneath::new(fd, access))
        .map_err(|e| landlock_err(&format!("add_rule for {label}"), e))?;
    debug!(path = %path_ref.display(), "landlock: added path rule");
    Ok(())
}

/// Convert a landlock error to our `SandboxError::Landlock` variant.
fn landlock_err(
    operation: &str,
    err: impl std::error::Error + Send + Sync + 'static,
) -> SandboxError {
    SandboxError::Landlock {
        operation: operation.into(),
        source: std::io::Error::other(err),
    }
}
