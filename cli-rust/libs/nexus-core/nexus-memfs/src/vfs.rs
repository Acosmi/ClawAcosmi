//! nexus-memfs — virtual filesystem core.
//!
//! Provides the `MemoryFS` struct: a per-tenant in-memory virtual filesystem
//! with hierarchical directories, L0/L1/L2 tiered content, and simple
//! keyword-based recursive directory search.
//!
//! Inspired by OpenViking's filesystem management paradigm.

use crate::node::*;

// ---------------------------------------------------------------------------
// MemoryFS — per-tenant virtual filesystem
// ---------------------------------------------------------------------------

/// In-memory virtual filesystem for a single tenant + user combination.
///
/// The tree structure follows the OpenViking pattern:
/// ```text
/// root/
/// ├── permanent/
/// │   ├── decisions/
/// │   │   └── {mem_id}.md
/// │   ├── facts/
/// │   ├── emotions/
/// │   └── todos/
/// ```
pub struct MemoryFS {
    root: DirNode,
}

impl MemoryFS {
    /// Create a new MemoryFS with pre-scaffolded permanent memory directories.
    pub fn new() -> Self {
        let mut root = DirNode::new();

        // Scaffold the permanent memory directory structure
        let mut permanent = DirNode::with_content(
            "Permanent memory archive",
            "Contains archived permanent memories organized by category: decisions, facts, emotions, todos.",
        );

        permanent.children.insert(
            "decisions".into(),
            VfsNode::Dir(DirNode::with_content(
                "Key decisions and their reasoning",
                "Archived decisions extracted from conversations with rationale.",
            )),
        );
        permanent.children.insert(
            "facts".into(),
            VfsNode::Dir(DirNode::with_content(
                "Important facts and data points",
                "Concrete facts, data points, and validated information.",
            )),
        );
        permanent.children.insert(
            "emotions".into(),
            VfsNode::Dir(DirNode::with_content(
                "Emotional tendencies and attitudes",
                "User sentiment, emotional shifts, and attitude patterns.",
            )),
        );
        permanent.children.insert(
            "todos".into(),
            VfsNode::Dir(DirNode::with_content(
                "Pending tasks and commitments",
                "Unfinished tasks, promises, and action items.",
            )),
        );

        root.children
            .insert("permanent".into(), VfsNode::Dir(permanent));

        // Scaffold the episodic memory directory structure (L2)
        let mut episodic = DirNode::with_content(
            "Episodic memory archive",
            "Contains episodic memories: dialogues and observations from user interactions.",
        );
        episodic.children.insert(
            "dialogues".into(),
            VfsNode::Dir(DirNode::with_content(
                "Dialogue records",
                "Conversation logs and dialogue-derived memories.",
            )),
        );
        episodic.children.insert(
            "observations".into(),
            VfsNode::Dir(DirNode::with_content(
                "Observations",
                "Observed facts, events, and user behaviors.",
            )),
        );
        root.children
            .insert("episodic".into(), VfsNode::Dir(episodic));

        // Scaffold the semantic memory directory structure (L3)
        let mut semantic = DirNode::with_content(
            "Semantic memory archive",
            "Contains semantic memories: reflections and distilled knowledge.",
        );
        semantic.children.insert(
            "reflections".into(),
            VfsNode::Dir(DirNode::with_content(
                "Reflections and insights",
                "Synthesized insights and reflective analyses from memory consolidation.",
            )),
        );
        semantic.children.insert(
            "knowledge".into(),
            VfsNode::Dir(DirNode::with_content(
                "Knowledge base",
                "Distilled knowledge, skills, and procedural information.",
            )),
        );
        root.children
            .insert("semantic".into(), VfsNode::Dir(semantic));

        // Scaffold the session management directory structure
        // Used by SessionCommitter for dialogue archive + memory extraction
        let mut session = DirNode::with_content(
            "Session management",
            "Contains session archives: compressed dialogue history and extracted memories.",
        );
        session.children.insert(
            "archives".into(),
            VfsNode::Dir(DirNode::with_content(
                "Session archives (0 entries)",
                "Compressed dialogue archives indexed by session commit order.",
            )),
        );
        root.children
            .insert("session".into(), VfsNode::Dir(session));

        Self { root }
    }

    // -----------------------------------------------------------------------
    // Write
    // -----------------------------------------------------------------------

