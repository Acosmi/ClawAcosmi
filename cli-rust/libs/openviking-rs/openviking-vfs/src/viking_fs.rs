// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! VikingFS core implementation — high-level file system abstraction.
//!
//! Ported from `openviking/storage/viking_fs.py` (1154 lines, 64 methods).
//!
//! This module provides the [`VikingFs`] struct which composes a low-level
//! [`FileSystem`] backend with optional [`VectorStore`] and [`Embedder`]
//! to provide URI-aware file operations, L0/L1 context reading, relation
//! management, semantic search, and vector store synchronization.

use std::collections::HashMap;

use log::{debug, info, warn};

use openviking_core::relation::RelationEntry;
use openviking_core::uri::{fs_path_to_uri, uri_to_fs_path, VikingUri};

use openviking_session::traits::{
    BoxError, Embedder, FileSystem, FsEntry, FsStat, GrepMatch, VectorHit, VectorStore,
};

use async_trait::async_trait;

// ---------------------------------------------------------------------------
// VikingFs
// ---------------------------------------------------------------------------

/// High-level file system abstraction over `FileSystem` + `VectorStore` +
/// `Embedder`.
///
/// Mirrors the Python `VikingFS` class, providing:
/// - Basic file operations delegated to the underlying `FileSystem`
/// - L0/L1 context reading (`.abstract.md` / `.overview.md`)
/// - Relation management via `.relations.json`
/// - Semantic search via vector store + embedder
/// - Vector store sync on destructive operations (rm/mv)
/// - Context writing (L0/L1/L2 bundles)
pub struct VikingFs<FS, VS, EMB>
where
    FS: FileSystem,
    VS: VectorStore,
    EMB: Embedder,
{
    /// The underlying file system backend.
    fs: FS,
    /// Optional vector store for semantic search.
    vector_store: Option<VS>,
    /// Optional embedder for query vectorization.
    embedder: Option<EMB>,
}

impl<FS: FileSystem, VS: VectorStore, EMB: Embedder> VikingFs<FS, VS, EMB> {
    /// Create a new `VikingFs` with only a file system backend.
    pub fn new(fs: FS) -> Self {
        Self {
            fs,
            vector_store: None,
            embedder: None,
        }
    }

    /// Create a fully configured `VikingFs`.
    pub fn with_backends(fs: FS, vector_store: VS, embedder: EMB) -> Self {
        Self {
            fs,
            vector_store: Some(vector_store),
            embedder: Some(embedder),
        }
    }

    /// Access the optional vector store (crate-internal).
    pub(crate) fn vector_store_ref(&self) -> Option<&VS> {
        self.vector_store.as_ref()
    }

    // =======================================================================
    // Basic File Operations (delegate to FS backend)
    // =======================================================================

    /// Read file content as string.
    pub async fn read(&self, uri: &str) -> Result<String, BoxError> {
        self.fs.read(uri).await
    }

    /// Read file content as bytes.
    pub async fn read_bytes(&self, uri: &str) -> Result<Vec<u8>, BoxError> {
        self.fs.read_bytes(uri).await
    }

    /// Write string content.
    pub async fn write(&self, uri: &str, content: &str) -> Result<(), BoxError> {
        self.fs.write(uri, content).await
    }

    /// Write binary content.
    pub async fn write_bytes(&self, uri: &str, content: &[u8]) -> Result<(), BoxError> {
        self.fs.write_bytes(uri, content).await
    }

    /// Create a directory (ensures parent directories exist).
    pub async fn mkdir(&self, uri: &str, exist_ok: bool) -> Result<(), BoxError> {
        self.ensure_parent_dirs(uri).await;
        if exist_ok {
            if let Ok(true) = self.fs.exists(uri).await {
                return Ok(());
            }
        }
        self.fs.mkdir(uri).await
    }

    /// List directory contents.
    pub async fn list_dir(&self, uri: &str) -> Result<Vec<FsEntry>, BoxError> {
        self.fs.ls(uri).await
    }

    /// Get file/directory metadata.
    pub async fn stat(&self, uri: &str) -> Result<FsStat, BoxError> {
        self.fs.stat(uri).await
    }

    /// Remove file/directory with vector store sync.
    pub async fn rm(&self, uri: &str, recursive: bool) -> Result<(), BoxError> {
        // Collect URIs before deletion for vector sync
        let uris_to_delete = if recursive {
            self.collect_child_uris(uri).await
        } else {
            vec![uri.to_owned()]
        };

        self.fs.rm(uri).await?;

        // Sync: remove from vector store
        if !uris_to_delete.is_empty() {
            self.sync_delete_vectors(&uris_to_delete).await;
        }
        Ok(())
    }

