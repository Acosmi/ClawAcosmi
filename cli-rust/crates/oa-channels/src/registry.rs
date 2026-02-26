/// Channel plugin registry: IDs, metadata, aliases, and normalization.
///
/// Source: `src/channels/registry.ts`

use std::collections::HashMap;
use std::sync::LazyLock;

use serde::{Deserialize, Serialize};

/// Identifies a supported chat channel.
///
/// Source: `src/channels/registry.ts` — `CHAT_CHANNEL_ORDER`
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ChatChannelId {
    /// Telegram Bot API channel.
    Telegram,
    /// WhatsApp Web (QR link) channel.
    #[serde(rename = "whatsapp")]
    WhatsApp,
    /// Discord Bot API channel.
    Discord,
    /// Google Chat (Chat API) channel.
    #[serde(rename = "googlechat")]
    GoogleChat,
    /// Slack (Socket Mode) channel.
    Slack,
    /// Signal (signal-cli) channel.
    Signal,
    /// iMessage channel.
    #[serde(rename = "imessage")]
    IMessage,
}

impl ChatChannelId {
    /// Returns the lowercase string identifier for this channel.
    ///
    /// Source: `src/channels/registry.ts` — `CHAT_CHANNEL_ORDER`
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Telegram => "telegram",
            Self::WhatsApp => "whatsapp",
            Self::Discord => "discord",
            Self::GoogleChat => "googlechat",
            Self::Slack => "slack",
            Self::Signal => "signal",
            Self::IMessage => "imessage",
        }
    }
}

impl std::fmt::Display for ChatChannelId {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(self.as_str())
    }
}

/// Display order for all chat channels.
///
/// Source: `src/channels/registry.ts` — `CHAT_CHANNEL_ORDER`
pub const CHAT_CHANNEL_ORDER: &[ChatChannelId] = &[
    ChatChannelId::Telegram,
    ChatChannelId::WhatsApp,
    ChatChannelId::Discord,
    ChatChannelId::GoogleChat,
    ChatChannelId::Slack,
    ChatChannelId::Signal,
    ChatChannelId::IMessage,
];

/// All known channel IDs (same as `CHAT_CHANNEL_ORDER`).
///
/// Source: `src/channels/registry.ts` — `CHANNEL_IDS`
pub const CHANNEL_IDS: &[ChatChannelId] = CHAT_CHANNEL_ORDER;

/// The default chat channel when none is specified.
///
/// Source: `src/channels/registry.ts` — `DEFAULT_CHAT_CHANNEL`
pub const DEFAULT_CHAT_CHANNEL: ChatChannelId = ChatChannelId::Discord;

/// Website base URL used in selection labels.
///
/// Source: `src/channels/registry.ts` — `WEBSITE_URL`
#[allow(dead_code)]
const WEBSITE_URL: &str = "https://openacosmi.ai";

/// Metadata about a chat channel.
///
/// Source: `src/channels/registry.ts` — `ChatChannelMeta` / `ChannelMeta`
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ChatChannelMeta {
    /// The channel identifier.
    pub id: ChatChannelId,
    /// Human-readable label (e.g. "Telegram").
    pub label: &'static str,
    /// Selection-screen label (e.g. "Telegram (Bot API)").
    pub selection_label: &'static str,
    /// Detail label (e.g. "Telegram Bot").
    pub detail_label: &'static str,
    /// Documentation path (e.g. "/channels/telegram").
    pub docs_path: &'static str,
    /// Documentation label.
    pub docs_label: &'static str,
    /// Short description blurb.
    pub blurb: &'static str,
    /// SF Symbols system image name.
    pub system_image: &'static str,
}

