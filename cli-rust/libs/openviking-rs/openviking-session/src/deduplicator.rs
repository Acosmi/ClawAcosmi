// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Memory deduplication — LLM-assisted skip/create/none decisions.
//!
//! Ported from `openviking/session/memory_deduplicator.py`.

use std::collections::HashMap;

use log::{debug, warn};
use serde::Deserialize;

use openviking_core::context::Context;
use openviking_core::math_utils::extract_facet_key;
use openviking_core::session_types::{
    CandidateMemory, DedupDecision, DedupResult, ExistingMemoryAction,
    MemoryActionDecision, category_uri_prefix,
};

use crate::json_utils::parse_json_from_response;
use crate::prompts;
use crate::traits::{BoxError, Embedder, LlmProvider, VectorStore};

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

/// Vector similarity threshold for pre-filtering.
pub const SIMILARITY_THRESHOLD: f64 = 0.0;
/// Max similar memories sent to LLM prompt.
pub const MAX_PROMPT_SIMILAR_MEMORIES: usize = 5;

// ---------------------------------------------------------------------------
// MemoryDeduplicator
// ---------------------------------------------------------------------------

/// Handles memory deduplication with LLM decision making.
pub struct MemoryDeduplicator<VS: VectorStore, EMB: Embedder, LLM: LlmProvider> {
    vs: VS,
    embedder: EMB,
    llm: LLM,
}

/// Internal LLM dedup response schema.
#[derive(Debug, Clone, Deserialize)]
struct LlmDedupResponse {
    #[serde(default)]
    decision: String,
    #[serde(default)]
    reason: String,
    #[serde(default)]
    list: Vec<LlmDedupAction>,
}

#[derive(Debug, Clone, Deserialize)]
struct LlmDedupAction {
    #[serde(default)]
    uri: Option<String>,
    #[serde(default)]
    index: Option<i32>,
    #[serde(default)]
    decide: String,
    #[serde(default)]
    reason: String,
}

impl<VS: VectorStore, EMB: Embedder, LLM: LlmProvider> MemoryDeduplicator<VS, EMB, LLM> {
    /// Create a new deduplicator.
    pub fn new(vs: VS, embedder: EMB, llm: LLM) -> Self {
        Self { vs, embedder, llm }
    }

    /// Decide how to handle a candidate memory.
    pub async fn deduplicate(
        &self,
        candidate: &CandidateMemory,
    ) -> Result<DedupResult, BoxError> {
        let similar = self.find_similar_memories(candidate).await?;

        if similar.is_empty() {
            return Ok(DedupResult {
                decision: DedupDecision::Create,
                candidate: candidate.clone(),
                similar_memories: Vec::new(),
                actions: Some(Vec::new()),
                reason: "No similar memories found".to_owned(),
            });
        }

        let similar_uris: Vec<String> = similar.iter().map(|c| c.uri.clone()).collect();
        let (decision, reason, actions) =
            self.llm_decision(candidate, &similar).await?;

        Ok(DedupResult {
            decision,
            candidate: candidate.clone(),
            similar_memories: similar_uris,
            actions: if decision == DedupDecision::Skip {
                None
            } else {
                Some(actions)
            },
            reason,
        })
    }

    // -----------------------------------------------------------------------
    // Internal: find similar memories
    // -----------------------------------------------------------------------

    async fn find_similar_memories(
        &self,
        candidate: &CandidateMemory,
    ) -> Result<Vec<Context>, BoxError> {
        let query_text = format!("{} {}", candidate.abstract_text, candidate.content);
        let embed_result = self.embedder.embed(&query_text).await?;

        let prefix = category_uri_prefix(candidate.category);
        let mut filter = HashMap::new();
        filter.insert("context_type".to_owned(), serde_json::json!("memory"));
        filter.insert("is_leaf".to_owned(), serde_json::json!(true));
        if !prefix.is_empty() {
            filter.insert("uri_prefix".to_owned(), serde_json::json!(prefix));
        }

        let sparse = embed_result.sparse_vector.as_ref();
        let hits = self
            .vs
            .search("context", &embed_result.dense_vector, sparse, 5, Some(&filter))
            .await?;

        debug!("Dedup prefilter hits={}", hits.len());

        let similar: Vec<Context> = hits
            .into_iter()
            .filter(|h| h.score >= SIMILARITY_THRESHOLD)
            .map(|h| {
                let abstract_text = h.fields
                    .get("abstract")
                    .and_then(|v| v.as_str())
                    .unwrap_or("")
                    .to_owned();
                let overview = h.fields
                    .get("overview")
                    .and_then(|v| v.as_str())
                    .unwrap_or("")
                    .to_owned();
                let content = h.fields
                    .get("content")
                    .and_then(|v| v.as_str())
                    .unwrap_or("")
                    .to_owned();
                let mut ctx = Context::new(&h.id, &abstract_text);
                ctx.meta.insert("_dedup_score".to_owned(), serde_json::json!(h.score));
                if !overview.is_empty() {
                    ctx.meta.insert("overview".to_owned(), serde_json::json!(overview));
                }
                if !content.is_empty() {
                    ctx.meta.insert("content".to_owned(), serde_json::json!(content));
                }
                ctx
            })
            .collect();

        debug!("Dedup similar after threshold={}", similar.len());
        Ok(similar)
    }

