/// `channels status` command: queries the gateway for live channel status or
/// falls back to config-only status when the gateway is unreachable.
///
/// Source: `src/commands/channels/status.ts`

use std::collections::HashMap;

use anyhow::Result;
use serde::{Deserialize, Serialize};

use oa_channels::registry::CHAT_CHANNEL_ORDER;
use oa_terminal::links::format_docs_link;
use oa_terminal::theme::Theme;

use crate::shared::format_channel_account_label;

/// Options for the `channels status` subcommand.
///
/// Source: `src/commands/channels/status.ts` — `ChannelsStatusOptions`
#[derive(Debug, Clone, Default)]
pub struct ChannelsStatusOptions {
    /// Output in JSON format.
    pub json: bool,
    /// Include probe results.
    pub probe: bool,
    /// Timeout in milliseconds.
    pub timeout: Option<String>,
}

/// A single account status entry from the gateway response.
///
/// Source: `src/commands/channels/status.ts` — inline account shape
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ChannelAccountStatus {
    /// Account identifier.
    #[serde(default)]
    pub account_id: Option<String>,
    /// Display name.
    #[serde(default)]
    pub name: Option<String>,
    /// Whether the account is enabled.
    #[serde(default)]
    pub enabled: Option<bool>,
    /// Whether the account is configured.
    #[serde(default)]
    pub configured: Option<bool>,
    /// Whether the account is linked (bound to an agent).
    #[serde(default)]
    pub linked: Option<bool>,
    /// Whether the channel adapter is running.
    #[serde(default)]
    pub running: Option<bool>,
    /// Whether the channel adapter is connected.
    #[serde(default)]
    pub connected: Option<bool>,
    /// Adapter operating mode.
    #[serde(default)]
    pub mode: Option<String>,
    /// Token source descriptor.
    #[serde(default)]
    pub token_source: Option<String>,
    /// Bot token source descriptor.
    #[serde(default)]
    pub bot_token_source: Option<String>,
    /// App token source descriptor.
    #[serde(default)]
    pub app_token_source: Option<String>,
    /// Base URL override.
    #[serde(default)]
    pub base_url: Option<String>,
    /// Last inbound message timestamp (epoch ms).
    #[serde(default)]
    pub last_inbound_at: Option<f64>,
    /// Last outbound message timestamp (epoch ms).
    #[serde(default)]
    pub last_outbound_at: Option<f64>,
    /// Last error message.
    #[serde(default)]
    pub last_error: Option<String>,
    /// DM policy.
    #[serde(default)]
    pub dm_policy: Option<String>,
}

/// Format a relative time-ago string from a duration in milliseconds.
///
/// Source: `src/infra/format-time/format-relative.ts` — `formatTimeAgo`
fn format_time_ago(ms: f64) -> String {
    if ms < 0.0 {
        return "just now".to_owned();
    }
    let seconds = (ms / 1000.0) as u64;
    if seconds < 60 {
        return format!("{seconds}s ago");
    }
    let minutes = seconds / 60;
    if minutes < 60 {
        return format!("{minutes}m ago");
    }
    let hours = minutes / 60;
    if hours < 24 {
        return format!("{hours}h ago");
    }
    let days = hours / 24;
    format!("{days}d ago")
}

/// Format status bits for a single account from the gateway response.
///
/// Source: `src/commands/channels/status.ts` — `accountLines` (inner closure)
fn format_account_status_bits(account: &ChannelAccountStatus) -> Vec<String> {
    let mut bits = Vec::new();

    if let Some(enabled) = account.enabled {
        bits.push(if enabled {
            "enabled".to_owned()
        } else {
            "disabled".to_owned()
        });
    }
    if let Some(configured) = account.configured {
        bits.push(if configured {
            "configured".to_owned()
        } else {
            "not configured".to_owned()
        });
    }
    if let Some(linked) = account.linked {
        bits.push(if linked {
            "linked".to_owned()
        } else {
            "not linked".to_owned()
        });
    }
    if let Some(running) = account.running {
        bits.push(if running {
            "running".to_owned()
        } else {
            "stopped".to_owned()
        });
    }
    if let Some(connected) = account.connected {
        bits.push(if connected {
            "connected".to_owned()
        } else {
            "disconnected".to_owned()
        });
    }
    if let Some(inbound_at) = account.last_inbound_at {
        if inbound_at.is_finite() && inbound_at > 0.0 {
            let now = std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap_or_default()
                .as_millis() as f64;
            bits.push(format!("in:{}", format_time_ago(now - inbound_at)));
        }
    }
    if let Some(outbound_at) = account.last_outbound_at {
        if outbound_at.is_finite() && outbound_at > 0.0 {
            let now = std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap_or_default()
                .as_millis() as f64;
            bits.push(format!("out:{}", format_time_ago(now - outbound_at)));
        }
    }
    if let Some(ref mode) = account.mode {
        if !mode.is_empty() {
            bits.push(format!("mode:{mode}"));
        }
    }
    if let Some(ref dm_policy) = account.dm_policy {
        if !dm_policy.is_empty() {
            bits.push(format!("dm:{dm_policy}"));
        }
    }
    if let Some(ref ts) = account.token_source {
        if !ts.is_empty() {
            bits.push(format!("token:{ts}"));
        }
    }
    if let Some(ref bts) = account.bot_token_source {
        if !bts.is_empty() {
            bits.push(format!("bot:{bts}"));
        }
    }
    if let Some(ref ats) = account.app_token_source {
        if !ats.is_empty() {
            bits.push(format!("app:{ats}"));
        }
    }
    if let Some(ref url) = account.base_url {
        if !url.is_empty() {
            bits.push(format!("url:{url}"));
        }
    }
    if let Some(ref err) = account.last_error {
        if !err.is_empty() {
            bits.push(format!("error:{err}"));
        }
    }

    bits
}

