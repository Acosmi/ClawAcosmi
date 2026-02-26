//! FFI bindings for macOS Seatbelt (Sandbox) private API.
//!
//! These functions are not in Apple's public headers but are available in `libSystem`
//! and used by Chromium, Firefox, and Nix package manager. The kernel-level sandbox
//! enforcement mechanism is stable; only the public CLI (`sandbox-exec`) is deprecated.
//!
//! # References
//!
//! - Chromium: `sandbox/mac/seatbelt.cc` — `sandbox_init_with_parameters`
//! - Apple: `sandbox(7)` man page
//! - Verification: `docs/claude/tracking/verification-macos-sandbox-apis-2026-02-25.md`

use std::ffi::{CStr, CString};
use std::ptr;

use crate::error::SandboxError;

// ── Private Apple API declarations ─────────────────────────────────────────

// SAFETY: These are private Apple API functions available in libSystem.
// They are stable and used by Chromium, Firefox, and other major software.
// The function signatures match Apple's internal headers and Chromium's declarations.
unsafe extern "C" {
    /// Apply a sandbox profile to the current process.
    ///
    /// - `profile`: SBPL string (when `flags` = 0) or named profile
    /// - `flags`: 0 = profile is a string, 1 = named built-in
    /// - `parameters`: null-terminated array of alternating key-value C strings
    ///   `["KEY1", "val1", "KEY2", "val2", NULL]`
    /// - `errorbuf`: on failure, set to an error message (must free with `sandbox_free_error`)
    ///
    /// Returns 0 on success, -1 on failure.
    fn sandbox_init_with_parameters(
        profile: *const libc::c_char,
        flags: u64,
        parameters: *const *const libc::c_char,
        errorbuf: *mut *mut libc::c_char,
    ) -> libc::c_int;

    /// Free an error buffer returned by `sandbox_init_with_parameters`.
    fn sandbox_free_error(errorbuf: *mut libc::c_char);
}

// ── Pre-built FFI arguments ────────────────────────────────────────────────

/// Pre-built arguments for `sandbox_init_with_parameters`.
///
/// All `CString` allocations happen at construction time (before `fork()`),
/// so [`apply()`](Self::apply) can be called in a post-fork/pre-exec context
/// without heap allocation.
pub struct SandboxArgs {
    /// The SBPL profile string as a C string.
    profile: CString,

    /// Owned storage for parameter key-value `CString`s.
    /// Must outlive `param_ptrs`.
    _param_storage: Vec<CString>,

    /// Null-terminated array of pointers into `_param_storage`:
    /// `[key0_ptr, val0_ptr, key1_ptr, val1_ptr, ..., NULL]`
    param_ptrs: Vec<*const libc::c_char>,
}

// SAFETY: The raw pointers in `param_ptrs` reference heap data owned by `_param_storage`.
// The entire struct is moved into a `pre_exec` closure and used from a single thread
// (the child process after fork). The pointed-to data does not move on the heap.
unsafe impl Send for SandboxArgs {}
// SAFETY: Same reasoning — no concurrent access occurs. The struct is consumed
// exactly once in the child's pre_exec callback.
unsafe impl Sync for SandboxArgs {}

impl SandboxArgs {
    /// Build `SandboxArgs` from a profile string and parameter key-value pairs.
    ///
    /// All `CString` conversions happen here (on the heap, before fork).
    pub fn new(profile: &str, params: &[(&str, &str)]) -> Result<Self, SandboxError> {
        let profile = CString::new(profile).map_err(|_| SandboxError::Seatbelt {
            message: "SBPL profile contains null bytes".into(),
        })?;

        let mut storage = Vec::with_capacity(params.len() * 2);
        let mut ptrs = Vec::with_capacity(params.len() * 2 + 1);

        for (key, value) in params {
            let key_cs = CString::new(*key).map_err(|_| SandboxError::Seatbelt {
                message: format!("parameter key '{key}' contains null bytes"),
            })?;
            let val_cs = CString::new(*value).map_err(|_| SandboxError::Seatbelt {
                message: format!("parameter value for '{key}' contains null bytes"),
            })?;
            // Collect pointers before pushing to storage to avoid borrow issues.
            // CString data lives on the heap; its pointer survives moves of the CString wrapper.
            ptrs.push(key_cs.as_ptr());
            ptrs.push(val_cs.as_ptr());
            storage.push(key_cs);
            storage.push(val_cs);
        }
        ptrs.push(ptr::null()); // Null terminator

        Ok(Self {
            profile,
            _param_storage: storage,
            param_ptrs: ptrs,
        })
    }

    /// Apply the sandbox profile to the current process.
    ///
    /// This is designed to be called inside a `Command::pre_exec` closure
    /// (after `fork()`, before `exec()`). No heap allocations occur here.
    ///
    /// # Errors
    ///
    /// Returns `std::io::Error` (compatible with `pre_exec` return type)
    /// if `sandbox_init_with_parameters` fails.
    pub fn apply(&self) -> std::io::Result<()> {
        let mut errorbuf: *mut libc::c_char = ptr::null_mut();

        // SAFETY:
        // - `self.profile` is a valid, non-null CString; pointer is stable (heap-backed).
        // - `self.param_ptrs` is a null-terminated array of valid CString pointers.
        //   The pointed-to data is owned by `self._param_storage` which is alive.
        // - `errorbuf` is a valid output pointer on the stack.
        // - `flags = 0` means the profile is interpreted as an SBPL string.
        // - This function is called exactly once in the child process after fork().
        //   The sandbox, once applied, is irreversible and inherited by exec'd processes.
        let result = unsafe {
            sandbox_init_with_parameters(
                self.profile.as_ptr(),
                0, // profile is a string
                self.param_ptrs.as_ptr(),
                std::ptr::addr_of_mut!(errorbuf),
            )
        };

        if result != 0 {
            let message = if errorbuf.is_null() {
                format!("sandbox_init_with_parameters returned {result}")
            } else {
                // SAFETY: errorbuf was set by sandbox_init_with_parameters to a valid C string.
                let msg = unsafe { CStr::from_ptr(errorbuf) }
                    .to_string_lossy()
                    .into_owned();
                // SAFETY: errorbuf was allocated by the sandbox subsystem; must be freed
                // with sandbox_free_error (not libc free).
                unsafe { sandbox_free_error(errorbuf) };
                msg
            };
            return Err(std::io::Error::other(format!(
                "seatbelt sandbox_init failed: {message}"
            )));
        }

        Ok(())
    }
}

#[cfg(test)]
#[allow(clippy::unwrap_used)]
mod tests {
    use super::*;

    #[test]
    fn sandbox_args_creation_valid() {
        let args = SandboxArgs::new(
            "(version 1)(deny default)(import \"bsd.sb\")",
            &[("WORKSPACE_DIR", "/tmp/test")],
        );
        assert!(args.is_ok());
    }

    #[test]
    fn sandbox_args_rejects_null_bytes_in_profile() {
        let args = SandboxArgs::new("profile\0with\0nulls", &[]);
        assert!(args.is_err());
    }

    #[test]
    fn sandbox_args_rejects_null_bytes_in_params() {
        let args = SandboxArgs::new("(version 1)", &[("key\0bad", "value")]);
        assert!(args.is_err());
    }
}