    /// Move/rename file or directory with vector store sync.
    pub async fn mv(&self, from_uri: &str, to_uri: &str) -> Result<(), BoxError> {
        // Collect URIs before move for vector sync
        let uris_to_move = self.collect_child_uris(from_uri).await;

        self.fs.mv(from_uri, to_uri).await?;

        // Sync: update URIs in vector store
        if !uris_to_move.is_empty() {
            self.sync_move_vectors(&uris_to_move, from_uri, to_uri)
                .await;
        }
        Ok(())
    }

    /// Content search by pattern.
    pub async fn grep(
        &self,
        uri: &str,
        pattern: &str,
        case_insensitive: bool,
    ) -> Result<Vec<GrepMatch>, BoxError> {
        self.fs.grep(uri, pattern, true, case_insensitive).await
    }

    /// Append content to a file.
    pub async fn append(&self, uri: &str, content: &str) -> Result<(), BoxError> {
        self.fs.append(uri, content).await
    }

    /// Check if a file or directory exists.
    pub async fn exists(&self, uri: &str) -> Result<bool, BoxError> {
        self.fs.exists(uri).await
    }

    // =======================================================================
    // L0/L1 Context Layer
    // =======================================================================

    /// Read directory's L0 summary (`.abstract.md`).
    ///
    /// Port of `VikingFS.abstract()`.
    pub async fn read_abstract(&self, uri: &str) -> Result<String, BoxError> {
        let stat = self.fs.stat(uri).await?;
        if !stat.is_dir {
            return Err(format!("{uri} is not a directory").into());
        }
        let abstract_uri = format!("{}/.abstract.md", uri.trim_end_matches('/'));
        self.fs.read(&abstract_uri).await
    }

    /// Read directory's L1 overview (`.overview.md`).
    ///
    /// Port of `VikingFS.overview()`.
    pub async fn read_overview(&self, uri: &str) -> Result<String, BoxError> {
        let stat = self.fs.stat(uri).await?;
        if !stat.is_dir {
            return Err(format!("{uri} is not a directory").into());
        }
        let overview_uri = format!("{}/.overview.md", uri.trim_end_matches('/'));
        self.fs.read(&overview_uri).await
    }

    /// Batch read abstracts or overviews for multiple URIs.
    ///
    /// Port of `VikingFS.read_batch()`.
    pub async fn batch_read(
        &self,
        uris: &[String],
        level: &str,
    ) -> HashMap<String, String> {
        let mut results = HashMap::new();
        for uri in uris {
            let content = match level {
                "l0" => self.read_abstract(uri).await.ok(),
                "l1" => self.read_overview(uri).await.ok(),
                _ => None,
            };
            if let Some(c) = content {
                results.insert(uri.clone(), c);
            }
        }
        results
    }

    // =======================================================================
    // Relation Management
    // =======================================================================

    /// Add a relation from `from_uri` to a set of target URIs.
    ///
    /// Port of `VikingFS.link()`.
    pub async fn add_relation(
        &self,
        from_uri: &str,
        target_uris: &[String],
        reason: &str,
    ) -> Result<(), BoxError> {
        let mut entries = self.read_relation_table(from_uri).await;
        let existing_ids: std::collections::HashSet<String> =
            entries.iter().map(|e| e.id.clone()).collect();

        // Generate next link_N id
        let link_id = (1..10000)
            .map(|i| format!("link_{i}"))
            .find(|id| !existing_ids.contains(id))
            .unwrap_or_else(|| "link_new".to_owned());

        entries.push(RelationEntry::new(link_id, target_uris.to_vec(), reason));
        self.write_relation_table(from_uri, &entries).await?;
        info!(
            "[VikingFs] Created relation: {from_uri} -> {:?}",
            target_uris
        );
        Ok(())
    }

