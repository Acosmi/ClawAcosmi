/// Auth choice types: the primary `AuthChoice` enum and `AuthChoiceGroupId`.
///
/// `AuthChoice` enumerates every supported authentication method across all
/// providers. `AuthChoiceGroupId` groups related choices by provider for the
/// interactive wizard prompt.
///
/// Source: `src/commands/onboard-types.ts`, `src/commands/auth-choice-options.ts`

use serde::{Deserialize, Serialize};

/// All supported authentication methods.
///
/// Each variant corresponds to a concrete auth flow the user can select during
/// onboarding. Legacy aliases (`Oauth`, `SetupToken`, `ClaudeCli`, `CodexCli`)
/// are preserved for backwards compatibility.
///
/// Source: `src/commands/onboard-types.ts` - `AuthChoice`
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub enum AuthChoice {
    /// Legacy alias for `setup-token` (backwards CLI compatibility).
    #[serde(rename = "oauth")]
    Oauth,
    /// Anthropic setup-token flow.
    #[serde(rename = "setup-token")]
    SetupToken,
    /// Claude CLI backend.
    #[serde(rename = "claude-cli")]
    ClaudeCli,
    /// Anthropic token flow (paste setup-token).
    #[serde(rename = "token")]
    Token,
    /// Chutes OAuth flow.
    #[serde(rename = "chutes")]
    Chutes,
    /// OpenAI Codex OAuth.
    #[serde(rename = "openai-codex")]
    OpenaiCodex,
    /// OpenAI API key.
    #[serde(rename = "openai-api-key")]
    OpenaiApiKey,
    /// OpenRouter API key.
    #[serde(rename = "openrouter-api-key")]
    OpenrouterApiKey,
    /// Vercel AI Gateway API key.
    #[serde(rename = "ai-gateway-api-key")]
    AiGatewayApiKey,
    /// Cloudflare AI Gateway API key.
    #[serde(rename = "cloudflare-ai-gateway-api-key")]
    CloudflareAiGatewayApiKey,
    /// Moonshot API key (.ai).
    #[serde(rename = "moonshot-api-key")]
    MoonshotApiKey,
    /// Moonshot API key (.cn).
    #[serde(rename = "moonshot-api-key-cn")]
    MoonshotApiKeyCn,
    /// Kimi Code API key (subscription).
    #[serde(rename = "kimi-code-api-key")]
    KimiCodeApiKey,
    /// Synthetic API key.
    #[serde(rename = "synthetic-api-key")]
    SyntheticApiKey,
    /// Venice AI API key.
    #[serde(rename = "venice-api-key")]
    VeniceApiKey,
    /// Codex CLI backend (legacy).
    #[serde(rename = "codex-cli")]
    CodexCli,
    /// Anthropic API key.
    #[serde(rename = "apiKey")]
    ApiKey,
    /// Google Gemini API key.
    #[serde(rename = "gemini-api-key")]
    GeminiApiKey,
    /// Google Antigravity OAuth.
    #[serde(rename = "google-antigravity")]
    GoogleAntigravity,
    /// Google Gemini CLI OAuth.
    #[serde(rename = "google-gemini-cli")]
    GoogleGeminiCli,
    /// Z.AI (GLM 4.7) API key.
    #[serde(rename = "zai-api-key")]
    ZaiApiKey,
    /// Xiaomi API key.
    #[serde(rename = "xiaomi-api-key")]
    XiaomiApiKey,
    /// MiniMax Cloud (legacy).
    #[serde(rename = "minimax-cloud")]
    MinimaxCloud,
    /// MiniMax local (LM Studio).
    #[serde(rename = "minimax")]
    Minimax,
    /// MiniMax M2.1 API.
    #[serde(rename = "minimax-api")]
    MinimaxApi,
    /// MiniMax M2.1 Lightning API.
    #[serde(rename = "minimax-api-lightning")]
    MinimaxApiLightning,
    /// MiniMax OAuth portal.
    #[serde(rename = "minimax-portal")]
    MinimaxPortal,
    /// OpenCode Zen multi-model proxy.
    #[serde(rename = "opencode-zen")]
    OpencodeZen,
    /// GitHub Copilot device login.
    #[serde(rename = "github-copilot")]
    GithubCopilot,
    /// Copilot Proxy (local).
    #[serde(rename = "copilot-proxy")]
    CopilotProxy,
    /// Qwen OAuth portal.
    #[serde(rename = "qwen-portal")]
    QwenPortal,
    /// xAI (Grok) API key.
    #[serde(rename = "xai-api-key")]
    XaiApiKey,
    /// Qianfan API key.
    #[serde(rename = "qianfan-api-key")]
    QianfanApiKey,
    /// Skip authentication (defer to later).
    #[serde(rename = "skip")]
    Skip,
}

