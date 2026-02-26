/// API provider auth choice handlers.
///
/// Handles API-key-based auth choices for many providers: OpenRouter,
/// Vercel AI Gateway, Cloudflare AI Gateway, Moonshot, Kimi Coding,
/// Gemini, Z.AI, Xiaomi, Synthetic, Venice, OpenCode Zen, and Qianfan.
///
/// Each handler configures the auth profile and sets the appropriate
/// default model.
///
/// Source: `src/commands/auth-choice.apply.api-providers.ts`

use oa_types::auth::AuthProfileMode;

use crate::auth_choice::AuthChoice;
use crate::default_model::{apply_default_model_choice, ensure_model_allowlist_entry, set_default_model_primary};

use super::{apply_auth_profile_config, ApplyAuthChoiceParams, ApplyAuthChoiceResult};

// ── Default model references ──

/// Source: `src/commands/onboard-auth.ts` - `OPENROUTER_DEFAULT_MODEL_REF`
pub const OPENROUTER_DEFAULT_MODEL_REF: &str = "openrouter/anthropic/claude-sonnet-4-5-20250514";

/// Source: `src/commands/onboard-auth.ts` - `VERCEL_AI_GATEWAY_DEFAULT_MODEL_REF`
pub const VERCEL_AI_GATEWAY_DEFAULT_MODEL_REF: &str = "vercel-ai-gateway/anthropic/claude-sonnet-4-5-20250514";

/// Source: `src/commands/onboard-auth.ts` - `CLOUDFLARE_AI_GATEWAY_DEFAULT_MODEL_REF`
pub const CLOUDFLARE_AI_GATEWAY_DEFAULT_MODEL_REF: &str = "cloudflare-ai-gateway/anthropic/claude-sonnet-4-5-20250514";

/// Source: `src/commands/onboard-auth.ts` - `MOONSHOT_DEFAULT_MODEL_REF`
pub const MOONSHOT_DEFAULT_MODEL_REF: &str = "moonshot/kimi-k2-0711";

/// Source: `src/commands/onboard-auth.ts` - `KIMI_CODING_MODEL_REF`
pub const KIMI_CODING_MODEL_REF: &str = "kimi-coding/kimi-coder-s-32k";

/// Source: `src/commands/google-gemini-model-default.ts` - `GOOGLE_GEMINI_DEFAULT_MODEL`
pub const GOOGLE_GEMINI_DEFAULT_MODEL: &str = "google/gemini-2.5-pro";

/// Source: `src/commands/onboard-auth.ts` - `ZAI_DEFAULT_MODEL_REF`
pub const ZAI_DEFAULT_MODEL_REF: &str = "zai/glm-4.7";

/// Source: `src/commands/onboard-auth.ts` - `XIAOMI_DEFAULT_MODEL_REF`
pub const XIAOMI_DEFAULT_MODEL_REF: &str = "xiaomi/MiMo-72B-A27B-RL";

/// Source: `src/commands/onboard-auth.ts` - `SYNTHETIC_DEFAULT_MODEL_REF`
pub const SYNTHETIC_DEFAULT_MODEL_REF: &str = "synthetic/claude-sonnet-4-5-20250514";

/// Source: `src/commands/onboard-auth.ts` - `VENICE_DEFAULT_MODEL_REF`
pub const VENICE_DEFAULT_MODEL_REF: &str = "venice/qwen3-235b";

/// Source: `src/commands/opencode-zen-model-default.ts` - `OPENCODE_ZEN_DEFAULT_MODEL`
pub const OPENCODE_ZEN_DEFAULT_MODEL: &str = "opencode/claude-sonnet-4-5-20250514";

/// Source: `src/commands/onboard-auth.ts` - `QIANFAN_DEFAULT_MODEL_REF`
pub const QIANFAN_DEFAULT_MODEL_REF: &str = "qianfan/ernie-4.5-8k";

