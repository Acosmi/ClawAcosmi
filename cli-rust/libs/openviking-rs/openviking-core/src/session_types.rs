// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Session-related data structures — pure data, zero IO.
//!
//! Ported from `openviking/session/*.py` data classes.

use serde::{Deserialize, Serialize};

// ---------------------------------------------------------------------------
// Session data structures (from session.py)
// ---------------------------------------------------------------------------

/// Session compression metadata.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct SessionCompression {
    /// Compression summary text.
    #[serde(default)]
    pub summary: String,
    /// Number of original turns before compression.
    #[serde(default)]
    pub original_count: u32,
    /// Number of compressed turns.
    #[serde(default)]
    pub compressed_count: u32,
    /// Index of latest compression checkpoint.
    #[serde(default)]
    pub compression_index: u32,
}

/// Session usage statistics.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct SessionStats {
    /// Total dialogue turns.
    pub total_turns: u32,
    /// Total token count (approximate).
    pub total_tokens: u64,
    /// Number of compression passes.
    pub compression_count: u32,
    /// Number of contexts referenced.
    pub contexts_used: u32,
    /// Number of skills invoked.
    pub skills_used: u32,
    /// Number of memories extracted.
    pub memories_extracted: u32,
}

/// A single usage record — tracks which context/skill was used.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Usage {
    /// URI of the used context or skill.
    pub uri: String,
    /// Type label (`"context"` / `"skill"`).
    #[serde(rename = "type")]
    pub usage_type: String,
    /// Contribution score (0.0–1.0).
    #[serde(default)]
    pub contribution: f64,
    /// Input text summary.
    #[serde(default)]
    pub input: String,
    /// Output text summary.
    #[serde(default)]
    pub output: String,
    /// Whether the usage was successful.
    #[serde(default = "default_true")]
    pub success: bool,
    /// ISO 8601 timestamp.
    #[serde(default)]
    pub timestamp: String,
}

fn default_true() -> bool {
    true
}

// ---------------------------------------------------------------------------
// Memory extraction types (from memory_extractor.py)
// ---------------------------------------------------------------------------

/// Memory category enumeration (6 categories).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum MemoryCategory {
    /// User profile information.
    Profile,
    /// User preferences.
    Preferences,
    /// Entity memories (people, projects, concepts).
    Entities,
    /// Historical event records.
    Events,
    /// Agent case studies.
    Cases,
    /// Agent interaction patterns.
    Patterns,
}

impl MemoryCategory {
    /// Categories that always merge (never create duplicates).
    pub const ALWAYS_MERGE: &'static [Self] = &[Self::Profile];

    /// Categories that support merge decisions.
    pub const MERGE_SUPPORTED: &'static [Self] =
        &[Self::Preferences, Self::Entities, Self::Patterns];
}

/// Candidate memory extracted from a session.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CandidateMemory {
    /// Memory category.
    pub category: MemoryCategory,
    /// L0 abstract.
    #[serde(rename = "abstract")]
    pub abstract_text: String,
    /// L1 overview.
    pub overview: String,
    /// L2 full content.
    pub content: String,
    /// Source session ID.
    pub source_session: String,
    /// User identifier string.
    pub user: String,
    /// Output language hint.
    #[serde(default = "default_auto")]
    pub language: String,
}

fn default_auto() -> String {
    "auto".into()
}

/// Merged memory payload from LLM.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct MergedMemoryPayload {
    /// Merged L0 abstract.
    #[serde(rename = "abstract")]
    pub abstract_text: String,
    /// Merged L1 overview.
    pub overview: String,
    /// Merged L2 content.
    pub content: String,
    /// Merge reason.
    #[serde(default)]
    pub reason: String,
}

// ---------------------------------------------------------------------------
// Deduplication types (from memory_deduplicator.py)
// ---------------------------------------------------------------------------

/// Deduplication decision types.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum DedupDecision {
    /// Skip — too similar to existing.
    Skip,
    /// Create — new unique memory.
    Create,
    /// No decision (fallback).
    None,
}

/// Memory action decision for existing memories.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum MemoryActionDecision {
    /// Merge candidate into existing.
    Merge,
    /// Delete existing memory.
    Delete,
}

/// Extraction round statistics.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ExtractionStats {
    /// Number of new memories created.
    pub created: u32,
    /// Number of memories merged.
    pub merged: u32,
    /// Number of memories deleted.
    pub deleted: u32,
    /// Number of candidates skipped.
    pub skipped: u32,
}

/// Decision for one existing memory during dedup.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ExistingMemoryAction {
    /// URI of the target memory.
    pub memory_uri: String,
    /// Merge or delete.
    pub decision: MemoryActionDecision,
    /// Reason for the decision.
    #[serde(default)]
    pub reason: String,
}

/// Result of a deduplication decision.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DedupResult {
    /// Top-level decision.
    pub decision: DedupDecision,
    /// The candidate memory.
    pub candidate: CandidateMemory,
    /// Similar existing memories found.
    pub similar_memories: Vec<String>,
    /// Per-memory actions (merge/delete).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub actions: Option<Vec<ExistingMemoryAction>>,
    /// Decision reason.
    #[serde(default)]
    pub reason: String,
}

/// URI prefix mapping for each memory category.
pub fn category_uri_prefix(cat: MemoryCategory) -> &'static str {
    match cat {
        MemoryCategory::Profile => "viking://user/memories/",
        MemoryCategory::Preferences => "viking://user/memories/preferences/",
        MemoryCategory::Entities => "viking://user/memories/entities/",
        MemoryCategory::Events => "viking://user/memories/events/",
        MemoryCategory::Cases => "viking://agent/memories/cases/",
        MemoryCategory::Patterns => "viking://agent/memories/patterns/",
    }
}

/// Category to directory path mapping.
pub fn category_dir(cat: MemoryCategory) -> &'static str {
    match cat {
        MemoryCategory::Profile => "memories/profile.md",
        MemoryCategory::Preferences => "memories/preferences",
        MemoryCategory::Entities => "memories/entities",
        MemoryCategory::Events => "memories/events",
        MemoryCategory::Cases => "memories/cases",
        MemoryCategory::Patterns => "memories/patterns",
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn session_compression_default() {
        let sc = SessionCompression::default();
        assert!(sc.summary.is_empty());
        assert_eq!(sc.original_count, 0);
    }

    #[test]
    fn memory_category_serde() {
        let cat = MemoryCategory::Preferences;
        let json = serde_json::to_string(&cat).unwrap();
        assert_eq!(json, "\"preferences\"");
        let restored: MemoryCategory = serde_json::from_str(&json).unwrap();
        assert_eq!(restored, cat);
    }

    #[test]
    fn candidate_memory_roundtrip() {
        let cm = CandidateMemory {
            category: MemoryCategory::Events,
            abstract_text: "Launch event".into(),
            overview: "Product launch in 2026".into(),
            content: "Full details...".into(),
            source_session: "session_123".into(),
            user: "default".into(),
            language: "en".into(),
        };
        let json = serde_json::to_string(&cm).unwrap();
        let restored: CandidateMemory = serde_json::from_str(&json).unwrap();
        assert_eq!(restored.category, MemoryCategory::Events);
        assert_eq!(restored.abstract_text, "Launch event");
    }

    #[test]
    fn dedup_decision_serde() {
        let d = DedupDecision::Create;
        let json = serde_json::to_string(&d).unwrap();
        assert_eq!(json, "\"create\"");
    }
}
