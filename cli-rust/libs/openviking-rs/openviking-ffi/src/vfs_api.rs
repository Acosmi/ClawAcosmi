// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! FFI exports for local file system and VikingFS operations.
//!
//! Provides opaque-handle based API for constructing a [`LocalFs`],
//! reading/writing files, creating directories, and checking existence.

use openviking_vfs::LocalFs;

use crate::error::{fail, FfiErrorCode};
use crate::runtime::block_on_ffi;

// Re-use the helper from session_api.
use crate::session_api::cstr_to_str;

// ---------------------------------------------------------------------------
// ovk_local_fs_new / free
// ---------------------------------------------------------------------------

/// Create a new [`LocalFs`] backed by the given root directory.
///
/// # Returns
/// Opaque `*mut LocalFs` on success, `null` on failure.
///
/// # Safety
/// `root_path` must be valid UTF-8 of `root_len` bytes.
#[no_mangle]
pub unsafe extern "C" fn ovk_local_fs_new(
    root_path: *const u8,
    root_len: usize,
) -> *mut LocalFs {
    let result = (|| -> Result<*mut LocalFs, i32> {
        let root = unsafe { cstr_to_str(root_path, root_len)? };
        Ok(Box::into_raw(Box::new(LocalFs::new(root))))
    })();

    match result {
        Ok(ptr) => ptr,
        Err(_) => std::ptr::null_mut(),
    }
}

/// Free a [`LocalFs`] handle.
///
/// # Safety
/// `handle` must have been returned by [`ovk_local_fs_new`].
#[no_mangle]
pub unsafe extern "C" fn ovk_local_fs_free(handle: *mut LocalFs) {
    if !handle.is_null() {
        unsafe {
            drop(Box::from_raw(handle));
        }
    }
}

// ---------------------------------------------------------------------------
// File operations
// ---------------------------------------------------------------------------

/// Read a file through the [`FileSystem`] trait.
///
/// # Parameters
/// - `fs`: LocalFs handle.
/// - `uri` / `uri_len`: Viking URI string.
/// - `out` / `out_cap`: caller-allocated output buffer.
///
/// # Returns
/// - Positive: bytes written.
/// - `0`: file is empty.
/// - Negative: error code.
///
/// # Safety
/// All pointers must be valid.
#[no_mangle]
pub unsafe extern "C" fn ovk_vfs_read(
    fs: *mut LocalFs,
    uri: *const u8,
    uri_len: usize,
    out: *mut u8,
    out_cap: usize,
) -> i32 {
    if fs.is_null() {
        return fail(FfiErrorCode::NullPointer, "fs handle is null");
    }

    let result = (|| -> Result<i32, i32> {
        let uri_str = unsafe { cstr_to_str(uri, uri_len)? };
        let fs_ref = unsafe { &*fs };

        block_on_ffi(async {
            use openviking_session::FileSystem;
            let content = fs_ref.read(uri_str).await.map_err(|e| {
                fail(FfiErrorCode::IoError, format!("read failed: {e}"))
            })?;

            if out.is_null() || out_cap == 0 {
                return Ok(0);
            }

            let write_len = content.len().min(out_cap);
            unsafe {
                std::ptr::copy_nonoverlapping(content.as_ptr(), out, write_len);
            }
            Ok(write_len as i32)
        })
    })();

    match result {
        Ok(n) => n,
        Err(code) => code,
    }
}

/// Write content to a file.
///
/// # Safety
/// All pointers must be valid.
#[no_mangle]
pub unsafe extern "C" fn ovk_vfs_write(
    fs: *mut LocalFs,
    uri: *const u8,
    uri_len: usize,
    content: *const u8,
    content_len: usize,
) -> i32 {
    if fs.is_null() {
        return fail(FfiErrorCode::NullPointer, "fs handle is null");
    }

    let result = (|| -> Result<(), i32> {
        let uri_str = unsafe { cstr_to_str(uri, uri_len)? };
        let text = unsafe { cstr_to_str(content, content_len)? };
        let fs_ref = unsafe { &*fs };

        block_on_ffi(async {
            use openviking_session::FileSystem;
            fs_ref.write(uri_str, text).await.map_err(|e| {
                fail(FfiErrorCode::IoError, format!("write failed: {e}"))
            })
        })
    })();

    match result {
        Ok(()) => FfiErrorCode::Ok.as_i32(),
        Err(code) => code,
    }
}

/// Create a directory.
///
/// # Safety
/// All pointers must be valid.
#[no_mangle]
pub unsafe extern "C" fn ovk_vfs_mkdir(
    fs: *mut LocalFs,
    uri: *const u8,
    uri_len: usize,
) -> i32 {
    if fs.is_null() {
        return fail(FfiErrorCode::NullPointer, "fs handle is null");
    }

    let result = (|| -> Result<(), i32> {
        let uri_str = unsafe { cstr_to_str(uri, uri_len)? };
        let fs_ref = unsafe { &*fs };

        block_on_ffi(async {
            use openviking_session::FileSystem;
            fs_ref.mkdir(uri_str).await.map_err(|e| {
                fail(FfiErrorCode::IoError, format!("mkdir failed: {e}"))
            })
        })
    })();

    match result {
        Ok(()) => FfiErrorCode::Ok.as_i32(),
        Err(code) => code,
    }
}

/// Check if a file or directory exists.
///
/// # Returns
/// - `1`: exists.
/// - `0`: does not exist.
/// - Negative: error code.
///
/// # Safety
/// All pointers must be valid.
#[no_mangle]
pub unsafe extern "C" fn ovk_vfs_exists(
    fs: *mut LocalFs,
    uri: *const u8,
    uri_len: usize,
) -> i32 {
    if fs.is_null() {
        return fail(FfiErrorCode::NullPointer, "fs handle is null");
    }

    let result = (|| -> Result<i32, i32> {
        let uri_str = unsafe { cstr_to_str(uri, uri_len)? };
        let fs_ref = unsafe { &*fs };

        block_on_ffi(async {
            use openviking_session::FileSystem;
            let exists = fs_ref.exists(uri_str).await.map_err(|e| {
                fail(FfiErrorCode::IoError, format!("exists failed: {e}"))
            })?;
            Ok(if exists { 1 } else { 0 })
        })
    })();

    match result {
        Ok(n) => n,
        Err(code) => code,
    }
}

/// Remove a file or directory.
///
/// # Safety
/// All pointers must be valid.
#[no_mangle]
pub unsafe extern "C" fn ovk_vfs_rm(
    fs: *mut LocalFs,
    uri: *const u8,
    uri_len: usize,
) -> i32 {
    if fs.is_null() {
        return fail(FfiErrorCode::NullPointer, "fs handle is null");
    }

    let result = (|| -> Result<(), i32> {
        let uri_str = unsafe { cstr_to_str(uri, uri_len)? };
        let fs_ref = unsafe { &*fs };

        block_on_ffi(async {
            use openviking_session::FileSystem;
            fs_ref.rm(uri_str).await.map_err(|e| {
                fail(FfiErrorCode::IoError, format!("rm failed: {e}"))
            })
        })
    })();

    match result {
        Ok(()) => FfiErrorCode::Ok.as_i32(),
        Err(code) => code,
    }
}
