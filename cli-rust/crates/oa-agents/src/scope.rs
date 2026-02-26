/// Agent scope resolution: listing agents, resolving defaults, and looking up
/// per-agent configuration from the merged [`OpenAcosmiConfig`].
///
/// Source: `src/agents/agent-scope.ts`

use std::collections::HashSet;
use std::path::{Path, PathBuf};

use oa_config::paths::resolve_state_dir;
use oa_routing::session_key::{normalize_agent_id, DEFAULT_AGENT_ID};
use oa_types::agents::{AgentConfig, AgentModelConfig};
use oa_types::config::OpenAcosmiConfig;

use std::sync::atomic::{AtomicBool, Ordering};

/// Tracks whether we have already warned about multiple default agents.
static DEFAULT_AGENT_WARNED: AtomicBool = AtomicBool::new(false);

// ── Internal helpers ──

/// Extract the list of agent entries from the config, filtering out invalid items.
///
/// Source: `src/agents/agent-scope.ts` - `listAgents`
fn list_agents(cfg: &OpenAcosmiConfig) -> Vec<&AgentConfig> {
    let agents_config = match &cfg.agents {
        Some(ac) => ac,
        None => return Vec::new(),
    };
    match &agents_config.list {
        Some(list) => list.iter().collect(),
        None => Vec::new(),
    }
}

// ── Public API ──

/// List all configured agent IDs, deduplicating and normalizing each one.
///
/// If no agents are configured, returns a single-element list containing
/// [`DEFAULT_AGENT_ID`] (`"main"`).
///
/// Source: `src/agents/agent-scope.ts` - `listAgentIds`
pub fn list_agent_ids(cfg: &OpenAcosmiConfig) -> Vec<String> {
    let agents = list_agents(cfg);
    if agents.is_empty() {
        return vec![DEFAULT_AGENT_ID.to_owned()];
    }

    let mut seen = HashSet::new();
    let mut ids = Vec::new();
    for entry in &agents {
        let id = normalize_agent_id(Some(&entry.id));
        if seen.contains(&id) {
            continue;
        }
        seen.insert(id.clone());
        ids.push(id);
    }
    if ids.is_empty() {
        vec![DEFAULT_AGENT_ID.to_owned()]
    } else {
        ids
    }
}

/// Resolve the default agent ID from the configuration.
///
/// Picks the first agent marked `default: true`. If none (or no agents at all),
/// falls back to the first agent's ID, or [`DEFAULT_AGENT_ID`].
///
/// Warns once (via `tracing`) if multiple agents are marked as default.
///
/// Source: `src/agents/agent-scope.ts` - `resolveDefaultAgentId`
pub fn resolve_default_agent_id(cfg: &OpenAcosmiConfig) -> String {
    let agents = list_agents(cfg);
    if agents.is_empty() {
        return DEFAULT_AGENT_ID.to_owned();
    }

    let defaults: Vec<&&AgentConfig> = agents
        .iter()
        .filter(|a| a.default.unwrap_or(false))
        .collect();

    if defaults.len() > 1 && !DEFAULT_AGENT_WARNED.swap(true, Ordering::Relaxed) {
        tracing::warn!(
            "Multiple agents marked default=true; using the first entry as default."
        );
    }

    let chosen = defaults
        .first()
        .copied()
        .or(agents.first())
        .map(|a| a.id.trim())
        .unwrap_or(DEFAULT_AGENT_ID);

    if chosen.is_empty() {
        normalize_agent_id(Some(DEFAULT_AGENT_ID))
    } else {
        normalize_agent_id(Some(chosen))
    }
}

/// Resolve the agent entry for a given agent ID from the configuration list.
///
/// Source: `src/agents/agent-scope.ts` - `resolveAgentEntry`
fn resolve_agent_entry<'a>(cfg: &'a OpenAcosmiConfig, agent_id: &str) -> Option<&'a AgentConfig> {
    let id = normalize_agent_id(Some(agent_id));
    list_agents(cfg)
        .into_iter()
        .find(|entry| normalize_agent_id(Some(&entry.id)) == id)
}

