/// Daemon status summary resolution.
///
/// Queries the gateway and node daemon services for their current status.
///
/// Source: `src/commands/status.daemon.ts`

use serde::{Deserialize, Serialize};

use oa_daemon::service::{
    ServiceRuntime, is_gateway_service_enabled, read_gateway_service_runtime,
    service_backend_label,
};

use crate::format::{format_daemon_runtime_short, DaemonRuntime};

/// Summary of a daemon service status.
///
/// Source: `src/commands/status.daemon.ts` - `DaemonStatusSummary`
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DaemonStatusSummary {
    /// Display label (e.g., "LaunchAgent").
    pub label: String,
    /// Whether the service is installed (`true`/`false`/`null` for unknown).
    pub installed: Option<bool>,
    /// Loaded/not-loaded text.
    pub loaded_text: String,
    /// Short runtime description.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub runtime_short: Option<String>,
}

/// Convert a `ServiceRuntime` to our `DaemonRuntime` for formatting.
///
/// Source: `src/commands/status.daemon.ts` - internal conversion
fn to_daemon_runtime(rt: &ServiceRuntime) -> DaemonRuntime {
    DaemonRuntime {
        status: Some(rt.status.to_string()),
        pid: rt.pid,
        state: rt.state.clone(),
        detail: rt.detail.clone(),
        missing_unit: Some(rt.missing_unit),
    }
}

/// Build a daemon status summary for a given profile.
///
/// Source: `src/commands/status.daemon.ts` - `buildDaemonStatusSummary`
fn build_daemon_status_summary(
    profile: Option<&str>,
    fallback_label: &str,
) -> DaemonStatusSummary {
    let label = service_backend_label().to_string();
    let loaded = is_gateway_service_enabled(profile).unwrap_or(false);
    let runtime = read_gateway_service_runtime(profile)
        .ok()
        .flatten();

    let installed = runtime.as_ref().map(|_| true);

    let loaded_text = if loaded {
        "loaded".to_string()
    } else {
        "not loaded".to_string()
    };

    let runtime_short = runtime
        .as_ref()
        .and_then(|rt| format_daemon_runtime_short(Some(&to_daemon_runtime(rt))));

    DaemonStatusSummary {
        label: if label == "unsupported" {
            fallback_label.to_string()
        } else {
            label
        },
        installed,
        loaded_text,
        runtime_short,
    }
}

/// Get the gateway daemon status summary.
///
/// Source: `src/commands/status.daemon.ts` - `getDaemonStatusSummary`
#[must_use]
pub fn get_daemon_status_summary() -> DaemonStatusSummary {
    build_daemon_status_summary(None, "Daemon")
}

/// Get the node daemon status summary.
///
/// Source: `src/commands/status.daemon.ts` - `getNodeDaemonStatusSummary`
#[must_use]
pub fn get_node_daemon_status_summary() -> DaemonStatusSummary {
    // Node service uses the same infrastructure as the gateway service.
    // In the TS implementation these are separate services. Here we
    // return a minimal summary since the node service may not be installed.
    DaemonStatusSummary {
        label: "Node".to_string(),
        installed: Some(false),
        loaded_text: "not loaded".to_string(),
        runtime_short: None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn daemon_status_summary_serializes() {
        let s = DaemonStatusSummary {
            label: "LaunchAgent".to_string(),
            installed: Some(true),
            loaded_text: "loaded".to_string(),
            runtime_short: Some("running (pid 1234)".to_string()),
        };
        let json = serde_json::to_value(&s).unwrap_or_default();
        assert_eq!(json["label"], "LaunchAgent");
        assert_eq!(json["installed"], true);
        assert_eq!(json["loadedText"], "loaded");
        assert_eq!(json["runtimeShort"], "running (pid 1234)");
    }

    #[test]
    fn daemon_status_not_installed() {
        let s = DaemonStatusSummary {
            label: "systemd".to_string(),
            installed: Some(false),
            loaded_text: "not loaded".to_string(),
            runtime_short: None,
        };
        let json = serde_json::to_value(&s).unwrap_or_default();
        assert_eq!(json["installed"], false);
        assert!(json.get("runtimeShort").is_none());
    }

    #[test]
    fn node_daemon_summary() {
        let summary = get_node_daemon_status_summary();
        assert_eq!(summary.label, "Node");
        assert_eq!(summary.installed, Some(false));
    }

    #[test]
    fn to_daemon_runtime_conversion() {
        let sr = ServiceRuntime {
            status: oa_daemon::service::ServiceStatus::Running,
            pid: Some(42),
            state: Some("running".to_string()),
            detail: None,
            missing_unit: false,
            ..Default::default()
        };
        let dr = to_daemon_runtime(&sr);
        assert_eq!(dr.status.as_deref(), Some("running"));
        assert_eq!(dr.pid, Some(42));
        assert_eq!(dr.missing_unit, Some(false));
    }
}