impl AuthChoice {
    /// Return the kebab-case string representation used in the TS codebase.
    ///
    /// Source: `src/commands/onboard-types.ts`
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Oauth => "oauth",
            Self::SetupToken => "setup-token",
            Self::ClaudeCli => "claude-cli",
            Self::Token => "token",
            Self::Chutes => "chutes",
            Self::OpenaiCodex => "openai-codex",
            Self::OpenaiApiKey => "openai-api-key",
            Self::OpenrouterApiKey => "openrouter-api-key",
            Self::AiGatewayApiKey => "ai-gateway-api-key",
            Self::CloudflareAiGatewayApiKey => "cloudflare-ai-gateway-api-key",
            Self::MoonshotApiKey => "moonshot-api-key",
            Self::MoonshotApiKeyCn => "moonshot-api-key-cn",
            Self::KimiCodeApiKey => "kimi-code-api-key",
            Self::SyntheticApiKey => "synthetic-api-key",
            Self::VeniceApiKey => "venice-api-key",
            Self::CodexCli => "codex-cli",
            Self::ApiKey => "apiKey",
            Self::GeminiApiKey => "gemini-api-key",
            Self::GoogleAntigravity => "google-antigravity",
            Self::GoogleGeminiCli => "google-gemini-cli",
            Self::ZaiApiKey => "zai-api-key",
            Self::XiaomiApiKey => "xiaomi-api-key",
            Self::MinimaxCloud => "minimax-cloud",
            Self::Minimax => "minimax",
            Self::MinimaxApi => "minimax-api",
            Self::MinimaxApiLightning => "minimax-api-lightning",
            Self::MinimaxPortal => "minimax-portal",
            Self::OpencodeZen => "opencode-zen",
            Self::GithubCopilot => "github-copilot",
            Self::CopilotProxy => "copilot-proxy",
            Self::QwenPortal => "qwen-portal",
            Self::XaiApiKey => "xai-api-key",
            Self::QianfanApiKey => "qianfan-api-key",
            Self::Skip => "skip",
        }
    }
}

impl std::fmt::Display for AuthChoice {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(self.as_str())
    }
}

/// Provider group identifiers for the grouped auth-choice prompt.
///
/// Source: `src/commands/auth-choice-options.ts` - `AuthChoiceGroupId`
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub enum AuthChoiceGroupId {
    /// OpenAI group (Codex OAuth + API key).
    Openai,
    /// Anthropic group (setup-token + API key).
    Anthropic,
    /// Google group (Gemini API key + OAuth).
    Google,
    /// Copilot group (GitHub + local proxy).
    Copilot,
    /// OpenRouter group (API key).
    Openrouter,
    /// Vercel AI Gateway group (API key).
    AiGateway,
    /// Cloudflare AI Gateway group.
    CloudflareAiGateway,
    /// Moonshot AI group (Kimi K2.5 + Kimi Coding).
    Moonshot,
    /// Z.AI group (GLM 4.7 API key).
    Zai,
    /// Xiaomi group (API key).
    Xiaomi,
    /// OpenCode Zen group (API key).
    OpencodeZen,
    /// MiniMax group.
    Minimax,
    /// Synthetic group (Anthropic-compatible).
    Synthetic,
    /// Venice AI group.
    Venice,
    /// Qwen group (OAuth).
    Qwen,
    /// Qianfan group (API key).
    Qianfan,
    /// xAI (Grok) group.
    Xai,
}

impl AuthChoiceGroupId {
    /// Return the kebab-case string representation.
    ///
    /// Source: `src/commands/auth-choice-options.ts`
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Openai => "openai",
            Self::Anthropic => "anthropic",
            Self::Google => "google",
            Self::Copilot => "copilot",
            Self::Openrouter => "openrouter",
            Self::AiGateway => "ai-gateway",
            Self::CloudflareAiGateway => "cloudflare-ai-gateway",
            Self::Moonshot => "moonshot",
            Self::Zai => "zai",
            Self::Xiaomi => "xiaomi",
            Self::OpencodeZen => "opencode-zen",
            Self::Minimax => "minimax",
            Self::Synthetic => "synthetic",
            Self::Venice => "venice",
            Self::Qwen => "qwen",
            Self::Qianfan => "qianfan",
            Self::Xai => "xai",
        }
    }
}

impl std::fmt::Display for AuthChoiceGroupId {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(self.as_str())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn auth_choice_as_str_roundtrips() {
        let choices = [
            AuthChoice::Oauth,
            AuthChoice::SetupToken,
            AuthChoice::Token,
            AuthChoice::ApiKey,
            AuthChoice::OpenaiCodex,
            AuthChoice::OpenaiApiKey,
            AuthChoice::GeminiApiKey,
            AuthChoice::Skip,
        ];
        for choice in choices {
            let s = choice.as_str();
            assert!(!s.is_empty(), "as_str should not return empty for {choice:?}");
        }
    }

    #[test]
    fn auth_choice_display_matches_as_str() {
        let choice = AuthChoice::OpenaiCodex;
        assert_eq!(choice.to_string(), "openai-codex");
    }

    #[test]
    fn auth_choice_serde_roundtrip() {
        let choice = AuthChoice::CloudflareAiGatewayApiKey;
        let json = serde_json::to_string(&choice).expect("serialize");
        let restored: AuthChoice = serde_json::from_str(&json).expect("deserialize");
        assert_eq!(restored, choice);
    }

    #[test]
    fn auth_choice_deserialize_kebab() {
        let json = r#""openai-api-key""#;
        let choice: AuthChoice = serde_json::from_str(json).expect("deserialize");
        assert_eq!(choice, AuthChoice::OpenaiApiKey);
    }

    #[test]
    fn auth_choice_deserialize_api_key() {
        // "apiKey" is camelCase, not kebab-case -- special case
        let json = r#""apiKey""#;
        let choice: AuthChoice = serde_json::from_str(json).expect("deserialize");
        assert_eq!(choice, AuthChoice::ApiKey);
    }

    #[test]
    fn group_id_as_str() {
        assert_eq!(AuthChoiceGroupId::Openai.as_str(), "openai");
        assert_eq!(
            AuthChoiceGroupId::CloudflareAiGateway.as_str(),
            "cloudflare-ai-gateway"
        );
        assert_eq!(AuthChoiceGroupId::AiGateway.as_str(), "ai-gateway");
    }

    #[test]
    fn group_id_serde_roundtrip() {
        let group = AuthChoiceGroupId::Minimax;
        let json = serde_json::to_string(&group).expect("serialize");
        let restored: AuthChoiceGroupId = serde_json::from_str(&json).expect("deserialize");
        assert_eq!(restored, group);
    }
}
