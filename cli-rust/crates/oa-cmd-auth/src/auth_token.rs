/// Auth token validation and profile ID construction.
///
/// Handles Anthropic setup-token validation (prefix + minimum length)
/// and constructs canonical `provider:name` profile identifiers from
/// user-supplied names.
///
/// Source: `src/commands/auth-token.ts`

use regex::Regex;
use std::sync::LazyLock;

/// Prefix expected on Anthropic setup tokens.
///
/// Source: `src/commands/auth-token.ts` - `ANTHROPIC_SETUP_TOKEN_PREFIX`
pub const ANTHROPIC_SETUP_TOKEN_PREFIX: &str = "sk-ant-oat01-";

/// Minimum length for a valid Anthropic setup token.
///
/// Source: `src/commands/auth-token.ts` - `ANTHROPIC_SETUP_TOKEN_MIN_LENGTH`
pub const ANTHROPIC_SETUP_TOKEN_MIN_LENGTH: usize = 80;

/// Default profile name used when the user leaves the name blank.
///
/// Source: `src/commands/auth-token.ts` - `DEFAULT_TOKEN_PROFILE_NAME`
pub const DEFAULT_TOKEN_PROFILE_NAME: &str = "default";

/// Regex for replacing non-alphanumeric sequences (excluding `.`, `-`, `_`) with hyphens.
///
/// Source: `src/commands/auth-token.ts` - `normalizeTokenProfileName`
static SLUG_REPLACE_RE: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"[^a-z0-9._-]+").expect("SLUG_REPLACE_RE is a valid regex"));

/// Regex for collapsing multiple consecutive hyphens.
///
/// Source: `src/commands/auth-token.ts` - `normalizeTokenProfileName`
static MULTI_HYPHEN_RE: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"-+").expect("MULTI_HYPHEN_RE is a valid regex"));

/// Normalize a raw profile name to a URL-safe slug.
///
/// Lowercases the input, replaces non-alphanumeric characters with hyphens,
/// collapses consecutive hyphens, and trims leading/trailing hyphens.
/// Returns [`DEFAULT_TOKEN_PROFILE_NAME`] when the input is empty.
///
/// Source: `src/commands/auth-token.ts` - `normalizeTokenProfileName`
#[must_use]
pub fn normalize_token_profile_name(raw: &str) -> String {
    let trimmed = raw.trim();
    if trimmed.is_empty() {
        return DEFAULT_TOKEN_PROFILE_NAME.to_owned();
    }

    let lower = trimmed.to_lowercase();
    let replaced = SLUG_REPLACE_RE.replace_all(&lower, "-");
    let collapsed = MULTI_HYPHEN_RE.replace_all(&replaced, "-");
    let slug = collapsed.trim_matches('-').to_owned();

    if slug.is_empty() {
        DEFAULT_TOKEN_PROFILE_NAME.to_owned()
    } else {
        slug
    }
}

/// Build a canonical token profile ID from a provider and a name.
///
/// The profile ID has the form `provider:name`, where provider is
/// normalized via [`oa_agents::model_selection::normalize_provider_id`]
/// and name is normalized via [`normalize_token_profile_name`].
///
/// Source: `src/commands/auth-token.ts` - `buildTokenProfileId`
#[must_use]
pub fn build_token_profile_id(provider: &str, name: &str) -> String {
    let normalized_provider = oa_agents::model_selection::normalize_provider_id(provider);
    let normalized_name = normalize_token_profile_name(name);
    format!("{normalized_provider}:{normalized_name}")
}

/// Validate an Anthropic setup token string.
///
/// Returns `None` if the token is valid, or `Some(error_message)` if it is
/// empty, has the wrong prefix, or is too short.
///
/// Source: `src/commands/auth-token.ts` - `validateAnthropicSetupToken`
#[must_use]
pub fn validate_anthropic_setup_token(raw: &str) -> Option<String> {
    let trimmed = raw.trim();
    if trimmed.is_empty() {
        return Some("Required".to_owned());
    }
    if !trimmed.starts_with(ANTHROPIC_SETUP_TOKEN_PREFIX) {
        return Some(format!(
            "Expected token starting with {ANTHROPIC_SETUP_TOKEN_PREFIX}"
        ));
    }
    if trimmed.len() < ANTHROPIC_SETUP_TOKEN_MIN_LENGTH {
        return Some("Token looks too short; paste the full setup-token".to_owned());
    }
    None
}

#[cfg(test)]
mod tests {
    use super::*;

    // ── normalize_token_profile_name ──

    #[test]
    fn normalize_empty_returns_default() {
        assert_eq!(normalize_token_profile_name(""), DEFAULT_TOKEN_PROFILE_NAME);
        assert_eq!(
            normalize_token_profile_name("   "),
            DEFAULT_TOKEN_PROFILE_NAME
        );
    }

    #[test]
    fn normalize_simple_name() {
        assert_eq!(normalize_token_profile_name("my-profile"), "my-profile");
    }

    #[test]
    fn normalize_with_spaces_and_special_chars() {
        assert_eq!(
            normalize_token_profile_name("My Profile!@# Name"),
            "my-profile-name"
        );
    }

    #[test]
    fn normalize_preserves_dots_and_underscores() {
        assert_eq!(
            normalize_token_profile_name("profile.v2_test"),
            "profile.v2_test"
        );
    }

    #[test]
    fn normalize_collapses_hyphens() {
        assert_eq!(
            normalize_token_profile_name("foo---bar"),
            "foo-bar"
        );
    }

    #[test]
    fn normalize_trims_leading_trailing_hyphens() {
        assert_eq!(normalize_token_profile_name("-foo-"), "foo");
    }

    #[test]
    fn normalize_all_special_returns_default() {
        assert_eq!(
            normalize_token_profile_name("!@#$%"),
            DEFAULT_TOKEN_PROFILE_NAME
        );
    }

    // ── build_token_profile_id ──

    #[test]
    fn build_profile_id_basic() {
        assert_eq!(
            build_token_profile_id("anthropic", "my-token"),
            "anthropic:my-token"
        );
    }

    #[test]
    fn build_profile_id_normalizes_provider() {
        assert_eq!(
            build_token_profile_id("z.ai", "default"),
            "zai:default"
        );
    }

    #[test]
    fn build_profile_id_empty_name() {
        assert_eq!(
            build_token_profile_id("anthropic", ""),
            "anthropic:default"
        );
    }

    // ── validate_anthropic_setup_token ──

    #[test]
    fn validate_empty_token() {
        assert_eq!(
            validate_anthropic_setup_token(""),
            Some("Required".to_owned())
        );
    }

    #[test]
    fn validate_wrong_prefix() {
        let result = validate_anthropic_setup_token("sk-wrong-prefix-abcdef");
        assert!(result.is_some());
        assert!(
            result
                .as_deref()
                .unwrap_or("")
                .contains(ANTHROPIC_SETUP_TOKEN_PREFIX)
        );
    }

    #[test]
    fn validate_too_short() {
        // Right prefix but too short
        let short_token = format!("{ANTHROPIC_SETUP_TOKEN_PREFIX}abc");
        let result = validate_anthropic_setup_token(&short_token);
        assert!(result.is_some());
        assert!(result.as_deref().unwrap_or("").contains("too short"));
    }

    #[test]
    fn validate_valid_token() {
        let valid = format!("{}{}", ANTHROPIC_SETUP_TOKEN_PREFIX, "a".repeat(80));
        assert!(validate_anthropic_setup_token(&valid).is_none());
    }
}
