/// Core provider config application functions.
///
/// Each function merges provider-specific settings (models, aliases, providers)
/// into an existing `OpenAcosmiConfig`, following the pattern:
/// 1. `apply<Provider>ProviderConfig` - registers provider without changing default model
/// 2. `apply<Provider>Config` - registers provider AND sets it as the default model
///
/// Source: `src/commands/onboard-auth.config-core.ts`

use std::collections::HashMap;

use oa_types::agent_defaults::{AgentModelEntryConfig, AgentModelListConfig};
use oa_types::agents::AgentsConfig;
use oa_types::agent_defaults::AgentDefaultsConfig;
use oa_types::config::OpenAcosmiConfig;

use super::credentials::{
    OPENROUTER_DEFAULT_MODEL_REF, XAI_DEFAULT_MODEL_REF,
    ZAI_DEFAULT_MODEL_REF,
};

/// Set the primary model reference in the agent defaults, preserving fallbacks.
///
/// Source: `src/commands/onboard-auth.config-core.ts` - shared pattern
fn set_primary_model(cfg: OpenAcosmiConfig, model_ref: &str) -> OpenAcosmiConfig {
    let agents = cfg.agents.unwrap_or_default();
    let defaults = agents.defaults.unwrap_or_default();

    let fallbacks = defaults
        .model
        .as_ref()
        .and_then(|m| m.fallbacks.clone());

    let model = AgentModelListConfig {
        primary: Some(model_ref.to_string()),
        fallbacks,
    };

    OpenAcosmiConfig {
        agents: Some(AgentsConfig {
            defaults: Some(AgentDefaultsConfig {
                model: Some(model),
                ..defaults
            }),
            ..agents
        }),
        ..cfg
    }
}

/// Set a model alias in the agent defaults models map.
///
/// Source: `src/commands/onboard-auth.config-core.ts` - shared pattern
fn set_model_alias(cfg: &OpenAcosmiConfig, model_ref: &str, alias: &str) -> HashMap<String, AgentModelEntryConfig> {
    let mut models = cfg
        .agents
        .as_ref()
        .and_then(|a| a.defaults.as_ref())
        .and_then(|d| d.models.clone())
        .unwrap_or_default();

    let existing = models.get(model_ref).cloned().unwrap_or_default();
    let entry = AgentModelEntryConfig {
        alias: existing.alias.or_else(|| Some(alias.to_string())),
        ..existing
    };
    models.insert(model_ref.to_string(), entry);
    models
}

/// Apply ZAI provider config: sets model alias and primary model.
///
/// Source: `src/commands/onboard-auth.config-core.ts` - `applyZaiConfig`
pub fn apply_zai_config(cfg: OpenAcosmiConfig) -> OpenAcosmiConfig {
    let models = set_model_alias(&cfg, ZAI_DEFAULT_MODEL_REF, "GLM");
    let agents = cfg.agents.unwrap_or_default();
    let defaults = agents.defaults.unwrap_or_default();

    let intermediate = OpenAcosmiConfig {
        agents: Some(AgentsConfig {
            defaults: Some(AgentDefaultsConfig {
                models: Some(models),
                ..defaults
            }),
            ..agents
        }),
        ..cfg
    };

    set_primary_model(intermediate, ZAI_DEFAULT_MODEL_REF)
}

/// Apply OpenRouter provider config without changing default model.
///
/// Source: `src/commands/onboard-auth.config-core.ts` - `applyOpenrouterProviderConfig`
pub fn apply_openrouter_provider_config(cfg: OpenAcosmiConfig) -> OpenAcosmiConfig {
    let models = set_model_alias(&cfg, OPENROUTER_DEFAULT_MODEL_REF, "OpenRouter");
    let agents = cfg.agents.unwrap_or_default();
    let defaults = agents.defaults.unwrap_or_default();

    OpenAcosmiConfig {
        agents: Some(AgentsConfig {
            defaults: Some(AgentDefaultsConfig {
                models: Some(models),
                ..defaults
            }),
            ..agents
        }),
        ..cfg
    }
}

/// Apply OpenRouter config: provider config + set as default model.
///
/// Source: `src/commands/onboard-auth.config-core.ts` - `applyOpenrouterConfig`
pub fn apply_openrouter_config(cfg: OpenAcosmiConfig) -> OpenAcosmiConfig {
    let next = apply_openrouter_provider_config(cfg);
    set_primary_model(next, OPENROUTER_DEFAULT_MODEL_REF)
}

/// Apply xAI provider config without changing default model.
///
/// Source: `src/commands/onboard-auth.config-core.ts` - `applyXaiProviderConfig`
pub fn apply_xai_provider_config(cfg: OpenAcosmiConfig) -> OpenAcosmiConfig {
    let models = set_model_alias(&cfg, XAI_DEFAULT_MODEL_REF, "Grok");
    let agents = cfg.agents.unwrap_or_default();
    let defaults = agents.defaults.unwrap_or_default();

    OpenAcosmiConfig {
        agents: Some(AgentsConfig {
            defaults: Some(AgentDefaultsConfig {
                models: Some(models),
                ..defaults
            }),
            ..agents
        }),
        ..cfg
    }
}

