/// OpenAcosmi configuration types.
///
/// Source: `src/config/types.openacosmi.ts` and related type files.

use std::collections::HashMap;

use serde::{Deserialize, Serialize};

use crate::agents::{AgentBinding, AgentsConfig};
use crate::approvals::ApprovalsConfig;
use crate::auth::AuthConfig;
use crate::base::{DiagnosticsConfig, LoggingConfig, SessionConfig, WebConfig};
use crate::browser::BrowserConfig;
use crate::channels::ChannelsConfig;
use crate::cron::CronConfig;
use crate::gateway::{CanvasHostConfig, DiscoveryConfig, GatewayConfig, TalkConfig};
use crate::hooks::HooksConfig;
use crate::memory::MemoryConfig;
use crate::messages::{AudioConfig, BroadcastConfig, CommandsConfig, MessagesConfig};
use crate::models::ModelsConfig;
use crate::node_host::NodeHostConfig;
use crate::plugins::PluginsConfig;
use crate::skills::SkillsConfig;
use crate::tools::ToolsConfig;

// ── Meta ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct ConfigMeta {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub last_touched_version: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub last_touched_at: Option<String>,
}

// ── Env ──

/// The `env` section is highly dynamic (index signature in TS).
/// We model known keys and flatten the rest.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct EnvShellEnvConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timeout_ms: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct EnvConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub shell_env: Option<EnvShellEnvConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub vars: Option<HashMap<String, String>>,
    /// Additional env vars (sugar: direct string values under `env`).
    #[serde(flatten)]
    pub extra: HashMap<String, serde_json::Value>,
}

// ── Wizard ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum WizardRunMode {
    Local,
    Remote,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct WizardConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub last_run_at: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub last_run_version: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub last_run_commit: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub last_run_command: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub last_run_mode: Option<WizardRunMode>,
}

// ── Update ──

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum UpdateChannel {
    Stable,
    Beta,
    Dev,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct UpdateConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub channel: Option<UpdateChannel>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub check_on_start: Option<bool>,
}

// ── UI ──

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct UiAssistantConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub avatar: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct UiConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub seam_color: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub assistant: Option<UiAssistantConfig>,
}

// ── OpenAcosmiConfig (root) ──

/// The root OpenAcosmi configuration object.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct OpenAcosmiConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub meta: Option<ConfigMeta>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub auth: Option<AuthConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub env: Option<EnvConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub wizard: Option<WizardConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub diagnostics: Option<DiagnosticsConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub logging: Option<LoggingConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub update: Option<UpdateConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub browser: Option<BrowserConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub ui: Option<UiConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub skills: Option<SkillsConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub plugins: Option<PluginsConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub models: Option<ModelsConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub node_host: Option<NodeHostConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub agents: Option<AgentsConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tools: Option<ToolsConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub bindings: Option<Vec<AgentBinding>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub broadcast: Option<BroadcastConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub audio: Option<AudioConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub messages: Option<MessagesConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub commands: Option<CommandsConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub approvals: Option<ApprovalsConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub session: Option<SessionConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub web: Option<WebConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub channels: Option<ChannelsConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cron: Option<CronConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub hooks: Option<HooksConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub discovery: Option<DiscoveryConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub canvas_host: Option<CanvasHostConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub talk: Option<TalkConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub gateway: Option<GatewayConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub memory: Option<MemoryConfig>,
}

// ── Config validation / snapshot types ──

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ConfigValidationIssue {
    pub path: String,
    pub message: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct LegacyConfigIssue {
    pub path: String,
    pub message: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ConfigFileSnapshot {
    pub path: String,
    pub exists: bool,
    pub raw: Option<String>,
    pub parsed: Option<serde_json::Value>,
    pub valid: bool,
    pub config: OpenAcosmiConfig,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub hash: Option<String>,
    pub issues: Vec<ConfigValidationIssue>,
    pub warnings: Vec<ConfigValidationIssue>,
    pub legacy_issues: Vec<LegacyConfigIssue>,
}
