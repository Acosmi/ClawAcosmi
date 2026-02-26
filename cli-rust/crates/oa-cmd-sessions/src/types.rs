/// Session row types for the sessions command.
///
/// Source: `src/commands/sessions.ts` - `SessionRow`, `classifyKey`, `toRows`

use std::collections::HashMap;

use serde::{Deserialize, Serialize};

use oa_types::session::SessionEntry;

/// Session kind classification.
///
/// Source: `src/commands/sessions.ts` - `SessionRow["kind"]`
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum SessionKind {
    /// Direct message session.
    Direct,
    /// Group or channel session.
    Group,
    /// Global (system-wide) session.
    Global,
    /// Unknown or legacy session.
    Unknown,
}

impl SessionKind {
    /// Returns the string representation of the session kind.
    ///
    /// Source: `src/commands/sessions.ts` - kind display
    pub fn as_str(&self) -> &str {
        match self {
            Self::Direct => "direct",
            Self::Group => "group",
            Self::Global => "global",
            Self::Unknown => "unknown",
        }
    }
}

impl std::fmt::Display for SessionKind {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(self.as_str())
    }
}

/// A row in the sessions table output.
///
/// Source: `src/commands/sessions.ts` - `SessionRow`
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SessionRow {
    /// Session key.
    pub key: String,
    /// Classification of the session.
    pub kind: SessionKind,
    /// Last update timestamp (epoch ms).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub updated_at: Option<u64>,
    /// Age in milliseconds since last update.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub age_ms: Option<u64>,
    /// Session identifier.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub session_id: Option<String>,
    /// Whether the system prompt has been sent.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub system_sent: Option<bool>,
    /// Whether the last run was aborted.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub aborted_last_run: Option<bool>,
    /// Thinking level override.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub thinking_level: Option<String>,
    /// Verbose level override.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub verbose_level: Option<String>,
    /// Reasoning level override.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub reasoning_level: Option<String>,
    /// Elevated level override.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub elevated_level: Option<String>,
    /// Response usage mode.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub response_usage: Option<String>,
    /// Group activation mode.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub group_activation: Option<String>,
    /// Input tokens consumed.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub input_tokens: Option<u64>,
    /// Output tokens consumed.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub output_tokens: Option<u64>,
    /// Total tokens consumed.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub total_tokens: Option<u64>,
    /// Model identifier.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model: Option<String>,
    /// Context window size.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub context_tokens: Option<u64>,
}

/// Classify a session key into a session kind.
///
/// Source: `src/commands/sessions.ts` - `classifyKey`
pub fn classify_key(key: &str, entry: Option<&SessionEntry>) -> SessionKind {
    if key == "global" {
        return SessionKind::Global;
    }
    if key == "unknown" {
        return SessionKind::Unknown;
    }
    if let Some(entry) = entry {
        if let Some(ref chat_type) = entry.chat_type {
            let ct_str = serde_json::to_string(chat_type).unwrap_or_default();
            if ct_str.contains("group") || ct_str.contains("channel") {
                return SessionKind::Group;
            }
        }
    }
    if key.contains(":group:") || key.contains(":channel:") {
        return SessionKind::Group;
    }
    SessionKind::Direct
}

