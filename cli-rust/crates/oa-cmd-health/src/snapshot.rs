/// Health snapshot building: agent ordering and session summaries.
///
/// Provides local (non-gateway) logic for resolving agent order and building
/// session store summaries from disk.
///
/// Source: `src/commands/health.ts` - `resolveAgentOrder`, `buildSessionSummary`

use std::collections::HashSet;
use std::path::Path;

use oa_agents::scope::resolve_default_agent_id;
use oa_config::sessions::store::load_session_store;
use oa_routing::session_key::normalize_agent_id;
use oa_types::config::OpenAcosmiConfig;

use crate::types::{RecentSession, SessionSummary};

/// An ordered agent entry for display.
///
/// Source: `src/commands/health.ts` - `resolveAgentOrder` return
#[derive(Debug, Clone)]
pub struct AgentEntry {
    /// Normalized agent identifier.
    pub id: String,
    /// Optional human-readable name.
    pub name: Option<String>,
}

/// Result of resolving agent display order.
///
/// Source: `src/commands/health.ts` - `resolveAgentOrder` return
#[derive(Debug, Clone)]
pub struct AgentOrder {
    /// The default agent identifier.
    pub default_agent_id: String,
    /// Agents in display order.
    pub ordered: Vec<AgentEntry>,
}

/// Resolve the ordered list of agents from the configuration.
///
/// Ensures the default agent is always present and listed first if not
/// already in the list. Deduplicates by normalized ID.
///
/// Source: `src/commands/health.ts` - `resolveAgentOrder`
pub fn resolve_agent_order(cfg: &OpenAcosmiConfig) -> AgentOrder {
    let default_agent_id = resolve_default_agent_id(cfg);
    let entries = cfg
        .agents
        .as_ref()
        .and_then(|a| a.list.as_ref())
        .cloned()
        .unwrap_or_default();

    let mut seen = HashSet::new();
    let mut ordered = Vec::new();

    for entry in &entries {
        let id_trimmed = entry.id.trim();
        if id_trimmed.is_empty() {
            continue;
        }
        let id = normalize_agent_id(Some(id_trimmed));
        if id.is_empty() || seen.contains(&id) {
            continue;
        }
        seen.insert(id.clone());
        ordered.push(AgentEntry {
            id,
            name: entry.name.clone(),
        });
    }

    if !seen.contains(&default_agent_id) {
        ordered.insert(
            0,
            AgentEntry {
                id: default_agent_id.clone(),
                name: None,
            },
        );
    }

    if ordered.is_empty() {
        ordered.push(AgentEntry {
            id: default_agent_id.clone(),
            name: None,
        });
    }

    AgentOrder {
        default_agent_id,
        ordered,
    }
}

/// Build a session summary from the session store at the given path.
///
/// Reads the session store, filters out internal keys ("global", "unknown"),
/// sorts by most recently updated, and returns the top 5.
///
/// Source: `src/commands/health.ts` - `buildSessionSummary`
pub fn build_session_summary(store_path: &Path) -> SessionSummary {
    let store = load_session_store(store_path);
    let now = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map_or(0, |d| d.as_millis() as u64);

    let mut sessions: Vec<(String, u64)> = store
        .iter()
        .filter(|(key, _)| key.as_str() != "global" && key.as_str() != "unknown")
        .map(|(key, entry)| (key.clone(), entry.updated_at))
        .collect();

    sessions.sort_by(|a, b| b.1.cmp(&a.1));

    let recent: Vec<RecentSession> = sessions
        .iter()
        .take(5)
        .map(|(key, updated_at)| {
            let ts = *updated_at;
            RecentSession {
                key: key.clone(),
                updated_at: if ts > 0 { Some(ts) } else { None },
                age: if ts > 0 {
                    Some(now.saturating_sub(ts))
                } else {
                    None
                },
            }
        })
        .collect();

    SessionSummary {
        path: store_path.display().to_string(),
        count: sessions.len(),
        recent,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use oa_types::agents::{AgentConfig, AgentsConfig};

    fn make_agent(id: &str, name: Option<&str>) -> AgentConfig {
        AgentConfig {
            id: id.to_owned(),
            default: None,
            name: name.map(str::to_owned),
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

    #[test]
    fn resolve_agent_order_empty_config() {
        let cfg = OpenAcosmiConfig::default();
        let order = resolve_agent_order(&cfg);
        assert_eq!(order.default_agent_id, "main");
        assert_eq!(order.ordered.len(), 1);
        assert_eq!(order.ordered[0].id, "main");
    }

    #[test]
    fn resolve_agent_order_with_agents() {
        let cfg = OpenAcosmiConfig {
            agents: Some(AgentsConfig {
                defaults: None,
                list: Some(vec![
                    make_agent("alpha", Some("Alpha Bot")),
                    make_agent("beta", None),
                ]),
            }),
            ..Default::default()
        };
        let order = resolve_agent_order(&cfg);
        assert_eq!(order.ordered.len(), 2);
        assert_eq!(order.ordered[0].id, "alpha");
        assert_eq!(order.ordered[0].name.as_deref(), Some("Alpha Bot"));
        assert_eq!(order.ordered[1].id, "beta");
    }

    #[test]
    fn resolve_agent_order_deduplicates() {
        let cfg = OpenAcosmiConfig {
            agents: Some(AgentsConfig {
                defaults: None,
                list: Some(vec![
                    make_agent("alpha", None),
                    make_agent("Alpha", None),
                    make_agent("beta", None),
                ]),
            }),
            ..Default::default()
        };
        let order = resolve_agent_order(&cfg);
        assert_eq!(order.ordered.len(), 2);
    }

    #[test]
    fn resolve_agent_order_prepends_default() {
        let cfg = OpenAcosmiConfig {
            agents: Some(AgentsConfig {
                defaults: None,
                list: Some(vec![make_agent("beta", None)]),
            }),
            ..Default::default()
        };
        let order = resolve_agent_order(&cfg);
        // default is "beta" (first in list), so no prepend needed
        assert_eq!(order.default_agent_id, "beta");
        assert_eq!(order.ordered.len(), 1);
    }

    #[test]
    fn build_session_summary_nonexistent_path() {
        let path = std::path::PathBuf::from("/tmp/nonexistent-sessions-12345.json");
        let summary = build_session_summary(&path);
        assert_eq!(summary.count, 0);
        assert!(summary.recent.is_empty());
    }
}
