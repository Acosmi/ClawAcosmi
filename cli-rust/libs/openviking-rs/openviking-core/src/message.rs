// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Message and Part types — the dialogue atom of OpenViking.
//!
//! Ported from `openviking/message/message.py` + `openviking/message/part.py`.
//!
//! Key differences from the Python original:
//! - `Union[TextPart, ContextPart, ToolPart]` → `#[serde(tag = "type")]` enum.
//! - `Literal["user", "assistant"]` → `enum Role`.
//! - Hand-written `_part_to_dict` / `from_dict` → zero-cost serde derive.

use std::collections::HashMap;

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

// ---------------------------------------------------------------------------
// Part – tagged union
// ---------------------------------------------------------------------------

/// A single message component.
///
/// Uses serde's internally-tagged representation so that the JSON output
/// contains `"type": "text"` (or `"context"` / `"tool"`) as the discriminant,
/// matching the Python wire format exactly.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "lowercase")]
pub enum Part {
    /// Plain text content.
    Text {
        /// The text body.
        text: String,
    },

    /// A reference to a context node (memory / resource / skill).
    Context {
        /// Viking URI of the referenced context.
        uri: String,
        /// Semantic type of the context.
        #[serde(default = "default_context_type")]
        context_type: String,
        /// L0 abstract summary.
        #[serde(default, rename = "abstract")]
        abstract_text: String,
    },

    /// A tool invocation record.
    Tool {
        /// Unique tool call identifier.
        tool_id: String,
        /// Human-readable tool name.
        tool_name: String,
        /// Viking URI of the tool file within the session.
        tool_uri: String,
        /// Viking URI of the parent skill.
        #[serde(default)]
        skill_uri: String,
        /// Structured input parameters.
        #[serde(skip_serializing_if = "Option::is_none")]
        tool_input: Option<HashMap<String, serde_json::Value>>,
        /// Serialized output.
        #[serde(default)]
        tool_output: String,
        /// Execution status: pending | running | completed | error.
        #[serde(default = "default_tool_status")]
        tool_status: String,
    },
}

fn default_context_type() -> String {
    "memory".to_owned()
}

fn default_tool_status() -> String {
    "pending".to_owned()
}

// ---------------------------------------------------------------------------
// Convenience constructors
// ---------------------------------------------------------------------------

impl Part {
    /// Create a `Text` part.
    #[must_use]
    pub fn text(text: impl Into<String>) -> Self {
        Self::Text { text: text.into() }
    }

    /// Create a `Context` part.
    #[must_use]
    pub fn context(
        uri: impl Into<String>,
        context_type: impl Into<String>,
        abstract_text: impl Into<String>,
    ) -> Self {
        Self::Context {
            uri: uri.into(),
            context_type: context_type.into(),
            abstract_text: abstract_text.into(),
        }
    }

    /// Create a `Tool` part with minimal required fields.
    #[must_use]
    pub fn tool(
        tool_id: impl Into<String>,
        tool_name: impl Into<String>,
        tool_uri: impl Into<String>,
    ) -> Self {
        Self::Tool {
            tool_id: tool_id.into(),
            tool_name: tool_name.into(),
            tool_uri: tool_uri.into(),
            skill_uri: String::new(),
            tool_input: None,
            tool_output: String::new(),
            tool_status: "pending".to_owned(),
        }
    }
}

// ---------------------------------------------------------------------------
// Role
// ---------------------------------------------------------------------------

/// The role of the speaker in a dialogue turn.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Role {
    /// A human user.
    User,
    /// The AI assistant.
    Assistant,
}

// ---------------------------------------------------------------------------
// Message
// ---------------------------------------------------------------------------

/// A single dialogue turn consisting of a role and a sequence of parts.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Message {
    /// Unique message identifier (e.g. `"msg_<hex>"`).
    pub id: String,
    /// Speaker role.
    pub role: Role,
    /// Ordered list of message components.
    pub parts: Vec<Part>,
    /// Creation timestamp (UTC).
    pub created_at: DateTime<Utc>,
}

impl Message {
    /// Quick accessor: returns the text of the first `Text` part, or `""`.
    #[must_use]
    pub fn content(&self) -> &str {
        for p in &self.parts {
            if let Part::Text { text } = p {
                return text.as_str();
            }
        }
        ""
    }

    /// Create a user message with a single text part.
    #[must_use]
    pub fn create_user(content: impl Into<String>) -> Self {
        Self {
            id: format!("msg_{}", Uuid::new_v4().simple()),
            role: Role::User,
            parts: vec![Part::text(content)],
            created_at: Utc::now(),
        }
    }

