// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Memory extraction — 6-category memory classification from sessions.
//!
//! Ported from `openviking/session/memory_extractor.py`.

use log::{debug, error, info, warn};
use regex::Regex;

use openviking_core::message::{Message, Role};
use serde::Deserialize;
use uuid::Uuid;

use openviking_core::context::{Context, Vectorize};
use openviking_core::session_types::{
    CandidateMemory, MemoryCategory, MergedMemoryPayload, category_dir,
};
use openviking_core::user::UserIdentifier;

use crate::json_utils::parse_json_from_response;
use crate::prompts;
use crate::traits::{BoxError, FileSystem, LlmProvider};

// ---------------------------------------------------------------------------
// MemoryExtractor
// ---------------------------------------------------------------------------

/// Extracts 6-category candidate memories from session messages via LLM.
pub struct MemoryExtractor<LLM: LlmProvider, FS: FileSystem> {
    llm: LLM,
    fs: FS,
}

/// Internal LLM response schema for extraction.
#[derive(Debug, Deserialize)]
struct LlmExtractResponse {
    #[serde(default)]
    memories: Vec<LlmMemoryItem>,
}

#[derive(Debug, Deserialize)]
struct LlmMemoryItem {
    #[serde(default)]
    category: String,
    #[serde(default, rename = "abstract")]
    abstract_text: String,
    #[serde(default)]
    overview: String,
    #[serde(default)]
    content: String,
}

/// Internal LLM response for memory merge.
#[derive(Debug, Deserialize)]
struct LlmMergeResponse {
    #[serde(default, rename = "abstract")]
    abstract_text: String,
    #[serde(default)]
    overview: String,
    #[serde(default)]
    content: String,
    #[serde(default)]
    reason: String,
    #[serde(default)]
    decision: String,
}

impl<LLM: LlmProvider, FS: FileSystem> MemoryExtractor<LLM, FS> {
    /// Create a new `MemoryExtractor`.
    pub fn new(llm: LLM, fs: FS) -> Self {
        Self { llm, fs }
    }

    /// Extract candidate memories from messages via LLM.
    pub async fn extract(
        &self,
        messages: &[Message],
        user: &UserIdentifier,
        session_id: &str,
    ) -> Result<Vec<CandidateMemory>, BoxError> {
        if messages.is_empty() {
            return Ok(Vec::new());
        }

        let formatted: String = messages
            .iter()
            .filter(|m| !m.content().is_empty())
            .map(|m| format!("[{:?}]: {}", m.role, m.content()))
            .collect::<Vec<_>>()
            .join("\n");

        if formatted.is_empty() {
            return Ok(Vec::new());
        }

        let output_language = Self::detect_output_language(
            messages,
            user.language.as_deref().unwrap_or("en"),
        );

        let prompt = prompts::apply(prompts::MEMORY_EXTRACT, &[
            ("user_id", &user.user_id),
            ("output_language", &output_language),
            ("messages", &formatted),
        ]);

        debug!("Memory extraction LLM request len={}", formatted.len());
        let response = self.llm.completion(&prompt).await?;
        debug!("Memory extraction LLM response len={}", response.len());

        let data: LlmExtractResponse =
            parse_json_from_response(&response).unwrap_or(LlmExtractResponse {
                memories: Vec::new(),
            });

        let candidates = data
            .memories
            .into_iter()
            .map(|mem| {
                let category = match mem.category.as_str() {
                    "profile" => MemoryCategory::Profile,
                    "preferences" => MemoryCategory::Preferences,
                    "entities" => MemoryCategory::Entities,
                    "events" => MemoryCategory::Events,
                    "cases" => MemoryCategory::Cases,
                    _ => MemoryCategory::Patterns,
                };
                CandidateMemory {
                    category,
                    abstract_text: mem.abstract_text,
                    overview: mem.overview,
                    content: mem.content,
                    source_session: session_id.to_owned(),
                    user: user.user_id.clone(),
                    language: output_language.clone(),
                }
            })
            .collect::<Vec<_>>();

        info!(
            "Extracted {} candidate memories (lang={})",
            candidates.len(),
            output_language
        );
        Ok(candidates)
    }

    /// Create a `Context` from a candidate and persist to FS.
    pub async fn create_memory(
        &self,
        candidate: &CandidateMemory,
        user: &UserIdentifier,
        session_id: &str,
    ) -> Result<Option<Context>, BoxError> {
        // Profile: special merge handling
        if candidate.category == MemoryCategory::Profile {
            let payload = self.append_to_profile(candidate).await?;
            if let Some(p) = payload {
                let mut ctx = Context::new(
                    "viking://user/memories/profile.md",
                    p.abstract_text.clone(),
                )
                .with_parent("viking://user/memories")
                .as_leaf();
                ctx.session_id = Some(session_id.to_owned());
                ctx.user = Some(user.clone());
                ctx.set_vectorize(Vectorize::new(p.content));
                return Ok(Some(ctx));
            }
            return Ok(None);
        }

        // Determine parent URI
        let dir = category_dir(candidate.category);
        let parent_uri = match candidate.category {
            MemoryCategory::Cases | MemoryCategory::Patterns => {
                format!("viking://agent/{dir}")
            }
            _ => format!("viking://user/{dir}"),
        };

        let memory_id = format!("mem_{}", Uuid::new_v4());
        let memory_uri = format!("{parent_uri}/{memory_id}.md");

        if let Err(e) = self.fs.write(&memory_uri, &candidate.content).await {
            error!("Failed to write memory to FS: {e}");
            return Ok(None);
        }
        info!("Created memory file: {memory_uri}");

        let mut ctx = Context::new(&memory_uri, &candidate.abstract_text)
            .with_parent(&parent_uri)
            .as_leaf();
        ctx.session_id = Some(session_id.to_owned());
        ctx.user = Some(user.clone());
        ctx.set_vectorize(Vectorize::new(&candidate.content));

        Ok(Some(ctx))
    }

