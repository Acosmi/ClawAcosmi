/// Daemon installation sub-wizard for the configure command.
///
/// Handles prompting for daemon service installation, restart, reinstall,
/// and runtime selection when configuring the background gateway service.
///
/// Source: `src/commands/configure.daemon.ts`

use serde::{Deserialize, Serialize};

/// Gateway daemon runtime options.
///
/// Source: `src/commands/daemon-runtime.ts` - `GatewayDaemonRuntime`
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum GatewayDaemonRuntime {
    /// Node.js runtime.
    Node,
    /// Bun runtime.
    Bun,
}

impl std::fmt::Display for GatewayDaemonRuntime {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Node => write!(f, "node"),
            Self::Bun => write!(f, "bun"),
        }
    }
}

/// Default gateway daemon runtime.
///
/// Source: `src/commands/daemon-runtime.ts` - `DEFAULT_GATEWAY_DAEMON_RUNTIME`
pub const DEFAULT_GATEWAY_DAEMON_RUNTIME: GatewayDaemonRuntime = GatewayDaemonRuntime::Node;

/// Action to take when a daemon service is already installed.
///
/// Source: `src/commands/configure.daemon.ts` - service action options
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum DaemonServiceAction {
    /// Restart the existing service.
    Restart,
    /// Uninstall and reinstall the service.
    Reinstall,
    /// Skip daemon installation.
    Skip,
}

/// Parameters for the daemon installation process.
///
/// Source: `src/commands/configure.daemon.ts` - `maybeInstallDaemon` params
pub struct DaemonInstallParams {
    /// Gateway port to bind the daemon to.
    pub port: u16,
    /// Optional gateway token for auth.
    pub gateway_token: Option<String>,
    /// Optional daemon runtime override; defaults to `DEFAULT_GATEWAY_DAEMON_RUNTIME`.
    pub daemon_runtime: Option<GatewayDaemonRuntime>,
}

/// Resolve the effective daemon runtime, using the override or the default.
///
/// Source: `src/commands/configure.daemon.ts` - runtime resolution logic
pub fn resolve_daemon_runtime(
    override_runtime: Option<GatewayDaemonRuntime>,
) -> GatewayDaemonRuntime {
    override_runtime.unwrap_or(DEFAULT_GATEWAY_DAEMON_RUNTIME)
}

/// Runtime option descriptor for interactive selection.
///
/// Source: `src/commands/daemon-runtime.ts` - `GATEWAY_DAEMON_RUNTIME_OPTIONS`
pub struct DaemonRuntimeOption {
    /// The runtime value.
    pub value: GatewayDaemonRuntime,
    /// Display label.
    pub label: &'static str,
}

/// Available daemon runtime options for interactive selection.
///
/// Source: `src/commands/daemon-runtime.ts` - `GATEWAY_DAEMON_RUNTIME_OPTIONS`
pub const DAEMON_RUNTIME_OPTIONS: &[DaemonRuntimeOption] = &[
    DaemonRuntimeOption {
        value: GatewayDaemonRuntime::Node,
        label: "Node.js",
    },
    DaemonRuntimeOption {
        value: GatewayDaemonRuntime::Bun,
        label: "Bun",
    },
];

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn default_runtime_is_node() {
        assert_eq!(DEFAULT_GATEWAY_DAEMON_RUNTIME, GatewayDaemonRuntime::Node);
    }

    #[test]
    fn resolve_daemon_runtime_with_override() {
        let runtime = resolve_daemon_runtime(Some(GatewayDaemonRuntime::Bun));
        assert_eq!(runtime, GatewayDaemonRuntime::Bun);
    }

    #[test]
    fn resolve_daemon_runtime_without_override() {
        let runtime = resolve_daemon_runtime(None);
        assert_eq!(runtime, DEFAULT_GATEWAY_DAEMON_RUNTIME);
    }

    #[test]
    fn daemon_runtime_display() {
        assert_eq!(GatewayDaemonRuntime::Node.to_string(), "node");
        assert_eq!(GatewayDaemonRuntime::Bun.to_string(), "bun");
    }

    #[test]
    fn daemon_runtime_options_count() {
        assert_eq!(DAEMON_RUNTIME_OPTIONS.len(), 2);
    }

    #[test]
    fn daemon_service_action_variants() {
        assert_ne!(DaemonServiceAction::Restart, DaemonServiceAction::Skip);
        assert_ne!(DaemonServiceAction::Reinstall, DaemonServiceAction::Restart);
    }
}
