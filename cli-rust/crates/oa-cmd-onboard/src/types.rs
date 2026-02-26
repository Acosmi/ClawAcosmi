/// Onboarding type definitions.
///
/// Contains all option types, enums, and structs used across the onboarding
/// flow including auth choices, gateway bind modes, tailscale modes, reset
/// scopes, and the main `OnboardOptions` struct.
///
/// Source: `src/commands/onboard-types.ts`

use serde::{Deserialize, Serialize};

/// Onboarding mode: local or remote gateway.
///
/// Source: `src/commands/onboard-types.ts` - `OnboardMode`
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum OnboardMode {
    /// Local gateway (this machine).
    Local,
    /// Remote gateway (info-only).
    Remote,
}

/// Auth provider choice during onboarding.
///
/// Source: `src/commands/onboard-types.ts` - `AuthChoice`
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub enum AuthChoice {
    /// Anthropic OAuth setup-token flow.
    SetupToken,
    /// Direct API token.
    Token,
    /// Anthropic API key.
    ApiKey,
    /// Google Gemini API key.
    GeminiApiKey,
    /// OpenAI Codex OAuth.
    OpenaiCodex,
    /// OpenAI API key.
    OpenaiApiKey,
    /// OpenRouter API key.
    OpenrouterApiKey,
    /// Vercel AI Gateway API key.
    AiGatewayApiKey,
    /// Cloudflare AI Gateway API key.
    CloudflareAiGatewayApiKey,
    /// Moonshot API key.
    MoonshotApiKey,
    /// Moonshot CN API key.
    MoonshotApiKeyCn,
    /// Kimi Code API key.
    KimiCodeApiKey,
    /// Synthetic API key.
    SyntheticApiKey,
    /// Venice AI API key.
    VeniceApiKey,
    /// ZAI API key.
    ZaiApiKey,
    /// Xiaomi API key.
    XiaomiApiKey,
    /// MiniMax local (LM Studio).
    Minimax,
    /// MiniMax cloud hosted.
    MinimaxCloud,
    /// MiniMax API.
    MinimaxApi,
    /// MiniMax API Lightning.
    MinimaxApiLightning,
    /// MiniMax portal.
    MinimaxPortal,
    /// Opencode Zen.
    OpencodeZen,
    /// GitHub Copilot.
    GithubCopilot,
    /// Copilot proxy.
    CopilotProxy,
    /// Qwen portal.
    QwenPortal,
    /// xAI API key.
    XaiApiKey,
    /// Qianfan API key.
    QianfanApiKey,
    /// Google Antigravity.
    GoogleAntigravity,
    /// Google Gemini CLI.
    GoogleGeminiCli,
    /// Chutes AI.
    Chutes,
    /// Skip auth configuration.
    Skip,
}

/// Gateway authentication choice.
///
/// Source: `src/commands/onboard-types.ts` - `GatewayAuthChoice`
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum GatewayAuthChoice {
    /// Token-based auth (recommended).
    Token,
    /// Password-based auth.
    Password,
}

/// Reset scope for the `--reset` flag.
///
/// Source: `src/commands/onboard-types.ts` - `ResetScope`
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ResetScope {
    /// Reset config file only.
    Config,
    /// Reset config + credentials + sessions.
    ConfigCredsAndSessions,
    /// Full reset including workspace.
    Full,
}

/// Gateway bind mode.
///
/// Source: `src/commands/onboard-types.ts` - `GatewayBind`
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum GatewayBind {
    /// Bind to 127.0.0.1.
    Loopback,
    /// Bind to 0.0.0.0.
    Lan,
    /// Auto-detect (prefer loopback).
    Auto,
    /// Custom IP address.
    Custom,
    /// Tailscale IP.
    Tailnet,
}

/// Tailscale exposure mode.
///
/// Source: `src/commands/onboard-types.ts` - `TailscaleMode`
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum TailscaleMode {
    /// No Tailscale exposure.
    Off,
    /// Private HTTPS for tailnet.
    Serve,
    /// Public HTTPS via Tailscale Funnel.
    Funnel,
}

/// Node package manager choice for skill installs.
///
/// Source: `src/commands/onboard-types.ts` - `NodeManagerChoice`
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum NodeManagerChoice {
    /// npm.
    Npm,
    /// pnpm.
    Pnpm,
    /// bun.
    Bun,
}

/// Onboarding flow type.
///
/// Source: `src/commands/onboard-types.ts` - `OnboardOptions.flow`
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum OnboardFlow {
    /// Quick-start guided flow.
    Quickstart,
    /// Advanced manual flow.
    Advanced,
}