/// Resolve the full agent configuration for a given agent ID.
///
/// Returns `None` if the agent is not found in the config.
///
/// Source: `src/agents/agent-scope.ts` - `resolveAgentConfig`
pub fn resolve_agent_config<'a>(
    cfg: &'a OpenAcosmiConfig,
    agent_id: &str,
) -> Option<&'a AgentConfig> {
    let id = normalize_agent_id(Some(agent_id));
    resolve_agent_entry(cfg, &id)
}

/// Resolve the skills filter list for a given agent.
///
/// Returns `None` if no skills are configured. Returns `Some(vec![])` if
/// an empty skills array is explicitly configured (meaning "no skills").
///
/// Source: `src/agents/agent-scope.ts` - `resolveAgentSkillsFilter`
pub fn resolve_agent_skills_filter(
    cfg: &OpenAcosmiConfig,
    agent_id: &str,
) -> Option<Vec<String>> {
    let entry = resolve_agent_config(cfg, agent_id)?;
    let raw = entry.skills.as_ref()?;

    let normalized: Vec<String> = raw
        .iter()
        .map(|s| s.trim().to_owned())
        .filter(|s| !s.is_empty())
        .collect();

    if normalized.is_empty() {
        Some(Vec::new())
    } else {
        Some(normalized)
    }
}

/// Resolve the primary model identifier for an agent.
///
/// The model can be specified as a simple string or as an object with a
/// `primary` field. Returns `None` if no model is configured.
///
/// Source: `src/agents/agent-scope.ts` - `resolveAgentModelPrimary`
pub fn resolve_agent_model_primary(cfg: &OpenAcosmiConfig, agent_id: &str) -> Option<String> {
    let entry = resolve_agent_config(cfg, agent_id)?;
    let model_cfg = entry.model.as_ref()?;
    match model_cfg {
        AgentModelConfig::String(s) => {
            let trimmed = s.trim();
            if trimmed.is_empty() {
                None
            } else {
                Some(trimmed.to_owned())
            }
        }
        AgentModelConfig::Object { primary, .. } => {
            let p = primary.as_deref()?.trim();
            if p.is_empty() {
                None
            } else {
                Some(p.to_owned())
            }
        }
    }
}

/// Resolve the model fallback overrides for an agent.
///
/// Returns `None` if no fallbacks key exists in the agent model config.
/// Returns `Some(vec![])` if fallbacks is explicitly set to an empty array
/// (disabling global fallbacks for this agent).
///
/// Source: `src/agents/agent-scope.ts` - `resolveAgentModelFallbacksOverride`
pub fn resolve_agent_model_fallbacks_override(
    cfg: &OpenAcosmiConfig,
    agent_id: &str,
) -> Option<Vec<String>> {
    let entry = resolve_agent_config(cfg, agent_id)?;
    let model_cfg = entry.model.as_ref()?;
    match model_cfg {
        AgentModelConfig::String(_) => None,
        AgentModelConfig::Object { fallbacks, .. } => fallbacks.clone(),
    }
}

/// Resolve the workspace directory for an agent.
///
/// Precedence:
/// 1. Agent-specific `workspace` field
/// 2. For the default agent: `agents.defaults.workspace`
/// 3. For the default agent: the state directory
/// 4. For non-default agents: `<state_dir>/workspace-<agent_id>`
///
/// Source: `src/agents/agent-scope.ts` - `resolveAgentWorkspaceDir`
pub fn resolve_agent_workspace_dir(cfg: &OpenAcosmiConfig, agent_id: &str) -> PathBuf {
    let id = normalize_agent_id(Some(agent_id));

    // Check agent-specific workspace
    if let Some(entry) = resolve_agent_config(cfg, &id) {
        if let Some(ref ws) = entry.workspace {
            let trimmed = ws.trim();
            if !trimmed.is_empty() {
                return resolve_user_path(trimmed);
            }
        }
    }

    let default_agent_id = resolve_default_agent_id(cfg);
    if id == default_agent_id {
        // Check agents.defaults.workspace
        if let Some(ref agents) = cfg.agents {
            if let Some(ref defaults) = agents.defaults {
                if let Some(ref ws) = defaults.workspace {
                    let trimmed = ws.trim();
                    if !trimmed.is_empty() {
                        return resolve_user_path(trimmed);
                    }
                }
            }
        }
        // Fallback: use the state directory as workspace for the default agent
        return resolve_state_dir();
    }

    // Non-default agents get a workspace under the state dir
    let state_dir = resolve_state_dir();
    state_dir.join(format!("workspace-{id}"))
}

