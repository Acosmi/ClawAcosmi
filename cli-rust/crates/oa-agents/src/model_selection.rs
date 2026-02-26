/// Model selection and resolution logic.
///
/// Handles parsing model references (e.g. `"anthropic/claude-opus-4-6"`),
/// normalizing provider identifiers, resolving the default model for an agent,
/// and checking model allowlists.
///
/// Source: `src/agents/model-selection.ts`

use std::collections::{HashMap, HashSet};

use serde::{Deserialize, Serialize};

use oa_types::config::OpenAcosmiConfig;

use crate::defaults::{DEFAULT_MODEL, DEFAULT_PROVIDER};
use crate::model_catalog::ModelCatalogEntry;
use crate::providers::normalize_google_model_id;
use crate::scope::resolve_agent_model_primary;

// ── Types ──

/// A reference to a specific model, consisting of a provider and model ID.
///
/// Source: `src/agents/model-selection.ts` - `ModelRef`
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ModelRef {
    /// Provider identifier (e.g. `"anthropic"`, `"openai"`).
    pub provider: String,
    /// Model identifier (e.g. `"claude-opus-4-6"`, `"gpt-4o"`).
    pub model: String,
}

/// Thinking/reasoning level for models that support extended thinking.
///
/// Source: `src/agents/model-selection.ts` - `ThinkLevel`
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ThinkLevel {
    /// Thinking disabled.
    Off,
    /// Minimal thinking.
    Minimal,
    /// Low thinking budget.
    Low,
    /// Medium thinking budget.
    Medium,
    /// High thinking budget.
    High,
    /// Extra-high thinking budget.
    Xhigh,
}

/// An index mapping model aliases to their canonical [`ModelRef`] targets.
///
/// Source: `src/agents/model-selection.ts` - `ModelAliasIndex`
#[derive(Debug, Clone, Default)]
pub struct ModelAliasIndex {
    /// Maps normalized alias key -> (original alias, canonical ref).
    pub by_alias: HashMap<String, (String, ModelRef)>,
    /// Maps model key (`provider/model`) -> list of alias names.
    pub by_key: HashMap<String, Vec<String>>,
}

/// Status of a model reference against the configured allowlist.
///
/// Source: `src/agents/model-selection.ts` - `ModelRefStatus`
#[derive(Debug, Clone)]
pub struct ModelRefStatus {
    /// The canonical key (`provider/model`).
    pub key: String,
    /// Whether the model exists in the catalog.
    pub in_catalog: bool,
    /// Whether the config allows any model (no allowlist configured).
    pub allow_any: bool,
    /// Whether this specific model is allowed.
    pub allowed: bool,
}

/// Result of building the allowed model set from config.
///
/// Source: `src/agents/model-selection.ts` - return type of `buildAllowedModelSet`
#[derive(Debug, Clone)]
pub struct AllowedModelSet {
    /// Whether any model is allowed (no allowlist configured).
    pub allow_any: bool,
    /// Catalog entries that match the allowlist.
    pub allowed_catalog: Vec<ModelCatalogEntry>,
    /// Set of allowed model keys (`provider/model`).
    pub allowed_keys: HashSet<String>,
}

// ── Anthropic model aliases ──

/// Known short aliases for Anthropic models.
///
/// Source: `src/agents/model-selection.ts` - `ANTHROPIC_MODEL_ALIASES`
fn anthropic_model_aliases() -> HashMap<&'static str, &'static str> {
    let mut m = HashMap::new();
    m.insert("opus-4.6", "claude-opus-4-6");
    m.insert("opus-4.5", "claude-opus-4-5");
    m.insert("sonnet-4.5", "claude-sonnet-4-5");
    m
}

// ── Internal helpers ──

/// Normalize an alias key to lowercase, trimmed.
fn normalize_alias_key(value: &str) -> String {
    value.trim().to_lowercase()
}

/// Build the canonical model key from provider and model ID.
///
/// Source: `src/agents/model-selection.ts` - `modelKey`
pub fn model_key(provider: &str, model: &str) -> String {
    format!("{provider}/{model}")
}