/// Options for the onboard command.
///
/// Source: `src/commands/onboard-types.ts` - `OnboardOptions`
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct OnboardOptions {
    /// Gateway mode (local or remote).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mode: Option<String>,
    /// Onboarding flow type.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub flow: Option<String>,
    /// Workspace directory override.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub workspace: Option<String>,
    /// Run in non-interactive mode.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub non_interactive: Option<bool>,
    /// Accept risk for non-interactive mode.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub accept_risk: Option<bool>,
    /// Reset existing config before onboarding.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub reset: Option<bool>,
    /// Auth provider choice.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub auth_choice: Option<String>,
    /// Token provider for non-interactive token auth.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub token_provider: Option<String>,
    /// Token value for non-interactive token auth.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub token: Option<String>,
    /// Token profile ID for non-interactive token auth.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub token_profile_id: Option<String>,
    /// Token expiration for non-interactive token auth.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub token_expires_in: Option<String>,
    /// Anthropic API key.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub anthropic_api_key: Option<String>,
    /// OpenAI API key.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub openai_api_key: Option<String>,
    /// OpenRouter API key.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub openrouter_api_key: Option<String>,
    /// Vercel AI Gateway API key.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub ai_gateway_api_key: Option<String>,
    /// Cloudflare AI Gateway account ID.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cloudflare_ai_gateway_account_id: Option<String>,
    /// Cloudflare AI Gateway gateway ID.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cloudflare_ai_gateway_gateway_id: Option<String>,
    /// Cloudflare AI Gateway API key.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cloudflare_ai_gateway_api_key: Option<String>,
    /// Moonshot API key.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub moonshot_api_key: Option<String>,
    /// Kimi Code API key.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub kimi_code_api_key: Option<String>,
    /// Gemini API key.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub gemini_api_key: Option<String>,
    /// ZAI API key.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub zai_api_key: Option<String>,
    /// Xiaomi API key.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub xiaomi_api_key: Option<String>,
    /// MiniMax API key.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub minimax_api_key: Option<String>,
    /// Synthetic API key.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub synthetic_api_key: Option<String>,
    /// Venice API key.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub venice_api_key: Option<String>,
    /// Opencode Zen API key.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub opencode_zen_api_key: Option<String>,
    /// xAI API key.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub xai_api_key: Option<String>,
    /// Qianfan API key.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub qianfan_api_key: Option<String>,
    /// Gateway port.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub gateway_port: Option<u16>,
    /// Gateway bind mode.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub gateway_bind: Option<String>,
    /// Gateway auth type.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub gateway_auth: Option<String>,
    /// Gateway token.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub gateway_token: Option<String>,
    /// Gateway password.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub gateway_password: Option<String>,
    /// Tailscale mode.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tailscale: Option<String>,
    /// Tailscale reset-on-exit.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tailscale_reset_on_exit: Option<bool>,
    /// Install daemon service.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub install_daemon: Option<bool>,
    /// Daemon runtime override.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub daemon_runtime: Option<String>,
    /// Skip channel configuration.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub skip_channels: Option<bool>,
    /// Skip skills configuration.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub skip_skills: Option<bool>,
    /// Skip health check.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub skip_health: Option<bool>,
    /// Skip UI launch.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub skip_ui: Option<bool>,
    /// Node manager for skill installs.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub node_manager: Option<String>,
    /// Remote gateway URL.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub remote_url: Option<String>,
    /// Remote gateway token.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub remote_token: Option<String>,
    /// Output as JSON.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub json: Option<bool>,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn onboard_options_default() {
        let opts = OnboardOptions::default();
        assert!(opts.mode.is_none());
        assert!(opts.non_interactive.is_none());
        assert!(opts.auth_choice.is_none());
    }

    #[test]
    fn onboard_options_serialize_roundtrip() {
        let opts = OnboardOptions {
            mode: Some("local".to_string()),
            gateway_port: Some(18789),
            non_interactive: Some(true),
            accept_risk: Some(true),
            ..Default::default()
        };
        let json = serde_json::to_string(&opts).expect("serialize");
        let deserialized: OnboardOptions = serde_json::from_str(&json).expect("deserialize");
        assert_eq!(deserialized.mode.as_deref(), Some("local"));
        assert_eq!(deserialized.gateway_port, Some(18789));
    }

    #[test]
    fn reset_scope_variants() {
        assert_ne!(ResetScope::Config, ResetScope::Full);
        assert_ne!(ResetScope::ConfigCredsAndSessions, ResetScope::Config);
    }

    #[test]
    fn onboard_mode_serialize() {
        let json = serde_json::to_string(&OnboardMode::Local).expect("serialize");
        assert_eq!(json, "\"local\"");
        let json = serde_json::to_string(&OnboardMode::Remote).expect("serialize");
        assert_eq!(json, "\"remote\"");
    }

    #[test]
    fn gateway_bind_variants() {
        let binds = [
            GatewayBind::Loopback,
            GatewayBind::Lan,
            GatewayBind::Auto,
            GatewayBind::Custom,
            GatewayBind::Tailnet,
        ];
        assert_eq!(binds.len(), 5);
    }

    #[test]
    fn node_manager_serialize() {
        let json = serde_json::to_string(&NodeManagerChoice::Npm).expect("serialize");
        assert_eq!(json, "\"npm\"");
    }
}
