/// Health check types for the health command.
///
/// Source: `src/commands/health.ts` types

use std::collections::HashMap;

use serde::{Deserialize, Serialize};

/// Summary of a channel account's health status.
///
/// Source: `src/commands/health.ts` - `ChannelAccountHealthSummary`
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct ChannelAccountHealthSummary {
    /// Account identifier.
    pub account_id: String,
    /// Whether the channel account is configured.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub configured: Option<bool>,
    /// Whether the channel account is linked.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub linked: Option<bool>,
    /// Authentication age in milliseconds.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub auth_age_ms: Option<f64>,
    /// Probe result (dynamic JSON structure).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub probe: Option<serde_json::Value>,
    /// Timestamp of the last probe.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub last_probe_at: Option<u64>,
    /// Additional dynamic fields.
    #[serde(flatten)]
    pub extra: HashMap<String, serde_json::Value>,
}

/// Summary of a channel's health status (includes per-account summaries).
///
/// Source: `src/commands/health.ts` - `ChannelHealthSummary`
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct ChannelHealthSummary {
    /// Default account identifier.
    #[serde(default)]
    pub account_id: String,
    /// Whether the channel is configured.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub configured: Option<bool>,
    /// Whether the channel is linked.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub linked: Option<bool>,
    /// Authentication age in milliseconds.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub auth_age_ms: Option<f64>,
    /// Probe result (dynamic JSON structure).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub probe: Option<serde_json::Value>,
    /// Timestamp of the last probe.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub last_probe_at: Option<u64>,
    /// Per-account health summaries.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub accounts: Option<HashMap<String, ChannelAccountHealthSummary>>,
    /// Additional dynamic fields.
    #[serde(flatten)]
    pub extra: HashMap<String, serde_json::Value>,
}

/// Heartbeat summary for an agent.
///
/// Source: `src/commands/health.ts` - `AgentHeartbeatSummary`
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct HeartbeatSummary {
    /// Heartbeat interval in milliseconds.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub every_ms: Option<u64>,
}

/// Session summary within health output.
///
/// Source: `src/commands/health.ts` - `HealthSummary["sessions"]`
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct SessionSummary {
    /// Path to the session store file.
    pub path: String,
    /// Total number of sessions.
    pub count: usize,
    /// Most recently active sessions.
    pub recent: Vec<RecentSession>,
}

/// A recently active session entry.
///
/// Source: `src/commands/health.ts` - `HealthSummary["sessions"]["recent"]`
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RecentSession {
    /// Session key.
    pub key: String,
    /// Last update timestamp (epoch ms), or null.
    pub updated_at: Option<u64>,
    /// Age in milliseconds since last update, or null.
    pub age: Option<u64>,
}

/// Health summary for a single agent.
///
/// Source: `src/commands/health.ts` - `AgentHealthSummary`
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AgentHealthSummary {
    /// Agent identifier.
    pub agent_id: String,
    /// Human-readable agent name.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    /// Whether this is the default agent.
    pub is_default: bool,
    /// Heartbeat configuration.
    pub heartbeat: HeartbeatSummary,
    /// Session store summary.
    pub sessions: SessionSummary,
}

/// Full health summary returned by the gateway.
///
/// Source: `src/commands/health.ts` - `HealthSummary`
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct HealthSummary {
    /// Always `true` when the gateway responds (indicates reachability).
    pub ok: bool,
    /// Timestamp of the health check (epoch ms).
    pub ts: u64,
    /// Duration of the health check in milliseconds.
    pub duration_ms: u64,
    /// Channel health summaries keyed by channel ID.
    #[serde(default)]
    pub channels: HashMap<String, ChannelHealthSummary>,
    /// Ordered list of channel IDs for display.
    #[serde(default)]
    pub channel_order: Vec<String>,
    /// Human-readable labels for channels.
    #[serde(default)]
    pub channel_labels: HashMap<String, String>,
    /// Default agent heartbeat interval in seconds (rounded).
    #[serde(default)]
    pub heartbeat_seconds: u64,
    /// Default agent identifier.
    #[serde(default)]
    pub default_agent_id: String,
    /// Per-agent health summaries.
    #[serde(default)]
    pub agents: Vec<AgentHealthSummary>,
    /// Session store summary.
    #[serde(default)]
    pub sessions: SessionSummary,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn health_summary_deserializes_minimal() {
        let json = r#"{
            "ok": true,
            "ts": 1700000000000,
            "durationMs": 150,
            "channels": {},
            "channelOrder": [],
            "channelLabels": {},
            "heartbeatSeconds": 30,
            "defaultAgentId": "main",
            "agents": [],
            "sessions": { "path": "/tmp/sessions.json", "count": 0, "recent": [] }
        }"#;
        let summary: HealthSummary =
            serde_json::from_str(json).expect("should deserialize");
        assert!(summary.ok);
        assert_eq!(summary.heartbeat_seconds, 30);
        assert_eq!(summary.default_agent_id, "main");
    }

    #[test]
    fn channel_health_summary_with_accounts() {
        let json = r#"{
            "accountId": "default",
            "configured": true,
            "linked": true,
            "accounts": {
                "default": {
                    "accountId": "default",
                    "configured": true,
                    "linked": true
                }
            }
        }"#;
        let summary: ChannelHealthSummary =
            serde_json::from_str(json).expect("should deserialize");
        assert_eq!(summary.linked, Some(true));
        assert!(summary.accounts.is_some());
    }

    #[test]
    fn agent_health_summary_roundtrip() {
        let agent = AgentHealthSummary {
            agent_id: "test".to_string(),
            name: Some("Test Agent".to_string()),
            is_default: true,
            heartbeat: HeartbeatSummary {
                every_ms: Some(30_000),
            },
            sessions: SessionSummary {
                path: "/tmp/sessions.json".to_string(),
                count: 5,
                recent: vec![],
            },
        };
        let json = serde_json::to_string(&agent).expect("should serialize");
        let parsed: AgentHealthSummary =
            serde_json::from_str(&json).expect("should deserialize");
        assert_eq!(parsed.agent_id, "test");
        assert_eq!(parsed.heartbeat.every_ms, Some(30_000));
    }

    #[test]
    fn channel_account_health_with_probe() {
        let json = r#"{
            "accountId": "bot1",
            "configured": true,
            "probe": { "ok": true, "elapsedMs": 42, "bot": { "username": "testbot" } },
            "lastProbeAt": 1700000000000
        }"#;
        let summary: ChannelAccountHealthSummary =
            serde_json::from_str(json).expect("should deserialize");
        assert_eq!(summary.account_id, "bot1");
        assert!(summary.probe.is_some());
    }
}
