/// Preferred provider mapping for each auth choice.
///
/// Maps each [`AuthChoice`] to its default provider identifier, which is
/// used to determine the `agents.defaults.model` provider when the user
/// completes an auth flow.
///
/// Source: `src/commands/auth-choice.preferred-provider.ts`

use crate::auth_choice::AuthChoice;

/// Resolve the preferred provider identifier for a given auth choice.
///
/// Returns `Some(provider_id)` for known auth choices, or `None` for
/// choices that do not have a preferred provider mapping (e.g., `Skip`).
///
/// Source: `src/commands/auth-choice.preferred-provider.ts` - `resolvePreferredProviderForAuthChoice`
#[must_use]
pub fn resolve_preferred_provider(choice: AuthChoice) -> Option<&'static str> {
    match choice {
        AuthChoice::Oauth
        | AuthChoice::SetupToken
        | AuthChoice::ClaudeCli
        | AuthChoice::Token
        | AuthChoice::ApiKey => Some("anthropic"),

        AuthChoice::OpenaiCodex | AuthChoice::CodexCli => Some("openai-codex"),

        AuthChoice::Chutes => Some("chutes"),
        AuthChoice::OpenaiApiKey => Some("openai"),
        AuthChoice::OpenrouterApiKey => Some("openrouter"),
        AuthChoice::AiGatewayApiKey => Some("vercel-ai-gateway"),
        AuthChoice::CloudflareAiGatewayApiKey => Some("cloudflare-ai-gateway"),
        AuthChoice::MoonshotApiKey | AuthChoice::MoonshotApiKeyCn => Some("moonshot"),
        AuthChoice::KimiCodeApiKey => Some("kimi-coding"),
        AuthChoice::GeminiApiKey => Some("google"),
        AuthChoice::GoogleAntigravity => Some("google-antigravity"),
        AuthChoice::GoogleGeminiCli => Some("google-gemini-cli"),
        AuthChoice::ZaiApiKey => Some("zai"),
        AuthChoice::XiaomiApiKey => Some("xiaomi"),
        AuthChoice::SyntheticApiKey => Some("synthetic"),
        AuthChoice::VeniceApiKey => Some("venice"),
        AuthChoice::GithubCopilot => Some("github-copilot"),
        AuthChoice::CopilotProxy => Some("copilot-proxy"),
        AuthChoice::MinimaxCloud | AuthChoice::MinimaxApi | AuthChoice::MinimaxApiLightning => {
            Some("minimax")
        }
        AuthChoice::Minimax => Some("lmstudio"),
        AuthChoice::OpencodeZen => Some("opencode"),
        AuthChoice::XaiApiKey => Some("xai"),
        AuthChoice::QwenPortal => Some("qwen-portal"),
        AuthChoice::MinimaxPortal => Some("minimax-portal"),
        AuthChoice::QianfanApiKey => Some("qianfan"),

        AuthChoice::Skip => None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn anthropic_choices_map_to_anthropic() {
        let anthropic_choices = [
            AuthChoice::Oauth,
            AuthChoice::SetupToken,
            AuthChoice::ClaudeCli,
            AuthChoice::Token,
            AuthChoice::ApiKey,
        ];
        for choice in anthropic_choices {
            assert_eq!(
                resolve_preferred_provider(choice),
                Some("anthropic"),
                "expected 'anthropic' for {choice:?}"
            );
        }
    }

    #[test]
    fn openai_codex_maps_correctly() {
        assert_eq!(
            resolve_preferred_provider(AuthChoice::OpenaiCodex),
            Some("openai-codex")
        );
    }

    #[test]
    fn openai_api_key_maps_to_openai() {
        assert_eq!(
            resolve_preferred_provider(AuthChoice::OpenaiApiKey),
            Some("openai")
        );
    }

    #[test]
    fn moonshot_variants_map_to_moonshot() {
        assert_eq!(
            resolve_preferred_provider(AuthChoice::MoonshotApiKey),
            Some("moonshot")
        );
        assert_eq!(
            resolve_preferred_provider(AuthChoice::MoonshotApiKeyCn),
            Some("moonshot")
        );
    }

    #[test]
    fn kimi_code_maps_to_kimi_coding() {
        assert_eq!(
            resolve_preferred_provider(AuthChoice::KimiCodeApiKey),
            Some("kimi-coding")
        );
    }

    #[test]
    fn xai_maps_to_xai() {
        assert_eq!(
            resolve_preferred_provider(AuthChoice::XaiApiKey),
            Some("xai")
        );
    }

    #[test]
    fn minimax_local_maps_to_lmstudio() {
        assert_eq!(
            resolve_preferred_provider(AuthChoice::Minimax),
            Some("lmstudio")
        );
    }

    #[test]
    fn minimax_api_variants_map_to_minimax() {
        assert_eq!(
            resolve_preferred_provider(AuthChoice::MinimaxApi),
            Some("minimax")
        );
        assert_eq!(
            resolve_preferred_provider(AuthChoice::MinimaxApiLightning),
            Some("minimax")
        );
        assert_eq!(
            resolve_preferred_provider(AuthChoice::MinimaxCloud),
            Some("minimax")
        );
    }

    #[test]
    fn skip_returns_none() {
        assert_eq!(resolve_preferred_provider(AuthChoice::Skip), None);
    }

    #[test]
    fn all_non_skip_choices_have_provider() {
        let choices = [
            AuthChoice::Oauth,
            AuthChoice::SetupToken,
            AuthChoice::Token,
            AuthChoice::ApiKey,
            AuthChoice::OpenaiCodex,
            AuthChoice::OpenaiApiKey,
            AuthChoice::Chutes,
            AuthChoice::OpenrouterApiKey,
            AuthChoice::AiGatewayApiKey,
            AuthChoice::CloudflareAiGatewayApiKey,
            AuthChoice::MoonshotApiKey,
            AuthChoice::MoonshotApiKeyCn,
            AuthChoice::KimiCodeApiKey,
            AuthChoice::GeminiApiKey,
            AuthChoice::GoogleAntigravity,
            AuthChoice::GoogleGeminiCli,
            AuthChoice::ZaiApiKey,
            AuthChoice::XiaomiApiKey,
            AuthChoice::SyntheticApiKey,
            AuthChoice::VeniceApiKey,
            AuthChoice::GithubCopilot,
            AuthChoice::CopilotProxy,
            AuthChoice::MinimaxCloud,
            AuthChoice::Minimax,
            AuthChoice::MinimaxApi,
            AuthChoice::MinimaxApiLightning,
            AuthChoice::MinimaxPortal,
            AuthChoice::OpencodeZen,
            AuthChoice::QwenPortal,
            AuthChoice::XaiApiKey,
            AuthChoice::QianfanApiKey,
            AuthChoice::ClaudeCli,
            AuthChoice::CodexCli,
        ];
        for choice in choices {
            assert!(
                resolve_preferred_provider(choice).is_some(),
                "expected Some for {choice:?}"
            );
        }
    }
}
