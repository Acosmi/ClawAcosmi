// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Embedding message type for async vector processing.
//!
//! Ported from `openviking/storage/queuefs/embedding_msg.py`.

use serde::{Deserialize, Serialize};
use uuid::Uuid;

/// Content to be embedded — either a single text or multiple structured items.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(untagged)]
pub enum EmbeddingContent {
    /// A single text string to embed.
    Text(String),
    /// Multiple structured items (e.g. multi-modal parts).
    Items(Vec<serde_json::Value>),
}

/// A message queued for asynchronous embedding.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EmbeddingMsg {
    /// Unique message ID (UUID v4).
    pub id: String,
    /// Content to be embedded.
    pub message: EmbeddingContent,
    /// Full context data for the source `Context` (serialized to JSON value).
    pub context_data: serde_json::Value,
}

impl EmbeddingMsg {
    /// Create a new `EmbeddingMsg` with a fresh UUID.
    pub fn new(message: EmbeddingContent, context_data: serde_json::Value) -> Self {
        Self {
            id: Uuid::new_v4().to_string(),
            message,
            context_data,
        }
    }

    /// Serialize to JSON string.
    pub fn to_json(&self) -> Result<String, serde_json::Error> {
        serde_json::to_string(self)
    }

    /// Deserialize from JSON string.
    pub fn from_json(json: &str) -> Result<Self, serde_json::Error> {
        serde_json::from_str(json)
    }
}
