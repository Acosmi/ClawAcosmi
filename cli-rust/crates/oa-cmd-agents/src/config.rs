/// Agent configuration utilities: listing, finding, applying, and pruning agent entries.
///
/// Source: `src/commands/agents.config.ts`

use oa_agents::scope::{
    resolve_agent_dir, resolve_agent_model_primary, resolve_agent_workspace_dir,
    resolve_default_agent_id,
};
use oa_routing::session_key::normalize_agent_id;
use oa_types::agents::{AgentConfig, AgentModelConfig};
use oa_types::base::IdentityConfig;
use oa_types::config::OpenAcosmiConfig;
use serde::{Deserialize, Serialize};

/// Summary of an agent for display purposes.
///
/// Source: `src/commands/agents.config.ts` - `AgentSummary`
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AgentSummary {
    /// Agent identifier.
    pub id: String,
    /// Display name, if set.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    /// Identity name from `IDENTITY.md` or config.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub identity_name: Option<String>,
    /// Identity emoji from `IDENTITY.md` or config.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub identity_emoji: Option<String>,
    /// Where the identity was sourced from.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub identity_source: Option<String>,
    /// Workspace directory path.
    pub workspace: String,
    /// Agent data directory path.
    pub agent_dir: String,
    /// Primary model reference.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model: Option<String>,
    /// Number of routing bindings.
    pub bindings: usize,
    /// Binding detail strings.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub binding_details: Option<Vec<String>>,
    /// Routing summary lines.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub routes: Option<Vec<String>>,
    /// Provider status lines.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub providers: Option<Vec<String>>,
    /// Whether this is the default agent.
    pub is_default: bool,
}

/// Extract the list of agent entries from config.
///
/// Source: `src/commands/agents.config.ts` - `listAgentEntries`
#[must_use]
pub fn list_agent_entries(cfg: &OpenAcosmiConfig) -> Vec<&AgentConfig> {
    cfg.agents
        .as_ref()
        .and_then(|a| a.list.as_ref())
        .map(|list| list.iter().collect())
        .unwrap_or_default()
}

/// Find the index of an agent entry in the list by ID.
///
/// Returns `-1` (as `Option::None`) if not found.
///
/// Source: `src/commands/agents.config.ts` - `findAgentEntryIndex`
#[must_use]
pub fn find_agent_entry_index(entries: &[&AgentConfig], agent_id: &str) -> Option<usize> {
    let id = normalize_agent_id(Some(agent_id));
    entries
        .iter()
        .position(|entry| normalize_agent_id(Some(&entry.id)) == id)
}

/// Resolve the display name for an agent.
fn resolve_agent_name(cfg: &OpenAcosmiConfig, agent_id: &str) -> Option<String> {
    let id = normalize_agent_id(Some(agent_id));
    let entries = list_agent_entries(cfg);
    entries
        .iter()
        .find(|entry| normalize_agent_id(Some(&entry.id)) == id)
        .and_then(|entry| entry.name.as_ref())
        .map(|n| n.trim().to_owned())
        .filter(|n| !n.is_empty())
}

/// Resolve the primary model for an agent, checking agent-specific config
/// and falling back to the global default.
fn resolve_agent_model(cfg: &OpenAcosmiConfig, agent_id: &str) -> Option<String> {
    let id = normalize_agent_id(Some(agent_id));

    // Check agent-specific model
    if let Some(model) = resolve_agent_model_primary(cfg, &id) {
        return Some(model);
    }

    // Fall back to global default
    let raw = cfg
        .agents
        .as_ref()
        .and_then(|a| a.defaults.as_ref())
        .and_then(|d| d.model.as_ref());

    raw.and_then(|m| m.primary.as_deref())
        .map(|s| s.trim().to_owned())
        .filter(|s| !s.is_empty())
}

