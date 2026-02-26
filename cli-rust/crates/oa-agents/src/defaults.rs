/// Agent default constants for provider, model, and context tokens.
///
/// These are used as fallback values when the user configuration does not
/// supply explicit overrides.
///
/// Source: `src/agents/defaults.ts`

/// Default AI provider identifier.
///
/// Source: `src/agents/defaults.ts` - `DEFAULT_PROVIDER`
pub const DEFAULT_PROVIDER: &str = "anthropic";

/// Default model identifier (Anthropic Claude Opus 4.6).
///
/// Source: `src/agents/defaults.ts` - `DEFAULT_MODEL`
pub const DEFAULT_MODEL: &str = "claude-opus-4-6";

/// Conservative fallback context window size (in tokens) used when model
/// metadata is unavailable.
///
/// Source: `src/agents/defaults.ts` - `DEFAULT_CONTEXT_TOKENS`
pub const DEFAULT_CONTEXT_TOKENS: usize = 200_000;

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn default_provider_is_anthropic() {
        assert_eq!(DEFAULT_PROVIDER, "anthropic");
    }

    #[test]
    fn default_model_is_claude_opus() {
        assert_eq!(DEFAULT_MODEL, "claude-opus-4-6");
    }

    #[test]
    fn default_context_tokens_value() {
        assert_eq!(DEFAULT_CONTEXT_TOKENS, 200_000);
    }
}
