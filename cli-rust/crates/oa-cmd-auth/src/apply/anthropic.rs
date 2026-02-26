/// Anthropic auth choice handler.
///
/// Handles `Token`, `Oauth`, `SetupToken`, and `ApiKey` auth choices for
/// the Anthropic provider. Token flows store setup-token credentials;
/// API key flows store an Anthropic API key.
///
/// Note: Interactive prompting (text input, confirm) is deferred to the
/// caller via the returned configuration. In non-interactive mode, the
/// handler applies pre-supplied options.
///
/// Source: `src/commands/auth-choice.apply.anthropic.ts`

use oa_types::auth::AuthProfileMode;

use crate::auth_choice::AuthChoice;

use super::{apply_auth_profile_config, ApplyAuthChoiceParams, ApplyAuthChoiceResult};

/// Handle Anthropic auth choices (`Token`, `Oauth`, `SetupToken`, `ApiKey`).
///
/// Returns `Some(result)` if this handler handles the choice, or `None`
/// to pass to the next handler.
///
/// For interactive flows (which require user prompts), this function
/// applies the profile configuration assuming credentials have been
/// collected by the caller.
///
/// Source: `src/commands/auth-choice.apply.anthropic.ts` - `applyAuthChoiceAnthropic`
pub fn handle_anthropic(params: &ApplyAuthChoiceParams) -> Option<ApplyAuthChoiceResult> {
    match params.auth_choice {
        AuthChoice::SetupToken | AuthChoice::Oauth | AuthChoice::Token => {
            // Token/setup-token flow: the caller must collect the token
            // interactively. Here we apply the profile configuration for
            // a token-based credential.
            let profile_id = "anthropic:default";
            let provider = "anthropic";

            let next_config = apply_auth_profile_config(
                params.config.clone(),
                profile_id,
                provider,
                AuthProfileMode::Token,
                None,
            );

            Some(ApplyAuthChoiceResult {
                config: next_config,
                agent_model_override: None,
            })
        }

        AuthChoice::ApiKey => {
            // Check if the token provider opt redirects away from Anthropic
            if let Some(ref token_provider) = params.opts.token_provider {
                if token_provider != "anthropic" {
                    return None;
                }
            }

            let next_config = apply_auth_profile_config(
                params.config.clone(),
                "anthropic:default",
                "anthropic",
                AuthProfileMode::ApiKey,
                None,
            );

            Some(ApplyAuthChoiceResult {
                config: next_config,
                agent_model_override: None,
            })
        }

        _ => None,
    }
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
            set_default_model: false,
            agent_id: None,
            agent_dir: None,
            opts: ApplyAuthChoiceOpts::default(),
        }
    }

    #[test]
    fn handles_token() {
        let result = handle_anthropic(&make_params(AuthChoice::Token));
        assert!(result.is_some());
        let profiles = result
            .as_ref()
            .and_then(|r| r.config.auth.as_ref())
            .and_then(|a| a.profiles.as_ref())
            .expect("should have profiles");
        assert!(profiles.contains_key("anthropic:default"));
        assert_eq!(
            profiles["anthropic:default"].mode,
            AuthProfileMode::Token
        );
    }

    #[test]
    fn handles_setup_token() {
        let result = handle_anthropic(&make_params(AuthChoice::SetupToken));
        assert!(result.is_some());
    }

    #[test]
    fn handles_oauth() {
        let result = handle_anthropic(&make_params(AuthChoice::Oauth));
        assert!(result.is_some());
    }

    #[test]
    fn handles_api_key() {
        let result = handle_anthropic(&make_params(AuthChoice::ApiKey));
        assert!(result.is_some());
        let profiles = result
            .as_ref()
            .and_then(|r| r.config.auth.as_ref())
            .and_then(|a| a.profiles.as_ref())
            .expect("should have profiles");
        assert_eq!(
            profiles["anthropic:default"].mode,
            AuthProfileMode::ApiKey
        );
    }

    #[test]
    fn api_key_with_non_anthropic_provider_passes() {
        let mut params = make_params(AuthChoice::ApiKey);
        params.opts.token_provider = Some("openai".to_owned());
        let result = handle_anthropic(&params);
        assert!(result.is_none());
    }

    #[test]
    fn api_key_with_anthropic_provider_handled() {
        let mut params = make_params(AuthChoice::ApiKey);
        params.opts.token_provider = Some("anthropic".to_owned());
        let result = handle_anthropic(&params);
        assert!(result.is_some());
    }

    #[test]
    fn ignores_unrelated_choices() {
        assert!(handle_anthropic(&make_params(AuthChoice::OpenaiApiKey)).is_none());
        assert!(handle_anthropic(&make_params(AuthChoice::Skip)).is_none());
        assert!(handle_anthropic(&make_params(AuthChoice::GeminiApiKey)).is_none());
    }
}
