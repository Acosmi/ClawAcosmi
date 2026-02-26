/// Session display formatting utilities.
///
/// Provides cell formatting functions for the sessions table output,
/// including kind, key, age, model, token usage, and flags columns.
///
/// Source: `src/commands/sessions.ts` - formatting functions

use oa_terminal::theme::Theme;

use crate::types::{SessionKind, SessionRow};

/// Format a token count as a human-readable "Nk" string.
///
/// Source: `src/commands/sessions.ts` - `formatKTokens`
fn format_k_tokens(value: u64) -> String {
    if value >= 10_000 {
        format!("{}k", value / 1000)
    } else {
        format!("{:.1}k", value as f64 / 1000.0)
    }
}

/// Truncate a session key to fit within the given width.
///
/// Source: `src/commands/sessions.ts` - `truncateKey`
pub fn truncate_key(key: &str, max_width: usize) -> String {
    if key.len() <= max_width {
        return key.to_string();
    }
    let head = 4.max(max_width.saturating_sub(10));
    let tail = 6.min(key.len());
    format!("{}...{}", &key[..head], &key[key.len() - tail..])
}

/// Apply color by usage percentage.
///
/// Source: `src/commands/sessions.ts` - `colorByPct`
fn color_by_pct(label: &str, pct: Option<u64>, rich: bool) -> String {
    if !rich {
        return label.to_string();
    }
    match pct {
        None => label.to_string(),
        Some(p) if p >= 95 => Theme::error(label),
        Some(p) if p >= 80 => Theme::warn(label),
        Some(p) if p >= 60 => Theme::success(label),
        Some(_) => Theme::muted(label),
    }
}

/// Format the tokens cell showing usage relative to context window.
///
/// Source: `src/commands/sessions.ts` - `formatTokensCell`
pub fn format_tokens_cell(total: u64, context_tokens: Option<u64>, rich: bool, pad: usize) -> String {
    if total == 0 {
        return pad_str("-", pad);
    }

    let total_label = format_k_tokens(total);
    let ctx_label = context_tokens.map_or_else(|| "?".to_string(), format_k_tokens);
    let pct = context_tokens.map(|ctx| {
        if ctx > 0 {
            999.min((total * 100) / ctx)
        } else {
            0
        }
    });
    let pct_str = pct.map_or_else(|| "?".to_string(), |p| p.to_string());
    let label = format!("{total_label}/{ctx_label} ({pct_str}%)");
    let padded = pad_str(&label, pad);
    color_by_pct(&padded, pct, rich)
}

/// Format the kind cell with color coding.
///
/// Source: `src/commands/sessions.ts` - `formatKindCell`
pub fn format_kind_cell(kind: &SessionKind, rich: bool, pad: usize) -> String {
    let label = pad_str(kind.as_str(), pad);
    if !rich {
        return label;
    }
    match kind {
        SessionKind::Group => Theme::accent_bright(&label),
        SessionKind::Global => Theme::warn(&label),
        SessionKind::Direct => Theme::accent(&label),
        SessionKind::Unknown => Theme::muted(&label),
    }
}

/// Format the age cell showing time since last update.
///
/// Source: `src/commands/sessions.ts` - `formatAgeCell`
pub fn format_age_cell(updated_at: Option<u64>, rich: bool, pad: usize) -> String {
    let age_label = updated_at
        .filter(|&ts| ts > 0)
        .map_or_else(
            || "unknown".to_string(),
            |ts| {
                let now = now_ms();
                format_time_ago(now.saturating_sub(ts))
            },
        );
    let padded = pad_str(&age_label, pad);
    if rich {
        Theme::muted(&padded)
    } else {
        padded
    }
}

/// Format the model cell.
///
/// Source: `src/commands/sessions.ts` - `formatModelCell`
pub fn format_model_cell(model: Option<&str>, rich: bool, pad: usize) -> String {
    let label = pad_str(model.unwrap_or("unknown"), pad);
    if rich {
        Theme::info(&label)
    } else {
        label
    }
}

/// Format the flags cell showing session metadata flags.
///
/// Source: `src/commands/sessions.ts` - `formatFlagsCell`
pub fn format_flags_cell(row: &SessionRow, rich: bool) -> String {
    let mut flags: Vec<String> = Vec::new();

    if let Some(ref level) = row.thinking_level {
        flags.push(format!("think:{level}"));
    }
    if let Some(ref level) = row.verbose_level {
        flags.push(format!("verbose:{level}"));
    }
    if let Some(ref level) = row.reasoning_level {
        flags.push(format!("reasoning:{level}"));
    }
    if let Some(ref level) = row.elevated_level {
        flags.push(format!("elev:{level}"));
    }
    if let Some(ref usage) = row.response_usage {
        flags.push(format!("usage:{usage}"));
    }
    if let Some(ref activation) = row.group_activation {
        flags.push(format!("activation:{activation}"));
    }
    if row.system_sent == Some(true) {
        flags.push("system".to_string());
    }
    if row.aborted_last_run == Some(true) {
        flags.push("aborted".to_string());
    }
    if let Some(ref id) = row.session_id {
        flags.push(format!("id:{id}"));
    }

    let label = flags.join(" ");
    if label.is_empty() {
        String::new()
    } else if rich {
        Theme::muted(&label)
    } else {
        label
    }
}

/// Format a duration (in ms) as a human-readable relative time.
///
/// Source: `src/infra/format-time/format-relative.ts` - `formatTimeAgo`
fn format_time_ago(duration_ms: u64) -> String {
    let total_seconds = (duration_ms + 500) / 1000; // round
    let minutes = (total_seconds + 30) / 60; // round

    if minutes < 1 {
        return "just now".to_string();
    }
    if minutes < 60 {
        return format!("{minutes}m ago");
    }
    let hours = (minutes + 30) / 60;
    if hours < 48 {
        return format!("{hours}h ago");
    }
    let days = (hours + 12) / 24;
    format!("{days}d ago")
}

