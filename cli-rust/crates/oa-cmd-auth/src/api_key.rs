/// API key normalization, validation, and preview formatting.
///
/// Handles shell-style assignments (`export KEY="value"`), unquoting,
/// trailing semicolons, and produces safe preview strings for user display.
///
/// Source: `src/commands/auth-choice.api-key.ts`

use regex::Regex;
use std::sync::LazyLock;

/// Default number of characters shown at the head of a key preview.
///
/// Source: `src/commands/auth-choice.api-key.ts` - `DEFAULT_KEY_PREVIEW`
const DEFAULT_KEY_PREVIEW_HEAD: usize = 4;

/// Default number of characters shown at the tail of a key preview.
///
/// Source: `src/commands/auth-choice.api-key.ts` - `DEFAULT_KEY_PREVIEW`
const DEFAULT_KEY_PREVIEW_TAIL: usize = 4;

/// Regex matching shell-style assignments like `export KEY="value"` or `KEY=value`.
///
/// Source: `src/commands/auth-choice.api-key.ts` - `normalizeApiKeyInput`
static ASSIGNMENT_RE: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"^(?:export\s+)?[A-Za-z_][A-Za-z0-9_]*\s*=\s*(.+)$")
        .expect("ASSIGNMENT_RE is a valid regex")
});

/// Normalize raw API key input.
///
/// Strips shell-style `export FOO="bar"` wrappers, removes surrounding
/// quotes (single, double, or backtick), and trims trailing semicolons.
///
/// # Examples
///
/// ```
/// use oa_cmd_auth::api_key::normalize_api_key_input;
///
/// assert_eq!(normalize_api_key_input("sk-abc123"), "sk-abc123");
/// assert_eq!(normalize_api_key_input("export KEY=\"sk-abc123\""), "sk-abc123");
/// assert_eq!(normalize_api_key_input("KEY='sk-abc123';"), "sk-abc123");
/// ```
///
/// Source: `src/commands/auth-choice.api-key.ts` - `normalizeApiKeyInput`
pub fn normalize_api_key_input(raw: &str) -> String {
    let trimmed = raw.trim();
    if trimmed.is_empty() {
        return String::new();
    }

    // Handle shell-style assignments: export KEY="value" or KEY=value
    let value_part = ASSIGNMENT_RE
        .captures(trimmed)
        .and_then(|caps| caps.get(1))
        .map_or(trimmed, |m| m.as_str().trim());

    // Strip trailing semicolon first (handles `KEY='val';` pattern)
    let without_semi = value_part.strip_suffix(';').unwrap_or(value_part);

    // Unquote surrounding quotes (double, single, or backtick)
    let unquoted = if without_semi.len() >= 2 {
        let bytes = without_semi.as_bytes();
        let first = bytes[0];
        let last = bytes[bytes.len() - 1];
        if (first == b'"' && last == b'"')
            || (first == b'\'' && last == b'\'')
            || (first == b'`' && last == b'`')
        {
            &without_semi[1..without_semi.len() - 1]
        } else {
            without_semi
        }
    } else {
        without_semi
    };

    // Strip any remaining trailing semicolon (in case it was inside quotes)
    let final_str = unquoted.strip_suffix(';').unwrap_or(unquoted);

    final_str.trim().to_owned()
}

/// Validate an API key input string.
///
/// Returns `None` if the input is valid (non-empty after normalization),
/// or `Some("Required")` if it is empty.
///
/// Source: `src/commands/auth-choice.api-key.ts` - `validateApiKeyInput`
#[must_use]
pub fn validate_api_key_input(value: &str) -> Option<&'static str> {
    if normalize_api_key_input(value).is_empty() {
        Some("Required")
    } else {
        None
    }
}

/// Options for controlling how much of a key is shown in a preview.
///
/// Source: `src/commands/auth-choice.api-key.ts` - `formatApiKeyPreview` opts
#[derive(Debug, Clone, Copy)]
pub struct KeyPreviewOpts {
    /// Characters to show at the beginning.
    pub head: usize,
    /// Characters to show at the end.
    pub tail: usize,
}

impl Default for KeyPreviewOpts {
    fn default() -> Self {
        Self {
            head: DEFAULT_KEY_PREVIEW_HEAD,
            tail: DEFAULT_KEY_PREVIEW_TAIL,
        }
    }
}