/// Resolve the agent data directory for persisting agent-specific state.
///
/// Checks for an agent-specific `agentDir` override first, then falls back
/// to `<state_dir>/agents/<agent_id>/agent`.
///
/// Source: `src/agents/agent-scope.ts` - `resolveAgentDir`
pub fn resolve_agent_dir(cfg: &OpenAcosmiConfig, agent_id: &str) -> PathBuf {
    let id = normalize_agent_id(Some(agent_id));

    if let Some(entry) = resolve_agent_config(cfg, &id) {
        if let Some(ref dir) = entry.agent_dir {
            let trimmed = dir.trim();
            if !trimmed.is_empty() {
                return resolve_user_path(trimmed);
            }
        }
    }

    let root = resolve_state_dir();
    root.join("agents").join(&id).join("agent")
}

/// Resolve the agent data directory from an explicit state directory path.
///
/// This variant does not read from config; it constructs the canonical path
/// `<state_dir>/agents/<agent_id>/agent` directly.
///
/// Source: `src/agents/agent-scope.ts` - `resolveAgentDir` (simplified)
pub fn resolve_agent_dir_from_state(state_dir: &Path, agent_id: &str) -> PathBuf {
    let id = normalize_agent_id(Some(agent_id));
    state_dir.join("agents").join(&id).join("agent")
}

/// Resolve session agent IDs: the default agent and the session-specific agent.
///
/// If a session key is provided and contains an agent prefix, extracts the
/// agent ID from it; otherwise uses the default.
///
/// Source: `src/agents/agent-scope.ts` - `resolveSessionAgentIds`
pub fn resolve_session_agent_ids(
    session_key: Option<&str>,
    cfg: &OpenAcosmiConfig,
) -> (String, String) {
    let default_agent_id = resolve_default_agent_id(cfg);
    let session_key = session_key.unwrap_or("").trim().to_lowercase();

    if session_key.is_empty() {
        return (default_agent_id.clone(), default_agent_id);
    }

    let parsed = oa_routing::session_key::parse_agent_session_key(Some(&session_key));
    let session_agent_id = parsed
        .map(|p| normalize_agent_id(Some(&p.agent_id)))
        .unwrap_or_else(|| default_agent_id.clone());

    (default_agent_id, session_agent_id)
}

/// Resolve the session agent ID (just the session-specific one).
///
/// Source: `src/agents/agent-scope.ts` - `resolveSessionAgentId`
pub fn resolve_session_agent_id(session_key: Option<&str>, cfg: &OpenAcosmiConfig) -> String {
    resolve_session_agent_ids(session_key, cfg).1
}

// ── Internal helper ──

/// Expand a leading `~` in a path to the user's home directory.
fn resolve_user_path(input: &str) -> PathBuf {
    let trimmed = input.trim();
    if trimmed.is_empty() {
        return PathBuf::from(trimmed);
    }
    if let Some(rest) = trimmed.strip_prefix('~') {
        let home = dirs::home_dir().unwrap_or_else(|| PathBuf::from("."));
        if rest.is_empty() {
            return home;
        }
        let rest = rest.strip_prefix('/').unwrap_or(rest);
        return home.join(rest);
    }
    PathBuf::from(trimmed)
}

/// Reset the default-agent warning flag (for testing only).
#[cfg(test)]
pub fn reset_default_agent_warned() {
    DEFAULT_AGENT_WARNED.store(false, Ordering::Relaxed);
}

#[cfg(test)]
mod tests {
    use super::*;
    use oa_types::agents::{AgentConfig, AgentsConfig};

    fn make_agent(id: &str, default: Option<bool>) -> AgentConfig {
        AgentConfig {
            id: id.to_owned(),
            default,
            name: None,
            workspace: None,
            agent_dir: None,
            model: None,
            skills: None,
            memory_search: None,
            human_delay: None,
            heartbeat: None,
            identity: None,
            group_chat: None,
            subagents: None,
            sandbox: None,
            tools: None,
        }
    }

