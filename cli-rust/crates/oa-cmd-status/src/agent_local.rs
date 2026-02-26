/// Agent local status resolution.
///
/// Resolves per-agent statuses including workspace dir, bootstrap state,
/// session count, and last active time.
///
/// Source: `src/commands/status.agent-local.ts`

use std::collections::HashMap;
use std::path::{Path, PathBuf};

use serde::{Deserialize, Serialize};

use oa_config::sessions::paths::resolve_store_path;
use oa_config::sessions::store::load_session_store;
use oa_types::config::OpenAcosmiConfig;

/// Local status of a single agent.
///
/// Source: `src/commands/status.agent-local.ts` - `AgentLocalStatus`
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AgentLocalStatus {
    /// Agent identifier.
    pub id: String,
    /// Agent display name.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    /// Workspace directory path.
    pub workspace_dir: Option<String>,
    /// Whether a BOOTSTRAP.md file exists.
    pub bootstrap_pending: Option<bool>,
    /// Path to the session store file.
    pub sessions_path: String,
    /// Number of active sessions.
    pub sessions_count: usize,
    /// Timestamp of last session update.
    pub last_updated_at: Option<u64>,
    /// Milliseconds since last session activity.
    pub last_active_age_ms: Option<u64>,
}

/// Aggregate agent status result.
///
/// Source: `src/commands/status.agent-local.ts` - return type
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AgentStatusResult {
    /// Default agent ID.
    pub default_id: String,
    /// Per-agent statuses.
    pub agents: Vec<AgentLocalStatus>,
    /// Total session count across all agents.
    pub total_sessions: usize,
    /// Number of agents with pending bootstrap.
    pub bootstrap_pending_count: usize,
}

/// Minimal agent descriptor for iteration.
///
/// Source: `src/commands/status.agent-local.ts` - agent list
#[derive(Debug, Clone)]
pub struct AgentDescriptor {
    /// Agent identifier.
    pub id: String,
    /// Optional display name.
    pub name: Option<String>,
}

/// List agents from configuration.
///
/// Returns a default agent if none are configured.
///
/// Source: `src/gateway/session-utils.ts` - `listAgentsForGateway`
#[must_use]
pub fn list_agents_for_config(cfg: &OpenAcosmiConfig) -> (String, Vec<AgentDescriptor>) {
    let agent_list = cfg
        .agents
        .as_ref()
        .and_then(|a| a.list.as_deref())
        .unwrap_or(&[]);

    if agent_list.is_empty() {
        return (
            "main".to_string(),
            vec![AgentDescriptor {
                id: "main".to_string(),
                name: None,
            }],
        );
    }

    // Find the default agent ID.
    let default_id = agent_list
        .iter()
        .find(|a| a.default == Some(true))
        .map_or("main".to_string(), |a| a.id.clone());

    let mut agents: Vec<AgentDescriptor> = agent_list
        .iter()
        .filter(|a| !a.id.trim().is_empty())
        .map(|a| AgentDescriptor {
            id: a.id.clone(),
            name: a.name.clone(),
        })
        .collect();

    // Ensure default agent is present.
    if !agents.iter().any(|a| a.id == default_id) {
        agents.insert(
            0,
            AgentDescriptor {
                id: default_id.clone(),
                name: None,
            },
        );
    }

    // Deduplicate by ID.
    let mut seen = std::collections::HashSet::new();
    agents.retain(|a| seen.insert(a.id.clone()));

    (default_id, agents)
}

/// Resolve the workspace directory for an agent.
///
/// Source: `src/agents/agent-scope.ts` - `resolveAgentWorkspaceDir`
#[must_use]
pub fn resolve_agent_workspace_dir(cfg: &OpenAcosmiConfig, agent_id: &str) -> Option<String> {
    let agent_list = cfg
        .agents
        .as_ref()
        .and_then(|a| a.list.as_deref())
        .unwrap_or(&[]);
    for agent in agent_list {
        if agent.id.trim() == agent_id {
            if let Some(ref ws) = agent.workspace {
                let trimmed = ws.trim();
                if !trimmed.is_empty() {
                    return Some(trimmed.to_string());
                }
            }
        }
    }
    // Also check the defaults workspace.
    cfg.agents
        .as_ref()
        .and_then(|a| a.defaults.as_ref())
        .and_then(|d| d.workspace.as_deref())
        .map(str::trim)
        .filter(|s| !s.is_empty())
        .map(String::from)
}

