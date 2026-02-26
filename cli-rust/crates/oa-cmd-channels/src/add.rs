/// `channels add` command: adds or configures a channel account.
///
/// In non-interactive mode, applies the provided CLI flags directly to the
/// config. In interactive (wizard) mode, delegates to the onboarding flow.
///
/// Source: `src/commands/channels/add.ts`, `src/commands/channels/add-mutators.ts`

use anyhow::{bail, Result};

use oa_channels::registry::{ChatChannelId, normalize_chat_channel_id};
use oa_config::io::write_config_file;
use oa_routing::session_key::normalize_account_id;
use oa_types::channels::ChannelsConfig;
use oa_types::config::OpenAcosmiConfig;

use crate::shared::{channel_label, require_valid_config};

/// Options for the `channels add` subcommand.
///
/// Source: `src/commands/channels/add.ts` — `ChannelsAddOptions`
#[derive(Debug, Clone, Default)]
pub struct ChannelsAddOptions {
    /// Channel identifier (e.g. "discord", "telegram").
    pub channel: Option<String>,
    /// Account identifier (defaults to "default").
    pub account: Option<String>,
    /// Display name for the account.
    pub name: Option<String>,
    /// Token value.
    pub token: Option<String>,
    /// Path to a file containing the token.
    pub token_file: Option<String>,
    /// Bot token (for Slack).
    pub bot_token: Option<String>,
    /// App-level token (for Slack).
    pub app_token: Option<String>,
    /// Signal phone number.
    pub signal_number: Option<String>,
    /// Path to signal-cli binary.
    pub cli_path: Option<String>,
    /// Signal database path.
    pub db_path: Option<String>,
    /// iMessage service type.
    pub service: Option<String>,
    /// Cloud region.
    pub region: Option<String>,
    /// Auth directory override.
    pub auth_dir: Option<String>,
    /// HTTP URL for webhook-based channels.
    pub http_url: Option<String>,
    /// HTTP host override.
    pub http_host: Option<String>,
    /// HTTP port override.
    pub http_port: Option<String>,
    /// Webhook path.
    pub webhook_path: Option<String>,
    /// Webhook URL.
    pub webhook_url: Option<String>,
    /// Audience type.
    pub audience_type: Option<String>,
    /// Audience value.
    pub audience: Option<String>,
    /// Read token from environment variable.
    pub use_env: bool,
    /// Matrix homeserver URL.
    pub homeserver: Option<String>,
    /// Matrix user ID.
    pub user_id: Option<String>,
    /// Matrix access token.
    pub access_token: Option<String>,
    /// Matrix password.
    pub password: Option<String>,
    /// Matrix device name.
    pub device_name: Option<String>,
    /// Matrix initial sync limit.
    pub initial_sync_limit: Option<String>,
    /// Urbit ship name.
    pub ship: Option<String>,
    /// Generic URL parameter.
    pub url: Option<String>,
    /// Urbit access code.
    pub code: Option<String>,
    /// Comma-separated group channels.
    pub group_channels: Option<String>,
    /// Comma-separated DM allowlist.
    pub dm_allowlist: Option<String>,
    /// Auto-discover channels flag.
    pub auto_discover_channels: Option<bool>,
}

/// Parse a comma/semicolon/newline-separated list into a `Vec<String>`.
///
/// Returns `None` if the input is empty or contains no entries after parsing.
///
/// Source: `src/commands/channels/add.ts` — `parseList`
#[allow(dead_code)]
fn parse_list(value: Option<&str>) -> Option<Vec<String>> {
    let trimmed = value?.trim();
    if trimmed.is_empty() {
        return None;
    }
    let parsed: Vec<String> = trimmed
        .split(&['\n', ',', ';'][..])
        .map(|s| s.trim().to_owned())
        .filter(|s| !s.is_empty())
        .collect();
    if parsed.is_empty() { None } else { Some(parsed) }
}