/// Apply xAI config: provider config + set as default model.
///
/// Source: `src/commands/onboard-auth.config-core.ts` - `applyXaiConfig`
pub fn apply_xai_config(cfg: OpenAcosmiConfig) -> OpenAcosmiConfig {
    let next = apply_xai_provider_config(cfg);
    set_primary_model(next, XAI_DEFAULT_MODEL_REF)
}

/// Apply auth profile config to the auth section of the config.
///
/// Source: `src/commands/onboard-auth.config-core.ts` - `applyAuthProfileConfig`
pub fn apply_auth_profile_config(
    cfg: OpenAcosmiConfig,
    profile_id: &str,
    provider: &str,
    mode: &str,
    email: Option<&str>,
) -> OpenAcosmiConfig {
    use oa_types::auth::{AuthProfileConfig, AuthProfileMode};

    let auth_mode = match mode {
        "api_key" => AuthProfileMode::ApiKey,
        "oauth" => AuthProfileMode::Oauth,
        "token" => AuthProfileMode::Token,
        _ => AuthProfileMode::ApiKey,
    };

    let profile = AuthProfileConfig {
        provider: provider.to_string(),
        mode: auth_mode,
        email: email.map(str::to_string),
    };

    let mut auth = cfg.auth.unwrap_or_default();
    let mut profiles = auth.profiles.unwrap_or_default();
    profiles.insert(profile_id.to_string(), profile);
    auth.profiles = Some(profiles);

    OpenAcosmiConfig {
        auth: Some(auth),
        ..cfg
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn apply_zai_config_sets_primary_model() {
        let cfg = OpenAcosmiConfig::default();
        let result = apply_zai_config(cfg);
        let primary = result
            .agents
            .as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.model.as_ref())
            .and_then(|m| m.primary.as_deref());
        assert_eq!(primary, Some(ZAI_DEFAULT_MODEL_REF));
    }

    #[test]
    fn apply_zai_config_sets_alias() {
        let cfg = OpenAcosmiConfig::default();
        let result = apply_zai_config(cfg);
        let models = result
            .agents
            .as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.models.as_ref());
        assert!(models.is_some());
        let entry = models
            .and_then(|m| m.get(ZAI_DEFAULT_MODEL_REF));
        assert_eq!(entry.and_then(|e| e.alias.as_deref()), Some("GLM"));
    }

    #[test]
    fn apply_openrouter_provider_does_not_set_primary() {
        let cfg = OpenAcosmiConfig::default();
        let result = apply_openrouter_provider_config(cfg);
        let primary = result
            .agents
            .as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.model.as_ref())
            .and_then(|m| m.primary.as_deref());
        assert!(primary.is_none());
    }

    #[test]
    fn apply_openrouter_config_sets_primary() {
        let cfg = OpenAcosmiConfig::default();
        let result = apply_openrouter_config(cfg);
        let primary = result
            .agents
            .as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.model.as_ref())
            .and_then(|m| m.primary.as_deref());
        assert_eq!(primary, Some(OPENROUTER_DEFAULT_MODEL_REF));
    }

    #[test]
    fn apply_xai_config_sets_grok_alias() {
        let cfg = OpenAcosmiConfig::default();
        let result = apply_xai_config(cfg);
        let models = result
            .agents
            .as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.models.as_ref());
        let entry = models.and_then(|m| m.get(XAI_DEFAULT_MODEL_REF));
        assert_eq!(entry.and_then(|e| e.alias.as_deref()), Some("Grok"));
    }

    #[test]
    fn apply_auth_profile_config_adds_profile() {
        let cfg = OpenAcosmiConfig::default();
        let result = apply_auth_profile_config(
            cfg,
            "anthropic:default",
            "anthropic",
            "api_key",
            None,
        );
        let profiles = result
            .auth
            .as_ref()
            .and_then(|a| a.profiles.as_ref());
        assert!(profiles.is_some());
        assert!(profiles
            .expect("profiles")
            .contains_key("anthropic:default"));
    }

    #[test]
    fn set_primary_model_preserves_fallbacks() {
        let mut cfg = OpenAcosmiConfig::default();
        cfg.agents = Some(AgentsConfig {
            defaults: Some(AgentDefaultsConfig {
                model: Some(AgentModelListConfig {
                    primary: Some("old/model".to_string()),
                    fallbacks: Some(vec!["fallback/model".to_string()]),
                }),
                ..Default::default()
            }),
            ..Default::default()
        });

        let result = set_primary_model(cfg, "new/model");
        let model = result
            .agents
            .as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.model.as_ref())
            .expect("model config");
        assert_eq!(model.primary.as_deref(), Some("new/model"));
        assert_eq!(
            model.fallbacks.as_deref(),
            Some(vec!["fallback/model".to_string()].as_slice())
        );
    }
}
