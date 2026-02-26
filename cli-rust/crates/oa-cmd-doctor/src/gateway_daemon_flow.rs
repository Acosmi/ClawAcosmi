/// Gateway daemon repair flow.
///
/// When the gateway health check fails, this module tries to:
/// - Repair macOS LaunchAgent bootstrap issues
/// - Inspect port conflicts
/// - Offer to install / start / restart the gateway service
///
/// Source: `src/commands/doctor-gateway-daemon-flow.ts`

use oa_terminal::note::note;
use oa_types::config::OpenAcosmiConfig;
use oa_types::gateway::GatewayMode;

use crate::format::{build_gateway_runtime_hints, format_gateway_runtime_summary, RuntimeHintOptions};
use crate::prompter::{DoctorOptions, DoctorPrompter};

/// Attempt to repair a macOS LaunchAgent bootstrap issue.
///
/// Returns `true` if the repair was performed and verified.
///
/// Source: `src/commands/doctor-gateway-daemon-flow.ts` — `maybeRepairLaunchAgentBootstrap`
async fn maybe_repair_launch_agent_bootstrap(
    _title: &str,
    _prompter: &DoctorPrompter,
) -> bool {
    if std::env::consts::OS != "macos" {
        return false;
    }
    // Stub: LaunchAgent bootstrap repair is platform-specific and requires
    // launchctl calls.  Full implementation delegates to `oa-daemon::launchd`.
    false
}

/// Main gateway daemon repair entry point.
///
/// Called after the health check; tries to diagnose and fix gateway issues.
///
/// Source: `src/commands/doctor-gateway-daemon-flow.ts` — `maybeRepairGatewayDaemon`
pub async fn maybe_repair_gateway_daemon(
    cfg: &OpenAcosmiConfig,
    prompter: &mut DoctorPrompter,
    _options: &DoctorOptions,
    health_ok: bool,
) {
    if health_ok {
        return;
    }

    let mode = cfg
        .gateway
        .as_ref()
        .and_then(|gw| gw.mode.clone())
        .unwrap_or(GatewayMode::Local);

    // Stub: in the full implementation, we would query the service loader here.
    let loaded = false;

    // ── macOS LaunchAgent repair ──
    if std::env::consts::OS == "macos" && mode != GatewayMode::Remote {
        let _gateway_repaired = maybe_repair_launch_agent_bootstrap("Gateway", prompter).await;
        let _node_repaired = maybe_repair_launch_agent_bootstrap("Node", prompter).await;
    }

    if !loaded {
        // ── Linux systemd availability ──
        if std::env::consts::OS == "linux" {
            note(
                "Gateway service not installed. Systemd user service setup required.",
                Some("Gateway"),
            );
            return;
        }

        note("Gateway service not installed.", Some("Gateway"));

        if mode != GatewayMode::Remote {
            let install = prompter
                .confirm_skip_in_non_interactive("Install gateway service now?", true)
                .await;
            if install {
                // Stub: delegate to `oa-daemon::service::install`.
                note(
                    "Gateway service installation is not yet implemented in the Rust port.",
                    Some("Gateway"),
                );
            }
        }
        return;
    }

    // ── Show runtime summary and hints ──
    let summary = format_gateway_runtime_summary(None);
    let hints = build_gateway_runtime_hints(None, &RuntimeHintOptions::default());
    if summary.is_some() || !hints.is_empty() {
        let mut lines = Vec::new();
        if let Some(ref s) = summary {
            lines.push(format!("Runtime: {s}"));
        }
        lines.extend(hints);
        note(&lines.join("\n"), Some("Gateway"));
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn repair_noop_when_healthy() {
        let cfg = OpenAcosmiConfig::default();
        let options = DoctorOptions::default();
        let mut prompter = crate::prompter::create_doctor_prompter(&options);
        // Should return immediately without side effects.
        maybe_repair_gateway_daemon(&cfg, &mut prompter, &options, true).await;
    }
}
