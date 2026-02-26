/// Channel configuration removal wizard.
///
/// Provides interactive removal of channel configurations from the config file.
/// Only the config entries are removed; on-disk credentials/sessions are left intact.
///
/// Source: `src/commands/configure.channels.ts`

use oa_types::config::OpenAcosmiConfig;

/// Metadata for a configured channel, used to display removal options.
///
/// Source: `src/commands/configure.channels.ts` (channel metadata from plugins)
#[derive(Debug, Clone)]
pub struct ChannelInfo {
    /// The channel identifier (e.g., "whatsapp", "telegram").
    pub id: String,
    /// Human-readable label for the channel.
    pub label: String,
}

/// Remove a specific channel's configuration from the config.
///
/// Returns the updated config with the channel removed from `channels`.
/// If no channels remain after removal, the `channels` field is set to `None`.
///
/// Source: `src/commands/configure.channels.ts` - `removeChannelConfigWizard`
pub fn remove_channel_config(cfg: &OpenAcosmiConfig, channel_id: &str) -> OpenAcosmiConfig {
    let mut next = cfg.clone();

    if let Some(ref channels) = next.channels {
        let mut channels_value = serde_json::to_value(channels).unwrap_or_default();
        if let Some(obj) = channels_value.as_object_mut() {
            obj.remove(channel_id);
            if obj.is_empty() {
                next.channels = None;
            } else {
                next.channels = serde_json::from_value(channels_value).ok();
            }
        }
    }

    next
}

/// List the channel IDs currently present in the config's `channels` map.
///
/// Source: `src/commands/configure.channels.ts` - `listConfiguredChannels`
pub fn list_configured_channel_ids(cfg: &OpenAcosmiConfig) -> Vec<String> {
    let Some(ref channels) = cfg.channels else {
        return Vec::new();
    };

    // Serialize the channels config to JSON to enumerate dynamic keys
    let value = serde_json::to_value(channels).unwrap_or_default();
    let Some(obj) = value.as_object() else {
        return Vec::new();
    };

    obj.keys().cloned().collect()
}

/// Check if a given channel ID is configured in the config.
///
/// Source: `src/commands/configure.channels.ts`
pub fn is_channel_configured(cfg: &OpenAcosmiConfig, channel_id: &str) -> bool {
    list_configured_channel_ids(cfg).iter().any(|id| id == channel_id)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn list_configured_channels_empty_config() {
        let cfg = OpenAcosmiConfig::default();
        let ids = list_configured_channel_ids(&cfg);
        assert!(ids.is_empty());
    }

    #[test]
    fn is_channel_configured_returns_false_for_empty_config() {
        let cfg = OpenAcosmiConfig::default();
        assert!(!is_channel_configured(&cfg, "whatsapp"));
    }

    #[test]
    fn remove_channel_from_empty_config() {
        let cfg = OpenAcosmiConfig::default();
        let result = remove_channel_config(&cfg, "telegram");
        // Should not panic; channels should remain None
        assert!(result.channels.is_none());
    }

    #[test]
    fn channel_info_debug() {
        let info = ChannelInfo {
            id: "whatsapp".to_string(),
            label: "WhatsApp".to_string(),
        };
        let debug = format!("{info:?}");
        assert!(debug.contains("whatsapp"));
        assert!(debug.contains("WhatsApp"));
    }
}
