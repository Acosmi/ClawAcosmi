//! Windows Job Object management.
//!
//! Provides process tree resource limits and automatic cleanup via
//! `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` — when the last handle to the
//! Job Object is closed, all associated processes are terminated.
//!
//! # Resource limits
//!
//! | Limit | Win32 flag | Config field |
//! |-------|-----------|--------------|
//! | Memory | `JOB_OBJECT_LIMIT_JOB_MEMORY` | `memory_bytes` |
//! | CPU | `JOBOBJECT_CPU_RATE_CONTROL_INFORMATION` | `cpu_millicores` |
//! | PIDs | `JOB_OBJECT_LIMIT_ACTIVE_PROCESS` | `max_pids` |
//! | Kill-on-close | `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` | always on |

use tracing::{debug, info};

use windows::Win32::Foundation::{CloseHandle, HANDLE};
use windows::Win32::System::JobObjects::{
    AssignProcessToJobObject, CreateJobObjectW, JOB_OBJECT_CPU_RATE_CONTROL_ENABLE,
    JOB_OBJECT_CPU_RATE_CONTROL_HARD_CAP, JOB_OBJECT_LIMIT_ACTIVE_PROCESS,
    JOB_OBJECT_LIMIT_JOB_MEMORY, JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
    JOBOBJECT_BASIC_LIMIT_INFORMATION, JOBOBJECT_CPU_RATE_CONTROL_INFORMATION,
    JOBOBJECT_EXTENDED_LIMIT_INFORMATION, JobObjectBasicLimitInformation,
    JobObjectCpuRateControlInformation, JobObjectExtendedLimitInformation, SetInformationJobObject,
};

use crate::config::ResourceLimits;
use crate::error::SandboxError;

/// RAII guard for a Windows Job Object.
///
/// On drop, closes the Job Object handle. Because `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`
/// is always set, this terminates all processes assigned to the job.
pub struct JobGuard {
    handle: HANDLE,
}

impl JobGuard {
    /// Get the raw Job Object handle (for `AssignProcessToJobObject`).
    pub fn handle(&self) -> HANDLE {
        self.handle
    }

    /// Assign a process to this Job Object.
    ///
    /// The process must not already be assigned to another job (on Windows 7).
    /// Windows 8+ supports nested jobs.
    pub fn assign_process(&self, process_handle: HANDLE) -> Result<(), SandboxError> {
        // SAFETY: Both handles are valid — job from create_job_object(),
        // process from CreateProcessAsUserW. AssignProcessToJobObject is
        // safe to call with valid handles.
        unsafe {
            AssignProcessToJobObject(self.handle, process_handle).map_err(|e| {
                SandboxError::Win32 {
                    operation: "AssignProcessToJobObject".into(),
                    error_code: e.code().0 as u32,
                }
            })?;
        }
        debug!("process assigned to job object");
        Ok(())
    }
}

impl Drop for JobGuard {
    fn drop(&mut self) {
        // SAFETY: self.handle is a valid Job Object handle created by CreateJobObjectW.
        // CloseHandle is always safe for handles we own. Because KILL_ON_JOB_CLOSE is set,
        // this will terminate all processes in the job when this is the last handle.
        unsafe {
            let _ = CloseHandle(self.handle);
        }
        debug!("job object handle closed (kill-on-close active)");
    }
}

// SAFETY: HANDLE is a raw pointer but Job Object handles are thread-safe.
// The Win32 API guarantees thread-safe access to Job Object handles.
unsafe impl Send for JobGuard {}
unsafe impl Sync for JobGuard {}

/// Create a Job Object with resource limits and kill-on-close behavior.
///
/// The returned [`JobGuard`] owns the handle. When dropped, all assigned
/// processes are terminated (KILL_ON_JOB_CLOSE).
pub fn create_job_object(limits: &ResourceLimits) -> Result<JobGuard, SandboxError> {
    // SAFETY: CreateJobObjectW with NULL security attributes and no name
    // creates an anonymous Job Object. This is safe — no external state affected.
    let handle = unsafe {
        CreateJobObjectW(None, None).map_err(|e| SandboxError::Win32 {
            operation: "CreateJobObjectW".into(),
            error_code: e.code().0 as u32,
        })?
    };

    // ── Extended limits: memory + kill-on-close + active process count ────
    let mut ext_info = JOBOBJECT_EXTENDED_LIMIT_INFORMATION::default();
    let basic = &mut ext_info.BasicLimitInformation;

    // Always set KILL_ON_JOB_CLOSE — core safety guarantee
    basic.LimitFlags |= JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;

    // Memory limit (per-job)
    if limits.memory_bytes > 0 {
        basic.LimitFlags |= JOB_OBJECT_LIMIT_JOB_MEMORY;
        ext_info.JobMemoryLimit = limits.memory_bytes as usize;
        debug!(bytes = limits.memory_bytes, "job memory limit set");
    }

    // Active process limit
    if limits.max_pids > 0 {
        basic.LimitFlags |= JOB_OBJECT_LIMIT_ACTIVE_PROCESS;
        basic.ActiveProcessLimit = limits.max_pids;
        debug!(max_pids = limits.max_pids, "active process limit set");
    }

    // SAFETY: handle is valid (just created above). We pass a properly initialized
    // JOBOBJECT_EXTENDED_LIMIT_INFORMATION struct with correct size.
    unsafe {
        SetInformationJobObject(
            handle,
            JobObjectExtendedLimitInformation,
            std::ptr::from_ref(&ext_info).cast(),
            u32::try_from(std::mem::size_of::<JOBOBJECT_EXTENDED_LIMIT_INFORMATION>())
                .unwrap_or(u32::MAX),
        )
        .map_err(|e| SandboxError::Win32 {
            operation: "SetInformationJobObject(ExtendedLimitInformation)".into(),
            error_code: e.code().0 as u32,
        })?;
    }

    // ── CPU rate control ─────────────────────────────────────────────────
    if limits.cpu_millicores > 0 {
        // Convert millicores to Windows CPU rate percentage * 100.
        // 1000 millicores = 1 full core = 10000 (100% * 100).
        // Windows range: 1 to 10000.
        let rate = (u32::from(limits.cpu_millicores.min(1000)) * 10000 / 1000).max(1);

        let cpu_info = JOBOBJECT_CPU_RATE_CONTROL_INFORMATION {
            ControlFlags: JOB_OBJECT_CPU_RATE_CONTROL_ENABLE | JOB_OBJECT_CPU_RATE_CONTROL_HARD_CAP,
            Anonymous:
                windows::Win32::System::JobObjects::JOBOBJECT_CPU_RATE_CONTROL_INFORMATION_0 {
                    CpuRate: rate,
                },
        };

        // SAFETY: handle is valid. cpu_info is properly initialized with valid flags and rate.
        unsafe {
            SetInformationJobObject(
                handle,
                JobObjectCpuRateControlInformation,
                std::ptr::from_ref(&cpu_info).cast(),
                u32::try_from(std::mem::size_of::<JOBOBJECT_CPU_RATE_CONTROL_INFORMATION>())
                    .unwrap_or(u32::MAX),
            )
            .map_err(|e| SandboxError::Win32 {
                operation: "SetInformationJobObject(CpuRateControl)".into(),
                error_code: e.code().0 as u32,
            })?;
        }

        debug!(
            millicores = limits.cpu_millicores,
            rate, "CPU rate control set (hard cap)"
        );
    }

    info!(
        memory = limits.memory_bytes,
        cpu = limits.cpu_millicores,
        pids = limits.max_pids,
        "job object created with resource limits"
    );

    Ok(JobGuard { handle })
}
