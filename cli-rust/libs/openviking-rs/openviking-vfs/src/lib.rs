// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! VikingFS — high-level file system abstraction for OpenViking memory.
//!
//! This crate provides [`VikingFs`], a composition layer that wraps the
//! low-level [`FileSystem`] trait with higher-level capabilities:
//! - URI-aware file management (read/write/mkdir/rm/mv)
//! - L0/L1 context layer reading (`.abstract.md`, `.overview.md`)
//! - Relation management (`.relations.json`)
//! - Semantic search (via `VectorStore` + `Embedder`)
//! - Vector store synchronization on file operations
//! - Context writing (L0/L1/L2)
//! - Preset directory initialization
//!
//! # Design Principles
//! - **Composition over inheritance**: `VikingFs` composes `FileSystem`,
//!   `VectorStore`, and `Embedder` via generics.
//! - **Zero hard-coded backends**: All I/O delegated to trait implementors.
//! - **Serde-first**: All public data types derive `Serialize`/`Deserialize`.

#![deny(clippy::clone_on_ref_ptr)]
#![warn(missing_docs)]

pub mod directory_init;
mod local_fs;
pub mod observer;
mod viking_fs;
pub mod vikingdb_manager;

#[cfg(test)]
mod local_fs_tests;
#[cfg(test)]
mod vfs_tests;

pub use directory_init::DirectoryInitializer;
pub use local_fs::LocalFs;
pub use observer::{Observer, QueueObserver, TransactionObserver, VectorStoreObserver};
pub use viking_fs::VikingFs;
pub use vikingdb_manager::VikingDBManager;
