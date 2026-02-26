//! Optional AppContainer isolation (Windows 8+).
//!
//! AppContainer provides an additional isolation layer on top of Restricted Tokens:
//! - Separate object namespace
//! - Network isolation
//! - Additional filesystem isolation via Package SID
//!
//! # Why optional
//!
//! AppContainer has significant compatibility issues with traditional `.exe` files:
//! - Most system DLLs have `ALL APPLICATION PACKAGES` ACE, but third-party don't
//! - Shell APIs don't function inside AppContainer
//! - DLL delay loading can fail if the DLL's location lacks ACLs
//!
//! This module is **opt-in only** — enabled via explicit user configuration.
//!
//! # Lifecycle
//!
//! ```text
//! CreateAppContainerProfile  →  get Package SID  →  add ACLs  →  run process
//!                                                                     │
//!                                                                     ▼
//!                                                    DeleteAppContainerProfile (cleanup)
//! ```

use tracing::{debug, info, warn};

use windows::Win32::Foundation::{LocalFree, PSID};
use windows::Win32::Security::Isolation::{
    CreateAppContainerProfile, DeleteAppContainerProfile, DeriveAppContainerSidFromAppContainerName,
};
use windows::core::PWSTR;

use crate::error::SandboxError;

/// RAII guard for an AppContainer profile.
///
/// On drop, deletes the AppContainer profile. This removes the Package SID
/// and cleans up the virtualization state.
pub struct AppContainerGuard {
    profile_name: String,
    sid: PSID,
}

impl AppContainerGuard {
    /// Get the Package SID for this AppContainer.
    ///
    /// Used to add ACEs to workspace directories.
    pub fn sid(&self) -> PSID {
        self.sid
    }

    /// Get the profile name.
    pub fn profile_name(&self) -> &str {
        &self.profile_name
    }
}

impl Drop for AppContainerGuard {
    fn drop(&mut self) {
        // Delete the profile
        let name_wide: Vec<u16> = self
            .profile_name
            .encode_utf16()
            .chain(std::iter::once(0))
            .collect();

        // SAFETY: name_wide is a valid null-terminated UTF-16 string matching
        // the profile name used in CreateAppContainerProfile.
        let result =
            unsafe { DeleteAppContainerProfile(windows::core::PCWSTR(name_wide.as_ptr())) };

        if let Err(e) = result {
            warn!(
                profile = %self.profile_name,
                error = e.code().0,
                "failed to delete AppContainer profile"
            );
        } else {
            debug!(profile = %self.profile_name, "AppContainer profile deleted");
        }

        // Free the SID
        // SAFETY: SID was allocated by DeriveAppContainerSidFromAppContainerName.
        // FreeSid / LocalFree is the documented cleanup.
        if !self.sid.0.is_null() {
            unsafe {
                let _ = LocalFree(Some(std::mem::transmute(self.sid.0)));
            }
        }
    }
}

// SAFETY: The PSID pointer is heap-allocated by Win32 and not shared.
unsafe impl Send for AppContainerGuard {}

/// Create an AppContainer profile for sandbox isolation.
///
/// Returns an [`AppContainerGuard`] that owns the profile and deletes it on drop.
///
/// # Arguments
///
/// * `name_suffix` — Unique suffix for the profile name (e.g., PID or UUID)
pub fn create_appcontainer(name_suffix: &str) -> Result<AppContainerGuard, SandboxError> {
    let profile_name = format!("oa-sandbox-{name_suffix}");
    let display_name = format!("OpenAcosmi Sandbox {name_suffix}");
    let description = "Temporary AppContainer for OpenAcosmi sandbox execution";

    let name_wide: Vec<u16> = profile_name
        .encode_utf16()
        .chain(std::iter::once(0))
        .collect();
    let display_wide: Vec<u16> = display_name
        .encode_utf16()
        .chain(std::iter::once(0))
        .collect();
    let desc_wide: Vec<u16> = description
        .encode_utf16()
        .chain(std::iter::once(0))
        .collect();

    let mut sid = PSID::default();

    // Try to create the profile. If it already exists (from a crashed previous run),
    // delete it first and retry.
    // SAFETY: All wide strings are valid null-terminated UTF-16.
    // CreateAppContainerProfile allocates and returns the SID.
    let create_result = unsafe {
        CreateAppContainerProfile(
            windows::core::PCWSTR(name_wide.as_ptr()),
            windows::core::PCWSTR(display_wide.as_ptr()),
            windows::core::PCWSTR(desc_wide.as_ptr()),
            None, // no capabilities
            &mut sid,
        )
    };

    if let Err(e) = create_result {
        // HRESULT 0x800700B7 = ERROR_ALREADY_EXISTS
        if e.code().0 as u32 == 0x800700B7 {
            debug!(profile = %profile_name, "AppContainer profile already exists, recreating");

            // Delete and retry
            // SAFETY: name_wide is valid. Deleting a stale profile is safe.
            unsafe {
                let _ = DeleteAppContainerProfile(windows::core::PCWSTR(name_wide.as_ptr()));
            }

            // SAFETY: Same as above — retrying after cleanup.
            unsafe {
                CreateAppContainerProfile(
                    windows::core::PCWSTR(name_wide.as_ptr()),
                    windows::core::PCWSTR(display_wide.as_ptr()),
                    windows::core::PCWSTR(desc_wide.as_ptr()),
                    None,
                    &mut sid,
                )
                .map_err(|e| SandboxError::Win32 {
                    operation: "CreateAppContainerProfile (retry)".into(),
                    error_code: e.code().0 as u32,
                })?;
            }
        } else {
            return Err(SandboxError::Win32 {
                operation: "CreateAppContainerProfile".into(),
                error_code: e.code().0 as u32,
            });
        }
    }

    info!(
        profile = %profile_name,
        "AppContainer profile created"
    );

    Ok(AppContainerGuard { profile_name, sid })
}