/// Format an API key as a safe preview string for user display.
///
/// Shows the first `head` and last `tail` characters separated by an
/// ellipsis. Short keys use a minimal preview to avoid revealing the
/// full value.
///
/// # Examples
///
/// ```
/// use oa_cmd_auth::api_key::format_api_key_preview;
///
/// assert_eq!(format_api_key_preview("sk-abcdef1234567890", None), "sk-a\u{2026}7890");
/// assert_eq!(format_api_key_preview("", None), "\u{2026}");
/// ```
///
/// Source: `src/commands/auth-choice.api-key.ts` - `formatApiKeyPreview`
#[must_use]
pub fn format_api_key_preview(raw: &str, opts: Option<KeyPreviewOpts>) -> String {
    let trimmed = raw.trim();
    if trimmed.is_empty() {
        return "\u{2026}".to_owned();
    }

    let opts = opts.unwrap_or_default();
    let head = opts.head;
    let tail = opts.tail;

    if trimmed.len() <= head + tail {
        // Short key: show minimal preview
        let short_head = std::cmp::min(2, trimmed.len());
        let short_tail = std::cmp::min(2, trimmed.len().saturating_sub(short_head));
        if short_tail == 0 {
            return format!("{}\u{2026}", &trimmed[..short_head]);
        }
        return format!(
            "{}\u{2026}{}",
            &trimmed[..short_head],
            &trimmed[trimmed.len() - short_tail..]
        );
    }

    format!(
        "{}\u{2026}{}",
        &trimmed[..head],
        &trimmed[trimmed.len() - tail..]
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    // ── normalize_api_key_input ──

    #[test]
    fn normalize_empty_string() {
        assert_eq!(normalize_api_key_input(""), "");
        assert_eq!(normalize_api_key_input("   "), "");
    }

    #[test]
    fn normalize_plain_key() {
        assert_eq!(normalize_api_key_input("sk-abc123"), "sk-abc123");
    }

    #[test]
    fn normalize_with_whitespace() {
        assert_eq!(normalize_api_key_input("  sk-abc123  "), "sk-abc123");
    }

    #[test]
    fn normalize_export_double_quoted() {
        assert_eq!(
            normalize_api_key_input(r#"export API_KEY="sk-abc123""#),
            "sk-abc123"
        );
    }

    #[test]
    fn normalize_export_single_quoted() {
        assert_eq!(
            normalize_api_key_input("export API_KEY='sk-abc123'"),
            "sk-abc123"
        );
    }

    #[test]
    fn normalize_assignment_no_export() {
        assert_eq!(
            normalize_api_key_input("MY_KEY=sk-abc123"),
            "sk-abc123"
        );
    }

    #[test]
    fn normalize_trailing_semicolon() {
        assert_eq!(
            normalize_api_key_input("export KEY='sk-abc123';"),
            "sk-abc123"
        );
    }

    #[test]
    fn normalize_backtick_quoted() {
        assert_eq!(
            normalize_api_key_input("KEY=`sk-abc123`"),
            "sk-abc123"
        );
    }

    // ── validate_api_key_input ──

    #[test]
    fn validate_empty_is_required() {
        assert_eq!(validate_api_key_input(""), Some("Required"));
        assert_eq!(validate_api_key_input("   "), Some("Required"));
    }

    #[test]
    fn validate_nonempty_is_ok() {
        assert_eq!(validate_api_key_input("sk-abc"), None);
    }

    // ── format_api_key_preview ──

    #[test]
    fn preview_empty_string() {
        assert_eq!(format_api_key_preview("", None), "\u{2026}");
    }

    #[test]
    fn preview_long_key() {
        assert_eq!(
            format_api_key_preview("sk-abcdef1234567890", None),
            "sk-a\u{2026}7890"
        );
    }

    #[test]
    fn preview_short_key() {
        // "abc" has 3 chars, head+tail=8, so it uses the short branch
        let result = format_api_key_preview("abc", None);
        assert_eq!(result, "ab\u{2026}c");
    }

    #[test]
    fn preview_very_short_key() {
        let result = format_api_key_preview("a", None);
        assert_eq!(result, "a\u{2026}");
    }

    #[test]
    fn preview_custom_opts() {
        let opts = KeyPreviewOpts { head: 2, tail: 2 };
        assert_eq!(
            format_api_key_preview("sk-abcdef1234567890", Some(opts)),
            "sk\u{2026}90"
        );
    }
}
