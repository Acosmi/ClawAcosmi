// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! BuildingTree — an indexed container for context trees.
//!
//! Ported from `openviking/core/building_tree.py`.
//!
//! Key improvements over the Python original:
//! - `get_children()` is O(1) via a pre-built parent→children index
//!   (Python's implementation was O(n) linear scan).
//! - All lookups are HashMap-backed for constant-time access.

use std::collections::HashMap;

use crate::context::Context;

// ---------------------------------------------------------------------------
// BuildingTree
// ---------------------------------------------------------------------------

/// In-memory indexed container for a tree of [`Context`] nodes.
///
/// Maintains:
/// - Ordered list of all contexts (insertion order).
/// - URI → Context map for O(1) lookup.
/// - Parent URI → child URIs index for O(1) children enumeration.
#[derive(Debug, Clone)]
pub struct BuildingTree {
    /// Optional provenance metadata.
    pub source_path: Option<String>,
    /// Optional format descriptor (e.g. `"markdown"`, `"jsonl"`).
    pub source_format: Option<String>,

    /// Ordered list of all contexts (insertion order preserved).
    contexts: Vec<Context>,
    /// URI → index into `contexts`.
    uri_index: HashMap<String, usize>,
    /// Parent URI → list of child URIs. Built incrementally on `add_context`.
    children_index: HashMap<String, Vec<String>>,
    /// URI of the root context (if set).
    root_uri: Option<String>,
}

impl BuildingTree {
    /// Create an empty tree.
    #[must_use]
    pub fn new() -> Self {
        Self {
            source_path: None,
            source_format: None,
            contexts: Vec::new(),
            uri_index: HashMap::new(),
            children_index: HashMap::new(),
            root_uri: None,
        }
    }

    /// Create an empty tree with provenance metadata.
    #[must_use]
    pub fn with_source(source_path: impl Into<String>, source_format: impl Into<String>) -> Self {
        Self {
            source_path: Some(source_path.into()),
            source_format: Some(source_format.into()),
            ..Self::new()
        }
    }

    /// Add a context to the tree.
    ///
    /// Automatically updates the URI index and parent→children index.
    pub fn add_context(&mut self, context: Context) {
        let uri = context.uri.clone();
        let idx = self.contexts.len();

        // Update children index.
        if let Some(parent) = &context.parent_uri {
            self.children_index
                .entry(parent.clone())
                .or_default()
                .push(uri.clone());
        }

        self.contexts.push(context);
        self.uri_index.insert(uri, idx);
    }

    /// Set the root URI explicitly.
    pub fn set_root(&mut self, uri: impl Into<String>) {
        self.root_uri = Some(uri.into());
    }

    /// Get the root URI string (if set).
    ///
    /// This is the equivalent of Python's `BuildingTree._root_uri` direct access,
    /// used by `ResourceProcessor` to return the root URI in result payloads.
    #[must_use]
    pub fn root_uri(&self) -> Option<&str> {
        self.root_uri.as_deref()
    }

    /// Get the root context (if a root URI has been set).
    #[must_use]
    pub fn root(&self) -> Option<&Context> {
        self.root_uri.as_ref().and_then(|uri| self.get(uri))
    }

    /// Get all contexts in insertion order.
    #[must_use]
    pub fn contexts(&self) -> &[Context] {
        &self.contexts
    }

    /// Lookup a context by URI — O(1).
    #[must_use]
    pub fn get(&self, uri: &str) -> Option<&Context> {
        self.uri_index.get(uri).map(|&idx| &self.contexts[idx])
    }

    /// Lookup a mutable context by URI — O(1).
    #[must_use]
    pub fn get_mut(&mut self, uri: &str) -> Option<&mut Context> {
        self.uri_index.get(uri).map(|&idx| &mut self.contexts[idx])
    }