/// Convert a session store to sorted rows.
///
/// Sorts by `updated_at` descending (most recent first).
///
/// Source: `src/commands/sessions.ts` - `toRows`
pub fn to_rows(store: &HashMap<String, SessionEntry>) -> Vec<SessionRow> {
    let now = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map_or(0, |d| d.as_millis() as u64);

    let mut rows: Vec<SessionRow> = store
        .iter()
        .map(|(key, entry)| {
            let updated_at = if entry.updated_at > 0 {
                Some(entry.updated_at)
            } else {
                None
            };

            SessionRow {
                key: key.clone(),
                kind: classify_key(key, Some(entry)),
                updated_at,
                age_ms: updated_at.map(|ts| now.saturating_sub(ts)),
                session_id: Some(entry.session_id.clone()),
                system_sent: entry.system_sent,
                aborted_last_run: entry.aborted_last_run,
                thinking_level: entry.thinking_level.clone(),
                verbose_level: entry.verbose_level.clone(),
                reasoning_level: entry.reasoning_level.clone(),
                elevated_level: entry.elevated_level.clone(),
                response_usage: entry.response_usage.clone(),
                group_activation: entry.group_activation.clone(),
                input_tokens: entry.input_tokens,
                output_tokens: entry.output_tokens,
                total_tokens: entry.total_tokens,
                model: entry.model.clone(),
                context_tokens: entry.context_tokens,
            }
        })
        .collect();

    rows.sort_by(|a, b| {
        let a_ts = a.updated_at.unwrap_or(0);
        let b_ts = b.updated_at.unwrap_or(0);
        b_ts.cmp(&a_ts)
    });

    rows
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_entry(updated_at: u64) -> SessionEntry {
        SessionEntry {
            session_id: "test-id".to_string(),
            updated_at,
            last_heartbeat_text: None,
            last_heartbeat_sent_at: None,
            session_file: None,
            spawned_by: None,
            system_sent: None,
            aborted_last_run: None,
            chat_type: None,
            thinking_level: None,
            verbose_level: None,
            reasoning_level: None,
            elevated_level: None,
            tts_auto: None,
            exec_host: None,
            exec_security: None,
            exec_ask: None,
            exec_node: None,
            response_usage: None,
            provider_override: None,
            model_override: None,
            auth_profile_override: None,
            auth_profile_override_source: None,
            auth_profile_override_compaction_count: None,
            group_activation: None,
            group_activation_needs_system_intro: None,
            send_policy: None,
            queue_mode: None,
            queue_debounce_ms: None,
            queue_cap: None,
            queue_drop: None,
            input_tokens: Some(1000),
            output_tokens: Some(500),
            total_tokens: Some(1500),
            model_provider: None,
            model: Some("claude-opus-4-6".to_string()),
            context_tokens: Some(200_000),
            compaction_count: None,
            memory_flush_at: None,
            memory_flush_compaction_count: None,
            cli_session_ids: None,
            claude_cli_session_id: None,
            label: None,
            display_name: None,
            channel: None,
            group_id: None,
            subject: None,
            group_channel: None,
            space: None,
            origin: None,
            delivery_context: None,
            last_channel: None,
            last_to: None,
            last_account_id: None,
            last_thread_id: None,
            skills_snapshot: None,
            system_prompt_report: None,
        }
    }

    #[test]
    fn classify_key_global() {
        assert_eq!(classify_key("global", None), SessionKind::Global);
    }

    #[test]
    fn classify_key_unknown() {
        assert_eq!(classify_key("unknown", None), SessionKind::Unknown);
    }

    #[test]
    fn classify_key_group_by_pattern() {
        assert_eq!(
            classify_key("agent:bot:discord:group:general", None),
            SessionKind::Group
        );
    }

    #[test]
    fn classify_key_channel_by_pattern() {
        // `:channel:` in the key maps to Group kind (channels are a type of group)
        assert_eq!(
            classify_key("agent:bot:slack:channel:C123", None),
            SessionKind::Group
        );
    }

    #[test]
    fn classify_key_direct() {
        assert_eq!(
            classify_key("agent:bot:main", None),
            SessionKind::Direct
        );
    }

    #[test]
    fn to_rows_sorts_by_updated_at() {
        let mut store = HashMap::new();
        store.insert("key1".to_string(), make_entry(100));
        store.insert("key2".to_string(), make_entry(300));
        store.insert("key3".to_string(), make_entry(200));

        let rows = to_rows(&store);
        assert_eq!(rows.len(), 3);
        assert_eq!(rows[0].key, "key2");
        assert_eq!(rows[1].key, "key3");
        assert_eq!(rows[2].key, "key1");
    }

    #[test]
    fn to_rows_empty_store() {
        let store = HashMap::new();
        let rows = to_rows(&store);
        assert!(rows.is_empty());
    }

    #[test]
    fn session_row_serializes() {
        let row = SessionRow {
            key: "test-key".to_string(),
            kind: SessionKind::Direct,
            updated_at: Some(1_700_000_000_000),
            age_ms: Some(60_000),
            session_id: Some("sid-123".to_string()),
            system_sent: Some(true),
            aborted_last_run: None,
            thinking_level: Some("low".to_string()),
            verbose_level: None,
            reasoning_level: None,
            elevated_level: None,
            response_usage: None,
            group_activation: None,
            input_tokens: Some(1000),
            output_tokens: Some(500),
            total_tokens: Some(1500),
            model: Some("claude-opus-4-6".to_string()),
            context_tokens: Some(200_000),
        };
        let json = serde_json::to_string(&row).expect("should serialize");
        // Field "key" stays as "key" in camelCase (single word unchanged)
        assert!(json.contains("\"key\":\"test-key\""));
        // Enum variant Direct serializes as "direct" via camelCase rename
        assert!(json.contains("\"direct\""));
        // Multi-word fields get camelCase: updatedAt, sessionId, thinkingLevel, etc.
        assert!(json.contains("\"updatedAt\""));
        assert!(json.contains("\"sessionId\""));
        assert!(json.contains("\"thinkingLevel\""));
    }

    #[test]
    fn session_kind_as_str() {
        assert_eq!(SessionKind::Direct.as_str(), "direct");
        assert_eq!(SessionKind::Group.as_str(), "group");
        assert_eq!(SessionKind::Global.as_str(), "global");
        assert_eq!(SessionKind::Unknown.as_str(), "unknown");
    }

    #[test]
    fn session_kind_display() {
        assert_eq!(SessionKind::Direct.to_string(), "direct");
        assert_eq!(SessionKind::Group.to_string(), "group");
    }
}
