// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Integration tests for the FFI layer.
//!
//! These tests exercise the exported `extern "C"` API surface to verify
//! correctness of handle lifecycle, error reporting, and runtime management.

use crate::error::{ovk_clear_error, ovk_last_error, FfiErrorCode};
use crate::runtime::{ovk_runtime_init, ovk_runtime_shutdown};

// ---------------------------------------------------------------------------
// Runtime lifecycle
// ---------------------------------------------------------------------------

#[test]
fn runtime_init_and_shutdown() {
    let c1 = ovk_runtime_init();
    assert_eq!(c1, FfiErrorCode::Ok.as_i32());

    let c2 = ovk_runtime_shutdown();
    assert_eq!(c2, FfiErrorCode::Ok.as_i32());
}

// ---------------------------------------------------------------------------
// Error reporting
// ---------------------------------------------------------------------------

#[test]
fn error_roundtrip() {
    crate::error::set_last_error("ffi test error");

    let mut buf = [0u8; 128];
    let n = unsafe { ovk_last_error(buf.as_mut_ptr(), buf.len()) };
    assert!(n > 0);

    let msg = std::str::from_utf8(&buf[..n as usize - 1]).unwrap();
    assert_eq!(msg, "ffi test error");

    ovk_clear_error();
    let n2 = unsafe { ovk_last_error(buf.as_mut_ptr(), buf.len()) };
    assert_eq!(n2, 0);
}

// ---------------------------------------------------------------------------
// LocalFs handle lifecycle
// ---------------------------------------------------------------------------

#[test]
fn local_fs_create_and_free() {
    let root = "/tmp/ovk_ffi_test";
    let handle = unsafe { crate::vfs_api::ovk_local_fs_new(root.as_ptr(), root.len()) };
    assert!(!handle.is_null());

    unsafe {
        crate::vfs_api::ovk_local_fs_free(handle);
    }
}

#[test]
fn local_fs_null_path_returns_null() {
    let handle = unsafe { crate::vfs_api::ovk_local_fs_new(std::ptr::null(), 0) };
    assert!(handle.is_null());
}

// ---------------------------------------------------------------------------
// VectorStore handle lifecycle
// ---------------------------------------------------------------------------

#[test]
fn vector_store_create_and_free() {
    let handle = crate::init_api::ovk_vector_store_new();
    assert!(!handle.is_null());

    unsafe {
        crate::init_api::ovk_vector_store_free(handle);
    }
}

// ---------------------------------------------------------------------------
// Session null-safety
// ---------------------------------------------------------------------------

#[test]
fn session_add_message_null_handle() {
    let code = unsafe {
        crate::session_api::ovk_session_add_message(std::ptr::null_mut(), 0, b"hello".as_ptr(), 5)
    };
    assert_eq!(code, FfiErrorCode::NullPointer.as_i32());
}

#[test]
fn session_commit_null_handle() {
    let code = unsafe {
        crate::session_api::ovk_session_commit(std::ptr::null_mut(), std::ptr::null_mut(), 0)
    };
    assert_eq!(code, FfiErrorCode::NullPointer.as_i32());
}

// ---------------------------------------------------------------------------
// Init API null-safety
// ---------------------------------------------------------------------------

#[test]
fn init_context_collection_null_vs() {
    let code = unsafe {
        crate::init_api::ovk_init_context_collection(std::ptr::null_mut(), b"col".as_ptr(), 3, 128)
    };
    assert_eq!(code, FfiErrorCode::NullPointer.as_i32());
}
