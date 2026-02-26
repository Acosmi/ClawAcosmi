// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Core data structures for the OpenViking memory system.
//!
//! This crate provides the foundational types used throughout the OpenViking
//! ecosystem: context nodes, message parts, directory definitions, and the
//! building tree container.
//!
//! # Design Principles
//! - **Zero IO**: This crate never performs file I/O, network calls, or LLM
//!   invocations. All external capabilities are injected via traits defined in
//!   higher-level crates.
//! - **Memory Safety**: Large text fields use owned `String` with explicit
//!   ownership transfer; vectors use `Option<Box<[f32]>>` to avoid excess
//!   capacity overhead.
//! - **Serde-first**: All public types derive `Serialize` / `Deserialize`,
//!   eliminating hand-written `to_dict` / `from_dict` marshalling.

#![deny(clippy::clone_on_ref_ptr)]
#![warn(missing_docs)]

pub mod context;
pub mod directory;
pub mod math_utils;
pub mod message;
pub mod relation;
pub mod retrieve_types;
pub mod session_types;
pub mod skill_loader;
pub mod tree;
pub mod uri;
pub mod user;

// Re-exports for ergonomic top-level access.
pub use context::{Context, ContextType, ResourceContentType, Vectorize};
pub use directory::{preset_directories, DirectoryDefinition};
pub use math_utils::{cosine_similarity, extract_facet_key};
pub use message::{Message, Part, Role};
pub use relation::RelationEntry;
pub use skill_loader::{SkillDefinition, SkillLoader};
pub use tree::BuildingTree;
pub use uri::{Scope, UriError, VikingUri};
pub use user::UserIdentifier;
