/// Image model fallback management commands: list, add, remove, and clear.
///
/// Source: `src/commands/models/image-fallbacks.ts`

use anyhow::{bail, Result};

use oa_agents::defaults::DEFAULT_PROVIDER;
use oa_agents::model_selection::{
    build_model_alias_index, model_key, resolve_model_ref_from_string,
};
use oa_config::io::load_config;

use crate::shared::{ensure_flag_compatibility, resolve_model_target, update_config};

/// List image model fallbacks.
///
/// Source: `src/commands/models/image-fallbacks.ts` - `modelsImageFallbacksListCommand`
pub fn models_image_fallbacks_list_command(json: bool, plain: bool) -> Result<String> {
    ensure_flag_compatibility(json, plain)?;
    let cfg = load_config()?;
    let fallbacks = cfg
        .agents
        .as_ref()
        .and_then(|a| a.defaults.as_ref())
        .and_then(|d| d.image_model.as_ref())
        .and_then(|m| m.fallbacks.as_ref())
        .cloned()
        .unwrap_or_default();

    if json {
        #[derive(serde::Serialize)]
        struct Output {
            fallbacks: Vec<String>,
        }
        return Ok(serde_json::to_string_pretty(&Output { fallbacks })?);
    }
    if plain {
        return Ok(fallbacks.join("\n"));
    }

    let mut lines = vec![format!("Image fallbacks ({}):", fallbacks.len())];
    if fallbacks.is_empty() {
        lines.push("- none".to_owned());
    } else {
        for entry in &fallbacks {
            lines.push(format!("- {entry}"));
        }
    }
    Ok(lines.join("\n"))
}

/// Add a model to the image fallback list.
///
/// Source: `src/commands/models/image-fallbacks.ts` - `modelsImageFallbacksAddCommand`
pub async fn models_image_fallbacks_add_command(model_raw: &str) -> Result<String> {
    let cfg_snapshot = load_config()?;
    let resolved = resolve_model_target(model_raw, &cfg_snapshot)?;
    let target_key = model_key(&resolved.provider, &resolved.model);

    let updated = update_config(|cfg| {
        let mut next = cfg.clone();

        // Ensure the target model is in the models map
        {
            let agents = next.agents.get_or_insert_with(Default::default);
            let defaults = agents.defaults.get_or_insert_with(Default::default);
            let models = defaults.models.get_or_insert_with(Default::default);
            models.entry(target_key.clone()).or_default();
        }

        // Build alias index with an immutable borrow
        let alias_index = build_model_alias_index(&next, DEFAULT_PROVIDER);
        let existing = next
            .agents
            .as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.image_model.as_ref())
            .and_then(|m| m.fallbacks.as_ref())
            .cloned()
            .unwrap_or_default();

        let existing_keys: Vec<String> = existing
            .iter()
            .filter_map(|entry| {
                resolve_model_ref_from_string(entry, DEFAULT_PROVIDER, Some(&alias_index))
                    .map(|(r, _)| model_key(&r.provider, &r.model))
            })
            .collect();

        if existing_keys.contains(&target_key) {
            return Ok(next);
        }

        // Preserve primary and update fallbacks
        let existing_primary = next
            .agents
            .as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.image_model.as_ref())
            .and_then(|m| m.primary.clone());

        let agents = next.agents.get_or_insert_with(Default::default);
        let defaults = agents.defaults.get_or_insert_with(Default::default);
        let image_config = defaults.image_model.get_or_insert_with(Default::default);
        if let Some(primary) = existing_primary {
            image_config.primary = Some(primary);
        }
        let mut new_fallbacks = existing;
        new_fallbacks.push(target_key);
        image_config.fallbacks = Some(new_fallbacks);

        Ok(next)
    })
    .await?;

    let fallbacks = updated
        .agents
        .as_ref()
        .and_then(|a| a.defaults.as_ref())
        .and_then(|d| d.image_model.as_ref())
        .and_then(|m| m.fallbacks.as_ref())
        .cloned()
        .unwrap_or_default();

    Ok(format!("Image fallbacks: {}", fallbacks.join(", ")))
}

