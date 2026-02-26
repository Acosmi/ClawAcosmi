/// Status scan orchestrator.
///
/// Collects all status information needed by the status command by probing
/// the gateway, loading config, checking agents, and building channel tables.
///
/// Source: `src/commands/status.scan.ts`

use anyhow::Result;

use oa_config::io::load_config;
use oa_gateway_rpc::call::{build_gateway_connection_details, GatewayConnectionDetails};
use oa_types::config::OpenAcosmiConfig;
use oa_types::gateway::GatewayMode;

use crate::agent_local::{get_agent_local_statuses, AgentStatusResult};
use crate::format::GatewayProbeAuth;
use crate::gateway_probe::{
    pick_gateway_self_presence, resolve_gateway_probe_auth, GatewaySelfPresence,
};
use crate::summary::get_status_summary_with_config;
use crate::types::StatusSummary;
use crate::update::UpdateCheckResult;

/// Memory plugin status.
///
/// Source: `src/commands/status.scan.ts` - `MemoryPluginStatus`
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct MemoryPluginStatus {
    /// Whether memory is enabled.
    pub enabled: bool,
    /// Plugin slot name.
    pub slot: Option<String>,
    /// Reason why memory is disabled.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub reason: Option<String>,
}

/// Channel row for display.
///
/// Source: `src/commands/status-all/channels.ts` - `ChannelRow`
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ChannelRow {
    /// Channel identifier.
    pub id: String,
    /// Display label.
    pub label: String,
    /// Whether the channel is enabled.
    pub enabled: bool,
    /// Channel state.
    pub state: String,
    /// Detail description.
    pub detail: String,
}

/// Channel table result.
///
/// Source: `src/commands/status-all/channels.ts` - `buildChannelsTable` return
#[derive(Debug, Clone, Default, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ChannelsTable {
    /// Channel rows.
    pub rows: Vec<ChannelRow>,
}

/// Complete status scan result.
///
/// Source: `src/commands/status.scan.ts` - `StatusScanResult`
pub struct StatusScanResult {
    /// Loaded configuration.
    pub cfg: OpenAcosmiConfig,
    /// OS summary label.
    pub os_summary_label: String,
    /// Tailscale mode string.
    pub tailscale_mode: String,
    /// Tailscale DNS name.
    pub tailscale_dns: Option<String>,
    /// Tailscale HTTPS URL.
    pub tailscale_https_url: Option<String>,
    /// Update check result.
    pub update: UpdateCheckResult,
    /// Gateway connection details.
    pub gateway_connection: GatewayConnectionDetails,
    /// Whether remote URL is missing in remote mode.
    pub remote_url_missing: bool,
    /// Gateway mode.
    pub gateway_mode: String,
    /// Gateway probe result.
    pub gateway_probe: Option<GatewayProbeResult>,
    /// Whether gateway is reachable.
    pub gateway_reachable: bool,
    /// Gateway self-identification.
    pub gateway_self: Option<GatewaySelfPresence>,
    /// Channel issues from gateway.
    pub channel_issues: Vec<ChannelIssue>,
    /// Agent status.
    pub agent_status: AgentStatusResult,
    /// Channels table.
    pub channels: ChannelsTable,
    /// Status summary.
    pub summary: StatusSummary,
    /// Memory plugin status.
    pub memory_plugin: MemoryPluginStatus,
}

/// Channel issue from gateway status.
///
/// Source: `src/infra/channels-status-issues.ts`
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ChannelIssue {
    /// Channel identifier.
    pub channel: String,
    /// Issue message.
    pub message: String,
}

/// Simplified gateway probe result.
///
/// Source: `src/gateway/probe.ts` - `GatewayProbeResult`
#[derive(Debug, Clone)]
pub struct GatewayProbeResult {
    /// Whether the probe succeeded.
    pub ok: bool,
    /// Connection latency in ms.
    pub connect_latency_ms: Option<u64>,
    /// Error message on failure.
    pub error: Option<String>,
    /// Presence entries.
    pub presence: Option<serde_json::Value>,
}

