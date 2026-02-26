/// Model default configuration helpers.
///
/// Provides functions to apply provider-specific model defaults to the
/// configuration. Each provider has a default model that is set when
/// the user authenticates with that provider.
///
/// Source: `src/commands/google-gemini-model-default.ts`,
///         `src/commands/openai-codex-model-default.ts`,
///         `src/commands/openai-model-default.ts`,
///         `src/commands/opencode-zen-model-default.ts`

use oa_types::agent_defaults::AgentModelListConfig;
use oa_types::agents::AgentsConfig;
use oa_types::config::OpenAcosmiConfig;

/// Default model for Google Gemini provider.
///
/// Source: `src/commands/google-gemini-model-default.ts` - `GOOGLE_GEMINI_DEFAULT_MODEL`
pub const GOOGLE_GEMINI_DEFAULT_MODEL: &str = "google/gemini-3-pro-preview";

/// Default model for OpenAI Codex provider.
///
/// Source: `src/commands/openai-codex-model-default.ts` - `OPENAI_CODEX_DEFAULT_MODEL`
pub const OPENAI_CODEX_DEFAULT_MODEL: &str = "openai-codex/gpt-5.3-codex";

/// Default model for OpenAI provider.
///
/// Source: `src/commands/openai-model-default.ts` - `OPENAI_DEFAULT_MODEL`
pub const OPENAI_DEFAULT_MODEL: &str = "openai/gpt-5.1-codex";

/// Default model for OpenCode Zen provider.
///
/// Source: `src/commands/opencode-zen-model-default.ts` - `OPENCODE_ZEN_DEFAULT_MODEL`
pub const OPENCODE_ZEN_DEFAULT_MODEL: &str = "opencode/claude-opus-4-6";

/// Legacy OpenCode Zen model identifiers that should be upgraded.
///
/// Source: `src/commands/opencode-zen-model-default.ts` - `LEGACY_OPENCODE_ZEN_DEFAULT_MODELS`
const LEGACY_OPENCODE_ZEN_MODELS: &[&str] = &[
    "opencode/claude-opus-4-5",
    "opencode-zen/claude-opus-4-5",
];

/// Result of applying a model default.
///
/// Source: Various model default files - return type of `apply*ModelDefault`
#[derive(Debug, Clone)]
pub struct ModelDefaultResult {
    /// The updated configuration.
    pub next: OpenAcosmiConfig,
    /// Whether the configuration was changed.
    pub changed: bool,
}

/// Extract the primary model string from the config's agent defaults.
///
/// Source: Various model default files - `resolvePrimaryModel`
fn resolve_primary_model(cfg: &OpenAcosmiConfig) -> Option<String> {
    let defaults = cfg.agents.as_ref()?.defaults.as_ref()?;
    let model_cfg = defaults.model.as_ref()?;
    model_cfg.primary.clone()
}

/// Set the primary model in the configuration, preserving other fields.
///
/// Source: Various model default files - model setter pattern
fn set_primary_model(cfg: &OpenAcosmiConfig, model: &str) -> OpenAcosmiConfig {
    let agents = cfg.agents.clone().unwrap_or_default();
    let defaults = agents.defaults.clone().unwrap_or_default();
    let model_list = defaults.model.clone().unwrap_or_default();

    let next_model = AgentModelListConfig {
        primary: Some(model.to_owned()),
        ..model_list
    };

    OpenAcosmiConfig {
        agents: Some(AgentsConfig {
            defaults: Some(oa_types::agent_defaults::AgentDefaultsConfig {
                model: Some(next_model),
                ..defaults
            }),
            ..agents
        }),
        ..cfg.clone()
    }
}

/// Apply the Google Gemini default model to the configuration.
///
/// Sets the primary model to `google/gemini-3-pro-preview` if it is
/// not already set to that value.
///
/// Source: `src/commands/google-gemini-model-default.ts` - `applyGoogleGeminiModelDefault`
pub fn apply_google_gemini_model_default(cfg: &OpenAcosmiConfig) -> ModelDefaultResult {
    let current = resolve_primary_model(cfg);
    if current.as_deref() == Some(GOOGLE_GEMINI_DEFAULT_MODEL) {
        return ModelDefaultResult {
            next: cfg.clone(),
            changed: false,
        };
    }

    ModelDefaultResult {
        next: set_primary_model(cfg, GOOGLE_GEMINI_DEFAULT_MODEL),
        changed: true,
    }
}

/// Check whether the OpenAI Codex default model should be applied.
///
/// Source: `src/commands/openai-codex-model-default.ts` - `shouldSetOpenAICodexModel`
fn should_set_openai_codex_model(model: Option<&str>) -> bool {
    let trimmed = match model {
        Some(m) => m.trim(),
        None => return true,
    };
    if trimmed.is_empty() {
        return true;
    }
    let normalized = trimmed.to_lowercase();
    if normalized.starts_with("openai-codex/") {
        return false;
    }
    if normalized.starts_with("openai/") {
        return true;
    }
    normalized == "gpt" || normalized == "gpt-mini"
}

