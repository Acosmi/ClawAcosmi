/// ANSI escape code utilities.
///
/// Strip ANSI codes and compute visible width of terminal strings.
///
/// Source: `src/terminal/ansi.ts`

use regex::Regex;
use std::sync::LazyLock;

/// Regex matching ANSI SGR sequences (e.g. `\x1b[31m`).
static ANSI_SGR_RE: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"\x1b\[[0-9;]*m").expect("valid regex"));

/// Regex matching OSC-8 hyperlink sequences.
static OSC8_RE: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"\x1b\]8;;.*?\x1b\\").expect("valid regex"));

/// Combined regex for stripping all ANSI escape sequences.
static ANSI_ALL_RE: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"(\x1b\[[0-9;]*m|\x1b\]8;;.*?\x1b\\)").expect("valid regex")
});

/// Strip all ANSI escape sequences from a string.
pub fn strip_ansi(input: &str) -> String {
    ANSI_ALL_RE.replace_all(input, "").into_owned()
}

/// Compute the visible width of a string (excluding ANSI escape sequences).
pub fn visible_width(input: &str) -> usize {
    strip_ansi(input).chars().count()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_strip_ansi_plain() {
        assert_eq!(strip_ansi("hello"), "hello");
    }

    #[test]
    fn test_strip_ansi_colored() {
        assert_eq!(strip_ansi("\x1b[31mred\x1b[0m"), "red");
    }

    #[test]
    fn test_visible_width_plain() {
        assert_eq!(visible_width("hello"), 5);
    }

    #[test]
    fn test_visible_width_colored() {
        assert_eq!(visible_width("\x1b[31mred\x1b[0m text"), 8);
    }
}
