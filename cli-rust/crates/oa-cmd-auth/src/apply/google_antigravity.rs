/// Google Antigravity auth choice handler.
///
/// Delegates to the plugin provider system for Google Antigravity OAuth.
///
/// Source: `src/commands/auth-choice.apply.google-antigravity.ts`

use oa_types::auth::AuthProfileMode;

use crate::auth_choice::AuthChoice;

use super::{apply_auth_profile_config, ApplyAuthChoiceParams, ApplyAuthChoiceResult};

/// Handle the Google Antigravity auth choice.
///
/// Returns `Some(result)` if this handler handles the choice, or `None`
/// to pass to the next handler.
///
/// Source: `src/commands/auth-choice.apply.google-antigravity.ts` - `applyAuthChoiceGoogleAntigravity`
pub fn handle_google_antigravity(params: &ApplyAuthChoiceParams) -> Option<ApplyAuthChoiceResult> {
    if params.auth_choice != AuthChoice::GoogleAntigravity {
        return None;
    }

    // Plugin-based auth: configure the profile for google-antigravity
    let config = apply_auth_profile_config(
        params.config.clone(),
        "google-antigravity:default",
        "google-antigravity",
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
    fn handles_google_antigravity() {
        let params = ApplyAuthChoiceParams {
            auth_choice: AuthChoice::GoogleAntigravity,
            config: OpenAcosmiConfig::default(),
            set_default_model: false,
            agent_id: None,
            agent_dir: None,
            opts: ApplyAuthChoiceOpts::default(),
        };
        let result = handle_google_antigravity(&params);
        assert!(result.is_some());
        let profiles = result.as_ref()
            .and_then(|r| r.config.auth.as_ref())
            .and_then(|a| a.profiles.as_ref())
            .expect("profiles");
        assert!(profiles.contains_key("google-antigravity:default"));
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
        assert!(handle_google_antigravity(&params).is_none());
    }
}
