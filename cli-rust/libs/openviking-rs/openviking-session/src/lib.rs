// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Session management, memory extraction, and retrieval for OpenViking.
//!
//! This crate defines the async trait abstractions for IO injection
//! (LLM, vector store, embedder, file system) and will host the
//! business logic orchestrators in future iterations.

#![deny(clippy::clone_on_ref_ptr)]
#![warn(missing_docs)]

pub mod collection_schemas;
pub mod compressor;
pub mod deduplicator;
pub mod extractor;
pub mod intent;
pub mod json_utils;
pub mod memory_vector_store;
pub mod prompts;
pub mod retriever;
pub mod session;
pub mod traits;

#[cfg(test)]
mod memory_vector_store_tests;
#[cfg(test)]
mod vector_store_tests;

// Re-exports for ergonomic access.
pub use collection_schemas::{init_context_collection, CollectionSchemas};
pub use memory_vector_store::InMemoryVectorStore;
pub use traits::{
    BoxError, CollectionInfo, CollectionSchema, DistanceMetric, EmbedResult, Embedder, FieldDef,
    FileSystem, FsEntry, FsStat, GrepMatch, LlmProvider, RerankResult, Reranker, ScrollResult,
    VectorHit, VectorStore, VectorStoreError,
};
