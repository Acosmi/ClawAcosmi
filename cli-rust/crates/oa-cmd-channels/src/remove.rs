/// `channels remove` command: disables or deletes a channel account.
///
/// By default, disables the account (sets `enabled: false`). With `--delete`,
/// removes the account entry entirely from the config.
///
/// Source: `src/commands/channels/remove.ts`

use anyhow::{bail, Result};

use oa_channels::registry::{ChatChannelId, normalize_chat_channel_id};
use oa_config::io::write_config_file;
use oa_routing::session_key::{normalize_account_id, DEFAULT_ACCOUNT_ID};
use oa_types::channels::ChannelsConfig;
use oa_types::config::OpenAcosmiConfig;

use crate::shared::{channel_label, require_valid_config};

/// Options for the `channels remove` subcommand.
///
/// Source: `src/commands/channels/remove.ts` — `ChannelsRemoveOptions`
#[derive(Debug, Clone, Default)]
pub struct ChannelsRemoveOptions {
    /// Channel identifier.
    pub channel: Option<String>,
    /// Account identifier.
    pub account: Option<String>,
    /// Whether to fully delete the account (vs. just disabling).
    pub delete: bool,
}

/// Set the `enabled` flag on a channel account.
///
/// Source: `src/commands/channels/remove.ts` — `plugin.config.setAccountEnabled`
fn set_account_enabled(
    mut cfg: OpenAcosmiConfig,
    channel: ChatChannelId,
    account_id: &str,
    enabled: bool,
) -> OpenAcosmiConfig {
    let channels = cfg.channels.get_or_insert_with(ChannelsConfig::default);
    let channel_raw = match channel {
        ChatChannelId::Telegram => &mut channels.telegram,
        ChatChannelId::WhatsApp => &mut channels.whatsapp,
        ChatChannelId::Discord => &mut channels.discord,
        ChatChannelId::GoogleChat => &mut channels.googlechat,
        ChatChannelId::Slack => &mut channels.slack,
        ChatChannelId::Signal => &mut channels.signal,
        ChatChannelId::IMessage => &mut channels.imessage,
    };

    if let Some(val) = channel_raw {
        if let Some(obj) = val.as_object_mut() {
            // Multi-account form
            if let Some(accounts) = obj.get_mut("accounts") {
                if let Some(accounts_obj) = accounts.as_object_mut() {
                    if let Some(account_val) = accounts_obj.get_mut(account_id) {
                        if let Some(account_obj) = account_val.as_object_mut() {
                            account_obj.insert(
                                "enabled".to_owned(),
                                serde_json::Value::Bool(enabled),
                            );
                        }
                    }
                }
            } else if account_id == "default" || account_id == DEFAULT_ACCOUNT_ID {
                // Single-account form
                obj.insert("enabled".to_owned(), serde_json::Value::Bool(enabled));
            }
        }
    }

    cfg
}

/// Delete an account entry from the channel config entirely.
///
/// Source: `src/commands/channels/remove.ts` — `plugin.config.deleteAccount`
fn delete_account(
    mut cfg: OpenAcosmiConfig,
    channel: ChatChannelId,
    account_id: &str,
) -> OpenAcosmiConfig {
    let channels = cfg.channels.get_or_insert_with(ChannelsConfig::default);
    let channel_raw = match channel {
        ChatChannelId::Telegram => &mut channels.telegram,
        ChatChannelId::WhatsApp => &mut channels.whatsapp,
        ChatChannelId::Discord => &mut channels.discord,
        ChatChannelId::GoogleChat => &mut channels.googlechat,
        ChatChannelId::Slack => &mut channels.slack,
        ChatChannelId::Signal => &mut channels.signal,
        ChatChannelId::IMessage => &mut channels.imessage,
    };

    if let Some(val) = channel_raw {
        if let Some(obj) = val.as_object_mut() {
            if let Some(accounts) = obj.get_mut("accounts") {
                if let Some(accounts_obj) = accounts.as_object_mut() {
                    accounts_obj.remove(account_id);
                    // If no accounts remain, remove the accounts key
                    if accounts_obj.is_empty() {
                        obj.remove("accounts");
                    }
                }
            } else if account_id == "default" || account_id == DEFAULT_ACCOUNT_ID {
                // Single-account form: remove the whole channel config
                *channel_raw = None;
            }
        }
    }

    cfg
}

