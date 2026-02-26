/// Session resolution for the agent command.
///
/// Resolves which session to use for an agent run based on the provided
/// options (--to, --session-id, --session-key, --agent). Handles session
/// freshness evaluation, session key construction, and persisted thinking /
/// verbose level retrieval.
///
/// Source: `src/commands/agent/session.ts`

use std::collections::HashMap;
use std::path::PathBuf;

use oa_config::sessions::paths::resolve_store_path;
use oa_config::sessions::store::load_session_store;
use oa_routing::session_key::{
    build_agent_main_session_key, normalize_agent_id, normalize_main_key,
    resolve_agent_id_from_session_key,
};
use oa_types::config::OpenAcosmiConfig;
use oa_types::session::SessionEntry;

/// Thinking level strings recognized by the system.
///
/// Source: `src/auto-reply/thinking.ts` - `ThinkLevel`
const THINKING_LEVELS: &[&str] = &["off", "minimal", "low", "medium", "high", "xhigh"];

/// Verbose level strings recognized by the system.
///
/// Source: `src/auto-reply/thinking.ts` - `VerboseLevel`
const VERBOSE_LEVELS: &[&str] = &["off", "on", "full"];

/// Normalize a thinking level string. Returns `None` for unrecognized values.
///
/// Source: `src/auto-reply/thinking.ts` - `normalizeThinkLevel`
pub fn normalize_think_level(value: Option<&str>) -> Option<String> {
    let trimmed = value?.trim().to_lowercase();
    if THINKING_LEVELS.contains(&trimmed.as_str()) {
        Some(trimmed)
    } else {
        None
    }
}

/// Normalize a verbose level string. Returns `None` for unrecognized values.
///
/// Source: `src/auto-reply/thinking.ts` - `normalizeVerboseLevel`
pub fn normalize_verbose_level(value: Option<&str>) -> Option<String> {
    let trimmed = value?.trim().to_lowercase();
    if VERBOSE_LEVELS.contains(&trimmed.as_str()) {
        Some(trimmed)
    } else {
        None
    }
}

/// Result of resolving a session key for a gateway request.
///
/// Source: `src/commands/agent/session.ts` - `SessionKeyResolution`
#[derive(Debug, Clone)]
pub struct SessionKeyResolution {
    /// The resolved session key (if any).
    pub session_key: Option<String>,
    /// The loaded session store.
    pub session_store: HashMap<String, SessionEntry>,
    /// Path to the session store file.
    pub store_path: PathBuf,
}

/// Result of full session resolution.
///
/// Source: `src/commands/agent/session.ts` - `SessionResolution`
#[derive(Debug, Clone)]
pub struct SessionResolution {
    /// The session identifier (UUID).
    pub session_id: String,
    /// The session key (if any).
    pub session_key: Option<String>,
    /// The session entry from the store (if found).
    pub session_entry: Option<SessionEntry>,
    /// The session store map.
    pub session_store: HashMap<String, SessionEntry>,
    /// Path to the session store file.
    pub store_path: PathBuf,
    /// Whether this is a new session (not fresh).
    pub is_new_session: bool,
    /// Persisted thinking level from a fresh session.
    pub persisted_thinking: Option<String>,
    /// Persisted verbose level from a fresh session.
    pub persisted_verbose: Option<String>,
}

/// Resolve the explicit agent session key from configuration.
///
/// Source: `src/config/sessions.ts` - `resolveExplicitAgentSessionKey`
fn resolve_explicit_agent_session_key(
    cfg: &OpenAcosmiConfig,
    agent_id: Option<&str>,
) -> Option<String> {
    // Build a deterministic key from the agent id when one is provided.
    let id = agent_id?;
    let normalized = normalize_agent_id(Some(id));
    let main_key = normalize_main_key(
        cfg.session
            .as_ref()
            .and_then(|s| s.main_key.as_deref()),
    );
    Some(build_agent_main_session_key(&normalized, Some(&main_key)))
}

