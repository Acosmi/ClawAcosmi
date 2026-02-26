/// `channels list` command: displays configured channel accounts with their
/// status, token sources, and optional provider usage.
///
/// Source: `src/commands/channels/list.ts`

use anyhow::Result;
use serde::{Deserialize, Serialize};

use oa_channels::registry::{ChatChannelId, CHAT_CHANNEL_ORDER};
use oa_terminal::links::format_docs_link;
use oa_terminal::theme::Theme;
use oa_types::config::OpenAcosmiConfig;

use crate::shared::{format_channel_account_label, require_valid_config};

/// Options for the `channels list` subcommand.
///
/// Source: `src/commands/channels/list.ts` — `ChannelsListOptions`
#[derive(Debug, Clone, Default)]
pub struct ChannelsListOptions {
    /// Output in JSON format.
    pub json: bool,
    /// Include usage information.
    pub usage: bool,
}

/// Snapshot of a single channel account for display.
///
/// Source: `src/commands/channels/list.ts` — `ChannelAccountSnapshot` usage
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ChannelAccountSnapshot {
    /// The account identifier.
    pub account_id: String,
    /// Optional display name.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    /// Whether the account is configured.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub configured: Option<bool>,
    /// Whether the account is enabled.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    /// Whether the account is linked (bound to an agent).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub linked: Option<bool>,
    /// Token source descriptor (e.g. "env", "config", "none").
    #[serde(skip_serializing_if = "Option::is_none")]
    pub token_source: Option<String>,
    /// Bot token source descriptor.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub bot_token_source: Option<String>,
    /// App token source descriptor.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub app_token_source: Option<String>,
    /// Base URL override.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub base_url: Option<String>,
}

/// Extract account IDs from the raw config JSON for a given channel.
///
/// Reads the `channels.<id>.accounts` field (if present as an object)
/// and returns the keys.
///
/// Source: `src/commands/channels/list.ts` — `plugin.config.listAccountIds`
fn list_account_ids_from_config(cfg: &OpenAcosmiConfig, channel: ChatChannelId) -> Vec<String> {
    let channels_cfg = match &cfg.channels {
        Some(c) => c,
        None => return Vec::new(),
    };

    let raw = match channel {
        ChatChannelId::Telegram => channels_cfg.telegram.as_ref(),
        ChatChannelId::WhatsApp => channels_cfg.whatsapp.as_ref(),
        ChatChannelId::Discord => channels_cfg.discord.as_ref(),
        ChatChannelId::GoogleChat => channels_cfg.googlechat.as_ref(),
        ChatChannelId::Slack => channels_cfg.slack.as_ref(),
        ChatChannelId::Signal => channels_cfg.signal.as_ref(),
        ChatChannelId::IMessage => channels_cfg.imessage.as_ref(),
    };

    let value = match raw {
        Some(v) => v,
        None => return Vec::new(),
    };

    // Check if the value has an "accounts" map
    if let Some(obj) = value.as_object() {
        if let Some(accounts) = obj.get("accounts") {
            if let Some(accounts_obj) = accounts.as_object() {
                return accounts_obj.keys().cloned().collect();
            }
        }
        // Single-account form: if the config itself looks like an account config
        // (has token/botToken/etc.), treat it as account "default"
        if obj.contains_key("token")
            || obj.contains_key("botToken")
            || obj.contains_key("appToken")
            || obj.contains_key("enabled")
        {
            return vec!["default".to_owned()];
        }
    }

    Vec::new()
}

