/// Delete an agent and prune its config, bindings, and workspace.
///
/// Source: `src/commands/agents.commands.delete.ts`

use anyhow::{Context, Result};

use oa_config::io::{load_config, write_config_file};
use oa_routing::session_key::normalize_agent_id;

use crate::config::{find_agent_entry_index, list_agent_entries, prune_agent_config};

/// Options for the `agents delete` command.
pub struct AgentsDeleteOptions<'a> {
    pub id: &'a str,
    pub yes: bool,
}

/// Execute the `agents delete` command.
pub async fn agents_delete_command(opts: &AgentsDeleteOptions<'_>) -> Result<String> {
    let cfg = load_config().context("Failed to load config")?;
    let agent_id = normalize_agent_id(Some(opts.id));

    // Verify agent exists
    let entries = list_agent_entries(&cfg);
    if find_agent_entry_index(&entries, &agent_id).is_none() {
        anyhow::bail!("Agent \"{agent_id}\" not found in config.");
    }

    // Prevent deleting the default agent
    let default_id = oa_agents::scope::resolve_default_agent_id(&cfg);
    let default_normalized = normalize_agent_id(Some(&default_id));
    if agent_id == default_normalized {
        anyhow::bail!("Cannot delete the default agent \"{agent_id}\".");
    }

    if !opts.yes {
        return Ok(format!(
            "Would delete agent \"{agent_id}\". Use --yes to confirm."
        ));
    }

    let result = prune_agent_config(&cfg, &agent_id);
    write_config_file(&result.config).await.context("Failed to save config")?;

    let mut msg = format!("Deleted agent \"{agent_id}\".");
    if result.removed_bindings > 0 {
        msg.push_str(&format!(" Removed {} binding(s).", result.removed_bindings));
    }
    if result.removed_allow > 0 {
        msg.push_str(&format!(
            " Removed {} agent-to-agent allow entry(ies).",
            result.removed_allow
        ));
    }

    Ok(msg)
}