    fn make_config(agents: Vec<AgentConfig>) -> OpenAcosmiConfig {
        OpenAcosmiConfig {
            agents: Some(AgentsConfig {
                defaults: None,
                list: Some(agents),
            }),
            ..Default::default()
        }
    }

    // ── list_agent_ids ──

    #[test]
    fn list_agent_ids_empty_config() {
        let cfg = OpenAcosmiConfig::default();
        assert_eq!(list_agent_ids(&cfg), vec!["main"]);
    }

    #[test]
    fn list_agent_ids_with_agents() {
        let cfg = make_config(vec![make_agent("alpha", None), make_agent("beta", None)]);
        assert_eq!(list_agent_ids(&cfg), vec!["alpha", "beta"]);
    }

    #[test]
    fn list_agent_ids_deduplicates() {
        let cfg = make_config(vec![
            make_agent("alpha", None),
            make_agent("Alpha", None),
            make_agent("beta", None),
        ]);
        assert_eq!(list_agent_ids(&cfg), vec!["alpha", "beta"]);
    }

    // ── resolve_default_agent_id ──

    #[test]
    fn default_agent_id_empty_config() {
        let cfg = OpenAcosmiConfig::default();
        assert_eq!(resolve_default_agent_id(&cfg), "main");
    }

    #[test]
    fn default_agent_id_first_when_no_default() {
        reset_default_agent_warned();
        let cfg = make_config(vec![make_agent("alpha", None), make_agent("beta", None)]);
        assert_eq!(resolve_default_agent_id(&cfg), "alpha");
    }

    #[test]
    fn default_agent_id_picks_default_flag() {
        reset_default_agent_warned();
        let cfg = make_config(vec![
            make_agent("alpha", None),
            make_agent("beta", Some(true)),
        ]);
        assert_eq!(resolve_default_agent_id(&cfg), "beta");
    }

    // ── resolve_agent_config ──

    #[test]
    fn resolve_agent_config_found() {
        let cfg = make_config(vec![make_agent("alpha", None)]);
        let result = resolve_agent_config(&cfg, "alpha");
        assert!(result.is_some());
        assert_eq!(result.map(|a| a.id.as_str()), Some("alpha"));
    }

    #[test]
    fn resolve_agent_config_case_insensitive() {
        let cfg = make_config(vec![make_agent("Alpha", None)]);
        let result = resolve_agent_config(&cfg, "alpha");
        assert!(result.is_some());
    }

    #[test]
    fn resolve_agent_config_not_found() {
        let cfg = make_config(vec![make_agent("alpha", None)]);
        assert!(resolve_agent_config(&cfg, "nonexistent").is_none());
    }

    // ── resolve_agent_skills_filter ──

    #[test]
    fn skills_filter_none_when_no_skills() {
        let cfg = make_config(vec![make_agent("alpha", None)]);
        assert!(resolve_agent_skills_filter(&cfg, "alpha").is_none());
    }

    #[test]
    fn skills_filter_returns_skills() {
        let mut agent = make_agent("alpha", None);
        agent.skills = Some(vec!["skill-a".to_owned(), "skill-b".to_owned()]);
        let cfg = make_config(vec![agent]);
        let skills = resolve_agent_skills_filter(&cfg, "alpha");
        assert_eq!(skills, Some(vec!["skill-a".to_owned(), "skill-b".to_owned()]));
    }

    #[test]
    fn skills_filter_empty_array() {
        let mut agent = make_agent("alpha", None);
        agent.skills = Some(Vec::new());
        let cfg = make_config(vec![agent]);
        let skills = resolve_agent_skills_filter(&cfg, "alpha");
        assert_eq!(skills, Some(Vec::new()));
    }

    // ── resolve_agent_model_primary ──

    #[test]
    fn model_primary_string() {
        let mut agent = make_agent("alpha", None);
        agent.model = Some(AgentModelConfig::String("anthropic/claude-opus-4-6".to_owned()));
        let cfg = make_config(vec![agent]);
        assert_eq!(
            resolve_agent_model_primary(&cfg, "alpha"),
            Some("anthropic/claude-opus-4-6".to_owned())
        );
    }

