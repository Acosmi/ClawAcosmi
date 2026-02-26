/// Formatting utilities for status output.
///
/// Source: `src/commands/status.format.ts`

use crate::types::SessionStatus;

/// Format a token count as a compact "Xk" label.
///
/// Source: `src/commands/status.format.ts` - `formatKTokens`
#[must_use]
pub fn format_k_tokens(value: u64) -> String {
    let k = value as f64 / 1000.0;
    if value >= 10_000 {
        format!("{:.0}k", k)
    } else {
        format!("{:.1}k", k)
    }
}

/// Format a duration in milliseconds as a human-readable string.
///
/// Source: `src/commands/status.format.ts` - `formatDuration`
#[must_use]
pub fn format_duration(ms: Option<u64>) -> String {
    match ms {
        Some(v) if v < 1000 => format!("{v}ms"),
        Some(v) => format!("{:.1}s", v as f64 / 1000.0),
        None => "unknown".to_string(),
    }
}

/// Format a duration in milliseconds with precise output.
///
/// Source: `src/commands/status-all/format.ts` - `formatDurationPrecise`
#[must_use]
pub fn format_duration_precise(ms: u64) -> String {
    if ms < 1000 {
        format!("{ms}ms")
    } else if ms < 60_000 {
        format!("{:.1}s", ms as f64 / 1000.0)
    } else {
        let minutes = ms / 60_000;
        let seconds = (ms % 60_000) / 1000;
        format!("{minutes}m{seconds}s")
    }
}

/// Shorten a text string to a maximum length, adding an ellipsis if truncated.
///
/// Source: `src/commands/status.format.ts` - `shortenText`
#[must_use]
pub fn shorten_text(value: &str, max_len: usize) -> String {
    let chars: Vec<char> = value.chars().collect();
    if chars.len() <= max_len {
        return value.to_string();
    }
    let truncated: String = chars[..max_len.saturating_sub(1)].iter().collect();
    format!("{truncated}\u{2026}")
}

/// Format token usage in a compact representation for a session.
///
/// Source: `src/commands/status.format.ts` - `formatTokensCompact`
#[must_use]
pub fn format_tokens_compact(sess: &SessionStatus) -> String {
    let used = sess.total_tokens.unwrap_or(0);
    let ctx = sess.context_tokens;
    match ctx {
        Some(c) if c > 0 => {
            let pct_label = sess
                .percent_used
                .map_or("?%".to_string(), |p| format!("{p}%"));
            format!(
                "{}/{} ({pct_label})",
                format_k_tokens(used),
                format_k_tokens(c)
            )
        }
        _ => format!("{} used", format_k_tokens(used)),
    }
}

/// Format daemon runtime info as a short label.
///
/// Source: `src/commands/status.format.ts` - `formatDaemonRuntimeShort`
#[must_use]
pub fn format_daemon_runtime_short(runtime: Option<&DaemonRuntime>) -> Option<String> {
    let rt = runtime?;
    let status = rt.status.as_deref().unwrap_or("unknown");
    let mut details: Vec<String> = Vec::new();
    if let Some(pid) = rt.pid {
        details.push(format!("pid {pid}"));
    }
    if let Some(ref state) = rt.state {
        if state.to_lowercase() != status {
            details.push(format!("state {state}"));
        }
    }
    let detail = rt
        .detail
        .as_deref()
        .map(|d| {
            // Collapse whitespace.
            d.split_whitespace().collect::<Vec<_>>().join(" ")
        })
        .unwrap_or_default();
    let noisy_launchctl = rt.missing_unit == Some(true)
        && detail.to_lowercase().contains("could not find service");
    if !detail.is_empty() && !noisy_launchctl {
        details.push(detail);
    }
    if details.is_empty() {
        Some(status.to_string())
    } else {
        Some(format!("{status} ({})", details.join(", ")))
    }
}

/// Minimal daemon runtime struct for formatting.
///
/// Source: `src/commands/status.format.ts` - runtime parameter
#[derive(Debug, Clone, Default)]
pub struct DaemonRuntime {
    /// Runtime status label.
    pub status: Option<String>,
    /// Process ID.
    pub pid: Option<u32>,
    /// State label.
    pub state: Option<String>,
    /// Detail string.
    pub detail: Option<String>,
    /// Whether the unit file is missing.
    pub missing_unit: Option<bool>,
}

