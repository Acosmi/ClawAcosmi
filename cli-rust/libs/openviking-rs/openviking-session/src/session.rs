// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Session — top-level orchestrator for conversation lifecycle.
//!
//! Ported from `openviking/session/session.py`.

use log::{debug, info, warn};
use regex::Regex;

use openviking_core::message::{Message, Part, Role};
use openviking_core::session_types::{SessionCompression, SessionStats, Usage};
use openviking_core::user::UserIdentifier;

use crate::traits::{BoxError, FileSystem, LlmProvider};

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

/// Default threshold for auto-commit (number of messages).
const DEFAULT_AUTO_COMMIT_THRESHOLD: usize = 20;

// ---------------------------------------------------------------------------
// Result types
// ---------------------------------------------------------------------------

/// Result of a commit operation.
#[derive(Debug, Clone, Default)]
pub struct CommitResult {
    /// Session ID.
    pub session_id: String,
    /// Commit status.
    pub status: String,
    /// Number of memories extracted.
    pub memories_extracted: usize,
    /// Whether messages were archived.
    pub archived: bool,
    /// Session stats snapshot.
    pub stats: Option<SessionStats>,
}

/// Context for session-aware search.
#[derive(Debug, Clone, Default)]
pub struct SearchContext {
    /// Relevant archive overviews.
    pub summaries: Vec<String>,
    /// Recent messages.
    pub recent_messages: Vec<Message>,
}

// ---------------------------------------------------------------------------
// Session
// ---------------------------------------------------------------------------

/// Top-level session orchestrator.
///
/// Manages the full lifecycle of a conversation session including
/// message persistence, auto-commit, compression, and memory extraction.
pub struct Session<FS: FileSystem, LLM: LlmProvider> {
    fs: FS,
    llm: LLM,
    user: UserIdentifier,
    session_id: String,
    session_uri: String,
    auto_commit_threshold: usize,
    messages: Vec<Message>,
    usage_records: Vec<Usage>,
    compression: SessionCompression,
    stats: SessionStats,
    loaded: bool,
}

impl<FS: FileSystem, LLM: LlmProvider> Session<FS, LLM> {
    /// Create a new session.
    pub fn new(
        fs: FS,
        llm: LLM,
        user: UserIdentifier,
        session_id: String,
    ) -> Self {
        // FIX-S12: unified URI scheme (singular "session")
        let session_uri = format!("viking://session/{session_id}");
        Self {
            fs,
            llm,
            user,
            session_id,
            session_uri,
            auto_commit_threshold: DEFAULT_AUTO_COMMIT_THRESHOLD,
            messages: Vec::new(),
            usage_records: Vec::new(),
            compression: SessionCompression::default(),
            stats: SessionStats::default(),
            loaded: false,
        }
    }

    /// Set auto-commit threshold (builder pattern).
    #[must_use]
    pub fn with_auto_commit(mut self, threshold: usize) -> Self {
        self.auto_commit_threshold = threshold;
        self
    }

    // -----------------------------------------------------------------------
    // Accessors
    // -----------------------------------------------------------------------

    /// Current message history.
    pub fn messages(&self) -> &[Message] {
        &self.messages
    }

    /// Session ID.
    pub fn session_id(&self) -> &str {
        &self.session_id
    }

    /// Session URI.
    pub fn session_uri(&self) -> &str {
        &self.session_uri
    }

    /// Session user.
    pub fn user(&self) -> &UserIdentifier {
        &self.user
    }

    /// Compression state.
    pub fn compression(&self) -> &SessionCompression {
        &self.compression
    }

    /// Session statistics.
    pub fn stats(&self) -> &SessionStats {
        &self.stats
    }

    /// Whether the session has been loaded from persistent storage.
    pub fn is_loaded(&self) -> bool {
        self.loaded
    }

    // -----------------------------------------------------------------------
    // Core methods
    // -----------------------------------------------------------------------