    // -----------------------------------------------------------------------
    // Internal: LLM decision
    // -----------------------------------------------------------------------

    async fn llm_decision(
        &self,
        candidate: &CandidateMemory,
        similar: &[Context],
    ) -> Result<(DedupDecision, String, Vec<ExistingMemoryAction>), BoxError> {
        let existing_fmt: String = similar
            .iter()
            .take(MAX_PROMPT_SIMILAR_MEMORIES)
            .enumerate()
            .map(|(i, mem)| {
                let facet = extract_facet_key(&mem.abstract_text);
                let score = mem.meta.get("_dedup_score")
                    .and_then(|v| v.as_f64())
                    .map(|s| format!("{s:.4}"))
                    .unwrap_or_else(|| "n/a".to_owned());
                format!(
                    "{}. uri={}\n   score={}\n   facet={}\n   abstract={}",
                    i + 1, mem.uri, score, facet, mem.abstract_text
                )
            })
            .collect::<Vec<_>>()
            .join("\n");

        let prompt = prompts::apply(prompts::DEDUP_DECIDE, &[
            ("candidate_abstract", &candidate.abstract_text),
            ("candidate_overview", &candidate.overview),
            ("candidate_content", &candidate.content),
            ("existing_memories", &existing_fmt),
        ]);

        let response = self.llm.completion(&prompt).await?;
        debug!("Dedup LLM response len={}", response.len());

        let data: LlmDedupResponse = parse_json_from_response(&response)
            .unwrap_or(LlmDedupResponse {
                decision: "create".to_owned(),
                reason: String::new(),
                list: Vec::new(),
            });

        Ok(Self::parse_decision_payload(&data, similar))
    }

    // -----------------------------------------------------------------------
    // Parse decision payload (pure logic, testable)
    // -----------------------------------------------------------------------

