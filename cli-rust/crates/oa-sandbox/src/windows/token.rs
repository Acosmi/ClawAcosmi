//! Windows Restricted Token creation.
//!
//! Creates a restricted version of the current process token with:
//! - All privileges removed
//! - All group SIDs set to deny-only (except the logon SID)
//! - Low Integrity Level
//!
//! This follows Chromium's approach for renderer process isolation.
//! The restricted token is used with `CreateProcessAsUserW` to spawn
//! sandboxed child processes.

use tracing::{debug, info};

use windows::Win32::Foundation::{CloseHandle, HANDLE, LUID};
use windows::Win32::Security::Authorization::ConvertStringSidToSidW;
use windows::Win32::Security::{
    AdjustTokenPrivileges, CreateRestrictedToken, DISABLE_MAX_PRIVILEGE, GetTokenInformation,
    SE_GROUP_INTEGRITY, SE_GROUP_LOGON_ID, SID_AND_ATTRIBUTES, SetTokenInformation,
    TOKEN_ACCESS_MASK, TOKEN_ALL_ACCESS, TOKEN_ASSIGN_PRIMARY, TOKEN_DUPLICATE, TOKEN_GROUPS,
    TOKEN_MANDATORY_LABEL, TOKEN_PRIVILEGES, TOKEN_QUERY, TokenGroups, TokenIntegrityLevel,
    TokenPrivileges,
};
use windows::Win32::System::Threading::{GetCurrentProcess, OpenProcessToken};

use crate::error::SandboxError;

/// RAII guard for a restricted token handle.
pub struct RestrictedToken {
    handle: HANDLE,
}

impl RestrictedToken {
    /// Get the raw token handle (for `CreateProcessAsUserW`).
    pub fn handle(&self) -> HANDLE {
        self.handle
    }
}

impl Drop for RestrictedToken {
    fn drop(&mut self) {
        // SAFETY: self.handle is a valid token handle from CreateRestrictedToken.
        // CloseHandle is safe for any handle we own.
        unsafe {
            let _ = CloseHandle(self.handle);
        }
        debug!("restricted token handle closed");
    }
}

// SAFETY: Token handles are thread-safe in Win32.
unsafe impl Send for RestrictedToken {}
unsafe impl Sync for RestrictedToken {}

/// Create a restricted token from the current process token.
///
/// The token has:
/// - All privileges stripped (`DISABLE_MAX_PRIVILEGE`)
/// - All group SIDs set to deny-only (except logon SID)
/// - Low Integrity Level (S-1-16-4096)
pub fn create_restricted_token() -> Result<RestrictedToken, SandboxError> {
    // ── 1. Open current process token ────────────────────────────────────
    let mut process_token = HANDLE::default();
    // SAFETY: GetCurrentProcess() returns a pseudo-handle (always valid).
    // OpenProcessToken with TOKEN_DUPLICATE | TOKEN_QUERY | TOKEN_ASSIGN_PRIMARY
    // is safe — we're opening our own process token.
    unsafe {
        OpenProcessToken(
            GetCurrentProcess(),
            TOKEN_DUPLICATE | TOKEN_QUERY | TOKEN_ASSIGN_PRIMARY,
            &mut process_token,
        )
        .map_err(|e| SandboxError::Win32 {
            operation: "OpenProcessToken".into(),
            error_code: e.code().0 as u32,
        })?;
    }

    // Ensure process token is closed even on error
    let _process_token_guard = HandleGuard(process_token);

    // ── 2. Get group SIDs to set as deny-only ────────────────────────────
    let deny_sids = get_deny_only_sids(process_token)?;

    // ── 3. Create restricted token ───────────────────────────────────────
    let mut raw_token = HANDLE::default();

    // SAFETY: process_token is a valid token handle opened above.
    // DISABLE_MAX_PRIVILEGE removes all privileges.
    // deny_sids contains properly formed SID_AND_ATTRIBUTES from our token.
    // The resulting raw_token is a new handle we own.
    unsafe {
        CreateRestrictedToken(
            process_token,
            DISABLE_MAX_PRIVILEGE,
            Some(&deny_sids),
            None, // no privileges to delete (DISABLE_MAX_PRIVILEGE handles it)
            None, // no restricting SIDs (using deny-only approach like Chromium)
            &mut raw_token,
        )
        .map_err(|e| SandboxError::Win32 {
            operation: "CreateRestrictedToken".into(),
            error_code: e.code().0 as u32,
        })?;
    }

    // S-6 audit fix: Wrap in RestrictedToken immediately so Drop closes the handle
    // if set_low_integrity_level fails below.
    let token = RestrictedToken { handle: raw_token };

    // ── 4. Set Low Integrity Level ───────────────────────────────────────
    set_low_integrity_level(token.handle)?;

    info!("restricted token created (privileges stripped, low integrity)");

    Ok(token)
}

