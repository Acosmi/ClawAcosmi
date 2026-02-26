/// OAuth provider auth choice handler (Chutes).
///
/// Handles the `Chutes` auth choice which uses an OAuth flow with PKCE.
/// The actual OAuth token exchange is performed externally; this handler
/// applies the resulting credential profile to the configuration.
///
/// Source: `src/commands/auth-choice.apply.oauth.ts`

use oa_types::auth::AuthProfileMode;

use crate::auth_choice::AuthChoice;

use super::{apply_auth_profile_config, ApplyAuthChoiceParams, ApplyAuthChoiceResult};

/// Handle OAuth auth choices (`Chutes`).
///
/// Returns `Some(result)` if this handler handles the choice, or `None`
/// to pass to the next handler.
///
/// Source: `src/commands/auth-choice.apply.oauth.ts` - `applyAuthChoiceOAuth`
pub fn handle_oauth(params: &ApplyAuthChoiceParams) -> Option<ApplyAuthChoiceResult> {
    if params.auth_choice != AuthChoice::Chutes {
        return None;
    }

    // The actual OAuth flow requires interactive browser interaction.
    // Here we set up the profile configuration for a Chutes OAuth credential.
    // In a full interactive implementation, the caller would run the OAuth
    // flow first and then call this to persist the result.
    let profile_id = "chutes:default";
    let provider = "chutes";

    let next_config = apply_auth_profile_config(
        params.config.clone(),
        profile_id,
        provider,
        AuthProfileMode::Oauth,
        None,
    );

    Some(ApplyAuthChoiceResult {
        config: next_config,
        agent_model_override: None,
    })
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
    fn handles_chutes() {
        let result = handle_oauth(&make_params(AuthChoice::Chutes));
        assert!(result.is_some());
        let profiles = result
            .as_ref()
            .and_then(|r| r.config.auth.as_ref())
            .and_then(|a| a.profiles.as_ref())
            .expect("should have profiles");
        assert!(profiles.contains_key("chutes:default"));
        assert_eq!(
            profiles["chutes:default"].mode,
            AuthProfileMode::Oauth
        );
    }

    #[test]
    fn ignores_non_chutes() {
        assert!(handle_oauth(&make_params(AuthChoice::Token)).is_none());
        assert!(handle_oauth(&make_params(AuthChoice::OpenaiCodex)).is_none());
    }
}