    /// Write a memory file into the VFS under `{section}/{category}/{memory_id}.md`.
    ///
    /// This is the generalized write method supporting any section/category path.
    /// The section directory and category sub-directory must already exist in the
    /// scaffold (created by `MemoryFS::new()`).
    ///
    /// Returns `Ok(())` on success, `Err(msg)` if the path is invalid.
    pub fn write_memory_to(
        &mut self,
        section: &str,
        category: &str,
        memory_id: &str,
        content: &str,
        l0_abstract: &str,
        l1_overview: &str,
    ) -> Result<(), String> {
        let cat_dir = self.get_section_category_dir_mut(section, category)?;
        let filename = format!("{}.md", memory_id);
        let file = FileNode::new(memory_id, category, l0_abstract, l1_overview, content);
        cat_dir.children.insert(filename, VfsNode::File(file));

        // Update directory-level L0 with child count
        let count = cat_dir.children.len();
        cat_dir.content.l0_abstract = format!(
            "{} ({} entries)",
            cat_dir
                .content
                .l0_abstract
                .split(" (")
                .next()
                .unwrap_or(category),
            count
        );

        Ok(())
    }

    /// Write a memory file into the VFS under `permanent/{category}/{memory_id}.md`.
    ///
    /// Backward-compatible wrapper for `write_memory_to("permanent", ...)`.
    pub fn write_memory(
        &mut self,
        memory_id: &str,
        category: &str,
        content: &str,
        l0_abstract: &str,
        l1_overview: &str,
    ) -> Result<(), String> {
        self.write_memory_to(
            "permanent",
            category,
            memory_id,
            content,
            l0_abstract,
            l1_overview,
        )
    }

    // -----------------------------------------------------------------------
    // Read
    // -----------------------------------------------------------------------

    /// Read content at the given path and tier level.
    ///
    /// Path is relative to root, e.g. `"permanent/decisions/{id}.md"`.
    /// For directories, returns L0 or L1 content. For files, returns the
    /// requested tier.
    pub fn read(&self, path: &str, tier: Tier) -> Result<String, String> {
        let node = self.resolve(path)?;
        match node {
            VfsNode::Dir(d) => Ok(d.content.get(tier).to_string()),
            VfsNode::File(f) => Ok(f.content.get(tier).to_string()),
        }
    }

    // -----------------------------------------------------------------------
    // List directory
    // -----------------------------------------------------------------------

    /// List entries in the directory at the given path.
    ///
    /// Returns a JSON-serializable list of `DirEntry`.
    pub fn list_dir(&self, path: &str) -> Result<Vec<DirEntry>, String> {
        let node = self.resolve(path)?;
        match node {
            VfsNode::Dir(d) => {
                let entries: Vec<DirEntry> = d
                    .children
                    .iter()
                    .map(|(name, child)| match child {
                        VfsNode::Dir(cd) => DirEntry {
                            name: name.clone(),
                            is_dir: true,
                            l0_abstract: cd.content.l0_abstract.clone(),
                            created_at: cd.created_at,
                        },
                        VfsNode::File(cf) => DirEntry {
                            name: name.clone(),
                            is_dir: false,
                            l0_abstract: cf.content.l0_abstract.clone(),
                            created_at: cf.created_at,
                        },
                    })
                    .collect();
                Ok(entries)
            }
            VfsNode::File(_) => Err(format!("'{}' is a file, not a directory", path)),
        }
    }

    // -----------------------------------------------------------------------
    // Search — directory recursive retrieval (simplified)
    // -----------------------------------------------------------------------

    /// Search for memories matching the query keywords.
    ///
    /// Implements a simplified version of OpenViking's Directory Recursive
    /// Retrieval: walks all files under all sections (permanent, episodic,
    /// semantic), scores them by keyword overlap across all tiers, and
    /// returns top results.
    pub fn search(&self, query: &str, max_results: usize) -> Vec<SearchHit> {
        let query_lower = query.to_lowercase();
        let keywords: Vec<&str> = query_lower.split_whitespace().collect();
        if keywords.is_empty() {
            return vec![];
        }

        let mut hits: Vec<SearchHit> = Vec::new();
        self.recursive_search(&self.root, "", &keywords, &mut hits);

        // Sort by score descending
        hits.sort_by(|a, b| {
            b.score
                .partial_cmp(&a.score)
                .unwrap_or(std::cmp::Ordering::Equal)
        });
        hits.truncate(max_results);
        hits
    }

    // -----------------------------------------------------------------------
    // Search with trace — visualized retrieval trajectory
    // -----------------------------------------------------------------------

