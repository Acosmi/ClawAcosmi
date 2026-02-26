//! Cgroups v2 resource limits (optional).
//!
//! Applies memory, CPU, and PID limits via the cgroups v2 unified hierarchy.
//! Requires systemd delegation — the current user must have write access to
//! a cgroup subtree.
//!
//! # Approach
//!
//! 1. Try to find a user-delegated cgroup (via systemd `user.slice`)
//! 2. Create a child cgroup for the sandbox
//! 3. Write resource limits (`memory.max`, `cpu.max`, `pids.max`)
//! 4. Return a `CgroupGuard` that removes the cgroup on drop
//!
//! If delegation is not available (no systemd, non-standard setup),
//! resource limits are silently skipped with a warning.

use std::path::{Path, PathBuf};

use tracing::{debug, info, warn};

use crate::config::{ResourceLimits, SandboxConfig};
use crate::error::SandboxError;

/// RAII guard that removes the cgroup directory on drop.
pub struct CgroupGuard {
    cgroup_path: PathBuf,
}

impl CgroupGuard {
    /// Get the cgroup path (for `cgroup.procs` writes).
    pub fn path(&self) -> &Path {
        &self.cgroup_path
    }

    /// Write the current process's PID into the cgroup's `cgroup.procs`.
    pub fn add_self(&self) -> Result<(), SandboxError> {
        let procs_path = self.cgroup_path.join("cgroup.procs");
        let pid = std::process::id();
        std::fs::write(&procs_path, pid.to_string().as_bytes()).map_err(|e| SandboxError::Io {
            context: format!("write PID {pid} to {}", procs_path.display()),
            source: e,
        })?;
        debug!(pid, cgroup = %self.cgroup_path.display(), "added process to cgroup");
        Ok(())
    }
}

impl Drop for CgroupGuard {
    fn drop(&mut self) {
        // Remove the cgroup directory. This will fail if processes are still in it,
        // which is expected (kernel cleans up when last process exits).
        if let Err(e) = std::fs::remove_dir(&self.cgroup_path) {
            debug!(
                path = %self.cgroup_path.display(),
                error = %e,
                "could not remove cgroup dir (processes may still be running)"
            );
        }
    }
}

/// Set up cgroup v2 resource limits for the sandbox.
///
/// Returns `Ok(Some(guard))` if a cgroup was created, or `Ok(None)` if
/// cgroup delegation is not available (resource limits silently skipped).
///
/// The caller should use `guard.add_self()` to move the child process
/// into the cgroup before exec.
pub fn setup_cgroup_limits(config: &SandboxConfig) -> Result<Option<CgroupGuard>, SandboxError> {
    let limits = &config.resource_limits;

    // Skip if no resource limits are configured (besides timeout)
    if limits.memory_bytes == 0 && limits.cpu_millicores == 0 && limits.max_pids == 0 {
        debug!("no cgroup resource limits configured, skipping");
        return Ok(None);
    }

    // Find the user's delegated cgroup
    let user_cgroup = match find_user_cgroup() {
        Some(path) => path,
        None => {
            warn!("cgroup v2 delegation not available — resource limits will not be enforced");
            return Ok(None);
        }
    };

    // Create a unique child cgroup for this sandbox instance
    let sandbox_cgroup_name = format!("oa-sandbox-{}", std::process::id());
    let cgroup_path = user_cgroup.join(&sandbox_cgroup_name);

    std::fs::create_dir_all(&cgroup_path).map_err(|e| SandboxError::Io {
        context: format!("create cgroup dir {}", cgroup_path.display()),
        source: e,
    })?;

    // Write resource limits
    write_limits(&cgroup_path, limits)?;

    info!(
        cgroup = %cgroup_path.display(),
        memory_bytes = limits.memory_bytes,
        cpu_millicores = limits.cpu_millicores,
        max_pids = limits.max_pids,
        "cgroup resource limits configured"
    );

    Ok(Some(CgroupGuard { cgroup_path }))
}

/// Find the current user's delegated cgroup path.
///
/// Looks for systemd's user slice: `/sys/fs/cgroup/user.slice/user-{UID}.slice/`
/// Returns `None` if cgroups v2 is not available or delegation is not set up.
fn find_user_cgroup() -> Option<PathBuf> {
    // Check if cgroups v2 unified hierarchy is mounted
    if !Path::new("/sys/fs/cgroup/cgroup.controllers").exists() {
        debug!("cgroups v2 unified hierarchy not found");
        return None;
    }

    // Check for user-level delegation via systemd
    // SAFETY: getuid() is always safe to call
    let uid = unsafe { libc::getuid() };
    let user_slice = PathBuf::from(format!(
        "/sys/fs/cgroup/user.slice/user-{uid}.slice/user@{uid}.service"
    ));

    if user_slice.is_dir() {
        debug!(path = %user_slice.display(), "found user-delegated cgroup");
        return Some(user_slice);
    }

    // Fallback: check the simpler path without user@.service
    let simple_slice = PathBuf::from(format!("/sys/fs/cgroup/user.slice/user-{uid}.slice"));
    if simple_slice.is_dir() {
        // Verify we have write access
        let test_path = simple_slice.join("cgroup.procs");
        if test_path.exists() {
            debug!(path = %simple_slice.display(), "found user cgroup slice");
            return Some(simple_slice);
        }
    }

    debug!("no user-delegated cgroup found");
    None
}

/// Write resource limits to the cgroup control files.
fn write_limits(cgroup_path: &Path, limits: &ResourceLimits) -> Result<(), SandboxError> {
    // memory.max — hard memory limit (OOM killer invoked when exceeded)
    if limits.memory_bytes > 0 {
        let memory_path = cgroup_path.join("memory.max");
        std::fs::write(&memory_path, limits.memory_bytes.to_string().as_bytes()).map_err(|e| {
            SandboxError::Io {
                context: format!("write memory.max to {}", memory_path.display()),
                source: e,
            }
        })?;
        debug!(bytes = limits.memory_bytes, "set memory.max");
    }

    // cpu.max — CPU bandwidth limit as "$MAX $PERIOD"
    // Period is 100000 µs (100ms). MAX = millicores * period / 1000
    if limits.cpu_millicores > 0 {
        let period: u64 = 100_000;
        let max = u64::from(limits.cpu_millicores) * period / 1000;
        let cpu_path = cgroup_path.join("cpu.max");
        std::fs::write(&cpu_path, format!("{max} {period}").as_bytes()).map_err(|e| {
            SandboxError::Io {
                context: format!("write cpu.max to {}", cpu_path.display()),
                source: e,
            }
        })?;
        debug!(
            millicores = limits.cpu_millicores,
            max, period, "set cpu.max"
        );
    }

    // pids.max — hard process count limit
    if limits.max_pids > 0 {
        let pids_path = cgroup_path.join("pids.max");
        std::fs::write(&pids_path, limits.max_pids.to_string().as_bytes()).map_err(|e| {
            SandboxError::Io {
                context: format!("write pids.max to {}", pids_path.display()),
                source: e,
            }
        })?;
        debug!(max_pids = limits.max_pids, "set pids.max");
    }

    Ok(())
}
