/// Google Gemini CLI auth choice handler.
///
/// Delegates to the plugin provider system for Google Gemini CLI OAuth.
///
/// Source: `src/commands/auth-choice.apply.google-gemini-cli.ts`

use oa_types::auth::AuthProfileMode;

use crate::auth_choice::AuthChoice;

use super::{apply_auth_profile_config, ApplyAuthChoiceParams, ApplyAuthChoiceResult};

/// Handle the Google Gemini CLI auth choice.
///
/// Returns `Some(result)` if this handler handles the choice, or `None`
/// to pass to the next handler.
///
/// Source: `src/commands/auth-choice.apply.google-gemini-cli.ts` - `applyAuthChoiceGoogleGeminiCli`
pub fn handle_google_gemini_cli(params: &ApplyAuthChoiceParams) -> Option<ApplyAuthChoiceResult> {
    if params.auth_choice != AuthChoice::GoogleGeminiCli {
        return None;
    }

    // Plugin-based auth: configure the profile for google-gemini-cli
    let config = apply_auth_profile_config(
        params.config.clone(),
        "google-gemini-cli:default",
        "google-gemini-cli",
        AuthProfileMode::Oauth,
        None,
    );

    Some(ApplyAuthChoiceResult {
        config,
        agent_model_override: None,
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::apply::ApplyAuthChoiceOpts;
    use oa_types::config::OpenAcosmiConfig;

    #[test]
    fn handles_google_gemini_cli() {
        let params = ApplyAuthChoiceParams {
            auth_choice: AuthChoice::GoogleGeminiCli,
            config: OpenAcosmiConfig::default(),
            set_default_model: false,
            agent_id: None,
            agent_dir: None,
            opts: ApplyAuthChoiceOpts::default(),
        };
        let result = handle_google_gemini_cli(&params);
        assert!(result.is_some());
        let profiles = result.as_ref()
            .and_then(|r| r.config.auth.as_ref())
            .and_then(|a| a.profiles.as_ref())
            .expect("profiles");
        assert!(profiles.contains_key("google-gemini-cli:default"));
    }

    #[test]
    fn ignores_unrelated() {
        let params = ApplyAuthChoiceParams {
            auth_choice: AuthChoice::Token,
            config: OpenAcosmiConfig::default(),
            set_default_model: false,
            agent_id: None,
            agent_dir: None,
            opts: ApplyAuthChoiceOpts::default(),
        };
        assert!(handle_google_gemini_cli(&params).is_none());
    }
}