/// Format a relative time ago from milliseconds.
///
/// Source: `src/infra/format-time/format-relative.ts` - `formatTimeAgo`
#[must_use]
pub fn format_time_ago(age_ms: u64) -> String {
    let seconds = age_ms / 1000;
    if seconds < 60 {
        return format!("{seconds}s");
    }
    let minutes = seconds / 60;
    if minutes < 60 {
        return format!("{minutes}m");
    }
    let hours = minutes / 60;
    if hours < 24 {
        let remaining_min = minutes % 60;
        if remaining_min > 0 {
            return format!("{hours}h{remaining_min}m");
        }
        return format!("{hours}h");
    }
    let days = hours / 24;
    if days < 30 {
        return format!("{days}d");
    }
    let months = days / 30;
    format!("{months}mo")
}

/// Format gateway auth used label.
///
/// Source: `src/commands/status-all/format.ts` - `formatGatewayAuthUsed`
#[must_use]
pub fn format_gateway_auth_used(auth: Option<&GatewayProbeAuth>) -> &'static str {
    let Some(a) = auth else {
        return "none";
    };
    let has_token = a
        .token
        .as_deref()
        .is_some_and(|t| !t.trim().is_empty());
    let has_password = a
        .password
        .as_deref()
        .is_some_and(|p| !p.trim().is_empty());
    match (has_token, has_password) {
        (true, true) => "token+password",
        (true, false) => "token",
        (false, true) => "password",
        (false, false) => "none",
    }
}

/// Gateway probe auth credentials.
///
/// Source: `src/commands/status.gateway-probe.ts` - return type
#[derive(Debug, Clone, Default)]
pub struct GatewayProbeAuth {
    /// Token credential.
    pub token: Option<String>,
    /// Password credential.
    pub password: Option<String>,
}