    /// Get the parent context of a URI — O(1).
    #[must_use]
    pub fn parent(&self, uri: &str) -> Option<&Context> {
        self.get(uri)
            .and_then(|ctx| ctx.parent_uri.as_deref())
            .and_then(|parent_uri| self.get(parent_uri))
    }

    /// Get children of a URI — O(1) (amortised).
    ///
    /// This is a significant performance improvement over the Python original
    /// which performed an O(n) linear scan.
    #[must_use]
    pub fn children(&self, uri: &str) -> Vec<&Context> {
        self.children_index
            .get(uri)
            .map(|child_uris| {
                child_uris
                    .iter()
                    .filter_map(|child_uri| self.get(child_uri))
                    .collect()
            })
            .unwrap_or_default()
    }

    /// Walk from a URI up to the root, collecting each context along the way.
    #[must_use]
    pub fn path_to_root(&self, uri: &str) -> Vec<&Context> {
        let mut path = Vec::new();
        let mut current = uri;
        while let Some(ctx) = self.get(current) {
            path.push(ctx);
            match &ctx.parent_uri {
                Some(parent) => current = parent.as_str(),
                None => break,
            }
        }
        path
    }

    /// Total number of contexts in the tree.
    #[must_use]
    pub fn len(&self) -> usize {
        self.contexts.len()
    }

    /// Whether the tree is empty.
    #[must_use]
    pub fn is_empty(&self) -> bool {
        self.contexts.is_empty()
    }

    /// Iterate over all contexts in insertion order.
    pub fn iter(&self) -> impl Iterator<Item = &Context> {
        self.contexts.iter()
    }

    /// Convert tree to a nested directory structure (JSON-compatible).
    ///
    /// Port of `BuildingTree.to_directory_structure()` from Python.
    #[must_use]
    pub fn to_directory_structure(&self) -> serde_json::Value {
        fn build_dir(tree: &BuildingTree, uri: &str) -> serde_json::Value {
            let ctx = match tree.get(uri) {
                Some(c) => c,
                None => return serde_json::Value::Null,
            };
            let title = ctx
                .meta
                .get("semantic_title")
                .and_then(|v| v.as_str())
                .or_else(|| ctx.meta.get("source_title").and_then(|v| v.as_str()))
                .unwrap_or("Untitled");
            let children: Vec<serde_json::Value> = tree
                .children(uri)
                .iter()
                .map(|c| build_dir(tree, &c.uri))
                .collect();
            serde_json::json!({
                "uri": uri,
                "title": title,
                "type": ctx.context_type,
                "children": children,
            })
        }

        match &self.root_uri {
            Some(root) => build_dir(self, root),
            None => serde_json::Value::Object(serde_json::Map::new()),
        }
    }
}