/// Resolve memory plugin status from configuration.
///
/// Source: `src/commands/status.scan.ts` - `resolveMemoryPluginStatus`
#[must_use]
pub fn resolve_memory_plugin_status(cfg: &OpenAcosmiConfig) -> MemoryPluginStatus {
    let plugins_enabled = cfg
        .plugins
        .as_ref()
        .and_then(|p| p.enabled)
        .unwrap_or(true);

    if !plugins_enabled {
        return MemoryPluginStatus {
            enabled: false,
            slot: None,
            reason: Some("plugins disabled".to_string()),
        };
    }

    let raw = cfg
        .plugins
        .as_ref()
        .and_then(|p| p.slots.as_ref())
        .and_then(|s| s.memory.as_deref())
        .unwrap_or("")
        .trim()
        .to_string();

    if raw.eq_ignore_ascii_case("none") {
        return MemoryPluginStatus {
            enabled: false,
            slot: None,
            reason: Some("plugins.slots.memory=\"none\"".to_string()),
        };
    }

    let slot = if raw.is_empty() {
        "memory-core".to_string()
    } else {
        raw
    };

    MemoryPluginStatus {
        enabled: true,
        slot: Some(slot),
        reason: None,
    }
}

/// Resolve the OS summary label for the current platform.
///
/// Source: `src/infra/os-summary.ts` - `resolveOsSummary`
#[must_use]
pub fn resolve_os_summary_label() -> String {
    let os = std::env::consts::OS;
    let arch = std::env::consts::ARCH;
    format!("{os} {arch}")
}

/// Run the full status scan.
///
/// Source: `src/commands/status.scan.ts` - `scanStatus`
pub async fn scan_status(
    _json: bool,
    _timeout_ms: Option<u64>,
    _all: bool,
) -> Result<StatusScanResult> {
    let cfg = load_config().unwrap_or_default();
    let os_summary_label = resolve_os_summary_label();

    // Tailscale (stub: not probing in Rust implementation).
    let tailscale_mode = cfg
        .gateway
        .as_ref()
        .and_then(|g| g.tailscale.as_ref())
        .and_then(|t| t.mode.as_ref())
        .map_or("off".to_string(), |m| format!("{m:?}").to_lowercase());
    let tailscale_dns: Option<String> = None;
    let tailscale_https_url: Option<String> = None;

    // Update check (stub: return defaults).
    let update = UpdateCheckResult {
        install_kind: "unknown".to_string(),
        package_manager: "unknown".to_string(),
        ..Default::default()
    };

    // Agent status.
    let agent_status = get_agent_local_statuses(&cfg);

    // Gateway connection.
    let gateway_connection = build_gateway_connection_details(&cfg, None, None);
    let is_remote = cfg
        .gateway
        .as_ref()
        .and_then(|g| g.mode.as_ref())
        .is_some_and(|m| *m == GatewayMode::Remote);
    let remote_url_raw = cfg
        .gateway
        .as_ref()
        .and_then(|g| g.remote.as_ref())
        .and_then(|r| r.url.as_deref())
        .unwrap_or("")
        .trim();
    let remote_url_missing = is_remote && remote_url_raw.is_empty();
    let gateway_mode = if is_remote { "remote" } else { "local" }.to_string();

    // Gateway probe.
    let probe_auth = resolve_gateway_probe_auth(&cfg);
    let timeout = _timeout_ms.unwrap_or(10_000).min(if _all { 5000 } else { 2500 });
    let gateway_probe = if remote_url_missing {
        None
    } else {
        probe_gateway_simple(&gateway_connection.url, &probe_auth, timeout).await
    };
    let gateway_reachable = gateway_probe
        .as_ref()
        .is_some_and(|p| p.ok);
    let gateway_self = gateway_probe
        .as_ref()
        .and_then(|p| p.presence.as_ref())
        .and_then(|pres| pick_gateway_self_presence(Some(pres)));

    // Channels (stub: empty table since channel plugins are TS-only).
    let channels = ChannelsTable::default();
    let channel_issues: Vec<ChannelIssue> = vec![];

    // Memory plugin.
    let memory_plugin = resolve_memory_plugin_status(&cfg);

    // Summary.
    let summary = get_status_summary_with_config(&cfg);

    Ok(StatusScanResult {
        cfg,
        os_summary_label,
        tailscale_mode,
        tailscale_dns,
        tailscale_https_url,
        update,
        gateway_connection,
        remote_url_missing,
        gateway_mode,
        gateway_probe,
        gateway_reachable,
        gateway_self,
        channel_issues,
        agent_status,
        channels,
        summary,
        memory_plugin,
    })
}

