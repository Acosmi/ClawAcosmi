// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Hierarchical retriever — recursive tree search with score propagation.
//!
//! Ported from `openviking/retrieve/hierarchical_retriever.py`.

use std::collections::{BinaryHeap, HashMap, HashSet};
use std::cmp::Ordering;

use log::{debug, warn};

use openviking_core::retrieve_types::{
    MatchedContext, QueryResult, RelatedContext, RetrieveContextType,
    ThinkingTrace, TypedQuery,
};

use crate::traits::{BoxError, Embedder, FileSystem, Reranker, VectorHit, VectorStore};

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

/// Max rounds of stable top-k before stopping.
pub const MAX_CONVERGENCE_ROUNDS: usize = 3;
/// Max related URIs to fetch.
pub const MAX_RELATIONS: usize = 5;
/// Score propagation factor: final = α*child + (1-α)*parent.
pub const SCORE_PROPAGATION_ALPHA: f64 = 0.5;
/// Global-search top-k.
pub const GLOBAL_SEARCH_TOPK: usize = 3;

// ---------------------------------------------------------------------------
// Internal types
// ---------------------------------------------------------------------------

/// Score-ordered URI for the priority queue.
#[derive(Debug, Clone, PartialEq)]
struct ScoredUri {
    score: f64,
    uri: String,
}

impl Eq for ScoredUri {}

impl PartialOrd for ScoredUri {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}

impl Ord for ScoredUri {
    fn cmp(&self, other: &Self) -> Ordering {
        self.score
            .partial_cmp(&other.score)
            .unwrap_or(Ordering::Equal)
    }
}

/// Scored candidate during recursive search.
#[derive(Debug, Clone)]
struct ScoredCandidate {
    hit: VectorHit,
    final_score: f64,
}

// ---------------------------------------------------------------------------
// HierarchicalRetriever
// ---------------------------------------------------------------------------

/// Recursive tree-based retrieval with score propagation.
pub struct HierarchicalRetriever<VS: VectorStore, EMB: Embedder, FS: FileSystem, RR: Reranker> {
    vs: VS,
    embedder: EMB,
    fs: FS,
    reranker: Option<RR>,
    threshold: f64,
}

impl<VS: VectorStore, EMB: Embedder, FS: FileSystem, RR: Reranker> HierarchicalRetriever<VS, EMB, FS, RR> {
    /// Create a new retriever.
    pub fn new(vs: VS, embedder: EMB, fs: FS, threshold: f64) -> Self {
        Self { vs, embedder, fs, reranker: None, threshold }
    }

    /// Create with optional reranker.
    pub fn with_reranker(vs: VS, embedder: EMB, fs: FS, reranker: RR, threshold: f64) -> Self {
        Self { vs, embedder, fs, reranker: Some(reranker), threshold }
    }

