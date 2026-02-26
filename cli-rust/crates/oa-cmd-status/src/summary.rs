/// Status summary builder.
///
/// Aggregates session data, heartbeat info, and channel summaries
/// into a comprehensive `StatusSummary`.
///
/// Source: `src/commands/status.summary.ts`

use std::collections::HashMap;

use oa_config::io::load_config;
use oa_config::sessions::paths::resolve_store_path;
use oa_config::sessions::store::load_session_store;
use oa_types::config::OpenAcosmiConfig;
use oa_types::session::SessionEntry;

use crate::agent_local::list_agents_for_config;
use crate::types::{
    AgentSessionSummary, HeartbeatStatus, HeartbeatSummary, SessionDefaults, SessionKind,
    SessionStatus, SessionsInfo, StatusSummary,
};

/// Default model name.
const DEFAULT_MODEL: &str = "claude-sonnet-4-20250514";

/// Default context token limit.
const DEFAULT_CONTEXT_TOKENS: u64 = 200_000;

/// Classify a session key into its kind.
///
/// Source: `src/commands/status.summary.ts` - `classifyKey`
fn classify_key(key: &str, entry: Option<&SessionEntry>) -> SessionKind {
    if key == "global" {
        return SessionKind::Global;
    }
    if key == "unknown" {
        return SessionKind::Unknown;
    }
    if let Some(e) = entry {
        if let Some(ref ct) = e.chat_type {
            match ct {
                oa_types::common::ChatType::Group | oa_types::common::ChatType::Channel => {
                    return SessionKind::Group;
                }
                _ => {}
            }
        }
    }
    if key.contains(":group:") || key.contains(":channel:") {
        return SessionKind::Group;
    }
    SessionKind::Direct
}

/// Build session flags from a session entry.
///
/// Source: `src/commands/status.summary.ts` - `buildFlags`
fn build_flags(entry: Option<&SessionEntry>) -> Vec<String> {
    let Some(e) = entry else {
        return vec![];
    };
    let mut flags: Vec<String> = Vec::new();
    if let Some(ref think) = e.thinking_level {
        if !think.is_empty() {
            flags.push(format!("think:{think}"));
        }
    }
    if let Some(ref verbose) = e.verbose_level {
        if !verbose.is_empty() {
            flags.push(format!("verbose:{verbose}"));
        }
    }
    if let Some(ref reasoning) = e.reasoning_level {
        if !reasoning.is_empty() {
            flags.push(format!("reasoning:{reasoning}"));
        }
    }
    if let Some(ref elevated) = e.elevated_level {
        if !elevated.is_empty() {
            flags.push(format!("elevated:{elevated}"));
        }
    }
    if e.system_sent == Some(true) {
        flags.push("system".to_string());
    }
    if e.aborted_last_run == Some(true) {
        flags.push("aborted".to_string());
    }
    flags
}

/// Build session status rows from a session store.
///
/// Source: `src/commands/status.summary.ts` - `buildSessionRows`
fn build_session_rows(
    store: &HashMap<String, SessionEntry>,
    config_model: &str,
    config_context_tokens: u64,
    now: u64,
    agent_id_override: Option<&str>,
) -> Vec<SessionStatus> {
    let mut rows: Vec<SessionStatus> = store
        .iter()
        .filter(|(key, _)| key.as_str() != "global" && key.as_str() != "unknown")
        .map(|(key, entry)| {
            let updated_at = Some(entry.updated_at).filter(|&ts| ts > 0);
            let age = updated_at.map(|ts| now.saturating_sub(ts));
            let model = entry
                .model
                .as_deref()
                .unwrap_or(config_model)
                .to_string();
            let context_tokens = entry.context_tokens.unwrap_or(config_context_tokens);
            let input = entry.input_tokens.unwrap_or(0);
            let output = entry.output_tokens.unwrap_or(0);
            let total = entry.total_tokens.unwrap_or(input + output);
            let remaining = if context_tokens > 0 {
                Some(context_tokens as i64 - total as i64)
            } else {
                None
            };
            let pct = if context_tokens > 0 {
                Some(((total as f64 / context_tokens as f64) * 100.0).round().min(999.0) as u64)
            } else {
                None
            };
            let agent_id = agent_id_override.map(String::from);

            SessionStatus {
                agent_id,
                key: key.clone(),
                kind: classify_key(key, Some(entry)),
                session_id: Some(entry.session_id.clone()),
                updated_at,
                age,
                thinking_level: entry.thinking_level.clone(),
                verbose_level: entry.verbose_level.clone(),
                reasoning_level: entry.reasoning_level.clone(),
                elevated_level: entry.elevated_level.clone(),
                system_sent: entry.system_sent,
                aborted_last_run: entry.aborted_last_run,
                input_tokens: entry.input_tokens,
                output_tokens: entry.output_tokens,
                total_tokens: Some(total),
                remaining_tokens: remaining,
                percent_used: pct,
                model: Some(model),
                context_tokens: Some(context_tokens),
                flags: build_flags(Some(entry)),
            }
        })
        .collect();

    rows.sort_by(|a, b| b.updated_at.cmp(&a.updated_at));
    rows
}

/// Get the full status summary.
///
/// Aggregates sessions from all agents, resolves heartbeat info,
/// and builds the `StatusSummary`.
///
/// Source: `src/commands/status.summary.ts` - `getStatusSummary`
pub fn get_status_summary() -> StatusSummary {
    let cfg = load_config().unwrap_or_default();
    get_status_summary_with_config(&cfg)
}

