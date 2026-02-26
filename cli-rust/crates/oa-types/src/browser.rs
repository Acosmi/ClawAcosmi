/// Browser configuration types.
///
/// Source: `src/config/types.browser.ts`

use std::collections::HashMap;

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum BrowserProfileDriver {
    Openacosmi,
    Extension,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct BrowserProfileConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cdp_port: Option<u16>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cdp_url: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub driver: Option<BrowserProfileDriver>,
    pub color: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum BrowserSnapshotMode {
    Efficient,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct BrowserSnapshotDefaults {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mode: Option<BrowserSnapshotMode>,
}

/// Top-level browser configuration.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct BrowserConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub evaluate_enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cdp_url: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub remote_cdp_timeout_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub remote_cdp_handshake_timeout_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub color: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub executable_path: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub headless: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub no_sandbox: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub attach_only: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub default_profile: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub profiles: Option<HashMap<String, BrowserProfileConfig>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub snapshot_defaults: Option<BrowserSnapshotDefaults>,
}
