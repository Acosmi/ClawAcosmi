// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Relation table entry — inter-context links.
//!
//! Ported from `RelationEntry` in `openviking/storage/viking_fs.py`.

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

/// A single entry in a context's `.relations.json` table.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct RelationEntry {
    /// Link identifier (e.g. `"link_1"`).
    pub id: String,
    /// Target URIs this relation points to.
    pub uris: Vec<String>,
    /// Human-readable reason for the link.
    #[serde(default)]
    pub reason: String,
    /// Creation timestamp (ISO 8601).
    pub created_at: DateTime<Utc>,
}

impl RelationEntry {
    /// Create a new relation entry with the current timestamp.
    #[must_use]
    pub fn new(id: impl Into<String>, uris: Vec<String>, reason: impl Into<String>) -> Self {
        Self {
            id: id.into(),
            uris,
            reason: reason.into(),
            created_at: Utc::now(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn serde_roundtrip() {
        let entry = RelationEntry::new(
            "link_1",
            vec!["viking://user/memories/events/launch".into()],
            "related event",
        );
        let json = serde_json::to_string(&entry).unwrap();
        let restored: RelationEntry = serde_json::from_str(&json).unwrap();
        assert_eq!(restored.id, "link_1");
        assert_eq!(restored.uris.len(), 1);
        assert_eq!(restored.reason, "related event");
    }

    #[test]
    fn deserialize_list() {
        let json = r#"[
            {"id":"link_1","uris":["viking://a"],"reason":"test","created_at":"2026-01-01T00:00:00Z"},
            {"id":"link_2","uris":["viking://b","viking://c"],"reason":"","created_at":"2026-01-02T00:00:00Z"}
        ]"#;
        let entries: Vec<RelationEntry> = serde_json::from_str(json).unwrap();
        assert_eq!(entries.len(), 2);
        assert_eq!(entries[1].uris.len(), 2);
    }
}
