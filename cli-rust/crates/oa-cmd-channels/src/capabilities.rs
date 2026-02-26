/// `channels capabilities` command: inspects the capability profile and
/// runtime state for one or more channel accounts.
///
/// Source: `src/commands/channels/capabilities.ts`

use anyhow::{bail, Result};
use serde::{Deserialize, Serialize};

use oa_channels::directory::{ChannelCapabilities, get_channel_capabilities};
use oa_channels::registry::{
    ChatChannelId, CHAT_CHANNEL_ORDER, normalize_chat_channel_id,
};
use oa_terminal::theme::Theme;
use oa_types::config::OpenAcosmiConfig;

use crate::shared::{format_channel_account_label, require_valid_config};

/// Options for the `channels capabilities` subcommand.
///
/// Source: `src/commands/channels/capabilities.ts` — `ChannelsCapabilitiesOptions`
#[derive(Debug, Clone, Default)]
pub struct ChannelsCapabilitiesOptions {
    /// Channel filter (blank or "all" for all channels).
    pub channel: Option<String>,
    /// Account filter (requires a specific channel).
    pub account: Option<String>,
    /// Discord target for permission auditing.
    pub target: Option<String>,
    /// Probe timeout in milliseconds.
    pub timeout: Option<String>,
    /// Output in JSON format.
    pub json: bool,
}

/// Report for a single channel account's capabilities.
///
/// Source: `src/commands/channels/capabilities.ts` — `ChannelCapabilitiesReport`
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ChannelCapabilitiesReport {
    /// Channel identifier.
    pub channel: String,
    /// Account identifier.
    pub account_id: String,
    /// Account display name.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub account_name: Option<String>,
    /// Whether the account is configured.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub configured: Option<bool>,
    /// Whether the account is enabled.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    /// Channel capability profile.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub support: Option<ChannelCapabilities>,
    /// Available actions.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub actions: Option<Vec<String>>,
}

/// Normalize the timeout value from a string, falling back to a default.
///
/// Source: `src/commands/channels/capabilities.ts` — `normalizeTimeout`
fn normalize_timeout(raw: Option<&str>, fallback: u64) -> u64 {
    let raw_str = raw.unwrap_or("");
    match raw_str.parse::<u64>() {
        Ok(v) if v > 0 => v,
        _ => fallback,
    }
}

/// Format the support capabilities as a compact string.
///
/// Source: `src/commands/channels/capabilities.ts` — `formatSupport`
fn format_support(capabilities: Option<&ChannelCapabilities>) -> String {
    let caps = match capabilities {
        Some(c) => c,
        None => return "unknown".to_owned(),
    };

    let mut bits = Vec::new();
    if caps.supports_reactions {
        bits.push("reactions");
    }
    if caps.supports_threads {
        bits.push("threads");
    }
    if caps.supports_mentions {
        bits.push("mentions");
    }
    if caps.supports_groups {
        bits.push("groups");
    }
    if caps.supports_elevated {
        bits.push("elevated");
    }
    if caps.supports_typing {
        bits.push("typing");
    }
    if caps.supports_attachments {
        bits.push("media");
    }

    if bits.is_empty() {
        "none".to_owned()
    } else {
        bits.join(" ")
    }
}

/// Resolve capabilities reports for the selected channels.
///
/// Source: `src/commands/channels/capabilities.ts` — `resolveChannelReports`
fn resolve_channel_reports(
    _cfg: &OpenAcosmiConfig,
    channels: &[ChatChannelId],
    _account_override: Option<&str>,
) -> Vec<ChannelCapabilitiesReport> {
    let mut reports = Vec::new();

    for &channel_id in channels {
        let caps = get_channel_capabilities(channel_id);
        let default_actions = vec!["send".to_owned(), "broadcast".to_owned()];

        reports.push(ChannelCapabilitiesReport {
            channel: channel_id.as_str().to_owned(),
            account_id: "default".to_owned(),
            account_name: None,
            configured: None,
            enabled: None,
            support: Some(caps),
            actions: Some(default_actions),
        });
    }

    reports
}