    /// Retrieve matching contexts for a typed query.
    pub async fn retrieve(
        &self,
        query: &TypedQuery,
        limit: usize,
        metadata_filter: Option<&HashMap<String, serde_json::Value>>,
    ) -> Result<QueryResult, BoxError> {
        let collection = Self::type_to_collection(query.context_type);

        // Embed query
        let embed_result = self.embedder.embed(&query.query).await?;
        let query_vector = &embed_result.dense_vector;
        let sparse = embed_result.sparse_vector.as_ref();

        // Global search for starting points
        let global_hits = self
            .global_vector_search(collection, query_vector, sparse, metadata_filter)
            .await?;

        let mut starting_points = Self::merge_starting_points(&global_hits, &query.target_directories);

        if starting_points.is_empty() {
            // FIX-R2: add root URIs for type as fallback starting points
            let root_uris = Self::get_root_uris_for_type(query.context_type);
            if root_uris.is_empty() {
                return Ok(QueryResult {
                    query: query.clone(),
                    matched_contexts: Vec::new(),
                    searched_directories: Vec::new(),
                    thinking_trace: ThinkingTrace::default(),
                });
            }
            // Use root URIs as starting points with neutral score
            for uri in &root_uris {
                if !starting_points.iter().any(|(u, _)| u == uri) {
                    starting_points.push((uri.clone(), 0.5));
                }
            }
        }

        // Recursive search
        let candidates = self
            .recursive_search(
                collection,
                query_vector,
                sparse,
                &starting_points,
                limit,
                metadata_filter,
            )
            .await?;

        let mut matches: Vec<MatchedContext> = candidates
            .into_iter()
            .take(limit)
            .map(|c| MatchedContext {
                uri: c.hit.id.clone(),
                context_type: query.context_type,
                is_leaf: c.hit.fields
                    .get("is_leaf")
                    .and_then(|v| v.as_bool())
                    .unwrap_or(false),
                abstract_text: c.hit.fields
                    .get("abstract")
                    .and_then(|v| v.as_str())
                    .unwrap_or("")
                    .to_owned(),
                overview: None,
                // FIX-R7: extract category from hit fields
                category: c.hit.fields
                    .get("category")
                    .and_then(|v| v.as_str())
                    .unwrap_or("")
                    .to_owned(),
                score: c.final_score,
                match_reason: String::new(),
                relations: Vec::new(),
            })
            .collect();

        // FIX-R1: Apply reranking if available
        if let Some(ref reranker) = self.reranker {
            let docs: Vec<String> = matches.iter()
                .map(|m| format!("{} {}", m.abstract_text, m.uri))
                .collect();
            match reranker.rerank(&query.query, &docs, limit).await {
                Ok(reranked) => {
                    let original = matches.clone();
                    matches.clear();
                    for rr in reranked {
                        if rr.index < original.len() {
                            let mut m = original[rr.index].clone();
                            m.score = rr.score;
                            matches.push(m);
                        }
                    }
                    debug!("Reranked {} results", matches.len());
                }
                Err(e) => {
                    warn!("Rerank failed, using original order: {e}");
                }
            }
        }

        // FIX-R3: Load relations for matched contexts
        for m in &mut matches {
            let relations_uri = format!("{}/.relations", m.uri.trim_end_matches(".md"));
            if let Ok(content) = self.fs.read(&relations_uri).await {
                m.relations = content.lines()
                    .filter(|l| !l.trim().is_empty())
                    .map(|l| RelatedContext {
                        uri: l.trim().to_owned(),
                        abstract_text: String::new(),
                    })
                    .collect();
            }
        }

        // FIX-R6: populate searched_directories from starting points
        let searched_dirs: Vec<String> = starting_points
            .iter()
            .map(|(uri, _)| uri.clone())
            .collect();

        Ok(QueryResult {
            query: query.clone(),
            matched_contexts: matches,
            searched_directories: searched_dirs,
            thinking_trace: ThinkingTrace::default(),
        })
    }

    // -----------------------------------------------------------------------
    // Global search
    // -----------------------------------------------------------------------

    async fn global_vector_search(
        &self,
        collection: &str,
        query_vector: &[f32],
        sparse: Option<&HashMap<String, f64>>,
        metadata_filter: Option<&HashMap<String, serde_json::Value>>,
    ) -> Result<Vec<VectorHit>, BoxError> {
        self.vs
            .search(collection, query_vector, sparse, GLOBAL_SEARCH_TOPK, metadata_filter)
            .await
    }

    // -----------------------------------------------------------------------
    // Starting points
    // -----------------------------------------------------------------------

    fn merge_starting_points(
        global_hits: &[VectorHit],
        target_dirs: &[String],
    ) -> Vec<(String, f64)> {
        let mut map: HashMap<String, f64> = HashMap::new();

        // Add global hits
        for hit in global_hits {
            let parent = hit.fields
                .get("parent_uri")
                .and_then(|v| v.as_str())
                .unwrap_or(&hit.id);
            let entry = map.entry(parent.to_owned()).or_insert(0.0);
            *entry = entry.max(hit.score);
        }

        // Add target directories
        for dir in target_dirs {
            map.entry(dir.clone()).or_insert(1.0);
        }

        let mut points: Vec<(String, f64)> = map.into_iter().collect();
        points.sort_by(|a, b| b.1.partial_cmp(&a.1).unwrap_or(Ordering::Equal));
        points
    }

    // -----------------------------------------------------------------------
    // Recursive search
    // -----------------------------------------------------------------------