    /// Load session state from persistent storage.
    pub async fn load(&mut self) -> Result<(), BoxError> {
        let messages_uri = format!("{}/messages.jsonl", self.session_uri);

        match self.fs.read(&messages_uri).await {
            Ok(content) if !content.trim().is_empty() => {
                self.messages = content
                    .lines()
                    .filter(|l| !l.trim().is_empty())
                    .filter_map(|line| {
                        serde_json::from_str::<Message>(line)
                            .map_err(|e| {
                                warn!("Failed to parse message: {e}");
                                e
                            })
                            .ok()
                    })
                    .collect();
                info!(
                    "Loaded {} messages from {}",
                    self.messages.len(),
                    messages_uri
                );
            }
            Ok(_) => {
                debug!("No messages found at {messages_uri}");
            }
            Err(e) => {
                debug!("Could not read session messages: {e}");
            }
        }

        // FIX-S5: Restore compression_index by scanning history directory
        let history_uri = format!("{}/history", self.session_uri);
        if let Ok(entries) = self.fs.ls(&history_uri).await {
            let archives: Vec<u32> = entries
                .iter()
                .filter(|e| e.name.starts_with("archive_"))
                .filter_map(|e| e.name.strip_prefix("archive_")?.parse::<u32>().ok())
                .collect();
            if let Some(&max_idx) = archives.iter().max() {
                self.compression.compression_index = max_idx;
                self.stats.compression_count = archives.len() as u32;
                debug!("Restored compression_index: {max_idx}");
            }
        }

        self.loaded = true;
        Ok(())
    }

    /// Record context or skill usage.
    pub fn used(&mut self, usage: Usage) {
        match usage.usage_type.as_str() {
            "context" => self.stats.contexts_used += 1,
            "skill" => self.stats.skills_used += 1,
            _ => {}
        }
        self.usage_records.push(usage);
    }

    /// Add a new message and persist to storage.
    ///
    /// Returns `true` if the session has reached the auto-commit threshold
    /// and the caller should trigger a commit.
    pub async fn add_message(&mut self, message: Message) -> Result<bool, BoxError> {
        let content = message.content().to_owned();
        let jsonl = serde_json::to_string(&message)?;
        let messages_uri = format!("{}/messages.jsonl", self.session_uri);

        self.fs.append(&messages_uri, &format!("{jsonl}\n")).await?;

        // FIX-S7/S10: track turns for user only + token estimation
        if message.role == Role::User {
            self.stats.total_turns += 1;
        }

        self.messages.push(message);
        self.stats.total_tokens += (content.len() / 4) as u64;

        debug!(
            "Added message #{} to session {}",
            self.messages.len(),
            self.session_id
        );

        // FIX-S11: check auto-commit threshold
        let should_commit = self.auto_commit_threshold > 0
            && self.stats.total_turns >= self.auto_commit_threshold as u32;
        if should_commit {
            info!(
                "Auto-commit threshold reached ({} >= {})",
                self.stats.total_turns, self.auto_commit_threshold
            );
        }
        Ok(should_commit)
    }

    /// FIX-S2: Update a tool part's status and output within an existing message.
    ///
    /// Searches through messages for a matching `tool_id` and updates its
    /// `tool_status` and `tool_output`. Also persists the tool result file.
    pub async fn update_tool_part(
        &mut self,
        tool_id: &str,
        status: &str,
        output: &str,
    ) -> Result<bool, BoxError> {
        let mut found_tool_uri: Option<String> = None;

        for msg in &mut self.messages {
            for part in &mut msg.parts {
                if let Part::Tool {
                    tool_id: ref tid,
                    ref mut tool_status,
                    ref mut tool_output,
                    ref tool_uri,
                    ..
                } = part
                {
                    if tid == tool_id {
                        *tool_status = status.to_owned();
                        *tool_output = output.to_owned();
                        found_tool_uri = Some(tool_uri.clone());
                        break;
                    }
                }
            }
            if found_tool_uri.is_some() {
                break;
            }
        }

        if let Some(ref uri) = found_tool_uri {
            // Persist tool result to FS
            self.save_tool_result(uri, status, output).await;

            // Rewrite messages.jsonl with updated content
            let messages_uri = format!("{}/messages.jsonl", self.session_uri);
            let lines: Vec<String> = self
                .messages
                .iter()
                .filter_map(|m| serde_json::to_string(m).ok())
                .collect();
            let content = lines.join("\n") + "\n";
            self.fs.write(&messages_uri, &content).await?;
            debug!("Updated tool part {tool_id} → {status}");
        } else {
            warn!("Tool part {tool_id} not found in session messages");
        }

        Ok(found_tool_uri.is_some())
    }

