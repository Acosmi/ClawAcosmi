// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! FFI error codes and thread-local error message storage.
//!
//! Every `extern "C"` function returns an [`FfiErrorCode`] (i32).
//! The last error message can be retrieved via [`ovk_last_error`].
//!
//! **Resolves deferred item D-A3-2**: Go callers use numeric error codes
//! instead of parsing error strings.

use std::cell::RefCell;

// ---------------------------------------------------------------------------
// Error codes (repr(i32) for C-ABI)
// ---------------------------------------------------------------------------

/// FFI error codes shared between Rust and Go/C callers.
#[repr(i32)]
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum FfiErrorCode {
    /// Success.
    Ok = 0,
    /// A required pointer argument was null.
    NullPointer = 1,
    /// A string argument contained invalid UTF-8.
    InvalidUtf8 = 2,
    /// An argument value is invalid (null pointer, out of range, etc.).
    InvalidArgument = 5,
    /// The global tokio runtime has not been initialised.
    RuntimeNotInit = 3,
    /// The output buffer is too small; required size stored in error message.
    BufferTooSmall = 4,
    /// Generic I/O error (file system, network).
    IoError = 10,
    /// Session-level error (load, commit, add_message).
    SessionError = 20,
    /// Generic vector store error.
    VectorStoreError = 30,
    /// The requested collection does not exist (maps to Python `CollectionNotFoundError`).
    CollectionNotFound = 31,
    /// Catch-all for unclassified errors.
    Other = 99,
}

impl FfiErrorCode {
    /// Convert to the raw `i32` representation.
    #[inline]
    pub fn as_i32(self) -> i32 {
        self as i32
    }
}

// ---------------------------------------------------------------------------
// Thread-local last-error storage
// ---------------------------------------------------------------------------

thread_local! {
    static LAST_ERROR: RefCell<String> = const { RefCell::new(String::new()) };
}

/// Record an error message on the current thread.
pub(crate) fn set_last_error(msg: impl Into<String>) {
    LAST_ERROR.with(|cell| {
        *cell.borrow_mut() = msg.into();
    });
}

/// Convenience: set the error and return the corresponding code.
pub(crate) fn fail(code: FfiErrorCode, msg: impl Into<String>) -> i32 {
    set_last_error(msg);
    code.as_i32()
}

// ---------------------------------------------------------------------------
// Exported: ovk_last_error
// ---------------------------------------------------------------------------

/// Copy the last error message into the caller-provided buffer.
///
/// # Returns
/// - `0` if no error is stored (buffer untouched).
/// - Positive value = number of bytes written (including NUL terminator).
/// - Negative value = required buffer size (absolute value), when `buf_len`
///   is too small.
///
/// # Safety
/// `buf` must point to a writeable region of at least `buf_len` bytes.
#[no_mangle]
pub unsafe extern "C" fn ovk_last_error(buf: *mut u8, buf_len: usize) -> i32 {
    LAST_ERROR.with(|cell| {
        let msg = cell.borrow();
        if msg.is_empty() {
            return 0;
        }
        let needed = msg.len() + 1; // +1 for NUL
        if buf.is_null() || buf_len < needed {
            return -(needed as i32);
        }
        unsafe {
            std::ptr::copy_nonoverlapping(msg.as_ptr(), buf, msg.len());
            *buf.add(msg.len()) = 0; // NUL terminator
        }
        needed as i32
    })
}

/// Clear the last error message on the current thread.
#[no_mangle]
pub extern "C" fn ovk_clear_error() {
    LAST_ERROR.with(|cell| cell.borrow_mut().clear());
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn error_code_values() {
        assert_eq!(FfiErrorCode::Ok.as_i32(), 0);
        assert_eq!(FfiErrorCode::NullPointer.as_i32(), 1);
        assert_eq!(FfiErrorCode::CollectionNotFound.as_i32(), 31);
        assert_eq!(FfiErrorCode::Other.as_i32(), 99);
    }

    #[test]
    fn set_and_read_last_error() {
        set_last_error("test error");
        let mut buf = [0u8; 64];
        let n = unsafe { ovk_last_error(buf.as_mut_ptr(), buf.len()) };
        assert!(n > 0);
        let s = std::str::from_utf8(&buf[..n as usize - 1]).unwrap();
        assert_eq!(s, "test error");
    }

    #[test]
    fn buffer_too_small_returns_negative() {
        set_last_error("hello");
        let mut buf = [0u8; 2];
        let n = unsafe { ovk_last_error(buf.as_mut_ptr(), buf.len()) };
        assert!(n < 0);
        assert_eq!(n.unsigned_abs() as usize, 6); // "hello" + NUL
    }

    #[test]
    fn clear_error() {
        set_last_error("something");
        ovk_clear_error();
        let n = unsafe { ovk_last_error(std::ptr::null_mut(), 0) };
        assert_eq!(n, 0);
    }

    #[test]
    fn fail_helper() {
        let code = fail(FfiErrorCode::IoError, "disk full");
        assert_eq!(code, 10);
        let mut buf = [0u8; 64];
        let n = unsafe { ovk_last_error(buf.as_mut_ptr(), buf.len()) };
        let s = std::str::from_utf8(&buf[..n as usize - 1]).unwrap();
        assert_eq!(s, "disk full");
    }
}
