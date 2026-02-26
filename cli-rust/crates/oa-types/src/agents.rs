/// Agent configuration types.
///
/// Source: `src/config/types.agents.ts`

use serde::{Deserialize, Serialize};

use crate::agent_defaults::{
    AgentDefaultsConfig, HeartbeatConfig, SandboxMode, SandboxScope, SessionToolsVisibility,
    WorkspaceAccess,
};
use crate::base::{HumanDelayConfig, IdentityConfig};
use crate::common::ChatType;
use crate::messages::GroupChatConfig;
use crate::sandbox::{SandboxBrowserSettings, SandboxDockerSettings, SandboxPruneSettings};
use crate::tools::{AgentToolsConfig, MemorySearchConfig, SubagentModelConfig};

/// Agent model config: either a simple string or an object with primary/fallbacks.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(untagged)]
pub enum AgentModelConfig {
    String(String),
    Object {
        #[serde(default, skip_serializing_if = "Option::is_none")]
        primary: Option<String>,
        #[serde(default, skip_serializing_if = "Option::is_none")]
        fallbacks: Option<Vec<String>>,
    },
}

/// Per-agent sandbox configuration.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct AgentSandboxConfig {
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

/// Per-agent subagents configuration.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct AgentSubagentsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_agents: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model: Option<SubagentModelConfig>,
}

/// A single agent configuration.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AgentConfig {
    pub id: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub default: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub workspace: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub agent_dir: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model: Option<AgentModelConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub skills: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub memory_search: Option<MemorySearchConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub human_delay: Option<HumanDelayConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub heartbeat: Option<HeartbeatConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub identity: Option<IdentityConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub group_chat: Option<GroupChatConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub subagents: Option<AgentSubagentsConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub sandbox: Option<AgentSandboxConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tools: Option<AgentToolsConfig>,
}

/// Top-level agents configuration.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct AgentsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub defaults: Option<AgentDefaultsConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub list: Option<Vec<AgentConfig>>,
}

/// Peer binding match for an agent.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AgentBindingPeer {
    pub kind: ChatType,
    pub id: String,
}

/// Agent binding match criteria.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AgentBindingMatch {
    pub channel: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub account_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub peer: Option<AgentBindingPeer>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub guild_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub team_id: Option<String>,
}

/// Bind an agent to a specific channel/peer pattern.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AgentBinding {
    pub agent_id: String,
    pub r#match: AgentBindingMatch,
}