    /// Search with full retrieval trace for visualization.
    ///
    /// Returns a `SearchTrace` containing the ordered list of directories
    /// visited and files scored, along with the final hits. This enables
    /// the frontend to render a tree-view heatmap showing how the search
    /// traversed the VFS.
    pub fn search_with_trace(&self, query: &str, max_results: usize) -> SearchTrace {
        let query_lower = query.to_lowercase();
        let keywords: Vec<&str> = query_lower.split_whitespace().collect();

        if keywords.is_empty() {
            return SearchTrace {
                query: query.to_string(),
                keywords: vec![],
                steps: vec![],
                total_dirs_visited: 0,
                total_files_scored: 0,
                hits: vec![],
            };
        }

        let mut hits: Vec<SearchHit> = Vec::new();
        let mut steps: Vec<TraceStep> = Vec::new();
        let mut dirs_visited: usize = 0;
        let mut files_scored: usize = 0;

        self.recursive_search_traced(
            &self.root,
            "",
            &keywords,
            &mut hits,
            &mut steps,
            &mut dirs_visited,
            &mut files_scored,
        );

        // Sort by score descending
        hits.sort_by(|a, b| {
            b.score
                .partial_cmp(&a.score)
                .unwrap_or(std::cmp::Ordering::Equal)
        });
        hits.truncate(max_results);

        // Mark matched steps
        let matched_paths: std::collections::HashSet<String> =
            hits.iter().map(|h| h.path.clone()).collect();
        for step in &mut steps {
            if step.node_type == "file" && matched_paths.contains(&step.path) {
                step.matched = true;
            }
        }

        SearchTrace {
            query: query.to_string(),
            keywords: keywords.iter().map(|k| k.to_string()).collect(),
            steps,
            total_dirs_visited: dirs_visited,
            total_files_scored: files_scored,
            hits,
        }
    }

    // -----------------------------------------------------------------------
    // Delete
    // -----------------------------------------------------------------------

    /// Delete a memory by ID from any section/category directory.
    ///
    /// Recursively searches all sections and their sub-directories.
    pub fn delete_memory(&mut self, memory_id: &str) -> Result<(), String> {
        let filename = format!("{}.md", memory_id);
        let sections: Vec<String> = self.root.children.keys().cloned().collect();

        for section in &sections {
            if let Some(VfsNode::Dir(sec_dir)) = self.root.children.get_mut(section) {
                let categories: Vec<String> = sec_dir.children.keys().cloned().collect();
                for cat in &categories {
                    if let Some(VfsNode::Dir(cat_dir)) = sec_dir.children.get_mut(cat) {
                        if cat_dir.children.remove(&filename).is_some() {
                            let count = cat_dir.children.len();
                            let label = cat_dir
                                .content
                                .l0_abstract
                                .split(" (")
                                .next()
                                .unwrap_or(cat)
                                .to_string();
                            cat_dir.content.l0_abstract = format!("{} ({} entries)", label, count);
                            return Ok(());
                        }
                    }
                }
            }
        }

        Err(format!("memory '{}' not found", memory_id))
    }

    // -----------------------------------------------------------------------
    // Session archive management
    // -----------------------------------------------------------------------

    /// Create a new archive directory under `session/archives/archive_{index}/`.
    ///
    /// Writes the provided summary into the directory's L0 (one-line summary)
    /// and L1 (structured overview) tiers. This mirrors OpenViking's
    /// `session.commit()` archive mechanism.
    ///
    /// Returns `Ok(())` on success, `Err(msg)` if session/archives is missing
    /// or the archive index already exists.
    pub fn create_archive_dir(
        &mut self,
        index: u32,
        l0_summary: &str,
        l1_overview: &str,
    ) -> Result<(), String> {
        let archives_dir = match self.root.children.get_mut("session") {
            Some(VfsNode::Dir(session)) => match session.children.get_mut("archives") {
                Some(VfsNode::Dir(d)) => d,
                _ => return Err("session/archives directory not found".into()),
            },
            _ => return Err("session directory not found".into()),
        };

        let dir_name = format!("archive_{}", index);
        if archives_dir.children.contains_key(&dir_name) {
            return Err(format!("archive directory '{}' already exists", dir_name));
        }

        let archive = DirNode::with_content(l0_summary, l1_overview);
        archives_dir
            .children
            .insert(dir_name, VfsNode::Dir(archive));

        // Update archives directory L0 with child count
        let count = archives_dir.children.len();
        archives_dir.content.l0_abstract = format!("Session archives ({} entries)", count);

        Ok(())
    }

    /// Get the next archive index (count of existing archives).
    pub fn next_archive_index(&self) -> u32 {
        match self.root.children.get("session") {
            Some(VfsNode::Dir(session)) => match session.children.get("archives") {
                Some(VfsNode::Dir(d)) => d.children.len() as u32,
                _ => 0,
            },
            _ => 0,
        }
    }

    // -----------------------------------------------------------------------
    // Stats
    // -----------------------------------------------------------------------

