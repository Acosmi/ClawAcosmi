/// OpenAI auth choice handler.
///
/// Handles `OpenaiApiKey` and `OpenaiCodex` auth choices. API key flows
/// store the key; Codex flows configure OAuth. Both set the default model
/// to the appropriate OpenAI model reference.
///
/// Source: `src/commands/auth-choice.apply.openai.ts`

use oa_types::auth::AuthProfileMode;

use crate::auth_choice::AuthChoice;
use crate::default_model::apply_default_model_choice;

use super::{apply_auth_profile_config, ApplyAuthChoiceParams, ApplyAuthChoiceResult};

/// Default model for OpenAI API key flows.
///
/// Source: `src/commands/openai-model-default.ts` - `OPENAI_DEFAULT_MODEL`
pub const OPENAI_DEFAULT_MODEL: &str = "openai/gpt-4o";

/// Default model for OpenAI Codex OAuth flows.
///
/// Source: `src/commands/openai-codex-model-default.ts` - `OPENAI_CODEX_DEFAULT_MODEL`
pub const OPENAI_CODEX_DEFAULT_MODEL: &str = "openai-codex/codex-mini-latest";

/// Apply OpenAI-specific default model configuration.
///
/// Source: `src/commands/openai-model-default.ts` - `applyOpenAIConfig`
fn apply_openai_config(config: oa_types::config::OpenAcosmiConfig) -> oa_types::config::OpenAcosmiConfig {
    crate::default_model::set_default_model_primary(config, OPENAI_DEFAULT_MODEL)
}

/// Apply OpenAI-specific provider configuration (for agent-level overrides).
///
/// Source: `src/commands/openai-model-default.ts` - `applyOpenAIProviderConfig`
fn apply_openai_provider_config(
    config: oa_types::config::OpenAcosmiConfig,
) -> oa_types::config::OpenAcosmiConfig {
    crate::default_model::ensure_model_allowlist_entry(config, OPENAI_DEFAULT_MODEL)
}

/// Apply OpenAI Codex default model configuration.
///
/// Source: `src/commands/openai-codex-model-default.ts` - `applyOpenAICodexModelDefault`
fn apply_openai_codex_config(
    config: oa_types::config::OpenAcosmiConfig,
) -> oa_types::config::OpenAcosmiConfig {
    crate::default_model::set_default_model_primary(config, OPENAI_CODEX_DEFAULT_MODEL)
}

/// Handle OpenAI auth choices (`OpenaiApiKey`, `OpenaiCodex`).
///
/// Returns `Some(result)` if this handler handles the choice, or `None`
/// to pass to the next handler.
///
/// Source: `src/commands/auth-choice.apply.openai.ts` - `applyAuthChoiceOpenAI`
pub fn handle_openai(params: &ApplyAuthChoiceParams) -> Option<ApplyAuthChoiceResult> {
    let mut auth_choice = params.auth_choice;

    // Redirect `ApiKey` with tokenProvider=openai to `OpenaiApiKey`
    if auth_choice == AuthChoice::ApiKey {
        if let Some(ref tp) = params.opts.token_provider {
            if tp == "openai" {
                auth_choice = AuthChoice::OpenaiApiKey;
            }
        }
    }

    if auth_choice == AuthChoice::OpenaiApiKey {
        // API key flow: configure profile and set default model
        let next_config = apply_auth_profile_config(
            params.config.clone(),
            "openai:default",
            "openai",
            AuthProfileMode::ApiKey,
            None,
        );

        let result = apply_default_model_choice(
            next_config,
            params.set_default_model,
            OPENAI_DEFAULT_MODEL,
            apply_openai_config,
            apply_openai_provider_config,
        );

        return Some(ApplyAuthChoiceResult {
            config: result.config,
            agent_model_override: result.agent_model_override,
        });
    }

    if params.auth_choice == AuthChoice::OpenaiCodex {
        // Codex OAuth flow: configure profile and set Codex default model
        let next_config = apply_auth_profile_config(
            params.config.clone(),
            "openai-codex:default",
            "openai-codex",
            AuthProfileMode::Oauth,
            None,
        );

        let result = apply_default_model_choice(
            next_config,
            params.set_default_model,
            OPENAI_CODEX_DEFAULT_MODEL,
            apply_openai_codex_config,
            |cfg| crate::default_model::ensure_model_allowlist_entry(cfg, OPENAI_CODEX_DEFAULT_MODEL),
        );

        return Some(ApplyAuthChoiceResult {
            config: result.config,
            agent_model_override: result.agent_model_override,
        });
    }

    None
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::apply::ApplyAuthChoiceOpts;
    use oa_types::config::OpenAcosmiConfig;

    fn make_params(choice: AuthChoice) -> ApplyAuthChoiceParams {
        ApplyAuthChoiceParams {
            auth_choice: choice,
            config: OpenAcosmiConfig::default(),
            set_default_model: true,
            agent_id: None,
            agent_dir: None,
            opts: ApplyAuthChoiceOpts::default(),
        }
    }

    #[test]
    fn handles_openai_api_key() {
        let result = handle_openai(&make_params(AuthChoice::OpenaiApiKey));
        assert!(result.is_some());
        let r = result.expect("should be some");
        let profiles = r
            .config
            .auth
            .as_ref()
            .and_then(|a| a.profiles.as_ref())
            .expect("should have profiles");
        assert!(profiles.contains_key("openai:default"));
    }

    #[test]
    fn handles_openai_codex() {
        let result = handle_openai(&make_params(AuthChoice::OpenaiCodex));
        assert!(result.is_some());
        let r = result.expect("should be some");
        let profiles = r
            .config
            .auth
            .as_ref()
            .and_then(|a| a.profiles.as_ref())
            .expect("should have profiles");
        assert!(profiles.contains_key("openai-codex:default"));
        assert_eq!(
            profiles["openai-codex:default"].mode,
            AuthProfileMode::Oauth
        );
    }

    #[test]
    fn openai_api_key_sets_default_model() {
        let result = handle_openai(&make_params(AuthChoice::OpenaiApiKey))
            .expect("should handle");
        let primary = result
            .config
            .agents
            .as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.model.as_ref())
            .and_then(|m| m.primary.as_deref());
        assert_eq!(primary, Some(OPENAI_DEFAULT_MODEL));
    }

    #[test]
    fn openai_codex_sets_codex_default_model() {
        let result = handle_openai(&make_params(AuthChoice::OpenaiCodex))
            .expect("should handle");
        let primary = result
            .config
            .agents
            .as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.model.as_ref())
            .and_then(|m| m.primary.as_deref());
        assert_eq!(primary, Some(OPENAI_CODEX_DEFAULT_MODEL));
    }

    #[test]
    fn api_key_with_openai_token_provider_redirects() {
        let mut params = make_params(AuthChoice::ApiKey);
        params.opts.token_provider = Some("openai".to_owned());
        let result = handle_openai(&params);
        assert!(result.is_some());
    }

    #[test]
    fn ignores_unrelated_choices() {
        assert!(handle_openai(&make_params(AuthChoice::Token)).is_none());
        assert!(handle_openai(&make_params(AuthChoice::GeminiApiKey)).is_none());
    }

    #[test]
    fn agent_override_when_not_default() {
        let mut params = make_params(AuthChoice::OpenaiApiKey);
        params.set_default_model = false;
        let result = handle_openai(&params).expect("should handle");
        assert_eq!(
            result.agent_model_override.as_deref(),
            Some(OPENAI_DEFAULT_MODEL)
        );
    }
}
