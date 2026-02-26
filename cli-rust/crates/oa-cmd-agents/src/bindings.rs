/// Agent binding utilities: matching, applying, and parsing binding specs.
///
/// Bindings map channel/account/peer patterns to agents, controlling which
/// agent handles messages from each source.
///
/// Source: `src/commands/agents.bindings.ts`

use oa_routing::session_key::{normalize_agent_id, DEFAULT_ACCOUNT_ID};
use oa_types::agents::{AgentBinding, AgentBindingMatch};
use oa_types::config::OpenAcosmiConfig;

/// Compute a deterministic match key for deduplication.
///
/// Source: `src/commands/agents.bindings.ts` - `bindingMatchKey`
fn binding_match_key(m: &AgentBindingMatch) -> String {
    let account_id = m
        .account_id
        .as_deref()
        .map(|s| s.trim())
        .filter(|s| !s.is_empty())
        .unwrap_or(DEFAULT_ACCOUNT_ID);
    let peer_kind = m
        .peer
        .as_ref()
        .map(|p| format!("{:?}", p.kind))
        .unwrap_or_default();
    let peer_id = m
        .peer
        .as_ref()
        .map(|p| p.id.as_str())
        .unwrap_or_default();
    let guild_id = m.guild_id.as_deref().unwrap_or_default();
    let team_id = m.team_id.as_deref().unwrap_or_default();
    format!(
        "{}|{}|{}|{}|{}|{}",
        m.channel, account_id, peer_kind, peer_id, guild_id, team_id
    )
}

/// Describe a binding for human-readable display.
///
/// Source: `src/commands/agents.bindings.ts` - `describeBinding`
#[must_use]
pub fn describe_binding(binding: &AgentBinding) -> String {
    let m = &binding.r#match;
    let mut parts = vec![m.channel.clone()];
    if let Some(ref account_id) = m.account_id {
        parts.push(format!("accountId={account_id}"));
    }
    if let Some(ref peer) = m.peer {
        parts.push(format!("peer={:?}:{}", peer.kind, peer.id));
    }
    if let Some(ref guild_id) = m.guild_id {
        parts.push(format!("guild={guild_id}"));
    }
    if let Some(ref team_id) = m.team_id {
        parts.push(format!("team={team_id}"));
    }
    parts.join(" ")
}

/// Result of applying a set of bindings to the config.
///
/// Source: `src/commands/agents.bindings.ts` - return type of `applyAgentBindings`
pub struct ApplyBindingsResult {
    /// The updated configuration.
    pub config: OpenAcosmiConfig,
    /// Bindings that were successfully added.
    pub added: Vec<AgentBinding>,
    /// Bindings that were skipped (already exist for the same agent).
    pub skipped: Vec<AgentBinding>,
    /// Bindings that conflict with an existing binding for a different agent.
    pub conflicts: Vec<BindingConflict>,
}

/// A binding that conflicts with an existing binding for a different agent.
///
/// Source: `src/commands/agents.bindings.ts` - inline type
pub struct BindingConflict {
    /// The proposed binding.
    pub binding: AgentBinding,
    /// The agent that already owns this match pattern.
    pub existing_agent_id: String,
}

/// Apply a set of bindings to the config, detecting duplicates and conflicts.
///
/// Source: `src/commands/agents.bindings.ts` - `applyAgentBindings`
#[must_use]
pub fn apply_agent_bindings(
    cfg: &OpenAcosmiConfig,
    bindings: &[AgentBinding],
) -> ApplyBindingsResult {
    let existing = cfg.bindings.as_deref().unwrap_or_default();
    let mut existing_match_map = std::collections::HashMap::new();
    for binding in existing {
        let key = binding_match_key(&binding.r#match);
        existing_match_map
            .entry(key)
            .or_insert_with(|| normalize_agent_id(Some(&binding.agent_id)));
    }

    let mut added = Vec::new();
    let mut skipped = Vec::new();
    let mut conflicts = Vec::new();

    for binding in bindings {
        let agent_id = normalize_agent_id(Some(&binding.agent_id));
        let key = binding_match_key(&binding.r#match);
        if let Some(existing_agent_id) = existing_match_map.get(&key) {
            if *existing_agent_id == agent_id {
                skipped.push(binding.clone());
            } else {
                conflicts.push(BindingConflict {
                    binding: binding.clone(),
                    existing_agent_id: existing_agent_id.clone(),
                });
            }
            continue;
        }
        existing_match_map.insert(key, agent_id.clone());
        added.push(AgentBinding {
            agent_id,
            r#match: binding.r#match.clone(),
        });
    }

    if added.is_empty() {
        return ApplyBindingsResult {
            config: cfg.clone(),
            added,
            skipped,
            conflicts,
        };
    }

    let mut all_bindings = existing.to_vec();
    all_bindings.extend(added.clone());

    ApplyBindingsResult {
        config: OpenAcosmiConfig {
            bindings: Some(all_bindings),
            ..cfg.clone()
        },
        added,
        skipped,
        conflicts,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use oa_types::agents::{AgentBinding, AgentBindingMatch};

    fn make_binding(agent: &str, channel: &str) -> AgentBinding {
        AgentBinding {
            agent_id: agent.to_owned(),
            r#match: AgentBindingMatch {
                channel: channel.to_owned(),
                account_id: None,
                peer: None,
                guild_id: None,
                team_id: None,
            },
        }
    }

    #[test]
    fn describe_binding_simple() {
        let binding = make_binding("agent1", "discord");
        let desc = describe_binding(&binding);
        assert_eq!(desc, "discord");
    }

    #[test]
    fn describe_binding_with_account() {
        let binding = AgentBinding {
            agent_id: "agent1".to_owned(),
            r#match: AgentBindingMatch {
                channel: "slack".to_owned(),
                account_id: Some("my-workspace".to_owned()),
                peer: None,
                guild_id: None,
                team_id: None,
            },
        };
        let desc = describe_binding(&binding);
        assert!(desc.contains("slack"));
        assert!(desc.contains("accountId=my-workspace"));
    }

    #[test]
    fn apply_bindings_adds_new() {
        let cfg = OpenAcosmiConfig::default();
        let bindings = vec![make_binding("agent1", "discord")];
        let result = apply_agent_bindings(&cfg, &bindings);
        assert_eq!(result.added.len(), 1);
        assert!(result.skipped.is_empty());
        assert!(result.conflicts.is_empty());
        assert!(result.config.bindings.is_some());
    }

    #[test]
    fn apply_bindings_skips_duplicate() {
        let cfg = OpenAcosmiConfig {
            bindings: Some(vec![make_binding("agent1", "discord")]),
            ..Default::default()
        };
        let bindings = vec![make_binding("agent1", "discord")];
        let result = apply_agent_bindings(&cfg, &bindings);
        assert!(result.added.is_empty());
        assert_eq!(result.skipped.len(), 1);
    }

    #[test]
    fn apply_bindings_detects_conflict() {
        let cfg = OpenAcosmiConfig {
            bindings: Some(vec![make_binding("agent1", "discord")]),
            ..Default::default()
        };
        let bindings = vec![make_binding("agent2", "discord")];
        let result = apply_agent_bindings(&cfg, &bindings);
        assert!(result.added.is_empty());
        assert_eq!(result.conflicts.len(), 1);
        assert_eq!(result.conflicts[0].existing_agent_id, "agent1");
    }
}
