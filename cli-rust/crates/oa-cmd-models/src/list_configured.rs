/// Resolve the set of configured model entries from the OpenAcosmi config.
///
/// Walks the default model, fallbacks, image model, image fallbacks, and
/// explicit allowlist to produce an ordered list of configured entries with
/// their tags and aliases.
///
/// Source: `src/commands/models/list.configured.ts`

use std::collections::HashSet;

use oa_agents::defaults::{DEFAULT_MODEL, DEFAULT_PROVIDER};
use oa_agents::model_selection::{
    build_model_alias_index, model_key, parse_model_ref, resolve_configured_model_ref,
    resolve_model_ref_from_string, ModelRef,
};
use oa_types::config::OpenAcosmiConfig;

use crate::list_types::ConfiguredEntry;

/// Resolve the list of configured model entries from config, including
/// the default model, fallbacks, image model, image fallbacks, and
/// explicit models allowlist.
///
/// Source: `src/commands/models/list.configured.ts` - `resolveConfiguredEntries`
#[must_use]
pub fn resolve_configured_entries(cfg: &OpenAcosmiConfig) -> Vec<ConfiguredEntry> {
    let resolved_default = resolve_configured_model_ref(cfg, DEFAULT_PROVIDER, DEFAULT_MODEL);
    let alias_index = build_model_alias_index(cfg, DEFAULT_PROVIDER);

    let mut order: Vec<String> = Vec::new();
    let mut tags_by_key = std::collections::HashMap::<String, HashSet<String>>::new();
    let mut aliases_by_key = std::collections::HashMap::<String, Vec<String>>::new();

    // Copy aliases from the alias index
    for (key, aliases) in &alias_index.by_key {
        aliases_by_key.insert(key.clone(), aliases.clone());
    }

    let mut add_entry = |model_ref: &ModelRef, tag: &str| {
        let key = model_key(&model_ref.provider, &model_ref.model);
        if !tags_by_key.contains_key(&key) {
            tags_by_key.insert(key.clone(), HashSet::new());
            order.push(key.clone());
        }
        if let Some(tags) = tags_by_key.get_mut(&key) {
            tags.insert(tag.to_owned());
        }
    };

    // 1. Default model
    add_entry(&resolved_default, "default");

    // 2. Model fallbacks
    let model_fallbacks = cfg
        .agents
        .as_ref()
        .and_then(|a| a.defaults.as_ref())
        .and_then(|d| d.model.as_ref())
        .and_then(|m| m.fallbacks.as_ref())
        .cloned()
        .unwrap_or_default();

    for (idx, raw) in model_fallbacks.iter().enumerate() {
        if let Some((resolved, _)) =
            resolve_model_ref_from_string(raw, DEFAULT_PROVIDER, Some(&alias_index))
        {
            add_entry(&resolved, &format!("fallback#{}", idx + 1));
        }
    }

    // 3. Image model primary
    let image_primary = cfg
        .agents
        .as_ref()
        .and_then(|a| a.defaults.as_ref())
        .and_then(|d| d.image_model.as_ref())
        .and_then(|m| m.primary.as_ref())
        .map(|s| s.trim().to_owned())
        .filter(|s| !s.is_empty());

    if let Some(ref raw) = image_primary {
        if let Some((resolved, _)) =
            resolve_model_ref_from_string(raw, DEFAULT_PROVIDER, Some(&alias_index))
        {
            add_entry(&resolved, "image");
        }
    }

    // 4. Image fallbacks
    let image_fallbacks = cfg
        .agents
        .as_ref()
        .and_then(|a| a.defaults.as_ref())
        .and_then(|d| d.image_model.as_ref())
        .and_then(|m| m.fallbacks.as_ref())
        .cloned()
        .unwrap_or_default();

    for (idx, raw) in image_fallbacks.iter().enumerate() {
        if let Some((resolved, _)) =
            resolve_model_ref_from_string(raw, DEFAULT_PROVIDER, Some(&alias_index))
        {
            add_entry(&resolved, &format!("img-fallback#{}", idx + 1));
        }
    }

    // 5. Explicit models allowlist
    let models_map = cfg
        .agents
        .as_ref()
        .and_then(|a| a.defaults.as_ref())
        .and_then(|d| d.models.as_ref());

    if let Some(models) = models_map {
        for key in models.keys() {
            if let Some(parsed) = parse_model_ref(key, DEFAULT_PROVIDER) {
                add_entry(&parsed, "configured");
            }
        }
    }

    // Build the output entries in order
    order
        .iter()
        .map(|key| {
            let slash = key.find('/');
            let (provider, model) = match slash {
                None => (key.as_str(), ""),
                Some(idx) => (&key[..idx], &key[idx + 1..]),
            };
            ConfiguredEntry {
                key: key.clone(),
                ref_provider: provider.to_owned(),
                ref_model: model.to_owned(),
                tags: tags_by_key.remove(key).unwrap_or_default(),
                aliases: aliases_by_key.remove(key).unwrap_or_default(),
            }
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use oa_types::agent_defaults::{AgentDefaultsConfig, AgentModelEntryConfig, AgentModelListConfig};
    use oa_types::agents::AgentsConfig;
    use std::collections::HashMap;

    #[test]
    fn default_config_has_default_entry() {
        let cfg = OpenAcosmiConfig::default();
        let entries = resolve_configured_entries(&cfg);
        assert!(!entries.is_empty());
        assert!(entries[0].tags.contains("default"));
        assert_eq!(entries[0].key, "anthropic/claude-opus-4-6");
    }

    #[test]
    fn fallbacks_get_numbered_tags() {
        let cfg = OpenAcosmiConfig {
            agents: Some(AgentsConfig {
                defaults: Some(AgentDefaultsConfig {
                    model: Some(AgentModelListConfig {
                        primary: Some("anthropic/claude-opus-4-6".to_owned()),
                        fallbacks: Some(vec![
                            "openai/gpt-4o".to_owned(),
                            "google/gemini-3-pro-preview".to_owned(),
                        ]),
                    }),
                    ..Default::default()
                }),
                list: None,
            }),
            ..Default::default()
        };
        let entries = resolve_configured_entries(&cfg);
        // Check that fallbacks are tagged
        let fallback1 = entries.iter().find(|e| e.tags.contains("fallback#1"));
        assert!(fallback1.is_some());
        let fallback2 = entries.iter().find(|e| e.tags.contains("fallback#2"));
        assert!(fallback2.is_some());
    }

    #[test]
    fn configured_models_tagged() {
        let mut models = HashMap::new();
        models.insert(
            "openai/gpt-4o".to_owned(),
            AgentModelEntryConfig::default(),
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
        let entries = resolve_configured_entries(&cfg);
        let gpt_entry = entries.iter().find(|e| e.key == "openai/gpt-4o");
        assert!(gpt_entry.is_some());
        assert!(
            gpt_entry
                .expect("gpt-4o should exist")
                .tags
                .contains("configured")
        );
    }

    #[test]
    fn aliases_populated() {
        let mut models = HashMap::new();
        models.insert(
            "anthropic/claude-opus-4-6".to_owned(),
            AgentModelEntryConfig {
                alias: Some("opus".to_owned()),
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
        let entries = resolve_configured_entries(&cfg);
        let default_entry = entries
            .iter()
            .find(|e| e.key == "anthropic/claude-opus-4-6");
        assert!(default_entry.is_some());
        let entry = default_entry.expect("should find default entry");
        assert!(entry.aliases.contains(&"opus".to_owned()));
    }
}