    /// Remove a specific target URI from relations.
    ///
    /// Port of `VikingFS.unlink()`.
    pub async fn remove_relation(
        &self,
        from_uri: &str,
        target_uri: &str,
    ) -> Result<(), BoxError> {
        let mut entries = self.read_relation_table(from_uri).await;

        // Find the entry containing target_uri
        let mut entry_idx = None;
        for (i, entry) in entries.iter().enumerate() {
            if entry.uris.contains(&target_uri.to_owned()) {
                entry_idx = Some(i);
                break;
            }
        }

        let Some(idx) = entry_idx else {
            warn!("[VikingFs] URI not found in relations: {target_uri}");
            return Ok(());
        };

        // Remove the target URI from the entry
        entries[idx].uris.retain(|u| u != target_uri);

        // If entry has no more URIs, remove it entirely
        if entries[idx].uris.is_empty() {
            debug!(
                "[VikingFs] Removed empty entry: {}",
                entries[idx].id
            );
            entries.remove(idx);
        }

        self.write_relation_table(from_uri, &entries).await?;
        info!("[VikingFs] Removed relation: {from_uri} -> {target_uri}");
        Ok(())
    }

    /// Get all related URIs (flat list).
    ///
    /// Port of `VikingFS.get_relations()`.
    pub async fn get_relations(&self, uri: &str) -> Vec<String> {
        let entries = self.read_relation_table(uri).await;
        entries
            .into_iter()
            .flat_map(|e| e.uris)
            .collect()
    }

    /// Get full relation table entries.
    ///
    /// Port of `VikingFS.get_relation_table()`.
    pub async fn get_relation_table(&self, uri: &str) -> Vec<RelationEntry> {
        self.read_relation_table(uri).await
    }

    /// Get relations with content (L0/L1).
    ///
    /// Port of `VikingFS.get_relations_with_content()`.
    pub async fn get_relations_with_content(
        &self,
        uri: &str,
        include_l0: bool,
        include_l1: bool,
    ) -> Vec<HashMap<String, String>> {
        let relation_uris = self.get_relations(uri).await;
        if relation_uris.is_empty() {
            return Vec::new();
        }

        let abstracts = if include_l0 {
            self.batch_read(&relation_uris, "l0").await
        } else {
            HashMap::new()
        };
        let overviews = if include_l1 {
            self.batch_read(&relation_uris, "l1").await
        } else {
            HashMap::new()
        };

        relation_uris
            .iter()
            .map(|rel_uri| {
                let mut info = HashMap::new();
                info.insert("uri".to_owned(), rel_uri.clone());
                if include_l0 {
                    info.insert(
                        "abstract".to_owned(),
                        abstracts.get(rel_uri).cloned().unwrap_or_default(),
                    );
                }
                if include_l1 {
                    info.insert(
                        "overview".to_owned(),
                        overviews.get(rel_uri).cloned().unwrap_or_default(),
                    );
                }
                info
            })
            .collect()
    }

    // =======================================================================
    // Directory Traversal
    // =======================================================================

    /// Recursively list all contents with relative paths.
    ///
    /// Port of `VikingFS.tree()` (original format).
    pub async fn tree(
        &self,
        uri: &str,
        show_hidden: bool,
        node_limit: usize,
    ) -> Result<Vec<TreeEntry>, BoxError> {
        let mut all = Vec::new();
        self.walk_tree(uri, "", show_hidden, node_limit, &mut all)
            .await;
        Ok(all)
    }

    /// File glob pattern matching.
    ///
    /// Port of `VikingFS.glob()`.
    pub async fn glob(
        &self,
        pattern: &str,
        uri: &str,
        node_limit: usize,
    ) -> Result<Vec<String>, BoxError> {
        let entries = self.tree(uri, false, node_limit).await?;
        let base_uri = uri.trim_end_matches('/');

        let matches: Vec<String> = entries
            .iter()
            .filter(|e| Self::simple_glob_match(pattern, &e.rel_path))
            .map(|e| format!("{base_uri}/{}", e.rel_path))
            .collect();

        Ok(matches)
    }

    // =======================================================================
    // Context Writing
    // =======================================================================

    /// Write context to file system (L0/L1/L2 bundle).
    ///
    /// Port of `VikingFS.write_context()`.
    pub async fn write_context(
        &self,
        uri: &str,
        content: &str,
        abstract_text: &str,
        overview: &str,
        content_filename: &str,
        _is_leaf: bool,
    ) -> Result<(), BoxError> {
        let dir_uri = uri.trim_end_matches('/');

        // Ensure parent directories exist, then create this directory
        self.ensure_parent_dirs(dir_uri).await;
        let _ = self.fs.mkdir(dir_uri).await; // ignore "already exists"

        if !content.is_empty() {
            let content_uri = format!("{dir_uri}/{content_filename}");
            self.fs.write(&content_uri, content).await?;
        }

        if !abstract_text.is_empty() {
            let abstract_uri = format!("{dir_uri}/.abstract.md");
            self.fs.write(&abstract_uri, abstract_text).await?;
        }

        if !overview.is_empty() {
            let overview_uri = format!("{dir_uri}/.overview.md");
            self.fs.write(&overview_uri, overview).await?;
        }

        Ok(())
    }