/// Resolve a session key for a gateway request.
///
/// Source: `src/commands/agent/session.ts` - `resolveSessionKeyForRequest`
pub fn resolve_session_key_for_request(
    cfg: &OpenAcosmiConfig,
    to: Option<&str>,
    session_id: Option<&str>,
    session_key: Option<&str>,
    agent_id: Option<&str>,
) -> SessionKeyResolution {
    let session_cfg = cfg.session.as_ref();
    let main_key = normalize_main_key(
        session_cfg.and_then(|s| s.main_key.as_deref()),
    );

    let explicit_session_key = session_key
        .map(|s| s.trim().to_owned())
        .filter(|s| !s.is_empty())
        .or_else(|| resolve_explicit_agent_session_key(cfg, agent_id));

    let store_agent_id =
        resolve_agent_id_from_session_key(explicit_session_key.as_deref());
    let store_path = resolve_store_path(
        session_cfg.and_then(|s| s.store.as_deref()),
        Some(&store_agent_id),
    );
    let session_store = load_session_store(&store_path);

    // Build session key from context.
    let mut resolved_key = explicit_session_key.clone();

    // For --to, build an agent peer session key using the main scope.
    if resolved_key.is_none() {
        if let Some(to_val) = to {
            let trimmed = to_val.trim();
            if !trimmed.is_empty() {
                let agent = agent_id
                    .map(|a| normalize_agent_id(Some(a)))
                    .unwrap_or_else(|| {
                        resolve_agent_id_from_session_key(None)
                    });
                resolved_key = Some(build_agent_main_session_key(&agent, Some(&main_key)));
            }
        }
    }

    // If a session id was provided, prefer re-using its entry.
    if explicit_session_key.is_none() {
        if let Some(sid) = session_id {
            let sid_trimmed = sid.trim();
            if !sid_trimmed.is_empty() {
                let matches_current = resolved_key
                    .as_ref()
                    .and_then(|k| session_store.get(k))
                    .is_some_and(|e| e.session_id == sid_trimmed);

                if !matches_current {
                    let found_key = session_store
                        .iter()
                        .find(|(_, entry)| entry.session_id == sid_trimmed)
                        .map(|(k, _)| k.clone());
                    if let Some(fk) = found_key {
                        resolved_key = Some(fk);
                    }
                }
            }
        }
    }

    SessionKeyResolution {
        session_key: resolved_key,
        session_store,
        store_path,
    }
}

/// Default session freshness threshold in milliseconds (30 minutes).
///
/// Source: `src/config/sessions.ts` - default freshness
const DEFAULT_FRESHNESS_MS: u64 = 30 * 60 * 1000;

/// Evaluate whether a session is still fresh based on its last update time.
///
/// Source: `src/config/sessions.ts` - `evaluateSessionFreshness`
fn evaluate_session_freshness(updated_at: u64, now: u64, threshold_ms: u64) -> bool {
    if updated_at == 0 {
        return false;
    }
    now.saturating_sub(updated_at) < threshold_ms
}

