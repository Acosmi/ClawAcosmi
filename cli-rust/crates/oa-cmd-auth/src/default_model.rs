/// Default model choice application logic.
///
/// When a user completes an auth flow, this module determines whether to
/// set the model as the global default or as an agent-specific override,
/// and applies the appropriate configuration changes.
///
/// Source: `src/commands/auth-choice.default-model.ts`

use std::collections::HashMap;

use oa_types::agent_defaults::{AgentDefaultsConfig, AgentModelEntryConfig, AgentModelListConfig};
use oa_types::agents::AgentsConfig;
use oa_types::config::OpenAcosmiConfig;

/// Result of applying a default model choice.
///
/// Source: `src/commands/auth-choice.default-model.ts` - return type of `applyDefaultModelChoice`
#[derive(Debug, Clone)]
#[allow(clippy::exhaustive_structs)]
pub struct DefaultModelResult {
    /// The updated configuration.
    pub config: OpenAcosmiConfig,
    /// If set, this model should be used as the agent-specific override
    /// rather than the global default.
    pub agent_model_override: Option<String>,
}

/// Apply a default model choice to the configuration.
///
/// When `set_default_model` is `true`, applies `apply_default_config` to
/// set the model as the global default and returns the result.
///
/// When `set_default_model` is `false`, applies `apply_provider_config`
/// and ensures the model is in the allowlist, returning it as an
/// agent-specific override.
///
/// Source: `src/commands/auth-choice.default-model.ts` - `applyDefaultModelChoice`
pub fn apply_default_model_choice(
    config: OpenAcosmiConfig,
    set_default_model: bool,
    default_model: &str,
    apply_default_config: impl FnOnce(OpenAcosmiConfig) -> OpenAcosmiConfig,
    apply_provider_config: impl FnOnce(OpenAcosmiConfig) -> OpenAcosmiConfig,
) -> DefaultModelResult {
    if set_default_model {
        let next = apply_default_config(config);
        return DefaultModelResult {
            config: next,
            agent_model_override: None,
        };
    }

    let next = apply_provider_config(config);
    let next_with_model = ensure_model_allowlist_entry(next, default_model);
    DefaultModelResult {
        config: next_with_model,
        agent_model_override: Some(default_model.to_owned()),
    }
}

/// Ensure a model reference exists in the `agents.defaults.models` allowlist.
///
/// If the model is not already present, adds an empty entry so that the
/// model is recognized by the allowlist checks.
///
/// Source: `src/commands/model-allowlist.ts` - `ensureModelAllowlistEntry`
#[must_use]
pub fn ensure_model_allowlist_entry(mut config: OpenAcosmiConfig, model_ref: &str) -> OpenAcosmiConfig {
    let agents = config.agents.get_or_insert_with(AgentsConfig::default);
    let defaults = agents.defaults.get_or_insert_with(AgentDefaultsConfig::default);
    let models = defaults.models.get_or_insert_with(HashMap::new);

    models
        .entry(model_ref.to_owned())
        .or_insert_with(AgentModelEntryConfig::default);

    config
}

/// Set the global default model primary to a specific model reference.
///
/// Overwrites `agents.defaults.model.primary` while preserving any
/// existing fallback list.
///
/// Source: `src/commands/auth-choice.apply.plugin-provider.ts` - `applyDefaultModel`
#[must_use]
pub fn set_default_model_primary(mut config: OpenAcosmiConfig, model: &str) -> OpenAcosmiConfig {
    let agents = config.agents.get_or_insert_with(AgentsConfig::default);
    let defaults = agents.defaults.get_or_insert_with(AgentDefaultsConfig::default);
    let model_cfg = defaults.model.get_or_insert_with(AgentModelListConfig::default);
    model_cfg.primary = Some(model.to_owned());

    // Ensure the model is in the allowlist
    let models = defaults.models.get_or_insert_with(HashMap::new);
    models
        .entry(model.to_owned())
        .or_insert_with(AgentModelEntryConfig::default);

    config
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn apply_default_model_sets_global() {
        let config = OpenAcosmiConfig::default();
        let result = apply_default_model_choice(
            config,
            true,
            "openai/gpt-4o",
            |mut cfg| {
                let agents = cfg.agents.get_or_insert_with(AgentsConfig::default);
                let defaults = agents.defaults.get_or_insert_with(AgentDefaultsConfig::default);
                let model = defaults.model.get_or_insert_with(AgentModelListConfig::default);
                model.primary = Some("openai/gpt-4o".to_owned());
                cfg
            },
            |cfg| cfg,
        );
        assert!(result.agent_model_override.is_none());
        let primary = result
            .config
            .agents
            .as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.model.as_ref())
            .and_then(|m| m.primary.as_deref());
        assert_eq!(primary, Some("openai/gpt-4o"));
    }

    #[test]
    fn apply_provider_config_returns_override() {
        let config = OpenAcosmiConfig::default();
        let result = apply_default_model_choice(
            config,
            false,
            "moonshot/kimi-k2.5",
            |cfg| cfg,
            |cfg| cfg,
        );
        assert_eq!(
            result.agent_model_override.as_deref(),
            Some("moonshot/kimi-k2.5")
        );
    }

    #[test]
    fn ensure_allowlist_entry_adds_model() {
        let config = OpenAcosmiConfig::default();
        let updated = ensure_model_allowlist_entry(config, "openai/gpt-4o");
        let models = updated
            .agents
            .as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.models.as_ref());
        assert!(models.is_some());
        assert!(models.unwrap_or(&HashMap::new()).contains_key("openai/gpt-4o"));
    }

    #[test]
    fn ensure_allowlist_entry_preserves_existing() {
        let mut models = HashMap::new();
        models.insert(
            "anthropic/claude-opus-4-6".to_owned(),
            AgentModelEntryConfig {
                alias: Some("opus".to_owned()),
                ..AgentModelEntryConfig::default()
            },
        );
        let config = OpenAcosmiConfig {
            agents: Some(AgentsConfig {
                defaults: Some(AgentDefaultsConfig {
                    models: Some(models),
                    ..AgentDefaultsConfig::default()
                }),
                list: None,
            }),
            ..OpenAcosmiConfig::default()
        };

        let updated = ensure_model_allowlist_entry(config, "openai/gpt-4o");
        let models = updated
            .agents
            .as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.models.as_ref())
            .expect("models map should exist");
        assert!(models.contains_key("anthropic/claude-opus-4-6"));
        assert!(models.contains_key("openai/gpt-4o"));
        assert_eq!(
            models["anthropic/claude-opus-4-6"].alias.as_deref(),
            Some("opus")
        );
    }

    #[test]
    fn set_default_model_primary_sets_primary() {
        let config = OpenAcosmiConfig::default();
        let updated = set_default_model_primary(config, "openai/gpt-4o");
        let primary = updated
            .agents
            .as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.model.as_ref())
            .and_then(|m| m.primary.as_deref());
        assert_eq!(primary, Some("openai/gpt-4o"));
    }

    #[test]
    fn set_default_model_primary_adds_to_allowlist() {
        let config = OpenAcosmiConfig::default();
        let updated = set_default_model_primary(config, "openai/gpt-4o");
        let models = updated
            .agents
            .as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.models.as_ref())
            .expect("models should exist");
        assert!(models.contains_key("openai/gpt-4o"));
    }
}
