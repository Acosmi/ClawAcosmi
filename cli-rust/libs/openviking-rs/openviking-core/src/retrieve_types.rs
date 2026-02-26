// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Retrieval data types — pure data, zero IO.
//!
//! Ported from `openviking_cli/retrieve/types.py` (413 lines).

use std::collections::HashMap;

use serde::{Deserialize, Serialize};

// ---------------------------------------------------------------------------
// Enums
// ---------------------------------------------------------------------------

/// Context type for retrieval queries.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum RetrieveContextType {
    /// Memory context.
    Memory,
    /// Resource context.
    Resource,
    /// Skill context.
    Skill,
}

/// Retriever execution mode.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum RetrieverMode {
    /// Full hierarchical search with convergence.
    Thinking,
    /// Fast single-pass search.
    Quick,
}

/// Types of trace events for retrieval visualization.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum TraceEventType {
    /// Recursive search enters a directory.
    SearchDirectoryStart,
    /// Results from searching a directory.
    SearchDirectoryResult,
    /// Embedding similarity scores.
    EmbeddingScores,
    /// Rerank scores.
    RerankScores,
    /// A candidate was selected.
    CandidateSelected,
    /// A candidate was excluded.
    CandidateExcluded,
    /// A sub-directory was queued for search.
    DirectoryQueued,
    /// Convergence check result.
    ConvergenceCheck,
    /// Search has converged.
    SearchConverged,
    /// Final summary.
    SearchSummary,
}

// ---------------------------------------------------------------------------
// Trace types
// ---------------------------------------------------------------------------

/// Single trace event for retrieval process.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TraceEvent {
    /// Type of event.
    pub event_type: TraceEventType,
    /// Relative timestamp in seconds from trace start.
    pub timestamp: f64,
    /// Human-readable description.
    pub message: String,
    /// Structured event data for visualization.
    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    pub data: HashMap<String, serde_json::Value>,
    /// Optional query identifier.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub query_id: Option<String>,
}

/// Score distribution statistics.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ScoreDistribution {
    /// (uri, score) pairs sorted by score descending.
    pub scores: Vec<(String, f64)>,
    /// Minimum score.
    pub min_score: f64,
    /// Maximum score.
    pub max_score: f64,
    /// Mean score.
    pub mean_score: f64,
    /// Threshold used for filtering.
    pub threshold: f64,
}

impl ScoreDistribution {
    /// Create from a list of (uri, score) tuples.
    #[must_use]
    pub fn from_scores(mut uri_scores: Vec<(String, f64)>, threshold: f64) -> Self {
        if uri_scores.is_empty() {
            return Self {
                threshold,
                ..Default::default()
            };
        }
        uri_scores.sort_by(|a, b| b.1.partial_cmp(&a.1).unwrap_or(std::cmp::Ordering::Equal));
        let scores_only: Vec<f64> = uri_scores.iter().map(|(_, s)| *s).collect();
        let min = scores_only.iter().cloned().fold(f64::INFINITY, f64::min);
        let max = scores_only
            .iter()
            .cloned()
            .fold(f64::NEG_INFINITY, f64::max);
        let mean = scores_only.iter().sum::<f64>() / scores_only.len() as f64;
        Self {
            scores: uri_scores,
            min_score: min,
            max_score: max,
            mean_score: mean,
            threshold,
        }
    }

    /// Count of scores at or above threshold.
    #[must_use]
    pub fn above_threshold(&self) -> usize {
        self.scores
            .iter()
            .filter(|(_, s)| *s >= self.threshold)
            .count()
    }
}

/// Structured thinking trace container.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ThinkingTrace {
    /// Ordered list of events.
    pub events: Vec<TraceEvent>,
}

impl ThinkingTrace {
    /// Add a trace event.
    pub fn add_event(
        &mut self,
        event_type: TraceEventType,
        timestamp: f64,
        message: impl Into<String>,
        data: HashMap<String, serde_json::Value>,
        query_id: Option<String>,
    ) {
        self.events.push(TraceEvent {
            event_type,
            timestamp,
            message: message.into(),
            data,
            query_id,
        });
    }

    /// Extract simple message list.
    #[must_use]
    pub fn to_messages(&self) -> Vec<&str> {
        self.events.iter().map(|e| e.message.as_str()).collect()
    }

    /// Calculate summary statistics.
    #[must_use]
    pub fn statistics(&self) -> TraceStatistics {
        let duration_seconds = self.events.last().map_or(0.0, |e| e.timestamp);
        let mut stats = TraceStatistics {
            total_events: self.events.len() as u32,
            duration_seconds,
            ..Default::default()
        };
        for e in &self.events {
            match e.event_type {
                TraceEventType::SearchDirectoryResult => stats.directories_searched += 1,
                TraceEventType::CandidateSelected => {
                    stats.candidates_collected +=
                        e.data.get("count").and_then(|v| v.as_u64()).unwrap_or(1) as u32;
                }
                TraceEventType::CandidateExcluded => {
                    stats.candidates_excluded +=
                        e.data.get("count").and_then(|v| v.as_u64()).unwrap_or(1) as u32;
                }
                TraceEventType::ConvergenceCheck => {
                    stats.convergence_rounds =
                        e.data.get("round").and_then(|v| v.as_u64()).unwrap_or(0) as u32;
                }
                _ => {}
            }
        }
        stats
    }
}

