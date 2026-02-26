/// Node host configuration types.
///
/// Source: `src/config/types.node-host.ts`

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NodeHostBrowserProxyConfig {
    /// Enable the browser proxy on the node host (default: true).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    /// Optional allowlist of profile names exposed via the proxy.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_profiles: Option<Vec<String>>,
}

/// Top-level node host configuration.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NodeHostConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub browser_proxy: Option<NodeHostBrowserProxyConfig>,
}
