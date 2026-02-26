/// Approvals configuration types.
///
/// Source: `src/config/types.approvals.ts`

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum ExecApprovalForwardingMode {
    Session,
    Targets,
    Both,
}

/// Thread ID can be a string or number.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(untagged)]
pub enum ThreadId {
    String(String),
    Number(i64),
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ExecApprovalForwardTarget {
    pub channel: String,
    pub to: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub account_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub thread_id: Option<ThreadId>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct ExecApprovalForwardingConfig {
    /// Enable forwarding exec approvals to chat channels. Default: false.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mode: Option<ExecApprovalForwardingMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub agent_filter: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub session_filter: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub targets: Option<Vec<ExecApprovalForwardTarget>>,
}

/// Top-level approvals configuration.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct ApprovalsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub exec: Option<ExecApprovalForwardingConfig>,
}