/// Summary statistics for a thinking trace.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct TraceStatistics {
    /// Total event count.
    pub total_events: u32,
    /// Total duration in seconds.
    pub duration_seconds: f64,
    /// Number of directories searched.
    pub directories_searched: u32,
    /// Number of candidates collected.
    pub candidates_collected: u32,
    /// Number of candidates excluded.
    pub candidates_excluded: u32,
    /// Number of convergence rounds.
    pub convergence_rounds: u32,
}

// ---------------------------------------------------------------------------
// Query types
// ---------------------------------------------------------------------------

/// Query targeting a specific context type.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TypedQuery {
    /// Query text.
    pub query: String,
    /// Target context type.
    pub context_type: RetrieveContextType,
    /// Query intent description.
    pub intent: String,
    /// Priority (1=highest, 5=lowest).
    #[serde(default = "default_priority")]
    pub priority: u8,
    /// Target directory URIs (located by LLM).
    #[serde(default)]
    pub target_directories: Vec<String>,
}

fn default_priority() -> u8 {
    3
}

/// Query plan containing multiple typed queries.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct QueryPlan {
    /// List of typed queries.
    pub queries: Vec<TypedQuery>,
    /// Session context summary.
    pub session_context: String,
    /// LLM reasoning process.
    pub reasoning: String,
}

// ---------------------------------------------------------------------------
// Result types
// ---------------------------------------------------------------------------

/// Related context with summary.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RelatedContext {
    /// URI of related context.
    pub uri: String,
    /// Abstract summary.
    #[serde(rename = "abstract")]
    pub abstract_text: String,
}

/// Matched context from retrieval.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MatchedContext {
    /// URI of matched context.
    pub uri: String,
    /// Context type.
    pub context_type: RetrieveContextType,
    /// Whether this is a leaf node.
    #[serde(default)]
    pub is_leaf: bool,
    /// L0 abstract.
    #[serde(default, rename = "abstract")]
    pub abstract_text: String,
    /// L1 overview.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub overview: Option<String>,
    /// Category label.
    #[serde(default)]
    pub category: String,
    /// Relevance score.
    #[serde(default)]
    pub score: f64,
    /// Match reason description.
    #[serde(default)]
    pub match_reason: String,
    /// Related contexts.
    #[serde(default)]
    pub relations: Vec<RelatedContext>,
}

/// Result for a single typed query.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct QueryResult {
    /// Original query.
    pub query: TypedQuery,
    /// Matched contexts.
    pub matched_contexts: Vec<MatchedContext>,
    /// Directories that were searched.
    pub searched_directories: Vec<String>,
    /// Structured thinking trace.
    #[serde(default)]
    pub thinking_trace: ThinkingTrace,
}

/// Final aggregated retrieval result.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct FindResult {
    /// Matched memories.
    pub memories: Vec<MatchedContext>,
    /// Matched resources.
    pub resources: Vec<MatchedContext>,
    /// Matched skills.
    pub skills: Vec<MatchedContext>,
    /// Query plan used.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub query_plan: Option<QueryPlan>,
    /// Detailed per-query results.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub query_results: Option<Vec<QueryResult>>,
    /// Total match count.
    #[serde(default)]
    pub total: usize,
}

impl FindResult {
    /// Recalculate total count.
    pub fn update_total(&mut self) {
        self.total = self.memories.len() + self.resources.len() + self.skills.len();
    }

    /// Iterate over all matched contexts.
    pub fn iter(&self) -> impl Iterator<Item = &MatchedContext> {
        self.memories
            .iter()
            .chain(self.resources.iter())
            .chain(self.skills.iter())
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn score_distribution_from_empty() {
        let sd = ScoreDistribution::from_scores(vec![], 0.5);
        assert_eq!(sd.scores.len(), 0);
        assert_eq!(sd.threshold, 0.5);
    }

    #[test]
    fn score_distribution_sorted() {
        let sd = ScoreDistribution::from_scores(
            vec![("a".into(), 0.3), ("b".into(), 0.9), ("c".into(), 0.6)],
            0.5,
        );
        assert_eq!(sd.scores[0].0, "b");
        assert!((sd.max_score - 0.9).abs() < f64::EPSILON);
        assert_eq!(sd.above_threshold(), 2);
    }

    #[test]
    fn find_result_iter() {
        let mut fr = FindResult::default();
        fr.memories.push(MatchedContext {
            uri: "viking://user/memories/a".into(),
            context_type: RetrieveContextType::Memory,
            is_leaf: true,
            abstract_text: "test".into(),
            overview: None,
            category: String::new(),
            score: 0.8,
            match_reason: String::new(),
            relations: vec![],
        });
        fr.update_total();
        assert_eq!(fr.total, 1);
        assert_eq!(fr.iter().count(), 1);
    }

    #[test]
    fn trace_event_type_serde() {
        let t = TraceEventType::SearchDirectoryStart;
        let json = serde_json::to_string(&t).unwrap();
        assert_eq!(json, "\"search_directory_start\"");
    }
}
