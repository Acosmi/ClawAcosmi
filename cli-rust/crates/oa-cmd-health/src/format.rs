/// Health channel line formatting.
///
/// Builds and styles the per-channel health status lines for terminal display.
///
/// Source: `src/commands/health.ts` - `formatHealthChannelLines`, `styleHealthChannelLine`,
///         `formatProbeLine`, `formatAccountProbeTiming`, `isProbeFailure`

use oa_channels::get_chat_channel_meta;
use oa_terminal::theme::Theme;

use crate::types::{ChannelAccountHealthSummary, ChannelHealthSummary, HealthSummary};

/// Extract a value from a probe JSON object.
///
/// Source: `src/commands/health.ts` - `asRecord` helper pattern
fn probe_field_bool(probe: Option<&serde_json::Value>, field: &str) -> Option<bool> {
    probe?.get(field)?.as_bool()
}

/// Extract a numeric field from a probe JSON object.
///
/// Source: `src/commands/health.ts` - probe field extraction
fn probe_field_f64(probe: Option<&serde_json::Value>, field: &str) -> Option<f64> {
    probe?.get(field)?.as_f64()
}

/// Extract a string field from a probe JSON object.
///
/// Source: `src/commands/health.ts` - probe field extraction
fn probe_field_str<'a>(probe: Option<&'a serde_json::Value>, field: &str) -> Option<&'a str> {
    probe?.get(field)?.as_str()
}

/// Check if a probe result indicates failure.
///
/// Source: `src/commands/health.ts` - `isProbeFailure`
fn is_probe_failure(summary: &ChannelAccountHealthSummary) -> bool {
    probe_field_bool(summary.probe.as_ref(), "ok") == Some(false)
}

/// Format a single probe result line.
///
/// Source: `src/commands/health.ts` - `formatProbeLine`
fn format_probe_line(
    probe: Option<&serde_json::Value>,
    bot_usernames: &[String],
) -> Option<String> {
    let probe = probe?;
    let ok = probe_field_bool(Some(probe), "ok")?;
    let elapsed_ms = probe_field_f64(Some(probe), "elapsedMs");
    let status = probe_field_f64(Some(probe), "status").map(|v| v as i64);
    let error = probe_field_str(Some(probe), "error");
    let bot_username = probe
        .get("bot")
        .and_then(|b| b.get("username"))
        .and_then(serde_json::Value::as_str);
    let webhook_url = probe
        .get("webhook")
        .and_then(|w| w.get("url"))
        .and_then(serde_json::Value::as_str);

    let mut usernames: Vec<String> = Vec::new();
    if let Some(un) = bot_username {
        usernames.push(un.to_string());
    }
    for extra in bot_usernames {
        if !extra.is_empty() && !usernames.contains(extra) {
            usernames.push(extra.clone());
        }
    }

    if ok {
        let mut label = "ok".to_string();
        if !usernames.is_empty() {
            let at_usernames: Vec<String> = usernames.iter().map(|u| format!("@{u}")).collect();
            label += &format!(" ({})", at_usernames.join(", "));
        }
        if let Some(ms) = elapsed_ms {
            label += &format!(" ({ms}ms)");
        }
        if let Some(url) = webhook_url {
            label += &format!(" - webhook {url}");
        }
        return Some(label);
    }

    let status_str = status
        .map_or_else(|| "unknown".to_string(), |s| s.to_string());
    let mut label = format!("failed ({status_str})");
    if let Some(err) = error {
        label += &format!(" - {err}");
    }
    Some(label)
}

/// Format account probe timing for verbose mode.
///
/// Source: `src/commands/health.ts` - `formatAccountProbeTiming`
fn format_account_probe_timing(summary: &ChannelAccountHealthSummary) -> Option<String> {
    let probe = summary.probe.as_ref()?;
    let elapsed_ms = probe_field_f64(Some(probe), "elapsedMs").map(|v| v.round() as i64);
    let ok = probe_field_bool(Some(probe), "ok");

    if elapsed_ms.is_none() && ok != Some(true) {
        return None;
    }

    let account_id = if summary.account_id.is_empty() {
        "default"
    } else {
        &summary.account_id
    };

    let bot_username = probe
        .get("bot")
        .and_then(|b| b.get("username"))
        .and_then(serde_json::Value::as_str);

    let handle = bot_username.map_or_else(
        || account_id.to_string(),
        |u| format!("@{u}"),
    );

    let timing = elapsed_ms.map_or_else(
        || "ok".to_string(),
        |ms| format!("{ms}ms"),
    );

    Some(format!("{handle}:{account_id}:{timing}"))
}

