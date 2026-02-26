// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Session compressor — memory extraction pipeline orchestrator.
//!
//! Ported from `openviking/session/compressor.py`.

use std::collections::{HashMap, HashSet};

use log::{debug, error, info, warn};

use openviking_core::context::Context;
use openviking_core::message::Message;
use openviking_core::session_types::{
    CandidateMemory, DedupDecision, ExtractionStats, MemoryActionDecision,
    MemoryCategory,
};
use openviking_core::user::UserIdentifier;

use crate::deduplicator::MemoryDeduplicator;
use crate::extractor::MemoryExtractor;
use crate::traits::{BoxError, Embedder, FileSystem, LlmProvider, VectorStore};

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

/// Categories that always merge (skip dedup).
const ALWAYS_MERGE: &[MemoryCategory] = &[MemoryCategory::Profile];

/// Categories that support the MERGE decision.
const MERGE_SUPPORTED: &[MemoryCategory] = &[
    MemoryCategory::Preferences,
    MemoryCategory::Entities,
    MemoryCategory::Patterns,
];

// ---------------------------------------------------------------------------
// SessionCompressor
// ---------------------------------------------------------------------------

/// Orchestrates memory extraction + deduplication pipeline.
///
/// Combines [`MemoryExtractor`] and [`MemoryDeduplicator`] to process
/// session messages into long-term memories.
pub struct SessionCompressor<VS, EMB, LLM, FS>
where
    VS: VectorStore,
    EMB: Embedder,
    LLM: LlmProvider,
    FS: FileSystem,
{
    extractor: MemoryExtractor<LLM, FS>,
    deduplicator: MemoryDeduplicator<VS, EMB, LLM>,
    embedder: EMB,
    fs: FS,
    vs: VS,
}

