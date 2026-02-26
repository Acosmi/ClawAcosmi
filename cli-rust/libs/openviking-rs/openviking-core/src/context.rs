// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Unified context node — the atom of OpenViking's memory graph.
//!
//! Ported from `openviking/core/context.py`.
//!
//! Key differences from the Python original:
//! - `Dict[str, Any]` magic → strong `serde` derive (zero hand-written ser/de).
//! - `datetime` → `chrono::DateTime<Utc>`.
//! - `vector: Optional[List[float]]` → `Option<Box<[f32]>>` (no excess capacity).

use std::collections::HashMap;

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

use crate::user::UserIdentifier;

// ---------------------------------------------------------------------------
// Enums
// ---------------------------------------------------------------------------

/// Multi-modal resource content type.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ResourceContentType {
    /// Plain text content.
    Text,
    /// Image content.
    Image,
    /// Video content.
    Video,
    /// Audio content.
    Audio,
    /// Arbitrary binary blob.
    Binary,
}

/// Semantic type of a context node inside the Viking URI namespace.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ContextType {
    /// An agent skill entry.
    Skill,
    /// A memory record (user or agent).
    Memory,
    /// A generic resource.
    Resource,
}

impl ContextType {
    /// Derive `ContextType` from a Viking URI string.
    ///
    /// Rules (matching the Python `_derive_context_type`):
    /// - URI starts with `"viking://agent/skills"` → Skill
    /// - URI contains `"memories"` → Memory
    /// - Otherwise → Resource
    #[must_use]
    pub fn from_uri(uri: &str) -> Self {
        if uri.starts_with("viking://agent/skills") {
            Self::Skill
        } else if uri.contains("memories") {
            Self::Memory
        } else {
            Self::Resource
        }
    }
}

/// Category tag derived from URI structure.
///
/// Replaces the dynamic string returns of `_derive_category()` in Python.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ContextCategory {
    /// User preference memories.
    Preferences,
    /// Entity memories (people, projects, concepts).
    Entities,
    /// Historical event records.
    Events,
    /// Agent interaction patterns.
    Patterns,
    /// Agent case studies.
    Cases,
    /// User profile information.
    Profile,
    /// No specific category.
    None,
}

impl ContextCategory {
    /// Derive category from a Viking URI (port of `_derive_category()`).
    #[must_use]
    pub fn from_uri(uri: &str) -> Self {
        if uri.starts_with("viking://agent/memories") {
            if uri.contains("patterns") {
                return Self::Patterns;
            }
            if uri.contains("cases") {
                return Self::Cases;
            }
        } else if uri.starts_with("viking://user/memories") {
            if uri.contains("profile") {
                return Self::Profile;
            }
            if uri.contains("preferences") {
                return Self::Preferences;
            }
            if uri.contains("entities") {
                return Self::Entities;
            }
            if uri.contains("events") {
                return Self::Events;
            }
        }
        Self::None
    }
}

// ---------------------------------------------------------------------------
// Vectorize helper
// ---------------------------------------------------------------------------

/// Holder for vectorization source data.
///
/// Currently text-only; multi-modal fields (image / audio / video) are reserved
/// for future expansion.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct Vectorize {
    /// The text that should be embedded into a vector.
    pub text: String,
}

impl Vectorize {
    /// Create a new `Vectorize` with the given text.
    #[must_use]
    pub fn new(text: impl Into<String>) -> Self {
        Self { text: text.into() }
    }
}

// ---------------------------------------------------------------------------
// Context
// ---------------------------------------------------------------------------

/// The unified context node — every piece of data in OpenViking's memory graph
/// is represented as a `Context`.
///
/// # Ownership Model
/// - All `String` fields are **owned**. Transfer happens at construction time;
///   thereafter the struct is freely movable / storable without lifetime
///   entanglement.
/// - `vector` uses `Box<[f32]>` to avoid the 8-byte capacity overhead of
///   `Vec<f32>` since vectors are never resized after initial embedding.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Context {
    /// Unique identifier (defaults to UUID v4 string).
    pub id: String,

    /// Viking URI, e.g. `"viking://user/memories/preferences/coding-style"`.
    pub uri: String,

    /// Parent URI for tree traversal.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub parent_uri: Option<String>,

    /// Whether this node is a leaf (contains data) rather than a directory.
    pub is_leaf: bool,

    /// L0 abstract — short summary of the context content.
    #[serde(rename = "abstract")]
    pub abstract_text: String,

    /// Semantic type (skill / memory / resource).
    pub context_type: ContextType,

    /// Finer-grained category within the type.
    pub category: ContextCategory,

    /// Creation timestamp (UTC).
    pub created_at: DateTime<Utc>,

    /// Last-update timestamp (UTC).
    pub updated_at: DateTime<Utc>,

    /// How many times this context has been actively referenced.
    pub active_count: u32,

    /// Related URI links.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub related_uri: Vec<String>,

    /// Arbitrary key-value metadata.
    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    pub meta: HashMap<String, serde_json::Value>,

    /// Session ID this context belongs to (if any).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub session_id: Option<String>,

    /// Owner identity (multi-tenant).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub user: Option<UserIdentifier>,

    /// Embedding vector (set after vectorization).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub vector: Option<Box<[f32]>>,

    /// Source data for vectorization.
    #[serde(skip)]
    pub vectorize: Vectorize,
}