/// Remove a model from the image fallback list.
///
/// Source: `src/commands/models/image-fallbacks.ts` - `modelsImageFallbacksRemoveCommand`
pub async fn models_image_fallbacks_remove_command(model_raw: &str) -> Result<String> {
    let cfg_snapshot = load_config()?;
    let resolved = resolve_model_target(model_raw, &cfg_snapshot)?;
    let target_key = model_key(&resolved.provider, &resolved.model);

    let updated = update_config(|cfg| {
        let mut next = cfg.clone();
        let alias_index = build_model_alias_index(&next, DEFAULT_PROVIDER);

        let existing = next
            .agents
            .as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.image_model.as_ref())
            .and_then(|m| m.fallbacks.as_ref())
            .cloned()
            .unwrap_or_default();

        let filtered: Vec<String> = existing
            .iter()
            .filter(|entry| {
                let resolved_entry =
                    resolve_model_ref_from_string(entry, DEFAULT_PROVIDER, Some(&alias_index));
                match resolved_entry {
                    None => true,
                    Some((r, _)) => model_key(&r.provider, &r.model) != target_key,
                }
            })
            .cloned()
            .collect();

        if filtered.len() == existing.len() {
            bail!("Image fallback not found: {target_key}");
        }

        let agents = next.agents.get_or_insert_with(Default::default);
        let defaults = agents.defaults.get_or_insert_with(Default::default);
        let existing_primary = defaults
            .image_model
            .as_ref()
            .and_then(|m| m.primary.clone());
        let image_config = defaults.image_model.get_or_insert_with(Default::default);
        if let Some(primary) = existing_primary {
            image_config.primary = Some(primary);
        }
        image_config.fallbacks = Some(filtered);

        Ok(next)
    })
    .await?;

    let fallbacks = updated
        .agents
        .as_ref()
        .and_then(|a| a.defaults.as_ref())
        .and_then(|d| d.image_model.as_ref())
        .and_then(|m| m.fallbacks.as_ref())
        .cloned()
        .unwrap_or_default();

    Ok(format!("Image fallbacks: {}", fallbacks.join(", ")))
}

/// Clear all image model fallbacks.
///
/// Source: `src/commands/models/image-fallbacks.ts` - `modelsImageFallbacksClearCommand`
pub async fn models_image_fallbacks_clear_command() -> Result<String> {
    update_config(|cfg| {
        let mut next = cfg.clone();
        let agents = next.agents.get_or_insert_with(Default::default);
        let defaults = agents.defaults.get_or_insert_with(Default::default);
        let existing_primary = defaults
            .image_model
            .as_ref()
            .and_then(|m| m.primary.clone());
        let image_config = defaults.image_model.get_or_insert_with(Default::default);
        if let Some(primary) = existing_primary {
            image_config.primary = Some(primary);
        }
        image_config.fallbacks = Some(Vec::new());
        Ok(next)
    })
    .await?;

    Ok("Image fallback list cleared.".to_owned())
}

#[cfg(test)]
mod tests {
    use oa_types::agent_defaults::{AgentDefaultsConfig, AgentModelListConfig};
    use oa_types::agents::AgentsConfig;
    use oa_types::config::OpenAcosmiConfig;

    #[test]
    fn extract_image_fallbacks_from_config() {
        let cfg = OpenAcosmiConfig {
            agents: Some(AgentsConfig {
                defaults: Some(AgentDefaultsConfig {
                    image_model: Some(AgentModelListConfig {
                        primary: Some("openai/dall-e-3".to_owned()),
                        fallbacks: Some(vec!["google/gemini-pro-vision".to_owned()]),
                    }),
                    ..Default::default()
                }),
                list: None,
            }),
            ..Default::default()
        };
        let fallbacks = cfg
            .agents
            .as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.image_model.as_ref())
            .and_then(|m| m.fallbacks.as_ref())
            .cloned()
            .unwrap_or_default();
        assert_eq!(fallbacks, vec!["google/gemini-pro-vision"]);
    }

    #[test]
    fn empty_image_fallbacks() {
        let cfg = OpenAcosmiConfig::default();
        let fallbacks = cfg
            .agents
            .as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.image_model.as_ref())
            .and_then(|m| m.fallbacks.as_ref())
            .cloned()
            .unwrap_or_default();
        assert!(fallbacks.is_empty());
    }
}