/// Format gateway channel status response lines.
///
/// Source: `src/commands/channels/status.ts` — `formatGatewayChannelsStatusLines`
pub fn format_gateway_channels_status_lines(
    payload: &HashMap<String, serde_json::Value>,
) -> Vec<String> {
    let mut lines = Vec::new();
    lines.push(Theme::success("Gateway reachable."));

    let channel_accounts = payload
        .get("channelAccounts")
        .and_then(|v| v.as_object());

    for &channel_id in CHAT_CHANNEL_ORDER {
        let channel_key = channel_id.as_str();
        let accounts_raw = channel_accounts.and_then(|ca| ca.get(channel_key));

        if let Some(arr) = accounts_raw.and_then(|v| v.as_array()) {
            for account_value in arr {
                let account: ChannelAccountStatus =
                    serde_json::from_value(account_value.clone()).unwrap_or_else(|_| {
                        ChannelAccountStatus {
                            account_id: Some("default".to_owned()),
                            name: None,
                            enabled: None,
                            configured: None,
                            linked: None,
                            running: None,
                            connected: None,
                            mode: None,
                            token_source: None,
                            bot_token_source: None,
                            app_token_source: None,
                            base_url: None,
                            last_inbound_at: None,
                            last_outbound_at: None,
                            last_error: None,
                            dm_policy: None,
                        }
                    });

                let account_id = account
                    .account_id
                    .as_deref()
                    .unwrap_or("default");
                let name = account.name.as_deref().and_then(|n| {
                    let trimmed = n.trim();
                    if trimmed.is_empty() { None } else { Some(trimmed) }
                });

                let label_text = format_channel_account_label(
                    channel_id,
                    account_id,
                    name,
                    None,
                    None,
                );

                let bits = format_account_status_bits(&account);
                lines.push(format!("- {label_text}: {}", bits.join(", ")));
            }
        }
    }

    lines.push(String::new());
    lines.push(format!(
        "Tip: {} adds gateway health probes to status output (requires a reachable gateway).",
        format_docs_link("/cli#status", Some("status --deep"))
    ));

    lines
}

/// Format config-only channel status lines (when gateway is unreachable).
///
/// Source: `src/commands/channels/status.ts` — `formatConfigChannelsStatusLines`
fn format_config_channels_status_lines(
    path: Option<&str>,
    mode: Option<&str>,
) -> Vec<String> {
    let mut lines = Vec::new();
    lines.push(Theme::warn(
        "Gateway not reachable; showing config-only status.",
    ));
    if let Some(p) = path {
        lines.push(format!("Config: {p}"));
    }
    if let Some(m) = mode {
        lines.push(format!("Mode: {m}"));
    }
    if path.is_some() || mode.is_some() {
        lines.push(String::new());
    }
    lines.push(String::new());
    lines.push(format!(
        "Tip: {} adds gateway health probes to status output (requires a reachable gateway).",
        format_docs_link("/cli#status", Some("status --deep"))
    ));
    lines
}