/// Check if a file exists on disk.
fn file_exists(path: &Path) -> bool {
    path.exists()
}

/// Get local statuses for all configured agents.
///
/// Source: `src/commands/status.agent-local.ts` - `getAgentLocalStatuses`
#[must_use]
pub fn get_agent_local_statuses(cfg: &OpenAcosmiConfig) -> AgentStatusResult {
    let (default_id, agents) = list_agents_for_config(cfg);
    let now = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis() as u64;

    let session_store_template = cfg
        .session
        .as_ref()
        .and_then(|s| s.store.as_deref());

    let mut statuses: Vec<AgentLocalStatus> = Vec::new();

    for agent in &agents {
        let workspace_dir = resolve_agent_workspace_dir(cfg, &agent.id);

        let bootstrap_pending = workspace_dir.as_ref().map(|ws| {
            let bootstrap_path = PathBuf::from(ws).join("BOOTSTRAP.md");
            file_exists(&bootstrap_path)
        });

        let store_path = resolve_store_path(session_store_template, Some(&agent.id));
        let sessions_path_str = store_path.display().to_string();

        let store: HashMap<String, oa_types::session::SessionEntry> =
            load_session_store(&store_path);
        let sessions: Vec<_> = store
            .iter()
            .filter(|(key, _)| key.as_str() != "global" && key.as_str() != "unknown")
            .collect();
        let sessions_count = sessions.len();
        let last_updated_at = sessions
            .iter()
            .map(|(_, entry)| entry.updated_at)
            .max()
            .filter(|&ts| ts > 0);

        let last_active_age_ms = last_updated_at.map(|ts| now.saturating_sub(ts));

        statuses.push(AgentLocalStatus {
            id: agent.id.clone(),
            name: agent.name.clone(),
            workspace_dir,
            bootstrap_pending,
            sessions_path: sessions_path_str,
            sessions_count,
            last_updated_at,
            last_active_age_ms,
        });
    }

    let total_sessions: usize = statuses.iter().map(|s| s.sessions_count).sum();
    let bootstrap_pending_count = statuses
        .iter()
        .filter(|s| s.bootstrap_pending == Some(true))
        .count();

    AgentStatusResult {
        default_id,
        agents: statuses,
        total_sessions,
        bootstrap_pending_count,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use oa_types::agents::{AgentConfig, AgentsConfig};

    #[test]
    fn list_agents_default_when_no_agents() {
        let cfg = OpenAcosmiConfig::default();
        let (default_id, agents) = list_agents_for_config(&cfg);
        assert_eq!(default_id, "main");
        assert_eq!(agents.len(), 1);
        assert_eq!(agents[0].id, "main");
    }

    #[test]
    fn list_agents_from_agent_list() {
        let cfg = OpenAcosmiConfig {
            agents: Some(AgentsConfig {
                list: Some(vec![
                    AgentConfig {
                        id: "agent1".to_string(),
                        name: Some("Agent One".to_string()),
                        default: Some(true),
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
                    },
                    AgentConfig {
                        id: "agent2".to_string(),
                        name: None,
                        default: None,
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
                    },
                ]),
                ..Default::default()
            }),
            ..Default::default()
        };
        let (default_id, agents) = list_agents_for_config(&cfg);
        assert_eq!(default_id, "agent1");
        assert_eq!(agents.len(), 2);
    }

    #[test]
    fn get_statuses_empty_config() {
        let cfg = OpenAcosmiConfig::default();
        let result = get_agent_local_statuses(&cfg);
        assert_eq!(result.default_id, "main");
        assert_eq!(result.agents.len(), 1);
        assert_eq!(result.agents[0].id, "main");
    }

    #[test]
    fn resolve_workspace_dir_not_found() {
        let cfg = OpenAcosmiConfig::default();
        assert!(resolve_agent_workspace_dir(&cfg, "main").is_none());
    }

    #[test]
    fn agent_local_status_serializes() {
        let status = AgentLocalStatus {
            id: "main".to_string(),
            name: None,
            workspace_dir: Some("/tmp/ws".to_string()),
            bootstrap_pending: Some(false),
            sessions_path: "/tmp/sessions.json".to_string(),
            sessions_count: 5,
            last_updated_at: Some(1_000_000),
            last_active_age_ms: Some(500),
        };
        let json = serde_json::to_value(&status).unwrap_or_default();
        assert_eq!(json["sessionsCount"], 5);
        assert_eq!(json["bootstrapPending"], false);
        assert_eq!(json["lastActiveAgeMs"], 500);
    }
}
