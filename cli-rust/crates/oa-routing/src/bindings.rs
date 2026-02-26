/// Channel binding resolution: maps agent bindings to accounts and channels.
///
/// Source: `src/routing/bindings.ts`

use std::collections::HashMap;

use oa_types::agents::AgentBinding;
use oa_types::config::OpenAcosmiConfig;

use crate::session_key::{normalize_account_id, normalize_agent_id};

// ── Internal helpers ──

/// Normalize a channel ID for binding matching.
///
/// This is a simplified version of `normalizeChatChannelId` from the channels
/// registry -- we just trim and lowercase since we don't have the full channel
/// registry available in this crate.
///
/// Source: `src/routing/bindings.ts` - `normalizeBindingChannelId`
fn normalize_binding_channel_id(raw: Option<&str>) -> Option<String> {
    let fallback = raw.unwrap_or("").trim().to_lowercase();
    if fallback.is_empty() { None } else { Some(fallback) }
}

// ── Public API ──

/// List all agent bindings from the configuration.
///
/// Source: `src/routing/bindings.ts` - `listBindings`
pub fn list_bindings(cfg: &OpenAcosmiConfig) -> &[AgentBinding] {
    cfg.bindings.as_deref().unwrap_or(&[])
}

/// List all unique account IDs bound to a specific channel.
///
/// Returns a sorted vector of normalized account IDs. Wildcard (`"*"`) and
/// blank account IDs are excluded.
///
/// Source: `src/routing/bindings.ts` - `listBoundAccountIds`
pub fn list_bound_account_ids(cfg: &OpenAcosmiConfig, channel_id: &str) -> Vec<String> {
    let normalized_channel = match normalize_binding_channel_id(Some(channel_id)) {
        Some(c) => c,
        None => return Vec::new(),
    };

    let mut ids = Vec::new();
    for binding in list_bindings(cfg) {
        let channel = normalize_binding_channel_id(Some(&binding.r#match.channel));
        if channel.as_deref() != Some(normalized_channel.as_str()) {
            continue;
        }
        let account_id = binding
            .r#match
            .account_id
            .as_deref()
            .unwrap_or("")
            .trim();
        if account_id.is_empty() || account_id == "*" {
            continue;
        }
        let normalized = normalize_account_id(Some(account_id));
        if !ids.contains(&normalized) {
            ids.push(normalized);
        }
    }
    ids.sort();
    ids
}

/// Resolve the account ID bound to the default agent for a given channel.
///
/// Returns the first matching binding's normalized account ID, or `None` if
/// no specific (non-wildcard) account is bound.
///
/// Source: `src/routing/bindings.ts` - `resolveDefaultAgentBoundAccountId`
pub fn resolve_default_agent_bound_account_id(
    cfg: &OpenAcosmiConfig,
    channel_id: &str,
    default_agent_id: &str,
) -> Option<String> {
    let normalized_channel = normalize_binding_channel_id(Some(channel_id))?;
    let target_agent_id = normalize_agent_id(Some(default_agent_id));

    for binding in list_bindings(cfg) {
        if normalize_agent_id(Some(&binding.agent_id)) != target_agent_id {
            continue;
        }
        let channel = normalize_binding_channel_id(Some(&binding.r#match.channel));
        if channel.as_deref() != Some(normalized_channel.as_str()) {
            continue;
        }
        let account_id = binding
            .r#match
            .account_id
            .as_deref()
            .unwrap_or("")
            .trim();
        if account_id.is_empty() || account_id == "*" {
            continue;
        }
        return Some(normalize_account_id(Some(account_id)));
    }
    None
}

/// Build a map of channel -> (agent -> [account IDs]) from the configuration bindings.
///
/// Source: `src/routing/bindings.ts` - `buildChannelAccountBindings`
pub fn build_channel_account_bindings(
    cfg: &OpenAcosmiConfig,
) -> HashMap<String, HashMap<String, Vec<String>>> {
    let mut map: HashMap<String, HashMap<String, Vec<String>>> = HashMap::new();

    for binding in list_bindings(cfg) {
        let channel_id = match normalize_binding_channel_id(Some(&binding.r#match.channel)) {
            Some(c) => c,
            None => continue,
        };
        let account_id = binding
            .r#match
            .account_id
            .as_deref()
            .unwrap_or("")
            .trim()
            .to_owned();
        if account_id.is_empty() || account_id == "*" {
            continue;
        }
        let agent_id = normalize_agent_id(Some(&binding.agent_id));
        let normalized_account = normalize_account_id(Some(&account_id));

        let by_agent = map.entry(channel_id).or_default();
        let list = by_agent.entry(agent_id).or_default();
        if !list.contains(&normalized_account) {
            list.push(normalized_account);
        }
    }
    map
}

/// Resolve the preferred account ID from a list of candidates.
///
/// If bound accounts are available, returns the first one; otherwise falls back
/// to the provided default.
///
/// Source: `src/routing/bindings.ts` - `resolvePreferredAccountId`
pub fn resolve_preferred_account_id(
    _account_ids: &[String],
    default_account_id: &str,
    bound_accounts: &[String],
) -> String {
    if let Some(first) = bound_accounts.first() {
        first.clone()
    } else {
        default_account_id.to_owned()
    }
}

// ── Tests ──

#[cfg(test)]
mod tests {
    use oa_types::agents::{AgentBinding, AgentBindingMatch};

    use super::*;

    fn make_binding(agent_id: &str, channel: &str, account_id: Option<&str>) -> AgentBinding {
        AgentBinding {
            agent_id: agent_id.to_owned(),
            r#match: AgentBindingMatch {
                channel: channel.to_owned(),
                account_id: account_id.map(str::to_owned),
                peer: None,
                guild_id: None,
                team_id: None,
            },
        }
    }

    fn make_config(bindings: Vec<AgentBinding>) -> OpenAcosmiConfig {
        OpenAcosmiConfig {
            bindings: Some(bindings),
            ..Default::default()
        }
    }

    #[test]
    fn list_bindings_empty() {
        let cfg = OpenAcosmiConfig::default();
        assert!(list_bindings(&cfg).is_empty());
    }

    #[test]
    fn list_bindings_returns_all() {
        let cfg = make_config(vec![
            make_binding("bot", "twilio", Some("acct1")),
            make_binding("bot", "discord", Some("acct2")),
        ]);
        assert_eq!(list_bindings(&cfg).len(), 2);
    }

    #[test]
    fn bound_account_ids_for_channel() {
        let cfg = make_config(vec![
            make_binding("bot", "twilio", Some("acct1")),
            make_binding("bot2", "twilio", Some("acct2")),
            make_binding("bot", "discord", Some("acct3")),
        ]);
        let ids = list_bound_account_ids(&cfg, "twilio");
        assert_eq!(ids, vec!["acct1", "acct2"]);
    }

    #[test]
    fn bound_account_ids_excludes_wildcards() {
        let cfg = make_config(vec![
            make_binding("bot", "twilio", Some("*")),
            make_binding("bot2", "twilio", Some("acct1")),
        ]);
        let ids = list_bound_account_ids(&cfg, "twilio");
        assert_eq!(ids, vec!["acct1"]);
    }

    #[test]
    fn bound_account_ids_empty_channel() {
        let cfg = make_config(vec![make_binding("bot", "twilio", Some("acct1"))]);
        let ids = list_bound_account_ids(&cfg, "");
        assert!(ids.is_empty());
    }

    #[test]
    fn resolve_default_agent_account() {
        let cfg = make_config(vec![
            make_binding("main", "twilio", Some("acct1")),
            make_binding("other", "twilio", Some("acct2")),
        ]);
        let result = resolve_default_agent_bound_account_id(&cfg, "twilio", "main");
        assert_eq!(result, Some("acct1".to_owned()));
    }

    #[test]
    fn resolve_default_agent_account_no_match() {
        let cfg = make_config(vec![make_binding("other", "twilio", Some("acct1"))]);
        let result = resolve_default_agent_bound_account_id(&cfg, "twilio", "main");
        assert_eq!(result, None);
    }

    #[test]
    fn channel_account_bindings_map() {
        let cfg = make_config(vec![
            make_binding("bot", "twilio", Some("acct1")),
            make_binding("bot", "twilio", Some("acct2")),
            make_binding("bot2", "twilio", Some("acct1")),
            make_binding("bot", "discord", Some("acct3")),
        ]);
        let map = build_channel_account_bindings(&cfg);
        assert_eq!(map.len(), 2);

        let twilio = map.get("twilio").expect("twilio should exist");
        assert_eq!(twilio.get("bot").expect("bot should exist"), &["acct1", "acct2"]);
        assert_eq!(twilio.get("bot2").expect("bot2 should exist"), &["acct1"]);

        let discord = map.get("discord").expect("discord should exist");
        assert_eq!(discord.get("bot").expect("bot should exist"), &["acct3"]);
    }

    #[test]
    fn preferred_account_id_from_bound() {
        let result = resolve_preferred_account_id(
            &["a".to_owned(), "b".to_owned()],
            "default",
            &["bound1".to_owned(), "bound2".to_owned()],
        );
        assert_eq!(result, "bound1");
    }

    #[test]
    fn preferred_account_id_fallback() {
        let result = resolve_preferred_account_id(
            &["a".to_owned(), "b".to_owned()],
            "default",
            &[],
        );
        assert_eq!(result, "default");
    }
}