/// Normalize an Anthropic model ID using known aliases.
///
/// Source: `src/agents/model-selection.ts` - `normalizeAnthropicModelId`
fn normalize_anthropic_model_id(model: &str) -> String {
    let trimmed = model.trim();
    if trimmed.is_empty() {
        return trimmed.to_owned();
    }
    let lower = trimmed.to_lowercase();
    let aliases = anthropic_model_aliases();
    aliases
        .get(lower.as_str())
        .map_or_else(|| trimmed.to_owned(), |canonical| (*canonical).to_owned())
}

/// Normalize a model ID based on its provider.
///
/// Source: `src/agents/model-selection.ts` - `normalizeProviderModelId`
fn normalize_provider_model_id(provider: &str, model: &str) -> String {
    if provider == "anthropic" {
        return normalize_anthropic_model_id(model);
    }
    if provider == "google" {
        return normalize_google_model_id(model);
    }
    model.to_owned()
}

// ── Public API ──

/// Normalize a provider identifier to its canonical form.
///
/// Handles known aliases and variant spellings:
/// - `"z.ai"` / `"z-ai"` -> `"zai"`
/// - `"opencode-zen"` -> `"opencode"`
/// - `"qwen"` -> `"qwen-portal"`
/// - `"kimi-code"` -> `"kimi-coding"`
///
/// Source: `src/agents/model-selection.ts` - `normalizeProviderId`
pub fn normalize_provider_id(provider: &str) -> String {
    let normalized = provider.trim().to_lowercase();
    match normalized.as_str() {
        "z.ai" | "z-ai" => "zai".to_owned(),
        "opencode-zen" => "opencode".to_owned(),
        "qwen" => "qwen-portal".to_owned(),
        "kimi-code" => "kimi-coding".to_owned(),
        _ => normalized,
    }
}

/// Check if a provider is a CLI-managed (external process) backend.
///
/// Returns `true` for built-in CLI providers (`claude-cli`, `codex-cli`) and
/// any user-configured CLI backends.
///
/// Source: `src/agents/model-selection.ts` - `isCliProvider`
pub fn is_cli_provider(provider: &str, cfg: Option<&OpenAcosmiConfig>) -> bool {
    let normalized = normalize_provider_id(provider);
    if normalized == "claude-cli" || normalized == "codex-cli" {
        return true;
    }
    if let Some(config) = cfg {
        if let Some(ref agents) = config.agents {
            if let Some(ref defaults) = agents.defaults {
                if let Some(ref backends) = defaults.cli_backends {
                    return backends
                        .keys()
                        .any(|key| normalize_provider_id(key) == normalized);
                }
            }
        }
    }
    false
}

/// Parse a raw model string into a [`ModelRef`].
///
/// Accepts formats:
/// - `"provider/model"` - explicit provider and model
/// - `"model"` - uses `default_provider` as the provider
///
/// Returns `None` if the input is empty or malformed.
///
/// Source: `src/agents/model-selection.ts` - `parseModelRef`
pub fn parse_model_ref(raw: &str, default_provider: &str) -> Option<ModelRef> {
    let trimmed = raw.trim();
    if trimmed.is_empty() {
        return None;
    }

    let slash = trimmed.find('/');
    match slash {
        None => {
            let provider = normalize_provider_id(default_provider);
            let model = normalize_provider_model_id(&provider, trimmed);
            Some(ModelRef { provider, model })
        }
        Some(idx) => {
            let provider_raw = trimmed[..idx].trim();
            let provider = normalize_provider_id(provider_raw);
            let model_raw = trimmed[idx + 1..].trim();
            if provider.is_empty() || model_raw.is_empty() {
                return None;
            }
            let model = normalize_provider_model_id(&provider, model_raw);
            Some(ModelRef { provider, model })
        }
    }
}

/// Resolve an allowlist model key from a raw string.
///
/// Parses the raw model reference and returns the canonical key, or `None`
/// if parsing fails.
///
/// Source: `src/agents/model-selection.ts` - `resolveAllowlistModelKey`
pub fn resolve_allowlist_model_key(raw: &str, default_provider: &str) -> Option<String> {
    let parsed = parse_model_ref(raw, default_provider)?;
    Some(model_key(&parsed.provider, &parsed.model))
}