/// Build the formatted channel health lines for display.
///
/// Source: `src/commands/health.ts` - `formatHealthChannelLines`
pub fn format_health_channel_lines(summary: &HealthSummary, verbose: bool) -> Vec<String> {
    let channels = &summary.channels;
    let channel_order = if summary.channel_order.is_empty() {
        channels.keys().cloned().collect::<Vec<_>>()
    } else {
        summary.channel_order.clone()
    };

    let mut lines = Vec::new();

    for channel_id in &channel_order {
        let channel_summary = match channels.get(channel_id) {
            Some(s) => s,
            None => continue,
        };

        let label = summary
            .channel_labels
            .get(channel_id)
            .cloned()
            .or_else(|| resolve_channel_label(channel_id))
            .unwrap_or_else(|| channel_id.clone());

        let account_summaries = channel_summary.accounts.as_ref();

        let list_summaries: Vec<&ChannelAccountHealthSummary> = if verbose {
            account_summaries.map_or_else(Vec::new, |accts| accts.values().collect())
        } else {
            account_summaries.map_or_else(Vec::new, |accts| accts.values().collect())
        };

        let bot_usernames: Vec<String> = list_summaries
            .iter()
            .filter_map(|account| {
                account
                    .probe
                    .as_ref()
                    .and_then(|p| p.get("bot"))
                    .and_then(|b| b.get("username"))
                    .and_then(serde_json::Value::as_str)
                    .map(String::from)
            })
            .collect();

        // Check linked status
        if let Some(linked) = channel_summary.linked {
            if linked {
                let auth_label = channel_summary
                    .auth_age_ms
                    .map(|ms| format!(" (auth age {}m)", (ms / 60_000.0).round() as i64))
                    .unwrap_or_default();
                lines.push(format!("{label}: linked{auth_label}"));
            } else {
                lines.push(format!("{label}: not linked"));
            }
            continue;
        }

        // Check configured status
        if channel_summary.configured == Some(false) {
            lines.push(format!("{label}: not configured"));
            continue;
        }

        // Build account timings for verbose mode
        let account_timings: Vec<String> = if verbose {
            list_summaries
                .iter()
                .filter_map(|account| format_account_probe_timing(account))
                .collect()
        } else {
            Vec::new()
        };

        // Check for probe failures
        let failed_summary = list_summaries.iter().find(|s| is_probe_failure(s));
        if let Some(failed) = failed_summary {
            if let Some(failure_line) = format_probe_line(failed.probe.as_ref(), &bot_usernames) {
                lines.push(format!("{label}: {failure_line}"));
                continue;
            }
        }

        // Show account timings
        if !account_timings.is_empty() {
            lines.push(format!("{label}: ok ({})", account_timings.join(", ")));
            continue;
        }

        // Try default probe line
        let base_summary = build_base_summary(channel_summary);
        if let Some(probe_line) = format_probe_line(base_summary.probe.as_ref(), &bot_usernames) {
            lines.push(format!("{label}: {probe_line}"));
            continue;
        }

        // Fallback statuses
        if channel_summary.configured == Some(true) {
            lines.push(format!("{label}: configured"));
            continue;
        }

        lines.push(format!("{label}: unknown"));
    }

    lines
}

/// Build a base summary from the channel summary for probe formatting.
///
/// Source: `src/commands/health.ts` - fallback summary logic
fn build_base_summary(channel_summary: &ChannelHealthSummary) -> ChannelAccountHealthSummary {
    ChannelAccountHealthSummary {
        account_id: channel_summary.account_id.clone(),
        configured: channel_summary.configured,
        linked: channel_summary.linked,
        auth_age_ms: channel_summary.auth_age_ms,
        probe: channel_summary.probe.clone(),
        last_probe_at: channel_summary.last_probe_at,
        extra: Default::default(),
    }
}

/// Resolve the human-readable label for a channel ID.
///
/// Source: `src/commands/health.ts` - channel label resolution
fn resolve_channel_label(channel_id: &str) -> Option<String> {
    let normalized = oa_channels::normalize_chat_channel_id(channel_id)?;
    let meta = get_chat_channel_meta(normalized);
    Some(meta.label.to_string())
}

