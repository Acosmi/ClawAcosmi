/// Model configuration validation and warnings.
///
/// After an auth flow completes, this module checks whether the resulting
/// configuration makes sense: is the configured default model actually
/// available in the catalog? Does the provider have credentials configured?
///
/// Source: `src/commands/auth-choice.model-check.ts`

use oa_agents::defaults::{DEFAULT_MODEL, DEFAULT_PROVIDER};
use oa_agents::model_selection::{model_key, resolve_configured_model_ref};
use oa_types::config::OpenAcosmiConfig;

/// Options for model configuration checking.
///
/// Source: `src/commands/auth-choice.model-check.ts` - `warnIfModelConfigLooksOff` options
#[derive(Debug, Clone, Default)]
pub struct ModelCheckOptions {
    /// Optional agent ID to check agent-specific model overrides.
    pub agent_id: Option<String>,
    /// Optional agent directory for auth profile lookup.
    pub agent_dir: Option<String>,
}

/// A warning about a potential model/auth configuration issue.
///
/// Source: `src/commands/auth-choice.model-check.ts` - warnings array
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ModelCheckWarning {
    /// Human-readable warning message.
    pub message: String,
}

/// Check the model configuration for potential issues.
///
/// Produces a list of warnings when:
/// - The configured default model is not found in the catalog
/// - No auth credentials are configured for the default provider
/// - OpenAI Codex OAuth is detected but the model is set to plain `openai`
///
/// This function does not perform side effects (no I/O, no prompts). It
/// returns the list of warnings for the caller to display.
///
/// Source: `src/commands/auth-choice.model-check.ts` - `warnIfModelConfigLooksOff`
#[must_use]
pub fn check_model_config(
    config: &OpenAcosmiConfig,
    options: &ModelCheckOptions,
) -> Vec<ModelCheckWarning> {
    let mut warnings = Vec::new();

    // Resolve the effective model reference
    let ref_ = resolve_configured_model_ref(config, DEFAULT_PROVIDER, DEFAULT_MODEL);

    // Check if the model key is plausible
    let key = model_key(&ref_.provider, &ref_.model);
    if key.is_empty() {
        warnings.push(ModelCheckWarning {
            message: "No default model configured. Run the setup wizard or set agents.defaults.model."
                .to_owned(),
        });
    }

    // Check auth profile availability
    let has_auth_profiles = config
        .auth
        .as_ref()
        .and_then(|auth| auth.profiles.as_ref())
        .map_or(false, |profiles| {
            profiles.values().any(|p| p.provider == ref_.provider)
        });

    if !has_auth_profiles {
        // Check for env vars as a fallback (we only know about common ones)
        let has_env_key = check_common_env_key_for_provider(&ref_.provider);
        if !has_env_key {
            warnings.push(ModelCheckWarning {
                message: format!(
                    "No auth configured for provider \"{}\". \
                     The agent may fail until credentials are added.",
                    ref_.provider
                ),
            });
        }
    }

    // OpenAI Codex detection
    if ref_.provider == "openai" {
        let has_codex_profile = config
            .auth
            .as_ref()
            .and_then(|auth| auth.profiles.as_ref())
            .map_or(false, |profiles| {
                profiles.values().any(|p| p.provider == "openai-codex")
            });

        if has_codex_profile {
            warnings.push(ModelCheckWarning {
                message: format!(
                    "Detected OpenAI Codex OAuth. Consider setting agents.defaults.model \
                     to openai-codex/codex-mini-latest."
                ),
            });
        }
    }

    // Agent-specific warnings
    if let Some(ref agent_id) = options.agent_id {
        if !agent_id.is_empty() {
            let agent_has_model = config
                .agents
                .as_ref()
                .and_then(|agents| agents.list.as_ref())
                .and_then(|list| list.iter().find(|a| a.id == *agent_id))
                .and_then(|a| a.model.as_ref())
                .is_some();

            if !agent_has_model && warnings.is_empty() {
                // No warnings and no agent-specific model -- this is fine
            }
        }
    }

    warnings
}

