/// xAI (Grok) auth choice handler.
///
/// Handles the `XaiApiKey` auth choice. Configures the xAI auth profile
/// and sets the appropriate default model.
///
/// Source: `src/commands/auth-choice.apply.xai.ts`

use oa_types::auth::AuthProfileMode;

use crate::auth_choice::AuthChoice;
use crate::default_model::{apply_default_model_choice, ensure_model_allowlist_entry, set_default_model_primary};

use super::{apply_auth_profile_config, ApplyAuthChoiceParams, ApplyAuthChoiceResult};

/// Default model for xAI (Grok) flows.
///
/// Source: `src/commands/onboard-auth.ts` - `XAI_DEFAULT_MODEL_REF`
pub const XAI_DEFAULT_MODEL_REF: &str = "xai/grok-3-mini-fast-latest";

/// Handle the xAI auth choice.
///
/// Returns `Some(result)` if this handler handles the choice, or `None`
/// to pass to the next handler.
///
/// Source: `src/commands/auth-choice.apply.xai.ts` - `applyAuthChoiceXAI`
pub fn handle_xai(params: &ApplyAuthChoiceParams) -> Option<ApplyAuthChoiceResult> {
    if params.auth_choice != AuthChoice::XaiApiKey {
        return None;
    }

    let config = apply_auth_profile_config(
        params.config.clone(),
        "xai:default",
        "xai",
        AuthProfileMode::ApiKey,
        None,
    );

    let result = apply_default_model_choice(
        config,
        params.set_default_model,
        XAI_DEFAULT_MODEL_REF,
        |cfg| set_default_model_primary(cfg, XAI_DEFAULT_MODEL_REF),
        |cfg| ensure_model_allowlist_entry(cfg, XAI_DEFAULT_MODEL_REF),
    );

    Some(ApplyAuthChoiceResult {
        config: result.config,
        agent_model_override: result.agent_model_override,
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
    fn handles_xai() {
        let result = handle_xai(&make_params(AuthChoice::XaiApiKey));
        assert!(result.is_some());
        let r = result.expect("should be some");
        let profiles = r.config.auth.as_ref()
            .and_then(|a| a.profiles.as_ref())
            .expect("profiles");
        assert!(profiles.contains_key("xai:default"));
        assert_eq!(profiles["xai:default"].mode, AuthProfileMode::ApiKey);
    }

    #[test]
    fn sets_default_model() {
        let result = handle_xai(&make_params(AuthChoice::XaiApiKey));
        let r = result.expect("should be some");
        let primary = r.config.agents.as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.model.as_ref())
            .and_then(|m| m.primary.as_deref());
        assert_eq!(primary, Some(XAI_DEFAULT_MODEL_REF));
    }

    #[test]
    fn agent_override_when_not_default() {
        let mut params = make_params(AuthChoice::XaiApiKey);
        params.set_default_model = false;
        let result = handle_xai(&params);
        let r = result.expect("should be some");
        assert_eq!(
            r.agent_model_override.as_deref(),
            Some(XAI_DEFAULT_MODEL_REF)
        );
    }

    #[test]
    fn ignores_unrelated() {
        assert!(handle_xai(&make_params(AuthChoice::Token)).is_none());
        assert!(handle_xai(&make_params(AuthChoice::OpenaiApiKey)).is_none());
    }
}