    #[test]
    fn model_primary_object() {
        let mut agent = make_agent("alpha", None);
        agent.model = Some(AgentModelConfig::Object {
            primary: Some("openai/gpt-4o".to_owned()),
            fallbacks: None,
        });
        let cfg = make_config(vec![agent]);
        assert_eq!(
            resolve_agent_model_primary(&cfg, "alpha"),
            Some("openai/gpt-4o".to_owned())
        );
    }

    #[test]
    fn model_primary_none() {
        let cfg = make_config(vec![make_agent("alpha", None)]);
        assert!(resolve_agent_model_primary(&cfg, "alpha").is_none());
    }

    #[test]
    fn model_primary_empty_string() {
        let mut agent = make_agent("alpha", None);
        agent.model = Some(AgentModelConfig::String("  ".to_owned()));
        let cfg = make_config(vec![agent]);
        assert!(resolve_agent_model_primary(&cfg, "alpha").is_none());
    }

    // ── resolve_agent_model_fallbacks_override ──

    #[test]
    fn model_fallbacks_string_returns_none() {
        let mut agent = make_agent("alpha", None);
        agent.model = Some(AgentModelConfig::String("some-model".to_owned()));
        let cfg = make_config(vec![agent]);
        assert!(resolve_agent_model_fallbacks_override(&cfg, "alpha").is_none());
    }

    #[test]
    fn model_fallbacks_object_with_fallbacks() {
        let mut agent = make_agent("alpha", None);
        agent.model = Some(AgentModelConfig::Object {
            primary: Some("primary-model".to_owned()),
            fallbacks: Some(vec!["fb1".to_owned(), "fb2".to_owned()]),
        });
        let cfg = make_config(vec![agent]);
        assert_eq!(
            resolve_agent_model_fallbacks_override(&cfg, "alpha"),
            Some(vec!["fb1".to_owned(), "fb2".to_owned()])
        );
    }

    #[test]
    fn model_fallbacks_explicit_empty() {
        let mut agent = make_agent("alpha", None);
        agent.model = Some(AgentModelConfig::Object {
            primary: Some("primary-model".to_owned()),
            fallbacks: Some(Vec::new()),
        });
        let cfg = make_config(vec![agent]);
        assert_eq!(
            resolve_agent_model_fallbacks_override(&cfg, "alpha"),
            Some(Vec::new())
        );
    }

    // ── resolve_agent_dir_from_state ──

    #[test]
    fn agent_dir_from_state() {
        let state = Path::new("/tmp/test-state");
        let result = resolve_agent_dir_from_state(state, "alpha");
        assert_eq!(result, PathBuf::from("/tmp/test-state/agents/alpha/agent"));
    }

    #[test]
    fn agent_dir_from_state_normalizes_id() {
        let state = Path::new("/tmp/test-state");
        let result = resolve_agent_dir_from_state(state, "My Bot!");
        assert_eq!(
            result,
            PathBuf::from("/tmp/test-state/agents/my-bot/agent")
        );
    }

    // ── resolve_session_agent_ids ──

    #[test]
    fn session_ids_no_key() {
        let cfg = make_config(vec![make_agent("alpha", None)]);
        let (default_id, session_id) = resolve_session_agent_ids(None, &cfg);
        assert_eq!(default_id, "alpha");
        assert_eq!(session_id, "alpha");
    }

    #[test]
    fn session_ids_with_agent_key() {
        let cfg = make_config(vec![make_agent("alpha", None), make_agent("beta", None)]);
        let (default_id, session_id) =
            resolve_session_agent_ids(Some("agent:beta:main"), &cfg);
        assert_eq!(default_id, "alpha");
        assert_eq!(session_id, "beta");
    }

    #[test]
    fn session_ids_with_non_agent_key() {
        let cfg = make_config(vec![make_agent("alpha", None)]);
        let (default_id, session_id) =
            resolve_session_agent_ids(Some("some-legacy-key"), &cfg);
        assert_eq!(default_id, "alpha");
        assert_eq!(session_id, "alpha");
    }
}
