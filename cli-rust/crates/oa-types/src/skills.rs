/// Skills configuration types.
///
/// Source: `src/config/types.skills.ts`

use std::collections::HashMap;

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct SkillConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub api_key: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub env: Option<HashMap<String, String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub config: Option<HashMap<String, serde_json::Value>>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct SkillsLoadConfig {
    /// Additional skill folders to scan.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub extra_dirs: Option<Vec<String>>,
    /// Watch skill folders for changes and refresh the skills snapshot.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub watch: Option<bool>,
    /// Debounce for the skills watcher (ms).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub watch_debounce_ms: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum SkillsNodeManager {
    Npm,
    Pnpm,
    Yarn,
    Bun,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct SkillsInstallConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub prefer_brew: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub node_manager: Option<SkillsNodeManager>,
}

/// Top-level skills configuration.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct SkillsConfig {
    /// Optional bundled-skill allowlist (only affects bundled skills).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_bundled: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub load: Option<SkillsLoadConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub install: Option<SkillsInstallConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub entries: Option<HashMap<String, SkillConfig>>,
}
