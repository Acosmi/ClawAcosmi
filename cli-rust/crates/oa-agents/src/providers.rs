/// Provider metadata and resolution for known AI model providers.
///
/// Provides a static registry of known provider metadata (display names,
/// API URL patterns) and helper functions for resolving provider information
/// by identifier.
///
/// Source: `src/agents/models-config.providers.ts`

use serde::{Deserialize, Serialize};

/// Metadata for a known AI model provider.
///
/// Source: `src/agents/models-config.providers.ts` - provider definitions
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ProviderMeta {
    /// Normalized provider identifier (e.g. `"anthropic"`, `"openai"`).
    pub id: &'static str,
    /// Human-readable display name.
    pub display_name: &'static str,
    /// API base URL pattern for this provider, if known.
    pub api_url_pattern: &'static str,
}

/// Static registry of known AI model providers.
///
/// This list covers the major providers supported by OpenAcosmi,
/// including cloud APIs, self-hosted, and CLI-backed providers.
///
/// Source: `src/agents/models-config.providers.ts` - provider configurations
pub const KNOWN_PROVIDERS: &[ProviderMeta] = &[
    ProviderMeta {
        id: "anthropic",
        display_name: "Anthropic",
        api_url_pattern: "https://api.anthropic.com",
    },
    ProviderMeta {
        id: "openai",
        display_name: "OpenAI",
        api_url_pattern: "https://api.openai.com/v1",
    },
    ProviderMeta {
        id: "google",
        display_name: "Google",
        api_url_pattern: "https://generativelanguage.googleapis.com",
    },
    ProviderMeta {
        id: "github-copilot",
        display_name: "GitHub Copilot",
        api_url_pattern: "https://api.individual.githubcopilot.com",
    },
    ProviderMeta {
        id: "amazon-bedrock",
        display_name: "Amazon Bedrock",
        api_url_pattern: "https://bedrock-runtime.*.amazonaws.com",
    },
    ProviderMeta {
        id: "mistral",
        display_name: "Mistral",
        api_url_pattern: "https://api.mistral.ai/v1",
    },
    ProviderMeta {
        id: "groq",
        display_name: "Groq",
        api_url_pattern: "https://api.groq.com/openai/v1",
    },
    ProviderMeta {
        id: "deepseek",
        display_name: "DeepSeek",
        api_url_pattern: "https://api.deepseek.com",
    },
    ProviderMeta {
        id: "openrouter",
        display_name: "OpenRouter",
        api_url_pattern: "https://openrouter.ai/api/v1",
    },
    ProviderMeta {
        id: "together",
        display_name: "Together AI",
        api_url_pattern: "https://api.together.xyz/v1",
    },
    ProviderMeta {
        id: "fireworks",
        display_name: "Fireworks AI",
        api_url_pattern: "https://api.fireworks.ai/inference/v1",
    },
    ProviderMeta {
        id: "perplexity",
        display_name: "Perplexity",
        api_url_pattern: "https://api.perplexity.ai",
    },
    ProviderMeta {
        id: "zai",
        display_name: "Z.AI",
        api_url_pattern: "https://api.z.ai/v1",
    },
    ProviderMeta {
        id: "minimax",
        display_name: "MiniMax",
        api_url_pattern: "https://api.minimax.chat/v1",
    },
    ProviderMeta {
        id: "minimax-portal",
        display_name: "MiniMax Portal",
        api_url_pattern: "https://api.minimax.io/anthropic",
    },
    ProviderMeta {
        id: "moonshot",
        display_name: "Moonshot (Kimi)",
        api_url_pattern: "https://api.moonshot.ai/v1",
    },
    ProviderMeta {
        id: "qwen-portal",
        display_name: "Qwen Portal",
        api_url_pattern: "https://portal.qwen.ai/v1",
    },
    ProviderMeta {
        id: "venice",
        display_name: "Venice AI",
        api_url_pattern: "https://api.venice.ai/api/v1",
    },
    ProviderMeta {
        id: "ollama",
        display_name: "Ollama (Local)",
        api_url_pattern: "http://127.0.0.1:11434/v1",
    },
    ProviderMeta {
        id: "xiaomi",
        display_name: "Xiaomi",
        api_url_pattern: "https://api.xiaomimimo.com/anthropic",
    },
    ProviderMeta {
        id: "qianfan",
        display_name: "Baidu Qianfan",
        api_url_pattern: "https://qianfan.baidubce.com/v2",
    },
    ProviderMeta {
        id: "cloudflare-ai-gateway",
        display_name: "Cloudflare AI Gateway",
        api_url_pattern: "https://gateway.ai.cloudflare.com",
    },
    ProviderMeta {
        id: "kimi-coding",
        display_name: "Kimi Coding",
        api_url_pattern: "https://api.moonshot.ai/v1",
    },
    ProviderMeta {
        id: "opencode",
        display_name: "OpenCode",
        api_url_pattern: "",
    },
    ProviderMeta {
        id: "claude-cli",
        display_name: "Claude CLI",
        api_url_pattern: "",
    },
    ProviderMeta {
        id: "codex-cli",
        display_name: "Codex CLI",
        api_url_pattern: "",
    },
    ProviderMeta {
        id: "synthetic",
        display_name: "Synthetic",
        api_url_pattern: "",
    },
];

/// Look up provider metadata by normalized provider identifier.
///
/// Returns `None` if the provider is not in the known registry.
///
/// Source: `src/agents/models-config.providers.ts`
pub fn resolve_provider_meta(provider_id: &str) -> Option<&'static ProviderMeta> {
    let normalized = provider_id.trim().to_lowercase();
    KNOWN_PROVIDERS.iter().find(|p| p.id == normalized)
}

/// Normalize a Google model ID, mapping shorthand names to their
/// full preview identifiers.
///
/// Source: `src/agents/models-config.providers.ts` - `normalizeGoogleModelId`
pub fn normalize_google_model_id(id: &str) -> String {
    match id {
        "gemini-3-pro" => "gemini-3-pro-preview".to_owned(),
        "gemini-3-flash" => "gemini-3-flash-preview".to_owned(),
        _ => id.to_owned(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn resolve_known_provider() {
        let meta = resolve_provider_meta("anthropic");
        assert!(meta.is_some());
        let meta = meta.expect("provider should exist");
        assert_eq!(meta.display_name, "Anthropic");
    }

    #[test]
    fn resolve_provider_case_insensitive() {
        let meta = resolve_provider_meta("OpenAI");
        assert!(meta.is_some());
        let meta = meta.expect("provider should exist");
        assert_eq!(meta.id, "openai");
    }

    #[test]
    fn resolve_unknown_provider() {
        assert!(resolve_provider_meta("nonexistent-provider").is_none());
    }

    #[test]
    fn known_providers_includes_major_providers() {
        let ids: Vec<&str> = KNOWN_PROVIDERS.iter().map(|p| p.id).collect();
        assert!(ids.contains(&"anthropic"));
        assert!(ids.contains(&"openai"));
        assert!(ids.contains(&"google"));
        assert!(ids.contains(&"deepseek"));
    }

    #[test]
    fn normalize_google_model_shorthand() {
        assert_eq!(normalize_google_model_id("gemini-3-pro"), "gemini-3-pro-preview");
        assert_eq!(
            normalize_google_model_id("gemini-3-flash"),
            "gemini-3-flash-preview"
        );
        assert_eq!(
            normalize_google_model_id("gemini-2.0-flash"),
            "gemini-2.0-flash"
        );
    }
}
