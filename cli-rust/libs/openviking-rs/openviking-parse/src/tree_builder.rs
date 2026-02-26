// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Tree builder — migrates parsed resources from temp to AGFS.
//!
//! Ported from `openviking/parse/tree_builder.py` (294L).
//! All file-system operations are delegated to the injected
//! [`FileSystem`](openviking_session::traits::FileSystem) trait.

use std::sync::Arc;

use log::{error, info, warn};

use openviking_core::uri::{self, VikingUri};
use openviking_core::tree::BuildingTree;
use openviking_session::traits::{BoxError, FileSystem};

// ---------------------------------------------------------------------------
// Enqueue callback
// ---------------------------------------------------------------------------

/// Callback trait for enqueuing semantic generation messages.
///
/// Consumers provide an implementation that bridges to their concrete
/// `QueueManager`, keeping `TreeBuilder` decoupled from queue internals.
#[async_trait::async_trait]
pub trait SemanticEnqueuer: Send + Sync {
    /// Enqueue a URI for semantic processing.
    ///
    /// # Arguments
    /// * `uri` — the AGFS URI to process.
    /// * `context_type` — e.g. `"resource"`, `"memory"`, `"skill"`.
    async fn enqueue(&self, uri: &str, context_type: &str) -> Result<(), BoxError>;
}

// ---------------------------------------------------------------------------
// TreeBuilder
// ---------------------------------------------------------------------------

/// Builds OpenViking context tree from parsed documents (v5.0 architecture).
///
/// Process flow:
/// 1. Parser creates directory structure in temp VikingFS.
/// 2. `finalize_from_temp()` moves to AGFS, enqueues to SemanticQueue,
///    creates resources.
/// 3. SemanticProcessor generates `.abstract.md` and `.overview.md`
///    asynchronously.
pub struct TreeBuilder<FS: FileSystem> {
    fs: Arc<FS>,
    enqueuer: Option<Arc<dyn SemanticEnqueuer>>,
}

impl<FS: FileSystem + 'static> TreeBuilder<FS> {
    /// Create a new `TreeBuilder`.
    ///
    /// # Arguments
    /// * `fs` — shared file-system reference.
    /// * `enqueuer` — optional semantic-queue enqueuer.
    pub fn new(fs: Arc<FS>, enqueuer: Option<Arc<dyn SemanticEnqueuer>>) -> Self {
        Self { fs, enqueuer }
    }

    /// Finalize a parsed tree from a temporary directory.
    ///
    /// 1. Locates the single document sub-directory inside `temp_dir_path`.
    /// 2. Resolves a unique target URI under `base_uri`.
    /// 3. Moves the directory tree from temp → final AGFS location.
    /// 4. Cleans up the temporary root.
    /// 5. Enqueues semantic generation (if enqueuer is set).
    ///
    /// Returns a [`BuildingTree`] with the final root URI set.
    pub async fn finalize_from_temp(
        &self,
        temp_dir_path: &str,
        _scope: &str,
        base_uri: &str,
        source_path: Option<&str>,
        source_format: Option<&str>,
    ) -> Result<BuildingTree, BoxError> {
        // 1. Find the single document directory inside temp.
        let entries = self.fs.ls(temp_dir_path).await?;
        let doc_dirs: Vec<_> = entries
            .iter()
            .filter(|e| e.is_dir && e.name != "." && e.name != "..")
            .collect();

        if doc_dirs.len() != 1 {
            return Err(format!(
                "[TreeBuilder] Expected 1 document directory in {}, found {}",
                temp_dir_path,
                doc_dirs.len()
            )
            .into());
        }

        let doc_name = uri::sanitize_segment(&doc_dirs[0].name);
        let temp_doc_uri = format!("{temp_dir_path}/{doc_name}");

        // 2. Resolve unique URI.
        let candidate_uri = match VikingUri::parse(base_uri) {
            Ok(u) => match u.join(&doc_name) {
                Ok(joined) => joined.to_string(),
                Err(_) => format!("{base_uri}/{doc_name}"),
            },
            Err(_) => format!("{base_uri}/{doc_name}"),
        };

        let final_uri = self.resolve_unique_uri(&candidate_uri, 100).await?;
        if final_uri != candidate_uri {
            info!("[TreeBuilder] Resolved name conflict: {candidate_uri} -> {final_uri}");
        } else {
            info!("[TreeBuilder] Finalizing from temp: {final_uri}");
        }

        // 3. Move directory tree.
        self.move_directory_in_agfs(&temp_doc_uri, &final_uri)
            .await?;
        info!("[TreeBuilder] Moved temp tree: {temp_doc_uri} -> {final_uri}");

        // 4. Cleanup temp root.
        if let Err(e) = self.fs.rm(temp_dir_path).await {
            warn!("[TreeBuilder] Failed to cleanup temp root: {e}");
        }

        // 5. Enqueue semantic generation.
        if let Some(enqueuer) = &self.enqueuer {
            match enqueuer.enqueue(&final_uri, "resource").await {
                Ok(()) => info!("[TreeBuilder] Enqueued semantic generation for: {final_uri}"),
                Err(e) => error!("[TreeBuilder] Failed to enqueue semantic generation: {e}"),
            }
        }

        // 6. Return a BuildingTree.
        let mut tree = match (source_path, source_format) {
            (Some(p), Some(f)) => BuildingTree::with_source(p, f),
            _ => BuildingTree::new(),
        };
        tree.set_root(final_uri);

        Ok(tree)
    }

    /// Resolve a URI that does not collide with an existing resource.
    ///
    /// If `uri` is free, returns it unchanged. Otherwise appends `_1`, `_2`,
    /// … until a free name is found (up to `max_attempts`).
    pub async fn resolve_unique_uri(
        &self,
        uri: &str,
        max_attempts: usize,
    ) -> Result<String, BoxError> {
        if !self.fs.exists(uri).await? {
            return Ok(uri.to_owned());
        }

        for i in 1..=max_attempts {
            let candidate = format!("{uri}_{i}");
            if !self.fs.exists(&candidate).await? {
                return Ok(candidate);
            }
        }

        Err(format!(
            "Cannot resolve unique name for {uri} after {max_attempts} attempts"
        )
        .into())
    }

    /// Recursively move an AGFS directory tree (copy + delete).
    pub async fn move_directory_in_agfs(
        &self,
        src_uri: &str,
        dst_uri: &str,
    ) -> Result<(), BoxError> {
        // 1. Create target directory.
        self.fs.mkdir(dst_uri).await?;

        // 2. List source directory contents.
        let entries = self.fs.ls(src_uri).await?;

        for entry in &entries {
            if entry.name.is_empty() || entry.name == "." || entry.name == ".." {
                continue;
            }

            let src_item = format!("{src_uri}/{}", entry.name);
            let dst_item = format!("{dst_uri}/{}", entry.name);

            if entry.is_dir {
                // Recursively move subdirectory.
                Box::pin(self.move_directory_in_agfs(&src_item, &dst_item)).await?;
            } else {
                // Move file.
                self.fs.mv(&src_item, &dst_item).await?;
            }
        }

        // 3. Delete source directory (should be empty now).
        let _ = self.fs.rm(src_uri).await;

        Ok(())
    }
}
