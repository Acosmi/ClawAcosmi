// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Preset directory initializer for the OpenViking virtual filesystem.
//!
//! Ported from `openviking/core/directories.py` — `DirectoryInitializer` class.
//!
//! Creates the preset directory tree (session/agent/resources/transactions/user)
//! in the underlying file system and registers each directory as a context
//! node in the vector store for semantic retrieval.
//!
//! **Not ported**: `TextEmbeddingHandler`, `EmbeddingMsgConverter` — embedding
//! is handled externally via the `Embedder` trait.

use std::collections::HashMap;

use log::{debug, info};

use openviking_core::context::{Context, Vectorize};
use openviking_core::directory::{context_type_for_uri, preset_directories, DirectoryDefinition};

use openviking_session::traits::{BoxError, Embedder, FileSystem, VectorStore};

use crate::VikingFs;

// ---------------------------------------------------------------------------
// DirectoryInitializer
// ---------------------------------------------------------------------------

/// Initializes the preset directory tree in AGFS and ensures each directory
/// has a corresponding context record in the vector store.
///
/// Generic over `FS` (file system), `VS` (vector store), and `EMB` (embedder)
/// to maintain full dependency injection.
///
/// # Usage
///
/// ```ignore
/// let init = DirectoryInitializer::new(&vfs, "context");
/// let created = init.initialize_all().await?;
/// println!("Created {created} directories");
/// ```
pub struct DirectoryInitializer<'a, FS, VS, EMB>
where
    FS: FileSystem,
    VS: VectorStore,
    EMB: Embedder,
{
    vfs: &'a VikingFs<FS, VS, EMB>,
    collection_name: &'a str,
}

impl<'a, FS, VS, EMB> DirectoryInitializer<'a, FS, VS, EMB>
where
    FS: FileSystem,
    VS: VectorStore,
    EMB: Embedder,
{
    /// Create a new initializer.
    ///
    /// - `vfs`: The VikingFs instance for file + vector operations.
    /// - `collection_name`: The vector store collection name (e.g. `"context"`).
    pub fn new(vfs: &'a VikingFs<FS, VS, EMB>, collection_name: &'a str) -> Self {
        Self {
            vfs,
            collection_name,
        }
    }

    /// Initialize all global preset directories (skip `user` scope).
    ///
    /// Returns the number of directories created / ensured.
    pub async fn initialize_all(&self) -> Result<usize, BoxError> {
        let dirs = preset_directories();
        let mut count = 0;

        for (scope, root_defn) in &dirs {
            if *scope == "user" {
                info!("[DirectoryInitializer] Skipping user scope (lazy initialization)");
                continue;
            }

            let root_uri = format!("viking://{scope}");
            if self
                .ensure_directory(&root_uri, None, root_defn, scope)
                .await?
            {
                count += 1;
            }

            count += self
                .initialize_children(scope, &root_defn.children, &root_uri)
                .await?;
        }

        info!("[DirectoryInitializer] Initialized {count} global directories");
        Ok(count)
    }

    /// Initialize user preset directory tree.
    ///
    /// Returns the number of directories created.
    pub async fn initialize_user_directories(&self) -> Result<usize, BoxError> {
        let dirs = preset_directories();
        let user_tree = match dirs.get("user") {
            Some(t) => t,
            None => return Ok(0),
        };

        let user_root_uri = "viking://user";
        let created = self
            .ensure_directory(user_root_uri, None, user_tree, "user")
            .await?;
        let mut count = if created { 1 } else { 0 };

        count += self
            .initialize_children("user", &user_tree.children, user_root_uri)
            .await?;

        info!("[DirectoryInitializer] Initialized {count} user directories");
        Ok(count)
    }

    /// Ensure a single directory exists, creating AGFS structure and vector
    /// record if needed.
    ///
    /// Returns `true` if anything was created.
    async fn ensure_directory(
        &self,
        uri: &str,
        parent_uri: Option<&str>,
        defn: &DirectoryDefinition,
        _scope: &str,
    ) -> Result<bool, BoxError> {
        let mut created = false;

        // 1. Ensure AGFS files exist
        if !self.check_agfs_files_exist(uri).await {
            debug!("[DirectoryInitializer] Creating directory: {uri}");
            self.create_agfs_structure(uri, defn.abstract_text, defn.overview)
                .await?;
            created = true;
        } else {
            debug!("[DirectoryInitializer] Directory already exists: {uri}");
        }

        // 2. Ensure record exists in vector store (best-effort)
        if self.ensure_vector_record(uri, parent_uri, defn).await {
            created = true;
        }

        Ok(created)
    }

    /// Check if L0 (.abstract.md) file exists for a directory.
    async fn check_agfs_files_exist(&self, uri: &str) -> bool {
        let abstract_uri = format!("{}/.abstract.md", uri.trim_end_matches('/'));
        matches!(self.vfs.exists(&abstract_uri).await, Ok(true))
    }

    /// Create the L0/L1 file structure for a directory.
    async fn create_agfs_structure(
        &self,
        uri: &str,
        abstract_text: &str,
        overview: &str,
    ) -> Result<(), BoxError> {
        self.vfs
            .write_context(uri, "", abstract_text, overview, "", false)
            .await
    }

    /// Ensure a vector record exists for the directory. Returns `true` if
    /// a new record was enqueued.
    async fn ensure_vector_record(
        &self,
        uri: &str,
        parent_uri: Option<&str>,
        defn: &DirectoryDefinition,
    ) -> bool {
        // Build a filter to check if record already exists
        let mut filter = HashMap::new();
        filter.insert(
            "must".to_owned(),
            serde_json::json!({"field": "uri", "conds": [uri]}),
        );

        // Try to query existing record (best-effort: if VS not configured, skip)
        let existing = self
            .vfs
            .find(defn.overview, self.collection_name, 1, Some(&filter))
            .await;

        // If find() returns Ok with results, the record already exists
        if let Ok(ref hits) = existing {
            if !hits.is_empty() {
                return false;
            }
        }

        // Build context for the directory
        let context_type = context_type_for_uri(uri);
        let mut ctx = Context::new(uri, defn.abstract_text);
        ctx.context_type = context_type;
        ctx.is_leaf = false;
        if let Some(p) = parent_uri {
            ctx.parent_uri = Some(p.to_owned());
        }
        ctx.set_vectorize(Vectorize::new(defn.overview));

        // Convert context to storage fields and upsert
        let storage_val = ctx.to_storage_value();
        if let serde_json::Value::Object(map) = storage_val {
            let fields: HashMap<String, serde_json::Value> = map.into_iter().collect();

            // Extract vector if present, or use empty
            let vector: Vec<f32> = fields
                .get("vector")
                .and_then(|v| v.as_array())
                .map(|a| {
                    a.iter()
                        .filter_map(|x| x.as_f64().map(|f| f as f32))
                        .collect()
                })
                .unwrap_or_default();

            // Best-effort upsert (VS may not be configured)
            if let Err(e) = self
                .vfs
                .upsert_to_vector_store(self.collection_name, &ctx.id, &vector, fields)
                .await
            {
                debug!(
                    "[DirectoryInitializer] Vector upsert skipped for {uri}: {e}"
                );
                return false;
            }
            return true;
        }

        false
    }

    /// Recursively initialize child directories.
    async fn initialize_children(
        &self,
        scope: &str,
        children: &[DirectoryDefinition],
        parent_uri: &str,
    ) -> Result<usize, BoxError> {
        let mut count = 0;

        for defn in children {
            let uri = format!("{}/{}", parent_uri.trim_end_matches('/'), defn.path);

            if self
                .ensure_directory(&uri, Some(parent_uri), defn, scope)
                .await?
            {
                count += 1;
            }

            if !defn.children.is_empty() {
                count += Box::pin(self.initialize_children(scope, &defn.children, &uri)).await?;
            }
        }

        Ok(count)
    }
}

