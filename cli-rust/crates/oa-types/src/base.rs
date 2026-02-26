/// Base configuration types shared across modules.
///
/// Source: `src/config/types.base.ts`

use std::collections::HashMap;

use serde::{Deserialize, Serialize};

use crate::common::ChatType;

// ── Enums ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum ReplyMode {
    Text,
    Command,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum TypingMode {
    Never,
    Instant,
    Thinking,
    Message,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum SessionScope {
    PerSender,
    Global,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum DmScope {
    Main,
    PerPeer,
    PerChannelPeer,
    PerAccountChannelPeer,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum ReplyToMode {
    Off,
    First,
    All,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum GroupPolicy {
    Open,
    Disabled,
    Allowlist,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum DmPolicy {
    Pairing,
    Allowlist,
    Open,
    Disabled,
}

// ── Outbound retry ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct OutboundRetryConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub attempts: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub min_delay_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_delay_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub jitter: Option<f64>,
}

// ── Block streaming ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct BlockStreamingCoalesceConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub min_chars: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_chars: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub idle_ms: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum BlockStreamingBreakPreference {
    Paragraph,
    Newline,
    Sentence,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct BlockStreamingChunkConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub min_chars: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_chars: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub break_preference: Option<BlockStreamingBreakPreference>,
}

// ── Markdown ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum MarkdownTableMode {
    Off,
    Bullets,
    Code,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MarkdownConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tables: Option<MarkdownTableMode>,
}

// ── Human delay ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum HumanDelayMode {
    Off,
    Natural,
    Custom,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct HumanDelayConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mode: Option<HumanDelayMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub min_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_ms: Option<u64>,
}

// ── Session send policy ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum SessionSendPolicyAction {
    Allow,
    Deny,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct SessionSendPolicyMatch {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub channel: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub chat_type: Option<ChatType>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub key_prefix: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SessionSendPolicyRule {
    pub action: SessionSendPolicyAction,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub r#match: Option<SessionSendPolicyMatch>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct SessionSendPolicyConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub default: Option<SessionSendPolicyAction>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub rules: Option<Vec<SessionSendPolicyRule>>,
}

// ── Session reset ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum SessionResetMode {
    Daily,
    Idle,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct SessionResetConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mode: Option<SessionResetMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub at_hour: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub idle_minutes: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct SessionResetByTypeConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub direct: Option<SessionResetConfig>,
    /// @deprecated Use `direct` instead.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub dm: Option<SessionResetConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub group: Option<SessionResetConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub thread: Option<SessionResetConfig>,
}

// ── Agent-to-agent session config ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct SessionAgentToAgentConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_ping_pong_turns: Option<u32>,
}

// ── Session config ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct SessionConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub scope: Option<SessionScope>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub dm_scope: Option<DmScope>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub identity_links: Option<HashMap<String, Vec<String>>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub reset_triggers: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub idle_minutes: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub reset: Option<SessionResetConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub reset_by_type: Option<SessionResetByTypeConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub reset_by_channel: Option<HashMap<String, SessionResetConfig>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub store: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub typing_interval_seconds: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub typing_mode: Option<TypingMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub main_key: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub send_policy: Option<SessionSendPolicyConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub agent_to_agent: Option<SessionAgentToAgentConfig>,
}

// ── Logging ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum LogLevel {
    Silent,
    Fatal,
    Error,
    Warn,
    Info,
    Debug,
    Trace,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum ConsoleStyle {
    Pretty,
    Compact,
    Json,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum RedactSensitiveMode {
    Off,
    Tools,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct LoggingConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub level: Option<LogLevel>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub file: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub console_level: Option<LogLevel>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub console_style: Option<ConsoleStyle>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub redact_sensitive: Option<RedactSensitiveMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub redact_patterns: Option<Vec<String>>,
}

// ── Diagnostics ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum DiagnosticsOtelProtocol {
    #[serde(rename = "http/protobuf")]
    HttpProtobuf,
    #[serde(rename = "grpc")]
    Grpc,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct DiagnosticsOtelConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub endpoint: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub protocol: Option<DiagnosticsOtelProtocol>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub headers: Option<HashMap<String, String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub service_name: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub traces: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub metrics: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub logs: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub sample_rate: Option<f64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub flush_interval_ms: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct DiagnosticsCacheTraceConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub file_path: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub include_messages: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub include_prompt: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub include_system: Option<bool>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct DiagnosticsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub flags: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub otel: Option<DiagnosticsOtelConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cache_trace: Option<DiagnosticsCacheTraceConfig>,
}

// ── Web config ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct WebReconnectConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub initial_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub factor: Option<f64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub jitter: Option<f64>,
    /// 0 = unlimited
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_attempts: Option<u32>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct WebConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub heartbeat_seconds: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub reconnect: Option<WebReconnectConfig>,
}

// ── Agent elevated allow-from ──

/// Provider docking: allowlists keyed by provider id.
/// Values are arrays of string or number identifiers.
pub type AgentElevatedAllowFromConfig = HashMap<String, Vec<serde_json::Value>>;

// ── Identity ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct IdentityConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub theme: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub emoji: Option<String>,
    /// Avatar image: workspace-relative path, http(s) URL, or data URI.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub avatar: Option<String>,
}