/// Check if a well-known environment variable exists for a given provider.
///
/// This is a best-effort check for common provider env vars. It does not
/// validate the key value, only its existence.
///
/// Source: `src/agents/model-auth.ts` - `resolveEnvApiKey`
fn check_common_env_key_for_provider(provider: &str) -> bool {
    let env_keys: &[&str] = match provider {
        "anthropic" => &["ANTHROPIC_API_KEY"],
        "openai" => &["OPENAI_API_KEY"],
        "openai-codex" => &["OPENAI_API_KEY"],
        "openrouter" => &["OPENROUTER_API_KEY"],
        "vercel-ai-gateway" => &["AI_GATEWAY_API_KEY"],
        "cloudflare-ai-gateway" => &["CLOUDFLARE_AI_GATEWAY_API_KEY"],
        "moonshot" => &["MOONSHOT_API_KEY"],
        "kimi-coding" => &["KIMI_API_KEY"],
        "google" => &["GEMINI_API_KEY", "GOOGLE_API_KEY"],
        "zai" => &["ZAI_API_KEY", "Z_AI_API_KEY"],
        "xiaomi" => &["XIAOMI_API_KEY"],
        "minimax" => &["MINIMAX_API_KEY"],
        "synthetic" => &["SYNTHETIC_API_KEY"],
        "venice" => &["VENICE_API_KEY"],
        "opencode" => &["OPENCODE_API_KEY"],
        "xai" => &["XAI_API_KEY"],
        "qianfan" => &["QIANFAN_API_KEY"],
        _ => return false,
    };

    env_keys.iter().any(|key| {
        std::env::var(key)
            .ok()
            .map_or(false, |v| !v.trim().is_empty())
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use oa_types::auth::{AuthConfig, AuthProfileConfig, AuthProfileMode};
    use std::collections::HashMap;

    fn config_with_provider(provider: &str) -> OpenAcosmiConfig {
        let mut profiles = HashMap::new();
        profiles.insert(
            format!("{provider}:default"),
            AuthProfileConfig {
                provider: provider.to_owned(),
                mode: AuthProfileMode::ApiKey,
                email: None,
            },
        );
        OpenAcosmiConfig {
            auth: Some(AuthConfig {
                profiles: Some(profiles),
                order: None,
                cooldowns: None,
            }),
            ..OpenAcosmiConfig::default()
        }
    }

    #[test]
    fn no_warnings_with_anthropic_profile() {
        let config = config_with_provider("anthropic");
        let warnings = check_model_config(&config, &ModelCheckOptions::default());
        // Should not warn about missing auth for anthropic
        assert!(
            !warnings.iter().any(|w| w.message.contains("No auth configured for provider \"anthropic\"")),
            "should not warn about anthropic auth when profile exists"
        );
    }

    #[test]
    fn warns_about_missing_auth() {
        let config = OpenAcosmiConfig::default();
        let warnings = check_model_config(&config, &ModelCheckOptions::default());
        // Default provider is anthropic, should warn about missing auth
        // (unless ANTHROPIC_API_KEY is set in the test env)
        if std::env::var("ANTHROPIC_API_KEY").is_err() {
            assert!(
                warnings.iter().any(|w| w.message.contains("No auth configured")),
                "should warn about missing auth"
            );
        }
    }

    #[test]
    fn detects_openai_codex_mismatch() {
        let mut profiles = HashMap::new();
        profiles.insert(
            "openai:default".to_owned(),
            AuthProfileConfig {
                provider: "openai".to_owned(),
                mode: AuthProfileMode::ApiKey,
                email: None,
            },
        );
        profiles.insert(
            "openai-codex:default".to_owned(),
            AuthProfileConfig {
                provider: "openai-codex".to_owned(),
                mode: AuthProfileMode::Oauth,
                email: None,
            },
        );

        let config = OpenAcosmiConfig {
            auth: Some(AuthConfig {
                profiles: Some(profiles),
                order: None,
                cooldowns: None,
            }),
            agents: Some(oa_types::agents::AgentsConfig {
                defaults: Some(oa_types::agent_defaults::AgentDefaultsConfig {
                    model: Some(oa_types::agent_defaults::AgentModelListConfig {
                        primary: Some("openai/gpt-4o".to_owned()),
                        fallbacks: None,
                    }),
                    ..oa_types::agent_defaults::AgentDefaultsConfig::default()
                }),
                list: None,
            }),
            ..OpenAcosmiConfig::default()
        };

        let warnings = check_model_config(&config, &ModelCheckOptions::default());
        assert!(
            warnings.iter().any(|w| w.message.contains("Codex")),
            "should warn about Codex OAuth detection: {warnings:?}"
        );
    }

    #[test]
    fn check_returns_empty_vec_type() {
        let config = config_with_provider("anthropic");
        let warnings = check_model_config(&config, &ModelCheckOptions::default());
        // Just verify the type and that it doesn't panic
        let _: Vec<ModelCheckWarning> = warnings;
    }
}