impl Context {
    /// Create a new `Context` with automatic type / category derivation.
    #[must_use]
    pub fn new(uri: impl Into<String>, abstract_text: impl Into<String>) -> Self {
        let uri = uri.into();
        let abstract_text = abstract_text.into();
        let context_type = ContextType::from_uri(&uri);
        let category = ContextCategory::from_uri(&uri);
        let now = Utc::now();

        Self {
            id: Uuid::new_v4().to_string(),
            uri,
            parent_uri: None,
            is_leaf: false,
            abstract_text: abstract_text.clone(),
            context_type,
            category,
            created_at: now,
            updated_at: now,
            active_count: 0,
            related_uri: Vec::new(),
            meta: HashMap::new(),
            session_id: None,
            user: None,
            vector: None,
            vectorize: Vectorize::new(abstract_text),
        }
    }

    /// Set the parent URI (builder pattern).
    #[must_use]
    pub fn with_parent(mut self, parent: impl Into<String>) -> Self {
        self.parent_uri = Some(parent.into());
        self
    }

    /// Mark this context as a leaf node.
    #[must_use]
    pub fn as_leaf(mut self) -> Self {
        self.is_leaf = true;
        self
    }

    /// Override the vectorization source text.
    pub fn set_vectorize(&mut self, vectorize: Vectorize) {
        self.vectorize = vectorize;
    }

    /// Get vectorization text (currently text-only).
    #[must_use]
    pub fn vectorization_text(&self) -> &str {
        &self.vectorize.text
    }

    /// Bump the activity counter and refresh `updated_at`.
    pub fn update_activity(&mut self) {
        self.active_count += 1;
        self.updated_at = Utc::now();
    }

    /// Serialize to a JSON value matching Python's `to_dict()` output.
    ///
    /// This adds skill-specific top-level fields (`name`, `description`)
    /// extracted from `meta` when `context_type == Skill`, mirroring the
    /// Python original's behavior.
    #[must_use]
    pub fn to_storage_value(&self) -> serde_json::Value {
        let mut val = serde_json::to_value(self).unwrap_or_default();
        if self.context_type == ContextType::Skill {
            if let serde_json::Value::Object(ref mut map) = val {
                let name = self
                    .meta
                    .get("name")
                    .and_then(|v| v.as_str())
                    .unwrap_or("")
                    .to_owned();
                let desc = self
                    .meta
                    .get("description")
                    .and_then(|v| v.as_str())
                    .unwrap_or("")
                    .to_owned();
                map.insert("name".into(), serde_json::Value::String(name));
                map.insert("description".into(), serde_json::Value::String(desc));
            }
        }
        val
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn context_type_from_uri() {
        assert_eq!(
            ContextType::from_uri("viking://agent/skills/search"),
            ContextType::Skill
        );
        assert_eq!(
            ContextType::from_uri("viking://user/memories/preferences/code-style"),
            ContextType::Memory
        );
        assert_eq!(
            ContextType::from_uri("viking://resources/docs"),
            ContextType::Resource
        );
    }

    #[test]
    fn category_from_uri() {
        assert_eq!(
            ContextCategory::from_uri("viking://user/memories/preferences/x"),
            ContextCategory::Preferences
        );
        assert_eq!(
            ContextCategory::from_uri("viking://user/memories/entities/project-a"),
            ContextCategory::Entities
        );
        assert_eq!(
            ContextCategory::from_uri("viking://agent/memories/patterns/debug"),
            ContextCategory::Patterns
        );
        assert_eq!(
            ContextCategory::from_uri("viking://session/abc123"),
            ContextCategory::None
        );
    }

    #[test]
    fn context_serde_roundtrip() {
        let ctx = Context::new(
            "viking://user/memories/preferences/code-style",
            "Prefers Rust with strict ownership",
        );

        let json = serde_json::to_string_pretty(&ctx).expect("serialize");
        let restored: Context = serde_json::from_str(&json).expect("deserialize");

        assert_eq!(restored.uri, ctx.uri);
        assert_eq!(restored.abstract_text, ctx.abstract_text);
        assert_eq!(restored.context_type, ContextType::Memory);
        assert_eq!(restored.category, ContextCategory::Preferences);
        assert!(!restored.is_leaf);
    }

    #[test]
    fn context_builder_pattern() {
        let ctx = Context::new(
            "viking://user/memories/events/launch",
            "Product launch event",
        )
        .with_parent("viking://user/memories/events")
        .as_leaf();

        assert_eq!(
            ctx.parent_uri.as_deref(),
            Some("viking://user/memories/events")
        );
        assert!(ctx.is_leaf);
    }

    #[test]
    fn update_activity_increments() {
        let mut ctx = Context::new("viking://resources/docs/readme", "README file");
        assert_eq!(ctx.active_count, 0);
        let t0 = ctx.updated_at;

        ctx.update_activity();
        assert_eq!(ctx.active_count, 1);
        assert!(ctx.updated_at >= t0);
    }
}