    /// Count total memory files across all sections and categories.
    pub fn memory_count(&self) -> usize {
        let mut count = 0;
        for (_, node) in &self.root.children {
            if let VfsNode::Dir(sec_dir) = node {
                for (_, cat_node) in &sec_dir.children {
                    if let VfsNode::Dir(cat_dir) = cat_node {
                        count += cat_dir.children.len();
                    }
                }
            }
        }
        count
    }

    // -----------------------------------------------------------------------
    // Internal helpers
    // -----------------------------------------------------------------------

    /// Resolve a path to its VfsNode.
    fn resolve(&self, path: &str) -> Result<&VfsNode, String> {
        let path = path.trim_matches('/');
        if path.is_empty() {
            // Wrap root in a temporary VfsNode reference — use a workaround
            // since root is not wrapped in VfsNode.
            // Instead, handle root specially in callers.
            return Err("use list_dir(\"\") or read(\"\", tier) for root".into());
        }

        let parts: Vec<&str> = path.split('/').collect();
        let mut current = &self.root;

        for (i, part) in parts.iter().enumerate() {
            match current.children.get(*part) {
                Some(VfsNode::Dir(d)) => {
                    if i == parts.len() - 1 {
                        return Ok(current.children.get(*part).unwrap());
                    }
                    current = d;
                }
                Some(node @ VfsNode::File(_)) => {
                    if i == parts.len() - 1 {
                        return Ok(node);
                    }
                    return Err(format!("'{}' is a file, cannot traverse into it", part));
                }
                None => return Err(format!("path segment '{}' not found", part)),
            }
        }

        // Should not reach here, but return the last dir's node if we do
        Err("path resolution failed".into())
    }

    /// Get a mutable reference to a category directory under `permanent/`.
    #[allow(dead_code)]
    fn get_category_dir_mut(&mut self, category: &str) -> Result<&mut DirNode, String> {
        let valid = ["decisions", "facts", "emotions", "todos"];
        if !valid.contains(&category) {
            return Err(format!(
                "invalid category '{}', expected one of: {:?}",
                category, valid
            ));
        }

        let permanent = match self.root.children.get_mut("permanent") {
            Some(VfsNode::Dir(d)) => d,
            _ => return Err("permanent directory not found".into()),
        };

        match permanent.children.get_mut(category) {
            Some(VfsNode::Dir(d)) => Ok(d),
            _ => Err(format!("category directory '{}' not found", category)),
        }
    }

    /// Get an immutable reference to a category directory under `permanent/`.
    #[allow(dead_code)]
    fn get_category_dir(&self, category: &str) -> Result<&DirNode, String> {
        let permanent = match self.root.children.get("permanent") {
            Some(VfsNode::Dir(d)) => d,
            _ => return Err("permanent directory not found".into()),
        };

        match permanent.children.get(category) {
            Some(VfsNode::Dir(d)) => Ok(d),
            _ => Err(format!("category directory '{}' not found", category)),
        }
    }

    /// Get a mutable reference to a category directory under any section.
    fn get_section_category_dir_mut(
        &mut self,
        section: &str,
        category: &str,
    ) -> Result<&mut DirNode, String> {
        let sec_dir = match self.root.children.get_mut(section) {
            Some(VfsNode::Dir(d)) => d,
            _ => return Err(format!("section directory '{}' not found", section)),
        };

        match sec_dir.children.get_mut(category) {
            Some(VfsNode::Dir(d)) => Ok(d),
            _ => Err(format!(
                "category directory '{}/{}' not found",
                section, category
            )),
        }
    }

    /// Recursive search helper — walks all file nodes and scores them.
    fn recursive_search(
        &self,
        dir: &DirNode,
        prefix: &str,
        keywords: &[&str],
        hits: &mut Vec<SearchHit>,
    ) {
        for (name, node) in &dir.children {
            let path = if prefix.is_empty() {
                name.clone()
            } else {
                format!("{}/{}", prefix, name)
            };

            match node {
                VfsNode::Dir(d) => {
                    self.recursive_search(d, &path, keywords, hits);
                }
                VfsNode::File(f) => {
                    let score = Self::score_file(f, keywords);
                    if score > 0.0 {
                        hits.push(SearchHit {
                            path,
                            memory_id: f.memory_id.clone(),
                            category: f.category.clone(),
                            score,
                            l0_abstract: f.content.l0_abstract.clone(),
                        });
                    }
                }
            }
        }
    }

