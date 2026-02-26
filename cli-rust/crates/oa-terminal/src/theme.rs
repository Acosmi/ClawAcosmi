/// Terminal theme configuration.
///
/// Provides color functions for themed output, matching the TS chalk-based theme.
///
/// Source: `src/terminal/theme.ts`

use colored::Colorize;

use crate::palette::LOBSTER_PALETTE;

/// Whether the terminal supports rich (colored) output.
///
/// Respects `NO_COLOR` and `FORCE_COLOR` environment variables.
pub fn is_rich() -> bool {
    if let Ok(force) = std::env::var("FORCE_COLOR") {
        let trimmed = force.trim();
        if !trimmed.is_empty() && trimmed != "0" {
            return true;
        }
    }
    if std::env::var("NO_COLOR").is_ok() {
        return false;
    }
    // Default: check if stdout is a TTY
    std::io::IsTerminal::is_terminal(&std::io::stdout())
}

/// Apply a color function only when rich mode is enabled.
pub fn colorize(rich: bool, text: &str, color_fn: fn(&str) -> String) -> String {
    if rich {
        color_fn(text)
    } else {
        text.to_string()
    }
}

/// Theme color functions for styled terminal output.
///
/// Each function takes a string and returns it colored with the appropriate theme color.
/// When rich mode is disabled, these return the input unchanged.
pub struct Theme;

impl Theme {
    /// Primary accent color (orange).
    pub fn accent(text: &str) -> String {
        if is_rich() {
            text.truecolor(0xFF, 0x5A, 0x2D).to_string()
        } else {
            text.to_string()
        }
    }

    /// Bright accent variant.
    pub fn accent_bright(text: &str) -> String {
        if is_rich() {
            text.truecolor(0xFF, 0x7A, 0x3D).to_string()
        } else {
            text.to_string()
        }
    }

    /// Dimmed accent variant.
    pub fn accent_dim(text: &str) -> String {
        if is_rich() {
            text.truecolor(0xD1, 0x4A, 0x22).to_string()
        } else {
            text.to_string()
        }
    }

    /// Info color (lighter orange).
    pub fn info(text: &str) -> String {
        if is_rich() {
            text.truecolor(0xFF, 0x8A, 0x5B).to_string()
        } else {
            text.to_string()
        }
    }

    /// Success color (green).
    pub fn success(text: &str) -> String {
        if is_rich() {
            text.truecolor(0x2F, 0xBF, 0x71).to_string()
        } else {
            text.to_string()
        }
    }

    /// Warning color (yellow/gold).
    pub fn warn(text: &str) -> String {
        if is_rich() {
            text.truecolor(0xFF, 0xB0, 0x20).to_string()
        } else {
            text.to_string()
        }
    }

    /// Error color (red).
    pub fn error(text: &str) -> String {
        if is_rich() {
            text.truecolor(0xE2, 0x3D, 0x2D).to_string()
        } else {
            text.to_string()
        }
    }

    /// Muted/dim color (gray).
    pub fn muted(text: &str) -> String {
        if is_rich() {
            text.truecolor(0x8B, 0x7F, 0x77).to_string()
        } else {
            text.to_string()
        }
    }

    /// Bold accent heading.
    pub fn heading(text: &str) -> String {
        if is_rich() {
            text.truecolor(0xFF, 0x5A, 0x2D).bold().to_string()
        } else {
            text.to_string()
        }
    }

    /// Command color (bright accent).
    pub fn command(text: &str) -> String {
        Self::accent_bright(text)
    }

    /// Option color (warning/gold).
    pub fn option(text: &str) -> String {
        Self::warn(text)
    }
}

/// Parse a hex color string like "#FF5A2D" into (r, g, b).
pub fn parse_hex_color(hex: &str) -> Option<(u8, u8, u8)> {
    let hex = hex.strip_prefix('#').unwrap_or(hex);
    if hex.len() != 6 {
        return None;
    }
    let r = u8::from_str_radix(&hex[0..2], 16).ok()?;
    let g = u8::from_str_radix(&hex[2..4], 16).ok()?;
    let b = u8::from_str_radix(&hex[4..6], 16).ok()?;
    Some((r, g, b))
}

/// Get the lobster palette reference.
pub fn lobster_palette() -> &'static crate::palette::LobsterPalette {
    &LOBSTER_PALETTE
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_hex_color_valid() {
        assert_eq!(parse_hex_color("#FF5A2D"), Some((0xFF, 0x5A, 0x2D)));
        assert_eq!(parse_hex_color("2FBF71"), Some((0x2F, 0xBF, 0x71)));
    }

    #[test]
    fn test_parse_hex_color_invalid() {
        assert_eq!(parse_hex_color(""), None);
        assert_eq!(parse_hex_color("#FFF"), None);
        assert_eq!(parse_hex_color("GGGGGG"), None);
    }
}
