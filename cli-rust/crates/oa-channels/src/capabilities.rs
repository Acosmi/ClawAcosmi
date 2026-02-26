/// Convenience helpers for querying individual channel capabilities.
///
/// These are thin wrappers around [`crate::directory::get_channel_capabilities`]
/// that make call-sites more readable when only a single boolean or value is needed.
///
/// Source: `src/channels/dock.ts`

use crate::directory::get_channel_capabilities;
use crate::registry::ChatChannelId;

/// Returns `true` if the channel supports threaded conversations.
///
/// Source: `src/channels/dock.ts` — `capabilities.threads`
#[must_use]
pub fn can_channel_thread(id: ChatChannelId) -> bool {
    get_channel_capabilities(id).supports_threads
}

/// Returns `true` if the channel supports emoji reactions.
///
/// Source: `src/channels/dock.ts` — `capabilities.reactions`
#[must_use]
pub fn can_channel_react(id: ChatChannelId) -> bool {
    get_channel_capabilities(id).supports_reactions
}

/// Returns `true` if the channel supports @-mentions.
///
/// Source: `src/channels/dock.ts` — `mentions` adapter presence
#[must_use]
pub fn can_channel_mention(id: ChatChannelId) -> bool {
    get_channel_capabilities(id).supports_mentions
}

/// Returns the maximum outbound text chunk size (in characters) for a channel.
///
/// Source: `src/channels/dock.ts` — `outbound.textChunkLimit`
#[must_use]
pub fn get_text_chunk_limit(id: ChatChannelId) -> usize {
    get_channel_capabilities(id).text_chunk_limit
}

/// Returns `true` if the channel supports media/file attachments.
///
/// Source: `src/channels/dock.ts` — `capabilities.media`
#[must_use]
pub fn can_channel_attach(id: ChatChannelId) -> bool {
    get_channel_capabilities(id).supports_attachments
}

/// Returns `true` if the channel supports group conversations.
///
/// Source: `src/channels/dock.ts` — `groups` adapter / `capabilities.chatTypes`
#[must_use]
pub fn can_channel_group(id: ChatChannelId) -> bool {
    get_channel_capabilities(id).supports_groups
}

/// Returns `true` if the channel supports elevated/DM access control.
///
/// Source: `src/channels/dock.ts` — `elevated` adapter presence
#[must_use]
pub fn can_channel_elevate(id: ChatChannelId) -> bool {
    get_channel_capabilities(id).supports_elevated
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn discord_can_thread() {
        assert!(can_channel_thread(ChatChannelId::Discord));
    }

    #[test]
    fn whatsapp_cannot_thread() {
        assert!(!can_channel_thread(ChatChannelId::WhatsApp));
    }

    #[test]
    fn discord_can_react() {
        assert!(can_channel_react(ChatChannelId::Discord));
    }

    #[test]
    fn telegram_cannot_react() {
        assert!(!can_channel_react(ChatChannelId::Telegram));
    }

    #[test]
    fn discord_can_mention() {
        assert!(can_channel_mention(ChatChannelId::Discord));
    }

    #[test]
    fn signal_cannot_mention() {
        assert!(!can_channel_mention(ChatChannelId::Signal));
    }

    #[test]
    fn discord_chunk_limit() {
        assert_eq!(get_text_chunk_limit(ChatChannelId::Discord), 2000);
    }

    #[test]
    fn slack_chunk_limit() {
        assert_eq!(get_text_chunk_limit(ChatChannelId::Slack), 4000);
    }

    #[test]
    fn discord_can_elevate() {
        assert!(can_channel_elevate(ChatChannelId::Discord));
    }

    #[test]
    fn signal_cannot_elevate() {
        assert!(!can_channel_elevate(ChatChannelId::Signal));
    }

    #[test]
    fn whatsapp_can_attach() {
        assert!(can_channel_attach(ChatChannelId::WhatsApp));
    }

    #[test]
    fn whatsapp_can_group() {
        assert!(can_channel_group(ChatChannelId::WhatsApp));
    }
}