/// Build the set of configured allowlist keys from config.
///
/// Returns `None` if no allowlist is configured (meaning all models allowed).
///
/// Source: `src/agents/model-selection.ts` - `buildConfiguredAllowlistKeys`
pub fn build_configured_allowlist_keys(
    cfg: Option<&OpenAcosmiConfig>,
    default_provider: &str,
) -> Option<HashSet<String>> {
    let models_map = cfg?
        .agents
        .as_ref()?
        .defaults
        .as_ref()?
        .models
        .as_ref()?;

    if models_map.is_empty() {
        return None;
    }

    let mut keys = HashSet::new();
    for raw in models_map.keys() {
        if let Some(key) = resolve_allowlist_model_key(raw, default_provider) {
            keys.insert(key);
        }
    }
    if keys.is_empty() { None } else { Some(keys) }
}

/// Build a model alias index from the configured models map.
///
/// The models map in `agents.defaults.models` can define aliases for models,
/// allowing users to reference models by short names.
///
/// Source: `src/agents/model-selection.ts` - `buildModelAliasIndex`
pub fn build_model_alias_index(cfg: &OpenAcosmiConfig, default_provider: &str) -> ModelAliasIndex {
    let mut index = ModelAliasIndex::default();

    let models_map = match cfg
        .agents
        .as_ref()
        .and_then(|a| a.defaults.as_ref())
        .and_then(|d| d.models.as_ref())
    {
        Some(m) => m,
        None => return index,
    };

    for (key_raw, entry) in models_map {
        let parsed = match parse_model_ref(key_raw, default_provider) {
            Some(p) => p,
            None => continue,
        };
        let alias = match &entry.alias {
            Some(a) => a.trim().to_owned(),
            None => continue,
        };
        if alias.is_empty() {
            continue;
        }

        let alias_key = normalize_alias_key(&alias);
        index
            .by_alias
            .insert(alias_key, (alias.clone(), parsed.clone()));

        let key = model_key(&parsed.provider, &parsed.model);
        index
            .by_key
            .entry(key)
            .or_default()
            .push(alias);
    }

    index
}

/// Resolve a model reference from a raw string, checking aliases first.
///
/// Returns the resolved [`ModelRef`] and optional alias name if matched.
///
/// Source: `src/agents/model-selection.ts` - `resolveModelRefFromString`
pub fn resolve_model_ref_from_string(
    raw: &str,
    default_provider: &str,
    alias_index: Option<&ModelAliasIndex>,
) -> Option<(ModelRef, Option<String>)> {
    let trimmed = raw.trim();
    if trimmed.is_empty() {
        return None;
    }

    // Check alias if no slash is present
    if !trimmed.contains('/') {
        let alias_key = normalize_alias_key(trimmed);
        if let Some(index) = alias_index {
            if let Some((alias, model_ref)) = index.by_alias.get(&alias_key) {
                return Some((model_ref.clone(), Some(alias.clone())));
            }
        }
    }

    let parsed = parse_model_ref(trimmed, default_provider)?;
    Some((parsed, None))
}

/// Resolve the configured default model reference from the config.
///
/// Checks `agents.defaults.model.primary` (or string form), resolves aliases,
/// and falls back to `default_provider/default_model`.
///
/// Source: `src/agents/model-selection.ts` - `resolveConfiguredModelRef`
pub fn resolve_configured_model_ref(
    cfg: &OpenAcosmiConfig,
    default_provider: &str,
    default_model: &str,
) -> ModelRef {
    let raw_model = extract_defaults_model_primary(cfg);

    if let Some(raw) = raw_model {
        let trimmed = raw.trim().to_owned();
        if !trimmed.is_empty() {
            let alias_index = build_model_alias_index(cfg, default_provider);

            if !trimmed.contains('/') {
                let alias_key = normalize_alias_key(&trimmed);
                if let Some((_alias, model_ref)) = alias_index.by_alias.get(&alias_key) {
                    return model_ref.clone();
                }

                tracing::warn!(
                    model = %trimmed,
                    "Model specified without provider. Falling back to \"anthropic/{trimmed}\". \
                     Please use \"anthropic/{trimmed}\" in your config."
                );
                return ModelRef {
                    provider: "anthropic".to_owned(),
                    model: trimmed,
                };
            }

            if let Some((model_ref, _alias)) =
                resolve_model_ref_from_string(&trimmed, default_provider, Some(&alias_index))
            {
                return model_ref;
            }
        }
    }

    ModelRef {
        provider: default_provider.to_owned(),
        model: default_model.to_owned(),
    }
}