    /// Score a file against search keywords.
    ///
    /// Scoring strategy inspired by OpenViking's directory recursive retrieval:
    /// - L0 match yields highest weight (3x) — abstract is most curated
    /// - L1 match yields medium weight (2x)
    /// - L2 match yields base weight (1x)
    /// - Category name match yields bonus (1x)
    fn score_file(file: &FileNode, keywords: &[&str]) -> f64 {
        let l0 = file.content.l0_abstract.to_lowercase();
        let l1 = file.content.l1_overview.to_lowercase();
        let l2 = file.content.l2_detail.to_lowercase();
        let cat = file.category.to_lowercase();

        let mut total = 0.0;
        let per_keyword = 1.0 / keywords.len() as f64;

        for kw in keywords {
            let mut kw_score = 0.0;
            if l0.contains(kw) {
                kw_score += 3.0;
            }
            if l1.contains(kw) {
                kw_score += 2.0;
            }
            if l2.contains(kw) {
                kw_score += 1.0;
            }
            if cat.contains(kw) {
                kw_score += 1.0;
            }
            // Normalize per keyword: max possible = 7.0
            total += (kw_score / 7.0) * per_keyword;
        }

        total
    }

    /// Score a directory's L0/L1 content against search keywords.
    ///
    /// Used only by the traced search — gives an approximate relevance
    /// score for directory-level content to show in the heatmap.
    fn score_dir(dir: &DirNode, keywords: &[&str]) -> f64 {
        let l0 = dir.content.l0_abstract.to_lowercase();
        let l1 = dir.content.l1_overview.to_lowercase();

        let mut total = 0.0;
        let per_keyword = 1.0 / keywords.len() as f64;

        for kw in keywords {
            let mut kw_score = 0.0;
            if l0.contains(kw) {
                kw_score += 3.0;
            }
            if l1.contains(kw) {
                kw_score += 2.0;
            }
            // Max possible for dir = 5.0
            total += (kw_score / 5.0) * per_keyword;
        }
        total
    }

    /// Traced recursive search — walks all nodes and records trace steps.
    fn recursive_search_traced(
        &self,
        dir: &DirNode,
        prefix: &str,
        keywords: &[&str],
        hits: &mut Vec<SearchHit>,
        steps: &mut Vec<TraceStep>,
        dirs_visited: &mut usize,
        files_scored: &mut usize,
    ) {
        for (name, node) in &dir.children {
            let path = if prefix.is_empty() {
                name.clone()
            } else {
                format!("{}/{}", prefix, name)
            };

            match node {
                VfsNode::Dir(d) => {
                    *dirs_visited += 1;
                    let dir_score = Self::score_dir(d, keywords);
                    let children_count = d.children.len();

                    steps.push(TraceStep {
                        path: path.clone(),
                        node_type: "dir".to_string(),
                        score: dir_score,
                        children_explored: children_count,
                        matched: false,
                    });

                    self.recursive_search_traced(
                        d,
                        &path,
                        keywords,
                        hits,
                        steps,
                        dirs_visited,
                        files_scored,
                    );
                }
                VfsNode::File(f) => {
                    *files_scored += 1;
                    let score = Self::score_file(f, keywords);

                    steps.push(TraceStep {
                        path: path.clone(),
                        node_type: "file".to_string(),
                        score,
                        children_explored: 0,
                        matched: false, // will be set later
                    });

                    if score > 0.0 {
                        hits.push(SearchHit {
                            path,
                            memory_id: f.memory_id.clone(),
                            category: f.category.clone(),
                            score,
                            l0_abstract: f.content.l0_abstract.clone(),
                        });
                    }
                }
            }
        }
    }

    #[allow(dead_code)]
    fn category_label(category: &str) -> &str {
        match category {
            "decisions" => "Key decisions and their reasoning",
            "facts" => "Important facts and data points",
            "emotions" => "Emotional tendencies and attitudes",
            "todos" => "Pending tasks and commitments",
            _ => "Unknown category",
        }
    }
}

// ---------------------------------------------------------------------------
// Root-level operations (special-cased since root is not a VfsNode)
// ---------------------------------------------------------------------------

impl MemoryFS {
    /// Read root directory content at a specific tier.
    pub fn read_root(&self, tier: Tier) -> String {
        self.root.content.get(tier).to_string()
    }

    /// List root directory entries.
    pub fn list_root(&self) -> Vec<DirEntry> {
        self.root
            .children
            .iter()
            .map(|(name, child)| match child {
                VfsNode::Dir(d) => DirEntry {
                    name: name.clone(),
                    is_dir: true,
                    l0_abstract: d.content.l0_abstract.clone(),
                    created_at: d.created_at,
                },
                VfsNode::File(f) => DirEntry {
                    name: name.clone(),
                    is_dir: false,
                    l0_abstract: f.content.l0_abstract.clone(),
                    created_at: f.created_at,
                },
            })
            .collect()
    }

