//! NTFS ACL management for workspace access control.
//!
//! Grants temporary file system access to the restricted sandbox process,
//! then revokes it on cleanup via RAII [`AclGuard`].
//!
//! # Workflow
//!
//! ```text
//! 1. GetNamedSecurityInfoW  →  get current DACL
//! 2. SetEntriesInAclW       →  add temporary ACE for sandbox SID
//! 3. SetNamedSecurityInfoW  →  apply modified DACL
//! 4. (on Drop)              →  restore original DACL
//! ```
//!
//! This ensures the restricted token process can access its workspace
//! without granting permanent filesystem permissions.

use std::path::{Path, PathBuf};

use tracing::{debug, info, warn};

use windows::Win32::Foundation::LocalFree;
use windows::Win32::Foundation::PSID;
use windows::Win32::Security::Authorization::{
    ACL as WIN_ACL, EXPLICIT_ACCESS_W, GRANT_ACCESS, GetNamedSecurityInfoW, NO_INHERITANCE,
    SE_FILE_OBJECT, SE_OBJECT_TYPE, SET_ACCESS, SUB_CONTAINERS_AND_OBJECTS_INHERIT,
    SetEntriesInAclW, SetNamedSecurityInfoW, TRUSTEE_IS_SID, TRUSTEE_IS_USER, TRUSTEE_W,
};
use windows::Win32::Security::{ACL as SEC_ACL, DACL_SECURITY_INFORMATION, PSECURITY_DESCRIPTOR};
use windows::Win32::Security::{CopySid, GetLengthSid, IsValidSid};

use crate::config::MountMode;
use crate::error::SandboxError;

/// File access rights for read-only access.
const READ_ONLY_ACCESS: u32 = 0x0012_0089; // FILE_GENERIC_READ | FILE_GENERIC_EXECUTE

/// File access rights for read-write access.
const READ_WRITE_ACCESS: u32 = 0x001F_01FF; // FILE_ALL_ACCESS

/// RAII guard that restores the original DACL when dropped.
///
/// This prevents permission persistence — the sandbox's workspace access
/// is revoked as soon as the guard goes out of scope.
pub struct AclGuard {
    path: PathBuf,
    original_dacl: *mut SEC_ACL,
    original_descriptor: PSECURITY_DESCRIPTOR,
}

impl AclGuard {
    /// Get the protected path.
    pub fn path(&self) -> &Path {
        &self.path
    }
}

impl Drop for AclGuard {
    fn drop(&mut self) {
        // Restore the original DACL
        let path_wide = to_wide_string(&self.path);

        // SAFETY: path_wide is a valid null-terminated wide string.
        // original_dacl points to the DACL from the original GetNamedSecurityInfoW call
        // (owned by original_descriptor). SetNamedSecurityInfoW restores it.
        let result = unsafe {
            SetNamedSecurityInfoW(
                windows::core::PCWSTR(path_wide.as_ptr()),
                SE_FILE_OBJECT,
                DACL_SECURITY_INFORMATION,
                None,
                None,
                Some(self.original_dacl),
                None,
            )
        };

        if let Err(e) = result {
            warn!(
                path = %self.path.display(),
                error = e.code().0,
                "failed to restore original DACL — permissions may persist"
            );
        } else {
            debug!(path = %self.path.display(), "original DACL restored");
        }

        // Free the security descriptor (which owns the original DACL)
        // SAFETY: original_descriptor was allocated by GetNamedSecurityInfoW.
        // LocalFree is the documented way to free it.
        if !self.original_descriptor.0.is_null() {
            unsafe {
                let _ = LocalFree(Some(std::mem::transmute(self.original_descriptor.0)));
            }
        }
    }
}

// SAFETY: The pointers in AclGuard are to heap memory allocated by Win32 APIs
// and are not shared with other threads. The guard is only used from the
// thread that created it (Drop runs on the same thread or on a single owner).
unsafe impl Send for AclGuard {}

