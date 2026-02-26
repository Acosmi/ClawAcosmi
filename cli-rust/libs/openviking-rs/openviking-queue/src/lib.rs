// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Async embedding queue and semantic processing for OpenViking.
//!
//! This crate provides:
//! - [`EmbeddingMsg`] / [`SemanticMsg`] — typed queue messages.
//! - [`QueueStatus`] / [`QueueError`] — queue health tracking.
//! - `trait EnqueueHook` / `trait DequeueHandler` — extension points.
//! - [`NamedQueue`] — persistent named queue backed by `trait FileSystem`.
//! - [`EmbeddingMsgConverter`] — Context → EmbeddingMsg conversion.
//! - [`QueueManager`] — multi-queue lifecycle manager.

pub mod converter;
pub mod embedding_msg;
pub mod named_queue;
pub mod queue_manager;
pub mod queue_types;
pub mod semantic_handler;
pub mod semantic_msg;

#[cfg(test)]
mod queue_tests;

// Public re-exports.
pub use converter::EmbeddingMsgConverter;
pub use embedding_msg::EmbeddingMsg;
pub use named_queue::NamedQueue;
pub use queue_manager::QueueManager;
pub use queue_types::{QueueError, QueueStatus};
pub use semantic_handler::SemanticHandler;
pub use semantic_msg::{SemanticMsg, SemanticMsgStatus};