/// Execute the `channels status` command.
///
/// In the full implementation this would call the gateway RPC endpoint.
/// Currently falls back to config-only display since the gateway client
/// is not yet wired up in Rust.
///
/// Source: `src/commands/channels/status.ts` — `channelsStatusCommand`
pub async fn channels_status_command(opts: &ChannelsStatusOptions) -> Result<()> {
    let _timeout_ms: u64 = opts
        .timeout
        .as_deref()
        .unwrap_or("10000")
        .parse()
        .unwrap_or(10_000);

    // Attempt gateway call (stubbed out -- falls back to config-only)
    let cfg = crate::shared::require_valid_config().await?;

    if opts.json {
        let payload = serde_json::json!({
            "error": "Gateway not reachable (stub)",
            "config_only": true,
        });
        println!("{}", serde_json::to_string_pretty(&payload)?);
        return Ok(());
    }

    eprintln!(
        "Gateway not reachable: {}",
        "gateway client not yet connected (Rust stub)"
    );

    if cfg.is_none() {
        return Ok(());
    }

    let mode = "local";
    let lines = format_config_channels_status_lines(None, Some(mode));
    println!("{}", lines.join("\n"));

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn format_time_ago_seconds() {
        assert_eq!(format_time_ago(5000.0), "5s ago");
        assert_eq!(format_time_ago(30_000.0), "30s ago");
    }

    #[test]
    fn format_time_ago_minutes() {
        assert_eq!(format_time_ago(120_000.0), "2m ago");
        assert_eq!(format_time_ago(3_300_000.0), "55m ago");
    }

    #[test]
    fn format_time_ago_hours() {
        assert_eq!(format_time_ago(7_200_000.0), "2h ago");
    }

    #[test]
    fn format_time_ago_days() {
        assert_eq!(format_time_ago(172_800_000.0), "2d ago");
    }

    #[test]
    fn format_time_ago_negative() {
        assert_eq!(format_time_ago(-100.0), "just now");
    }

    #[test]
    fn format_account_status_bits_full() {
        let account = ChannelAccountStatus {
            account_id: Some("default".to_owned()),
            name: None,
            enabled: Some(true),
            configured: Some(true),
            linked: Some(true),
            running: Some(true),
            connected: Some(true),
            mode: Some("polling".to_owned()),
            token_source: Some("env".to_owned()),
            bot_token_source: None,
            app_token_source: None,
            base_url: None,
            last_inbound_at: None,
            last_outbound_at: None,
            last_error: None,
            dm_policy: Some("allow".to_owned()),
        };
        let bits = format_account_status_bits(&account);
        assert!(bits.contains(&"enabled".to_owned()));
        assert!(bits.contains(&"configured".to_owned()));
        assert!(bits.contains(&"linked".to_owned()));
        assert!(bits.contains(&"running".to_owned()));
        assert!(bits.contains(&"connected".to_owned()));
        assert!(bits.contains(&"mode:polling".to_owned()));
        assert!(bits.contains(&"token:env".to_owned()));
        assert!(bits.contains(&"dm:allow".to_owned()));
    }

    #[test]
    fn format_account_status_bits_disabled() {
        let account = ChannelAccountStatus {
            account_id: Some("test".to_owned()),
            name: None,
            enabled: Some(false),
            configured: Some(false),
            linked: None,
            running: None,
            connected: None,
            mode: None,
            token_source: None,
            bot_token_source: None,
            app_token_source: None,
            base_url: None,
            last_inbound_at: None,
            last_outbound_at: None,
            last_error: Some("auth failed".to_owned()),
            dm_policy: None,
        };
        let bits = format_account_status_bits(&account);
        assert!(bits.contains(&"disabled".to_owned()));
        assert!(bits.contains(&"not configured".to_owned()));
        assert!(bits.contains(&"error:auth failed".to_owned()));
    }

    #[test]
    fn format_gateway_lines_has_header() {
        let payload: HashMap<String, serde_json::Value> = HashMap::new();
        let lines = format_gateway_channels_status_lines(&payload);
        assert!(!lines.is_empty());
        assert!(lines[0].contains("Gateway reachable"));
    }

    #[test]
    fn format_gateway_lines_with_accounts() {
        let mut payload = HashMap::new();
        let accounts = serde_json::json!({
            "discord": [{
                "accountId": "default",
                "enabled": true,
                "running": true
            }]
        });
        payload.insert(
            "channelAccounts".to_owned(),
            accounts,
        );
        let lines = format_gateway_channels_status_lines(&payload);
        let has_discord_line = lines.iter().any(|l| l.contains("Discord"));
        assert!(has_discord_line);
    }

    #[test]
    fn format_config_lines_has_warning() {
        let lines = format_config_channels_status_lines(Some("/tmp/config.json5"), Some("local"));
        assert!(lines[0].contains("Gateway not reachable"));
        assert!(lines.iter().any(|l| l.contains("Config: /tmp/config.json5")));
        assert!(lines.iter().any(|l| l.contains("Mode: local")));
    }
}