/// Resolve the default model for a specific agent.
///
/// Checks agent-specific model overrides first, then falls back to the
/// global default model configuration.
///
/// Source: `src/agents/model-selection.ts` - `resolveDefaultModelForAgent`
pub fn resolve_default_model_for_agent(cfg: &OpenAcosmiConfig, agent_id: Option<&str>) -> ModelRef {
    let agent_model_override = agent_id.and_then(|id| resolve_agent_model_primary(cfg, id));

    if let Some(ref override_model) = agent_model_override {
        if !override_model.is_empty() {
            // Build a synthetic config with the agent's model as the defaults primary
            let mut synthetic_cfg = cfg.clone();
            let agents = synthetic_cfg.agents.get_or_insert_with(Default::default);
            let defaults = agents.defaults.get_or_insert_with(Default::default);
            let model = defaults.model.get_or_insert_with(Default::default);
            model.primary = Some(override_model.clone());

            return resolve_configured_model_ref(&synthetic_cfg, DEFAULT_PROVIDER, DEFAULT_MODEL);
        }
    }

    resolve_configured_model_ref(cfg, DEFAULT_PROVIDER, DEFAULT_MODEL)
}

/// Build the set of allowed models from config and catalog.
///
/// If no allowlist is configured (`agents.defaults.models` is empty),
/// all catalog models are allowed. Otherwise only models in the allowlist
/// (and the default model) are permitted.
///
/// Source: `src/agents/model-selection.ts` - `buildAllowedModelSet`
pub fn build_allowed_model_set(
    cfg: &OpenAcosmiConfig,
    catalog: &[ModelCatalogEntry],
    default_provider: &str,
    default_model: Option<&str>,
) -> AllowedModelSet {
    let raw_allowlist: Vec<String> = cfg
        .agents
        .as_ref()
        .and_then(|a| a.defaults.as_ref())
        .and_then(|d| d.models.as_ref())
        .map(|m| m.keys().cloned().collect())
        .unwrap_or_default();

    let allow_any = raw_allowlist.is_empty();
    let default_model_trimmed = default_model.map(|m| m.trim()).filter(|m| !m.is_empty());
    let default_key = default_model_trimmed
        .filter(|_| !default_provider.is_empty())
        .map(|m| model_key(default_provider, m));

    let catalog_keys: HashSet<String> = catalog
        .iter()
        .map(|entry| model_key(&entry.provider, &entry.id))
        .collect();

    if allow_any {
        let mut all_keys = catalog_keys;
        if let Some(ref dk) = default_key {
            all_keys.insert(dk.clone());
        }
        return AllowedModelSet {
            allow_any: true,
            allowed_catalog: catalog.to_vec(),
            allowed_keys: all_keys,
        };
    }

    let configured_providers = cfg
        .models
        .as_ref()
        .and_then(|m| m.providers.as_ref());

    let mut allowed_keys = HashSet::new();
    for raw in &raw_allowlist {
        let parsed = match parse_model_ref(raw, default_provider) {
            Some(p) => p,
            None => continue,
        };
        let key = model_key(&parsed.provider, &parsed.model);
        let provider_key = normalize_provider_id(&parsed.provider);

        if is_cli_provider(&parsed.provider, Some(cfg)) {
            allowed_keys.insert(key);
        } else if catalog_keys.contains(&key) {
            allowed_keys.insert(key);
        } else if configured_providers
            .is_some_and(|p| p.contains_key(&provider_key))
        {
            allowed_keys.insert(key);
        }
    }

    if let Some(ref dk) = default_key {
        allowed_keys.insert(dk.clone());
    }

    let allowed_catalog: Vec<ModelCatalogEntry> = catalog
        .iter()
        .filter(|entry| allowed_keys.contains(&model_key(&entry.provider, &entry.id)))
        .cloned()
        .collect();

    // If the allowlist resolved to nothing, fall back to allowing everything
    if allowed_catalog.is_empty() && allowed_keys.is_empty() {
        let mut all_keys = catalog_keys;
        if let Some(ref dk) = default_key {
            all_keys.insert(dk.clone());
        }
        return AllowedModelSet {
            allow_any: true,
            allowed_catalog: catalog.to_vec(),
            allowed_keys: all_keys,
        };
    }

    AllowedModelSet {
        allow_any: false,
        allowed_catalog,
        allowed_keys,
    }
}

