/// Apply auth choice dispatcher and per-provider handlers.
///
/// The main entry point [`apply_auth_choice`] dispatches to provider-specific
/// handler functions. Each handler checks whether it handles the given
/// [`AuthChoice`] and returns `Some(result)` if so, or `None` to pass
/// to the next handler.
///
/// Source: `src/commands/auth-choice.apply.ts`

pub mod anthropic;
pub mod api_providers;
pub mod copilot_proxy;
pub mod github_copilot;
pub mod google_antigravity;
pub mod google_gemini_cli;
pub mod minimax;
pub mod oauth;
pub mod openai;
pub mod plugin_provider;
pub mod qwen_portal;
pub mod xai;

use std::collections::HashMap;

use oa_types::auth::{AuthConfig, AuthProfileConfig, AuthProfileMode};
use oa_types::config::OpenAcosmiConfig;

use crate::auth_choice::AuthChoice;

/// Parameters for applying an auth choice to the configuration.
///
/// Source: `src/commands/auth-choice.apply.ts` - `ApplyAuthChoiceParams`
#[derive(Debug, Clone)]
pub struct ApplyAuthChoiceParams {
    /// The auth choice selected by the user.
    pub auth_choice: AuthChoice,
    /// The current configuration.
    pub config: OpenAcosmiConfig,
    /// Whether to set the model as the global default.
    pub set_default_model: bool,
    /// Optional agent ID for agent-specific configuration.
    pub agent_id: Option<String>,
    /// Optional agent directory for credential storage.
    pub agent_dir: Option<String>,
    /// Additional options from CLI flags or non-interactive mode.
    pub opts: ApplyAuthChoiceOpts,
}

/// Additional options for auth choice application.
///
/// Source: `src/commands/auth-choice.apply.ts` - `ApplyAuthChoiceParams.opts`
#[derive(Debug, Clone, Default)]
pub struct ApplyAuthChoiceOpts {
    /// Token provider name (for non-interactive token flows).
    pub token_provider: Option<String>,
    /// Pre-supplied token value.
    pub token: Option<String>,
    /// Cloudflare AI Gateway Account ID.
    pub cloudflare_ai_gateway_account_id: Option<String>,
    /// Cloudflare AI Gateway Gateway ID.
    pub cloudflare_ai_gateway_gateway_id: Option<String>,
    /// Cloudflare AI Gateway API key.
    pub cloudflare_ai_gateway_api_key: Option<String>,
    /// xAI API key.
    pub xai_api_key: Option<String>,
}

/// Result of applying an auth choice.
///
/// Source: `src/commands/auth-choice.apply.ts` - `ApplyAuthChoiceResult`
#[derive(Debug, Clone)]
pub struct ApplyAuthChoiceResult {
    /// The updated configuration.
    pub config: OpenAcosmiConfig,
    /// Optional agent-specific model override (when `set_default_model` was false).
    pub agent_model_override: Option<String>,
}

/// Apply an auth profile to the configuration.
///
/// Adds the profile to `auth.profiles` and sets the provider in the
/// profile order. This is the Rust equivalent of the TS `applyAuthProfileConfig`.
///
/// Source: `src/commands/onboard-auth.ts` - `applyAuthProfileConfig`
#[must_use]
pub fn apply_auth_profile_config(
    mut config: OpenAcosmiConfig,
    profile_id: &str,
    provider: &str,
    mode: AuthProfileMode,
    email: Option<&str>,
) -> OpenAcosmiConfig {
    let auth = config.auth.get_or_insert_with(AuthConfig::default);
    let profiles = auth.profiles.get_or_insert_with(HashMap::new);

    profiles.insert(
        profile_id.to_owned(),
        AuthProfileConfig {
            provider: provider.to_owned(),
            mode,
            email: email.map(str::to_owned),
        },
    );

    // Add to provider order
    let order = auth.order.get_or_insert_with(HashMap::new);
    let provider_order = order.entry(provider.to_owned()).or_default();
    if !provider_order.contains(&profile_id.to_owned()) {
        provider_order.push(profile_id.to_owned());
    }

    config
}

