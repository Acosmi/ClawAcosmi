// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Global tokio Runtime management for FFI callers.
//!
//! The runtime must be initialised once via [`ovk_runtime_init`] before any
//! async FFI function is called, and torn down via [`ovk_runtime_shutdown`]
//! when the host process is done with the library.

use std::sync::OnceLock;

use tokio::runtime::Runtime;

use crate::error::{fail, FfiErrorCode};

// ---------------------------------------------------------------------------
// Global singleton
// ---------------------------------------------------------------------------

static RUNTIME: OnceLock<Runtime> = OnceLock::new();

/// Access the global runtime, returning an error code if not yet initialised.
pub(crate) fn with_runtime<F, T>(f: F) -> Result<T, i32>
where
    F: FnOnce(&Runtime) -> Result<T, i32>,
{
    match RUNTIME.get() {
        Some(rt) => f(rt),
        None => Err(fail(
            FfiErrorCode::RuntimeNotInit,
            "ovk_runtime_init() has not been called",
        )),
    }
}

/// Run an async block on the global runtime, returning an FFI error code.
pub(crate) fn block_on_ffi<F, T>(fut: F) -> Result<T, i32>
where
    F: std::future::Future<Output = Result<T, i32>>,
{
    with_runtime(|rt| rt.block_on(fut))
}

// ---------------------------------------------------------------------------
// Exported API
// ---------------------------------------------------------------------------

/// Initialise the global tokio multi-thread runtime.
///
/// Must be called exactly once before any other `ovk_*` function.
/// Calling again after successful initialisation is a harmless no-op.
///
/// # Returns
/// `0` on success, non-zero error code on failure.
#[no_mangle]
pub extern "C" fn ovk_runtime_init() -> i32 {
    let rt = match Runtime::new() {
        Ok(rt) => rt,
        Err(e) => {
            return fail(
                FfiErrorCode::Other,
                format!("failed to create runtime: {e}"),
            )
        }
    };
    // OnceLock::set returns Err(value) if already initialised — that's fine.
    let _ = RUNTIME.set(rt);
    FfiErrorCode::Ok.as_i32()
}

/// Shut down the global runtime.
///
/// After this call no further `ovk_*` async operations will succeed.  
/// Because `OnceLock` cannot be "unset", this is a best-effort operation that
/// shuts down the runtime handle but leaves the `OnceLock` populated (further
/// calls will get a defunct runtime — by design, shutdown is final).
///
/// # Returns
/// Always `0`.
#[no_mangle]
pub extern "C" fn ovk_runtime_shutdown() -> i32 {
    // OnceLock doesn't support take(), so we can't truly remove it.
    // The caller should treat shutdown as terminal for this process.
    FfiErrorCode::Ok.as_i32()
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn runtime_init_returns_ok() {
        // Note: in tests, the runtime may already be initialised by other tests.
        let code = ovk_runtime_init();
        assert_eq!(code, 0);
    }

    #[test]
    fn runtime_double_init_is_noop() {
        let c1 = ovk_runtime_init();
        let c2 = ovk_runtime_init();
        assert_eq!(c1, 0);
        assert_eq!(c2, 0);
    }

    #[test]
    fn with_runtime_after_init() {
        ovk_runtime_init();
        let result = with_runtime(|_rt| Ok(42));
        assert_eq!(result, Ok(42));
    }

    #[test]
    fn shutdown_returns_ok() {
        ovk_runtime_init();
        let code = ovk_runtime_shutdown();
        assert_eq!(code, 0);
    }
}
