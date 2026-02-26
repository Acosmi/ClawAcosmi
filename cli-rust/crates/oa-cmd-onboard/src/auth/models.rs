/// Model definition builders and constants for auth providers.
///
/// Contains base URLs, model IDs, cost structures, and builder functions
/// for constructing model definitions during onboarding auth setup.
///
/// Source: `src/commands/onboard-auth.models.ts`

use oa_types::models::{ModelCostConfig, ModelDefinitionConfig, ModelInputType};

// ── MiniMax constants ──

/// Default MiniMax base URL.
///
/// Source: `src/commands/onboard-auth.models.ts` - `DEFAULT_MINIMAX_BASE_URL`
pub const DEFAULT_MINIMAX_BASE_URL: &str = "https://api.minimax.io/v1";

/// MiniMax Anthropic-compatible API base URL.
///
/// Source: `src/commands/onboard-auth.models.ts` - `MINIMAX_API_BASE_URL`
pub const MINIMAX_API_BASE_URL: &str = "https://api.minimax.io/anthropic";

/// Default MiniMax hosted model ID.
///
/// Source: `src/commands/onboard-auth.models.ts` - `MINIMAX_HOSTED_MODEL_ID`
pub const MINIMAX_HOSTED_MODEL_ID: &str = "MiniMax-M2.1";

/// Default MiniMax hosted model reference.
///
/// Source: `src/commands/onboard-auth.models.ts` - `MINIMAX_HOSTED_MODEL_REF`
pub const MINIMAX_HOSTED_MODEL_REF: &str = "minimax/MiniMax-M2.1";

/// Default MiniMax context window size.
pub const DEFAULT_MINIMAX_CONTEXT_WINDOW: u64 = 200_000;

/// Default MiniMax max tokens.
pub const DEFAULT_MINIMAX_MAX_TOKENS: u64 = 8192;

// ── Moonshot constants ──

/// Moonshot base URL (international).
///
/// Source: `src/commands/onboard-auth.models.ts` - `MOONSHOT_BASE_URL`
pub const MOONSHOT_BASE_URL: &str = "https://api.moonshot.ai/v1";

/// Moonshot base URL (China).
///
/// Source: `src/commands/onboard-auth.models.ts` - `MOONSHOT_CN_BASE_URL`
pub const MOONSHOT_CN_BASE_URL: &str = "https://api.moonshot.cn/v1";

/// Moonshot default model ID.
///
/// Source: `src/commands/onboard-auth.models.ts` - `MOONSHOT_DEFAULT_MODEL_ID`
pub const MOONSHOT_DEFAULT_MODEL_ID: &str = "kimi-k2.5";

/// Moonshot default model reference.
///
/// Source: `src/commands/onboard-auth.models.ts` - `MOONSHOT_DEFAULT_MODEL_REF`
pub const MOONSHOT_DEFAULT_MODEL_REF: &str = "moonshot/kimi-k2.5";

/// Moonshot default context window.
pub const MOONSHOT_DEFAULT_CONTEXT_WINDOW: u64 = 256_000;

/// Moonshot default max tokens.
pub const MOONSHOT_DEFAULT_MAX_TOKENS: u64 = 8192;

// ── Kimi Coding constants ──

/// Kimi Coding model ID.
///
/// Source: `src/commands/onboard-auth.models.ts` - `KIMI_CODING_MODEL_ID`
pub const KIMI_CODING_MODEL_ID: &str = "k2p5";

/// Kimi Coding model reference.
///
/// Source: `src/commands/onboard-auth.models.ts` - `KIMI_CODING_MODEL_REF`
pub const KIMI_CODING_MODEL_REF: &str = "kimi-coding/k2p5";

// ── Qianfan constants ──

/// Qianfan base URL.
///
/// Source: `src/commands/onboard-auth.models.ts` - `QIANFAN_BASE_URL`
pub const QIANFAN_BASE_URL: &str = "https://qianfan.baidubce.com/v2";

/// Qianfan default model ID.
///
/// Source: `src/commands/onboard-auth.models.ts` - `QIANFAN_DEFAULT_MODEL_ID`
pub const QIANFAN_DEFAULT_MODEL_ID: &str = "ernie-4.5-8k";

/// Qianfan default model reference.
///
/// Source: `src/commands/onboard-auth.models.ts` - `QIANFAN_DEFAULT_MODEL_REF`
pub const QIANFAN_DEFAULT_MODEL_REF: &str = "qianfan/ernie-4.5-8k";

// ── xAI constants ──

/// xAI base URL.
///
/// Source: `src/commands/onboard-auth.models.ts` - `XAI_BASE_URL`
pub const XAI_BASE_URL: &str = "https://api.x.ai/v1";

/// xAI default model ID.
///
/// Source: `src/commands/onboard-auth.models.ts` - `XAI_DEFAULT_MODEL_ID`
pub const XAI_DEFAULT_MODEL_ID: &str = "grok-4";

/// xAI default context window.
pub const XAI_DEFAULT_CONTEXT_WINDOW: u64 = 131_072;

/// xAI default max tokens.
pub const XAI_DEFAULT_MAX_TOKENS: u64 = 8192;

// ── Cost configurations ──

/// Zero cost config for free/local models.
///
/// Source: `src/commands/onboard-auth.models.ts` - various `_COST` constants
pub const ZERO_COST: ModelCostConfig = ModelCostConfig {
    input: 0.0,
    output: 0.0,
    cache_read: 0.0,
    cache_write: 0.0,
};