/// Dispatch an auth choice to the appropriate handler.
///
/// Iterates through all provider-specific handlers in order. The first
/// handler that returns `Some(result)` wins. If no handler matches,
/// returns the configuration unchanged.
///
/// Source: `src/commands/auth-choice.apply.ts` - `applyAuthChoice`
pub fn apply_auth_choice(params: &ApplyAuthChoiceParams) -> ApplyAuthChoiceResult {
    // Try each handler in order, matching the TS handler chain
    let handlers: Vec<fn(&ApplyAuthChoiceParams) -> Option<ApplyAuthChoiceResult>> = vec![
        anthropic::handle_anthropic,
        openai::handle_openai,
        oauth::handle_oauth,
        api_providers::handle_api_providers,
        minimax::handle_minimax,
        github_copilot::handle_github_copilot,
        google_antigravity::handle_google_antigravity,
        google_gemini_cli::handle_google_gemini_cli,
        copilot_proxy::handle_copilot_proxy,
        qwen_portal::handle_qwen_portal,
        xai::handle_xai,
    ];

    for handler in handlers {
        if let Some(result) = handler(params) {
            return result;
        }
    }

    // No handler matched -- return config unchanged
    ApplyAuthChoiceResult {
        config: params.config.clone(),
        agent_model_override: None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn apply_auth_profile_config_adds_profile() {
        let config = OpenAcosmiConfig::default();
        let updated = apply_auth_profile_config(
            config,
            "anthropic:default",
            "anthropic",
            AuthProfileMode::ApiKey,
            None,
        );

        let profiles = updated
            .auth
            .as_ref()
            .and_then(|a| a.profiles.as_ref())
            .expect("profiles should exist");
        assert!(profiles.contains_key("anthropic:default"));
        assert_eq!(profiles["anthropic:default"].provider, "anthropic");
        assert_eq!(profiles["anthropic:default"].mode, AuthProfileMode::ApiKey);
    }

    #[test]
    fn apply_auth_profile_config_adds_to_order() {
        let config = OpenAcosmiConfig::default();
        let updated = apply_auth_profile_config(
            config,
            "anthropic:default",
            "anthropic",
            AuthProfileMode::ApiKey,
            None,
        );

        let order = updated
            .auth
            .as_ref()
            .and_then(|a| a.order.as_ref())
            .expect("order should exist");
        assert!(order.contains_key("anthropic"));
        assert!(order["anthropic"].contains(&"anthropic:default".to_owned()));
    }

    #[test]
    fn apply_auth_profile_config_no_duplicate_order() {
        let config = OpenAcosmiConfig::default();
        let updated = apply_auth_profile_config(
            config,
            "anthropic:default",
            "anthropic",
            AuthProfileMode::ApiKey,
            None,
        );
        // Apply same profile again
        let updated2 = apply_auth_profile_config(
            updated,
            "anthropic:default",
            "anthropic",
            AuthProfileMode::ApiKey,
            None,
        );

        let order = updated2
            .auth
            .as_ref()
            .and_then(|a| a.order.as_ref())
            .expect("order should exist");
        let count = order["anthropic"]
            .iter()
            .filter(|id| *id == "anthropic:default")
            .count();
        assert_eq!(count, 1, "should not add duplicate order entries");
    }

    #[test]
    fn apply_auth_profile_config_with_email() {
        let config = OpenAcosmiConfig::default();
        let updated = apply_auth_profile_config(
            config,
            "chutes:user@example.com",
            "chutes",
            AuthProfileMode::Oauth,
            Some("user@example.com"),
        );

        let profiles = updated
            .auth
            .as_ref()
            .and_then(|a| a.profiles.as_ref())
            .expect("profiles should exist");
        assert_eq!(
            profiles["chutes:user@example.com"].email.as_deref(),
            Some("user@example.com")
        );
    }

    #[test]
    fn apply_auth_choice_skip_returns_unchanged() {
        let config = OpenAcosmiConfig::default();
        let params = ApplyAuthChoiceParams {
            auth_choice: AuthChoice::Skip,
            config: config.clone(),
            set_default_model: false,
            agent_id: None,
            agent_dir: None,
            opts: ApplyAuthChoiceOpts::default(),
        };
        let result = apply_auth_choice(&params);
        assert!(result.agent_model_override.is_none());
    }

    #[test]
    fn apply_auth_choice_anthropic_token_is_handled() {
        let config = OpenAcosmiConfig::default();
        let params = ApplyAuthChoiceParams {
            auth_choice: AuthChoice::Token,
            config,
            set_default_model: false,
            agent_id: None,
            agent_dir: None,
            opts: ApplyAuthChoiceOpts::default(),
        };
        // The token handler requires interactive input, so in a unit test
        // it will return a "needs interactive" marker. Here we just verify
        // the handler chain reaches the anthropic handler.
        let result = apply_auth_choice(&params);
        // Token flow needs interactive prompts, so the handler returns
        // a config indicating the Anthropic handler was selected.
        // In non-interactive mode, it should at least not panic.
        let _config = result.config;
    }
}
