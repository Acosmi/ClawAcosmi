/// GitHub Copilot auth choice handler.
///
/// Handles the `GithubCopilot` auth choice, which uses a GitHub device
/// login flow and requires an active GitHub Copilot subscription.
///
/// Source: `src/commands/auth-choice.apply.github-copilot.ts`

use oa_types::auth::AuthProfileMode;

use crate::auth_choice::AuthChoice;
use crate::default_model::set_default_model_primary;

use super::{apply_auth_profile_config, ApplyAuthChoiceParams, ApplyAuthChoiceResult};

/// Default model for GitHub Copilot flows.
///
/// Source: `src/commands/auth-choice.apply.github-copilot.ts`
pub const GITHUB_COPILOT_DEFAULT_MODEL: &str = "github-copilot/gpt-4o";

/// Handle the GitHub Copilot auth choice.
///
/// Returns `Some(result)` if this handler handles the choice, or `None`
/// to pass to the next handler.
///
/// Source: `src/commands/auth-choice.apply.github-copilot.ts` - `applyAuthChoiceGitHubCopilot`
pub fn handle_github_copilot(params: &ApplyAuthChoiceParams) -> Option<ApplyAuthChoiceResult> {
    if params.auth_choice != AuthChoice::GithubCopilot {
        return None;
    }

    let mut next_config = apply_auth_profile_config(
        params.config.clone(),
        "github-copilot:github",
        "github-copilot",
        AuthProfileMode::Token,
        None,
    );

    if params.set_default_model {
        next_config = set_default_model_primary(next_config, GITHUB_COPILOT_DEFAULT_MODEL);
    }

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
            set_default_model: true,
            agent_id: None,
            agent_dir: None,
            opts: ApplyAuthChoiceOpts::default(),
        }
    }

    #[test]
    fn handles_github_copilot() {
        let result = handle_github_copilot(&make_params(AuthChoice::GithubCopilot));
        assert!(result.is_some());
        let r = result.expect("should be some");
        let profiles = r.config.auth.as_ref()
            .and_then(|a| a.profiles.as_ref())
            .expect("profiles");
        assert!(profiles.contains_key("github-copilot:github"));
        assert_eq!(profiles["github-copilot:github"].mode, AuthProfileMode::Token);
    }

    #[test]
    fn sets_default_model_when_requested() {
        let result = handle_github_copilot(&make_params(AuthChoice::GithubCopilot));
        let r = result.expect("should be some");
        let primary = r.config.agents.as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.model.as_ref())
            .and_then(|m| m.primary.as_deref());
        assert_eq!(primary, Some(GITHUB_COPILOT_DEFAULT_MODEL));
    }

    #[test]
    fn does_not_set_model_when_not_requested() {
        let mut params = make_params(AuthChoice::GithubCopilot);
        params.set_default_model = false;
        let result = handle_github_copilot(&params);
        let r = result.expect("should be some");
        let primary = r.config.agents.as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.model.as_ref())
            .and_then(|m| m.primary.as_deref());
        assert!(primary.is_none());
    }

    #[test]
    fn ignores_unrelated() {
        assert!(handle_github_copilot(&make_params(AuthChoice::Token)).is_none());
    }
}