/// Get all group SIDs from the token, marking non-logon SIDs as deny-only.
///
/// The logon SID is preserved because it's needed for desktop/window station access.
fn get_deny_only_sids(token: HANDLE) -> Result<Vec<SID_AND_ATTRIBUTES>, SandboxError> {
    // First call to get required buffer size
    let mut size = 0u32;
    // SAFETY: First call with null buffer to get size. This is the documented pattern
    // for GetTokenInformation — it sets size and returns ERROR_INSUFFICIENT_BUFFER.
    unsafe {
        let _ = GetTokenInformation(token, TokenGroups, None, 0, &mut size);
    }

    if size == 0 {
        return Err(SandboxError::Win32 {
            operation: "GetTokenInformation(TokenGroups) size query".into(),
            error_code: 0,
        });
    }

    let mut buffer = vec![0u8; size as usize];

    // SAFETY: buffer is properly sized (from the first call). token is valid.
    // We cast the buffer to TOKEN_GROUPS after the call succeeds.
    unsafe {
        GetTokenInformation(
            token,
            TokenGroups,
            Some(buffer.as_mut_ptr().cast()),
            size,
            &mut size,
        )
        .map_err(|e| SandboxError::Win32 {
            operation: "GetTokenInformation(TokenGroups)".into(),
            error_code: e.code().0 as u32,
        })?;
    }

    // SAFETY: GetTokenInformation succeeded and filled buffer with TOKEN_GROUPS data.
    // The buffer is properly aligned (Vec<u8> from the heap) and sized.
    let groups = unsafe { &*(buffer.as_ptr().cast::<TOKEN_GROUPS>()) };

    let group_count = groups.GroupCount as usize;
    let mut deny_sids = Vec::with_capacity(group_count);

    for i in 0..group_count {
        // SAFETY: GroupCount tells us how many SID_AND_ATTRIBUTES entries exist.
        // The Groups field is a variable-length array at the end of TOKEN_GROUPS.
        let group = unsafe { *groups.Groups.as_ptr().add(i) };

        // Skip the logon SID — it's needed for desktop/window station access
        if (group.Attributes & SE_GROUP_LOGON_ID.0 as u32) != 0 {
            debug!("preserving logon SID (not deny-only)");
            continue;
        }

        deny_sids.push(SID_AND_ATTRIBUTES {
            Sid: group.Sid,
            Attributes: 0, // CreateRestrictedToken sets deny-only for these
        });
    }

    debug!(
        total_groups = group_count,
        deny_only = deny_sids.len(),
        "collected group SIDs for deny-only"
    );

    Ok(deny_sids)
}

/// Set the integrity level of a token to Low (S-1-16-4096).
///
/// Low Integrity Level prevents the process from writing to objects at
/// Medium or higher integrity (including most user files by default).
fn set_low_integrity_level(token: HANDLE) -> Result<(), SandboxError> {
    // Low Integrity Level SID: S-1-16-4096
    let low_il_str = windows::core::w!("S-1-16-4096");

    let mut low_il_sid = windows::Win32::Foundation::PSID::default();

    // SAFETY: ConvertStringSidToSidW converts a well-known SID string to a SID.
    // "S-1-16-4096" is the documented Low Integrity Level SID.
    // The function allocates the SID — we must free it with LocalFree.
    unsafe {
        ConvertStringSidToSidW(low_il_str, &mut low_il_sid).map_err(|e| SandboxError::Win32 {
            operation: "ConvertStringSidToSidW(Low IL)".into(),
            error_code: e.code().0 as u32,
        })?;
    }

    // Ensure SID is freed even on error
    let _sid_guard = SidGuard(low_il_sid);

    let label = TOKEN_MANDATORY_LABEL {
        Label: SID_AND_ATTRIBUTES {
            Sid: low_il_sid,
            Attributes: SE_GROUP_INTEGRITY.0 as u32,
        },
    };

    // SAFETY: token is a valid restricted token handle.
    // label contains a valid Low IL SID from ConvertStringSidToSidW.
    // SetTokenInformation with TokenIntegrityLevel sets the integrity level.
    unsafe {
        SetTokenInformation(
            token,
            TokenIntegrityLevel,
            std::ptr::from_ref(&label).cast(),
            u32::try_from(
                std::mem::size_of::<TOKEN_MANDATORY_LABEL>()
                    + windows::Win32::Security::GetLengthSid(low_il_sid) as usize,
            )
            .unwrap_or(u32::MAX),
        )
        .map_err(|e| SandboxError::Win32 {
            operation: "SetTokenInformation(IntegrityLevel=Low)".into(),
            error_code: e.code().0 as u32,
        })?;
    }

    debug!("token integrity level set to Low (S-1-16-4096)");
    Ok(())
}

/// RAII guard that closes a HANDLE on drop.
struct HandleGuard(HANDLE);

impl Drop for HandleGuard {
    fn drop(&mut self) {
        // SAFETY: Handle was obtained from a successful Win32 API call.
        unsafe {
            let _ = CloseHandle(self.0);
        }
    }
}

/// RAII guard that frees a PSID allocated by `ConvertStringSidToSidW`.
struct SidGuard(windows::Win32::Foundation::PSID);

impl Drop for SidGuard {
    fn drop(&mut self) {
        // SAFETY: PSID was allocated by ConvertStringSidToSidW.
        // LocalFree is the documented way to release it.
        unsafe {
            let _ = windows::Win32::Foundation::LocalFree(Some(std::mem::transmute(self.0)));
        }
    }
}