    /// Merge memory bundle via LLM (used by compressor for merge operations).
    #[allow(clippy::too_many_arguments)]
    pub async fn merge_memory_bundle(
        &self,
        existing_abstract: &str,
        existing_overview: &str,
        existing_content: &str,
        new_abstract: &str,
        new_overview: &str,
        new_content: &str,
        category: &str,
        output_language: &str,
    ) -> Result<Option<MergedMemoryPayload>, BoxError> {
        let prompt = prompts::apply(prompts::MEMORY_MERGE, &[
            ("category", category),
            ("output_language", output_language),
            ("existing_abstract", existing_abstract),
            ("existing_overview", existing_overview),
            ("existing_content", existing_content),
            ("new_abstract", new_abstract),
            ("new_overview", new_overview),
            ("new_content", new_content),
        ]);

        let response = self.llm.completion(&prompt).await?;
        let data: LlmMergeResponse = match parse_json_from_response(&response) {
            Some(d) => d,
            None => {
                error!("Memory merge bundle parse failed");
                return Ok(None);
            }
        };

        if !data.decision.is_empty() && data.decision.to_lowercase() != "merge" {
            error!("Memory merge bundle invalid decision={}", data.decision);
            return Ok(None);
        }
        if data.abstract_text.trim().is_empty() || data.content.trim().is_empty() {
            error!("Memory merge bundle missing required fields");
            return Ok(None);
        }

        Ok(Some(MergedMemoryPayload {
            abstract_text: data.abstract_text,
            overview: data.overview,
            content: data.content,
            reason: data.reason,
        }))
    }

    // -----------------------------------------------------------------------
    // Internal
    // -----------------------------------------------------------------------

    async fn append_to_profile(
        &self,
        candidate: &CandidateMemory,
    ) -> Result<Option<MergedMemoryPayload>, BoxError> {
        let uri = "viking://user/memories/profile.md";
        let existing = self.fs.read(uri).await.unwrap_or_default();

        if existing.trim().is_empty() {
            self.fs.write(uri, &candidate.content).await?;
            info!("Created profile at {uri}");
            return Ok(Some(MergedMemoryPayload {
                abstract_text: candidate.abstract_text.clone(),
                overview: candidate.overview.clone(),
                content: candidate.content.clone(),
                reason: "created".to_owned(),
            }));
        }

        let payload = self
            .merge_memory_bundle(
                "", "", &existing,
                &candidate.abstract_text, &candidate.overview,
                &candidate.content, "profile", &candidate.language,
            )
            .await?;

        if let Some(ref p) = payload {
            self.fs.write(uri, &p.content).await?;
            info!("Merged profile info to {uri}");
        } else {
            warn!("Profile merge failed; keeping existing profile unchanged");
        }
        Ok(payload)
    }

    /// Detect dominant language from user messages (pure function).
    pub fn detect_output_language(messages: &[Message], fallback: &str) -> String {
        let user_text: String = messages
            .iter()
            .filter(|m| m.role == Role::User)
            .map(|m| m.content().to_owned())
            .collect::<Vec<_>>()
            .join("\n");

        if user_text.is_empty() {
            return fallback.to_owned();
        }

        let ko = Regex::new(r"[\uac00-\ud7af]").unwrap();
        let ru = Regex::new(r"[\u0400-\u04ff]").unwrap();
        let ar = Regex::new(r"[\u0600-\u06ff]").unwrap();

        let counts = [
            ("ko", ko.find_iter(&user_text).count()),
            ("ru", ru.find_iter(&user_text).count()),
            ("ar", ar.find_iter(&user_text).count()),
        ];
        if let Some((lang, score)) = counts.iter().max_by_key(|c| c.1) {
            if *score > 0 {
                return lang.to_string();
            }
        }

        let kana = Regex::new(r"[\u3040-\u30ff\u31f0-\u31ff\uff66-\uff9f]").unwrap();
        let han = Regex::new(r"[\u4e00-\u9fff]").unwrap();

        if kana.find(&user_text).is_some() {
            return "ja".to_owned();
        }
        if han.find(&user_text).is_some() {
            return "zh-CN".to_owned();
        }

        fallback.to_owned()
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

    struct MockFs;
    #[async_trait]
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

    #[test]
    fn detect_lang_fallback() {
        let msgs: Vec<Message> = vec![];
        assert_eq!(
            MemoryExtractor::<MockLlm, MockFs>::detect_output_language(&msgs, "en"),
            "en"
        );
    }

    #[tokio::test]
    async fn extract_empty_messages() {
        let ext = MemoryExtractor::new(
            MockLlm { response: String::new() },
            MockFs,
        );
        let user = UserIdentifier::new("acme", "test", "agent").unwrap();
        let result = ext.extract(&[], &user, "s1").await.unwrap();
        assert!(result.is_empty());
    }
}