/// Build agent summaries for all configured agents.
///
/// Source: `src/commands/agents.config.ts` - `buildAgentSummaries`
#[must_use]
pub fn build_agent_summaries(cfg: &OpenAcosmiConfig) -> Vec<AgentSummary> {
    let default_agent_id = normalize_agent_id(Some(&resolve_default_agent_id(cfg)));
    let configured_agents = list_agent_entries(cfg);

    let ordered_ids: Vec<String> = if configured_agents.is_empty() {
        vec![default_agent_id.clone()]
    } else {
        configured_agents
            .iter()
            .map(|a| normalize_agent_id(Some(&a.id)))
            .collect()
    };

    // Deduplicate preserving order
    let mut seen = std::collections::HashSet::new();
    let ordered: Vec<String> = ordered_ids
        .into_iter()
        .filter(|id| seen.insert(id.clone()))
        .collect();

    // Count bindings per agent
    let mut binding_counts = std::collections::HashMap::new();
    for binding in cfg.bindings.as_deref().unwrap_or_default() {
        let agent_id = normalize_agent_id(Some(&binding.agent_id));
        *binding_counts.entry(agent_id).or_insert(0usize) += 1;
    }

    ordered
        .iter()
        .map(|id| {
            let workspace = resolve_agent_workspace_dir(cfg, id)
                .display()
                .to_string();
            let agent_dir = resolve_agent_dir(cfg, id).display().to_string();

            // Resolve identity from config (IDENTITY.md resolution would need
            // file I/O; we use config-only identity here).
            let config_identity = configured_agents
                .iter()
                .find(|a| normalize_agent_id(Some(&a.id)) == *id)
                .and_then(|a| a.identity.as_ref());

            let identity_name = config_identity
                .and_then(|i| i.name.as_ref())
                .map(|s| s.trim().to_owned())
                .filter(|s| !s.is_empty());
            let identity_emoji = config_identity
                .and_then(|i| i.emoji.as_ref())
                .map(|s| s.trim().to_owned())
                .filter(|s| !s.is_empty());
            let identity_source = if identity_name.is_some() || identity_emoji.is_some() {
                Some("config".to_owned())
            } else {
                None
            };

            AgentSummary {
                id: id.clone(),
                name: resolve_agent_name(cfg, id),
                identity_name,
                identity_emoji,
                identity_source,
                workspace,
                agent_dir,
                model: resolve_agent_model(cfg, id),
                bindings: binding_counts.get(id).copied().unwrap_or(0),
                binding_details: None,
                routes: None,
                providers: None,
                is_default: *id == default_agent_id,
            }
        })
        .collect()
}

/// Apply agent configuration changes (upsert).
///
/// Source: `src/commands/agents.config.ts` - `applyAgentConfig`
#[must_use]
pub fn apply_agent_config(
    cfg: &OpenAcosmiConfig,
    agent_id: &str,
    name: Option<&str>,
    workspace: Option<&str>,
    agent_dir_override: Option<&str>,
    model: Option<&str>,
) -> OpenAcosmiConfig {
    let id = normalize_agent_id(Some(agent_id));
    let entries = list_agent_entries(cfg);
    let index = find_agent_entry_index(&entries, &id);

    let base = index
        .map(|i| entries[i].clone())
        .unwrap_or_else(|| AgentConfig {
            id: id.clone(),
            default: None,
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
        });

    let next_entry = AgentConfig {
        name: name.map(|n| n.trim().to_owned()).or(base.name),
        workspace: workspace.map(|w| w.to_owned()).or(base.workspace),
        agent_dir: agent_dir_override.map(|d| d.to_owned()).or(base.agent_dir),
        model: model
            .map(|m| AgentModelConfig::String(m.to_owned()))
            .or(base.model),
        ..base
    };

    let mut next_list: Vec<AgentConfig> = entries.iter().map(|e| (*e).clone()).collect();
    if let Some(i) = index {
        next_list[i] = next_entry;
    } else {
        let default_id = normalize_agent_id(Some(&resolve_default_agent_id(cfg)));
        if next_list.is_empty() && id != default_id {
            next_list.push(AgentConfig {
                id: resolve_default_agent_id(cfg),
                default: None,
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
            });
        }
        next_list.push(next_entry);
    }

    OpenAcosmiConfig {
        agents: Some(oa_types::agents::AgentsConfig {
            defaults: cfg.agents.as_ref().and_then(|a| a.defaults.clone()),
            list: Some(next_list),
        }),
        ..cfg.clone()
    }
}

