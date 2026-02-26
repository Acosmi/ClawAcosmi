/// Hooks configuration types.
///
/// Source: `src/config/types.hooks.ts`

use std::collections::HashMap;

use serde::{Deserialize, Serialize};

// ── Hook mapping ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct HookMappingMatch {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub path: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub source: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct HookMappingTransform {
    pub module: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub export: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum HookMappingAction {
    Wake,
    Agent,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum HookWakeMode {
    Now,
    NextHeartbeat,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum HookMappingChannel {
    Last,
    Whatsapp,
    Telegram,
    Discord,
    Googlechat,
    Slack,
    Signal,
    Imessage,
    Msteams,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct HookMappingConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub r#match: Option<HookMappingMatch>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub action: Option<HookMappingAction>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub wake_mode: Option<HookWakeMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub session_key: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub message_template: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub text_template: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub deliver: Option<bool>,
    /// DANGEROUS: Disable external content safety wrapping for this hook.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_unsafe_external_content: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub channel: Option<HookMappingChannel>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub to: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub thinking: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timeout_seconds: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub transform: Option<HookMappingTransform>,
}

// ── Gmail hooks ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum HooksGmailTailscaleMode {
    Off,
    Serve,
    Funnel,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum GmailHookThinking {
    Off,
    Minimal,
    Low,
    Medium,
    High,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct HooksGmailServeConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub bind: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub port: Option<u16>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub path: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct HooksGmailTailscaleConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mode: Option<HooksGmailTailscaleMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub path: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub target: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct HooksGmailConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub account: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub label: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub topic: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub subscription: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub push_token: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub hook_url: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub include_body: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_bytes: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub renew_every_minutes: Option<u64>,
    /// DANGEROUS: Disable external content safety wrapping for Gmail hooks.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_unsafe_external_content: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub serve: Option<HooksGmailServeConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tailscale: Option<HooksGmailTailscaleConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub thinking: Option<GmailHookThinking>,
}

// ── Internal hooks ──

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct InternalHookHandlerConfig {
    pub event: String,
    pub module: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub export: Option<String>,
}

/// Per-hook configuration. Uses flatten for dynamic keys plus known keys.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct HookConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub env: Option<HashMap<String, String>>,
    /// Catch-all for additional hook config keys.
    #[serde(flatten)]
    pub extra: HashMap<String, serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum HookInstallSource {
    Npm,
    Archive,
    Path,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct HookInstallRecord {
    pub source: HookInstallSource,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub spec: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub source_path: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub install_path: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub version: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub installed_at: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub hooks: Option<Vec<String>>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct InternalHooksLoadConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub extra_dirs: Option<Vec<String>>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct InternalHooksConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub handlers: Option<Vec<InternalHookHandlerConfig>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub entries: Option<HashMap<String, HookConfig>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub load: Option<InternalHooksLoadConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub installs: Option<HashMap<String, HookInstallRecord>>,
}

// ── Top-level hooks config ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct HooksConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub path: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub token: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_body_bytes: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub presets: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub transforms_dir: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mappings: Option<Vec<HookMappingConfig>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub gmail: Option<HooksGmailConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub internal: Option<InternalHooksConfig>,
}
