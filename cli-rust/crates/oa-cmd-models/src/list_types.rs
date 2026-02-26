/// Types used by the models list and status commands.
///
/// Source: `src/commands/models/list.types.ts`

use std::collections::HashSet;

use serde::{Deserialize, Serialize};

/// A model entry that has been resolved from the configuration, with
/// its canonical key, provider/model ref, tags, and aliases.
///
/// Source: `src/commands/models/list.types.ts` - `ConfiguredEntry`
#[derive(Debug, Clone, Serialize)]
pub struct ConfiguredEntry {
    /// Canonical model key, e.g. `"anthropic/claude-opus-4-6"`.
    pub key: String,
    /// Provider and model ID.
    pub ref_provider: String,
    /// Model identifier within the provider.
    pub ref_model: String,
    /// Tags assigned to this entry (e.g. `"default"`, `"fallback#1"`).
    pub tags: HashSet<String>,
    /// User-defined aliases for this model.
    pub aliases: Vec<String>,
}

/// A row in the model table shown by `models list`.
///
/// Source: `src/commands/models/list.types.ts` - `ModelRow`
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ModelRow {
    /// Canonical model key.
    pub key: String,
    /// Display name.
    pub name: String,
    /// Input modalities (e.g. `"text+image"`).
    pub input: String,
    /// Context window size in tokens.
    pub context_window: Option<u64>,
    /// Whether the model runs locally.
    pub local: Option<bool>,
    /// Whether auth is available for this model.
    pub available: Option<bool>,
    /// Tags for display.
    pub tags: Vec<String>,
    /// Whether this model is missing from the registry.
    pub missing: bool,
}

/// Auth overview for a provider, showing how authentication is resolved.
///
/// Source: `src/commands/models/list.types.ts` - `ProviderAuthOverview`
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ProviderAuthOverview {
    /// Provider identifier.
    pub provider: String,
    /// The effective auth source.
    pub effective: ProviderAuthEffective,
    /// Profile statistics.
    pub profiles: ProviderAuthProfileStats,
    /// Environment variable source, if any.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub env: Option<ProviderAuthSource>,
    /// models.json source, if any.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub models_json: Option<ProviderAuthSource>,
}

/// The effective auth kind and detail string.
///
/// Source: `src/commands/models/list.types.ts` - `ProviderAuthOverview.effective`
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ProviderAuthEffective {
    /// Auth kind: `"profiles"`, `"env"`, `"models.json"`, or `"missing"`.
    pub kind: String,
    /// Human-readable detail string.
    pub detail: String,
}

/// Summary of auth profiles for a provider.
///
/// Source: `src/commands/models/list.types.ts` - `ProviderAuthOverview.profiles`
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ProviderAuthProfileStats {
    /// Total number of profiles.
    pub count: usize,
    /// Number of OAuth profiles.
    pub oauth: usize,
    /// Number of token profiles.
    pub token: usize,
    /// Number of API key profiles.
    pub api_key: usize,
    /// Labels for display.
    pub labels: Vec<String>,
}

/// A named auth source with a value and source description.
///
/// Source: `src/commands/models/list.types.ts` - inline type
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ProviderAuthSource {
    /// Value description (e.g. masked API key).
    pub value: String,
    /// Source description (e.g. env var name).
    pub source: String,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn model_row_serializes_to_camel_case() {
        let row = ModelRow {
            key: "anthropic/claude-opus-4-6".to_owned(),
            name: "Claude Opus 4.6".to_owned(),
            input: "text+image".to_owned(),
            context_window: Some(200_000),
            local: Some(false),
            available: Some(true),
            tags: vec!["default".to_owned()],
            missing: false,
        };
        let json = serde_json::to_string(&row).unwrap_or_default();
        assert!(json.contains("\"contextWindow\""));
        assert!(!json.contains("\"context_window\""));
    }

    #[test]
    fn configured_entry_tags() {
        let mut tags = HashSet::new();
        tags.insert("default".to_owned());
        let entry = ConfiguredEntry {
            key: "anthropic/claude-opus-4-6".to_owned(),
            ref_provider: "anthropic".to_owned(),
            ref_model: "claude-opus-4-6".to_owned(),
            tags,
            aliases: vec![],
        };
        assert!(entry.tags.contains("default"));
    }
}
