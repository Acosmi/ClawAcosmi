// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Semantic extraction queue message type.
//!
//! Ported from `openviking/storage/queuefs/semantic_msg.py`.

use serde::{Deserialize, Serialize};
use uuid::Uuid;

/// Processing status for a semantic extraction message.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum SemanticMsgStatus {
    /// Waiting to be processed.
    #[default]
    Pending,
    /// Currently being processed.
    Processing,
    /// Processing complete.
    Completed,
}

impl std::fmt::Display for SemanticMsgStatus {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Pending => f.write_str("pending"),
            Self::Processing => f.write_str("processing"),
            Self::Completed => f.write_str("completed"),
        }
    }
}

/// A message requesting semantic extraction for a directory.
///
/// When dequeued, a `SemanticHandler` implementation generates
/// `.abstract.md` and `.overview.md` for the target URI.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SemanticMsg {
    /// Unique message ID (UUID v4).
    pub id: String,
    /// Directory URI to process.
    pub uri: String,
    /// Type of context (resource, memory, skill).
    pub context_type: String,
    /// Processing status.
    #[serde(default)]
    pub status: SemanticMsgStatus,
    /// Creation timestamp (Unix seconds).
    pub timestamp: i64,
    /// Whether to recursively process subdirectories.
    #[serde(default = "default_recursive")]
    pub recursive: bool,
}

fn default_recursive() -> bool {
    true
}

impl SemanticMsg {
    /// Create a new `SemanticMsg` with a fresh UUID.
    pub fn new(uri: impl Into<String>, context_type: impl Into<String>, recursive: bool) -> Self {
        Self {
            id: Uuid::new_v4().to_string(),
            uri: uri.into(),
            context_type: context_type.into(),
            status: SemanticMsgStatus::Pending,
            timestamp: chrono::Utc::now().timestamp(),
            recursive,
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
