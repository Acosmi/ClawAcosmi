// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Path-level transaction locking for OpenViking memory operations.
//!
//! This crate provides:
//! - [`TransactionRecord`] — tracks a single transaction's lifecycle and locks.
//! - [`PathLock`] — file-system-based path-level locking (normal / rm / mv).
//! - [`TransactionManager`] — manages transaction creation, commit, rollback,
//!   and background timeout cleanup.
//!
//! All file-system operations are delegated to the injected
//! [`FileSystem`](openviking_session::traits::FileSystem) trait, keeping this
//! crate fully decoupled from any concrete storage backend.

pub mod path_lock;
pub mod transaction_manager;
pub mod transaction_record;

#[cfg(test)]
mod transaction_tests;

// Public re-exports.
pub use path_lock::PathLock;
pub use transaction_manager::TransactionManager;
pub use transaction_record::{TransactionRecord, TransactionStatus};