/// Resolve a possibly-redirected auth choice based on `tokenProvider`.
///
/// When the user selects `ApiKey` with a non-Anthropic/non-OpenAI token
/// provider, this function maps to the appropriate specific auth choice.
///
/// Source: `src/commands/auth-choice.apply.api-providers.ts` - top of `applyAuthChoiceApiProviders`
fn resolve_auth_choice(params: &ApplyAuthChoiceParams) -> AuthChoice {
    let mut choice = params.auth_choice;

    if choice == AuthChoice::ApiKey {
        if let Some(ref tp) = params.opts.token_provider {
            if tp != "anthropic" && tp != "openai" {
                choice = match tp.as_str() {
                    "openrouter" => AuthChoice::OpenrouterApiKey,
                    "vercel-ai-gateway" => AuthChoice::AiGatewayApiKey,
                    "cloudflare-ai-gateway" => AuthChoice::CloudflareAiGatewayApiKey,
                    "moonshot" => AuthChoice::MoonshotApiKey,
                    "kimi-code" | "kimi-coding" => AuthChoice::KimiCodeApiKey,
                    "google" => AuthChoice::GeminiApiKey,
                    "zai" => AuthChoice::ZaiApiKey,
                    "xiaomi" => AuthChoice::XiaomiApiKey,
                    "synthetic" => AuthChoice::SyntheticApiKey,
                    "venice" => AuthChoice::VeniceApiKey,
                    "opencode" => AuthChoice::OpencodeZen,
                    "qianfan" => AuthChoice::QianfanApiKey,
                    _ => choice,
                };
            }
        }
    }
    choice
}

