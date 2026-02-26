/// Messages, broadcast, audio, and commands configuration types.
///
/// Source: `src/config/types.messages.ts`

use std::collections::HashMap;

use serde::{Deserialize, Serialize};

use crate::queue::{QueueDropPolicy, QueueMode, QueueModeByProvider};
use crate::tts::TtsConfig;

// ── Group chat ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct GroupChatConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mention_patterns: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub history_limit: Option<u32>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct DmConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub history_limit: Option<u32>,
}

// ── Queue ──

/// Per-channel debounce overrides (ms).
pub type InboundDebounceByProvider = HashMap<String, u64>;

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct QueueConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mode: Option<QueueMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub by_channel: Option<QueueModeByProvider>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub debounce_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub debounce_ms_by_channel: Option<InboundDebounceByProvider>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cap: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub drop: Option<QueueDropPolicy>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct InboundDebounceConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub debounce_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub by_channel: Option<InboundDebounceByProvider>,
}

// ── Broadcast ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum BroadcastStrategy {
    Parallel,
    Sequential,
}

/// BroadcastConfig is a dynamic map. The "strategy" key is special (BroadcastStrategy),
/// while other keys map peer IDs to arrays of agent IDs.
/// We use serde_json::Value + flatten for the dynamic portion.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct BroadcastConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub strategy: Option<BroadcastStrategy>,
    /// Peer-to-agent mappings (key=peerId, value=array of agent IDs).
    #[serde(flatten)]
    pub peers: HashMap<String, serde_json::Value>,
}

// ── Audio ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct AudioTranscriptionConfig {
    pub command: Vec<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timeout_seconds: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct AudioConfig {
    /// @deprecated Use tools.media.audio.models instead.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub transcription: Option<AudioTranscriptionConfig>,
}

// ── Ack reaction scope ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum AckReactionScope {
    GroupMentions,
    GroupAll,
    Direct,
    All,
}

// ── Messages config ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MessagesConfig {
    /// @deprecated Use `whatsapp.messagePrefix` (WhatsApp-only inbound prefix).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub message_prefix: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub response_prefix: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub group_chat: Option<GroupChatConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub queue: Option<QueueConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub inbound: Option<InboundDebounceConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub ack_reaction: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub ack_reaction_scope: Option<AckReactionScope>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub remove_ack_after_reply: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tts: Option<TtsConfig>,
}

// ── Native commands setting (bool | "auto") ──

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(untagged)]
pub enum NativeCommandsSetting {
    Bool(bool),
    Auto(String),
}

// ── Provider commands ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct ProviderCommandsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub native: Option<NativeCommandsSetting>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub native_skills: Option<NativeCommandsSetting>,
}

// ── Commands config ──

/// Owner allow-from: array of string or number identifiers.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(untagged)]
pub enum OwnerAllowFromEntry {
    String(String),
    Number(i64),
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct CommandsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub native: Option<NativeCommandsSetting>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub native_skills: Option<NativeCommandsSetting>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub text: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub bash: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub bash_foreground_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub config: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub debug: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub restart: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub use_access_groups: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub owner_allow_from: Option<Vec<OwnerAllowFromEntry>>,
}