/// Get status summary with a pre-loaded config.
///
/// Source: `src/commands/status.summary.ts` - `getStatusSummary`
#[must_use]
pub fn get_status_summary_with_config(cfg: &OpenAcosmiConfig) -> StatusSummary {
    let (default_id, agents) = list_agents_for_config(cfg);

    let config_model = cfg
        .agents
        .as_ref()
        .and_then(|a| a.defaults.as_ref())
        .and_then(|d| d.model.as_ref())
        .and_then(|m| m.primary.as_deref())
        .unwrap_or(DEFAULT_MODEL);
    let config_context_tokens = cfg
        .agents
        .as_ref()
        .and_then(|a| a.defaults.as_ref())
        .and_then(|d| d.context_tokens)
        .unwrap_or(DEFAULT_CONTEXT_TOKENS);

    let now = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis() as u64;

    let session_store_template = cfg.session.as_ref().and_then(|s| s.store.as_deref());

    let mut all_paths = std::collections::BTreeSet::new();
    let mut store_cache: HashMap<String, HashMap<String, SessionEntry>> = HashMap::new();

    let load_store_cached =
        |path_str: &str, cache: &mut HashMap<String, HashMap<String, SessionEntry>>| {
            if let Some(cached) = cache.get(path_str) {
                return cached.clone();
            }
            let store = load_session_store(std::path::Path::new(path_str));
            cache.insert(path_str.to_string(), store.clone());
            store
        };

    let by_agent: Vec<AgentSessionSummary> = agents
        .iter()
        .map(|agent| {
            let store_path = resolve_store_path(session_store_template, Some(&agent.id));
            let path_str = store_path.display().to_string();
            all_paths.insert(path_str.clone());
            let store = load_store_cached(&path_str, &mut store_cache);
            let sessions =
                build_session_rows(&store, config_model, config_context_tokens, now, Some(&agent.id));
            let count = sessions.len();
            let recent: Vec<SessionStatus> = sessions.into_iter().take(10).collect();
            AgentSessionSummary {
                agent_id: agent.id.clone(),
                path: path_str,
                count,
                recent,
            }
        })
        .collect();

    // Build aggregated recent sessions across all stores.
    let mut all_sessions: Vec<SessionStatus> = Vec::new();
    for path_str in &all_paths {
        let store = load_store_cached(path_str, &mut store_cache);
        let sessions = build_session_rows(&store, config_model, config_context_tokens, now, None);
        all_sessions.extend(sessions);
    }
    all_sessions.sort_by(|a, b| b.updated_at.cmp(&a.updated_at));
    let recent: Vec<SessionStatus> = all_sessions.iter().take(10).cloned().collect();
    let total_count = all_sessions.len();

    // Build heartbeat summary.
    let heartbeat_agents: Vec<HeartbeatStatus> = agents
        .iter()
        .map(|agent| {
            // Heartbeat configuration resolution is a stub.
            HeartbeatStatus {
                agent_id: agent.id.clone(),
                enabled: false,
                every: "off".to_string(),
                every_ms: None,
            }
        })
        .collect();

    StatusSummary {
        link_channel: None,
        heartbeat: HeartbeatSummary {
            default_agent_id: default_id,
            agents: heartbeat_agents,
        },
        channel_summary: vec![],
        queued_system_events: vec![],
        sessions: SessionsInfo {
            paths: all_paths.into_iter().collect(),
            count: total_count,
            defaults: SessionDefaults {
                model: Some(config_model.to_string()),
                context_tokens: Some(config_context_tokens),
            },
            recent,
            by_agent,
        },
    }
}

#[cfg(test)]
mod tests {
    use super::*;

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
            classify_key("agent:group:123", None),
            SessionKind::Group
        );
        assert_eq!(
            classify_key("agent:channel:456", None),
            SessionKind::Group
        );
    }

    #[test]
    fn classify_key_direct() {
        assert_eq!(classify_key("user:dm:alice", None), SessionKind::Direct);
    }

    #[test]
    fn build_flags_empty() {
        assert!(build_flags(None).is_empty());
    }

    #[test]
    fn build_flags_with_values() {
        let entry = SessionEntry {
            session_id: "s1".to_string(),
            updated_at: 1000,
            thinking_level: Some("high".to_string()),
            system_sent: Some(true),
            aborted_last_run: Some(true),
            ..create_default_entry()
        };
        let flags = build_flags(Some(&entry));
        assert!(flags.contains(&"think:high".to_string()));
        assert!(flags.contains(&"system".to_string()));
        assert!(flags.contains(&"aborted".to_string()));
    }

    #[test]
    fn build_session_rows_empty_store() {
        let store: HashMap<String, SessionEntry> = HashMap::new();
        let rows = build_session_rows(&store, "gpt-4", 128_000, 1_000_000, None);
        assert!(rows.is_empty());
    }

    #[test]
    fn build_session_rows_filters_global() {
        let mut store: HashMap<String, SessionEntry> = HashMap::new();
        store.insert("global".to_string(), create_default_entry());
        store.insert("user:dm:alice".to_string(), create_default_entry());
        let rows = build_session_rows(&store, "gpt-4", 128_000, 1_000_000, None);
        assert_eq!(rows.len(), 1);
        assert_eq!(rows[0].key, "user:dm:alice");
    }

    #[test]
    fn get_summary_default_config() {
        let cfg = OpenAcosmiConfig::default();
        let summary = get_status_summary_with_config(&cfg);
        assert_eq!(summary.heartbeat.default_agent_id, "main");
        assert!(summary.sessions.defaults.model.is_some());
    }

    /// Helper to create a default `SessionEntry` for tests.
    fn create_default_entry() -> SessionEntry {
        SessionEntry {
            session_id: "test".to_string(),
            updated_at: 1000,
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
            input_tokens: None,
            output_tokens: None,
            total_tokens: None,
            model_provider: None,
            model: None,
            context_tokens: None,
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
}
