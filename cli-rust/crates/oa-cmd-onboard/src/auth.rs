/// Auth provider configuration for onboarding.
///
/// Contains model constants, credential storage types, and provider-specific
/// config application functions used during auth setup in the onboarding flow.
///
/// Source: `src/commands/onboard-auth.ts`, `src/commands/onboard-auth.config-core.ts`,
///         `src/commands/onboard-auth.config-minimax.ts`,
///         `src/commands/onboard-auth.config-opencode.ts`,
///         `src/commands/onboard-auth.credentials.ts`,
///         `src/commands/onboard-auth.models.ts`

pub mod credentials;
pub mod models;
pub mod config_core;

// ── Re-exports ──

pub use credentials::{
    CLOUDFLARE_AI_GATEWAY_DEFAULT_MODEL_REF, OPENROUTER_DEFAULT_MODEL_REF,
    VERCEL_AI_GATEWAY_DEFAULT_MODEL_REF, XAI_DEFAULT_MODEL_REF, XIAOMI_DEFAULT_MODEL_REF,
    ZAI_DEFAULT_MODEL_REF,
};
pub use models::{
    DEFAULT_MINIMAX_BASE_URL, KIMI_CODING_MODEL_ID, KIMI_CODING_MODEL_REF,
    MINIMAX_API_BASE_URL, MINIMAX_HOSTED_MODEL_ID, MINIMAX_HOSTED_MODEL_REF,
    MOONSHOT_BASE_URL, MOONSHOT_CN_BASE_URL, MOONSHOT_DEFAULT_MODEL_ID,
    MOONSHOT_DEFAULT_MODEL_REF, QIANFAN_BASE_URL, QIANFAN_DEFAULT_MODEL_ID,
    QIANFAN_DEFAULT_MODEL_REF, XAI_BASE_URL, XAI_DEFAULT_MODEL_ID,
};

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn model_refs_are_formatted_correctly() {
        assert!(MOONSHOT_DEFAULT_MODEL_REF.starts_with("moonshot/"));
        assert!(MINIMAX_HOSTED_MODEL_REF.starts_with("minimax/"));
        assert!(QIANFAN_DEFAULT_MODEL_REF.starts_with("qianfan/"));
        assert!(KIMI_CODING_MODEL_REF.starts_with("kimi-coding/"));
    }

    #[test]
    fn default_model_refs_not_empty() {
        assert!(!ZAI_DEFAULT_MODEL_REF.is_empty());
        assert!(!XIAOMI_DEFAULT_MODEL_REF.is_empty());
        assert!(!OPENROUTER_DEFAULT_MODEL_REF.is_empty());
        assert!(!VERCEL_AI_GATEWAY_DEFAULT_MODEL_REF.is_empty());
        assert!(!CLOUDFLARE_AI_GATEWAY_DEFAULT_MODEL_REF.is_empty());
        assert!(!XAI_DEFAULT_MODEL_REF.is_empty());
    }
}