    /// Persist tool result to a separate file under the session.
    async fn save_tool_result(&self, tool_uri: &str, status: &str, output: &str) {
        let result_json = serde_json::json!({
            "status": status,
            "output": output,
        });
        if let Ok(content) = serde_json::to_string_pretty(&result_json) {
            if let Err(e) = self.fs.write(tool_uri, &content).await {
                warn!("Failed to save tool result to {tool_uri}: {e}");
            }
        }
    }

    /// Commit session: archive messages, write AGFS, update stats.
    ///
    /// FIX-S1: Full commit flow matching Python session.py.
    /// Note: Memory extraction must be triggered externally via SessionCompressor.
    pub async fn commit(&mut self) -> Result<CommitResult, BoxError> {
        let mut result = CommitResult {
            session_id: self.session_id.clone(),
            ..CommitResult::default()
        };

        if self.messages.is_empty() {
            return Ok(result);
        }

        // 1. Archive current messages
        self.compression.compression_index += 1;
        let messages_to_archive = self.messages.clone();

        let summary = self.generate_archive_summary(&messages_to_archive).await;
        let abstract_text = Self::extract_abstract_from_summary(&summary);

        self.write_archive(
            self.compression.compression_index,
            &messages_to_archive,
            &abstract_text,
            &summary,
        ).await?;

        self.compression.original_count += messages_to_archive.len() as u32;
        result.archived = true;

        self.messages.clear();
        info!(
            "Archived: {} messages → history/archive_{:03}/",
            messages_to_archive.len(),
            self.compression.compression_index
        );

        // 2. Memory extraction is external (SessionCompressor)
        // Caller should invoke compressor.extract_long_term_memories() separately
        // and update result.memories_extracted

        // 3. Write current messages to AGFS
        self.write_to_agfs(&self.messages.clone()).await?;

        // 4. Create relations
        self.write_relations().await;

        // 5. Update stats
        self.stats.compression_count = self.compression.compression_index;
        result.stats = Some(self.stats.clone());
        result.status = "committed".to_owned();

        self.stats.total_tokens = 0;
        info!("Session {} committed", self.session_id);
        Ok(result)
    }

    /// Get context summary for search queries.
    ///
    /// FIX-S8: Enhanced to include archive summaries when available.
    pub async fn get_context_for_search(
        &self,
        query: &str,
        max_archives: usize,
        max_messages: usize,
    ) -> Result<SearchContext, BoxError> {
        // 1. Recent messages
        let start = self.messages.len().saturating_sub(max_messages);
        let recent_messages: Vec<Message> = self.messages[start..].to_vec();

        // 2. Find most relevant archives
        let mut summaries: Vec<String> = Vec::new();
        if self.compression.compression_index > 0 {
            let history_uri = format!("{}/history", self.session_uri);
            if let Ok(entries) = self.fs.ls(&history_uri).await {
                let query_lower = query.to_lowercase();
                let mut scored: Vec<(usize, u32, String)> = Vec::new();

                for entry in &entries {
                    if entry.name.starts_with("archive_") {
                        let overview_uri = format!(
                            "{}/history/{}/.overview.md",
                            self.session_uri, entry.name
                        );
                        if let Ok(overview) = self.fs.read(&overview_uri).await {
                            let score = overview.to_lowercase()
                                .matches(&query_lower).count();
                            let num = entry.name
                                .strip_prefix("archive_")
                                .and_then(|s| s.parse::<u32>().ok())
                                .unwrap_or(0);
                            scored.push((score, num, overview));
                        }
                    }
                }

                scored.sort_by(|a, b| b.0.cmp(&a.0).then(b.1.cmp(&a.1)));
                summaries = scored.into_iter()
                    .take(max_archives)
                    .map(|(_, _, overview)| overview)
                    .collect();
            }
        }

        Ok(SearchContext {
            summaries,
            recent_messages,
        })
    }

    // -----------------------------------------------------------------------
    // Internal
    // -----------------------------------------------------------------------

    /// FIX-S3: Generate structured summary for archive using LLM.
    async fn generate_archive_summary(&self, messages: &[Message]) -> String {
        if messages.is_empty() {
            return String::new();
        }

        let formatted: String = messages
            .iter()
            .map(|m| format!("[{:?}]: {}", m.role, m.content()))
            .collect::<Vec<_>>()
            .join("\n");

        let prompt = format!(
            "Summarize this conversation concisely in structured markdown:\n\n{formatted}"
        );

        match self.llm.completion(&prompt).await {
            Ok(summary) => summary,
            Err(e) => {
                warn!("LLM summary failed: {e}");
                let turn_count = messages.iter()
                    .filter(|m| m.role == Role::User)
                    .count();
                format!("# Session Summary\n\n**Overview**: {} turns, {} messages",
                    turn_count, messages.len())
            }
        }
    }

