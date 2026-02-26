/// Model catalog for available AI models.
///
/// Provides types and functions for loading, querying, and filtering the
/// set of models available to agents. The catalog combines built-in entries
/// from configured providers with any user-configured custom models.
///
/// Source: `src/agents/model-catalog.ts`

use serde::{Deserialize, Serialize};

use oa_types::config::OpenAcosmiConfig;
use oa_types::models::ModelInputType;

/// A single entry in the model catalog.
///
/// Source: `src/agents/model-catalog.ts` - `ModelCatalogEntry`
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ModelCatalogEntry {
    /// Model identifier (e.g. `"claude-opus-4-6"`).
    pub id: String,
    /// Human-readable display name.
    pub name: String,
    /// Provider identifier (e.g. `"anthropic"`, `"openai"`).
    pub provider: String,
    /// Context window size in tokens, if known.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub context_window: Option<u64>,
    /// Whether this model supports extended reasoning/thinking.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub reasoning: Option<bool>,
    /// Input modalities supported by this model.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub input: Option<Vec<ModelInputType>>,
}

/// Load the model catalog from configured providers in the config.
///
/// Iterates over `models.providers` in the configuration, extracts each
/// provider's model definitions, and returns a sorted list of catalog entries.
/// Models are sorted by (provider, name).
///
/// Source: `src/agents/model-catalog.ts` - `loadModelCatalog`
pub fn load_model_catalog(cfg: &OpenAcosmiConfig) -> Vec<ModelCatalogEntry> {
    let mut entries = Vec::new();

    if let Some(ref models_config) = cfg.models {
        if let Some(ref providers) = models_config.providers {
            for (provider_id, provider_config) in providers {
                let provider = provider_id.trim().to_owned();
                if provider.is_empty() {
                    continue;
                }
                for model_def in &provider_config.models {
                    let id = model_def.id.trim().to_owned();
                    if id.is_empty() {
                        continue;
                    }
                    let name = {
                        let n = model_def.name.trim();
                        if n.is_empty() { id.clone() } else { n.to_owned() }
                    };
                    let context_window = if model_def.context_window > 0 {
                        Some(model_def.context_window)
                    } else {
                        None
                    };
                    let reasoning = Some(model_def.reasoning);
                    let input = if model_def.input.is_empty() {
                        None
                    } else {
                        Some(model_def.input.clone())
                    };

                    entries.push(ModelCatalogEntry {
                        id,
                        name,
                        provider: provider.clone(),
                        context_window,
                        reasoning,
                        input,
                    });
                }
            }
        }
    }

    entries.sort_by(|a, b| {
        let p = a.provider.cmp(&b.provider);
        if p != std::cmp::Ordering::Equal {
            return p;
        }
        a.name.cmp(&b.name)
    });

    entries
}

/// Check if a model catalog entry supports image (vision) input.
///
/// Source: `src/agents/model-catalog.ts` - `modelSupportsVision`
pub fn model_supports_vision(entry: Option<&ModelCatalogEntry>) -> bool {
    entry
        .and_then(|e| e.input.as_ref())
        .is_some_and(|inputs| inputs.contains(&ModelInputType::Image))
}

/// Find a model in the catalog by provider and model ID (case-insensitive).
///
/// Source: `src/agents/model-catalog.ts` - `findModelInCatalog`
pub fn find_model_in_catalog<'a>(
    catalog: &'a [ModelCatalogEntry],
    provider: &str,
    model_id: &str,
) -> Option<&'a ModelCatalogEntry> {
    let normalized_provider = provider.to_lowercase().trim().to_owned();
    let normalized_model_id = model_id.to_lowercase().trim().to_owned();
    catalog.iter().find(|entry| {
        entry.provider.to_lowercase() == normalized_provider
            && entry.id.to_lowercase() == normalized_model_id
    })
}