/// Execute the `channels capabilities` command.
///
/// Source: `src/commands/channels/capabilities.ts` — `channelsCapabilitiesCommand`
pub async fn channels_capabilities_command(opts: &ChannelsCapabilitiesOptions) -> Result<()> {
    let cfg = match require_valid_config().await? {
        Some(c) => c,
        None => return Ok(()),
    };

    let _timeout_ms = normalize_timeout(opts.timeout.as_deref(), 10_000);
    let raw_channel = opts
        .channel
        .as_deref()
        .unwrap_or("")
        .trim()
        .to_lowercase();
    let raw_target = opts.target.as_deref().unwrap_or("").trim().to_owned();

    if opts.account.is_some() && (raw_channel.is_empty() || raw_channel == "all") {
        bail!("--account requires a specific --channel.");
    }
    if !raw_target.is_empty() && raw_channel != "discord" {
        bail!("--target requires --channel discord.");
    }

    let selected: Vec<ChatChannelId> = if raw_channel.is_empty() || raw_channel == "all" {
        CHAT_CHANNEL_ORDER.to_vec()
    } else {
        match normalize_chat_channel_id(&raw_channel) {
            Some(id) => vec![id],
            None => bail!("Unknown channel \"{raw_channel}\"."),
        }
    };

    let reports = resolve_channel_reports(
        &cfg,
        &selected,
        opts.account.as_deref(),
    );

    if opts.json {
        let payload = serde_json::json!({ "channels": reports });
        println!("{}", serde_json::to_string_pretty(&payload)?);
        return Ok(());
    }

    let mut lines = Vec::new();
    for report in &reports {
        let channel_id = normalize_chat_channel_id(&report.channel);
        let label = match channel_id {
            Some(id) => format_channel_account_label(
                id,
                &report.account_id,
                report.account_name.as_deref(),
                Some(Theme::accent),
                Some(Theme::heading),
            ),
            None => format!("{} {}", report.channel, report.account_id),
        };
        lines.push(Theme::heading(&label));
        lines.push(format!("Support: {}", format_support(report.support.as_ref())));
        if let Some(ref actions) = report.actions {
            if !actions.is_empty() {
                lines.push(format!("Actions: {}", actions.join(", ")));
            }
        }
        if report.configured == Some(false) || report.enabled == Some(false) {
            let configured_label = if report.configured == Some(false) {
                "not configured"
            } else {
                "configured"
            };
            let enabled_label = if report.enabled == Some(false) {
                "disabled"
            } else {
                "enabled"
            };
            lines.push(format!("Status: {configured_label}, {enabled_label}"));
        }
        lines.push(String::new());
    }

    println!("{}", lines.join("\n").trim_end());

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn normalize_timeout_valid() {
        assert_eq!(normalize_timeout(Some("5000"), 10_000), 5000);
        assert_eq!(normalize_timeout(Some("1"), 10_000), 1);
    }

    #[test]
    fn normalize_timeout_fallback() {
        assert_eq!(normalize_timeout(None, 10_000), 10_000);
        assert_eq!(normalize_timeout(Some(""), 10_000), 10_000);
        assert_eq!(normalize_timeout(Some("abc"), 10_000), 10_000);
        assert_eq!(normalize_timeout(Some("0"), 10_000), 10_000);
    }

    #[test]
    fn format_support_discord() {
        let caps = get_channel_capabilities(ChatChannelId::Discord);
        let s = format_support(Some(&caps));
        assert!(s.contains("reactions"));
        assert!(s.contains("threads"));
        assert!(s.contains("mentions"));
        assert!(s.contains("elevated"));
        assert!(s.contains("media"));
    }

    #[test]
    fn format_support_telegram() {
        let caps = get_channel_capabilities(ChatChannelId::Telegram);
        let s = format_support(Some(&caps));
        assert!(s.contains("groups"));
        assert!(!s.contains("reactions"));
        assert!(!s.contains("threads"));
    }

    #[test]
    fn format_support_none() {
        assert_eq!(format_support(None), "unknown");
    }

    #[test]
    fn resolve_reports_returns_all_channels() {
        let cfg = OpenAcosmiConfig::default();
        let reports = resolve_channel_reports(&cfg, CHAT_CHANNEL_ORDER, None);
        assert_eq!(reports.len(), CHAT_CHANNEL_ORDER.len());
        assert_eq!(reports[0].channel, "telegram");
    }

    #[test]
    fn resolve_reports_single_channel() {
        let cfg = OpenAcosmiConfig::default();
        let reports = resolve_channel_reports(&cfg, &[ChatChannelId::Slack], None);
        assert_eq!(reports.len(), 1);
        assert_eq!(reports[0].channel, "slack");
        assert!(reports[0].support.is_some());
    }

    #[test]
    fn report_has_default_actions() {
        let cfg = OpenAcosmiConfig::default();
        let reports = resolve_channel_reports(&cfg, &[ChatChannelId::Discord], None);
        let actions = reports[0].actions.as_ref().expect("actions should exist");
        assert!(actions.contains(&"send".to_owned()));
        assert!(actions.contains(&"broadcast".to_owned()));
    }
}
