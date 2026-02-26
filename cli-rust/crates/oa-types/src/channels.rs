/// Channel configuration types.
///
/// Source: `src/config/types.channels.ts`

use serde::{Deserialize, Serialize};

use crate::base::GroupPolicy;

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct ChannelHeartbeatVisibilityConfig {
    /// Show HEARTBEAT_OK acknowledgments in chat (default: false).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub show_ok: Option<bool>,
    /// Show heartbeat alerts with actual content (default: true).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub show_alerts: Option<bool>,
    /// Emit indicator events for UI status display (default: true).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub use_indicator: Option<bool>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct ChannelDefaultsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub group_policy: Option<GroupPolicy>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub heartbeat: Option<ChannelHeartbeatVisibilityConfig>,
}

/// Top-level channels configuration.
///
/// Channel-specific configs (whatsapp, telegram, etc.) are stored as
/// `serde_json::Value` since their full schemas are channel-specific
/// and vary widely.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct ChannelsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub defaults: Option<ChannelDefaultsConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub whatsapp: Option<serde_json::Value>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub telegram: Option<serde_json::Value>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub discord: Option<serde_json::Value>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub googlechat: Option<serde_json::Value>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub slack: Option<serde_json::Value>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub signal: Option<serde_json::Value>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub imessage: Option<serde_json::Value>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub msteams: Option<serde_json::Value>,
    /// Catch-all for additional channel configs (plugin channels, etc.).
    #[serde(flatten)]
    pub extra: std::collections::HashMap<String, serde_json::Value>,
}