    /// Create an assistant message with optional text, context refs, and tool
    /// calls.
    #[must_use]
    pub fn create_assistant(
        content: Option<String>,
        context_refs: Vec<Part>,
        tool_calls: Vec<Part>,
    ) -> Self {
        let mut parts = Vec::new();
        if let Some(text) = content {
            if !text.is_empty() {
                parts.push(Part::text(text));
            }
        }
        parts.extend(context_refs);
        parts.extend(tool_calls);

        Self {
            id: format!("msg_{}", Uuid::new_v4().simple()),
            role: Role::Assistant,
            parts,
            created_at: Utc::now(),
        }
    }

    /// Collect all `Context` parts.
    #[must_use]
    pub fn context_parts(&self) -> Vec<&Part> {
        self.parts
            .iter()
            .filter(|p| matches!(p, Part::Context { .. }))
            .collect()
    }

    /// Collect all `Tool` parts.
    #[must_use]
    pub fn tool_parts(&self) -> Vec<&Part> {
        self.parts
            .iter()
            .filter(|p| matches!(p, Part::Tool { .. }))
            .collect()
    }

    /// Find a `Tool` part by its `tool_id`.
    #[must_use]
    pub fn find_tool(&self, target_id: &str) -> Option<&Part> {
        self.parts
            .iter()
            .find(|p| matches!(p, Part::Tool { tool_id, .. } if tool_id == target_id))
    }

    /// Serialize to a single JSON line (JSONL format).
    ///
    /// # Errors
    /// Returns `serde_json::Error` if serialization fails.
    pub fn to_jsonl(&self) -> Result<String, serde_json::Error> {
        serde_json::to_string(self)
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use pretty_assertions::assert_eq;

    #[test]
    fn part_text_serde() {
        let part = Part::text("hello world");
        let json = serde_json::to_value(&part).unwrap();
        assert_eq!(json["type"], "text");
        assert_eq!(json["text"], "hello world");
    }

    #[test]
    fn part_context_serde() {
        let part = Part::context(
            "viking://user/memories/preferences/code-style",
            "memory",
            "Prefers Rust",
        );
        let json = serde_json::to_value(&part).unwrap();
        assert_eq!(json["type"], "context");
        assert_eq!(json["uri"], "viking://user/memories/preferences/code-style");
    }

    #[test]
    fn part_tool_serde() {
        let part = Part::tool("t1", "search", "viking://session/s1/tools/t1");
        let json = serde_json::to_value(&part).unwrap();
        assert_eq!(json["type"], "tool");
        assert_eq!(json["tool_status"], "pending");
    }

    #[test]
    fn part_roundtrip() {
        let parts = vec![
            Part::text("hi"),
            Part::context(
                "viking://user/memories/entities/proj",
                "memory",
                "Project A",
            ),
            Part::tool("t2", "exec", "viking://session/s1/tools/t2"),
        ];

        for part in &parts {
            let json = serde_json::to_string(part).unwrap();
            let restored: Part = serde_json::from_str(&json).unwrap();
            assert_eq!(&restored, part);
        }
    }

    #[test]
    fn message_create_user() {
        let msg = Message::create_user("Hello, agent!");
        assert_eq!(msg.role, Role::User);
        assert_eq!(msg.content(), "Hello, agent!");
        assert!(msg.id.starts_with("msg_"));
    }

    #[test]
    fn message_create_assistant() {
        let ctx = Part::context(
            "viking://agent/memories/cases/debug",
            "memory",
            "Debug case",
        );
        let tool = Part::tool("t3", "read_file", "viking://session/s1/tools/t3");

        let msg = Message::create_assistant(
            Some("Here's what I found.".to_owned()),
            vec![ctx],
            vec![tool],
        );

        assert_eq!(msg.role, Role::Assistant);
        assert_eq!(msg.content(), "Here's what I found.");
        assert_eq!(msg.context_parts().len(), 1);
        assert_eq!(msg.tool_parts().len(), 1);
    }

    #[test]
    fn message_jsonl_roundtrip() {
        let msg = Message::create_user("Test JSONL");
        let jsonl = msg.to_jsonl().unwrap();
        let restored: Message = serde_json::from_str(&jsonl).unwrap();
        assert_eq!(restored.id, msg.id);
        assert_eq!(restored.content(), "Test JSONL");
    }

    #[test]
    fn message_find_tool() {
        let msg = Message::create_assistant(
            None,
            vec![],
            vec![
                Part::tool("t1", "read", "viking://session/s/tools/t1"),
                Part::tool("t2", "write", "viking://session/s/tools/t2"),
            ],
        );

        assert!(msg.find_tool("t2").is_some());
        assert!(msg.find_tool("t999").is_none());
    }
}