    async fn recursive_search(
        &self,
        collection: &str,
        query_vector: &[f32],
        sparse: Option<&HashMap<String, f64>>,
        starting_points: &[(String, f64)],
        limit: usize,
        metadata_filter: Option<&HashMap<String, serde_json::Value>>,
    ) -> Result<Vec<ScoredCandidate>, BoxError> {
        let alpha = SCORE_PROPAGATION_ALPHA;

        let mut dir_queue = BinaryHeap::new();
        for (uri, score) in starting_points {
            dir_queue.push(ScoredUri {
                score: *score,
                uri: uri.clone(),
            });
        }

        let mut visited: HashSet<String> = HashSet::new();
        let mut collected: Vec<ScoredCandidate> = Vec::new();
        let mut prev_topk: HashSet<String> = HashSet::new();
        let mut convergence_rounds = 0usize;

        while let Some(current) = dir_queue.pop() {
            if visited.contains(&current.uri) {
                continue;
            }
            visited.insert(current.uri.clone());
            debug!("[RecursiveSearch] Entering URI: {}", current.uri);

            let mut filter = metadata_filter.cloned().unwrap_or_default();
            filter.insert(
                "parent_uri".to_owned(),
                serde_json::json!(current.uri),
            );

            let pre_filter_limit = (limit * 2).max(20);
            let results = self
                .vs
                .search(
                    collection,
                    query_vector,
                    sparse,
                    pre_filter_limit,
                    Some(&filter),
                )
                .await?;

            if results.is_empty() {
                continue;
            }

            for hit in results {
                let score = hit.score;
                let final_score = alpha * score + (1.0 - alpha) * current.score;

                if final_score < self.threshold {
                    debug!("[RecursiveSearch] {} score {:.4} below threshold {:.4}", hit.id, final_score, self.threshold);
                    continue;
                }

                let uri = hit.id.clone();
                if !collected.iter().any(|c| c.hit.id == uri) {
                    collected.push(ScoredCandidate {
                        hit,
                        final_score,
                    });
                }

                let is_leaf = collected
                    .last()
                    .and_then(|c| c.hit.fields.get("is_leaf"))
                    .and_then(|v| v.as_bool())
                    .unwrap_or(false);

                if !visited.contains(&uri) {
                    if is_leaf {
                        visited.insert(uri);
                    } else {
                        dir_queue.push(ScoredUri {
                            score: final_score,
                            uri,
                        });
                    }
                }
            }

            // Convergence check
            collected.sort_by(|a, b| {
                b.final_score
                    .partial_cmp(&a.final_score)
                    .unwrap_or(Ordering::Equal)
            });
            let current_topk: HashSet<String> = collected
                .iter()
                .take(limit)
                .map(|c| c.hit.id.clone())
                .collect();

            if current_topk == prev_topk && current_topk.len() >= limit {
                convergence_rounds += 1;
                if convergence_rounds >= MAX_CONVERGENCE_ROUNDS {
                    break;
                }
            } else {
                convergence_rounds = 0;
                prev_topk = current_topk;
            }
        }

        collected.sort_by(|a, b| {
            b.final_score
                .partial_cmp(&a.final_score)
                .unwrap_or(Ordering::Equal)
        });
        collected.truncate(limit);
        Ok(collected)
    }

    // -----------------------------------------------------------------------
    // Helpers
    // -----------------------------------------------------------------------

