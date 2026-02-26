/// Agent defaults configuration types.
///
/// Source: `src/config/types.agent-defaults.ts`

use std::collections::HashMap;

use serde::{Deserialize, Serialize};

use crate::base::{BlockStreamingChunkConfig, BlockStreamingCoalesceConfig, HumanDelayConfig, TypingMode};
use crate::sandbox::{SandboxBrowserSettings, SandboxDockerSettings, SandboxPruneSettings};
use crate::tools::{MemorySearchConfig, SubagentModelConfig};

// ── Agent model entry config ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct AgentModelEntryConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub alias: Option<String>,
    /// Provider-specific API parameters.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub params: Option<HashMap<String, serde_json::Value>>,
    /// Enable streaming for this model (default: true).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub streaming: Option<bool>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct AgentModelListConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub primary: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub fallbacks: Option<Vec<String>>,
}

// ── Context pruning ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum AgentContextPruningMode {
    Off,
    CacheTtl,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct AgentContextPruningSoftTrimConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_chars: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub head_chars: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tail_chars: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct AgentContextPruningHardClearConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub placeholder: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct AgentContextPruningToolsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub deny: Option<Vec<String>>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct AgentContextPruningConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mode: Option<AgentContextPruningMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub ttl: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub keep_last_assistants: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub soft_trim_ratio: Option<f64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub hard_clear_ratio: Option<f64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub min_prunable_tool_chars: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tools: Option<AgentContextPruningToolsConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub soft_trim: Option<AgentContextPruningSoftTrimConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub hard_clear: Option<AgentContextPruningHardClearConfig>,
}

// ── CLI backend ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum CliBackendOutputMode {
    Json,
    Text,
    Jsonl,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum CliBackendInputMode {
    Arg,
    Stdin,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum CliBackendSessionMode {
    Always,
    Existing,
    None,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum CliBackendSystemPromptMode {
    Append,
    Replace,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum CliBackendSystemPromptWhen {
    First,
    Always,
    Never,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum CliBackendImageMode {
    Repeat,
    List,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CliBackendConfig {
    pub command: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub args: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub output: Option<CliBackendOutputMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub resume_output: Option<CliBackendOutputMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub input: Option<CliBackendInputMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_prompt_arg_chars: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub env: Option<HashMap<String, String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub clear_env: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model_arg: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model_aliases: Option<HashMap<String, String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub session_arg: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub session_args: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub resume_args: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub session_mode: Option<CliBackendSessionMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub session_id_fields: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub system_prompt_arg: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub system_prompt_mode: Option<CliBackendSystemPromptMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub system_prompt_when: Option<CliBackendSystemPromptWhen>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub image_arg: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub image_mode: Option<CliBackendImageMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub serialize: Option<bool>,
}

// ── Compaction ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum AgentCompactionMode {
    Default,
    Safeguard,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct AgentCompactionMemoryFlushConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub soft_threshold_tokens: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub prompt: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub system_prompt: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct AgentCompactionConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mode: Option<AgentCompactionMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub reserve_tokens_floor: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_history_share: Option<f64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub memory_flush: Option<AgentCompactionMemoryFlushConfig>,
}

// ── Thinking / verbose / elevated / block streaming levels ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum ThinkingDefault {
    Off,
    Minimal,
    Low,
    Medium,
    High,
    Xhigh,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum VerboseDefault {
    Off,
    On,
    Full,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum ElevatedDefault {
    Off,
    On,
    Ask,
    Full,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum BlockStreamingDefault {
    Off,
    On,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum BlockStreamingBreak {
    TextEnd,
    MessageEnd,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum TimeFormat {
    Auto,
    #[serde(rename = "12")]
    Twelve,
    #[serde(rename = "24")]
    TwentyFour,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum EnvelopeToggle {
    On,
    Off,
}

// ── Sandbox mode (agents.defaults.sandbox) ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum SandboxMode {
    Off,
    NonMain,
    All,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum WorkspaceAccess {
    None,
    Ro,
    Rw,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum SessionToolsVisibility {
    Spawned,
    All,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum SandboxScope {
    Session,
    Agent,
    Shared,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct AgentDefaultsSandboxConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mode: Option<SandboxMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub workspace_access: Option<WorkspaceAccess>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub session_tools_visibility: Option<SessionToolsVisibility>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub scope: Option<SandboxScope>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub per_session: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub workspace_root: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub docker: Option<SandboxDockerSettings>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub browser: Option<SandboxBrowserSettings>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub prune: Option<SandboxPruneSettings>,
}

// ── Heartbeat ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct HeartbeatActiveHoursConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub start: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub end: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timezone: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct HeartbeatConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub every: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub active_hours: Option<HeartbeatActiveHoursConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub session: Option<String>,
    /// Delivery target ("last", "none", or a channel id).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub target: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub to: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub account_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub prompt: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub ack_max_chars: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub include_reasoning: Option<bool>,
}

// ── Subagent defaults ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct AgentDefaultsSubagentsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_concurrent: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub archive_after_minutes: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model: Option<SubagentModelConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub thinking: Option<String>,
}

// ── Top-level agent defaults ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct AgentDefaultsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model: Option<AgentModelListConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub image_model: Option<AgentModelListConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub models: Option<HashMap<String, AgentModelEntryConfig>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub workspace: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub repo_root: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub skip_bootstrap: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub bootstrap_max_chars: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub user_timezone: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub time_format: Option<TimeFormat>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub envelope_timezone: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub envelope_timestamp: Option<EnvelopeToggle>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub envelope_elapsed: Option<EnvelopeToggle>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub context_tokens: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cli_backends: Option<HashMap<String, CliBackendConfig>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub context_pruning: Option<AgentContextPruningConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub compaction: Option<AgentCompactionConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub memory_search: Option<MemorySearchConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub thinking_default: Option<ThinkingDefault>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub verbose_default: Option<VerboseDefault>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub elevated_default: Option<ElevatedDefault>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub block_streaming_default: Option<BlockStreamingDefault>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub block_streaming_break: Option<BlockStreamingBreak>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub block_streaming_chunk: Option<BlockStreamingChunkConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub block_streaming_coalesce: Option<BlockStreamingCoalesceConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub human_delay: Option<HumanDelayConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timeout_seconds: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub media_max_mb: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub typing_interval_seconds: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub typing_mode: Option<TypingMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub heartbeat: Option<HeartbeatConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_concurrent: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub subagents: Option<AgentDefaultsSubagentsConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub sandbox: Option<AgentDefaultsSandboxConfig>,
}
