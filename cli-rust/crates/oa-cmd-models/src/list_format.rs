/// Formatting utilities for model list output.
///
/// Source: `src/commands/models/list.format.ts`

/// Determine whether terminal output should use rich formatting (color/style).
///
/// Returns `true` when neither `--json` nor `--plain` are set and stdout
/// is a TTY.
///
/// Source: `src/commands/models/list.format.ts` - `isRich`
#[must_use]
pub fn is_rich(json: bool, plain: bool) -> bool {
    !json && !plain && atty::is(atty::Stream::Stdout)
}

/// Pad a string to the specified width.
///
/// Source: `src/commands/models/list.format.ts` - `pad`
#[must_use]
pub fn pad(value: &str, size: usize) -> String {
    format!("{value:<size$}")
}

/// Truncate a string to the specified maximum length, appending `"..."` if needed.
///
/// Source: `src/commands/models/list.format.ts` - `truncate`
#[must_use]
pub fn truncate(value: &str, max: usize) -> String {
    if value.len() <= max {
        return value.to_owned();
    }
    if max <= 3 {
        return value[..max].to_owned();
    }
    format!("{}...", &value[..max - 3])
}

/// Mask an API key for display, showing the first and last 8 characters.
///
/// Source: `src/commands/models/list.format.ts` - `maskApiKey`
#[must_use]
pub fn mask_api_key(value: &str) -> String {
    let trimmed = value.trim();
    if trimmed.is_empty() {
        return "missing".to_owned();
    }
    if trimmed.len() <= 16 {
        return trimmed.to_owned();
    }
    format!("{}...{}", &trimmed[..8], &trimmed[trimmed.len() - 8..])
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn pad_shorter() {
        let result = pad("hello", 10);
        assert_eq!(result.len(), 10);
        assert_eq!(result, "hello     ");
    }

    #[test]
    fn pad_exact() {
        let result = pad("hello", 5);
        assert_eq!(result, "hello");
    }

    #[test]
    fn truncate_short() {
        assert_eq!(truncate("hi", 10), "hi");
    }

    #[test]
    fn truncate_exact() {
        assert_eq!(truncate("hello", 5), "hello");
    }

    #[test]
    fn truncate_long() {
        assert_eq!(truncate("hello world", 8), "hello...");
    }

    #[test]
    fn truncate_very_small_max() {
        assert_eq!(truncate("hello", 2), "he");
    }

    #[test]
    fn mask_api_key_short() {
        assert_eq!(mask_api_key("abc"), "abc");
    }

    #[test]
    fn mask_api_key_empty() {
        assert_eq!(mask_api_key(""), "missing");
    }

    #[test]
    fn mask_api_key_long() {
        let key = "sk-12345678abcdefgh12345678";
        let masked = mask_api_key(key);
        assert!(masked.starts_with("sk-12345"));
        assert!(masked.contains("..."));
        assert!(masked.ends_with("12345678"));
    }
}