    fn parse_decision_payload(
        data: &LlmDedupResponse,
        similar: &[Context],
    ) -> (DedupDecision, String, Vec<ExistingMemoryAction>) {
        let decision_str = data.decision.to_lowercase();
        let mut reason = data.reason.clone();

        let mut decision = match decision_str.as_str() {
            "skip" => DedupDecision::Skip,
            "create" => DedupDecision::Create,
            "none" => DedupDecision::None,
            "merge" => DedupDecision::None,
            _ => DedupDecision::Create,
        };

        let mut raw_actions = data.list.clone();

        // Legacy: {decision: "merge"} with no list
        if decision_str == "merge" && raw_actions.is_empty() && !similar.is_empty() {
            raw_actions.push(LlmDedupAction {
                uri: Some(similar[0].uri.clone()),
                index: None,
                decide: "merge".to_owned(),
                reason: "Legacy candidate merge mapped to none".to_owned(),
            });
            if reason.is_empty() {
                reason = "Legacy candidate merge mapped to none".to_owned();
            }
        }

        let uri_map: HashMap<&str, usize> = similar
            .iter()
            .enumerate()
            .map(|(i, c)| (c.uri.as_str(), i))
            .collect();

        let mut actions: Vec<ExistingMemoryAction> = Vec::new();
        let mut seen: HashMap<String, MemoryActionDecision> = HashMap::new();

        for item in &raw_actions {
            let action = match item.decide.to_lowercase().as_str() {
                "merge" => MemoryActionDecision::Merge,
                "delete" => MemoryActionDecision::Delete,
                _ => continue,
            };

            // Resolve URI
            let resolved_uri = item.uri.as_deref()
                .filter(|u| uri_map.contains_key(u))
                .map(|u| u.to_owned())
                .or_else(|| {
                    item.index.and_then(|idx| {
                        let idx = idx as usize;
                        if idx >= 1 && idx <= similar.len() {
                            Some(similar[idx - 1].uri.clone())
                        } else if idx < similar.len() {
                            Some(similar[idx].uri.clone())
                        } else {
                            None
                        }
                    })
                });

            let memory_uri = match resolved_uri {
                Some(u) => u,
                None => continue,
            };

            // Handle conflicts
            if let Some(prev) = seen.get(&memory_uri) {
                if *prev != action {
                    actions.retain(|a| a.memory_uri != memory_uri);
                    seen.remove(&memory_uri);
                    warn!("Conflicting actions for {memory_uri}, dropping both");
                }
                continue;
            }

            seen.insert(memory_uri.clone(), action);
            actions.push(ExistingMemoryAction {
                memory_uri,
                decision: action,
                reason: item.reason.clone(),
            });
        }

        // Rule: skip never carries actions
        if decision == DedupDecision::Skip {
            return (decision, reason, Vec::new());
        }

        // Rule: create + merge → none
        let has_merge = actions.iter().any(|a| a.decision == MemoryActionDecision::Merge);
        if decision == DedupDecision::Create && has_merge {
            decision = DedupDecision::None;
            reason = format!(
                "{} | normalized:create+merge->none",
                reason.trim_matches(|c: char| c == ' ' || c == '|')
            );
            return (decision, reason, actions);
        }

        // Rule: create only allows delete actions
        if decision == DedupDecision::Create {
            actions.retain(|a| a.decision == MemoryActionDecision::Delete);
        }

        (decision, reason, actions)
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    fn ctx(uri: &str, abs: &str) -> Context {
        Context::new(uri, abs)
    }

    // Mock types for generic bounds
    struct MockVs;
    #[async_trait::async_trait]
    impl VectorStore for MockVs {
        async fn search(&self, _: &str, _: &[f32], _: Option<&HashMap<String, f64>>, _: usize, _: Option<&HashMap<String, serde_json::Value>>) -> Result<Vec<crate::traits::VectorHit>, BoxError> { Ok(Vec::new()) }
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
    struct MockLlm;
    #[async_trait::async_trait]
    impl LlmProvider for MockLlm {
        async fn completion(&self, _: &str) -> Result<String, BoxError> { Ok(String::new()) }
    }

    type Dedup = MemoryDeduplicator<MockVs, MockEmb, MockLlm>;

    #[test]
    fn skip_clears_actions() {
        let data = LlmDedupResponse {
            decision: "skip".into(), reason: "dup".into(),
            list: vec![LlmDedupAction { uri: Some("viking://x".into()), index: None, decide: "delete".into(), reason: String::new() }],
        };
        let (dec, _, acts) = Dedup::parse_decision_payload(&data, &[ctx("viking://x", "t")]);
        assert_eq!(dec, DedupDecision::Skip);
        assert!(acts.is_empty());
    }

    #[test]
    fn create_with_merge_becomes_none() {
        let data = LlmDedupResponse {
            decision: "create".into(), reason: "new".into(),
            list: vec![LlmDedupAction { uri: Some("viking://x".into()), index: None, decide: "merge".into(), reason: String::new() }],
        };
        let (dec, _, _) = Dedup::parse_decision_payload(&data, &[ctx("viking://x", "t")]);
        assert_eq!(dec, DedupDecision::None);
    }

    #[test]
    fn legacy_merge_compat() {
        let data = LlmDedupResponse {
            decision: "merge".into(), reason: String::new(), list: Vec::new(),
        };
        let (dec, _, acts) = Dedup::parse_decision_payload(&data, &[ctx("viking://x", "t")]);
        assert_eq!(dec, DedupDecision::None);
        assert_eq!(acts.len(), 1);
        assert_eq!(acts[0].decision, MemoryActionDecision::Merge);
    }

    #[test]
    fn create_filters_to_delete_only() {
        let data = LlmDedupResponse {
            decision: "create".into(), reason: "new".into(),
            list: vec![
                LlmDedupAction { uri: Some("viking://a".into()), index: None, decide: "delete".into(), reason: String::new() },
            ],
        };
        let similar = vec![ctx("viking://a", "a")];
        let (dec, _, acts) = Dedup::parse_decision_payload(&data, &similar);
        assert_eq!(dec, DedupDecision::Create);
        assert_eq!(acts.len(), 1);
        assert_eq!(acts[0].decision, MemoryActionDecision::Delete);
    }
}
