// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Intent analysis — generates query plans from session context.
//!
//! Ported from `openviking/retrieve/intent_analyzer.py`.

use log::debug;
use serde::Deserialize;

use openviking_core::message::Message;
use openviking_core::retrieve_types::{QueryPlan, RetrieveContextType, TypedQuery};

use crate::json_utils::parse_json_from_response;
use crate::prompts;
use crate::traits::{BoxError, LlmProvider};

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

/// Default number of recent messages included in the intent analysis prompt.
const DEFAULT_MAX_RECENT_MESSAGES: usize = 5;

// ---------------------------------------------------------------------------
// IntentAnalyzer
// ---------------------------------------------------------------------------

/// Analyzes session context and generates a [`QueryPlan`] via an LLM.
///
/// # Type Parameters
/// - `LLM`: Any type implementing [`LlmProvider`].
pub struct IntentAnalyzer<LLM: LlmProvider> {
    llm: LLM,
    max_recent_messages: usize,
}

/// Internal JSON schema returned by the LLM for intent analysis.
#[derive(Debug, Deserialize)]
struct LlmIntentResponse {
    #[serde(default)]
    queries: Vec<LlmQueryItem>,
    #[serde(default)]
    reasoning: String,
}

#[derive(Debug, Deserialize)]
struct LlmQueryItem {
    #[serde(default)]
    query: String,
    #[serde(default)]
    context_type: String,
    #[serde(default)]
    intent: String,
    #[serde(default = "default_priority")]
    priority: u8,
}

fn default_priority() -> u8 {
    3
}

impl<LLM: LlmProvider> IntentAnalyzer<LLM> {
    /// Create a new `IntentAnalyzer`.
    pub fn new(llm: LLM) -> Self {
        Self {
            llm,
            max_recent_messages: DEFAULT_MAX_RECENT_MESSAGES,
        }
    }

    /// Create with a custom `max_recent_messages` value.
    pub fn with_max_recent_messages(llm: LLM, max_recent_messages: usize) -> Self {
        Self {
            llm,
            max_recent_messages,
        }
    }

    /// Analyze session context and generate a query plan.
    ///
    /// # Arguments
    /// - `compression_summary`: Prior session compression summary.
    /// - `messages`: Full message history.
    /// - `current_message`: Current user message (if any).
    /// - `context_type`: Constrained context type filter (optional).
    /// - `target_abstract`: Target directory abstract for more precise queries.
    pub async fn analyze(
        &self,
        compression_summary: &str,
        messages: &[Message],
        current_message: Option<&str>,
        context_type: Option<RetrieveContextType>,
        target_abstract: &str,
    ) -> Result<QueryPlan, BoxError> {
        let prompt = self.build_context_prompt(
            compression_summary,
            messages,
            current_message,
            context_type,
            target_abstract,
        );

        let response = self.llm.completion(&prompt).await?;

        let parsed: LlmIntentResponse = parse_json_from_response(&response)
            .ok_or_else(|| "Failed to parse intent analysis response".to_string())?;

        let queries: Vec<TypedQuery> = parsed
            .queries
            .into_iter()
            .map(|q| {
                let ct = match q.context_type.as_str() {
                    "memory" => RetrieveContextType::Memory,
                    "skill" => RetrieveContextType::Skill,
                    _ => RetrieveContextType::Resource,
                };
                TypedQuery {
                    query: q.query,
                    context_type: ct,
                    intent: q.intent,
                    priority: q.priority,
                    target_directories: Vec::new(),
                }
            })
            .collect();

        for (i, q) in queries.iter().enumerate() {
            debug!(
                "  [{}] type={:?}, priority={}, query=\"{}\"",
                i + 1,
                q.context_type,
                q.priority,
                q.query
            );
        }

        Ok(QueryPlan {
            queries,
            session_context: Self::summarize_context(compression_summary, current_message),
            reasoning: parsed.reasoning,
        })
    }

    // -----------------------------------------------------------------------
    // Internal helpers
    // -----------------------------------------------------------------------

    fn build_context_prompt(
        &self,
        compression_summary: &str,
        messages: &[Message],
        current_message: Option<&str>,
        context_type: Option<RetrieveContextType>,
        target_abstract: &str,
    ) -> String {
        let summary = if compression_summary.is_empty() {
            "None"
        } else {
            compression_summary
        };

        let recent = if messages.is_empty() {
            "None".to_owned()
        } else {
            let start = messages.len().saturating_sub(self.max_recent_messages);
            messages[start..]
                .iter()
                .filter_map(|m| {
                    let content = m.content();
                    if content.is_empty() {
                        None
                    } else {
                        Some(format!("[{:?}]: {}", m.role, content))
                    }
                })
                .collect::<Vec<_>>()
                .join("\n")
        };

        let current = current_message.unwrap_or("None");
        let ctx_type = context_type
            .map(|ct| format!("{ct:?}").to_lowercase())
            .unwrap_or_default();

        prompts::apply(prompts::INTENT_ANALYZE, &[
            ("summary", summary),
            ("recent", &recent),
            ("current", current),
            ("ctx_type", &ctx_type),
            ("target_abstract", target_abstract),
        ])
    }

    fn summarize_context(compression_summary: &str, current_message: Option<&str>) -> String {
        let mut parts = Vec::new();
        if !compression_summary.is_empty() {
            parts.push(format!("Session summary: {compression_summary}"));
        }
        if let Some(msg) = current_message {
            let truncated = if msg.len() > 100 { &msg[..100] } else { msg };
            parts.push(format!("Current message: {truncated}"));
        }
        if parts.is_empty() {
            "No context".to_owned()
        } else {
            parts.join(" | ")
        }
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use crate::traits::BoxError;
    use async_trait::async_trait;

    struct MockLlm {
        response: String,
    }

    #[async_trait]
    impl LlmProvider for MockLlm {
        async fn completion(&self, _prompt: &str) -> Result<String, BoxError> {
            Ok(self.response.clone())
        }
    }

    #[tokio::test]
    async fn analyze_returns_query_plan() {
        let llm = MockLlm {
            response: r#"{"queries":[{"query":"user preferences","context_type":"memory","intent":"find prefs","priority":1}],"reasoning":"test"}"#.to_owned(),
        };
        let analyzer = IntentAnalyzer::new(llm);
        let plan = analyzer
            .analyze("summary", &[], None, None, "")
            .await
            .unwrap();
        assert_eq!(plan.queries.len(), 1);
        assert_eq!(plan.queries[0].query, "user preferences");
        assert_eq!(plan.queries[0].context_type, RetrieveContextType::Memory);
        assert_eq!(plan.queries[0].priority, 1);
        assert_eq!(plan.reasoning, "test");
    }

    #[tokio::test]
    async fn analyze_defaults_to_resource() {
        let llm = MockLlm {
            response: r#"{"queries":[{"query":"q","context_type":"unknown"}]}"#.to_owned(),
        };
        let analyzer = IntentAnalyzer::new(llm);
        let plan = analyzer.analyze("", &[], None, None, "").await.unwrap();
        assert_eq!(plan.queries[0].context_type, RetrieveContextType::Resource);
    }

    #[test]
    fn summarize_context_both() {
        let s = IntentAnalyzer::<MockLlm>::summarize_context("sum", Some("msg"));
        assert!(s.contains("Session summary: sum"));
        assert!(s.contains("Current message: msg"));
    }

    #[test]
    fn summarize_context_empty() {
        let s = IntentAnalyzer::<MockLlm>::summarize_context("", None);
        assert_eq!(s, "No context");
    }
}