/// Get the allowlist status for a model reference.
///
/// Source: `src/agents/model-selection.ts` - `getModelRefStatus`
pub fn get_model_ref_status(
    cfg: &OpenAcosmiConfig,
    catalog: &[ModelCatalogEntry],
    model_ref: &ModelRef,
    default_provider: &str,
    default_model: Option<&str>,
) -> ModelRefStatus {
    let allowed = build_allowed_model_set(cfg, catalog, default_provider, default_model);
    let key = model_key(&model_ref.provider, &model_ref.model);
    let in_catalog = catalog
        .iter()
        .any(|entry| model_key(&entry.provider, &entry.id) == key);

    ModelRefStatus {
        key: key.clone(),
        in_catalog,
        allow_any: allowed.allow_any,
        allowed: allowed.allow_any || allowed.allowed_keys.contains(&key),
    }
}

/// Resolve and validate a model reference from a raw string against the allowlist.
///
/// Returns `Ok((ModelRef, key))` if the model is valid and allowed, or
/// `Err(message)` if the model is invalid or not permitted.
///
/// Source: `src/agents/model-selection.ts` - `resolveAllowedModelRef`
pub fn resolve_allowed_model_ref(
    cfg: &OpenAcosmiConfig,
    catalog: &[ModelCatalogEntry],
    raw: &str,
    default_provider: &str,
    default_model: Option<&str>,
) -> Result<(ModelRef, String), String> {
    let trimmed = raw.trim();
    if trimmed.is_empty() {
        return Err("invalid model: empty".to_owned());
    }

    let alias_index = build_model_alias_index(cfg, default_provider);
    let resolved = resolve_model_ref_from_string(trimmed, default_provider, Some(&alias_index));
    let (model_ref, _alias) = match resolved {
        Some(r) => r,
        None => return Err(format!("invalid model: {trimmed}")),
    };

    let status = get_model_ref_status(cfg, catalog, &model_ref, default_provider, default_model);
    if !status.allowed {
        return Err(format!("model not allowed: {}", status.key));
    }

    Ok((model_ref, status.key))
}

/// Resolve the default thinking level for a model.
///
/// Checks `agents.defaults.thinkingDefault` in config first; if not set,
/// checks whether the model is marked as a reasoning model in the catalog
/// (returns `Low` for reasoning models, `Off` otherwise).
///
/// Source: `src/agents/model-selection.ts` - `resolveThinkingDefault`
pub fn resolve_thinking_default(
    cfg: &OpenAcosmiConfig,
    provider: &str,
    model: &str,
    catalog: Option<&[ModelCatalogEntry]>,
) -> ThinkLevel {
    // Check explicit config
    if let Some(configured) = cfg
        .agents
        .as_ref()
        .and_then(|a| a.defaults.as_ref())
        .and_then(|d| d.thinking_default.as_ref())
    {
        return match configured {
            oa_types::agent_defaults::ThinkingDefault::Off => ThinkLevel::Off,
            oa_types::agent_defaults::ThinkingDefault::Minimal => ThinkLevel::Minimal,
            oa_types::agent_defaults::ThinkingDefault::Low => ThinkLevel::Low,
            oa_types::agent_defaults::ThinkingDefault::Medium => ThinkLevel::Medium,
            oa_types::agent_defaults::ThinkingDefault::High => ThinkLevel::High,
            oa_types::agent_defaults::ThinkingDefault::Xhigh => ThinkLevel::Xhigh,
        };
    }

    // Check catalog for reasoning flag
    if let Some(catalog) = catalog {
        let candidate = catalog
            .iter()
            .find(|entry| entry.provider == provider && entry.id == model);
        if let Some(entry) = candidate {
            if entry.reasoning.unwrap_or(false) {
                return ThinkLevel::Low;
            }
        }
    }

    ThinkLevel::Off
}