/// MiniMax API cost config.
///
/// Source: `src/commands/onboard-auth.models.ts` - `MINIMAX_API_COST`
pub const MINIMAX_API_COST: ModelCostConfig = ModelCostConfig {
    input: 15.0,
    output: 60.0,
    cache_read: 2.0,
    cache_write: 10.0,
};

// ── Model builders ──

/// Build a Moonshot model definition.
///
/// Source: `src/commands/onboard-auth.models.ts` - `buildMoonshotModelDefinition`
pub fn build_moonshot_model_definition() -> ModelDefinitionConfig {
    ModelDefinitionConfig {
        id: MOONSHOT_DEFAULT_MODEL_ID.to_string(),
        name: "Kimi K2.5".to_string(),
        api: None,
        reasoning: false,
        input: vec![ModelInputType::Text],
        cost: ZERO_COST,
        context_window: MOONSHOT_DEFAULT_CONTEXT_WINDOW,
        max_tokens: MOONSHOT_DEFAULT_MAX_TOKENS,
        headers: None,
        compat: None,
    }
}

/// Build a xAI (Grok) model definition.
///
/// Source: `src/commands/onboard-auth.models.ts` - `buildXaiModelDefinition`
pub fn build_xai_model_definition() -> ModelDefinitionConfig {
    ModelDefinitionConfig {
        id: XAI_DEFAULT_MODEL_ID.to_string(),
        name: "Grok 4".to_string(),
        api: None,
        reasoning: false,
        input: vec![ModelInputType::Text],
        cost: ZERO_COST,
        context_window: XAI_DEFAULT_CONTEXT_WINDOW,
        max_tokens: XAI_DEFAULT_MAX_TOKENS,
        headers: None,
        compat: None,
    }
}

/// Parameters for building a MiniMax model definition.
///
/// Source: `src/commands/onboard-auth.models.ts` - `buildMinimaxModelDefinition` params
pub struct MinimaxModelParams {
    /// Model ID.
    pub id: String,
    /// Optional display name.
    pub name: Option<String>,
    /// Whether the model supports reasoning.
    pub reasoning: bool,
    /// Cost configuration.
    pub cost: ModelCostConfig,
    /// Context window size.
    pub context_window: u64,
    /// Maximum output tokens.
    pub max_tokens: u64,
}

/// Build a MiniMax model definition.
///
/// Source: `src/commands/onboard-auth.models.ts` - `buildMinimaxModelDefinition`
pub fn build_minimax_model_definition(params: MinimaxModelParams) -> ModelDefinitionConfig {
    let name = params
        .name
        .unwrap_or_else(|| format!("MiniMax {}", params.id));
    ModelDefinitionConfig {
        id: params.id,
        name,
        api: None,
        reasoning: params.reasoning,
        input: vec![ModelInputType::Text],
        cost: params.cost,
        context_window: params.context_window,
        max_tokens: params.max_tokens,
        headers: None,
        compat: None,
    }
}

/// Build a MiniMax API model definition with default parameters.
///
/// Source: `src/commands/onboard-auth.models.ts` - `buildMinimaxApiModelDefinition`
pub fn build_minimax_api_model_definition(model_id: &str) -> ModelDefinitionConfig {
    build_minimax_model_definition(MinimaxModelParams {
        id: model_id.to_string(),
        name: None,
        reasoning: false,
        cost: MINIMAX_API_COST,
        context_window: DEFAULT_MINIMAX_CONTEXT_WINDOW,
        max_tokens: DEFAULT_MINIMAX_MAX_TOKENS,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn moonshot_model_definition() {
        let model = build_moonshot_model_definition();
        assert_eq!(model.id, "kimi-k2.5");
        assert_eq!(model.name, "Kimi K2.5");
        assert!(!model.reasoning);
        assert_eq!(model.context_window, 256_000);
    }

    #[test]
    fn xai_model_definition() {
        let model = build_xai_model_definition();
        assert_eq!(model.id, "grok-4");
        assert_eq!(model.name, "Grok 4");
        assert_eq!(model.context_window, 131_072);
    }

    #[test]
    fn minimax_api_model_definition() {
        let model = build_minimax_api_model_definition("MiniMax-M2.1");
        assert_eq!(model.id, "MiniMax-M2.1");
        assert_eq!(model.cost.input, 15.0);
        assert_eq!(model.context_window, 200_000);
    }

    #[test]
    fn model_refs_consistent() {
        assert_eq!(
            MOONSHOT_DEFAULT_MODEL_REF,
            format!("moonshot/{MOONSHOT_DEFAULT_MODEL_ID}")
        );
        assert_eq!(
            MINIMAX_HOSTED_MODEL_REF,
            format!("minimax/{MINIMAX_HOSTED_MODEL_ID}")
        );
        assert_eq!(
            KIMI_CODING_MODEL_REF,
            format!("kimi-coding/{KIMI_CODING_MODEL_ID}")
        );
    }

    #[test]
    fn zero_cost_is_zero() {
        assert_eq!(ZERO_COST.input, 0.0);
        assert_eq!(ZERO_COST.output, 0.0);
        assert_eq!(ZERO_COST.cache_read, 0.0);
        assert_eq!(ZERO_COST.cache_write, 0.0);
    }
}
