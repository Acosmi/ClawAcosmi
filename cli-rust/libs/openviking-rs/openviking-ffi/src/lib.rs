// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! C-ABI foreign function interface for the OpenViking memory library.
//!
//! This crate exposes the core Rust functionality as `extern "C"` functions
//! that can be called from Go (CGo), C, or any language supporting the C ABI.
//!
//! # Usage
//!
//! 1. Call [`ovk_runtime_init`] once at process start.
//! 2. Create handles via `ovk_*_new` functions.
//! 3. Operate on handles via `ovk_*` functions.
//! 4. Free handles via `ovk_*_free` functions.
//! 5. Call [`ovk_runtime_shutdown`] at process exit.
//!
//! # Error handling
//!
//! All functions return `i32` error codes defined in [`error::FfiErrorCode`].
//! The last error message can be retrieved via [`ovk_last_error`].

#![allow(clippy::missing_safety_doc)]

pub mod callbacks;
pub mod error;
pub mod init_api;
pub mod runtime;
pub mod session_api;
pub mod vector_api;
pub mod tokenizer_api;
pub mod vfs_api;
pub mod vfs_semantic_api;

#[cfg(test)]
mod ffi_tests;