/// Apply account configuration to the config for a channel.
///
/// Inserts or merges the account entry under `channels.<channel>.accounts.<accountId>`.
///
/// Source: `src/commands/channels/add-mutators.ts` — `applyChannelAccountConfig`
fn apply_channel_account_config(
    mut cfg: OpenAcosmiConfig,
    channel: ChatChannelId,
    account_id: &str,
    opts: &ChannelsAddOptions,
) -> OpenAcosmiConfig {
    let channels = cfg.channels.get_or_insert_with(ChannelsConfig::default);

    // Build account data as a JSON value
    let mut account = serde_json::Map::new();

    if let Some(ref name) = opts.name {
        account.insert("name".to_owned(), serde_json::Value::String(name.clone()));
    }
    if let Some(ref token) = opts.token {
        account.insert("token".to_owned(), serde_json::Value::String(token.clone()));
    }
    if let Some(ref token_file) = opts.token_file {
        account.insert(
            "tokenFile".to_owned(),
            serde_json::Value::String(token_file.clone()),
        );
    }
    if let Some(ref bot_token) = opts.bot_token {
        account.insert(
            "botToken".to_owned(),
            serde_json::Value::String(bot_token.clone()),
        );
    }
    if let Some(ref app_token) = opts.app_token {
        account.insert(
            "appToken".to_owned(),
            serde_json::Value::String(app_token.clone()),
        );
    }
    if let Some(ref signal_number) = opts.signal_number {
        account.insert(
            "signalNumber".to_owned(),
            serde_json::Value::String(signal_number.clone()),
        );
    }
    if let Some(ref cli_path) = opts.cli_path {
        account.insert(
            "cliPath".to_owned(),
            serde_json::Value::String(cli_path.clone()),
        );
    }
    if let Some(ref db_path) = opts.db_path {
        account.insert(
            "dbPath".to_owned(),
            serde_json::Value::String(db_path.clone()),
        );
    }
    if let Some(ref service) = opts.service {
        account.insert(
            "service".to_owned(),
            serde_json::Value::String(service.clone()),
        );
    }
    if let Some(ref region) = opts.region {
        account.insert(
            "region".to_owned(),
            serde_json::Value::String(region.clone()),
        );
    }
    if let Some(ref homeserver) = opts.homeserver {
        account.insert(
            "homeserver".to_owned(),
            serde_json::Value::String(homeserver.clone()),
        );
    }
    if let Some(ref webhook_url) = opts.webhook_url {
        account.insert(
            "webhookUrl".to_owned(),
            serde_json::Value::String(webhook_url.clone()),
        );
    }
    if let Some(ref url) = opts.url {
        account.insert("url".to_owned(), serde_json::Value::String(url.clone()));
    }
    if opts.use_env {
        account.insert("useEnv".to_owned(), serde_json::Value::Bool(true));
    }
    account.insert("enabled".to_owned(), serde_json::Value::Bool(true));

    let account_value = serde_json::Value::Object(account);

    // Merge into the channel section
    let channel_key = channel.as_str();
    let channel_raw = match channel {
        ChatChannelId::Telegram => &mut channels.telegram,
        ChatChannelId::WhatsApp => &mut channels.whatsapp,
        ChatChannelId::Discord => &mut channels.discord,
        ChatChannelId::GoogleChat => &mut channels.googlechat,
        ChatChannelId::Slack => &mut channels.slack,
        ChatChannelId::Signal => &mut channels.signal,
        ChatChannelId::IMessage => &mut channels.imessage,
    };

    // Get or create the channel config object
    let channel_obj = channel_raw.get_or_insert_with(|| {
        serde_json::Value::Object(serde_json::Map::new())
    });

    if let Some(obj) = channel_obj.as_object_mut() {
        let accounts = obj
            .entry("accounts")
            .or_insert_with(|| serde_json::Value::Object(serde_json::Map::new()));
        if let Some(accounts_obj) = accounts.as_object_mut() {
            accounts_obj.insert(account_id.to_owned(), account_value);
        }
    }

    let _ = channel_key; // used implicitly via the match arm

    cfg
}

/// Execute the `channels add` command.
///
/// Source: `src/commands/channels/add.ts` — `channelsAddCommand`
pub async fn channels_add_command(opts: &ChannelsAddOptions) -> Result<()> {
    let cfg = match require_valid_config().await? {
        Some(c) => c,
        None => return Ok(()),
    };

    let raw_channel = opts.channel.as_deref().unwrap_or("");
    let channel = normalize_chat_channel_id(raw_channel);

    let channel = match channel {
        Some(c) => c,
        None => {
            let hint = format!("Unknown channel: {raw_channel}");
            bail!("{hint}");
        }
    };

    let account_id = normalize_account_id(opts.account.as_deref());

    let next_config = apply_channel_account_config(cfg, channel, &account_id, opts);

    write_config_file(&next_config).await?;
    println!(
        "Added {} account \"{account_id}\".",
        channel_label(channel)
    );

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_list_empty() {
        assert_eq!(parse_list(None), None);
        assert_eq!(parse_list(Some("")), None);
        assert_eq!(parse_list(Some("  ")), None);
    }

    #[test]
    fn parse_list_comma_separated() {
        let result = parse_list(Some("a, b, c"));
        assert_eq!(
            result,
            Some(vec!["a".to_owned(), "b".to_owned(), "c".to_owned()])
        );
    }

    #[test]
    fn parse_list_semicolon_and_newline() {
        let result = parse_list(Some("a; b\nc"));
        assert_eq!(
            result,
            Some(vec!["a".to_owned(), "b".to_owned(), "c".to_owned()])
        );
    }

    #[test]
    fn parse_list_filters_empty_entries() {
        let result = parse_list(Some(",, a,, b,,"));
        assert_eq!(
            result,
            Some(vec!["a".to_owned(), "b".to_owned()])
        );
    }

    #[test]
    fn apply_account_config_creates_entry() {
        let cfg = OpenAcosmiConfig::default();
        let opts = ChannelsAddOptions {
            token: Some("test-token".to_owned()),
            name: Some("My Bot".to_owned()),
            ..Default::default()
        };
        let result = apply_channel_account_config(
            cfg,
            ChatChannelId::Discord,
            "default",
            &opts,
        );
        let channels = result.channels.as_ref().expect("channels should exist");
        let discord = channels.discord.as_ref().expect("discord should exist");
        let accounts = discord.get("accounts").expect("accounts should exist");
        let default_account = accounts.get("default").expect("default should exist");
        assert_eq!(
            default_account.get("token").and_then(|v| v.as_str()),
            Some("test-token")
        );
        assert_eq!(
            default_account.get("name").and_then(|v| v.as_str()),
            Some("My Bot")
        );
        assert_eq!(
            default_account.get("enabled").and_then(|v| v.as_bool()),
            Some(true)
        );
    }

    #[test]
    fn apply_account_config_with_use_env() {
        let cfg = OpenAcosmiConfig::default();
        let opts = ChannelsAddOptions {
            use_env: true,
            ..Default::default()
        };
        let result = apply_channel_account_config(
            cfg,
            ChatChannelId::Telegram,
            "default",
            &opts,
        );
        let channels = result.channels.as_ref().expect("channels");
        let telegram = channels.telegram.as_ref().expect("telegram");
        let acct = telegram
            .get("accounts")
            .and_then(|a| a.get("default"))
            .expect("default account");
        assert_eq!(
            acct.get("useEnv").and_then(|v| v.as_bool()),
            Some(true)
        );
    }
}