/// Prune an agent from the config, removing its entry, bindings, and
/// agent-to-agent allow entries.
///
/// Source: `src/commands/agents.config.ts` - `pruneAgentConfig`
#[must_use]
pub fn prune_agent_config(cfg: &OpenAcosmiConfig, agent_id: &str) -> PruneResult {
    let id = normalize_agent_id(Some(agent_id));

    // Remove from agents list
    let agents = list_agent_entries(cfg);
    let next_agents_list: Vec<AgentConfig> = agents
        .iter()
        .filter(|entry| normalize_agent_id(Some(&entry.id)) != id)
        .map(|e| (*e).clone())
        .collect();
    let next_agents = if next_agents_list.is_empty() {
        None
    } else {
        Some(next_agents_list)
    };

    // Remove bindings
    let bindings = cfg.bindings.as_deref().unwrap_or_default();
    let filtered_bindings: Vec<_> = bindings
        .iter()
        .filter(|b| normalize_agent_id(Some(&b.agent_id)) != id)
        .cloned()
        .collect();
    let removed_bindings = bindings.len() - filtered_bindings.len();

    // Remove from agent-to-agent allow list
    let allow = cfg
        .tools
        .as_ref()
        .and_then(|t| t.agent_to_agent.as_ref())
        .and_then(|a| a.allow.as_ref())
        .cloned()
        .unwrap_or_default();
    let filtered_allow: Vec<_> = allow.iter().filter(|a| **a != id).cloned().collect();
    let removed_allow = allow.len() - filtered_allow.len();

    let next_agents_config = cfg.agents.as_ref().map(|a| oa_types::agents::AgentsConfig {
        defaults: a.defaults.clone(),
        list: next_agents,
    });

    let mut next_config = cfg.clone();
    next_config.agents = next_agents_config;
    next_config.bindings = if filtered_bindings.is_empty() {
        None
    } else {
        Some(filtered_bindings)
    };

    // Update tools.agentToAgent.allow
    if let Some(ref mut tools) = next_config.tools {
        if let Some(ref mut a2a) = tools.agent_to_agent {
            a2a.allow = if filtered_allow.is_empty() {
                None
            } else {
                Some(filtered_allow)
            };
        }
    }

    PruneResult {
        config: next_config,
        removed_bindings,
        removed_allow,
    }
}

/// Result of pruning an agent from config.
///
/// Source: `src/commands/agents.config.ts` - return type of `pruneAgentConfig`
pub struct PruneResult {
    /// The updated configuration.
    pub config: OpenAcosmiConfig,
    /// Number of bindings removed.
    pub removed_bindings: usize,
    /// Number of allow entries removed.
    pub removed_allow: usize,
}