/// Apply the OpenAI Codex default model to the configuration.
///
/// Sets the primary model to `openai-codex/gpt-5.3-codex` when appropriate.
///
/// Source: `src/commands/openai-codex-model-default.ts` - `applyOpenAICodexModelDefault`
pub fn apply_openai_codex_model_default(cfg: &OpenAcosmiConfig) -> ModelDefaultResult {
    let current = resolve_primary_model(cfg);
    if !should_set_openai_codex_model(current.as_deref()) {
        return ModelDefaultResult {
            next: cfg.clone(),
            changed: false,
        };
    }

    ModelDefaultResult {
        next: set_primary_model(cfg, OPENAI_CODEX_DEFAULT_MODEL),
        changed: true,
    }
}

/// Apply the OpenCode Zen default model to the configuration.
///
/// Sets the primary model to `opencode/claude-opus-4-6`. Also upgrades
/// legacy model identifiers.
///
/// Source: `src/commands/opencode-zen-model-default.ts` - `applyOpencodeZenModelDefault`
pub fn apply_opencode_zen_model_default(cfg: &OpenAcosmiConfig) -> ModelDefaultResult {
    let current = resolve_primary_model(cfg);
    let current_trimmed = current.as_deref().map(str::trim);

    // Check for legacy models that should be upgraded.
    let normalized = current_trimmed
        .filter(|c| LEGACY_OPENCODE_ZEN_MODELS.contains(c))
        .map_or_else(
            || current_trimmed.map(String::from),
            |_| Some(OPENCODE_ZEN_DEFAULT_MODEL.to_owned()),
        );

    if normalized.as_deref() == Some(OPENCODE_ZEN_DEFAULT_MODEL) {
        return ModelDefaultResult {
            next: cfg.clone(),
            changed: false,
        };
    }

    ModelDefaultResult {
        next: set_primary_model(cfg, OPENCODE_ZEN_DEFAULT_MODEL),
        changed: true,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_config_with_model(model: &str) -> OpenAcosmiConfig {
        set_primary_model(&OpenAcosmiConfig::default(), model)
    }

    // ── Google Gemini ──

    #[test]
    fn gemini_default_applied_when_empty() {
        let cfg = OpenAcosmiConfig::default();
        let result = apply_google_gemini_model_default(&cfg);
        assert!(result.changed);
        assert_eq!(
            resolve_primary_model(&result.next).as_deref(),
            Some(GOOGLE_GEMINI_DEFAULT_MODEL)
        );
    }

    #[test]
    fn gemini_default_not_applied_when_already_set() {
        let cfg = make_config_with_model(GOOGLE_GEMINI_DEFAULT_MODEL);
        let result = apply_google_gemini_model_default(&cfg);
        assert!(!result.changed);
    }

    // ── OpenAI Codex ──

    #[test]
    fn codex_should_set_empty() {
        assert!(should_set_openai_codex_model(None));
        assert!(should_set_openai_codex_model(Some("")));
    }

    #[test]
    fn codex_should_set_openai_prefix() {
        assert!(should_set_openai_codex_model(Some("openai/gpt-4o")));
    }

    #[test]
    fn codex_should_not_set_codex_prefix() {
        assert!(!should_set_openai_codex_model(Some(
            "openai-codex/gpt-5.3-codex"
        )));
    }

    #[test]
    fn codex_should_set_gpt_alias() {
        assert!(should_set_openai_codex_model(Some("gpt")));
        assert!(should_set_openai_codex_model(Some("gpt-mini")));
    }

    #[test]
    fn codex_default_applied() {
        let cfg = OpenAcosmiConfig::default();
        let result = apply_openai_codex_model_default(&cfg);
        assert!(result.changed);
        assert_eq!(
            resolve_primary_model(&result.next).as_deref(),
            Some(OPENAI_CODEX_DEFAULT_MODEL)
        );
    }

    // ── OpenCode Zen ──

    #[test]
    fn zen_default_applied_when_empty() {
        let cfg = OpenAcosmiConfig::default();
        let result = apply_opencode_zen_model_default(&cfg);
        assert!(result.changed);
        assert_eq!(
            resolve_primary_model(&result.next).as_deref(),
            Some(OPENCODE_ZEN_DEFAULT_MODEL)
        );
    }

    #[test]
    fn zen_upgrades_legacy_model() {
        let cfg = make_config_with_model("opencode/claude-opus-4-5");
        let result = apply_opencode_zen_model_default(&cfg);
        assert!(!result.changed); // Legacy is normalized to current, so no change
    }

    #[test]
    fn zen_not_applied_when_already_current() {
        let cfg = make_config_with_model(OPENCODE_ZEN_DEFAULT_MODEL);
        let result = apply_opencode_zen_model_default(&cfg);
        assert!(!result.changed);
    }

    // ── Constants ──

    #[test]
    fn model_constants() {
        assert_eq!(GOOGLE_GEMINI_DEFAULT_MODEL, "google/gemini-3-pro-preview");
        assert_eq!(OPENAI_CODEX_DEFAULT_MODEL, "openai-codex/gpt-5.3-codex");
        assert_eq!(OPENAI_DEFAULT_MODEL, "openai/gpt-5.1-codex");
        assert_eq!(OPENCODE_ZEN_DEFAULT_MODEL, "opencode/claude-opus-4-6");
    }
}
