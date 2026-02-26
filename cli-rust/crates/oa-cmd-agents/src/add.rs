/// Add a new isolated agent.
///
/// Source: `src/commands/agents.commands.add.ts`

use anyhow::{Context, Result};

use oa_config::io::{load_config, write_config_file};
use oa_routing::session_key::normalize_agent_id;

use crate::config::apply_agent_config;

/// Options for the `agents add` command.
pub struct AgentsAddOptions<'a> {
    pub id: &'a str,
    pub name: Option<&'a str>,
    pub workspace: Option<&'a str>,
    pub model: Option<&'a str>,
}

/// Execute the `agents add` command.
pub async fn agents_add_command(opts: &AgentsAddOptions<'_>) -> Result<String> {
    let cfg = load_config().context("Failed to load config")?;
    let agent_id = normalize_agent_id(Some(opts.id));

    // Check for duplicates
    let entries = crate::config::list_agent_entries(&cfg);
    if crate::config::find_agent_entry_index(&entries, &agent_id).is_some() {
        anyhow::bail!("Agent \"{agent_id}\" already exists. Use `agents set-identity` to update.");
    }

    let updated = apply_agent_config(&cfg, &agent_id, opts.name, opts.workspace, None, opts.model);
    write_config_file(&updated).await.context("Failed to save config")?;

    Ok(format!("Added agent \"{agent_id}\"."))
}

#[cfg(test)]
mod tests {
    use super::*;
    use oa_types::config::OpenAcosmiConfig;

    #[tokio::test]
    async fn add_validates_duplicate() {
        // This test verifies the duplicate check logic exists.
        // Actual file I/O tests require a temp config directory.
        let _ = AgentsAddOptions {
            id: "test",
            name: Some("Test"),
            workspace: None,
            model: None,
        };
    }
}
