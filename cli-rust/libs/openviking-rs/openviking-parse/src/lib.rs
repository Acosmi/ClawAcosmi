// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Document parsing types, registry, and tree builder for OpenViking.
//!
//! This crate provides:
//! - [`ResourceNode`] / [`ParseResult`] — core document parsing types.
//! - [`NodeType`] / [`ResourceCategory`] / [`DocumentType`] / [`MediaType`] — type enums.
//! - [`Parser`] / [`CustomParser`] — async trait abstractions for document parsers.
//! - [`ParserRegistry`] — extension-based parser lookup and management.
//! - [`TreeBuilder`] — temp directory → AGFS migration with semantic queue.
//!
//! # Design Principles
//! - **Zero IO**: Parsing types are pure data structures; all IO is injected.
//! - **Serde-first**: All public types derive `Serialize` / `Deserialize`.
//! - **Extension via traits**: Custom parsers implement `Parser` or `CustomParser`.

#![deny(clippy::clone_on_ref_ptr)]
#![warn(missing_docs)]

pub mod registry;
pub mod tree_builder;
pub mod types;

#[cfg(test)]
mod registry_tests;
#[cfg(test)]
mod tree_builder_tests;
#[cfg(test)]
mod types_tests;

// Re-exports for ergonomic access.
pub use registry::ParserRegistry;
pub use tree_builder::TreeBuilder;
pub use types::{
    calculate_media_strategy, create_parse_result, format_table_to_markdown, CustomParser,
    DocumentType, MediaStrategy, MediaType, NodeType, ParseResult, Parser, ResourceCategory,
    ResourceNode,
};