    /// Write a file directly (with parent directory creation).
    ///
    /// Port of `VikingFS.write_file()`.
    pub async fn write_file(&self, uri: &str, content: &str) -> Result<(), BoxError> {
        self.ensure_parent_dirs(uri).await;
        self.fs.write(uri, content).await
    }

    // =======================================================================
    // Temp File Operations
    // =======================================================================

    /// Create a temporary URI.
    ///
    /// Port of `VikingFS.create_temp_uri()`.
    #[must_use]
    pub fn create_temp_uri() -> String {
        VikingUri::create_temp_uri().to_string()
    }

    /// Delete a temporary directory and all contents.
    ///
    /// Port of `VikingFS.delete_temp()`.
    pub async fn delete_temp(&self, temp_uri: &str) -> Result<(), BoxError> {
        // Recursive rm
        self.rm(temp_uri, true).await
    }

    // =======================================================================
    // Semantic Search
    // =======================================================================

    /// Simple semantic search.
    ///
    /// Port of `VikingFS.find()` — searches via vector store + embedder.
    pub async fn find(
        &self,
        query: &str,
        collection: &str,
        limit: usize,
        filter: Option<&HashMap<String, serde_json::Value>>,
    ) -> Result<Vec<VectorHit>, BoxError> {
        let embedder = self
            .embedder
            .as_ref()
            .ok_or("Embedder not configured")?;
        let vs = self
            .vector_store
            .as_ref()
            .ok_or("Vector store not configured")?;

        let embed_result = embedder.embed(query).await?;
        let sparse = embed_result.sparse_vector.as_ref();

        vs.search(
            collection,
            &embed_result.dense_vector,
            sparse,
            limit,
            filter,
        )
        .await
    }

    // =======================================================================
    // URI Conversion (static helpers, delegate to openviking-core)
    // =======================================================================

    /// Convert a Viking URI to a filesystem path.
    ///
    /// Port of `VikingFS._uri_to_path()`.
    #[must_use]
    pub fn uri_to_path(uri: &str) -> String {
        uri_to_fs_path(uri)
    }

    /// Convert a filesystem path to a Viking URI.
    ///
    /// Port of `VikingFS._path_to_uri()`.
    #[must_use]
    pub fn path_to_uri(path: &str) -> String {
        fs_path_to_uri(path)
    }

    // =======================================================================
    // Internal Helpers
    // =======================================================================

    /// Ensure parent directories exist for a given URI.
    async fn ensure_parent_dirs(&self, uri: &str) {
        // Split URI path and create each directory level
        let trimmed = uri.trim_start_matches("viking://");
        let parts: Vec<&str> = trimmed.split('/').collect();
        // We need to create parent dirs (not the last component itself if it's a file)
        // But since VikingFS always uses mkdir for the parent, we create up to len-1
        for i in 1..parts.len() {
            let parent = format!("viking://{}", parts[..i].join("/"));
            let _ = self.fs.mkdir(&parent).await;
        }
    }

    /// Recursively collect all child URIs (for rm/mv vector sync).
    async fn collect_child_uris(&self, uri: &str) -> Vec<String> {
        let mut uris = Vec::new();
        self.collect_recursive(uri, &mut uris).await;
        uris
    }

    #[allow(clippy::only_used_in_recursion)]
    async fn collect_recursive(&self, uri: &str, uris: &mut Vec<String>) {
        if let Ok(entries) = self.fs.ls(uri).await {
            for entry in entries {
                if entry.name == "." || entry.name == ".." {
                    continue;
                }
                let child_uri = format!("{}/{}", uri.trim_end_matches('/'), entry.name);
                if entry.is_dir {
                    // Box::pin to handle recursive async
                    Box::pin(self.collect_recursive(&child_uri, uris)).await;
                } else {
                    uris.push(child_uri);
                }
            }
        }
    }