impl<VS, EMB, LLM, FS> SessionCompressor<VS, EMB, LLM, FS>
where
    VS: VectorStore + Clone,
    EMB: Embedder + Clone,
    LLM: LlmProvider + Clone,
    FS: FileSystem + Clone,
{
    /// Create a new session compressor.
    pub fn new(vs: VS, embedder: EMB, llm: LLM, fs: FS) -> Self {
        Self {
            extractor: MemoryExtractor::new(llm.clone(), fs.clone()),
            deduplicator: MemoryDeduplicator::new(vs.clone(), embedder.clone(), llm),
            embedder,
            fs,
            vs,
        }
    }

    /// Extract long-term memories from session messages.
    pub async fn extract_long_term_memories(
        &self,
        messages: &[Message],
        user: &UserIdentifier,
        session_id: &str,
    ) -> Result<Vec<Context>, BoxError> {
        if messages.is_empty() {
            return Ok(Vec::new());
        }

        let candidates = self.extractor.extract(messages, user, session_id).await?;
        if candidates.is_empty() {
            return Ok(Vec::new());
        }

        let mut memories: Vec<Context> = Vec::new();
        let mut stats = ExtractionStats::default();

        for candidate in &candidates {
            // Profile: skip dedup, always merge
            if ALWAYS_MERGE.contains(&candidate.category) {
                match self
                    .extractor
                    .create_memory(candidate, user, session_id)
                    .await?
                {
                    Some(mem) => {
                        self.index_memory(&mem).await;
                        memories.push(mem);
                        stats.created += 1;
                    }
                    None => stats.skipped += 1,
                }
                continue;
            }

            // Dedup check
            let result = self.deduplicator.deduplicate(candidate).await?;
            let actions = result.actions.unwrap_or_default();
            let decision = result.decision;

            // Safety net: create+merge → none
            if decision == DedupDecision::Create
                && actions
                    .iter()
                    .any(|a| a.decision == MemoryActionDecision::Merge)
            {
                warn!(
                    "Dedup returned create with merge, normalizing to none: {}",
                    candidate.abstract_text
                );
                // Fall through to none handling below
                self.handle_none_actions(candidate, &actions, &mut stats)
                    .await;
                continue;
            }

            match decision {
                DedupDecision::Skip => {
                    stats.skipped += 1;
                }
                DedupDecision::None => {
                    self.handle_none_actions(candidate, &actions, &mut stats)
                        .await;
                }
                DedupDecision::Create => {
                    // Create can optionally carry delete actions first
                    for action in &actions {
                        if action.decision == MemoryActionDecision::Delete {
                            if self.delete_existing_memory(&action.memory_uri).await {
                                stats.deleted += 1;
                            } else {
                                stats.skipped += 1;
                            }
                        }
                    }
                    match self
                        .extractor
                        .create_memory(candidate, user, session_id)
                        .await?
                    {
                        Some(mem) => {
                            self.index_memory(&mem).await;
                            memories.push(mem);
                            stats.created += 1;
                        }
                        None => stats.skipped += 1,
                    }
                }
            }
        }

        // Extract URIs used in messages and create relations
        let used_uris = Self::extract_used_uris(messages);
        if !used_uris.is_empty() && !memories.is_empty() {
            self.create_relations(&memories, &used_uris).await;
        }

        info!(
            "Memory extraction: created={}, merged={}, deleted={}, skipped={}",
            stats.created, stats.merged, stats.deleted, stats.skipped
        );
        Ok(memories)
    }

    // -----------------------------------------------------------------------
    // Vector indexing (FIX-C1)
    // -----------------------------------------------------------------------

    /// Embed and upsert a Context into the vector store.
    async fn index_memory(&self, memory: &Context) {
        let text = format!("{} {}", memory.abstract_text, memory.uri);
        let embed_result = match self.embedder.embed(&text).await {
            Ok(r) => r,
            Err(e) => {
                error!("Failed to embed memory {}: {e}", memory.uri);
                return;
            }
        };

        let mut fields = HashMap::new();
        fields.insert("uri".to_owned(), serde_json::json!(memory.uri));
        fields.insert("abstract".to_owned(), serde_json::json!(memory.abstract_text));
        fields.insert("context_type".to_owned(), serde_json::json!("memory"));
        fields.insert("is_leaf".to_owned(), serde_json::json!(memory.is_leaf));
        fields.insert("parent_uri".to_owned(), serde_json::json!(memory.parent_uri));
        fields.insert("category".to_owned(), serde_json::json!(memory.category));

        if let Err(e) = self.vs.upsert(
            "context",
            &memory.uri,
            &embed_result.dense_vector,
            fields,
        ).await {
            error!("Failed to index memory {}: {e}", memory.uri);
        } else {
            info!("Indexed memory: {}", memory.uri);
        }
    }

    // -----------------------------------------------------------------------
    // Internal helpers
    // -----------------------------------------------------------------------

    async fn handle_none_actions(
        &self,
        candidate: &CandidateMemory,
        actions: &[openviking_core::session_types::ExistingMemoryAction],
        stats: &mut ExtractionStats,
    ) {
        if actions.is_empty() {
            stats.skipped += 1;
            return;
        }

        for action in actions {
            match action.decision {
                MemoryActionDecision::Delete => {
                    if self.delete_existing_memory(&action.memory_uri).await {
                        stats.deleted += 1;
                    } else {
                        stats.skipped += 1;
                    }
                }
                MemoryActionDecision::Merge => {
                    if MERGE_SUPPORTED.contains(&candidate.category) {
                        if self
                            .merge_into_existing(candidate, &action.memory_uri)
                            .await
                        {
                            stats.merged += 1;
                        } else {
                            stats.skipped += 1;
                        }
                    } else {
                        stats.skipped += 1;
                    }
                }
            }
        }
    }

    async fn merge_into_existing(
        &self,
        candidate: &CandidateMemory,
        target_uri: &str,
    ) -> bool {
        let existing_content = match self.fs.read(target_uri).await {
            Ok(c) => c,
            Err(e) => {
                error!("Failed to read target memory {target_uri}: {e}");
                return false;
            }
        };

        let payload = match self
            .extractor
            .merge_memory_bundle(
                "", "", &existing_content,
                &candidate.abstract_text, &candidate.overview,
                &candidate.content, &format!("{:?}", candidate.category),
                &candidate.language,
            )
            .await
        {
            Ok(Some(p)) => p,
            Ok(None) => return false,
            Err(e) => {
                error!("Merge failed for {target_uri}: {e}");
                return false;
            }
        };

        if let Err(e) = self.fs.write(target_uri, &payload.content).await {
            error!("Failed to write merged memory {target_uri}: {e}");
            return false;
        }

        // FIX-C2: re-index merged memory with updated content
        let merged_ctx = Context::new(target_uri, &payload.abstract_text);
        self.index_memory(&merged_ctx).await;

        info!("Merged memory {target_uri}");
        true
    }

    async fn delete_existing_memory(&self, uri: &str) -> bool {
        if let Err(e) = self.fs.rm(uri).await {
            error!("Failed to delete memory file {uri}: {e}");
            return false;
        }

        // FIX-C3 + GAP-COMPRESSOR: use remove_by_uri for recursive cleanup
        match self.vs.remove_by_uri("context", uri).await {
            Ok(_) => {}
            Err(_) => {
                // Fallback to single delete if remove_by_uri not implemented
                if let Err(e) = self.vs.delete("context", uri).await {
                    warn!("Failed to remove vector record for {uri}: {e}");
                }
            }
        }

        debug!("Deleted memory {uri}");
        true
    }

    fn extract_used_uris(messages: &[Message]) -> HashMap<String, Vec<String>> {
        use openviking_core::message::Part;

        let mut resources: HashSet<String> = HashSet::new();
        let mut skills: HashSet<String> = HashSet::new();

        for msg in messages {
            for part in &msg.parts {
                match part {
                    Part::Context { uri, context_type, .. } => {
                        match context_type.as_str() {
                            "skill" => { skills.insert(uri.clone()); }
                            _ => { resources.insert(uri.clone()); }
                        }
                    }
                    Part::Tool { skill_uri, tool_uri, .. } => {
                        if !skill_uri.is_empty() {
                            skills.insert(skill_uri.clone());
                        }
                        if !tool_uri.is_empty() {
                            resources.insert(tool_uri.clone());
                        }
                    }
                    Part::Text { .. } => {}
                }
            }
        }

        let mut result: HashMap<String, Vec<String>> = HashMap::new();
        if !resources.is_empty() {
            result.insert("resources".to_owned(), resources.into_iter().collect());
        }
        if !skills.is_empty() {
            result.insert("skills".to_owned(), skills.into_iter().collect());
        }
        result
    }

    async fn create_relations(
        &self,
        memories: &[Context],
        used_uris: &HashMap<String, Vec<String>>,
    ) {
        let resource_uris = used_uris.get("resources").cloned().unwrap_or_default();
        let skill_uris = used_uris.get("skills").cloned().unwrap_or_default();
        let memory_uris: Vec<&str> = memories.iter().map(|m| m.uri.as_str()).collect();

        // Forward: memory → resources/skills
        for mem in memories {
            for res_uri in &resource_uris {
                if let Err(e) = self.fs.link(&mem.uri, res_uri).await {
                    warn!("Failed to create relation {} -> {}: {e}", mem.uri, res_uri);
                }
            }
            for skill_uri in &skill_uris {
                if let Err(e) = self.fs.link(&mem.uri, skill_uri).await {
                    warn!("Failed to create relation {} -> {}: {e}", mem.uri, skill_uri);
                }
            }
        }

        // FIX-C5: Reverse: resources/skills → memories
        for res_uri in &resource_uris {
            for mem_uri in &memory_uris {
                if let Err(e) = self.fs.link(res_uri, mem_uri).await {
                    warn!("Failed to create reverse relation {} -> {}: {e}", res_uri, mem_uri);
                }
            }
        }
        for skill_uri in &skill_uris {
            for mem_uri in &memory_uris {
                if let Err(e) = self.fs.link(skill_uri, mem_uri).await {
                    warn!("Failed to create reverse relation {} -> {}: {e}", skill_uri, mem_uri);
                }
            }
        }
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use crate::traits::{BoxError, EmbedResult, FsEntry, VectorHit};
    use async_trait::async_trait;

    #[derive(Clone)]
    struct Vs;
    #[async_trait]
    impl VectorStore for Vs {
        async fn search(&self, _: &str, _: &[f32], _: Option<&HashMap<String, f64>>, _: usize, _: Option<&HashMap<String, serde_json::Value>>) -> Result<Vec<VectorHit>, BoxError> { Ok(Vec::new()) }
        async fn upsert(&self, _: &str, _: &str, _: &[f32], _: HashMap<String, serde_json::Value>) -> Result<(), BoxError> { Ok(()) }
        async fn update(&self, _: &str, _: &str, _: HashMap<String, serde_json::Value>) -> Result<(), BoxError> { Ok(()) }
        async fn delete(&self, _: &str, _: &str) -> Result<(), BoxError> { Ok(()) }
    }

    #[derive(Clone)]
    struct Emb;
    #[async_trait]
    impl Embedder for Emb {
        async fn embed(&self, _: &str) -> Result<EmbedResult, BoxError> {
            Ok(EmbedResult { dense_vector: Vec::new(), sparse_vector: None })
        }
    }

    #[derive(Clone)]
    struct Llm;
    #[async_trait]
    impl LlmProvider for Llm {
        async fn completion(&self, _: &str) -> Result<String, BoxError> {
            Ok(r#"{"memories":[]}"#.to_owned())
        }
    }

    #[derive(Clone)]
    struct Fs;
    #[async_trait]
    impl FileSystem for Fs {
        async fn read(&self, _: &str) -> Result<String, BoxError> { Ok(String::new()) }
        async fn read_bytes(&self, _: &str) -> Result<Vec<u8>, BoxError> { Ok(Vec::new()) }
        async fn write(&self, _: &str, _: &str) -> Result<(), BoxError> { Ok(()) }
        async fn write_bytes(&self, _: &str, _: &[u8]) -> Result<(), BoxError> { Ok(()) }
        async fn mkdir(&self, _: &str) -> Result<(), BoxError> { Ok(()) }
        async fn ls(&self, _: &str) -> Result<Vec<FsEntry>, BoxError> { Ok(Vec::new()) }
        async fn rm(&self, _: &str) -> Result<(), BoxError> { Ok(()) }
        async fn mv(&self, _: &str, _: &str) -> Result<(), BoxError> { Ok(()) }
        async fn stat(&self, _: &str) -> Result<crate::traits::FsStat, BoxError> { Err("not implemented".into()) }
        async fn grep(&self, _: &str, _: &str, _: bool, _: bool) -> Result<Vec<crate::traits::GrepMatch>, BoxError> { Ok(Vec::new()) }
        async fn exists(&self, _: &str) -> Result<bool, BoxError> { Ok(false) }
        async fn append(&self, _: &str, _: &str) -> Result<(), BoxError> { Ok(()) }
        async fn link(&self, _: &str, _: &str) -> Result<(), BoxError> { Ok(()) }
    }

    #[tokio::test]
    async fn empty_messages_returns_empty() {
        let comp = SessionCompressor::new(Vs, Emb, Llm, Fs);
        let user = UserIdentifier::default_user();
        let result = comp.extract_long_term_memories(&[], &user, "s1").await.unwrap();
        assert!(result.is_empty());
    }
}