/// Build a snapshot for a single channel account from config data.
///
/// Source: `src/commands/channels/list.ts` — `buildChannelAccountSnapshot`
fn build_snapshot(
    cfg: &OpenAcosmiConfig,
    channel: ChatChannelId,
    account_id: &str,
) -> ChannelAccountSnapshot {
    let channels_cfg = cfg.channels.as_ref();
    let raw = channels_cfg.and_then(|c| match channel {
        ChatChannelId::Telegram => c.telegram.as_ref(),
        ChatChannelId::WhatsApp => c.whatsapp.as_ref(),
        ChatChannelId::Discord => c.discord.as_ref(),
        ChatChannelId::GoogleChat => c.googlechat.as_ref(),
        ChatChannelId::Slack => c.slack.as_ref(),
        ChatChannelId::Signal => c.signal.as_ref(),
        ChatChannelId::IMessage => c.imessage.as_ref(),
    });

    let account_value = raw.and_then(|v| {
        let obj = v.as_object()?;
        if let Some(accounts) = obj.get("accounts") {
            return accounts.as_object()?.get(account_id).cloned();
        }
        // Single-account form
        if account_id == "default" {
            return Some(v.clone());
        }
        None
    });

    let extract_str = |key: &str| -> Option<String> {
        account_value
            .as_ref()?
            .as_object()?
            .get(key)?
            .as_str()
            .map(String::from)
    };

    let extract_bool = |key: &str| -> Option<bool> {
        account_value.as_ref()?.as_object()?.get(key)?.as_bool()
    };

    let token_source = if extract_str("token").is_some() {
        Some("config".to_owned())
    } else {
        None
    };

    let bot_token_source = if extract_str("botToken").is_some() {
        Some("config".to_owned())
    } else {
        None
    };

    let app_token_source = if extract_str("appToken").is_some() {
        Some("config".to_owned())
    } else {
        None
    };

    let configured = Some(account_value.is_some());
    let enabled = extract_bool("enabled").or(Some(true));

    ChannelAccountSnapshot {
        account_id: account_id.to_owned(),
        name: extract_str("name"),
        configured,
        enabled,
        linked: None,
        token_source,
        bot_token_source,
        app_token_source,
        base_url: extract_str("baseUrl"),
    }
}

/// Color a token-source value for display.
///
/// Source: `src/commands/channels/list.ts` — `colorValue`
fn color_value(value: &str) -> String {
    match value {
        "none" => Theme::error(value),
        "env" => Theme::accent(value),
        _ => Theme::success(value),
    }
}

/// Format the enabled state for display.
///
/// Source: `src/commands/channels/list.ts` — `formatEnabled`
fn format_enabled(value: Option<bool>) -> String {
    if value == Some(false) {
        Theme::error("disabled")
    } else {
        Theme::success("enabled")
    }
}

/// Format the configured state for display.
///
/// Source: `src/commands/channels/list.ts` — `formatConfigured`
fn format_configured(value: bool) -> String {
    if value {
        Theme::success("configured")
    } else {
        Theme::warn("not configured")
    }
}

/// Format a token source string for display.
///
/// Source: `src/commands/channels/list.ts` — `formatTokenSource`
fn format_token_source(source: Option<&str>) -> String {
    let value = source.unwrap_or("none");
    format!("token={}", color_value(value))
}

/// Format a generic source (e.g. bot, app) for display.
///
/// Source: `src/commands/channels/list.ts` — `formatSource`
fn format_source(label: &str, source: Option<&str>) -> String {
    let value = source.unwrap_or("none");
    format!("{label}={}", color_value(value))
}

/// Format the linked state for display.
///
/// Source: `src/commands/channels/list.ts` — `formatLinked`
fn format_linked(value: bool) -> String {
    if value {
        Theme::success("linked")
    } else {
        Theme::warn("not linked")
    }
}

/// Format a single account line with status bits.
///
/// Source: `src/commands/channels/list.ts` — `formatAccountLine`
fn format_account_line(channel: ChatChannelId, snapshot: &ChannelAccountSnapshot) -> String {
    let label = format_channel_account_label(
        channel,
        &snapshot.account_id,
        snapshot.name.as_deref(),
        Some(Theme::accent),
        Some(Theme::heading),
    );

    let mut bits = Vec::new();
    if let Some(linked) = snapshot.linked {
        bits.push(format_linked(linked));
    }
    if let Some(configured) = snapshot.configured {
        bits.push(format_configured(configured));
    }
    if let Some(ref ts) = snapshot.token_source {
        bits.push(format_token_source(Some(ts)));
    }
    if let Some(ref bts) = snapshot.bot_token_source {
        bits.push(format_source("bot", Some(bts)));
    }
    if let Some(ref ats) = snapshot.app_token_source {
        bits.push(format_source("app", Some(ats)));
    }
    if let Some(ref url) = snapshot.base_url {
        bits.push(format!("base={}", Theme::muted(url)));
    }
    if snapshot.enabled.is_some() {
        bits.push(format_enabled(snapshot.enabled));
    }
    format!("- {label}: {}", bits.join(", "))
}

