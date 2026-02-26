/// MiniMax auth choice handler.
///
/// Handles `MinimaxPortal`, `MinimaxCloud`, `MinimaxApi`,
/// `MinimaxApiLightning`, and `Minimax` (local) auth choices.
///
/// Source: `src/commands/auth-choice.apply.minimax.ts`

use oa_types::auth::AuthProfileMode;

use crate::auth_choice::AuthChoice;
use crate::default_model::{apply_default_model_choice, ensure_model_allowlist_entry, set_default_model_primary};

use super::{apply_auth_profile_config, ApplyAuthChoiceParams, ApplyAuthChoiceResult};

/// Default model for MiniMax M2.1 (standard speed).
///
/// Source: `src/commands/auth-choice.apply.minimax.ts`
const MINIMAX_M2_1_MODEL: &str = "MiniMax-M2.1";

/// Default model for MiniMax M2.1 Lightning (faster).
///
/// Source: `src/commands/auth-choice.apply.minimax.ts`
const MINIMAX_M2_1_LIGHTNING_MODEL: &str = "MiniMax-M2.1-lightning";

/// Default model for MiniMax local (LM Studio).
///
/// Source: `src/commands/auth-choice.apply.minimax.ts`
const MINIMAX_LOCAL_DEFAULT_MODEL: &str = "lmstudio/minimax-m2.1-gs32";

/// Handle MiniMax auth choices.
///
/// Returns `Some(result)` if this handler handles the choice, or `None`
/// to pass to the next handler.
///
/// Source: `src/commands/auth-choice.apply.minimax.ts` - `applyAuthChoiceMiniMax`
pub fn handle_minimax(params: &ApplyAuthChoiceParams) -> Option<ApplyAuthChoiceResult> {
    match params.auth_choice {
        AuthChoice::MinimaxPortal => {
            // Plugin-provider flow: the actual OAuth is handled by the plugin.
            // Here we configure the profile for the portal provider.
            let config = apply_auth_profile_config(
                params.config.clone(),
                "minimax-portal:default",
                "minimax-portal",
                AuthProfileMode::Oauth,
                None,
            );
            Some(ApplyAuthChoiceResult {
                config,
                agent_model_override: None,
            })
        }

        AuthChoice::MinimaxCloud | AuthChoice::MinimaxApi | AuthChoice::MinimaxApiLightning => {
            let model_id = if params.auth_choice == AuthChoice::MinimaxApiLightning {
                MINIMAX_M2_1_LIGHTNING_MODEL
            } else {
                MINIMAX_M2_1_MODEL
            };
            let model_ref = format!("minimax/{model_id}");

            let config = apply_auth_profile_config(
                params.config.clone(),
                "minimax:default",
                "minimax",
                AuthProfileMode::ApiKey,
                None,
            );

            let result = apply_default_model_choice(
                config,
                params.set_default_model,
                &model_ref,
                |cfg| set_default_model_primary(cfg, &model_ref),
                |cfg| ensure_model_allowlist_entry(cfg, &model_ref),
            );

            Some(ApplyAuthChoiceResult {
                config: result.config,
                agent_model_override: result.agent_model_override,
            })
        }

        AuthChoice::Minimax => {
            // Local MiniMax (LM Studio)
            let result = apply_default_model_choice(
                params.config.clone(),
                params.set_default_model,
                MINIMAX_LOCAL_DEFAULT_MODEL,
                |cfg| set_default_model_primary(cfg, MINIMAX_LOCAL_DEFAULT_MODEL),
                |cfg| ensure_model_allowlist_entry(cfg, MINIMAX_LOCAL_DEFAULT_MODEL),
            );

            Some(ApplyAuthChoiceResult {
                config: result.config,
                agent_model_override: result.agent_model_override,
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
            set_default_model: true,
            agent_id: None,
            agent_dir: None,
            opts: ApplyAuthChoiceOpts::default(),
        }
    }

    #[test]
    fn handles_minimax_portal() {
        let result = handle_minimax(&make_params(AuthChoice::MinimaxPortal));
        assert!(result.is_some());
        let profiles = result.as_ref()
            .and_then(|r| r.config.auth.as_ref())
            .and_then(|a| a.profiles.as_ref())
            .expect("profiles");
        assert!(profiles.contains_key("minimax-portal:default"));
    }

    #[test]
    fn handles_minimax_api() {
        let result = handle_minimax(&make_params(AuthChoice::MinimaxApi));
        assert!(result.is_some());
        let r = result.expect("should be some");
        let primary = r.config.agents.as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.model.as_ref())
            .and_then(|m| m.primary.as_deref());
        assert_eq!(primary, Some("minimax/MiniMax-M2.1"));
    }

    #[test]
    fn handles_minimax_api_lightning() {
        let result = handle_minimax(&make_params(AuthChoice::MinimaxApiLightning));
        assert!(result.is_some());
        let r = result.expect("should be some");
        let primary = r.config.agents.as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.model.as_ref())
            .and_then(|m| m.primary.as_deref());
        assert_eq!(primary, Some("minimax/MiniMax-M2.1-lightning"));
    }

    #[test]
    fn handles_minimax_local() {
        let result = handle_minimax(&make_params(AuthChoice::Minimax));
        assert!(result.is_some());
        let r = result.expect("should be some");
        let primary = r.config.agents.as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.model.as_ref())
            .and_then(|m| m.primary.as_deref());
        assert_eq!(primary, Some(MINIMAX_LOCAL_DEFAULT_MODEL));
    }

    #[test]
    fn ignores_unrelated() {
        assert!(handle_minimax(&make_params(AuthChoice::Token)).is_none());
    }
}