    /// Update the root directory's L0/L1 content.
    pub fn set_root_content(&mut self, l0: &str, l1: &str) {
        self.root.content.l0_abstract = l0.to_string();
        self.root.content.l1_overview = l1.to_string();
    }

    /// Immutable reference to the root node (used by storage serialization).
    pub fn root_ref(&self) -> &DirNode {
        &self.root
    }

    /// Construct a MemoryFS from a deserialized root node.
    pub fn from_root(root: DirNode) -> Self {
        Self { root }
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_new_has_scaffold() {
        let fs = MemoryFS::new();
        assert!(fs.root.children.contains_key("permanent"));
        assert!(fs.root.children.contains_key("episodic"));
        assert!(fs.root.children.contains_key("semantic"));
        let entries = fs.list_root();
        assert_eq!(entries.len(), 4);
        // permanent should be present
        assert!(entries.iter().any(|e| e.name == "permanent" && e.is_dir));
    }

    #[test]
    fn test_list_permanent_categories() {
        let fs = MemoryFS::new();
        let entries = fs.list_dir("permanent").unwrap();
        let names: Vec<&str> = entries.iter().map(|e| e.name.as_str()).collect();
        assert!(names.contains(&"decisions"));
        assert!(names.contains(&"facts"));
        assert!(names.contains(&"emotions"));
        assert!(names.contains(&"todos"));
        assert_eq!(entries.len(), 4);
    }

    #[test]
    fn test_write_and_read_memory() {
        let mut fs = MemoryFS::new();
        fs.write_memory(
            "mem-001",
            "decisions",
            "完整的决策内容：选择了 Rust 作为实现语言",
            "选择 Rust 实现语言",
            "团队决定使用 Rust 实现 nexus-memfs 模块",
        )
        .unwrap();

        // Read L0
        let l0 = fs.read("permanent/decisions/mem-001.md", Tier::L0).unwrap();
        assert_eq!(l0, "选择 Rust 实现语言");

        // Read L1
        let l1 = fs.read("permanent/decisions/mem-001.md", Tier::L1).unwrap();
        assert!(l1.contains("Rust"));

        // Read L2
        let l2 = fs.read("permanent/decisions/mem-001.md", Tier::L2).unwrap();
        assert!(l2.contains("完整的决策内容"));
    }

    #[test]
    fn test_write_invalid_category() {
        let mut fs = MemoryFS::new();
        let result = fs.write_memory("m", "invalid", "c", "a", "o");
        assert!(result.is_err());
    }

    #[test]
    fn test_delete_memory() {
        let mut fs = MemoryFS::new();
        fs.write_memory("mem-002", "facts", "事实内容", "摘要", "概述")
            .unwrap();
        assert_eq!(fs.memory_count(), 1);

        fs.delete_memory("mem-002").unwrap();
        assert_eq!(fs.memory_count(), 0);
    }

    #[test]
    fn test_delete_nonexistent() {
        let mut fs = MemoryFS::new();
        let result = fs.delete_memory("nonexistent");
        assert!(result.is_err());
    }

    #[test]
    fn test_search_basic() {
        let mut fs = MemoryFS::new();
        fs.write_memory(
            "m1",
            "decisions",
            "选择 Rust",
            "Rust 决策",
            "使用 Rust 编写模块",
        )
        .unwrap();
        fs.write_memory("m2", "facts", "Go 很快", "Go 速度", "Go 编译速度很快")
            .unwrap();
        fs.write_memory(
            "m3",
            "decisions",
            "Python 弃用",
            "弃用 Python",
            "放弃 Python 方案",
        )
        .unwrap();

        let results = fs.search("Rust", 10);
        assert!(!results.is_empty());
        assert_eq!(results[0].memory_id, "m1");
    }

    #[test]
    fn test_search_no_match() {
        let mut fs = MemoryFS::new();
        fs.write_memory("m1", "facts", "内容A", "摘要A", "概述A")
            .unwrap();
        let results = fs.search("XYZ不存在的关键词", 10);
        assert!(results.is_empty());
    }

    #[test]
    fn test_search_multi_keyword() {
        let mut fs = MemoryFS::new();
        fs.write_memory(
            "m1",
            "facts",
            "Rust 高性能",
            "Rust 性能",
            "Rust 是高性能语言",
        )
        .unwrap();
        fs.write_memory("m2", "facts", "高性能计算", "计算速度", "GPU 高性能计算")
            .unwrap();

        // "Rust 高性能" should favor m1 over m2
        let results = fs.search("Rust 高性能", 10);
        assert!(!results.is_empty());
        assert_eq!(results[0].memory_id, "m1");
    }

    #[test]
    fn test_memory_count() {
        let mut fs = MemoryFS::new();
        assert_eq!(fs.memory_count(), 0);

        fs.write_memory("m1", "facts", "c", "a", "o").unwrap();
        fs.write_memory("m2", "decisions", "c", "a", "o").unwrap();
        fs.write_memory("m3", "todos", "c", "a", "o").unwrap();
        assert_eq!(fs.memory_count(), 3);
    }

    #[test]
    fn test_list_dir_with_files() {
        let mut fs = MemoryFS::new();
        fs.write_memory("m1", "facts", "内容", "摘要", "概述")
            .unwrap();
        fs.write_memory("m2", "facts", "内容2", "摘要2", "概述2")
            .unwrap();

        let entries = fs.list_dir("permanent/facts").unwrap();
        assert_eq!(entries.len(), 2);
        assert!(!entries[0].is_dir);
    }

    // --- Phase 2: episodic / semantic tests ---

    #[test]
    fn test_new_has_episodic_semantic_scaffold() {
        let fs = MemoryFS::new();
        let root_entries = fs.list_root();
        let names: Vec<&str> = root_entries.iter().map(|e| e.name.as_str()).collect();
        assert!(names.contains(&"permanent"));
        assert!(names.contains(&"episodic"));
        assert!(names.contains(&"semantic"));
        assert_eq!(root_entries.len(), 4);
    }

    #[test]
    fn test_episodic_subdirs() {
        let fs = MemoryFS::new();
        let entries = fs.list_dir("episodic").unwrap();
        let names: Vec<&str> = entries.iter().map(|e| e.name.as_str()).collect();
        assert!(names.contains(&"dialogues"));
        assert!(names.contains(&"observations"));
    }

    #[test]
    fn test_semantic_subdirs() {
        let fs = MemoryFS::new();
        let entries = fs.list_dir("semantic").unwrap();
        let names: Vec<&str> = entries.iter().map(|e| e.name.as_str()).collect();
        assert!(names.contains(&"reflections"));
        assert!(names.contains(&"knowledge"));
    }

    #[test]
    fn test_write_memory_to_episodic() {
        let mut fs = MemoryFS::new();
        fs.write_memory_to(
            "episodic",
            "dialogues",
            "d1",
            "对话全文内容",
            "用户讨论了 Rust",
            "讨论 Rust 作为 VFS 实现语言",
        )
        .unwrap();

        let l0 = fs.read("episodic/dialogues/d1.md", Tier::L0).unwrap();
        assert_eq!(l0, "用户讨论了 Rust");
        let l2 = fs.read("episodic/dialogues/d1.md", Tier::L2).unwrap();
        assert!(l2.contains("对话全文"));
    }

    #[test]
    fn test_write_memory_to_semantic() {
        let mut fs = MemoryFS::new();
        fs.write_memory_to(
            "semantic",
            "reflections",
            "r1",
            "反思内容",
            "用户偏好总结",
            "综合分析",
        )
        .unwrap();

        let content = fs.read("semantic/reflections/r1.md", Tier::L1).unwrap();
        assert!(content.contains("综合分析"));
    }

    #[test]
    fn test_write_memory_to_invalid_section() {
        let mut fs = MemoryFS::new();
        let result = fs.write_memory_to("nonexistent", "cat", "m1", "c", "a", "o");
        assert!(result.is_err());
    }

    #[test]
    fn test_search_across_sections() {
        let mut fs = MemoryFS::new();
        fs.write_memory("m1", "facts", "Rust 高效", "Rust", "Rust 工具")
            .unwrap();
        fs.write_memory_to(
            "episodic",
            "dialogues",
            "d1",
            "讨论 Rust",
            "Rust 对话",
            "关于 Rust 的讨论",
        )
        .unwrap();

        let results = fs.search("Rust", 10);
        assert!(results.len() >= 2);
    }

    // --- Phase 3: session archive tests ---

    #[test]
    fn test_new_has_session_scaffold() {
        let fs = MemoryFS::new();
        let root_entries = fs.list_root();
        let names: Vec<&str> = root_entries.iter().map(|e| e.name.as_str()).collect();
        assert!(names.contains(&"session"));
        // session should have archives sub-directory
        let session_entries = fs.list_dir("session").unwrap();
        assert!(session_entries
            .iter()
            .any(|e| e.name == "archives" && e.is_dir));
    }

    #[test]
    fn test_create_archive_dir() {
        let mut fs = MemoryFS::new();
        assert_eq!(fs.next_archive_index(), 0);

        fs.create_archive_dir(
            0,
            "讨论了 Rust VFS 设计",
            "用户与助手讨论了 VFS 的目录结构设计，决定使用 L0/L1/L2 分层",
        )
        .unwrap();

        assert_eq!(fs.next_archive_index(), 1);

        // Verify the archive directory exists and has correct content
        let l0 = fs.read("session/archives/archive_0", Tier::L0).unwrap();
        assert_eq!(l0, "讨论了 Rust VFS 设计");
        let l1 = fs.read("session/archives/archive_0", Tier::L1).unwrap();
        assert!(l1.contains("VFS"));
    }

    #[test]
    fn test_create_archive_dir_duplicate() {
        let mut fs = MemoryFS::new();
        fs.create_archive_dir(0, "摘要", "概述").unwrap();
        let result = fs.create_archive_dir(0, "重复", "重复");
        assert!(result.is_err());
    }

    #[test]
    fn test_write_memory_to_archive() {
        let mut fs = MemoryFS::new();
        fs.create_archive_dir(0, "归档 0", "第一次会话归档")
            .unwrap();
        // Write a memory into the archive directory
        fs.write_memory_to(
            "session",
            "archives",
            "commit-mem-001",
            "提取的记忆内容",
            "用户偏好暗色主题",
            "用户明确表示偏好暗色主题界面",
        )
        .unwrap();
        let l0 = fs
            .read("session/archives/commit-mem-001.md", Tier::L0)
            .unwrap();
        assert_eq!(l0, "用户偏好暗色主题");
    }

    #[test]
    fn test_delete_across_sections() {
        let mut fs = MemoryFS::new();
        fs.write_memory_to(
            "episodic",
            "observations",
            "e1",
            "观察内容",
            "观察摘要",
            "用户行为观察",
        )
        .unwrap();
        assert_eq!(fs.memory_count(), 1);

        fs.delete_memory("e1").unwrap();
        assert_eq!(fs.memory_count(), 0);
    }

    #[test]
    fn test_memory_count_all_sections() {
        let mut fs = MemoryFS::new();
        fs.write_memory("m1", "facts", "c", "a", "o").unwrap();
        fs.write_memory_to("episodic", "dialogues", "d1", "c", "a", "o")
            .unwrap();
        fs.write_memory_to("semantic", "knowledge", "k1", "c", "a", "o")
            .unwrap();
        assert_eq!(fs.memory_count(), 3);
    }

    // --- Phase 4: search trace tests ---

    #[test]
    fn test_search_with_trace_basic() {
        let mut fs = MemoryFS::new();
        fs.write_memory(
            "m1",
            "decisions",
            "选择 Rust",
            "Rust 决策",
            "使用 Rust 编写模块",
        )
        .unwrap();
        fs.write_memory("m2", "facts", "Go 速度快", "Go 速度", "Go 编译快")
            .unwrap();

        let trace = fs.search_with_trace("Rust", 10);
        assert_eq!(trace.query, "Rust");
        assert_eq!(trace.keywords, vec!["rust"]);
        assert!(!trace.steps.is_empty());
        assert!(trace.total_dirs_visited > 0);
        assert!(trace.total_files_scored > 0);
        // At least one hit for "Rust"
        assert!(!trace.hits.is_empty());
        assert_eq!(trace.hits[0].memory_id, "m1");
        // The matched file step should be marked
        let matched_steps: Vec<_> = trace.steps.iter().filter(|s| s.matched).collect();
        assert!(!matched_steps.is_empty());
    }

    #[test]
    fn test_search_with_trace_structure() {
        let mut fs = MemoryFS::new();
        fs.write_memory("m1", "facts", "数据 Alpha", "Alpha 摘要", "Alpha 概述")
            .unwrap();
        fs.write_memory_to(
            "episodic",
            "dialogues",
            "d1",
            "Alpha 对话",
            "Alpha 讨论",
            "讨论 Alpha",
        )
        .unwrap();

        let trace = fs.search_with_trace("alpha", 10);

        // Verify dir steps exist
        let dir_steps: Vec<_> = trace
            .steps
            .iter()
            .filter(|s| s.node_type == "dir")
            .collect();
        assert!(dir_steps.len() >= 2); // at least permanent + episodic dirs

        // Verify file steps exist
        let file_steps: Vec<_> = trace
            .steps
            .iter()
            .filter(|s| s.node_type == "file")
            .collect();
        assert!(file_steps.len() >= 2); // m1 + d1

        // Counters should match
        assert_eq!(trace.total_dirs_visited, dir_steps.len());
        assert_eq!(trace.total_files_scored, file_steps.len());

        // Both memories should be hits
        assert!(trace.hits.len() >= 2);
    }
}
