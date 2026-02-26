/// Session file path resolution.
///
/// Resolves paths to session transcript files and the session store
/// (sessions.json) within the agent-specific sessions directory.
///
/// Source: `src/config/sessions/paths.ts`

use std::path::PathBuf;

use crate::paths::resolve_state_dir;

/// Default agent identifier used when no agent is specified.
const DEFAULT_AGENT_ID: &str = "default";

/// Normalize an agent ID to a filesystem-safe form.
fn normalize_agent_id(agent_id: &str) -> String {
    let trimmed = agent_id.trim();
    if trimmed.is_empty() {
        return DEFAULT_AGENT_ID.to_string();
    }
    trimmed.to_lowercase()
}

/// Resolve the sessions directory for a given agent.
fn resolve_agent_sessions_dir(agent_id: Option<&str>) -> PathBuf {
    let root = resolve_state_dir();
    let id = normalize_agent_id(agent_id.unwrap_or(DEFAULT_AGENT_ID));
    root.join("agents").join(id).join("sessions")
}

/// Resolve the session transcripts directory for the default agent.
pub fn resolve_session_transcripts_dir() -> PathBuf {
    resolve_agent_sessions_dir(Some(DEFAULT_AGENT_ID))
}

/// Resolve the session transcripts directory for a specific agent.
pub fn resolve_session_transcripts_dir_for_agent(agent_id: Option<&str>) -> PathBuf {
    resolve_agent_sessions_dir(agent_id)
}

/// Resolve the default session store file path for a given agent.
///
/// Returns `<state_dir>/agents/<agent_id>/sessions/sessions.json`.
pub fn resolve_default_session_store_path(agent_id: Option<&str>) -> PathBuf {
    resolve_agent_sessions_dir(agent_id).join("sessions.json")
}

/// Resolve the path to a session transcript file.
///
/// Returns `<sessions_dir>/<session_id>.jsonl` or
/// `<sessions_dir>/<session_id>-topic-<topic_id>.jsonl` if a topic is provided.
pub fn resolve_session_transcript_path(
    session_id: &str,
    agent_id: Option<&str>,
    topic_id: Option<&str>,
) -> PathBuf {
    let file_name = match topic_id {
        Some(tid) if !tid.is_empty() => {
            // URL-encode the topic ID for filesystem safety
            let safe_topic = urlencoding_encode(tid);
            format!("{session_id}-topic-{safe_topic}.jsonl")
        }
        _ => format!("{session_id}.jsonl"),
    };
    resolve_agent_sessions_dir(agent_id).join(file_name)
}

/// Resolve the store path, with optional template expansion.
///
/// If `store` contains `{agentId}`, it is replaced with the normalized agent ID.
/// Tilde expansion is supported.
pub fn resolve_store_path(store: Option<&str>, agent_id: Option<&str>) -> PathBuf {
    let id = normalize_agent_id(agent_id.unwrap_or(DEFAULT_AGENT_ID));
    match store {
        None | Some("") => resolve_default_session_store_path(Some(&id)),
        Some(s) if s.contains("{agentId}") => {
            let expanded = s.replace("{agentId}", &id);
            expand_and_resolve(&expanded)
        }
        Some(s) => expand_and_resolve(s),
    }
}

/// Expand a leading tilde and resolve the path.
fn expand_and_resolve(input: &str) -> PathBuf {
    if let Some(rest) = input.strip_prefix('~') {
        let home = dirs::home_dir().unwrap_or_else(|| PathBuf::from("."));
        let rest = rest.strip_prefix('/').unwrap_or(rest);
        home.join(rest)
    } else {
        PathBuf::from(input)
            .canonicalize()
            .unwrap_or_else(|_| PathBuf::from(input))
    }
}

/// Simple percent-encoding for topic IDs (mimics `encodeURIComponent`).
fn urlencoding_encode(input: &str) -> String {
    let mut result = String::new();
    for byte in input.bytes() {
        match byte {
            b'A'..=b'Z'
            | b'a'..=b'z'
            | b'0'..=b'9'
            | b'-'
            | b'_'
            | b'.'
            | b'~' => result.push(byte as char),
            _ => {
                result.push('%');
                result.push_str(&format!("{byte:02X}"));
            }
        }
    }
    result
}