/// Pad a string to the given width, right-padding with spaces.
///
/// Source: internal helper
fn pad_str(s: &str, width: usize) -> String {
    if s.len() >= width {
        s.to_string()
    } else {
        format!("{s}{}", " ".repeat(width - s.len()))
    }
}

/// Get current time in milliseconds since epoch.
///
/// Source: internal helper
fn now_ms() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map_or(0, |d| d.as_millis() as u64)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn format_k_tokens_small() {
        assert_eq!(format_k_tokens(500), "0.5k");
        assert_eq!(format_k_tokens(1000), "1.0k");
        assert_eq!(format_k_tokens(5500), "5.5k");
    }

    #[test]
    fn format_k_tokens_large() {
        assert_eq!(format_k_tokens(10_000), "10k");
        assert_eq!(format_k_tokens(200_000), "200k");
    }

    #[test]
    fn truncate_key_short() {
        assert_eq!(truncate_key("short", 26), "short");
    }

    #[test]
    fn truncate_key_long() {
        let long_key = "agent:bot:discord:group:very-long-channel-name-here";
        let truncated = truncate_key(long_key, 26);
        assert!(truncated.len() <= 30); // approximately
        assert!(truncated.contains("..."));
    }

    #[test]
    fn truncate_key_exact_width() {
        let key = "a".repeat(26);
        assert_eq!(truncate_key(&key, 26), key);
    }

    #[test]
    fn format_time_ago_just_now() {
        assert_eq!(format_time_ago(500), "just now");
    }

    #[test]
    fn format_time_ago_minutes() {
        assert_eq!(format_time_ago(5 * 60_000), "5m ago");
    }

    #[test]
    fn format_time_ago_hours() {
        assert_eq!(format_time_ago(3 * 60 * 60_000), "3h ago");
    }

    #[test]
    fn format_time_ago_days() {
        assert_eq!(format_time_ago(3 * 24 * 60 * 60_000), "3d ago");
    }

    #[test]
    fn format_tokens_cell_zero() {
        let result = format_tokens_cell(0, Some(200_000), false, 20);
        assert!(result.starts_with('-'));
    }

    #[test]
    fn format_tokens_cell_with_context() {
        let result = format_tokens_cell(10_000, Some(200_000), false, 20);
        assert!(result.contains("10k/200k"));
        assert!(result.contains("5%"));
    }

    #[test]
    fn format_tokens_cell_no_context() {
        let result = format_tokens_cell(10_000, None, false, 20);
        assert!(result.contains("10k/?"));
        assert!(result.contains("?%"));
    }

    #[test]
    fn format_kind_cell_direct() {
        let result = format_kind_cell(&SessionKind::Direct, false, 6);
        assert_eq!(result.trim(), "direct");
    }

    #[test]
    fn format_kind_cell_group() {
        let result = format_kind_cell(&SessionKind::Group, false, 6);
        assert_eq!(result.trim(), "group");
    }

    #[test]
    fn format_age_cell_unknown() {
        let result = format_age_cell(None, false, 9);
        assert!(result.contains("unknown"));
    }

    #[test]
    fn format_model_cell_default() {
        let result = format_model_cell(None, false, 14);
        assert!(result.contains("unknown"));
    }

    #[test]
    fn format_model_cell_custom() {
        let result = format_model_cell(Some("gpt-4o"), false, 14);
        assert!(result.contains("gpt-4o"));
    }

    #[test]
    fn format_flags_cell_empty() {
        let row = SessionRow {
            key: "test".to_string(),
            kind: SessionKind::Direct,
            updated_at: None,
            age_ms: None,
            session_id: None,
            system_sent: None,
            aborted_last_run: None,
            thinking_level: None,
            verbose_level: None,
            reasoning_level: None,
            elevated_level: None,
            response_usage: None,
            group_activation: None,
            input_tokens: None,
            output_tokens: None,
            total_tokens: None,
            model: None,
            context_tokens: None,
        };
        assert_eq!(format_flags_cell(&row, false), "");
    }

    #[test]
    fn format_flags_cell_with_flags() {
        let row = SessionRow {
            key: "test".to_string(),
            kind: SessionKind::Direct,
            updated_at: None,
            age_ms: None,
            session_id: Some("sid-123".to_string()),
            system_sent: Some(true),
            aborted_last_run: Some(true),
            thinking_level: Some("low".to_string()),
            verbose_level: None,
            reasoning_level: None,
            elevated_level: None,
            response_usage: None,
            group_activation: None,
            input_tokens: None,
            output_tokens: None,
            total_tokens: None,
            model: None,
            context_tokens: None,
        };
        let result = format_flags_cell(&row, false);
        assert!(result.contains("think:low"));
        assert!(result.contains("system"));
        assert!(result.contains("aborted"));
        assert!(result.contains("id:sid-123"));
    }

    #[test]
    fn color_by_pct_thresholds() {
        // No color when not rich
        assert_eq!(color_by_pct("test", Some(99), false), "test");
        // None percentage returns as-is
        assert_eq!(color_by_pct("test", None, false), "test");
    }

    #[test]
    fn pad_str_various() {
        assert_eq!(pad_str("ab", 5), "ab   ");
        assert_eq!(pad_str("abcde", 5), "abcde");
        assert_eq!(pad_str("abcdef", 5), "abcdef");
    }
}
