/// Model alias management commands: list, add, and remove aliases.
///
/// Source: `src/commands/models/aliases.ts`

use anyhow::Result;

use oa_config::io::load_config;
use oa_types::config::OpenAcosmiConfig;

use crate::shared::{
    ensure_flag_compatibility, normalize_alias, resolve_model_target, update_config,
};

/// Output format for the alias list.
///
/// Source: `src/commands/models/aliases.ts`
#[derive(Debug, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
struct AliasesJsonOutput {
    aliases: std::collections::HashMap<String, String>,
}

/// Collect configured aliases from the models map.
///
/// Returns a map from alias name -> model key.
fn collect_aliases(cfg: &OpenAcosmiConfig) -> std::collections::HashMap<String, String> {
    let mut aliases = std::collections::HashMap::new();
    let models = match cfg
        .agents
        .as_ref()
        .and_then(|a| a.defaults.as_ref())
        .and_then(|d| d.models.as_ref())
    {
        Some(m) => m,
        None => return aliases,
    };
    for (model_key, entry) in models {
        if let Some(alias) = entry.alias.as_ref() {
            let trimmed = alias.trim();
            if !trimmed.is_empty() {
                aliases.insert(trimmed.to_owned(), model_key.clone());
            }
        }
    }
    aliases
}

/// List all configured model aliases.
///
/// Source: `src/commands/models/aliases.ts` - `modelsAliasesListCommand`
pub fn models_aliases_list_command(json: bool, plain: bool) -> Result<String> {
    ensure_flag_compatibility(json, plain)?;
    let cfg = load_config()?;
    let aliases = collect_aliases(&cfg);

    if json {
        let output = AliasesJsonOutput { aliases };
        return Ok(serde_json::to_string_pretty(&output)?);
    }

    if plain {
        let mut lines = Vec::new();
        for (alias, target) in &aliases {
            lines.push(format!("{alias} {target}"));
        }
        return Ok(lines.join("\n"));
    }

    let mut lines = vec![format!("Aliases ({}):", aliases.len())];
    if aliases.is_empty() {
        lines.push("- none".to_owned());
    } else {
        for (alias, target) in &aliases {
            lines.push(format!("- {alias} -> {target}"));
        }
    }
    Ok(lines.join("\n"))
}

/// Add a model alias.
///
/// Source: `src/commands/models/aliases.ts` - `modelsAliasesAddCommand`
pub async fn models_aliases_add_command(alias_raw: &str, model_raw: &str) -> Result<String> {
    let alias = normalize_alias(alias_raw)?;
    let cfg_snapshot = load_config()?;
    let resolved = resolve_model_target(model_raw, &cfg_snapshot)?;
    let target_key = format!("{}/{}", resolved.provider, resolved.model);

    update_config(|cfg| {
        let mut next = cfg.clone();
        let agents = next.agents.get_or_insert_with(Default::default);
        let defaults = agents.defaults.get_or_insert_with(Default::default);
        let models = defaults.models.get_or_insert_with(Default::default);

        // Check for existing alias pointing to a different model
        for (key, entry) in models.iter() {
            if let Some(existing_alias) = entry.alias.as_ref() {
                let existing_trimmed = existing_alias.trim();
                if existing_trimmed == alias && *key != target_key {
                    anyhow::bail!("Alias {alias} already points to {key}.");
                }
            }
        }

        let entry = models.entry(target_key.clone()).or_default();
        entry.alias = Some(alias.clone());

        Ok(next)
    })
    .await?;

    Ok(format!("Alias {alias} -> {}/{}", resolved.provider, resolved.model))
}

/// Remove a model alias.
///
/// Source: `src/commands/models/aliases.ts` - `modelsAliasesRemoveCommand`
pub async fn models_aliases_remove_command(alias_raw: &str) -> Result<String> {
    let alias = normalize_alias(alias_raw)?;

    let updated = update_config(|cfg| {
        let mut next = cfg.clone();
        let agents = next.agents.get_or_insert_with(Default::default);
        let defaults = agents.defaults.get_or_insert_with(Default::default);
        let models = defaults.models.get_or_insert_with(Default::default);

        let mut found = false;
        for entry in models.values_mut() {
            if let Some(existing_alias) = entry.alias.as_ref() {
                if existing_alias.trim() == alias {
                    entry.alias = None;
                    found = true;
                    break;
                }
            }
        }
        if !found {
            anyhow::bail!("Alias not found: {alias}");
        }

        Ok(next)
    })
    .await?;

    // Check if any aliases remain
    let remaining_aliases = collect_aliases(&updated);
    if remaining_aliases.is_empty() {
        Ok("No aliases configured.".to_owned())
    } else {
        Ok(format!("Removed alias: {alias}"))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use oa_types::agent_defaults::{AgentDefaultsConfig, AgentModelEntryConfig};
    use oa_types::agents::AgentsConfig;
    use std::collections::HashMap;

    #[test]
    fn collect_aliases_empty() {
        let cfg = OpenAcosmiConfig::default();
        assert!(collect_aliases(&cfg).is_empty());
    }

    #[test]
    fn collect_aliases_with_entries() {
        let mut models = HashMap::new();
        models.insert(
            "anthropic/claude-opus-4-6".to_owned(),
            AgentModelEntryConfig {
                alias: Some("opus".to_owned()),
                params: None,
                streaming: None,
            },
        );
        models.insert(
            "openai/gpt-4o".to_owned(),
            AgentModelEntryConfig {
                alias: Some("gpt".to_owned()),
                params: None,
                streaming: None,
            },
        );
        let cfg = OpenAcosmiConfig {
            agents: Some(AgentsConfig {
                defaults: Some(AgentDefaultsConfig {
                    models: Some(models),
                    ..Default::default()
                }),
                list: None,
            }),
            ..Default::default()
        };
        let result = collect_aliases(&cfg);
        assert_eq!(result.len(), 2);
        assert_eq!(
            result.get("opus"),
            Some(&"anthropic/claude-opus-4-6".to_owned())
        );
        assert_eq!(result.get("gpt"), Some(&"openai/gpt-4o".to_owned()));
    }

    #[test]
    fn collect_aliases_skips_empty() {
        let mut models = HashMap::new();
        models.insert(
            "anthropic/claude-opus-4-6".to_owned(),
            AgentModelEntryConfig {
                alias: Some("  ".to_owned()),
                params: None,
                streaming: None,
            },
        );
        let cfg = OpenAcosmiConfig {
            agents: Some(AgentsConfig {
                defaults: Some(AgentDefaultsConfig {
                    models: Some(models),
                    ..Default::default()
                }),
                list: None,
            }),
            ..Default::default()
        };
        assert!(collect_aliases(&cfg).is_empty());
    }

    #[test]
    fn aliases_list_json_format() {
        // Test that the JSON output produces valid JSON
        let output = AliasesJsonOutput {
            aliases: {
                let mut m = std::collections::HashMap::new();
                m.insert(
                    "opus".to_owned(),
                    "anthropic/claude-opus-4-6".to_owned(),
                );
                m
            },
        };
        let json = serde_json::to_string_pretty(&output).unwrap_or_default();
        assert!(json.contains("\"aliases\""));
        assert!(json.contains("\"opus\""));
    }
}