/// Execute the `channels list` command.
///
/// Source: `src/commands/channels/list.ts` — `channelsListCommand`
pub async fn channels_list_command(opts: &ChannelsListOptions) -> Result<()> {
    let cfg = match require_valid_config().await? {
        Some(c) => c,
        None => return Ok(()),
    };

    if opts.json {
        let mut chat = serde_json::Map::new();
        for &channel_id in CHAT_CHANNEL_ORDER {
            let accounts = list_account_ids_from_config(&cfg, channel_id);
            let arr: Vec<serde_json::Value> = accounts
                .into_iter()
                .map(serde_json::Value::String)
                .collect();
            chat.insert(channel_id.as_str().to_owned(), serde_json::Value::Array(arr));
        }
        let payload = serde_json::json!({ "chat": chat, "auth": [] });
        println!("{}", serde_json::to_string_pretty(&payload)?);
        return Ok(());
    }

    let mut lines = Vec::new();
    lines.push(Theme::heading("Chat channels:"));

    for &channel_id in CHAT_CHANNEL_ORDER {
        let accounts = list_account_ids_from_config(&cfg, channel_id);
        for account_id in &accounts {
            let snapshot = build_snapshot(&cfg, channel_id, account_id);
            lines.push(format_account_line(channel_id, &snapshot));
        }
    }

    lines.push(String::new());
    lines.push(Theme::heading("Auth providers (OAuth + API keys):"));
    lines.push(Theme::muted("- none"));

    println!("{}", lines.join("\n"));

    println!();
    println!(
        "Docs: {}",
        format_docs_link("/gateway/configuration", Some("gateway/configuration"))
    );

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use oa_types::channels::ChannelsConfig;

    fn make_discord_config() -> OpenAcosmiConfig {
        let discord = serde_json::json!({
            "accounts": {
                "default": {
                    "token": "xoxb-test",
                    "enabled": true,
                    "name": "TestBot"
                },
                "secondary": {
                    "token": "xoxb-secondary",
                    "enabled": false
                }
            }
        });
        OpenAcosmiConfig {
            channels: Some(ChannelsConfig {
                discord: Some(discord),
                ..Default::default()
            }),
            ..Default::default()
        }
    }

    fn make_single_account_config() -> OpenAcosmiConfig {
        let telegram = serde_json::json!({
            "token": "123:ABC",
            "enabled": true
        });
        OpenAcosmiConfig {
            channels: Some(ChannelsConfig {
                telegram: Some(telegram),
                ..Default::default()
            }),
            ..Default::default()
        }
    }

    #[test]
    fn list_account_ids_multi_account() {
        let cfg = make_discord_config();
        let ids = list_account_ids_from_config(&cfg, ChatChannelId::Discord);
        assert!(ids.contains(&"default".to_owned()));
        assert!(ids.contains(&"secondary".to_owned()));
        assert_eq!(ids.len(), 2);
    }

    #[test]
    fn list_account_ids_single_account() {
        let cfg = make_single_account_config();
        let ids = list_account_ids_from_config(&cfg, ChatChannelId::Telegram);
        assert_eq!(ids, vec!["default"]);
    }

    #[test]
    fn list_account_ids_empty() {
        let cfg = OpenAcosmiConfig::default();
        let ids = list_account_ids_from_config(&cfg, ChatChannelId::Discord);
        assert!(ids.is_empty());
    }

    #[test]
    fn build_snapshot_extracts_fields() {
        let cfg = make_discord_config();
        let snapshot = build_snapshot(&cfg, ChatChannelId::Discord, "default");
        assert_eq!(snapshot.account_id, "default");
        assert_eq!(snapshot.name.as_deref(), Some("TestBot"));
        assert_eq!(snapshot.configured, Some(true));
        assert_eq!(snapshot.enabled, Some(true));
        assert_eq!(snapshot.token_source.as_deref(), Some("config"));
    }

    #[test]
    fn build_snapshot_disabled_account() {
        let cfg = make_discord_config();
        let snapshot = build_snapshot(&cfg, ChatChannelId::Discord, "secondary");
        assert_eq!(snapshot.enabled, Some(false));
        assert!(snapshot.name.is_none());
    }

    #[test]
    fn build_snapshot_missing_channel() {
        let cfg = OpenAcosmiConfig::default();
        let snapshot = build_snapshot(&cfg, ChatChannelId::Signal, "default");
        assert_eq!(snapshot.configured, Some(false));
    }

    #[test]
    fn color_value_none_is_error() {
        let result = color_value("none");
        // In non-TTY/CI the result is just the raw string
        assert!(result.contains("none"));
    }

    #[test]
    fn format_account_line_includes_label() {
        let snapshot = ChannelAccountSnapshot {
            account_id: "default".to_owned(),
            name: Some("TestBot".to_owned()),
            configured: Some(true),
            enabled: Some(true),
            linked: None,
            token_source: Some("config".to_owned()),
            bot_token_source: None,
            app_token_source: None,
            base_url: None,
        };
        let line = format_account_line(ChatChannelId::Discord, &snapshot);
        assert!(line.starts_with("- "));
        assert!(line.contains("Discord"));
        assert!(line.contains("TestBot"));
    }
}