    /// Map context type to vector collection name.
    fn type_to_collection(ct: RetrieveContextType) -> &'static str {
        match ct {
            RetrieveContextType::Memory => "context",
            RetrieveContextType::Skill => "context",
            RetrieveContextType::Resource => "context",
        }
    }

    /// FIX-R2: Return root URIs for a given context type.
    fn get_root_uris_for_type(ct: RetrieveContextType) -> Vec<String> {
        match ct {
            RetrieveContextType::Memory => vec![
                "viking://memories".to_owned(),
                "viking://memories/profile.md".to_owned(),
            ],
            RetrieveContextType::Resource => vec![
                "viking://resources".to_owned(),
            ],
            RetrieveContextType::Skill => vec![
                "viking://skills".to_owned(),
            ],
        }
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn merge_starting_points_dedup() {
        let hits = vec![
            VectorHit {
                id: "child1".into(),
                score: 0.9,
                fields: {
                    let mut m = HashMap::new();
                    m.insert("parent_uri".into(), serde_json::json!("dir_a"));
                    m
                },
            },
            VectorHit {
                id: "child2".into(),
                score: 0.8,
                fields: {
                    let mut m = HashMap::new();
                    m.insert("parent_uri".into(), serde_json::json!("dir_a"));
                    m
                },
            },
        ];
        let dirs = vec!["dir_b".to_owned()];
        let pts = HierarchicalRetriever::<MockVs, MockEmb, MockFs, MockRr>::merge_starting_points(&hits, &dirs);
        assert_eq!(pts.len(), 2); // dir_a (deduped) + dir_b
        assert!(pts[0].1 >= pts[1].1); // sorted desc
    }

    #[test]
    fn scored_uri_ordering() {
        let a = ScoredUri { score: 0.5, uri: "a".into() };
        let b = ScoredUri { score: 0.9, uri: "b".into() };
        assert!(b > a);
    }

    // Mock types
    struct MockVs;
    #[async_trait::async_trait]
    impl VectorStore for MockVs {
        async fn search(&self, _: &str, _: &[f32], _: Option<&HashMap<String, f64>>, _: usize, _: Option<&HashMap<String, serde_json::Value>>) -> Result<Vec<VectorHit>, BoxError> { Ok(Vec::new()) }
        async fn upsert(&self, _: &str, _: &str, _: &[f32], _: HashMap<String, serde_json::Value>) -> Result<(), BoxError> { Ok(()) }
        async fn update(&self, _: &str, _: &str, _: HashMap<String, serde_json::Value>) -> Result<(), BoxError> { Ok(()) }
        async fn delete(&self, _: &str, _: &str) -> Result<(), BoxError> { Ok(()) }
    }
    struct MockEmb;
    #[async_trait::async_trait]
    impl Embedder for MockEmb {
        async fn embed(&self, _: &str) -> Result<crate::traits::EmbedResult, BoxError> {
            Ok(crate::traits::EmbedResult { dense_vector: Vec::new(), sparse_vector: None })
        }
    }

    struct MockFs;
    #[async_trait::async_trait]
    impl FileSystem for MockFs {
        async fn read(&self, _: &str) -> Result<String, BoxError> { Ok(String::new()) }
        async fn read_bytes(&self, _: &str) -> Result<Vec<u8>, BoxError> { Ok(Vec::new()) }
        async fn write(&self, _: &str, _: &str) -> Result<(), BoxError> { Ok(()) }
        async fn write_bytes(&self, _: &str, _: &[u8]) -> Result<(), BoxError> { Ok(()) }
        async fn mkdir(&self, _: &str) -> Result<(), BoxError> { Ok(()) }
        async fn ls(&self, _: &str) -> Result<Vec<crate::traits::FsEntry>, BoxError> { Ok(Vec::new()) }
        async fn rm(&self, _: &str) -> Result<(), BoxError> { Ok(()) }
        async fn mv(&self, _: &str, _: &str) -> Result<(), BoxError> { Ok(()) }
        async fn stat(&self, _: &str) -> Result<crate::traits::FsStat, BoxError> { Err("not implemented".into()) }
        async fn grep(&self, _: &str, _: &str, _: bool, _: bool) -> Result<Vec<crate::traits::GrepMatch>, BoxError> { Ok(Vec::new()) }
        async fn exists(&self, _: &str) -> Result<bool, BoxError> { Ok(false) }
        async fn append(&self, _: &str, _: &str) -> Result<(), BoxError> { Ok(()) }
        async fn link(&self, _: &str, _: &str) -> Result<(), BoxError> { Ok(()) }
    }

    struct MockRr;
    #[async_trait::async_trait]
    impl Reranker for MockRr {
        async fn rerank(&self, _: &str, _: &[String], _: usize) -> Result<Vec<crate::traits::RerankResult>, BoxError> {
            Ok(Vec::new())
        }
    }
}