/// Handle API-provider auth choices.
///
/// Returns `Some(result)` if this handler handles the choice, or `None`
/// to pass to the next handler in the chain.
///
/// Source: `src/commands/auth-choice.apply.api-providers.ts` - `applyAuthChoiceApiProviders`
#[allow(clippy::too_many_lines)]
pub fn handle_api_providers(params: &ApplyAuthChoiceParams) -> Option<ApplyAuthChoiceResult> {
    let auth_choice = resolve_auth_choice(params);

    match auth_choice {
        AuthChoice::OpenrouterApiKey => {
            let config = apply_auth_profile_config(
                params.config.clone(),
                "openrouter:default",
                "openrouter",
                AuthProfileMode::ApiKey,
                None,
            );
            let result = apply_default_model_choice(
                config,
                params.set_default_model,
                OPENROUTER_DEFAULT_MODEL_REF,
                |cfg| set_default_model_primary(cfg, OPENROUTER_DEFAULT_MODEL_REF),
                |cfg| ensure_model_allowlist_entry(cfg, OPENROUTER_DEFAULT_MODEL_REF),
            );
            Some(ApplyAuthChoiceResult {
                config: result.config,
                agent_model_override: result.agent_model_override,
            })
        }

        AuthChoice::AiGatewayApiKey => {
            let config = apply_auth_profile_config(
                params.config.clone(),
                "vercel-ai-gateway:default",
                "vercel-ai-gateway",
                AuthProfileMode::ApiKey,
                None,
            );
            let result = apply_default_model_choice(
                config,
                params.set_default_model,
                VERCEL_AI_GATEWAY_DEFAULT_MODEL_REF,
                |cfg| set_default_model_primary(cfg, VERCEL_AI_GATEWAY_DEFAULT_MODEL_REF),
                |cfg| ensure_model_allowlist_entry(cfg, VERCEL_AI_GATEWAY_DEFAULT_MODEL_REF),
            );
            Some(ApplyAuthChoiceResult {
                config: result.config,
                agent_model_override: result.agent_model_override,
            })
        }

        AuthChoice::CloudflareAiGatewayApiKey => {
            let config = apply_auth_profile_config(
                params.config.clone(),
                "cloudflare-ai-gateway:default",
                "cloudflare-ai-gateway",
                AuthProfileMode::ApiKey,
                None,
            );
            let result = apply_default_model_choice(
                config,
                params.set_default_model,
                CLOUDFLARE_AI_GATEWAY_DEFAULT_MODEL_REF,
                |cfg| set_default_model_primary(cfg, CLOUDFLARE_AI_GATEWAY_DEFAULT_MODEL_REF),
                |cfg| ensure_model_allowlist_entry(cfg, CLOUDFLARE_AI_GATEWAY_DEFAULT_MODEL_REF),
            );
            Some(ApplyAuthChoiceResult {
                config: result.config,
                agent_model_override: result.agent_model_override,
            })
        }

        AuthChoice::MoonshotApiKey => {
            let config = apply_auth_profile_config(
                params.config.clone(),
                "moonshot:default",
                "moonshot",
                AuthProfileMode::ApiKey,
                None,
            );
            let result = apply_default_model_choice(
                config,
                params.set_default_model,
                MOONSHOT_DEFAULT_MODEL_REF,
                |cfg| set_default_model_primary(cfg, MOONSHOT_DEFAULT_MODEL_REF),
                |cfg| ensure_model_allowlist_entry(cfg, MOONSHOT_DEFAULT_MODEL_REF),
            );
            Some(ApplyAuthChoiceResult {
                config: result.config,
                agent_model_override: result.agent_model_override,
            })
        }

        AuthChoice::MoonshotApiKeyCn => {
            let config = apply_auth_profile_config(
                params.config.clone(),
                "moonshot:default",
                "moonshot",
                AuthProfileMode::ApiKey,
                None,
            );
            // CN variant uses the same model ref but different base URL config
            let result = apply_default_model_choice(
                config,
                params.set_default_model,
                MOONSHOT_DEFAULT_MODEL_REF,
                |cfg| set_default_model_primary(cfg, MOONSHOT_DEFAULT_MODEL_REF),
                |cfg| ensure_model_allowlist_entry(cfg, MOONSHOT_DEFAULT_MODEL_REF),
            );
            Some(ApplyAuthChoiceResult {
                config: result.config,
                agent_model_override: result.agent_model_override,
            })
        }

        AuthChoice::KimiCodeApiKey => {
            let config = apply_auth_profile_config(
                params.config.clone(),
                "kimi-coding:default",
                "kimi-coding",
                AuthProfileMode::ApiKey,
                None,
            );
            let result = apply_default_model_choice(
                config,
                params.set_default_model,
                KIMI_CODING_MODEL_REF,
                |cfg| set_default_model_primary(cfg, KIMI_CODING_MODEL_REF),
                |cfg| ensure_model_allowlist_entry(cfg, KIMI_CODING_MODEL_REF),
            );
            Some(ApplyAuthChoiceResult {
                config: result.config,
                agent_model_override: result.agent_model_override,
            })
        }

        AuthChoice::GeminiApiKey => {
            let config = apply_auth_profile_config(
                params.config.clone(),
                "google:default",
                "google",
                AuthProfileMode::ApiKey,
                None,
            );
            let result = apply_default_model_choice(
                config,
                params.set_default_model,
                GOOGLE_GEMINI_DEFAULT_MODEL,
                |cfg| set_default_model_primary(cfg, GOOGLE_GEMINI_DEFAULT_MODEL),
                |cfg| ensure_model_allowlist_entry(cfg, GOOGLE_GEMINI_DEFAULT_MODEL),
            );
            Some(ApplyAuthChoiceResult {
                config: result.config,
                agent_model_override: result.agent_model_override,
            })
        }

        AuthChoice::ZaiApiKey => {
            let config = apply_auth_profile_config(
                params.config.clone(),
                "zai:default",
                "zai",
                AuthProfileMode::ApiKey,
                None,
            );
            let result = apply_default_model_choice(
                config,
                params.set_default_model,
                ZAI_DEFAULT_MODEL_REF,
                |cfg| set_default_model_primary(cfg, ZAI_DEFAULT_MODEL_REF),
                |cfg| ensure_model_allowlist_entry(cfg, ZAI_DEFAULT_MODEL_REF),
            );
            Some(ApplyAuthChoiceResult {
                config: result.config,
                agent_model_override: result.agent_model_override,
            })
        }

        AuthChoice::XiaomiApiKey => {
            let config = apply_auth_profile_config(
                params.config.clone(),
                "xiaomi:default",
                "xiaomi",
                AuthProfileMode::ApiKey,
                None,
            );
            let result = apply_default_model_choice(
                config,
                params.set_default_model,
                XIAOMI_DEFAULT_MODEL_REF,
                |cfg| set_default_model_primary(cfg, XIAOMI_DEFAULT_MODEL_REF),
                |cfg| ensure_model_allowlist_entry(cfg, XIAOMI_DEFAULT_MODEL_REF),
            );
            Some(ApplyAuthChoiceResult {
                config: result.config,
                agent_model_override: result.agent_model_override,
            })
        }

        AuthChoice::SyntheticApiKey => {
            let config = apply_auth_profile_config(
                params.config.clone(),
                "synthetic:default",
                "synthetic",
                AuthProfileMode::ApiKey,
                None,
            );
            let result = apply_default_model_choice(
                config,
                params.set_default_model,
                SYNTHETIC_DEFAULT_MODEL_REF,
                |cfg| set_default_model_primary(cfg, SYNTHETIC_DEFAULT_MODEL_REF),
                |cfg| ensure_model_allowlist_entry(cfg, SYNTHETIC_DEFAULT_MODEL_REF),
            );
            Some(ApplyAuthChoiceResult {
                config: result.config,
                agent_model_override: result.agent_model_override,
            })
        }

        AuthChoice::VeniceApiKey => {
            let config = apply_auth_profile_config(
                params.config.clone(),
                "venice:default",
                "venice",
                AuthProfileMode::ApiKey,
                None,
            );
            let result = apply_default_model_choice(
                config,
                params.set_default_model,
                VENICE_DEFAULT_MODEL_REF,
                |cfg| set_default_model_primary(cfg, VENICE_DEFAULT_MODEL_REF),
                |cfg| ensure_model_allowlist_entry(cfg, VENICE_DEFAULT_MODEL_REF),
            );
            Some(ApplyAuthChoiceResult {
                config: result.config,
                agent_model_override: result.agent_model_override,
            })
        }

        AuthChoice::OpencodeZen => {
            let config = apply_auth_profile_config(
                params.config.clone(),
                "opencode:default",
                "opencode",
                AuthProfileMode::ApiKey,
                None,
            );
            let result = apply_default_model_choice(
                config,
                params.set_default_model,
                OPENCODE_ZEN_DEFAULT_MODEL,
                |cfg| set_default_model_primary(cfg, OPENCODE_ZEN_DEFAULT_MODEL),
                |cfg| ensure_model_allowlist_entry(cfg, OPENCODE_ZEN_DEFAULT_MODEL),
            );
            Some(ApplyAuthChoiceResult {
                config: result.config,
                agent_model_override: result.agent_model_override,
            })
        }

        AuthChoice::QianfanApiKey => {
            let config = apply_auth_profile_config(
                params.config.clone(),
                "qianfan:default",
                "qianfan",
                AuthProfileMode::ApiKey,
                None,
            );
            let result = apply_default_model_choice(
                config,
                params.set_default_model,
                QIANFAN_DEFAULT_MODEL_REF,
                |cfg| set_default_model_primary(cfg, QIANFAN_DEFAULT_MODEL_REF),
                |cfg| ensure_model_allowlist_entry(cfg, QIANFAN_DEFAULT_MODEL_REF),
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

    fn make_params(choice: AuthChoice, set_default: bool) -> ApplyAuthChoiceParams {
        ApplyAuthChoiceParams {
            auth_choice: choice,
            config: OpenAcosmiConfig::default(),
            set_default_model: set_default,
            agent_id: None,
            agent_dir: None,
            opts: ApplyAuthChoiceOpts::default(),
        }
    }

    #[test]
    fn handles_openrouter() {
        let result = handle_api_providers(&make_params(AuthChoice::OpenrouterApiKey, true));
        assert!(result.is_some());
        let r = result.expect("should be some");
        let profiles = r.config.auth.as_ref()
            .and_then(|a| a.profiles.as_ref())
            .expect("profiles");
        assert!(profiles.contains_key("openrouter:default"));
    }

    #[test]
    fn handles_ai_gateway() {
        let result = handle_api_providers(&make_params(AuthChoice::AiGatewayApiKey, true));
        assert!(result.is_some());
    }

    #[test]
    fn handles_cloudflare() {
        let result = handle_api_providers(&make_params(AuthChoice::CloudflareAiGatewayApiKey, true));
        assert!(result.is_some());
    }

    #[test]
    fn handles_moonshot() {
        let result = handle_api_providers(&make_params(AuthChoice::MoonshotApiKey, true));
        assert!(result.is_some());
    }

    #[test]
    fn handles_moonshot_cn() {
        let result = handle_api_providers(&make_params(AuthChoice::MoonshotApiKeyCn, true));
        assert!(result.is_some());
    }

    #[test]
    fn handles_kimi_code() {
        let result = handle_api_providers(&make_params(AuthChoice::KimiCodeApiKey, true));
        assert!(result.is_some());
    }

    #[test]
    fn handles_gemini() {
        let result = handle_api_providers(&make_params(AuthChoice::GeminiApiKey, true));
        assert!(result.is_some());
        let r = result.expect("should be some");
        let primary = r.config.agents.as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.model.as_ref())
            .and_then(|m| m.primary.as_deref());
        assert_eq!(primary, Some(GOOGLE_GEMINI_DEFAULT_MODEL));
    }

    #[test]
    fn handles_zai() {
        let result = handle_api_providers(&make_params(AuthChoice::ZaiApiKey, true));
        assert!(result.is_some());
    }

    #[test]
    fn handles_xiaomi() {
        let result = handle_api_providers(&make_params(AuthChoice::XiaomiApiKey, true));
        assert!(result.is_some());
    }

    #[test]
    fn handles_synthetic() {
        let result = handle_api_providers(&make_params(AuthChoice::SyntheticApiKey, true));
        assert!(result.is_some());
    }

    #[test]
    fn handles_venice() {
        let result = handle_api_providers(&make_params(AuthChoice::VeniceApiKey, true));
        assert!(result.is_some());
    }

    #[test]
    fn handles_opencode_zen() {
        let result = handle_api_providers(&make_params(AuthChoice::OpencodeZen, true));
        assert!(result.is_some());
    }

    #[test]
    fn handles_qianfan() {
        let result = handle_api_providers(&make_params(AuthChoice::QianfanApiKey, true));
        assert!(result.is_some());
    }

    #[test]
    fn api_key_redirects_to_openrouter() {
        let mut params = make_params(AuthChoice::ApiKey, true);
        params.opts.token_provider = Some("openrouter".to_owned());
        let result = handle_api_providers(&params);
        assert!(result.is_some());
    }

    #[test]
    fn api_key_redirects_to_zai() {
        let mut params = make_params(AuthChoice::ApiKey, true);
        params.opts.token_provider = Some("zai".to_owned());
        let result = handle_api_providers(&params);
        assert!(result.is_some());
    }

    #[test]
    fn agent_override_when_not_default_model() {
        let result = handle_api_providers(&make_params(AuthChoice::OpenrouterApiKey, false));
        assert!(result.is_some());
        let r = result.expect("should be some");
        assert_eq!(
            r.agent_model_override.as_deref(),
            Some(OPENROUTER_DEFAULT_MODEL_REF)
        );
    }

    #[test]
    fn ignores_unrelated_choices() {
        assert!(handle_api_providers(&make_params(AuthChoice::Token, true)).is_none());
        assert!(handle_api_providers(&make_params(AuthChoice::OpenaiCodex, true)).is_none());
    }
}
