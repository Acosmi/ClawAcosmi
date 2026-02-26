/// Memory configuration types.
///
/// Source: `src/config/types.memory.ts`

use serde::{Deserialize, Serialize};

use crate::base::SessionSendPolicyConfig;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum MemoryBackend {
    Builtin,
    Qmd,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum MemoryCitationsMode {
    Auto,
    On,
    Off,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct MemoryQmdIndexPath {
    pub path: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub pattern: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MemoryQmdSessionConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub export_dir: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub retention_days: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MemoryQmdUpdateConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub interval: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub debounce_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub on_boot: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub wait_for_boot_sync: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub embed_interval: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub command_timeout_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub update_timeout_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub embed_timeout_ms: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MemoryQmdLimitsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_results: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_snippet_chars: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_injected_chars: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timeout_ms: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MemoryQmdConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub command: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub include_default_memory: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub paths: Option<Vec<MemoryQmdIndexPath>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub sessions: Option<MemoryQmdSessionConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub update: Option<MemoryQmdUpdateConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub limits: Option<MemoryQmdLimitsConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub scope: Option<SessionSendPolicyConfig>,
}

/// Top-level memory configuration.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MemoryConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub backend: Option<MemoryBackend>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub citations: Option<MemoryCitationsMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub qmd: Option<MemoryQmdConfig>,
}