/// Resolve a full session for the agent command.
///
/// Source: `src/commands/agent/session.ts` - `resolveSession`
pub fn resolve_session(
    cfg: &OpenAcosmiConfig,
    to: Option<&str>,
    session_id: Option<&str>,
    session_key: Option<&str>,
    agent_id: Option<&str>,
) -> SessionResolution {
    let resolution = resolve_session_key_for_request(
        cfg, to, session_id, session_key, agent_id,
    );
    let now = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis() as u64;

    let session_entry = resolution
        .session_key
        .as_ref()
        .and_then(|k| resolution.session_store.get(k))
        .cloned();

    let session_cfg = cfg.session.as_ref();
    let threshold_ms = session_cfg
        .and_then(|s| s.idle_minutes)
        .map(|mins| mins * 60 * 1000)
        .unwrap_or(DEFAULT_FRESHNESS_MS);

    let fresh = session_entry
        .as_ref()
        .is_some_and(|entry| evaluate_session_freshness(entry.updated_at, now, threshold_ms));

    let resolved_session_id = session_id
        .map(|s| s.trim().to_owned())
        .filter(|s| !s.is_empty())
        .or_else(|| {
            if fresh {
                session_entry.as_ref().map(|e| e.session_id.clone())
            } else {
                None
            }
        })
        .unwrap_or_else(|| uuid::Uuid::new_v4().to_string());

    let is_new_session = !fresh && session_id.is_none();

    let persisted_thinking = if fresh {
        session_entry
            .as_ref()
            .and_then(|e| e.thinking_level.as_deref())
            .and_then(|v| normalize_think_level(Some(v)))
    } else {
        None
    };

    let persisted_verbose = if fresh {
        session_entry
            .as_ref()
            .and_then(|e| e.verbose_level.as_deref())
            .and_then(|v| normalize_verbose_level(Some(v)))
    } else {
        None
    };

    SessionResolution {
        session_id: resolved_session_id,
        session_key: resolution.session_key,
        session_entry,
        session_store: resolution.session_store,
        store_path: resolution.store_path,
        is_new_session,
        persisted_thinking,
        persisted_verbose,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn normalize_think_level_valid() {
        assert_eq!(normalize_think_level(Some("high")), Some("high".to_owned()));
        assert_eq!(normalize_think_level(Some("OFF")), Some("off".to_owned()));
        assert_eq!(normalize_think_level(Some(" xhigh ")), Some("xhigh".to_owned()));
    }

    #[test]
    fn normalize_think_level_invalid() {
        assert_eq!(normalize_think_level(Some("super")), None);
        assert_eq!(normalize_think_level(Some("")), None);
        assert_eq!(normalize_think_level(None), None);
    }

    #[test]
    fn normalize_verbose_level_valid() {
        assert_eq!(normalize_verbose_level(Some("on")), Some("on".to_owned()));
        assert_eq!(normalize_verbose_level(Some("FULL")), Some("full".to_owned()));
    }

    #[test]
    fn normalize_verbose_level_invalid() {
        assert_eq!(normalize_verbose_level(Some("partial")), None);
        assert_eq!(normalize_verbose_level(None), None);
    }

    #[test]
    fn session_freshness_fresh() {
        let now = 10_000_000;
        let updated_at = now - 1000; // 1 second ago
        assert!(evaluate_session_freshness(updated_at, now, DEFAULT_FRESHNESS_MS));
    }

    #[test]
    fn session_freshness_stale() {
        let now = 10_000_000;
        let updated_at = now - DEFAULT_FRESHNESS_MS - 1;
        assert!(!evaluate_session_freshness(updated_at, now, DEFAULT_FRESHNESS_MS));
    }

    #[test]
    fn session_freshness_zero() {
        assert!(!evaluate_session_freshness(0, 10_000_000, DEFAULT_FRESHNESS_MS));
    }

    #[test]
    fn resolve_session_key_with_agent_id() {
        let cfg = OpenAcosmiConfig::default();
        let res = resolve_session_key_for_request(
            &cfg,
            None,
            None,
            None,
            Some("mybot"),
        );
        assert_eq!(res.session_key, Some("agent:mybot:main".to_owned()));
    }

    #[test]
    fn resolve_session_key_explicit() {
        let cfg = OpenAcosmiConfig::default();
        let res = resolve_session_key_for_request(
            &cfg,
            None,
            None,
            Some("agent:custom:work"),
            None,
        );
        assert_eq!(res.session_key, Some("agent:custom:work".to_owned()));
    }

    #[test]
    fn resolve_session_new_produces_uuid() {
        let cfg = OpenAcosmiConfig::default();
        let res = resolve_session(
            &cfg,
            Some("+15551234567"),
            None,
            None,
            Some("mybot"),
        );
        assert!(!res.session_id.is_empty());
        assert!(res.is_new_session);
        // UUID v4 format: 36 chars with hyphens.
        assert_eq!(res.session_id.len(), 36);
    }

    #[test]
    fn resolve_session_with_explicit_session_id() {
        let cfg = OpenAcosmiConfig::default();
        let res = resolve_session(
            &cfg,
            None,
            Some("my-session-123"),
            None,
            Some("mybot"),
        );
        assert_eq!(res.session_id, "my-session-123");
        assert!(!res.is_new_session);
    }

    #[test]
    fn explicit_agent_session_key_builds_correctly() {
        let cfg = OpenAcosmiConfig::default();
        let key = resolve_explicit_agent_session_key(&cfg, Some("mybot"));
        assert_eq!(key, Some("agent:mybot:main".to_owned()));
    }

    #[test]
    fn explicit_agent_session_key_none_without_agent() {
        let cfg = OpenAcosmiConfig::default();
        let key = resolve_explicit_agent_session_key(&cfg, None);
        assert!(key.is_none());
    }
}
