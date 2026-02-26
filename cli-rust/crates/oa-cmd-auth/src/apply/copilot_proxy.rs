/// Copilot Proxy auth choice handler.
///
/// Delegates to the plugin provider system for the local Copilot Proxy.
///
/// Source: `src/commands/auth-choice.apply.copilot-proxy.ts`

use oa_types::auth::AuthProfileMode;

use crate::auth_choice::AuthChoice;

use super::{apply_auth_profile_config, ApplyAuthChoiceParams, ApplyAuthChoiceResult};

/// Handle the Copilot Proxy auth choice.
///
/// Returns `Some(result)` if this handler handles the choice, or `None`
/// to pass to the next handler.
///
/// Source: `src/commands/auth-choice.apply.copilot-proxy.ts` - `applyAuthChoiceCopilotProxy`
pub fn handle_copilot_proxy(params: &ApplyAuthChoiceParams) -> Option<ApplyAuthChoiceResult> {
    if params.auth_choice != AuthChoice::CopilotProxy {
        return None;
    }

    // Plugin-based auth: configure the profile for copilot-proxy
    let config = apply_auth_profile_config(
        params.config.clone(),
        "copilot-proxy:default",
        "copilot-proxy",
        AuthProfileMode::Token,
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
    fn handles_copilot_proxy() {
        let params = ApplyAuthChoiceParams {
            auth_choice: AuthChoice::CopilotProxy,
            config: OpenAcosmiConfig::default(),
            set_default_model: false,
            agent_id: None,
            agent_dir: None,
            opts: ApplyAuthChoiceOpts::default(),
        };
        let result = handle_copilot_proxy(&params);
        assert!(result.is_some());
        let profiles = result.as_ref()
            .and_then(|r| r.config.auth.as_ref())
            .and_then(|a| a.profiles.as_ref())
            .expect("profiles");
        assert!(profiles.contains_key("copilot-proxy:default"));
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
        assert!(handle_copilot_proxy(&params).is_none());
    }
}