/// Apply terminal styling to a channel health line.
///
/// Colors the status prefix (ok, failed, linked, etc.) appropriately.
///
/// Source: `src/commands/health.ts` - `styleHealthChannelLine`
pub fn style_health_channel_line(line: &str) -> String {
    let colon = match line.find(':') {
        Some(idx) => idx,
        None => return line.to_string(),
    };

    let label = &line[..=colon];
    let detail = line[colon + 1..].trim_start();
    let normalized = detail.to_lowercase();

    let apply_prefix = |prefix: &str, color_fn: fn(&str) -> String| -> String {
        format!(
            "{} {}{}",
            label,
            color_fn(&detail[..prefix.len()]),
            &detail[prefix.len()..]
        )
    };

    if normalized.starts_with("failed") {
        return apply_prefix("failed", Theme::error);
    }
    if normalized.starts_with("ok") {
        return apply_prefix("ok", Theme::success);
    }
    if normalized.starts_with("linked") {
        return apply_prefix("linked", Theme::success);
    }
    if normalized.starts_with("configured") {
        return apply_prefix("configured", Theme::success);
    }
    if normalized.starts_with("not linked") {
        return apply_prefix("not linked", Theme::warn);
    }
    if normalized.starts_with("not configured") {
        return apply_prefix("not configured", Theme::muted);
    }
    if normalized.starts_with("unknown") {
        return apply_prefix("unknown", Theme::warn);
    }

    line.to_string()
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::HashMap;

    fn make_summary_with_channels(
        channels: HashMap<String, ChannelHealthSummary>,
        order: Vec<String>,
        labels: HashMap<String, String>,
    ) -> HealthSummary {
        HealthSummary {
            ok: true,
            ts: 1_700_000_000_000,
            duration_ms: 100,
            channels,
            channel_order: order,
            channel_labels: labels,
            heartbeat_seconds: 30,
            default_agent_id: "main".to_string(),
            agents: vec![],
            sessions: crate::types::SessionSummary::default(),
        }
    }

    #[test]
    fn format_lines_not_configured() {
        let mut channels = HashMap::new();
        channels.insert(
            "telegram".to_string(),
            ChannelHealthSummary {
                configured: Some(false),
                ..Default::default()
            },
        );
        let mut labels = HashMap::new();
        labels.insert("telegram".to_string(), "Telegram".to_string());
        let summary =
            make_summary_with_channels(channels, vec!["telegram".to_string()], labels);
        let lines = format_health_channel_lines(&summary, false);
        assert_eq!(lines.len(), 1);
        assert_eq!(lines[0], "Telegram: not configured");
    }

    #[test]
    fn format_lines_linked() {
        let mut channels = HashMap::new();
        channels.insert(
            "whatsapp".to_string(),
            ChannelHealthSummary {
                linked: Some(true),
                auth_age_ms: Some(300_000.0),
                ..Default::default()
            },
        );
        let mut labels = HashMap::new();
        labels.insert("whatsapp".to_string(), "WhatsApp".to_string());
        let summary =
            make_summary_with_channels(channels, vec!["whatsapp".to_string()], labels);
        let lines = format_health_channel_lines(&summary, false);
        assert_eq!(lines.len(), 1);
        assert_eq!(lines[0], "WhatsApp: linked (auth age 5m)");
    }

    #[test]
    fn format_lines_not_linked() {
        let mut channels = HashMap::new();
        channels.insert(
            "whatsapp".to_string(),
            ChannelHealthSummary {
                linked: Some(false),
                ..Default::default()
            },
        );
        let mut labels = HashMap::new();
        labels.insert("whatsapp".to_string(), "WhatsApp".to_string());
        let summary =
            make_summary_with_channels(channels, vec!["whatsapp".to_string()], labels);
        let lines = format_health_channel_lines(&summary, false);
        assert_eq!(lines[0], "WhatsApp: not linked");
    }

    #[test]
    fn format_lines_probe_ok() {
        let mut channels = HashMap::new();
        channels.insert(
            "telegram".to_string(),
            ChannelHealthSummary {
                configured: Some(true),
                probe: Some(serde_json::json!({
                    "ok": true,
                    "elapsedMs": 42,
                    "bot": { "username": "testbot" }
                })),
                ..Default::default()
            },
        );
        let mut labels = HashMap::new();
        labels.insert("telegram".to_string(), "Telegram".to_string());
        let summary =
            make_summary_with_channels(channels, vec!["telegram".to_string()], labels);
        let lines = format_health_channel_lines(&summary, false);
        assert_eq!(lines.len(), 1);
        assert!(lines[0].contains("ok"));
        assert!(lines[0].contains("@testbot"));
        assert!(lines[0].contains("42ms"));
    }

    #[test]
    fn format_lines_probe_failed() {
        let mut channels = HashMap::new();
        channels.insert(
            "telegram".to_string(),
            ChannelHealthSummary {
                configured: Some(true),
                probe: Some(serde_json::json!({
                    "ok": false,
                    "status": 401,
                    "error": "Unauthorized"
                })),
                ..Default::default()
            },
        );
        let mut labels = HashMap::new();
        labels.insert("telegram".to_string(), "Telegram".to_string());
        let summary =
            make_summary_with_channels(channels, vec!["telegram".to_string()], labels);
        let lines = format_health_channel_lines(&summary, false);
        assert_eq!(lines.len(), 1);
        assert!(lines[0].contains("failed"));
        assert!(lines[0].contains("401"));
        assert!(lines[0].contains("Unauthorized"));
    }

    #[test]
    fn format_lines_configured_no_probe() {
        let mut channels = HashMap::new();
        channels.insert(
            "discord".to_string(),
            ChannelHealthSummary {
                configured: Some(true),
                ..Default::default()
            },
        );
        let mut labels = HashMap::new();
        labels.insert("discord".to_string(), "Discord".to_string());
        let summary =
            make_summary_with_channels(channels, vec!["discord".to_string()], labels);
        let lines = format_health_channel_lines(&summary, false);
        assert_eq!(lines[0], "Discord: configured");
    }

    #[test]
    fn format_lines_unknown() {
        let mut channels = HashMap::new();
        channels.insert(
            "signal".to_string(),
            ChannelHealthSummary::default(),
        );
        let mut labels = HashMap::new();
        labels.insert("signal".to_string(), "Signal".to_string());
        let summary =
            make_summary_with_channels(channels, vec!["signal".to_string()], labels);
        let lines = format_health_channel_lines(&summary, false);
        assert_eq!(lines[0], "Signal: unknown");
    }

    #[test]
    fn style_line_ok() {
        let styled = style_health_channel_line("Telegram: ok (@bot) (42ms)");
        assert!(!styled.is_empty());
    }

    #[test]
    fn style_line_failed() {
        let styled = style_health_channel_line("Telegram: failed (401) - Unauthorized");
        assert!(!styled.is_empty());
    }

    #[test]
    fn style_line_not_configured() {
        let styled = style_health_channel_line("Signal: not configured");
        assert!(!styled.is_empty());
    }

    #[test]
    fn style_line_no_colon() {
        let input = "no colon here";
        assert_eq!(style_health_channel_line(input), input);
    }

    #[test]
    fn format_probe_line_ok_with_bot() {
        let probe = serde_json::json!({
            "ok": true,
            "elapsedMs": 55,
            "bot": { "username": "mybot" }
        });
        let result = format_probe_line(Some(&probe), &[]);
        assert_eq!(result, Some("ok (@mybot) (55ms)".to_string()));
    }

    #[test]
    fn format_probe_line_failed() {
        let probe = serde_json::json!({
            "ok": false,
            "status": 500,
            "error": "Internal Server Error"
        });
        let result = format_probe_line(Some(&probe), &[]);
        assert_eq!(
            result,
            Some("failed (500) - Internal Server Error".to_string())
        );
    }

    #[test]
    fn format_probe_line_none() {
        assert_eq!(format_probe_line(None, &[]), None);
    }

    #[test]
    fn format_probe_line_no_ok_field() {
        let probe = serde_json::json!({ "elapsedMs": 42 });
        assert_eq!(format_probe_line(Some(&probe), &[]), None);
    }

    #[test]
    fn is_probe_failure_true() {
        let summary = ChannelAccountHealthSummary {
            probe: Some(serde_json::json!({ "ok": false })),
            ..Default::default()
        };
        assert!(is_probe_failure(&summary));
    }

    #[test]
    fn is_probe_failure_false_when_ok() {
        let summary = ChannelAccountHealthSummary {
            probe: Some(serde_json::json!({ "ok": true })),
            ..Default::default()
        };
        assert!(!is_probe_failure(&summary));
    }

    #[test]
    fn is_probe_failure_false_when_no_probe() {
        let summary = ChannelAccountHealthSummary::default();
        assert!(!is_probe_failure(&summary));
    }
}
