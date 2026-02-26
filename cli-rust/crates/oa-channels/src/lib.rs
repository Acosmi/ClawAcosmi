/// Channel plugin registry, directory, and capabilities.
///
/// This crate provides the static metadata and capability profiles for all
/// built-in chat channels (Telegram, WhatsApp, Discord, Google Chat, Slack,
/// Signal, iMessage). It is intentionally lightweight: no monitors, no heavy
/// channel implementations, just IDs, metadata, and capability checks.
///
/// Source: `src/channels/registry.ts`, `src/channels/dock.ts`

pub mod registry;
pub mod directory;
pub mod capabilities;

// ── Convenience re-exports ──

pub use registry::{
    ChatChannelId, ChatChannelMeta, CHANNEL_IDS, CHAT_CHANNEL_ORDER, DEFAULT_CHAT_CHANNEL,
    format_channel_primer_line, get_chat_channel_meta, list_chat_channels,
    normalize_any_channel_id, normalize_chat_channel_id,
};

pub use directory::{ChannelCapabilities, ReplyMode, get_channel_capabilities};

pub use capabilities::{
    can_channel_attach, can_channel_elevate, can_channel_group, can_channel_mention,
    can_channel_react, can_channel_thread, get_text_chunk_limit,
};