/// Simple gateway probe via RPC.
///
/// Source: `src/gateway/probe.ts` - `probeGateway`
async fn probe_gateway_simple(
    url: &str,
    auth: &GatewayProbeAuth,
    timeout_ms: u64,
) -> Option<GatewayProbeResult> {
    let start = std::time::Instant::now();

    let opts = oa_gateway_rpc::call::CallGatewayOptions {
        url: Some(url.to_string()),
        token: auth.token.clone(),
        password: auth.password.clone(),
        method: "ping".to_string(),
        timeout_ms: Some(timeout_ms),
        ..Default::default()
    };

    match oa_gateway_rpc::call::call_gateway::<serde_json::Value>(opts).await {
        Ok(response) => {
            let elapsed = start.elapsed().as_millis() as u64;
            let presence = response.get("presence").cloned();
            Some(GatewayProbeResult {
                ok: true,
                connect_latency_ms: Some(elapsed),
                error: None,
                presence,
            })
        }
        Err(e) => {
            let elapsed = start.elapsed().as_millis() as u64;
            Some(GatewayProbeResult {
                ok: false,
                connect_latency_ms: if elapsed > 0 { Some(elapsed) } else { None },
                error: Some(format!("{e}")),
                presence: None,
            })
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn resolve_memory_plugin_default() {
        let cfg = OpenAcosmiConfig::default();
        let status = resolve_memory_plugin_status(&cfg);
        assert!(status.enabled);
        assert_eq!(status.slot.as_deref(), Some("memory-core"));
    }

    #[test]
    fn resolve_memory_plugin_disabled() {
        let cfg = OpenAcosmiConfig {
            plugins: Some(oa_types::plugins::PluginsConfig {
                enabled: Some(false),
                ..Default::default()
            }),
            ..Default::default()
        };
        let status = resolve_memory_plugin_status(&cfg);
        assert!(!status.enabled);
        assert_eq!(status.reason.as_deref(), Some("plugins disabled"));
    }

    #[test]
    fn os_summary_label_not_empty() {
        let label = resolve_os_summary_label();
        assert!(!label.is_empty());
    }

    #[test]
    fn channel_row_serialization() {
        let row = ChannelRow {
            id: "slack".to_string(),
            label: "Slack".to_string(),
            enabled: true,
            state: "ok".to_string(),
            detail: "configured".to_string(),
        };
        let json = serde_json::to_value(&row).unwrap_or_default();
        assert_eq!(json["id"], "slack");
        assert_eq!(json["enabled"], true);
    }

    #[test]
    fn channel_issue_serialization() {
        let issue = ChannelIssue {
            channel: "discord".to_string(),
            message: "token expired".to_string(),
        };
        let json = serde_json::to_value(&issue).unwrap_or_default();
        assert_eq!(json["channel"], "discord");
    }

    #[test]
    fn memory_plugin_status_serialization() {
        let s = MemoryPluginStatus {
            enabled: true,
            slot: Some("memory-core".to_string()),
            reason: None,
        };
        let json = serde_json::to_value(&s).unwrap_or_default();
        assert_eq!(json["enabled"], true);
        assert_eq!(json["slot"], "memory-core");
        assert!(json.get("reason").is_none());
    }
}
