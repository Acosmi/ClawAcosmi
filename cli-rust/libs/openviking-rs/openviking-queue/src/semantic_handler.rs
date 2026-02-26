// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Semantic processing handler trait.
//!
//! Ported from `openviking/storage/queuefs/semantic_processor.py`.
//!
//! The Python `SemanticProcessor` (556 lines) performs LLM-heavy operations
//! such as generating `.abstract.md` and `.overview.md` files. Per the
//! "zero IO / zero LLM" library principle, this Rust port defines only the
//! **trait interface** that consumers must implement.
//!
//! Concrete LLM logic lives in the application layer, not in this library.

use async_trait::async_trait;
use serde_json::Value;

use crate::semantic_msg::SemanticMsg;

/// Error type for semantic processing operations.
pub type SemanticError = Box<dyn std::error::Error + Send + Sync>;

/// Trait for processing semantic extraction messages.
///
/// Implementations of this trait handle the actual LLM-related work:
/// - Generating file summaries
/// - Creating `.abstract.md` and `.overview.md` for directories
/// - Enqueuing results for vectorization
///
/// # Example
///
/// ```ignore
/// struct MySemanticProcessor { /* llm client, config, etc. */ }
///
/// #[async_trait]
/// impl SemanticHandler for MySemanticProcessor {
///     async fn process_directory(&self, msg: &SemanticMsg) -> Result<(), SemanticError> {
///         // 1. List files in msg.uri
///         // 2. Generate summaries via LLM
///         // 3. Write .abstract.md and .overview.md
///         // 4. Enqueue for embedding
///         Ok(())
///     }
/// }
/// ```
#[async_trait]
pub trait SemanticHandler: Send + Sync {
    /// Process a semantic extraction message for a single directory.
    ///
    /// This is the core method that implementations must provide.
    /// It corresponds to Python `SemanticProcessor._process_single_directory`.
    async fn process_directory(&self, msg: &SemanticMsg) -> Result<(), SemanticError>;

    /// Called when a message is dequeued from the semantic queue.
    ///
    /// Default implementation deserializes the JSON data into a [`SemanticMsg`]
    /// and delegates to [`process_directory`](Self::process_directory).
    /// Override for custom pre/post-processing.
    async fn on_dequeue(&self, data: Option<Value>) -> Result<Option<Value>, SemanticError> {
        let Some(data) = data else {
            return Ok(None);
        };

        let msg: SemanticMsg = serde_json::from_value(data)
            .map_err(|e| format!("Invalid SemanticMsg: {e}"))?;

        self.process_directory(&msg).await?;
        Ok(None)
    }

    /// Get processing statistics (optional).
    ///
    /// Implementations may return structured stats about processed
    /// directories, errors, etc.
    fn get_stats(&self) -> Option<Value> {
        None
    }
}
