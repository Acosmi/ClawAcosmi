/// Gateway configuration types.
///
/// Source: `src/config/types.gateway.ts`

use std::collections::HashMap;

use serde::{Deserialize, Serialize};

// ── Bind mode ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum GatewayBindMode {
    Auto,
    Lan,
    Loopback,
    Custom,
    Tailnet,
}

// ── TLS ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct GatewayTlsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub auto_generate: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cert_path: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub key_path: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub ca_path: Option<String>,
}

// ── Discovery ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct WideAreaDiscoveryConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub domain: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum MdnsDiscoveryMode {
    Off,
    Minimal,
    Full,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct MdnsDiscoveryConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mode: Option<MdnsDiscoveryMode>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct DiscoveryConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub wide_area: Option<WideAreaDiscoveryConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mdns: Option<MdnsDiscoveryConfig>,
}

// ── Canvas host ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct CanvasHostConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub root: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub port: Option<u16>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub live_reload: Option<bool>,
}

// ── Talk ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct TalkConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub voice_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub voice_aliases: Option<HashMap<String, String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub output_format: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub api_key: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub interrupt_on_speech: Option<bool>,
}

// ── Control UI ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct GatewayControlUiConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub base_path: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub root: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allowed_origins: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_insecure_auth: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub dangerously_disable_device_auth: Option<bool>,
}

// ── Auth ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum GatewayAuthMode {
    Token,
    Password,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct GatewayAuthConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mode: Option<GatewayAuthMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub token: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub password: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_tailscale: Option<bool>,
}

// ── Tailscale ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum GatewayTailscaleMode {
    Off,
    Serve,
    Funnel,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct GatewayTailscaleConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mode: Option<GatewayTailscaleMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub reset_on_exit: Option<bool>,
}

// ── Remote ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum GatewayRemoteTransport {
    Ssh,
    Direct,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct GatewayRemoteConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub url: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub transport: Option<GatewayRemoteTransport>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub token: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub password: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tls_fingerprint: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub ssh_target: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub ssh_identity: Option<String>,
}

// ── Reload ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum GatewayReloadMode {
    Off,
    Restart,
    Hot,
    Hybrid,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct GatewayReloadConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mode: Option<GatewayReloadMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub debounce_ms: Option<u64>,
}

// ── HTTP endpoints ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct GatewayHttpChatCompletionsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct GatewayHttpResponsesPdfConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_pages: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_pixels: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub min_text_chars: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct GatewayHttpResponsesFilesConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_url: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allowed_mimes: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_bytes: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_chars: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_redirects: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timeout_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub pdf: Option<GatewayHttpResponsesPdfConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct GatewayHttpResponsesImagesConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_url: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allowed_mimes: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_bytes: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_redirects: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timeout_ms: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct GatewayHttpResponsesConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_body_bytes: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub files: Option<GatewayHttpResponsesFilesConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub images: Option<GatewayHttpResponsesImagesConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct GatewayHttpEndpointsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub chat_completions: Option<GatewayHttpChatCompletionsConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub responses: Option<GatewayHttpResponsesConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct GatewayHttpConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub endpoints: Option<GatewayHttpEndpointsConfig>,
}

// ── Nodes ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum GatewayNodesBrowserMode {
    Auto,
    Manual,
    Off,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct GatewayNodesBrowserConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mode: Option<GatewayNodesBrowserMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub node: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct GatewayNodesConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub browser: Option<GatewayNodesBrowserConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_commands: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub deny_commands: Option<Vec<String>>,
}

// ── Gateway mode ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum GatewayMode {
    Local,
    Remote,
}

// ── Top-level gateway config ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct GatewayConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub port: Option<u16>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mode: Option<GatewayMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub bind: Option<GatewayBindMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub custom_bind_host: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub control_ui: Option<GatewayControlUiConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub auth: Option<GatewayAuthConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tailscale: Option<GatewayTailscaleConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub remote: Option<GatewayRemoteConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub reload: Option<GatewayReloadConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tls: Option<GatewayTlsConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub http: Option<GatewayHttpConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub nodes: Option<GatewayNodesConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub trusted_proxies: Option<Vec<String>>,
}