    /// Delete vectors for removed files.
    ///
    /// Uses `remove_by_uri` for recursive deletion matching Python behavior.
    async fn sync_delete_vectors(&self, uris: &[String]) {
        let Some(ref vs) = self.vector_store else {
            return;
        };
        for uri in uris {
            match vs.remove_by_uri("context", uri).await {
                Ok(n) => debug!("[VikingFs] Deleted {n} vector(s) for: {uri}"),
                Err(e) => {
                    // Fallback: try single delete if remove_by_uri not implemented
                    if let Err(e2) = vs.delete("context", uri).await {
                        warn!("[VikingFs] Failed to delete vector for {uri}: {e} / {e2}");
                    } else {
                        debug!("[VikingFs] Deleted vector (fallback): {uri}");
                    }
                }
            }
        }
    }

    /// Update vector URIs after a move operation.
    ///
    /// Queries via `filter_query` to find actual record IDs, then updates
    /// URI fields — matching the Python `_update_vector_store_uris` behavior.
    async fn sync_move_vectors(
        &self,
        old_uris: &[String],
        old_base: &str,
        new_base: &str,
    ) {
        let Some(ref vs) = self.vector_store else {
            return;
        };
        for old_uri in old_uris {
            let new_uri = old_uri.replacen(old_base, new_base, 1);

            // Compute new parent_uri
            let new_parent = new_uri
                .rfind('/')
                .map(|i| new_uri[..i].to_owned())
                .unwrap_or_default();

            let mut update_fields = HashMap::new();
            update_fields.insert("uri".to_owned(), serde_json::json!(new_uri));
            if !new_parent.is_empty() {
                update_fields.insert("parent_uri".to_owned(), serde_json::json!(new_parent));
            }

            // Try filter_query first to find actual record ID (Python behavior)
            let mut filter = HashMap::new();
            filter.insert("uri".to_owned(), serde_json::json!(old_uri));

            let record_id = match vs.filter_query("context", &filter, 1, 0, None, None, false).await
            {
                Ok(records) if !records.is_empty() => records[0]
                    .get("id")
                    .and_then(|v| v.as_str())
                    .map(|s| s.to_owned()),
                _ => None,
            };

            let id = record_id.as_deref().unwrap_or(old_uri.as_str());
            if let Err(e) = vs.update("context", id, update_fields).await {
                warn!("[VikingFs] Failed to update vector URI {old_uri}: {e}");
            } else {
                debug!("[VikingFs] Updated vector: {old_uri} -> {new_uri}");
            }
        }
    }

    /// Read relation table from `.relations.json`.
    async fn read_relation_table(&self, uri: &str) -> Vec<RelationEntry> {
        let table_uri = format!(
            "{}/.relations.json",
            uri.trim_end_matches('/')
        );
        let content = match self.fs.read(&table_uri).await {
            Ok(c) if !c.trim().is_empty() => c,
            _ => return Vec::new(),
        };

        // Try flat list format first, then nested format
        if let Ok(entries) = serde_json::from_str::<Vec<RelationEntry>>(&content) {
            return entries;
        }

        // Fallback: nested {namespace: {user: [entries]}} format
        if let Ok(data) =
            serde_json::from_str::<HashMap<String, HashMap<String, Vec<RelationEntry>>>>(&content)
        {
            return data
                .into_values()
                .flat_map(|user_map| user_map.into_values().flatten())
                .collect();
        }

        warn!("[VikingFs] Failed to parse relation table at {table_uri}");
        Vec::new()
    }

    /// Write relation table as `.relations.json`.
    async fn write_relation_table(
        &self,
        uri: &str,
        entries: &[RelationEntry],
    ) -> Result<(), BoxError> {
        let table_uri = format!(
            "{}/.relations.json",
            uri.trim_end_matches('/')
        );
        let json = serde_json::to_string_pretty(entries)?;
        self.fs.write(&table_uri, &json).await
    }