/// Static metadata for all chat channels, indexed by display order.
///
/// Source: `src/channels/registry.ts` — `CHAT_CHANNEL_META`
static CHAT_CHANNEL_META: LazyLock<Vec<ChatChannelMeta>> = LazyLock::new(|| {
    vec![
        ChatChannelMeta {
            id: ChatChannelId::Telegram,
            label: "Telegram",
            selection_label: "Telegram (Bot API)",
            detail_label: "Telegram Bot",
            docs_path: "/channels/telegram",
            docs_label: "telegram",
            blurb: "simplest way to get started \u{2014} register a bot with @BotFather and get going.",
            system_image: "paperplane",
        },
        ChatChannelMeta {
            id: ChatChannelId::WhatsApp,
            label: "WhatsApp",
            selection_label: "WhatsApp (QR link)",
            detail_label: "WhatsApp Web",
            docs_path: "/channels/whatsapp",
            docs_label: "whatsapp",
            blurb: "works with your own number; recommend a separate phone + eSIM.",
            system_image: "message",
        },
        ChatChannelMeta {
            id: ChatChannelId::Discord,
            label: "Discord",
            selection_label: "Discord (Bot API)",
            detail_label: "Discord Bot",
            docs_path: "/channels/discord",
            docs_label: "discord",
            blurb: "very well supported right now.",
            system_image: "bubble.left.and.bubble.right",
        },
        ChatChannelMeta {
            id: ChatChannelId::GoogleChat,
            label: "Google Chat",
            selection_label: "Google Chat (Chat API)",
            detail_label: "Google Chat",
            docs_path: "/channels/googlechat",
            docs_label: "googlechat",
            blurb: "Google Workspace Chat app with HTTP webhook.",
            system_image: "message.badge",
        },
        ChatChannelMeta {
            id: ChatChannelId::Slack,
            label: "Slack",
            selection_label: "Slack (Socket Mode)",
            detail_label: "Slack Bot",
            docs_path: "/channels/slack",
            docs_label: "slack",
            blurb: "supported (Socket Mode).",
            system_image: "number",
        },
        ChatChannelMeta {
            id: ChatChannelId::Signal,
            label: "Signal",
            selection_label: "Signal (signal-cli)",
            detail_label: "Signal REST",
            docs_path: "/channels/signal",
            docs_label: "signal",
            blurb: "signal-cli linked device; more setup (David Reagans: \"Hop on Discord.\").",
            system_image: "antenna.radiowaves.left.and.right",
        },
        ChatChannelMeta {
            id: ChatChannelId::IMessage,
            label: "iMessage",
            selection_label: "iMessage (imsg)",
            detail_label: "iMessage",
            docs_path: "/channels/imessage",
            docs_label: "imessage",
            blurb: "this is still a work in progress.",
            system_image: "message.fill",
        },
    ]
});

/// Map from channel ID to its position in `CHAT_CHANNEL_META`.
static META_INDEX: LazyLock<HashMap<ChatChannelId, usize>> = LazyLock::new(|| {
    CHAT_CHANNEL_META
        .iter()
        .enumerate()
        .map(|(i, meta)| (meta.id, i))
        .collect()
});

/// Alias map for channel IDs (e.g. "tg" -> Telegram, "imsg" -> IMessage).
///
/// Source: `src/channels/registry.ts` — `CHAT_CHANNEL_ALIASES`
pub static CHAT_CHANNEL_ALIASES: LazyLock<HashMap<&'static str, ChatChannelId>> =
    LazyLock::new(|| {
        let mut m = HashMap::new();
        m.insert("tg", ChatChannelId::Telegram);
        m.insert("imsg", ChatChannelId::IMessage);
        m.insert("google-chat", ChatChannelId::GoogleChat);
        m.insert("gchat", ChatChannelId::GoogleChat);
        m
    });

/// Returns metadata for all chat channels in display order.
///
/// Source: `src/channels/registry.ts` — `listChatChannels()`
#[must_use]
pub fn list_chat_channels() -> &'static [ChatChannelMeta] {
    &CHAT_CHANNEL_META
}

/// Returns metadata for a specific chat channel.
///
/// Source: `src/channels/registry.ts` — `getChatChannelMeta()`
#[must_use]
pub fn get_chat_channel_meta(id: ChatChannelId) -> &'static ChatChannelMeta {
    let idx = META_INDEX
        .get(&id)
        .copied()
        .unwrap_or(0);
    &CHAT_CHANNEL_META[idx]
}

/// Normalizes a raw string key: trims whitespace and lowercases.
///
/// Source: `src/channels/registry.ts` — `normalizeChannelKey()`
fn normalize_channel_key(raw: &str) -> Option<String> {
    let normalized = raw.trim().to_lowercase();
    if normalized.is_empty() {
        None
    } else {
        Some(normalized)
    }
}

/// Attempts to parse a string into a `ChatChannelId`, applying alias resolution.
///
/// Case-insensitive. Recognizes both canonical IDs (e.g. "discord") and aliases
/// (e.g. "tg" for Telegram, "imsg" for iMessage).
///
/// Source: `src/channels/registry.ts` — `normalizeChatChannelId()`
#[must_use]
pub fn normalize_chat_channel_id(input: &str) -> Option<ChatChannelId> {
    let key = normalize_channel_key(input)?;

    // Check aliases first, then try direct match.
    let resolved = CHAT_CHANNEL_ALIASES
        .get(key.as_str())
        .copied()
        .map_or_else(|| key.clone(), |id| id.as_str().to_owned());

    CHAT_CHANNEL_ORDER
        .iter()
        .find(|id| id.as_str() == resolved)
        .copied()
}

/// Attempts to normalize any channel ID string, including aliases.
///
/// This is the main entry point for resolving user-provided channel names.
/// In the full system this would also consult a plugin registry; here we
/// delegate to `normalize_chat_channel_id` for built-in channels.
///
/// Source: `src/channels/registry.ts` — `normalizeAnyChannelId()`
#[must_use]
pub fn normalize_any_channel_id(input: &str) -> Option<ChatChannelId> {
    normalize_chat_channel_id(input)
}