    /// Extract one-sentence abstract from structured summary.
    fn extract_abstract_from_summary(summary: &str) -> String {
        if summary.is_empty() {
            return String::new();
        }

        // Match "**Something**: rest of line"
        if let Ok(re) = Regex::new(r"(?m)^\*\*[^*]+\*\*:\s*(.+)$") {
            if let Some(caps) = re.captures(summary) {
                if let Some(m) = caps.get(1) {
                    return m.as_str().trim().to_owned();
                }
            }
        }

        summary.lines().next()
            .map(|l| l.trim().to_owned())
            .unwrap_or_default()
    }

    /// Generate one-sentence summary for session.
    fn generate_abstract(&self) -> String {
        if self.messages.is_empty() {
            return String::new();
        }
        let first_content = self.messages[0].content();
        let preview: String = first_content.chars().take(50).collect();
        format!("{} turns, starting from '{preview}...'", self.stats.total_turns)
    }

    /// Generate session directory structure description.
    fn generate_overview(&self, turn_count: usize) -> String {
        let mut parts = vec![
            "# Session Directory Structure".to_owned(),
            String::new(),
            "## File Description".to_owned(),
            format!("- `messages.jsonl` - Current messages ({turn_count} turns)"),
        ];
        if self.compression.compression_index > 0 {
            parts.push(format!(
                "- `history/` - Historical archives ({} total)",
                self.compression.compression_index
            ));
        }
        parts.extend([
            String::new(),
            "## Access Methods".to_owned(),
            format!("- Full conversation: `{}`", self.session_uri),
        ]);
        if self.compression.compression_index > 0 {
            parts.push(format!("- Historical archives: `{}/history/`", self.session_uri));
        }
        parts.join("\n")
    }

    /// FIX-S9: Write archive with L0 abstract + L1 overview.
    async fn write_archive(
        &self,
        index: u32,
        messages: &[Message],
        abstract_text: &str,
        overview: &str,
    ) -> Result<(), BoxError> {
        let archive_uri = format!(
            "{}/history/archive_{:03}",
            self.session_uri, index
        );
        let lines: Vec<String> = messages
            .iter()
            .filter_map(|m| serde_json::to_string(m).ok())
            .collect();
        let content = lines.join("\n") + "\n";

        self.fs
            .write(&format!("{archive_uri}/messages.jsonl"), &content)
            .await?;
        self.fs
            .write(&format!("{archive_uri}/.abstract.md"), abstract_text)
            .await?;
        self.fs
            .write(&format!("{archive_uri}/.overview.md"), overview)
            .await?;

        debug!("Written archive: {archive_uri}");
        Ok(())
    }

    /// FIX-S4: Write messages.jsonl + L0/L1 to AGFS.
    async fn write_to_agfs(&self, messages: &[Message]) -> Result<(), BoxError> {
        let turn_count = messages.iter()
            .filter(|m| m.role == Role::User)
            .count();

        let abstract_text = self.generate_abstract();
        let overview = self.generate_overview(turn_count);

        let lines: Vec<String> = messages
            .iter()
            .filter_map(|m| serde_json::to_string(m).ok())
            .collect();
        let content = if lines.is_empty() {
            String::new()
        } else {
            lines.join("\n") + "\n"
        };

        self.fs
            .write(&format!("{}/messages.jsonl", self.session_uri), &content)
            .await?;
        self.fs
            .write(&format!("{}/.abstract.md", self.session_uri), &abstract_text)
            .await?;
        self.fs
            .write(&format!("{}/.overview.md", self.session_uri), &overview)
            .await?;

        Ok(())
    }

