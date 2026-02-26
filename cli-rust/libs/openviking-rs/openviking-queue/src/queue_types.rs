// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Queue status tracking and extension traits.
//!
//! Ported from the first 90 lines of
//! `openviking/storage/queuefs/named_queue.py`.



use async_trait::async_trait;
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

// ---------------------------------------------------------------------------
// QueueError / QueueStatus
// ---------------------------------------------------------------------------

/// A single recorded queue processing error.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct QueueError {
    /// When the error occurred.
    pub timestamp: DateTime<Utc>,
    /// Human-readable error message.
    pub message: String,
    /// Optional data that caused the error.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub data: Option<serde_json::Value>,
}

/// Aggregate status of a named queue.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct QueueStatus {
    /// Messages waiting to be processed.
    pub pending: usize,
    /// Messages currently being processed.
    pub in_progress: usize,
    /// Successfully processed messages.
    pub processed: usize,
    /// Total error count.
    pub error_count: usize,
    /// Recent error records.
    #[serde(default)]
    pub errors: Vec<QueueError>,
}

impl QueueStatus {
    /// Returns `true` if any processing errors have been recorded.
    pub fn has_errors(&self) -> bool {
        self.error_count > 0
    }

    /// Returns `true` if no messages are pending or in progress.
    pub fn is_complete(&self) -> bool {
        self.pending == 0 && self.in_progress == 0
    }
}

// ---------------------------------------------------------------------------
// Extension traits
// ---------------------------------------------------------------------------

/// Hook called before a message is enqueued.
///
/// Implementations can transform, validate, or enrich the data.
#[async_trait]
pub trait EnqueueHook: Send + Sync {
    /// Process data before enqueue. Return the (possibly modified) data.
    async fn on_enqueue(
        &self,
        data: serde_json::Value,
    ) -> Result<serde_json::Value, Box<dyn std::error::Error + Send + Sync>>;
}

/// Handler called after a message is dequeued.
///
/// Implementations perform the actual processing (e.g. embedding, semantic
/// extraction) and report success or error via the provided callbacks.
#[async_trait]
pub trait DequeueHandler: Send + Sync {
    /// Process dequeued data. Return `None` to discard the message.
    async fn on_dequeue(
        &self,
        data: Option<serde_json::Value>,
    ) -> Result<Option<serde_json::Value>, Box<dyn std::error::Error + Send + Sync>>;
}
