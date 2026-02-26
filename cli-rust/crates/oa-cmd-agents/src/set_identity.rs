/// Update an agent's identity (name, theme, emoji, avatar).
///
/// Source: `src/commands/agents.commands.identity.ts`

use anyhow::{Context, Result};

use oa_config::io::{load_config, write_config_file};
use oa_routing::session_key::normalize_agent_id;
use oa_types::base::IdentityConfig;

use crate::config::apply_agent_identity;

/// Options for the `agents set-identity` command.
pub struct AgentsSetIdentityOptions<'a> {
    pub id: &'a str,
    pub name: Option<&'a str>,
    pub theme: Option<&'a str>,
    pub emoji: Option<&'a str>,
    pub avatar: Option<&'a str>,
}

/// Execute the `agents set-identity` command.
pub async fn agents_set_identity_command(opts: &AgentsSetIdentityOptions<'_>) -> Result<String> {
    let cfg = load_config().context("Failed to load config")?;
    let agent_id = normalize_agent_id(Some(opts.id));

    let identity = IdentityConfig {
        name: opts.name.map(|s| s.to_owned()),
        theme: opts.theme.map(|s| s.to_owned()),
        emoji: opts.emoji.map(|s| s.to_owned()),
        avatar: opts.avatar.map(|s| s.to_owned()),
    };

    let updated = apply_agent_identity(&cfg, &agent_id, &identity);
    write_config_file(&updated).await.context("Failed to save config")?;

    let mut changes = Vec::new();
    if let Some(n) = opts.name {
        changes.push(format!("name={n}"));
    }
    if let Some(t) = opts.theme {
        changes.push(format!("theme={t}"));
    }
    if let Some(e) = opts.emoji {
        changes.push(format!("emoji={e}"));
    }
    if let Some(a) = opts.avatar {
        changes.push(format!("avatar={a}"));
    }

    Ok(format!(
        "Updated identity for agent \"{agent_id}\": {}",
        changes.join(", ")
    ))
}
