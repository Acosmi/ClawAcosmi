/// Tool configuration types.
///
/// Source: `src/config/types.tools.ts`

use std::collections::HashMap;

use serde::{Deserialize, Serialize};

use crate::base::{AgentElevatedAllowFromConfig, SessionSendPolicyAction};
use crate::common::ChatType;

// ── Media understanding scope ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MediaUnderstandingScopeMatch {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub channel: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub chat_type: Option<ChatType>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub key_prefix: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct MediaUnderstandingScopeRule {
    pub action: SessionSendPolicyAction,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub r#match: Option<MediaUnderstandingScopeMatch>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MediaUnderstandingScopeConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub default: Option<SessionSendPolicyAction>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub rules: Option<Vec<MediaUnderstandingScopeRule>>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum MediaUnderstandingCapability {
    Image,
    Audio,
    Video,
}

// ── Media understanding attachments ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum MediaAttachmentsMode {
    First,
    All,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum MediaAttachmentsPrefer {
    First,
    Last,
    Path,
    Url,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MediaUnderstandingAttachmentsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mode: Option<MediaAttachmentsMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_attachments: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub prefer: Option<MediaAttachmentsPrefer>,
}

// ── Media understanding model ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum MediaUnderstandingModelType {
    Provider,
    Cli,
}

/// @deprecated Use providerOptions.deepgram instead.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct DeepgramLegacyConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub detect_language: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub punctuate: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub smart_format: Option<bool>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MediaUnderstandingModelConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub provider: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub capabilities: Option<Vec<MediaUnderstandingCapability>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub r#type: Option<MediaUnderstandingModelType>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub command: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub args: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub prompt: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_chars: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_bytes: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timeout_seconds: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub language: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub provider_options: Option<HashMap<String, HashMap<String, serde_json::Value>>>,
    /// @deprecated Use providerOptions.deepgram instead.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub deepgram: Option<DeepgramLegacyConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub base_url: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub headers: Option<HashMap<String, String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub profile: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub preferred_profile: Option<String>,
}

// ── Media understanding config ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MediaUnderstandingConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub scope: Option<MediaUnderstandingScopeConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_bytes: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_chars: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub prompt: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timeout_seconds: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub language: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub provider_options: Option<HashMap<String, HashMap<String, serde_json::Value>>>,
    /// @deprecated Use providerOptions.deepgram instead.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub deepgram: Option<DeepgramLegacyConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub base_url: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub headers: Option<HashMap<String, String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub attachments: Option<MediaUnderstandingAttachmentsConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub models: Option<Vec<MediaUnderstandingModelConfig>>,
}

// ── Link tools ──

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct LinkModelConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub r#type: Option<String>,
    pub command: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub args: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timeout_seconds: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct LinkToolsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub scope: Option<MediaUnderstandingScopeConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_links: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timeout_seconds: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub models: Option<Vec<LinkModelConfig>>,
}

// ── Media tools ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MediaToolsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub models: Option<Vec<MediaUnderstandingModelConfig>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub concurrency: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub image: Option<MediaUnderstandingConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub audio: Option<MediaUnderstandingConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub video: Option<MediaUnderstandingConfig>,
}

// ── Tool profiles and policies ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum ToolProfileId {
    Minimal,
    Coding,
    Messaging,
    Full,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct ToolPolicyConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub also_allow: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub deny: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub profile: Option<ToolProfileId>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct GroupToolPolicyConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub also_allow: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub deny: Option<Vec<String>>,
}

/// Per-sender group tool policy.
pub type GroupToolPolicyBySenderConfig = HashMap<String, GroupToolPolicyConfig>;

// ── Exec tool ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum ExecToolHost {
    Sandbox,
    Gateway,
    Node,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum ExecToolSecurity {
    Deny,
    Allowlist,
    Full,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum ExecToolAsk {
    Off,
    OnMiss,
    Always,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct ExecApplyPatchConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_models: Option<Vec<String>>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct ExecToolConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub host: Option<ExecToolHost>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub security: Option<ExecToolSecurity>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub ask: Option<ExecToolAsk>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub node: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub path_prepend: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub safe_bins: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub background_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timeout_sec: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub approval_running_notice_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cleanup_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub notify_on_exit: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub apply_patch: Option<ExecApplyPatchConfig>,
}

// ── Agent tools config ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct AgentToolsElevatedConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_from: Option<AgentElevatedAllowFromConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct AgentToolsSandboxToolsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub deny: Option<Vec<String>>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct AgentToolsSandboxConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tools: Option<AgentToolsSandboxToolsConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct AgentToolsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub profile: Option<ToolProfileId>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub also_allow: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub deny: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub by_provider: Option<HashMap<String, ToolPolicyConfig>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub elevated: Option<AgentToolsElevatedConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub exec: Option<ExecToolConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub sandbox: Option<AgentToolsSandboxConfig>,
}