/// Formats a one-line primer for a channel: "Label: blurb".
///
/// Source: `src/channels/registry.ts` — `formatChannelPrimerLine()`
#[must_use]
pub fn format_channel_primer_line(id: ChatChannelId) -> String {
    let meta = get_chat_channel_meta(id);
    format!("{}: {}", meta.label, meta.blurb)
}

/// Returns all known alias strings.
///
/// Source: `src/channels/registry.ts` — `listChatChannelAliases()`
#[must_use]
pub fn list_chat_channel_aliases() -> Vec<&'static str> {
    CHAT_CHANNEL_ALIASES.keys().copied().collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn channel_order_has_all_variants() {
        assert_eq!(CHAT_CHANNEL_ORDER.len(), 7);
        assert_eq!(CHAT_CHANNEL_ORDER[0], ChatChannelId::Telegram);
        assert_eq!(CHAT_CHANNEL_ORDER[6], ChatChannelId::IMessage);
    }

    #[test]
    fn list_channels_returns_all_in_order() {
        let channels = list_chat_channels();
        assert_eq!(channels.len(), 7);
        assert_eq!(channels[0].id, ChatChannelId::Telegram);
        assert_eq!(channels[2].id, ChatChannelId::Discord);
        assert_eq!(channels[6].id, ChatChannelId::IMessage);
    }

    #[test]
    fn get_meta_returns_correct_data() {
        let meta = get_chat_channel_meta(ChatChannelId::Discord);
        assert_eq!(meta.label, "Discord");
        assert_eq!(meta.docs_path, "/channels/discord");
    }

    #[test]
    fn normalize_canonical_ids() {
        assert_eq!(
            normalize_chat_channel_id("discord"),
            Some(ChatChannelId::Discord)
        );
        assert_eq!(
            normalize_chat_channel_id("telegram"),
            Some(ChatChannelId::Telegram)
        );
        assert_eq!(
            normalize_chat_channel_id("imessage"),
            Some(ChatChannelId::IMessage)
        );
    }

    #[test]
    fn normalize_case_insensitive() {
        assert_eq!(
            normalize_chat_channel_id("DISCORD"),
            Some(ChatChannelId::Discord)
        );
        assert_eq!(
            normalize_chat_channel_id("Telegram"),
            Some(ChatChannelId::Telegram)
        );
        assert_eq!(
            normalize_chat_channel_id("SlAcK"),
            Some(ChatChannelId::Slack)
        );
    }

    #[test]
    fn normalize_aliases() {
        assert_eq!(
            normalize_chat_channel_id("tg"),
            Some(ChatChannelId::Telegram)
        );
        assert_eq!(
            normalize_chat_channel_id("imsg"),
            Some(ChatChannelId::IMessage)
        );
        assert_eq!(
            normalize_chat_channel_id("gchat"),
            Some(ChatChannelId::GoogleChat)
        );
        assert_eq!(
            normalize_chat_channel_id("google-chat"),
            Some(ChatChannelId::GoogleChat)
        );
    }

    #[test]
    fn normalize_trims_whitespace() {
        assert_eq!(
            normalize_chat_channel_id("  discord  "),
            Some(ChatChannelId::Discord)
        );
    }

    #[test]
    fn normalize_rejects_unknown() {
        assert_eq!(normalize_chat_channel_id("matrix"), None);
        assert_eq!(normalize_chat_channel_id(""), None);
        assert_eq!(normalize_chat_channel_id("   "), None);
    }

    #[test]
    fn normalize_any_delegates_correctly() {
        assert_eq!(
            normalize_any_channel_id("tg"),
            Some(ChatChannelId::Telegram)
        );
        assert_eq!(normalize_any_channel_id("unknown"), None);
    }

    #[test]
    fn format_primer_line_includes_label_and_blurb() {
        let line = format_channel_primer_line(ChatChannelId::Discord);
        assert!(line.starts_with("Discord: "));
        assert!(line.contains("very well supported"));
    }

    #[test]
    fn default_channel_is_discord() {
        assert_eq!(DEFAULT_CHAT_CHANNEL, ChatChannelId::Discord);
    }

    #[test]
    fn channel_id_serialization_roundtrip() {
        let id = ChatChannelId::GoogleChat;
        let json = serde_json::to_string(&id).expect("serialize");
        assert_eq!(json, "\"googlechat\"");
        let parsed: ChatChannelId = serde_json::from_str(&json).expect("deserialize");
        assert_eq!(parsed, id);
    }

    #[test]
    fn channel_id_display() {
        assert_eq!(ChatChannelId::Telegram.to_string(), "telegram");
        assert_eq!(ChatChannelId::WhatsApp.to_string(), "whatsapp");
        assert_eq!(ChatChannelId::GoogleChat.to_string(), "googlechat");
        assert_eq!(ChatChannelId::IMessage.to_string(), "imessage");
    }
}
