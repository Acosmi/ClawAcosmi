//! nexus-memfs — node types for the virtual filesystem.
//!
//! Defines `VfsNode` (file or directory), tiered content (L0/L1/L2),
//! and serialization formats for directory listing results.

use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;
use std::time::{SystemTime, UNIX_EPOCH};

// ---------------------------------------------------------------------------
// Content tier level
// ---------------------------------------------------------------------------

/// Context loading tier — mirrors OpenViking L0/L1/L2.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[repr(C)]
pub enum Tier {
    /// L0: Abstract (~100 tokens) — one-sentence summary for quick relevance check.
    L0 = 0,
    /// L1: Overview (~2k tokens) — structured overview for Agent planning.
    L1 = 1,
    /// L2: Details — full original content for deep reading.
    L2 = 2,
}

impl Tier {
    pub fn from_int(v: i32) -> Option<Self> {
        match v {
            0 => Some(Tier::L0),
            1 => Some(Tier::L1),
            2 => Some(Tier::L2),
            _ => None,
        }
    }
}

// ---------------------------------------------------------------------------
// Tiered content — L0 / L1 / L2
// ---------------------------------------------------------------------------

/// Holds the three tiers of content for a single memory entry.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct TieredContent {
    /// L0: one-sentence abstract (~100 tokens).
    pub l0_abstract: String,
    /// L1: structured overview (~2k tokens).
    pub l1_overview: String,
    /// L2: full original content (unlimited).
    pub l2_detail: String,
}

impl TieredContent {
    pub fn get(&self, tier: Tier) -> &str {
        match tier {
            Tier::L0 => &self.l0_abstract,
            Tier::L1 => &self.l1_overview,
            Tier::L2 => &self.l2_detail,
        }
    }
}

// ---------------------------------------------------------------------------
// VFS node types
// ---------------------------------------------------------------------------

/// A node in the virtual filesystem — either a directory or a file.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum VfsNode {
    Dir(DirNode),
    File(FileNode),
}

/// A directory node containing children keyed by name.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DirNode {
    /// Directory-level tiered content (`.abstract` / `.overview`).
    pub content: TieredContent,
    /// Children: name → node.
    pub children: BTreeMap<String, VfsNode>,
    /// Creation timestamp (Unix seconds).
    pub created_at: u64,
}

/// A file (leaf) node representing a single memory entry.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FileNode {
    /// Memory UUID (stored as string for C ABI simplicity).
    pub memory_id: String,
    /// Category: "decisions" | "facts" | "emotions" | "todos".
    pub category: String,
    /// Tiered content.
    pub content: TieredContent,
    /// Creation timestamp (Unix seconds).
    pub created_at: u64,
    /// Optional metadata as JSON map.
    pub metadata: Option<serde_json::Value>,
}

impl DirNode {
    pub fn new() -> Self {
        Self {
            content: TieredContent::default(),
            children: BTreeMap::new(),
            created_at: unix_now(),
        }
    }

    /// Create with pre-populated tiered content.
    pub fn with_content(l0: &str, l1: &str) -> Self {
        Self {
            content: TieredContent {
                l0_abstract: l0.to_string(),
                l1_overview: l1.to_string(),
                l2_detail: String::new(),
            },
            children: BTreeMap::new(),
            created_at: unix_now(),
        }
    }
}

impl FileNode {
    pub fn new(memory_id: &str, category: &str, l0: &str, l1: &str, l2: &str) -> Self {
        Self {
            memory_id: memory_id.to_string(),
            category: category.to_string(),
            content: TieredContent {
                l0_abstract: l0.to_string(),
                l1_overview: l1.to_string(),
                l2_detail: l2.to_string(),
            },
            created_at: unix_now(),
            metadata: None,
        }
    }
}

// ---------------------------------------------------------------------------
// Directory listing result — returned to Go as JSON
// ---------------------------------------------------------------------------

/// Result of listing a directory.
#[derive(Debug, Serialize, Deserialize)]
pub struct DirEntry {
    pub name: String,
    pub is_dir: bool,
    /// L0 abstract of the entry for quick scanning.
    pub l0_abstract: String,
    pub created_at: u64,
}

/// Search result entry.
#[derive(Debug, Serialize, Deserialize)]
pub struct SearchHit {
    pub path: String,
    pub memory_id: String,
    pub category: String,
    pub score: f64,
    pub l0_abstract: String,
}

// ---------------------------------------------------------------------------
// Search trace — visualized retrieval trajectory
// ---------------------------------------------------------------------------

/// A single step in the search trace — records visiting a directory or scoring a file.
#[derive(Debug, Serialize, Deserialize)]
pub struct TraceStep {
    /// Path relative to root, e.g. "permanent/decisions".
    pub path: String,
    /// Node type: "dir" or "file".
    pub node_type: String,
    /// Relevance score (0.0 for directories with no keyword match).
    pub score: f64,
    /// Number of children explored (only meaningful for directories).
    pub children_explored: usize,
    /// Whether this node was included in the final search results.
    pub matched: bool,
}

/// Complete search trace — returned to the caller for visualization.
#[derive(Debug, Serialize, Deserialize)]
pub struct SearchTrace {
    /// Original query string.
    pub query: String,
    /// Keywords extracted from the query.
    pub keywords: Vec<String>,
    /// Ordered list of trace steps (directories and files visited).
    pub steps: Vec<TraceStep>,
    /// Total directories visited during the search.
    pub total_dirs_visited: usize,
    /// Total files scored during the search.
    pub total_files_scored: usize,
    /// Final search results (top hits after scoring and truncation).
    pub hits: Vec<SearchHit>,
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

fn unix_now() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_tiered_content_get() {
        let tc = TieredContent {
            l0_abstract: "abstract".into(),
            l1_overview: "overview".into(),
            l2_detail: "detail".into(),
        };
        assert_eq!(tc.get(Tier::L0), "abstract");
        assert_eq!(tc.get(Tier::L1), "overview");
        assert_eq!(tc.get(Tier::L2), "detail");
    }

    #[test]
    fn test_tier_from_int() {
        assert_eq!(Tier::from_int(0), Some(Tier::L0));
        assert_eq!(Tier::from_int(1), Some(Tier::L1));
        assert_eq!(Tier::from_int(2), Some(Tier::L2));
        assert_eq!(Tier::from_int(3), None);
    }

    #[test]
    fn test_file_node_creation() {
        let f = FileNode::new("abc-123", "decisions", "摘要", "概述", "完整内容");
        assert_eq!(f.memory_id, "abc-123");
        assert_eq!(f.category, "decisions");
        assert_eq!(f.content.l0_abstract, "摘要");
        assert!(f.created_at > 0);
    }

    #[test]
    fn test_dir_node_creation() {
        let d = DirNode::with_content("目录摘要", "目录概述");
        assert_eq!(d.content.l0_abstract, "目录摘要");
        assert!(d.children.is_empty());
    }
}
