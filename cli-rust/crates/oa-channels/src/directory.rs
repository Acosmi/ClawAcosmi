/// Channel directory: capabilities metadata for each built-in channel.
///
/// Provides the `ChannelCapabilities` struct and a lookup function that returns
/// the static capability profile for any `ChatChannelId`. This is the Rust
/// equivalent of the "docks" metadata from the TypeScript codebase.
///
/// Source: `src/channels/dock.ts`

use serde::{Deserialize, Serialize};

use crate::registry::ChatChannelId;

/// Describes the reply mode a channel uses by default.
///
/// Source: `src/channels/dock.ts` — `resolveReplyToMode`
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ReplyMode {
    /// No automatic replies to threads.
    Off,
    /// Reply to the first message in a conversation.
    First,
    /// Reply to every message.
    All,
}

/// Static capability profile for a chat channel.
///
/// Each channel has a fixed set of capabilities that shared code can query
/// without importing heavy channel implementations.
///
/// Source: `src/channels/dock.ts` — `ChannelCapabilities` + dock entries
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ChannelCapabilities {
    /// Maximum text chunk size for outbound messages (characters).
    pub text_chunk_limit: usize,
    /// Whether the channel supports emoji reactions.
    pub supports_reactions: bool,
    /// Whether the channel supports threaded conversations.
    pub supports_threads: bool,
    /// Whether the channel supports @-mentions.
    pub supports_mentions: bool,
    /// Whether the channel supports group conversations.
    pub supports_groups: bool,
    /// Whether the channel supports elevated/DM-specific access control.
    pub supports_elevated: bool,
    /// Whether the channel supports typing indicators.
    pub supports_typing: bool,
    /// Whether the channel supports media/file attachments.
    pub supports_attachments: bool,
    /// Default reply mode for threaded conversations.
    pub reply_mode: ReplyMode,
}

/// Returns the capability profile for a given channel.
///
/// The values are derived from the TypeScript dock entries: `capabilities`,
/// `outbound.textChunkLimit`, and `threading.resolveReplyToMode` defaults.
///
/// Source: `src/channels/dock.ts` — `DOCKS`
#[must_use]
pub fn get_channel_capabilities(id: ChatChannelId) -> ChannelCapabilities {
    match id {
        ChatChannelId::Telegram => ChannelCapabilities {
            text_chunk_limit: 4000,
            supports_reactions: false,
            supports_threads: false,
            supports_mentions: false,
            supports_groups: true,
            supports_elevated: false,
            supports_typing: false,
            supports_attachments: false,
            reply_mode: ReplyMode::First,
        },
        ChatChannelId::WhatsApp => ChannelCapabilities {
            text_chunk_limit: 4000,
            supports_reactions: true,
            supports_threads: false,
            supports_mentions: true,
            supports_groups: true,
            supports_elevated: false,
            supports_typing: false,
            supports_attachments: true,
            reply_mode: ReplyMode::Off,
        },
        ChatChannelId::Discord => ChannelCapabilities {
            text_chunk_limit: 2000,
            supports_reactions: true,
            supports_threads: true,
            supports_mentions: true,
            supports_groups: false,
            supports_elevated: true,
            supports_typing: false,
            supports_attachments: true,
            reply_mode: ReplyMode::Off,
        },
        ChatChannelId::GoogleChat => ChannelCapabilities {
            text_chunk_limit: 4000,
            supports_reactions: true,
            supports_threads: true,
            supports_mentions: false,
            supports_groups: true,
            supports_elevated: false,
            supports_typing: false,
            supports_attachments: true,
            reply_mode: ReplyMode::Off,
        },
        ChatChannelId::Slack => ChannelCapabilities {
            text_chunk_limit: 4000,
            supports_reactions: true,
            supports_threads: true,
            supports_mentions: true,
            supports_groups: false,
            supports_elevated: false,
            supports_typing: false,
            supports_attachments: true,
            reply_mode: ReplyMode::Off,
        },
        ChatChannelId::Signal => ChannelCapabilities {
            text_chunk_limit: 4000,
            supports_reactions: true,
            supports_threads: false,
            supports_mentions: false,
            supports_groups: true,
            supports_elevated: false,
            supports_typing: false,
            supports_attachments: true,
            reply_mode: ReplyMode::Off,
        },
        ChatChannelId::IMessage => ChannelCapabilities {
            text_chunk_limit: 4000,
            supports_reactions: true,
            supports_threads: false,
            supports_mentions: false,
            supports_groups: true,
            supports_elevated: false,
            supports_typing: false,
            supports_attachments: true,
            reply_mode: ReplyMode::Off,
        },
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn discord_has_2000_chunk_limit() {
        let caps = get_channel_capabilities(ChatChannelId::Discord);
        assert_eq!(caps.text_chunk_limit, 2000);
    }

    #[test]
    fn telegram_chunk_limit_is_4000() {
        let caps = get_channel_capabilities(ChatChannelId::Telegram);
        assert_eq!(caps.text_chunk_limit, 4000);
    }

    #[test]
    fn discord_supports_threads() {
        let caps = get_channel_capabilities(ChatChannelId::Discord);
        assert!(caps.supports_threads);
    }

    #[test]
    fn whatsapp_supports_reactions_but_not_threads() {
        let caps = get_channel_capabilities(ChatChannelId::WhatsApp);
        assert!(caps.supports_reactions);
        assert!(!caps.supports_threads);
    }

    #[test]
    fn slack_supports_threads_and_reactions() {
        let caps = get_channel_capabilities(ChatChannelId::Slack);
        assert!(caps.supports_threads);
        assert!(caps.supports_reactions);
    }

    #[test]
    fn telegram_default_reply_mode_is_first() {
        let caps = get_channel_capabilities(ChatChannelId::Telegram);
        assert_eq!(caps.reply_mode, ReplyMode::First);
    }

    #[test]
    fn discord_default_reply_mode_is_off() {
        let caps = get_channel_capabilities(ChatChannelId::Discord);
        assert_eq!(caps.reply_mode, ReplyMode::Off);
    }

    #[test]
    fn signal_supports_reactions_and_attachments() {
        let caps = get_channel_capabilities(ChatChannelId::Signal);
        assert!(caps.supports_reactions);
        assert!(caps.supports_attachments);
    }

    #[test]
    fn discord_supports_elevated() {
        let caps = get_channel_capabilities(ChatChannelId::Discord);
        assert!(caps.supports_elevated);
    }

    #[test]
    fn imessage_does_not_support_elevated() {
        let caps = get_channel_capabilities(ChatChannelId::IMessage);
        assert!(!caps.supports_elevated);
    }

    #[test]
    fn capabilities_serialization_roundtrip() {
        let caps = get_channel_capabilities(ChatChannelId::Discord);
        let json = serde_json::to_string(&caps).expect("serialize");
        let parsed: ChannelCapabilities = serde_json::from_str(&json).expect("deserialize");
        assert_eq!(parsed.text_chunk_limit, caps.text_chunk_limit);
        assert_eq!(parsed.supports_threads, caps.supports_threads);
    }

    #[test]
    fn all_channels_have_positive_chunk_limit() {
        for id in crate::registry::CHAT_CHANNEL_ORDER {
            let caps = get_channel_capabilities(*id);
            assert!(
                caps.text_chunk_limit > 0,
                "channel {:?} has zero text_chunk_limit",
                id
            );
        }
    }
}