    /// Write relations to used contexts/skills.
    async fn write_relations(&self) {
        for usage in &self.usage_records {
            if let Err(e) = self.fs.link(&self.session_uri, &usage.uri).await {
                warn!("Failed to create relation to {}: {e}", usage.uri);
            } else {
                debug!("Created relation: {} -> {}", self.session_uri, usage.uri);
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
    use crate::traits::{BoxError, FsEntry};
    use async_trait::async_trait;
    use std::sync::Mutex;

    #[derive(Clone)]
    struct MockFs {
        store: std::sync::Arc<Mutex<std::collections::HashMap<String, String>>>,
    }
    impl MockFs {
        fn new() -> Self {
            Self {
                store: std::sync::Arc::new(Mutex::new(std::collections::HashMap::new())),
            }
        }
    }

    #[async_trait]
    impl FileSystem for MockFs {
        async fn read(&self, uri: &str) -> Result<String, BoxError> {
            let s = self.store.lock().unwrap();
            s.get(uri).cloned().ok_or_else(|| "not found".into())
        }
        async fn read_bytes(&self, _: &str) -> Result<Vec<u8>, BoxError> { Ok(Vec::new()) }
        async fn write(&self, uri: &str, content: &str) -> Result<(), BoxError> {
            self.store.lock().unwrap().insert(uri.to_owned(), content.to_owned());
            Ok(())
        }
        async fn write_bytes(&self, _: &str, _: &[u8]) -> Result<(), BoxError> { Ok(()) }
        async fn mkdir(&self, _: &str) -> Result<(), BoxError> { Ok(()) }
        async fn ls(&self, _: &str) -> Result<Vec<FsEntry>, BoxError> { Ok(Vec::new()) }
        async fn rm(&self, _: &str) -> Result<(), BoxError> { Ok(()) }
        async fn mv(&self, _: &str, _: &str) -> Result<(), BoxError> { Ok(()) }
        async fn stat(&self, _: &str) -> Result<crate::traits::FsStat, BoxError> { Err("not implemented".into()) }
        async fn grep(&self, _: &str, _: &str, _: bool, _: bool) -> Result<Vec<crate::traits::GrepMatch>, BoxError> { Ok(Vec::new()) }
        async fn exists(&self, _: &str) -> Result<bool, BoxError> { Ok(false) }
        async fn append(&self, uri: &str, content: &str) -> Result<(), BoxError> {
            let mut s = self.store.lock().unwrap();
            let entry = s.entry(uri.to_owned()).or_default();
            entry.push_str(content);
            Ok(())
        }
        async fn link(&self, _: &str, _: &str) -> Result<(), BoxError> { Ok(()) }
    }

    struct MockLlm;
    #[async_trait]
    impl LlmProvider for MockLlm {
        async fn completion(&self, _: &str) -> Result<String, BoxError> { Ok(String::new()) }
    }

    fn make_session() -> Session<MockFs, MockLlm> {
        Session::new(
            MockFs::new(),
            MockLlm,
            UserIdentifier::default_user(),
            "test-session".to_owned(),
        )
    }

    #[test]
    fn accessors_work() {
        let s = make_session();
        assert_eq!(s.session_id(), "test-session");
        assert_eq!(s.session_uri(), "viking://session/test-session");
        assert!(s.messages().is_empty());
        assert!(!s.is_loaded());
    }

    #[tokio::test]
    async fn add_message_persists() {
        let mut s = make_session();
        let msg = Message::create_user("hello");
        s.add_message(msg).await.unwrap();
        assert_eq!(s.messages().len(), 1);
        assert_eq!(s.stats().total_turns, 1);
    }

    #[tokio::test]
    async fn commit_archives_and_clears() {
        let mut s = make_session();
        s.add_message(Message::create_user("hello"))
            .await
            .unwrap();
        let result = s.commit().await.unwrap();
        assert!(s.messages().is_empty());
        assert_eq!(s.compression().compression_index, 1);
        assert!(result.archived);
        assert_eq!(result.status, "committed");
    }

    #[tokio::test]
    async fn get_context_empty() {
        let s = make_session();
        let ctx = s.get_context_for_search("test", 3, 20).await.unwrap();
        assert!(ctx.recent_messages.is_empty());
        assert!(ctx.summaries.is_empty());
    }

    #[test]
    fn usage_tracking() {
        let mut s = make_session();
        s.used(Usage {
            uri: "viking://test".into(),
            usage_type: "context".into(),
            contribution: 1.0,
            input: String::new(),
            output: String::new(),
            success: true,
            timestamp: String::new(),
        });
        assert_eq!(s.stats().contexts_used, 1);
    }

    #[test]
    fn extract_abstract_from_summary() {
        let summary = "**Overview**: A session about Rust porting\n\nMore details...";
        let result = Session::<MockFs, MockLlm>::extract_abstract_from_summary(summary);
        assert_eq!(result, "A session about Rust porting");
    }
}