/// Grant workspace access to a SID (Security Identifier).
///
/// Adds an ACE (Access Control Entry) to the DACL of `path` granting
/// either read-only or read-write access to `sid`. Returns an [`AclGuard`]
/// that restores the original DACL on drop.
///
/// # Arguments
///
/// * `path` — Directory to grant access to
/// * `sid` — SID of the restricted process (from the restricted token)
/// * `mode` — Read-only or read-write access
pub fn grant_workspace_access(
    path: &Path,
    sid: PSID,
    mode: MountMode,
) -> Result<AclGuard, SandboxError> {
    let path_wide = to_wide_string(path);

    // ── 1. Get current DACL ──────────────────────────────────────────────
    let mut dacl_ptr: *mut SEC_ACL = std::ptr::null_mut();
    let mut descriptor = PSECURITY_DESCRIPTOR::default();

    // SAFETY: path_wide is a valid null-terminated wide string.
    // We request DACL_SECURITY_INFORMATION which is always readable for the
    // file owner. The function allocates descriptor (freed via LocalFree).
    unsafe {
        GetNamedSecurityInfoW(
            windows::core::PCWSTR(path_wide.as_ptr()),
            SE_FILE_OBJECT,
            DACL_SECURITY_INFORMATION,
            None,
            None,
            Some(&mut dacl_ptr),
            None,
            &mut descriptor,
        )
        .map_err(|e| SandboxError::Win32 {
            operation: format!("GetNamedSecurityInfoW({})", path.display()),
            error_code: e.code().0 as u32,
        })?;
    }

    // ── 2. Build new ACE ─────────────────────────────────────────────────
    let access_mask = match mode {
        MountMode::ReadOnly => READ_ONLY_ACCESS,
        MountMode::ReadWrite => READ_WRITE_ACCESS,
    };

    let mut ea = EXPLICIT_ACCESS_W {
        grfAccessPermissions: access_mask,
        grfAccessMode: GRANT_ACCESS,
        grfInheritance: SUB_CONTAINERS_AND_OBJECTS_INHERIT,
        Trustee: TRUSTEE_W {
            TrusteeForm: TRUSTEE_IS_SID,
            TrusteeType: TRUSTEE_IS_USER,
            ptstrName: windows::core::PWSTR(sid.0.cast()),
            ..Default::default()
        },
    };

    // ── 3. Merge new ACE into existing DACL ──────────────────────────────
    let mut new_dacl: *mut SEC_ACL = std::ptr::null_mut();

    // SAFETY: dacl_ptr is a valid DACL from GetNamedSecurityInfoW.
    // ea is a properly initialized EXPLICIT_ACCESS_W.
    // SetEntriesInAclW merges the new ACE and allocates new_dacl.
    unsafe {
        SetEntriesInAclW(Some(&[ea]), Some(dacl_ptr), &mut new_dacl).map_err(|e| {
            SandboxError::Win32 {
                operation: "SetEntriesInAclW".into(),
                error_code: e.code().0 as u32,
            }
        })?;
    }

    // Ensure new_dacl is freed even on error below
    let new_dacl_guard = DaclGuard(new_dacl);

    // ── 4. Apply modified DACL ───────────────────────────────────────────
    // SAFETY: path_wide is valid. new_dacl is a valid merged DACL from SetEntriesInAclW.
    unsafe {
        SetNamedSecurityInfoW(
            windows::core::PCWSTR(path_wide.as_ptr()),
            SE_FILE_OBJECT,
            DACL_SECURITY_INFORMATION,
            None,
            None,
            Some(new_dacl),
            None,
        )
        .map_err(|e| SandboxError::Win32 {
            operation: format!("SetNamedSecurityInfoW({})", path.display()),
            error_code: e.code().0 as u32,
        })?;
    }

    // Don't free new_dacl yet — it's now in use by the filesystem
    std::mem::forget(new_dacl_guard);

    // Free new_dacl (the OS has copied it)
    // SAFETY: new_dacl was allocated by SetEntriesInAclW. Now that the OS has
    // applied it, we can free our copy.
    unsafe {
        let _ = LocalFree(Some(std::mem::transmute(new_dacl)));
    }

    info!(
        path = %path.display(),
        mode = ?mode,
        "workspace ACL modified — access granted"
    );

    Ok(AclGuard {
        path: path.to_path_buf(),
        original_dacl: dacl_ptr,
        original_descriptor: descriptor,
    })
}

/// RAII guard that frees a DACL allocated by `SetEntriesInAclW`.
struct DaclGuard(*mut SEC_ACL);

impl Drop for DaclGuard {
    fn drop(&mut self) {
        if !self.0.is_null() {
            // SAFETY: DACL was allocated by SetEntriesInAclW. LocalFree is documented.
            unsafe {
                let _ = LocalFree(Some(std::mem::transmute(self.0)));
            }
        }
    }
}

/// Convert a `Path` to a null-terminated UTF-16 wide string for Win32 APIs.
fn to_wide_string(path: &Path) -> Vec<u16> {
    use std::os::windows::ffi::OsStrExt;
    path.as_os_str()
        .encode_wide()
        .chain(std::iter::once(0))
        .collect()
}
