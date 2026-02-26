/// Credential storage and default model reference constants.
///
/// Provides functions to persist API keys and OAuth credentials to disk,
/// along with default model reference constants for each provider.
///
/// Source: `src/commands/onboard-auth.credentials.ts`

/// Default model reference for ZAI.
///
/// Source: `src/commands/onboard-auth.credentials.ts` - `ZAI_DEFAULT_MODEL_REF`
pub const ZAI_DEFAULT_MODEL_REF: &str = "zai/glm-4.7";

/// Default model reference for Xiaomi.
///
/// Source: `src/commands/onboard-auth.credentials.ts` - `XIAOMI_DEFAULT_MODEL_REF`
pub const XIAOMI_DEFAULT_MODEL_REF: &str = "xiaomi/mimo-v2-flash";

/// Default model reference for OpenRouter.
///
/// Source: `src/commands/onboard-auth.credentials.ts` - `OPENROUTER_DEFAULT_MODEL_REF`
pub const OPENROUTER_DEFAULT_MODEL_REF: &str = "openrouter/auto";

/// Default model reference for Vercel AI Gateway.
///
/// Source: `src/commands/onboard-auth.credentials.ts` - `VERCEL_AI_GATEWAY_DEFAULT_MODEL_REF`
pub const VERCEL_AI_GATEWAY_DEFAULT_MODEL_REF: &str =
    "vercel-ai-gateway/anthropic/claude-opus-4.6";

/// Default model reference for Cloudflare AI Gateway.
///
/// Source: `src/commands/onboard-auth.credentials.ts` - re-exported from cloudflare module
pub const CLOUDFLARE_AI_GATEWAY_DEFAULT_MODEL_REF: &str =
    "cloudflare-ai-gateway/claude-opus-4-6";

/// Default model reference for xAI (Grok).
///
/// Source: `src/commands/onboard-auth.credentials.ts` - re-exported from models
pub const XAI_DEFAULT_MODEL_REF: &str = "xai/grok-4";

/// Credential type for auth profile storage.
///
/// Source: `src/commands/onboard-auth.credentials.ts` - credential types
#[derive(Debug, Clone)]
pub enum CredentialType {
    /// API key credential.
    ApiKey {
        /// Provider identifier.
        provider: String,
        /// The API key.
        key: String,
    },
    /// OAuth credential.
    OAuth {
        /// Provider identifier.
        provider: String,
        /// OAuth tokens and metadata.
        email: Option<String>,
    },
}

/// Parameters for upserting an auth profile.
///
/// Source: `src/commands/onboard-auth.credentials.ts` - `upsertAuthProfile` params
#[derive(Debug, Clone)]
pub struct UpsertAuthProfileParams {
    /// Profile ID (e.g., "anthropic:default").
    pub profile_id: String,
    /// The credential to store.
    pub credential: CredentialType,
    /// Agent directory to store credentials in.
    pub agent_dir: Option<String>,
}

/// Store an Anthropic API key credential.
///
/// Source: `src/commands/onboard-auth.credentials.ts` - `setAnthropicApiKey`
pub fn build_anthropic_api_key_profile(key: &str) -> UpsertAuthProfileParams {
    UpsertAuthProfileParams {
        profile_id: "anthropic:default".to_string(),
        credential: CredentialType::ApiKey {
            provider: "anthropic".to_string(),
            key: key.to_string(),
        },
        agent_dir: None,
    }
}

/// Store a Gemini API key credential.
///
/// Source: `src/commands/onboard-auth.credentials.ts` - `setGeminiApiKey`
pub fn build_gemini_api_key_profile(key: &str) -> UpsertAuthProfileParams {
    UpsertAuthProfileParams {
        profile_id: "google:default".to_string(),
        credential: CredentialType::ApiKey {
            provider: "google".to_string(),
            key: key.to_string(),
        },
        agent_dir: None,
    }
}

/// Store a Moonshot API key credential.
///
/// Source: `src/commands/onboard-auth.credentials.ts` - `setMoonshotApiKey`
pub fn build_moonshot_api_key_profile(key: &str) -> UpsertAuthProfileParams {
    UpsertAuthProfileParams {
        profile_id: "moonshot:default".to_string(),
        credential: CredentialType::ApiKey {
            provider: "moonshot".to_string(),
            key: key.to_string(),
        },
        agent_dir: None,
    }
}

/// Store a xAI API key credential.
///
/// Source: `src/commands/onboard-auth.credentials.ts` - `setXaiApiKey`
pub fn build_xai_api_key_profile(key: &str) -> UpsertAuthProfileParams {
    UpsertAuthProfileParams {
        profile_id: "xai:default".to_string(),
        credential: CredentialType::ApiKey {
            provider: "xai".to_string(),
            key: key.to_string(),
        },
        agent_dir: None,
    }
}

/// Store an OpenRouter API key credential.
///
/// Source: `src/commands/onboard-auth.credentials.ts` - `setOpenrouterApiKey`
pub fn build_openrouter_api_key_profile(key: &str) -> UpsertAuthProfileParams {
    UpsertAuthProfileParams {
        profile_id: "openrouter:default".to_string(),
        credential: CredentialType::ApiKey {
            provider: "openrouter".to_string(),
            key: key.to_string(),
        },
        agent_dir: None,
    }
}

/// Store a ZAI API key credential.
///
/// Source: `src/commands/onboard-auth.credentials.ts` - `setZaiApiKey`
pub fn build_zai_api_key_profile(key: &str) -> UpsertAuthProfileParams {
    UpsertAuthProfileParams {
        profile_id: "zai:default".to_string(),
        credential: CredentialType::ApiKey {
            provider: "zai".to_string(),
            key: key.to_string(),
        },
        agent_dir: None,
    }
}

/// Store a Venice API key credential.
///
/// Source: `src/commands/onboard-auth.credentials.ts` - `setVeniceApiKey`
pub fn build_venice_api_key_profile(key: &str) -> UpsertAuthProfileParams {
    UpsertAuthProfileParams {
        profile_id: "venice:default".to_string(),
        credential: CredentialType::ApiKey {
            provider: "venice".to_string(),
            key: key.to_string(),
        },
        agent_dir: None,
    }
}

/// Store a Qianfan API key credential.
///
/// Source: `src/commands/onboard-auth.credentials.ts` - `setQianfanApiKey`
pub fn build_qianfan_api_key_profile(key: &str) -> UpsertAuthProfileParams {
    UpsertAuthProfileParams {
        profile_id: "qianfan:default".to_string(),
        credential: CredentialType::ApiKey {
            provider: "qianfan".to_string(),
            key: key.to_string(),
        },
        agent_dir: None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn build_anthropic_profile() {
        let profile = build_anthropic_api_key_profile("sk-test");
        assert_eq!(profile.profile_id, "anthropic:default");
        match profile.credential {
            CredentialType::ApiKey { provider, key } => {
                assert_eq!(provider, "anthropic");
                assert_eq!(key, "sk-test");
            }
            _ => panic!("expected ApiKey credential"),
        }
    }

    #[test]
    fn build_xai_profile() {
        let profile = build_xai_api_key_profile("xai-key");
        assert_eq!(profile.profile_id, "xai:default");
    }

    #[test]
    fn model_refs_format() {
        assert_eq!(ZAI_DEFAULT_MODEL_REF, "zai/glm-4.7");
        assert_eq!(XIAOMI_DEFAULT_MODEL_REF, "xiaomi/mimo-v2-flash");
        assert_eq!(OPENROUTER_DEFAULT_MODEL_REF, "openrouter/auto");
    }
}