/// Find a model in the catalog by a combined `provider/model_id` query string.
///
/// If the query contains a `/`, splits into provider and model ID.
/// Otherwise searches by model ID across all providers.
///
/// Source: `src/agents/model-catalog.ts` (derived helper)
pub fn find_model_by_query<'a>(
    catalog: &'a [ModelCatalogEntry],
    query: &str,
) -> Option<&'a ModelCatalogEntry> {
    let trimmed = query.trim();
    if trimmed.is_empty() {
        return None;
    }

    if let Some(slash) = trimmed.find('/') {
        let provider = &trimmed[..slash];
        let model_id = &trimmed[slash + 1..];
        find_model_in_catalog(catalog, provider, model_id)
    } else {
        // Search by model ID only across all providers
        let lower = trimmed.to_lowercase();
        catalog
            .iter()
            .find(|entry| entry.id.to_lowercase() == lower)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use oa_types::models::{
        ModelCostConfig, ModelDefinitionConfig, ModelProviderConfig, ModelsConfig,
    };
    use std::collections::HashMap;

    fn sample_cost() -> ModelCostConfig {
        ModelCostConfig {
            input: 3.0,
            output: 15.0,
            cache_read: 0.3,
            cache_write: 3.75,
        }
    }

    fn sample_model(id: &str, name: &str, reasoning: bool) -> ModelDefinitionConfig {
        ModelDefinitionConfig {
            id: id.to_owned(),
            name: name.to_owned(),
            api: None,
            reasoning,
            input: vec![ModelInputType::Text, ModelInputType::Image],
            cost: sample_cost(),
            context_window: 200_000,
            max_tokens: 4096,
            headers: None,
            compat: None,
        }
    }

    fn sample_config() -> OpenAcosmiConfig {
        let mut providers = HashMap::new();
        providers.insert(
            "anthropic".to_owned(),
            ModelProviderConfig {
                base_url: "https://api.anthropic.com".to_owned(),
                api_key: None,
                auth: None,
                api: None,
                headers: None,
                auth_header: None,
                models: vec![
                    sample_model("claude-opus-4-6", "Claude Opus 4.6", true),
                    sample_model("claude-sonnet-4-5", "Claude Sonnet 4.5", false),
                ],
            },
        );
        providers.insert(
            "openai".to_owned(),
            ModelProviderConfig {
                base_url: "https://api.openai.com/v1".to_owned(),
                api_key: None,
                auth: None,
                api: None,
                headers: None,
                auth_header: None,
                models: vec![sample_model("gpt-4o", "GPT-4o", false)],
            },
        );
        OpenAcosmiConfig {
            models: Some(ModelsConfig {
                mode: None,
                providers: Some(providers),
                bedrock_discovery: None,
            }),
            ..Default::default()
        }
    }

    #[test]
    fn load_catalog_from_config() {
        let cfg = sample_config();
        let catalog = load_model_catalog(&cfg);
        // Should have 3 models total
        assert_eq!(catalog.len(), 3);
        // Sorted by provider, then name
        assert_eq!(catalog[0].provider, "anthropic");
        assert_eq!(catalog[0].name, "Claude Opus 4.6");
        assert_eq!(catalog[1].provider, "anthropic");
        assert_eq!(catalog[1].name, "Claude Sonnet 4.5");
        assert_eq!(catalog[2].provider, "openai");
    }

    #[test]
    fn load_catalog_empty_config() {
        let cfg = OpenAcosmiConfig::default();
        let catalog = load_model_catalog(&cfg);
        assert!(catalog.is_empty());
    }

    #[test]
    fn find_model_by_provider_and_id() {
        let cfg = sample_config();
        let catalog = load_model_catalog(&cfg);
        let found = find_model_in_catalog(&catalog, "anthropic", "claude-opus-4-6");
        assert!(found.is_some());
        assert_eq!(found.map(|e| e.name.as_str()), Some("Claude Opus 4.6"));
    }

    #[test]
    fn find_model_case_insensitive() {
        let cfg = sample_config();
        let catalog = load_model_catalog(&cfg);
        let found = find_model_in_catalog(&catalog, "Anthropic", "Claude-Opus-4-6");
        assert!(found.is_some());
    }

    #[test]
    fn find_model_not_found() {
        let cfg = sample_config();
        let catalog = load_model_catalog(&cfg);
        assert!(find_model_in_catalog(&catalog, "anthropic", "nonexistent").is_none());
    }

    #[test]
    fn find_model_by_query_with_provider() {
        let cfg = sample_config();
        let catalog = load_model_catalog(&cfg);
        let found = find_model_by_query(&catalog, "anthropic/claude-opus-4-6");
        assert!(found.is_some());
    }

    #[test]
    fn find_model_by_query_id_only() {
        let cfg = sample_config();
        let catalog = load_model_catalog(&cfg);
        let found = find_model_by_query(&catalog, "gpt-4o");
        assert!(found.is_some());
        assert_eq!(found.map(|e| e.provider.as_str()), Some("openai"));
    }

    #[test]
    fn vision_support_check() {
        let entry = ModelCatalogEntry {
            id: "test".to_owned(),
            name: "Test".to_owned(),
            provider: "test".to_owned(),
            context_window: None,
            reasoning: None,
            input: Some(vec![ModelInputType::Text, ModelInputType::Image]),
        };
        assert!(model_supports_vision(Some(&entry)));
    }

    #[test]
    fn vision_support_text_only() {
        let entry = ModelCatalogEntry {
            id: "test".to_owned(),
            name: "Test".to_owned(),
            provider: "test".to_owned(),
            context_window: None,
            reasoning: None,
            input: Some(vec![ModelInputType::Text]),
        };
        assert!(!model_supports_vision(Some(&entry)));
    }

    #[test]
    fn vision_support_none() {
        assert!(!model_supports_vision(None));
    }
}