impl Default for BuildingTree {
    fn default() -> Self {
        Self::new()
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    fn make_tree() -> BuildingTree {
        let mut tree = BuildingTree::new();

        let root = Context::new("viking://user", "User scope root");
        let memories = Context::new("viking://user/memories", "Memories directory")
            .with_parent("viking://user");
        let prefs = Context::new(
            "viking://user/memories/preferences",
            "Preferences directory",
        )
        .with_parent("viking://user/memories");
        let events = Context::new("viking://user/memories/events", "Events directory")
            .with_parent("viking://user/memories");

        tree.add_context(root);
        tree.add_context(memories);
        tree.add_context(prefs);
        tree.add_context(events);
        tree.set_root("viking://user");

        tree
    }

    #[test]
    fn tree_len_and_empty() {
        let tree = make_tree();
        assert_eq!(tree.len(), 4);
        assert!(!tree.is_empty());

        let empty = BuildingTree::new();
        assert!(empty.is_empty());
    }

    #[test]
    fn tree_root() {
        let tree = make_tree();
        let root = tree.root().expect("root should exist");
        assert_eq!(root.uri, "viking://user");
    }

    #[test]
    fn tree_get() {
        let tree = make_tree();
        assert!(tree.get("viking://user/memories").is_some());
        assert!(tree.get("viking://nonexistent").is_none());
    }

    #[test]
    fn tree_parent() {
        let tree = make_tree();
        let parent = tree.parent("viking://user/memories/preferences").unwrap();
        assert_eq!(parent.uri, "viking://user/memories");
    }

    #[test]
    fn tree_children() {
        let tree = make_tree();
        // user → memories
        let user_children = tree.children("viking://user");
        assert_eq!(user_children.len(), 1);
        assert_eq!(user_children[0].uri, "viking://user/memories");

        // memories → preferences, events
        let mem_children = tree.children("viking://user/memories");
        assert_eq!(mem_children.len(), 2);
        let child_uris: Vec<&str> = mem_children.iter().map(|c| c.uri.as_str()).collect();
        assert!(child_uris.contains(&"viking://user/memories/preferences"));
        assert!(child_uris.contains(&"viking://user/memories/events"));
    }

    #[test]
    fn tree_path_to_root() {
        let tree = make_tree();
        let path = tree.path_to_root("viking://user/memories/preferences");
        assert_eq!(path.len(), 3);
        assert_eq!(path[0].uri, "viking://user/memories/preferences");
        assert_eq!(path[1].uri, "viking://user/memories");
        assert_eq!(path[2].uri, "viking://user");
    }

    #[test]
    fn tree_get_mut() {
        let mut tree = make_tree();
        let ctx = tree.get_mut("viking://user").unwrap();
        ctx.update_activity();
        assert_eq!(tree.get("viking://user").unwrap().active_count, 1);
    }

    #[test]
    fn tree_iteration() {
        let tree = make_tree();
        let uris: Vec<&str> = tree.iter().map(|c| c.uri.as_str()).collect();
        assert_eq!(
            uris,
            vec![
                "viking://user",
                "viking://user/memories",
                "viking://user/memories/preferences",
                "viking://user/memories/events",
            ]
        );
    }

    #[test]
    fn tree_to_directory_structure() {
        let tree = make_tree();
        let dir = tree.to_directory_structure();

        // Root node
        assert_eq!(dir["uri"], "viking://user");
        assert_eq!(dir["title"], "Untitled"); // no semantic_title in meta
        assert_eq!(dir["type"], "resource"); // viking://user → Resource

        // Root has one child: memories
        let children = dir["children"].as_array().expect("children array");
        assert_eq!(children.len(), 1);
        assert_eq!(children[0]["uri"], "viking://user/memories");

        // memories has two children: preferences, events
        let mem_children = children[0]["children"].as_array().expect("mem children");
        assert_eq!(mem_children.len(), 2);
        let child_uris: Vec<&str> = mem_children
            .iter()
            .filter_map(|v| v["uri"].as_str())
            .collect();
        assert!(child_uris.contains(&"viking://user/memories/preferences"));
        assert!(child_uris.contains(&"viking://user/memories/events"));

        // Leaf nodes have empty children arrays
        for leaf in mem_children {
            assert_eq!(leaf["children"].as_array().map(|a| a.len()), Some(0),);
        }
    }

    #[test]
    fn tree_to_directory_structure_no_root() {
        // Tree without set_root should return empty JSON object
        let mut tree = BuildingTree::new();
        tree.add_context(Context::new("viking://orphan", "No root set"));
        let dir = tree.to_directory_structure();
        assert!(dir.is_object());
        assert!(dir.as_object().unwrap().is_empty());
    }

    #[test]
    fn tree_to_directory_structure_with_semantic_title() {
        let mut tree = BuildingTree::new();
        let mut root = Context::new("viking://user", "Root");
        root.meta.insert(
            "semantic_title".to_string(),
            serde_json::Value::String("My Workspace".to_string()),
        );
        tree.add_context(root);
        tree.set_root("viking://user");

        let dir = tree.to_directory_structure();
        // semantic_title takes precedence over "Untitled" fallback
        assert_eq!(dir["title"], "My Workspace");
    }
}