/// Redact secrets from a text string.
///
/// Source: `src/commands/status-all/format.ts` - `redactSecrets`
#[must_use]
pub fn redact_secrets(text: &str) -> String {
    if text.is_empty() {
        return text.to_string();
    }
    let mut out = text.to_string();
    // Redact token/secret/password/api-key patterns.
    let re = regex::Regex::new(
        r#"(?i)(\b(?:access[_-]?token|refresh[_-]?token|token|password|secret|api[_-]?key)\b\s*[:=]\s*)(["']?)([^"'\s]+)(["']?)"#,
    );
    if let Ok(re) = re {
        out = re.replace_all(&out, "$1$2***$4").to_string();
    }
    // Redact Bearer tokens.
    let bearer_re = regex::Regex::new(r"\bBearer\s+[A-Za-z0-9._-]+\b");
    if let Ok(re) = bearer_re {
        out = re.replace_all(&out, "Bearer ***").to_string();
    }
    // Redact sk- API keys.
    let sk_re = regex::Regex::new(r"\bsk-[A-Za-z0-9]{10,}\b");
    if let Ok(re) = sk_re {
        out = re.replace_all(&out, "sk-***").to_string();
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn format_k_tokens_small() {
        assert_eq!(format_k_tokens(500), "0.5k");
        assert_eq!(format_k_tokens(1000), "1.0k");
        assert_eq!(format_k_tokens(9999), "10.0k");
    }

    #[test]
    fn format_k_tokens_large() {
        assert_eq!(format_k_tokens(10_000), "10k");
        assert_eq!(format_k_tokens(128_000), "128k");
    }

    #[test]
    fn format_duration_values() {
        assert_eq!(format_duration(None), "unknown");
        assert_eq!(format_duration(Some(50)), "50ms");
        assert_eq!(format_duration(Some(999)), "999ms");
        assert_eq!(format_duration(Some(1500)), "1.5s");
    }

    #[test]
    fn format_duration_precise_values() {
        assert_eq!(format_duration_precise(50), "50ms");
        assert_eq!(format_duration_precise(1500), "1.5s");
        assert_eq!(format_duration_precise(90_000), "1m30s");
    }

    #[test]
    fn shorten_text_short() {
        assert_eq!(shorten_text("hello", 10), "hello");
    }

    #[test]
    fn shorten_text_exact() {
        assert_eq!(shorten_text("hello", 5), "hello");
    }

    #[test]
    fn shorten_text_truncated() {
        let result = shorten_text("hello world foo", 10);
        assert!(result.chars().count() <= 10); // 9 chars + 1 ellipsis char
        assert!(result.ends_with('\u{2026}'));
    }

    #[test]
    fn format_tokens_compact_with_context() {
        let sess = SessionStatus {
            agent_id: None,
            key: "k".to_string(),
            kind: crate::types::SessionKind::Direct,
            session_id: None,
            updated_at: None,
            age: None,
            thinking_level: None,
            verbose_level: None,
            reasoning_level: None,
            elevated_level: None,
            system_sent: None,
            aborted_last_run: None,
            input_tokens: None,
            output_tokens: None,
            total_tokens: Some(5000),
            remaining_tokens: Some(123_000),
            percent_used: Some(4),
            model: None,
            context_tokens: Some(128_000),
            flags: vec![],
        };
        let result = format_tokens_compact(&sess);
        assert!(result.contains("5.0k"));
        assert!(result.contains("128k"));
        assert!(result.contains("4%"));
    }

    #[test]
    fn format_tokens_compact_no_context() {
        let sess = SessionStatus {
            agent_id: None,
            key: "k".to_string(),
            kind: crate::types::SessionKind::Direct,
            session_id: None,
            updated_at: None,
            age: None,
            thinking_level: None,
            verbose_level: None,
            reasoning_level: None,
            elevated_level: None,
            system_sent: None,
            aborted_last_run: None,
            input_tokens: None,
            output_tokens: None,
            total_tokens: Some(5000),
            remaining_tokens: None,
            percent_used: None,
            model: None,
            context_tokens: None,
            flags: vec![],
        };
        let result = format_tokens_compact(&sess);
        assert!(result.contains("5.0k used"));
    }

    #[test]
    fn format_time_ago_seconds() {
        assert_eq!(format_time_ago(5_000), "5s");
        assert_eq!(format_time_ago(59_000), "59s");
    }

    #[test]
    fn format_time_ago_minutes() {
        assert_eq!(format_time_ago(60_000), "1m");
        assert_eq!(format_time_ago(3_599_000), "59m");
    }

    #[test]
    fn format_time_ago_hours() {
        assert_eq!(format_time_ago(3_600_000), "1h");
        assert_eq!(format_time_ago(7_200_000 + 1_800_000), "2h30m");
    }

    #[test]
    fn format_time_ago_days() {
        assert_eq!(format_time_ago(86_400_000), "1d");
    }

    #[test]
    fn format_gateway_auth_used_variants() {
        assert_eq!(format_gateway_auth_used(None), "none");
        let auth_token = GatewayProbeAuth {
            token: Some("t".to_string()),
            password: None,
        };
        assert_eq!(format_gateway_auth_used(Some(&auth_token)), "token");
        let auth_pw = GatewayProbeAuth {
            token: None,
            password: Some("p".to_string()),
        };
        assert_eq!(format_gateway_auth_used(Some(&auth_pw)), "password");
        let auth_both = GatewayProbeAuth {
            token: Some("t".to_string()),
            password: Some("p".to_string()),
        };
        assert_eq!(
            format_gateway_auth_used(Some(&auth_both)),
            "token+password"
        );
    }

    #[test]
    fn daemon_runtime_format() {
        assert!(format_daemon_runtime_short(None).is_none());
        let rt = DaemonRuntime {
            status: Some("running".to_string()),
            pid: Some(1234),
            ..Default::default()
        };
        let result = format_daemon_runtime_short(Some(&rt)).unwrap_or_default();
        assert!(result.contains("running"));
        assert!(result.contains("pid 1234"));
    }

    #[test]
    fn redact_secrets_bearer() {
        let input = "Authorization: Bearer sk-abc123456789xyz";
        let result = redact_secrets(input);
        assert!(result.contains("Bearer ***"));
        assert!(!result.contains("sk-abc123456789xyz"));
    }

    #[test]
    fn redact_secrets_key() {
        let input = "token=mysecrettoken123";
        let result = redact_secrets(input);
        assert!(result.contains("***"));
    }
}