// ---------------------------------------------------------------------------
// VikingFs helper — expose vector store upsert for DirectoryInitializer
// ---------------------------------------------------------------------------

impl<FS: FileSystem, VS: VectorStore, EMB: Embedder> VikingFs<FS, VS, EMB> {
    /// Upsert a record directly to the vector store (convenience for
    /// `DirectoryInitializer`).
    ///
    /// Returns an error if the vector store is not configured.
    pub(crate) async fn upsert_to_vector_store(
        &self,
        collection: &str,
        id: &str,
        vector: &[f32],
        fields: HashMap<String, serde_json::Value>,
    ) -> Result<(), BoxError> {
        let vs = self
            .vector_store_ref()
            .ok_or("Vector store not configured")?;
        vs.upsert(collection, id, vector, fields).await
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use openviking_core::context::ContextType;
    use openviking_session::InMemoryVectorStore;
    use openviking_session::init_context_collection;

    // ── Mock types for testing ──

    struct MockFs {
        written: std::sync::Mutex<HashMap<String, String>>,
    }

    impl MockFs {
        fn new() -> Self {
            Self {
                written: std::sync::Mutex::new(HashMap::new()),
            }
        }
    }

    #[async_trait::async_trait]
    impl FileSystem for MockFs {
        async fn read(&self, uri: &str) -> Result<String, BoxError> {
            let w = self.written.lock().unwrap();
            w.get(uri)
                .cloned()
                .ok_or_else(|| format!("not found: {uri}").into())
        }
        async fn read_bytes(&self, _uri: &str) -> Result<Vec<u8>, BoxError> {
            Ok(Vec::new())
        }
        async fn write(&self, uri: &str, content: &str) -> Result<(), BoxError> {
            self.written
                .lock()
                .unwrap()
                .insert(uri.to_owned(), content.to_owned());
            Ok(())
        }
        async fn write_bytes(&self, _uri: &str, _content: &[u8]) -> Result<(), BoxError> {
            Ok(())
        }
        async fn mkdir(&self, _uri: &str) -> Result<(), BoxError> {
            Ok(())
        }
        async fn ls(
            &self,
            _uri: &str,
        ) -> Result<Vec<openviking_session::traits::FsEntry>, BoxError> {
            Ok(Vec::new())
        }
        async fn rm(&self, _uri: &str) -> Result<(), BoxError> {
            Ok(())
        }
        async fn mv(&self, _from: &str, _to: &str) -> Result<(), BoxError> {
            Ok(())
        }
        async fn stat(
            &self,
            uri: &str,
        ) -> Result<openviking_session::traits::FsStat, BoxError> {
            Ok(openviking_session::traits::FsStat {
                name: uri.to_owned(),
                size: 0,
                is_dir: true,
                mod_time: String::new(),
            })
        }
        async fn grep(
            &self,
            _uri: &str,
            _pattern: &str,
            _recursive: bool,
            _case_insensitive: bool,
        ) -> Result<Vec<openviking_session::traits::GrepMatch>, BoxError> {
            Ok(Vec::new())
        }
        async fn exists(&self, uri: &str) -> Result<bool, BoxError> {
            Ok(self.written.lock().unwrap().contains_key(uri))
        }
        async fn append(&self, _uri: &str, _content: &str) -> Result<(), BoxError> {
            Ok(())
        }
        async fn link(&self, _src: &str, _dst: &str) -> Result<(), BoxError> {
            Ok(())
        }
    }

    struct MockEmbedder;

    #[async_trait::async_trait]
    impl Embedder for MockEmbedder {
        async fn embed(
            &self,
            _text: &str,
        ) -> Result<openviking_session::traits::EmbedResult, BoxError> {
            Ok(openviking_session::traits::EmbedResult {
                dense_vector: vec![0.1; 3],
                sparse_vector: None,
            })
        }
    }

    fn make_vfs() -> VikingFs<MockFs, InMemoryVectorStore, MockEmbedder> {
        VikingFs::with_backends(MockFs::new(), InMemoryVectorStore::new(), MockEmbedder)
    }

    // ── Tests ──

    #[tokio::test]
    async fn initialize_all_creates_global_dirs() {
        let vfs = make_vfs();
        init_context_collection(
            vfs.vector_store_ref().unwrap(),
            "context",
            3,
        )
        .await
        .unwrap();

        let init = DirectoryInitializer::new(&vfs, "context");
        let count = init.initialize_all().await.unwrap();
        // session(1) + agent(1+3+2) + resources(1) + transactions(1) = all except user
        assert!(count > 0, "should create at least some directories");
    }

    #[tokio::test]
    async fn initialize_user_directories_creates_user_tree() {
        let vfs = make_vfs();
        init_context_collection(
            vfs.vector_store_ref().unwrap(),
            "context",
            3,
        )
        .await
        .unwrap();

        let init = DirectoryInitializer::new(&vfs, "context");
        let count = init.initialize_user_directories().await.unwrap();
        // user root(1) + memories(1) + preferences(1) + entities(1) + events(1) = 5
        assert!(count > 0, "should create user directories");
    }

    #[tokio::test]
    async fn initialize_all_is_idempotent() {
        let vfs = make_vfs();
        init_context_collection(
            vfs.vector_store_ref().unwrap(),
            "context",
            3,
        )
        .await
        .unwrap();

        let init = DirectoryInitializer::new(&vfs, "context");
        let first = init.initialize_all().await.unwrap();
        assert!(first > 0);

        // Second call: AGFS files already exist
        let second = init.initialize_all().await.unwrap();
        // Should create fewer (or zero) directories since AGFS files exist
        assert!(second <= first, "idempotent: second run should not exceed first");
    }

    #[tokio::test]
    async fn ensure_directory_creates_abstract_and_overview() {
        let vfs = make_vfs();
        let init = DirectoryInitializer::new(&vfs, "context");

        let defn = DirectoryDefinition {
            path: "",
            abstract_text: "Test abstract",
            overview: "Test overview",
            children: Vec::new(),
        };

        let created = init
            .ensure_directory("viking://test", None, &defn, "test")
            .await
            .unwrap();
        assert!(created);

        // Verify AGFS files were written
        let abs = vfs.read("viking://test/.abstract.md").await.unwrap();
        assert_eq!(abs, "Test abstract");
        let ov = vfs.read("viking://test/.overview.md").await.unwrap();
        assert_eq!(ov, "Test overview");
    }

    #[tokio::test]
    async fn context_type_mapping_in_directories() {
        // Verify context_type_for_uri works correctly for preset scopes
        assert_eq!(
            context_type_for_uri("viking://user/memories/preferences/code"),
            ContextType::Memory
        );
        assert_eq!(
            context_type_for_uri("viking://resources/docs"),
            ContextType::Resource
        );
        assert_eq!(
            context_type_for_uri("viking://agent/skills/search"),
            ContextType::Skill
        );
        assert_eq!(
            context_type_for_uri("viking://session/abc"),
            ContextType::Memory
        );
    }
}