// ── Memory search ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum MemorySearchSource {
    Memory,
    Sessions,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum MemorySearchProvider {
    Openai,
    Gemini,
    Local,
    Voyage,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum MemorySearchFallback {
    Openai,
    Gemini,
    Local,
    Voyage,
    None,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MemorySearchExperimentalConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub session_memory: Option<bool>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MemorySearchRemoteBatchConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub wait: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub concurrency: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub poll_interval_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timeout_minutes: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MemorySearchRemoteConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub base_url: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub api_key: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub headers: Option<HashMap<String, String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub batch: Option<MemorySearchRemoteBatchConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MemorySearchLocalConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model_path: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model_cache_dir: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MemorySearchStoreVectorConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub extension_path: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MemorySearchStoreCacheConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_entries: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum MemorySearchStoreDriver {
    Sqlite,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MemorySearchStoreConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub driver: Option<MemorySearchStoreDriver>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub path: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub vector: Option<MemorySearchStoreVectorConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cache: Option<MemorySearchStoreCacheConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MemorySearchChunkingConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tokens: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub overlap: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MemorySearchSyncSessionsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub delta_bytes: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub delta_messages: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MemorySearchSyncConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub on_session_start: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub on_search: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub watch: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub watch_debounce_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub interval_minutes: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub sessions: Option<MemorySearchSyncSessionsConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MemorySearchQueryHybridConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub vector_weight: Option<f64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub text_weight: Option<f64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub candidate_multiplier: Option<u32>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MemorySearchQueryConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_results: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub min_score: Option<f64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub hybrid: Option<MemorySearchQueryHybridConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MemorySearchCacheConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_entries: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MemorySearchConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub sources: Option<Vec<MemorySearchSource>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub extra_paths: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub experimental: Option<MemorySearchExperimentalConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub provider: Option<MemorySearchProvider>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub remote: Option<MemorySearchRemoteConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub fallback: Option<MemorySearchFallback>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub local: Option<MemorySearchLocalConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub store: Option<MemorySearchStoreConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub chunking: Option<MemorySearchChunkingConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub sync: Option<MemorySearchSyncConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub query: Option<MemorySearchQueryConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cache: Option<MemorySearchCacheConfig>,
}

// ── Web search / fetch ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum WebSearchProvider {
    Brave,
    Perplexity,
    Grok,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct WebSearchPerplexityConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub api_key: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub base_url: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct WebSearchGrokConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub api_key: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub inline_citations: Option<bool>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct WebSearchConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub provider: Option<WebSearchProvider>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub api_key: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_results: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timeout_seconds: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cache_ttl_minutes: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub perplexity: Option<WebSearchPerplexityConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub grok: Option<WebSearchGrokConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct WebFetchFirecrawlConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub api_key: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub base_url: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub only_main_content: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_age_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timeout_seconds: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct WebFetchConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_chars: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_chars_cap: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timeout_seconds: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cache_ttl_minutes: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_redirects: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub user_agent: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub readability: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub firecrawl: Option<WebFetchFirecrawlConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct WebToolsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub search: Option<WebSearchConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub fetch: Option<WebFetchConfig>,
}

// ── Message tool ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MessageCrossContextMarkerConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub prefix: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub suffix: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MessageCrossContextConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_within_provider: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_across_providers: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub marker: Option<MessageCrossContextMarkerConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MessageBroadcastConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MessageToolConfig {
    /// @deprecated Use tools.message.crossContext settings.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_cross_context_send: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cross_context: Option<MessageCrossContextConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub broadcast: Option<MessageBroadcastConfig>,
}

// ── Agent-to-agent ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct AgentToAgentToolsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow: Option<Vec<String>>,
}

// ── Elevated ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct ToolsElevatedConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_from: Option<AgentElevatedAllowFromConfig>,
}

// ── Subagent model (union type) ──

/// Sub-agent model config: either a string or an object with primary + fallbacks.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(untagged)]
pub enum SubagentModelConfig {
    String(String),
    Object {
        #[serde(default, skip_serializing_if = "Option::is_none")]
        primary: Option<String>,
        #[serde(default, skip_serializing_if = "Option::is_none")]
        fallbacks: Option<Vec<String>>,
    },
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct ToolsSubagentsToolsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub deny: Option<Vec<String>>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct ToolsSubagentsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model: Option<SubagentModelConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tools: Option<ToolsSubagentsToolsConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct ToolsSandboxToolsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub deny: Option<Vec<String>>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct ToolsSandboxConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tools: Option<ToolsSandboxToolsConfig>,
}

// ── Top-level tools config ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct ToolsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub profile: Option<ToolProfileId>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub also_allow: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub deny: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub by_provider: Option<HashMap<String, ToolPolicyConfig>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub web: Option<WebToolsConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub media: Option<MediaToolsConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub links: Option<LinkToolsConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub message: Option<MessageToolConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub agent_to_agent: Option<AgentToAgentToolsConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub elevated: Option<ToolsElevatedConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub exec: Option<ExecToolConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub subagents: Option<ToolsSubagentsConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub sandbox: Option<ToolsSandboxConfig>,
}