/// Apply an identity update to a specific agent in the config.
///
/// Source: `src/commands/agents.commands.identity.ts` - inline logic
#[must_use]
pub fn apply_agent_identity(
    cfg: &OpenAcosmiConfig,
    agent_id: &str,
    identity: &IdentityConfig,
) -> OpenAcosmiConfig {
    let id = normalize_agent_id(Some(agent_id));
    let entries = list_agent_entries(cfg);
    let index = find_agent_entry_index(&entries, &id);

    let base = index
        .map(|i| entries[i].clone())
        .unwrap_or_else(|| AgentConfig {
            id: id.clone(),
            default: None,
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
        });

    let merged_identity = IdentityConfig {
        name: identity
            .name
            .clone()
            .or_else(|| base.identity.as_ref().and_then(|i| i.name.clone())),
        theme: identity
            .theme
            .clone()
            .or_else(|| base.identity.as_ref().and_then(|i| i.theme.clone())),
        emoji: identity
            .emoji
            .clone()
            .or_else(|| base.identity.as_ref().and_then(|i| i.emoji.clone())),
        avatar: identity
            .avatar
            .clone()
            .or_else(|| base.identity.as_ref().and_then(|i| i.avatar.clone())),
    };

    let next_entry = AgentConfig {
        identity: Some(merged_identity),
        ..base
    };

    let mut next_list: Vec<AgentConfig> = entries.iter().map(|e| (*e).clone()).collect();
    if let Some(i) = index {
        next_list[i] = next_entry;
    } else {
        let default_id = normalize_agent_id(Some(&resolve_default_agent_id(cfg)));
        if next_list.is_empty() && id != default_id {
            next_list.push(AgentConfig {
                id: resolve_default_agent_id(cfg),
                default: None,
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
            });
        }
        next_list.push(next_entry);
    }

    OpenAcosmiConfig {
        agents: Some(oa_types::agents::AgentsConfig {
            defaults: cfg.agents.as_ref().and_then(|a| a.defaults.clone()),
            list: Some(next_list),
        }),
        ..cfg.clone()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use oa_types::agents::{AgentConfig, AgentsConfig};

    fn make_agent(id: &str) -> AgentConfig {
        AgentConfig {
            id: id.to_owned(),
            default: None,
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

    #[test]
    fn list_agent_entries_empty() {
        let cfg = OpenAcosmiConfig::default();
        assert!(list_agent_entries(&cfg).is_empty());
    }

    #[test]
    fn list_agent_entries_with_agents() {
        let cfg = make_config(vec![make_agent("alpha"), make_agent("beta")]);
        let entries = list_agent_entries(&cfg);
        assert_eq!(entries.len(), 2);
    }

    #[test]
    fn find_agent_entry_index_found() {
        let cfg = make_config(vec![make_agent("alpha"), make_agent("beta")]);
        let entries = list_agent_entries(&cfg);
        assert_eq!(find_agent_entry_index(&entries, "alpha"), Some(0));
        assert_eq!(find_agent_entry_index(&entries, "beta"), Some(1));
    }

    #[test]
    fn find_agent_entry_index_not_found() {
        let cfg = make_config(vec![make_agent("alpha")]);
        let entries = list_agent_entries(&cfg);
        assert_eq!(find_agent_entry_index(&entries, "gamma"), None);
    }

    #[test]
    fn find_agent_entry_index_case_insensitive() {
        let cfg = make_config(vec![make_agent("Alpha")]);
        let entries = list_agent_entries(&cfg);
        assert_eq!(find_agent_entry_index(&entries, "alpha"), Some(0));
    }

    #[test]
    fn build_agent_summaries_default_only() {
        let cfg = OpenAcosmiConfig::default();
        let summaries = build_agent_summaries(&cfg);
        assert_eq!(summaries.len(), 1);
        assert_eq!(summaries[0].id, "main");
        assert!(summaries[0].is_default);
    }

    #[test]
    fn build_agent_summaries_with_agents() {
        let cfg = make_config(vec![make_agent("alpha"), make_agent("beta")]);
        let summaries = build_agent_summaries(&cfg);
        assert_eq!(summaries.len(), 2);
        assert_eq!(summaries[0].id, "alpha");
        assert_eq!(summaries[1].id, "beta");
    }

    #[test]
    fn apply_agent_config_new() {
        let cfg = OpenAcosmiConfig::default();
        let result = apply_agent_config(&cfg, "test-agent", Some("Test"), None, None, None);
        let entries = list_agent_entries(&result);
        // Should have "main" (default) plus "test-agent"
        assert_eq!(entries.len(), 2);
        let test_entry = entries.iter().find(|e| e.id == "test-agent");
        assert!(test_entry.is_some());
        assert_eq!(
            test_entry.expect("test agent exists").name.as_deref(),
            Some("Test")
        );
    }

    #[test]
    fn apply_agent_config_update() {
        let cfg = make_config(vec![make_agent("alpha")]);
        let result = apply_agent_config(
            &cfg,
            "alpha",
            Some("Alpha Bot"),
            None,
            None,
            Some("openai/gpt-4o"),
        );
        let entries = list_agent_entries(&result);
        assert_eq!(entries.len(), 1);
        assert_eq!(entries[0].name.as_deref(), Some("Alpha Bot"));
    }

    #[test]
    fn prune_agent_config_removes_entry() {
        let cfg = make_config(vec![make_agent("alpha"), make_agent("beta")]);
        let result = prune_agent_config(&cfg, "alpha");
        let entries = list_agent_entries(&result.config);
        assert_eq!(entries.len(), 1);
        assert_eq!(entries[0].id, "beta");
    }

    #[test]
    fn prune_agent_config_removes_bindings() {
        use oa_types::agents::{AgentBinding, AgentBindingMatch};
        let cfg = OpenAcosmiConfig {
            agents: Some(AgentsConfig {
                defaults: None,
                list: Some(vec![make_agent("alpha"), make_agent("beta")]),
            }),
            bindings: Some(vec![
                AgentBinding {
                    agent_id: "alpha".to_owned(),
                    r#match: AgentBindingMatch {
                        channel: "discord".to_owned(),
                        account_id: None,
                        peer: None,
                        guild_id: None,
                        team_id: None,
                    },
                },
                AgentBinding {
                    agent_id: "beta".to_owned(),
                    r#match: AgentBindingMatch {
                        channel: "slack".to_owned(),
                        account_id: None,
                        peer: None,
                        guild_id: None,
                        team_id: None,
                    },
                },
            ]),
            ..Default::default()
        };
        let result = prune_agent_config(&cfg, "alpha");
        assert_eq!(result.removed_bindings, 1);
        assert_eq!(result.config.bindings.as_ref().map(Vec::len), Some(1));
    }

    #[test]
    fn apply_agent_identity_new() {
        let cfg = make_config(vec![make_agent("alpha")]);
        let identity = IdentityConfig {
            name: Some("Alpha Bot".to_owned()),
            theme: Some("dark".to_owned()),
            emoji: None,
            avatar: None,
        };
        let result = apply_agent_identity(&cfg, "alpha", &identity);
        let entries = list_agent_entries(&result);
        let entry = entries
            .iter()
            .find(|e| normalize_agent_id(Some(&e.id)) == "alpha");
        assert!(entry.is_some());
        let ident = entry
            .expect("alpha exists")
            .identity
            .as_ref();
        assert!(ident.is_some());
        assert_eq!(
            ident.expect("identity exists").name.as_deref(),
            Some("Alpha Bot")
        );
    }

    #[test]
    fn apply_agent_identity_merges() {
        let mut agent = make_agent("alpha");
        agent.identity = Some(IdentityConfig {
            name: Some("Old Name".to_owned()),
            emoji: Some("robot".to_owned()),
            theme: None,
            avatar: None,
        });
        let cfg = make_config(vec![agent]);
        let identity = IdentityConfig {
            name: Some("New Name".to_owned()),
            theme: None,
            emoji: None,
            avatar: None,
        };
        let result = apply_agent_identity(&cfg, "alpha", &identity);
        let entries = list_agent_entries(&result);
        let ident = entries[0].identity.as_ref().expect("has identity");
        assert_eq!(ident.name.as_deref(), Some("New Name"));
        assert_eq!(ident.emoji.as_deref(), Some("robot"));
    }

    #[test]
    fn agent_summary_serializes_camel_case() {
        let summary = AgentSummary {
            id: "test".to_owned(),
            name: None,
            identity_name: None,
            identity_emoji: None,
            identity_source: None,
            workspace: "/tmp".to_owned(),
            agent_dir: "/tmp/agent".to_owned(),
            model: None,
            bindings: 0,
            binding_details: None,
            routes: None,
            providers: None,
            is_default: true,
        };
        let json = serde_json::to_string(&summary).unwrap_or_default();
        assert!(json.contains("\"isDefault\""));
        assert!(json.contains("\"agentDir\""));
        assert!(!json.contains("\"is_default\""));
        assert!(!json.contains("\"agent_dir\""));
    }
}
