/// Qwen Portal auth choice handler.
///
/// Delegates to the plugin provider system for Qwen device-flow OAuth.
///
/// Source: `src/commands/auth-choice.apply.qwen-portal.ts`

use oa_types::auth::AuthProfileMode;

use crate::auth_choice::AuthChoice;

use super::{apply_auth_profile_config, ApplyAuthChoiceParams, ApplyAuthChoiceResult};

/// Handle the Qwen Portal auth choice.
///
/// Returns `Some(result)` if this handler handles the choice, or `None`
/// to pass to the next handler.
///
/// Source: `src/commands/auth-choice.apply.qwen-portal.ts` - `applyAuthChoiceQwenPortal`
pub fn handle_qwen_portal(params: &ApplyAuthChoiceParams) -> Option<ApplyAuthChoiceResult> {
    if params.auth_choice != AuthChoice::QwenPortal {
        return None;
    }

    // Plugin-based auth: configure the profile for qwen-portal
    let config = apply_auth_profile_config(
        params.config.clone(),
        "qwen-portal:default",
        "qwen-portal",
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
    fn handles_qwen_portal() {
        let params = ApplyAuthChoiceParams {
            auth_choice: AuthChoice::QwenPortal,
            config: OpenAcosmiConfig::default(),
            set_default_model: false,
            agent_id: None,
            agent_dir: None,
            opts: ApplyAuthChoiceOpts::default(),
        };
        let result = handle_qwen_portal(&params);
        assert!(result.is_some());
        let profiles = result.as_ref()
            .and_then(|r| r.config.auth.as_ref())
            .and_then(|a| a.profiles.as_ref())
            .expect("profiles");
        assert!(profiles.contains_key("qwen-portal:default"));
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
        assert!(handle_qwen_portal(&params).is_none());
    }
}