    /// Recursive directory walk for `tree()`.
    async fn walk_tree(
        &self,
        uri: &str,
        rel_prefix: &str,
        show_hidden: bool,
        limit: usize,
        out: &mut Vec<TreeEntry>,
    ) {
        if out.len() >= limit {
            return;
        }
        let entries = match self.fs.ls(uri).await {
            Ok(e) => e,
            Err(_) => return,
        };
        for entry in entries {
            if out.len() >= limit {
                break;
            }
            let name = &entry.name;
            if name == "." || name == ".." {
                continue;
            }
            let rel_path = if rel_prefix.is_empty() {
                name.clone()
            } else {
                format!("{rel_prefix}/{name}")
            };
            let child_uri = format!("{}/{name}", uri.trim_end_matches('/'));

            if entry.is_dir {
                out.push(TreeEntry {
                    uri: child_uri.clone(),
                    rel_path: rel_path.clone(),
                    is_dir: true,
                    size: 0,
                });
                Box::pin(
                    self.walk_tree(&child_uri, &rel_path, show_hidden, limit, out),
                )
                .await;
            } else if !name.starts_with('.') || show_hidden {
                out.push(TreeEntry {
                    uri: child_uri,
                    rel_path,
                    is_dir: false,
                    size: entry.size,
                });
            }
        }
    }

    /// Simple glob pattern matching (supports `*` and `**`).
    fn simple_glob_match(pattern: &str, path: &str) -> bool {
        // Handle common patterns: *.md, **/*.rs, etc.
        if pattern == "*" {
            return true;
        }
        if let Some(ext) = pattern.strip_prefix("*.") {
            return path.ends_with(&format!(".{ext}"));
        }
        if let Some(rest) = pattern.strip_prefix("**/") {
            // Recursive: match any depth
            return Self::simple_glob_match(rest, path)
                || path
                    .split('/')
                    .enumerate()
                    .any(|(i, _)| {
                        let suffix = path.split('/').skip(i).collect::<Vec<_>>().join("/");
                        Self::simple_glob_match(rest, &suffix)
                    });
        }
        // Exact match fallback
        pattern == path
    }
}

// ---------------------------------------------------------------------------
// FileSystem trait impl — allows VikingFs to be used as a FileSystem backend
// ---------------------------------------------------------------------------

#[async_trait]
impl<FS: FileSystem, VS: VectorStore, EMB: Embedder> FileSystem for VikingFs<FS, VS, EMB> {
    async fn read(&self, uri: &str) -> Result<String, BoxError> {
        self.fs.read(uri).await
    }

    async fn read_bytes(&self, uri: &str) -> Result<Vec<u8>, BoxError> {
        self.fs.read_bytes(uri).await
    }

    async fn write(&self, uri: &str, content: &str) -> Result<(), BoxError> {
        self.fs.write(uri, content).await
    }

    async fn write_bytes(&self, uri: &str, content: &[u8]) -> Result<(), BoxError> {
        self.fs.write_bytes(uri, content).await
    }

    async fn mkdir(&self, uri: &str) -> Result<(), BoxError> {
        self.fs.mkdir(uri).await
    }

    async fn ls(&self, uri: &str) -> Result<Vec<FsEntry>, BoxError> {
        self.fs.ls(uri).await
    }

    /// Delegates to `VikingFs::rm` with `recursive=true` to include vector sync.
    async fn rm(&self, uri: &str) -> Result<(), BoxError> {
        // Use the high-level rm which includes vector store sync
        VikingFs::rm(self, uri, true).await
    }

    /// Delegates to `VikingFs::mv` to include vector sync.
    async fn mv(&self, from_uri: &str, to_uri: &str) -> Result<(), BoxError> {
        VikingFs::mv(self, from_uri, to_uri).await
    }

    async fn stat(&self, uri: &str) -> Result<FsStat, BoxError> {
        self.fs.stat(uri).await
    }

    async fn grep(
        &self,
        uri: &str,
        pattern: &str,
        recursive: bool,
        case_insensitive: bool,
    ) -> Result<Vec<GrepMatch>, BoxError> {
        self.fs.grep(uri, pattern, recursive, case_insensitive).await
    }

    async fn exists(&self, uri: &str) -> Result<bool, BoxError> {
        self.fs.exists(uri).await
    }

    async fn append(&self, uri: &str, content: &str) -> Result<(), BoxError> {
        self.fs.append(uri, content).await
    }

    async fn link(&self, source_uri: &str, target_uri: &str) -> Result<(), BoxError> {
        self.fs.link(source_uri, target_uri).await
    }
}

// ---------------------------------------------------------------------------
// TreeEntry
// ---------------------------------------------------------------------------

/// Entry in a tree listing.
#[derive(Debug, Clone)]
pub struct TreeEntry {
    /// Full Viking URI.
    pub uri: String,
    /// Relative path from the tree root.
    pub rel_path: String,
    /// Whether this is a directory.
    pub is_dir: bool,
    /// File size in bytes (0 for directories).
    pub size: u64,
}