/// Execute the `channels remove` command.
///
/// Source: `src/commands/channels/remove.ts` — `channelsRemoveCommand`
pub async fn channels_remove_command(opts: &ChannelsRemoveOptions) -> Result<()> {
    let cfg = match require_valid_config().await? {
        Some(c) => c,
        None => return Ok(()),
    };

    let channel = match &opts.channel {
        Some(raw) => match normalize_chat_channel_id(raw) {
            Some(c) => c,
            None => bail!("Unknown channel: {raw}"),
        },
        None => bail!("Channel is required. Use --channel <name>."),
    };

    let account_id = normalize_account_id(opts.account.as_deref());
    let account_key = if account_id.is_empty() {
        DEFAULT_ACCOUNT_ID.to_owned()
    } else {
        account_id.clone()
    };

    let next = if opts.delete {
        delete_account(cfg, channel, &account_id)
    } else {
        set_account_enabled(cfg, channel, &account_id, false)
    };

    write_config_file(&next).await?;

    if opts.delete {
        println!(
            "Deleted {} account \"{account_key}\".",
            channel_label(channel)
        );
    } else {
        println!(
            "Disabled {} account \"{account_key}\".",
            channel_label(channel)
        );
    }

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
                    "token": "test-token",
                    "enabled": true
                },
                "secondary": {
                    "token": "test-token-2",
                    "enabled": true
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
    fn set_account_enabled_multi_account() {
        let cfg = make_discord_config();
        let result = set_account_enabled(cfg, ChatChannelId::Discord, "default", false);
        let discord = result
            .channels
            .as_ref()
            .and_then(|c| c.discord.as_ref())
            .expect("discord config");
        let acct = discord
            .get("accounts")
            .and_then(|a| a.get("default"))
            .expect("default account");
        assert_eq!(acct.get("enabled").and_then(|v| v.as_bool()), Some(false));
    }

    #[test]
    fn set_account_enabled_single_account() {
        let cfg = make_single_account_config();
        let result = set_account_enabled(cfg, ChatChannelId::Telegram, "default", false);
        let tg = result
            .channels
            .as_ref()
            .and_then(|c| c.telegram.as_ref())
            .expect("telegram config");
        assert_eq!(tg.get("enabled").and_then(|v| v.as_bool()), Some(false));
    }

    #[test]
    fn delete_account_multi_account() {
        let cfg = make_discord_config();
        let result = delete_account(cfg, ChatChannelId::Discord, "secondary");
        let discord = result
            .channels
            .as_ref()
            .and_then(|c| c.discord.as_ref())
            .expect("discord config");
        let accounts = discord.get("accounts").and_then(|a| a.as_object());
        assert!(accounts.is_some());
        let accounts = accounts.expect("accounts");
        assert!(accounts.get("secondary").is_none());
        assert!(accounts.get("default").is_some());
    }

    #[test]
    fn delete_account_single_account_removes_channel() {
        let cfg = make_single_account_config();
        let result = delete_account(cfg, ChatChannelId::Telegram, "default");
        assert!(result.channels.as_ref().and_then(|c| c.telegram.as_ref()).is_none());
    }

    #[test]
    fn delete_account_last_in_multi_removes_accounts_key() {
        let discord = serde_json::json!({
            "accounts": {
                "only": {
                    "token": "test",
                    "enabled": true
                }
            }
        });
        let cfg = OpenAcosmiConfig {
            channels: Some(ChannelsConfig {
                discord: Some(discord),
                ..Default::default()
            }),
            ..Default::default()
        };
        let result = delete_account(cfg, ChatChannelId::Discord, "only");
        let discord = result
            .channels
            .as_ref()
            .and_then(|c| c.discord.as_ref())
            .expect("discord config should still exist");
        assert!(discord.get("accounts").is_none());
    }
}
