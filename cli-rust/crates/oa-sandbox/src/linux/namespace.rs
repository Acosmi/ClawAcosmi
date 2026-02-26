//! Linux namespace isolation (optional enhancement layer).
//!
//! When unprivileged user namespaces are available, this module provides
//! additional isolation via User NS + Mount NS. This is an enhancement on top
//! of the mandatory Landlock + Seccomp base layer.
//!
//! # Why optional
//!
//! Ubuntu 24.04+ blocks unprivileged user namespaces via AppArmor by default.
//! The sandbox must work without namespaces (Landlock+Seccomp only).
//!
//! # Namespace layering
//!
//! ```text
//! unshare(CLONE_NEWUSER)  →  UID/GID mapping  →  unshare(CLONE_NEWNS)  →  mount setup
//! ```
//!
//! PID namespace is NOT used in this phase because it requires double-fork
//! and a PID 1 init process. This is deferred to a future enhancement.

use std::path::Path;

use nix::sched::CloneFlags;
use nix::unistd::{Gid, Uid};
use tracing::{debug, info};

use crate::config::{MountMode, SandboxConfig, SecurityLevel};
use crate::error::SandboxError;

/// Apply User Namespace isolation.
///
/// Creates a new user namespace where the current UID/GID are mapped to
/// unprivileged IDs (1000:1000) inside the namespace. This provides:
/// - Process appears to run as a different user
/// - Capability set is isolated (full caps inside NS, no caps outside)
///
/// Must be called before `apply_mount_namespace()`.
pub fn apply_user_namespace() -> Result<(), SandboxError> {
    let real_uid = Uid::current();
    let real_gid = Gid::current();

    // Create user namespace
    nix::sched::unshare(CloneFlags::CLONE_NEWUSER).map_err(|e| SandboxError::Namespace {
        operation: "unshare(CLONE_NEWUSER)".into(),
        source: e.into(),
    })?;

    // Write UID mapping: map container UID 1000 → host real UID
    // Format: <inside_uid> <outside_uid> <count>
    let uid_map = format!("1000 {} 1", real_uid);
    std::fs::write("/proc/self/uid_map", uid_map.as_bytes()).map_err(|e| {
        SandboxError::Namespace {
            operation: "write /proc/self/uid_map".into(),
            source: e,
        }
    })?;

    // Must write "deny" to setgroups before writing gid_map (kernel requirement)
    std::fs::write("/proc/self/setgroups", b"deny").map_err(|e| SandboxError::Namespace {
        operation: "write /proc/self/setgroups".into(),
        source: e,
    })?;

    // Write GID mapping
    let gid_map = format!("1000 {} 1", real_gid);
    std::fs::write("/proc/self/gid_map", gid_map.as_bytes()).map_err(|e| {
        SandboxError::Namespace {
            operation: "write /proc/self/gid_map".into(),
            source: e,
        }
    })?;

    info!(
        host_uid = real_uid.as_raw(),
        host_gid = real_gid.as_raw(),
        "user namespace created (mapped to 1000:1000)"
    );

    Ok(())
}

/// Apply Mount Namespace isolation.
///
/// Creates a new mount namespace to isolate the filesystem view.
/// Remounts `/` as private (prevents mount propagation to host), then
/// optionally sets up bind mounts for the workspace and additional paths.
///
/// Must be called AFTER `apply_user_namespace()` (needs capabilities from user NS).
pub fn apply_mount_namespace(config: &SandboxConfig) -> Result<(), SandboxError> {
    // Create mount namespace
    nix::sched::unshare(CloneFlags::CLONE_NEWNS).map_err(|e| SandboxError::Namespace {
        operation: "unshare(CLONE_NEWNS)".into(),
        source: e.into(),
    })?;

    // Make all mounts private — prevent propagation to/from host.
    // This is essential: without it, unmounts inside the sandbox affect the host.
    nix::mount::mount(
        None::<&str>,
        "/",
        None::<&str>,
        nix::mount::MsFlags::MS_PRIVATE | nix::mount::MsFlags::MS_REC,
        None::<&str>,
    )
    .map_err(|e| SandboxError::Namespace {
        operation: "mount --make-rprivate /".into(),
        source: e.into(),
    })?;

    debug!("mount namespace created, / remounted as private");

    // ── Bind mount additional paths ───────────────────────────────────────
    // Note: In the Landlock+Seccomp-only path, bind mounts are not used
    // (Landlock handles path access control). Bind mounts provide additional
    // isolation in the namespace path by hiding the rest of the filesystem.
    for mount in &config.mounts {
        if !mount.host_path.exists() {
            debug!(path = %mount.host_path.display(), "skipping non-existent mount");
            continue;
        }

        // Ensure sandbox_path exists
        if mount.sandbox_path.is_dir() || mount.host_path.is_dir() {
            std::fs::create_dir_all(&mount.sandbox_path).map_err(|e| SandboxError::Namespace {
                operation: format!(
                    "mkdir for bind mount target {}",
                    mount.sandbox_path.display()
                ),
                source: e,
            })?;
        }

        // Bind mount
        nix::mount::mount(
            Some(mount.host_path.as_path()),
            mount.sandbox_path.as_path(),
            None::<&str>,
            nix::mount::MsFlags::MS_BIND | nix::mount::MsFlags::MS_REC,
            None::<&str>,
        )
        .map_err(|e| SandboxError::Namespace {
            operation: format!(
                "bind mount {} → {}",
                mount.host_path.display(),
                mount.sandbox_path.display()
            ),
            source: e.into(),
        })?;

        // Remount read-only if needed
        if mount.mode == MountMode::ReadOnly {
            nix::mount::mount(
                None::<&str>,
                mount.sandbox_path.as_path(),
                None::<&str>,
                nix::mount::MsFlags::MS_BIND
                    | nix::mount::MsFlags::MS_REMOUNT
                    | nix::mount::MsFlags::MS_RDONLY,
                None::<&str>,
            )
            .map_err(|e| SandboxError::Namespace {
                operation: format!("remount read-only {}", mount.sandbox_path.display()),
                source: e.into(),
            })?;
        }

        debug!(
            src = %mount.host_path.display(),
            dst = %mount.sandbox_path.display(),
            mode = ?mount.mode,
            "bind mount applied"
        );
    }

    info!(
        mount_count = config.mounts.len(),
        "mount namespace configured"
    );

    Ok(())
}

/// Check if user namespace creation would likely succeed.
///
/// This performs a cheap check without actually creating a namespace.
/// The definitive test is in `platform.rs::detect_user_namespace()`.
pub fn user_namespace_likely_available() -> bool {
    // Quick check: /proc/self/ns/user should exist
    Path::new("/proc/self/ns/user").exists()
}