// ── Internal helpers ──

/// Extract the `agents.defaults.model` primary string from config.
///
/// Handles both the string form and the object form with a `primary` field.
fn extract_defaults_model_primary(cfg: &OpenAcosmiConfig) -> Option<String> {
    let model_cfg = cfg
        .agents
        .as_ref()?
        .defaults
        .as_ref()?
        .model
        .as_ref()?;

    let primary = model_cfg.primary.as_deref()?.trim();
    if primary.is_empty() {
        None
    } else {
        Some(primary.to_owned())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use oa_types::agent_defaults::{AgentDefaultsConfig, AgentModelEntryConfig, AgentModelListConfig};
    use oa_types::agents::AgentsConfig;
    use oa_types::models::ModelInputType;
    use std::collections::HashMap;

    fn make_catalog() -> Vec<ModelCatalogEntry> {
        vec![
            ModelCatalogEntry {
                id: "claude-opus-4-6".to_owned(),
                name: "Claude Opus 4.6".to_owned(),
                provider: "anthropic".to_owned(),
                context_window: Some(200_000),
                reasoning: Some(true),
                input: Some(vec![ModelInputType::Text, ModelInputType::Image]),
            },
            ModelCatalogEntry {
                id: "claude-sonnet-4-5".to_owned(),
                name: "Claude Sonnet 4.5".to_owned(),
                provider: "anthropic".to_owned(),
                context_window: Some(200_000),
                reasoning: Some(false),
                input: Some(vec![ModelInputType::Text, ModelInputType::Image]),
            },
            ModelCatalogEntry {
                id: "gpt-4o".to_owned(),
                name: "GPT-4o".to_owned(),
                provider: "openai".to_owned(),
                context_window: Some(128_000),
                reasoning: Some(false),
                input: Some(vec![ModelInputType::Text, ModelInputType::Image]),
            },
        ]
    }

    // ── normalize_provider_id ──

    #[test]
    fn normalize_provider_basic() {
        assert_eq!(normalize_provider_id("Anthropic"), "anthropic");
        assert_eq!(normalize_provider_id("OpenAI"), "openai");
    }

    #[test]
    fn normalize_provider_aliases() {
        assert_eq!(normalize_provider_id("z.ai"), "zai");
        assert_eq!(normalize_provider_id("z-ai"), "zai");
        assert_eq!(normalize_provider_id("opencode-zen"), "opencode");
        assert_eq!(normalize_provider_id("qwen"), "qwen-portal");
        assert_eq!(normalize_provider_id("kimi-code"), "kimi-coding");
    }

    // ── is_cli_provider ──

    #[test]
    fn cli_provider_builtins() {
        assert!(is_cli_provider("claude-cli", None));
        assert!(is_cli_provider("codex-cli", None));
        assert!(!is_cli_provider("anthropic", None));
    }

    // ── parse_model_ref ──

    #[test]
    fn parse_model_ref_with_provider() {
        let result = parse_model_ref("openai/gpt-4o", "anthropic");
        assert_eq!(
            result,
            Some(ModelRef {
                provider: "openai".to_owned(),
                model: "gpt-4o".to_owned(),
            })
        );
    }

    #[test]
    fn parse_model_ref_without_provider() {
        let result = parse_model_ref("claude-opus-4-6", "anthropic");
        assert_eq!(
            result,
            Some(ModelRef {
                provider: "anthropic".to_owned(),
                model: "claude-opus-4-6".to_owned(),
            })
        );
    }

    #[test]
    fn parse_model_ref_normalizes_alias() {
        let result = parse_model_ref("opus-4.6", "anthropic");
        assert_eq!(
            result,
            Some(ModelRef {
                provider: "anthropic".to_owned(),
                model: "claude-opus-4-6".to_owned(),
            })
        );
    }

    #[test]
    fn parse_model_ref_normalizes_google() {
        let result = parse_model_ref("google/gemini-3-pro", "anthropic");
        assert_eq!(
            result,
            Some(ModelRef {
                provider: "google".to_owned(),
                model: "gemini-3-pro-preview".to_owned(),
            })
        );
    }

    #[test]
    fn parse_model_ref_empty() {
        assert!(parse_model_ref("", "anthropic").is_none());
        assert!(parse_model_ref("  ", "anthropic").is_none());
    }

    #[test]
    fn parse_model_ref_empty_parts() {
        assert!(parse_model_ref("/model", "anthropic").is_none());
        assert!(parse_model_ref("provider/", "anthropic").is_none());
    }

    // ── model_key ──

    #[test]
    fn model_key_format() {
        assert_eq!(model_key("anthropic", "claude-opus-4-6"), "anthropic/claude-opus-4-6");
    }

    // ── resolve_configured_model_ref ──

    #[test]
    fn configured_model_ref_fallback() {
        let cfg = OpenAcosmiConfig::default();
        let result = resolve_configured_model_ref(&cfg, "anthropic", "claude-opus-4-6");
        assert_eq!(result.provider, "anthropic");
        assert_eq!(result.model, "claude-opus-4-6");
    }

    #[test]
    fn configured_model_ref_from_config() {
        let cfg = OpenAcosmiConfig {
            agents: Some(AgentsConfig {
                defaults: Some(AgentDefaultsConfig {
                    model: Some(AgentModelListConfig {
                        primary: Some("openai/gpt-4o".to_owned()),
                        fallbacks: None,
                    }),
                    ..Default::default()
                }),
                list: None,
            }),
            ..Default::default()
        };
        let result = resolve_configured_model_ref(&cfg, "anthropic", "claude-opus-4-6");
        assert_eq!(result.provider, "openai");
        assert_eq!(result.model, "gpt-4o");
    }

    // ── resolve_default_model_for_agent ──

    #[test]
    fn default_model_no_agent() {
        let cfg = OpenAcosmiConfig::default();
        let result = resolve_default_model_for_agent(&cfg, None);
        assert_eq!(result.provider, DEFAULT_PROVIDER);
        assert_eq!(result.model, DEFAULT_MODEL);
    }

    // ── build_allowed_model_set ──

    #[test]
    fn allowed_model_set_no_allowlist() {
        let cfg = OpenAcosmiConfig::default();
        let catalog = make_catalog();
        let result = build_allowed_model_set(&cfg, &catalog, "anthropic", None);
        assert!(result.allow_any);
        assert_eq!(result.allowed_catalog.len(), 3);
    }

    #[test]
    fn allowed_model_set_with_allowlist() {
        let mut models = HashMap::new();
        models.insert(
            "anthropic/claude-opus-4-6".to_owned(),
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
        let catalog = make_catalog();
        let result = build_allowed_model_set(&cfg, &catalog, "anthropic", None);
        assert!(!result.allow_any);
        assert_eq!(result.allowed_catalog.len(), 1);
        assert_eq!(result.allowed_catalog[0].id, "claude-opus-4-6");
    }

    // ── get_model_ref_status ──

    #[test]
    fn model_ref_status_allowed_no_list() {
        let cfg = OpenAcosmiConfig::default();
        let catalog = make_catalog();
        let model_ref = ModelRef {
            provider: "anthropic".to_owned(),
            model: "claude-opus-4-6".to_owned(),
        };
        let status = get_model_ref_status(&cfg, &catalog, &model_ref, "anthropic", None);
        assert!(status.allowed);
        assert!(status.allow_any);
        assert!(status.in_catalog);
    }

    #[test]
    fn model_ref_status_not_in_catalog() {
        let cfg = OpenAcosmiConfig::default();
        let catalog = make_catalog();
        let model_ref = ModelRef {
            provider: "unknown".to_owned(),
            model: "mystery-model".to_owned(),
        };
        let status = get_model_ref_status(&cfg, &catalog, &model_ref, "anthropic", None);
        assert!(status.allowed); // No allowlist, so everything is allowed
        assert!(!status.in_catalog);
    }

    // ── resolve_allowed_model_ref ──

    #[test]
    fn resolve_allowed_empty() {
        let cfg = OpenAcosmiConfig::default();
        let catalog = make_catalog();
        let result = resolve_allowed_model_ref(&cfg, &catalog, "", "anthropic", None);
        assert!(result.is_err());
    }

    #[test]
    fn resolve_allowed_valid() {
        let cfg = OpenAcosmiConfig::default();
        let catalog = make_catalog();
        let result =
            resolve_allowed_model_ref(&cfg, &catalog, "anthropic/claude-opus-4-6", "anthropic", None);
        assert!(result.is_ok());
        let (model_ref, key) = result.expect("should be ok");
        assert_eq!(model_ref.provider, "anthropic");
        assert_eq!(model_ref.model, "claude-opus-4-6");
        assert_eq!(key, "anthropic/claude-opus-4-6");
    }

    // ── resolve_thinking_default ──

    #[test]
    fn thinking_default_no_config() {
        let cfg = OpenAcosmiConfig::default();
        let catalog = make_catalog();
        let result = resolve_thinking_default(&cfg, "anthropic", "claude-opus-4-6", Some(&catalog));
        assert_eq!(result, ThinkLevel::Low); // opus is a reasoning model
    }

    #[test]
    fn thinking_default_non_reasoning() {
        let cfg = OpenAcosmiConfig::default();
        let catalog = make_catalog();
        let result = resolve_thinking_default(&cfg, "openai", "gpt-4o", Some(&catalog));
        assert_eq!(result, ThinkLevel::Off);
    }

    #[test]
    fn thinking_default_from_config() {
        let cfg = OpenAcosmiConfig {
            agents: Some(AgentsConfig {
                defaults: Some(AgentDefaultsConfig {
                    thinking_default: Some(oa_types::agent_defaults::ThinkingDefault::High),
                    ..Default::default()
                }),
                list: None,
            }),
            ..Default::default()
        };
        let result = resolve_thinking_default(&cfg, "anthropic", "claude-opus-4-6", None);
        assert_eq!(result, ThinkLevel::High);
    }

    // ── build_model_alias_index ──

    #[test]
    fn alias_index_empty() {
        let cfg = OpenAcosmiConfig::default();
        let index = build_model_alias_index(&cfg, "anthropic");
        assert!(index.by_alias.is_empty());
        assert!(index.by_key.is_empty());
    }

    #[test]
    fn alias_index_with_aliases() {
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
        let index = build_model_alias_index(&cfg, "anthropic");
        assert!(index.by_alias.contains_key("opus"));
        let (alias, model_ref) = &index.by_alias["opus"];
        assert_eq!(alias, "opus");
        assert_eq!(model_ref.provider, "anthropic");
        assert_eq!(model_ref.model, "claude-opus-4-6");
    }

    // ── resolve_model_ref_from_string ──

    #[test]
    fn resolve_ref_from_alias() {
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
        let index = build_model_alias_index(&cfg, "anthropic");
        let result = resolve_model_ref_from_string("opus", "anthropic", Some(&index));
        assert!(result.is_some());
        let (model_ref, alias) = result.expect("should resolve");
        assert_eq!(model_ref.model, "claude-opus-4-6");
        assert_eq!(alias, Some("opus".to_owned()));
    }

    #[test]
    fn resolve_ref_from_explicit() {
        let result = resolve_model_ref_from_string("openai/gpt-4o", "anthropic", None);
        assert!(result.is_some());
        let (model_ref, alias) = result.expect("should resolve");
        assert_eq!(model_ref.provider, "openai");
        assert_eq!(model_ref.model, "gpt-4o");
        assert!(alias.is_none());
    }
}
