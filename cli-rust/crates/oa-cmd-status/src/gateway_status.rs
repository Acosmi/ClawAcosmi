/// Gateway status command implementation.
///
/// Probes one or more gateway targets and displays their reachability,
/// latency, and configuration.
///
/// Source: `src/commands/gateway-status.ts`, `src/commands/gateway-status/helpers.ts`

use anyhow::Result;

use oa_config::io::load_config;
use oa_config::paths::resolve_gateway_port;
use oa_terminal::theme::Theme;
use oa_types::config::OpenAcosmiConfig;
use oa_types::gateway::GatewayMode;

use crate::format::GatewayProbeAuth;
use crate::gateway_probe::{pick_gateway_self_presence, GatewaySelfPresence};

/// Kind of a gateway status target.
///
/// Source: `src/commands/gateway-status/helpers.ts` - `TargetKind`
#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum TargetKind {
    /// Explicitly provided URL.
    Explicit,
    /// From `gateway.remote.url` config.
    ConfigRemote,
    /// Local loopback.
    LocalLoopback,
    /// SSH tunnel.
    SshTunnel,
}

/// A gateway status probe target.
///
/// Source: `src/commands/gateway-status/helpers.ts` - `GatewayStatusTarget`
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct GatewayStatusTarget {
    /// Target identifier.
    pub id: String,
    /// Target kind.
    pub kind: TargetKind,
    /// WebSocket URL.
    pub url: String,
    /// Whether this is the active target.
    pub active: bool,
}

/// Gateway config summary extracted from a config snapshot.
///
/// Source: `src/commands/gateway-status/helpers.ts` - `GatewayConfigSummary`
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct GatewayConfigSummary {
    /// Config path.
    pub path: Option<String>,
    /// Whether config exists.
    pub exists: bool,
    /// Whether config is valid.
    pub valid: bool,
    /// Gateway section summary.
    pub gateway: GatewayConfigGatewaySummary,
    /// Discovery section summary.
    pub discovery: GatewayConfigDiscoverySummary,
}

/// Gateway-specific config summary.
///
/// Source: `src/commands/gateway-status/helpers.ts` - `GatewayConfigSummary.gateway`
#[derive(Debug, Clone, Default, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct GatewayConfigGatewaySummary {
    /// Gateway mode.
    pub mode: Option<String>,
    /// Bind mode.
    pub bind: Option<String>,
    /// Port.
    pub port: Option<u16>,
    /// Control UI enabled.
    pub control_ui_enabled: Option<bool>,
    /// Tailscale mode.
    pub tailscale_mode: Option<String>,
}

/// Discovery-specific config summary.
///
/// Source: `src/commands/gateway-status/helpers.ts` - `GatewayConfigSummary.discovery`
#[derive(Debug, Clone, Default, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct GatewayConfigDiscoverySummary {
    /// Whether wide-area discovery is enabled.
    pub wide_area_enabled: Option<bool>,
}

/// Network hints for status display.
///
/// Source: `src/commands/gateway-status/helpers.ts` - `buildNetworkHints`
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct NetworkHints {
    /// Local loopback URL.
    pub local_loopback_url: String,
    /// Local tailnet URL.
    pub local_tailnet_url: Option<String>,
    /// Tailnet IPv4 address.
    pub tailnet_ipv4: Option<String>,
}

/// Probed target result.
///
/// Source: `src/commands/gateway-status.ts` - probed
#[derive(Debug, Clone)]
pub struct ProbedTarget {
    /// The target that was probed.
    pub target: GatewayStatusTarget,
    /// Whether the probe succeeded.
    pub ok: bool,
    /// Connection latency.
    pub connect_latency_ms: Option<u64>,
    /// Error on failure.
    pub error: Option<String>,
    /// Gateway self presence.
    pub self_presence: Option<GatewaySelfPresence>,
    /// Config summary from probe.
    pub config_summary: Option<GatewayConfigSummary>,
}

/// Warning from the gateway status check.
///
/// Source: `src/commands/gateway-status.ts` - warnings
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct GatewayWarning {
    /// Warning code.
    pub code: String,
    /// Warning message.
    pub message: String,
}

/// Parse a timeout value from raw input.
///
/// Source: `src/commands/gateway-status/helpers.ts` - `parseTimeoutMs`
#[must_use]
pub fn parse_timeout_ms(raw: Option<&str>, fallback: u64) -> u64 {
    let Some(value) = raw else {
        return fallback;
    };
    let trimmed = value.trim();
    if trimmed.is_empty() {
        return fallback;
    }
    trimmed.parse::<u64>().unwrap_or(fallback)
}

/// Resolve probe targets from config and optional URL override.
///
/// Source: `src/commands/gateway-status/helpers.ts` - `resolveTargets`
#[must_use]
pub fn resolve_targets(cfg: &OpenAcosmiConfig, explicit_url: Option<&str>) -> Vec<GatewayStatusTarget> {
    let mut targets: Vec<GatewayStatusTarget> = Vec::new();

    let add = |targets: &mut Vec<GatewayStatusTarget>, t: GatewayStatusTarget| {
        if !targets.iter().any(|x| x.url == t.url) {
            targets.push(t);
        }
    };

    // Explicit URL.
    if let Some(url) = explicit_url {
        let trimmed = url.trim();
        if trimmed.starts_with("ws://") || trimmed.starts_with("wss://") {
            add(
                &mut targets,
                GatewayStatusTarget {
                    id: "explicit".to_string(),
                    kind: TargetKind::Explicit,
                    url: trimmed.to_string(),
                    active: true,
                },
            );
        }
    }

    // Config remote URL.
    let remote_url = cfg
        .gateway
        .as_ref()
        .and_then(|g| g.remote.as_ref())
        .and_then(|r| r.url.as_deref())
        .map(str::trim)
        .filter(|s| s.starts_with("ws://") || s.starts_with("wss://"))
        .map(String::from);
    if let Some(ref url) = remote_url {
        let is_remote_active = cfg
            .gateway
            .as_ref()
            .and_then(|g| g.mode.as_ref())
            .is_some_and(|m| *m == GatewayMode::Remote);
        add(
            &mut targets,
            GatewayStatusTarget {
                id: "configRemote".to_string(),
                kind: TargetKind::ConfigRemote,
                url: url.clone(),
                active: is_remote_active,
            },
        );
    }

    // Local loopback.
    let port = resolve_gateway_port(Some(cfg));
    let is_local_active = cfg
        .gateway
        .as_ref()
        .and_then(|g| g.mode.as_ref())
        .map_or(true, |m| *m != GatewayMode::Remote);
    add(
        &mut targets,
        GatewayStatusTarget {
            id: "localLoopback".to_string(),
            kind: TargetKind::LocalLoopback,
            url: format!("ws://127.0.0.1:{port}"),
            active: is_local_active,
        },
    );

    targets
}

/// Resolve probe budget in ms for a target kind.
///
/// Source: `src/commands/gateway-status/helpers.ts` - `resolveProbeBudgetMs`
#[must_use]
pub fn resolve_probe_budget_ms(overall_ms: u64, kind: &TargetKind) -> u64 {
    match kind {
        TargetKind::LocalLoopback => overall_ms.min(800),
        TargetKind::SshTunnel => overall_ms.min(2000),
        _ => overall_ms.min(1500),
    }
}

/// Resolve authentication for a target.
///
/// Source: `src/commands/gateway-status/helpers.ts` - `resolveAuthForTarget`
#[must_use]
pub fn resolve_auth_for_target(
    cfg: &OpenAcosmiConfig,
    target: &GatewayStatusTarget,
    overrides: &GatewayProbeAuth,
) -> GatewayProbeAuth {
    // Explicit overrides take priority.
    let token_override = overrides
        .token
        .as_deref()
        .map(str::trim)
        .filter(|s| !s.is_empty())
        .map(String::from);
    let password_override = overrides
        .password
        .as_deref()
        .map(str::trim)
        .filter(|s| !s.is_empty())
        .map(String::from);

    if token_override.is_some() || password_override.is_some() {
        return GatewayProbeAuth {
            token: token_override,
            password: password_override,
        };
    }

    // Remote targets use remote config.
    if target.kind == TargetKind::ConfigRemote || target.kind == TargetKind::SshTunnel {
        let token = cfg
            .gateway
            .as_ref()
            .and_then(|g| g.remote.as_ref())
            .and_then(|r| r.token.as_deref())
            .map(str::trim)
            .filter(|s| !s.is_empty())
            .map(String::from);
        let password = cfg
            .gateway
            .as_ref()
            .and_then(|g| g.remote.as_ref())
            .and_then(|r| r.password.as_deref())
            .map(str::trim)
            .filter(|s| !s.is_empty())
            .map(String::from);
        return GatewayProbeAuth { token, password };
    }

    // Local targets use env + config auth.
    let env_token = std::env::var("OPENACOSMI_GATEWAY_TOKEN")
        .ok()
        .map(|s| s.trim().to_string())
        .filter(|s| !s.is_empty());
    let env_password = std::env::var("OPENACOSMI_GATEWAY_PASSWORD")
        .ok()
        .map(|s| s.trim().to_string())
        .filter(|s| !s.is_empty());
    let cfg_token = cfg
        .gateway
        .as_ref()
        .and_then(|g| g.auth.as_ref())
        .and_then(|a| a.token.as_deref())
        .map(str::trim)
        .filter(|s| !s.is_empty())
        .map(String::from);
    let cfg_password = cfg
        .gateway
        .as_ref()
        .and_then(|g| g.auth.as_ref())
        .and_then(|a| a.password.as_deref())
        .map(str::trim)
        .filter(|s| !s.is_empty())
        .map(String::from);

    GatewayProbeAuth {
        token: env_token.or(cfg_token),
        password: env_password.or(cfg_password),
    }
}

/// Render a target header line.
///
/// Source: `src/commands/gateway-status/helpers.ts` - `renderTargetHeader`
#[must_use]
pub fn render_target_header(target: &GatewayStatusTarget) -> String {
    let kind_label = match target.kind {
        TargetKind::LocalLoopback => "Local loopback",
        TargetKind::SshTunnel => "Remote over SSH",
        TargetKind::ConfigRemote => {
            if target.active {
                "Remote (configured)"
            } else {
                "Remote (configured, inactive)"
            }
        }
        TargetKind::Explicit => "URL (explicit)",
    };
    format!(
        "{} {}",
        Theme::heading(kind_label),
        Theme::muted(&target.url)
    )
}

/// Render a probe summary line.
///
/// Source: `src/commands/gateway-status/helpers.ts` - `renderProbeSummaryLine`
#[must_use]
pub fn render_probe_summary_line(probed: &ProbedTarget) -> String {
    if probed.ok {
        let latency = probed
            .connect_latency_ms
            .map_or("unknown".to_string(), |ms| format!("{ms}ms"));
        format!(
            "{} ({latency}) \u{00b7} {}",
            Theme::success("Connect: ok"),
            Theme::success("RPC: ok")
        )
    } else {
        let detail = probed
            .error
            .as_deref()
            .map_or(String::new(), |e| format!(" - {e}"));
        if probed.connect_latency_ms.is_some() {
            let latency = probed
                .connect_latency_ms
                .map_or("unknown".to_string(), |ms| format!("{ms}ms"));
            format!(
                "{} ({latency}) \u{00b7} {}{}",
                Theme::success("Connect: ok"),
                Theme::error("RPC: failed"),
                detail
            )
        } else {
            format!("{}{detail}", Theme::error("Connect: failed"))
        }
    }
}

/// Execute the gateway-status command.
///
/// Source: `src/commands/gateway-status.ts` - `gatewayStatusCommand`
pub async fn gateway_status_command(
    url: Option<&str>,
    token: Option<&str>,
    password: Option<&str>,
    timeout: Option<&str>,
    json: bool,
) -> Result<()> {
    let started_at = std::time::Instant::now();
    let cfg = load_config().unwrap_or_default();
    let overall_timeout_ms = parse_timeout_ms(timeout, 3000);

    let targets = resolve_targets(&cfg, url);

    let auth_overrides = GatewayProbeAuth {
        token: token.map(String::from),
        password: password.map(String::from),
    };

    // Probe each target.
    let mut probed: Vec<ProbedTarget> = Vec::new();
    for target in &targets {
        let auth = resolve_auth_for_target(&cfg, target, &auth_overrides);
        let timeout_budget = resolve_probe_budget_ms(overall_timeout_ms, &target.kind);

        let start = std::time::Instant::now();
        let opts = oa_gateway_rpc::call::CallGatewayOptions {
            url: Some(target.url.clone()),
            token: auth.token,
            password: auth.password,
            method: "ping".to_string(),
            timeout_ms: Some(timeout_budget),
            ..Default::default()
        };

        let result = oa_gateway_rpc::call::call_gateway::<serde_json::Value>(opts).await;
        let elapsed = start.elapsed().as_millis() as u64;

        match result {
            Ok(response) => {
                let presence = response.get("presence").cloned();
                let self_presence =
                    pick_gateway_self_presence(presence.as_ref());
                probed.push(ProbedTarget {
                    target: target.clone(),
                    ok: true,
                    connect_latency_ms: Some(elapsed),
                    error: None,
                    self_presence,
                    config_summary: None,
                });
            }
            Err(e) => {
                probed.push(ProbedTarget {
                    target: target.clone(),
                    ok: false,
                    connect_latency_ms: if elapsed > 0 { Some(elapsed) } else { None },
                    error: Some(format!("{e}")),
                    self_presence: None,
                    config_summary: None,
                });
            }
        }
    }

    let reachable: Vec<&ProbedTarget> = probed.iter().filter(|p| p.ok).collect();
    let ok = !reachable.is_empty();
    let multiple_gateways = reachable.len() > 1;

    let mut warnings: Vec<GatewayWarning> = Vec::new();
    if multiple_gateways {
        warnings.push(GatewayWarning {
            code: "multiple_gateways".to_string(),
            message: "Multiple reachable gateways detected.".to_string(),
        });
    }

    if json {
        let duration_ms = started_at.elapsed().as_millis() as u64;
        let output = serde_json::json!({
            "ok": ok,
            "ts": chrono::Utc::now().timestamp_millis(),
            "durationMs": duration_ms,
            "timeoutMs": overall_timeout_ms,
            "warnings": warnings,
            "targets": probed.iter().map(|p| {
                serde_json::json!({
                    "id": p.target.id,
                    "kind": p.target.kind,
                    "url": p.target.url,
                    "active": p.target.active,
                    "connect": {
                        "ok": p.ok,
                        "latencyMs": p.connect_latency_ms,
                        "error": p.error,
                    },
                    "self": p.self_presence,
                    "config": p.config_summary,
                })
            }).collect::<Vec<_>>(),
        });
        println!("{}", serde_json::to_string_pretty(&output).unwrap_or_default());
        if !ok {
            std::process::exit(1);
        }
        return Ok(());
    }

    // Rich output.
    println!("{}", Theme::heading("Gateway Status"));
    if ok {
        println!("{}: yes", Theme::success("Reachable"));
    } else {
        println!("{}: no", Theme::error("Reachable"));
    }
    println!(
        "{}",
        Theme::muted(&format!("Probe budget: {overall_timeout_ms}ms"))
    );

    if !warnings.is_empty() {
        println!();
        println!("{}", Theme::warn("Warning:"));
        for w in &warnings {
            println!("- {}", w.message);
        }
    }

    println!();
    println!("{}", Theme::heading("Targets"));
    for p in &probed {
        println!("{}", render_target_header(&p.target));
        println!("  {}", render_probe_summary_line(p));
        if p.ok {
            if let Some(ref self_pres) = p.self_presence {
                let host = self_pres.host.as_deref().unwrap_or("unknown");
                let ip = self_pres
                    .ip
                    .as_deref()
                    .map_or(String::new(), |ip| format!(" ({ip})"));
                let platform = self_pres
                    .platform
                    .as_deref()
                    .map_or(String::new(), |p| format!(" \u{00b7} {p}"));
                let version = self_pres
                    .version
                    .as_deref()
                    .map_or(String::new(), |v| format!(" \u{00b7} app {v}"));
                println!(
                    "  {}: {host}{ip}{platform}{version}",
                    Theme::info("Gateway")
                );
            }
        }
        println!();
    }

    if !ok {
        std::process::exit(1);
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_timeout_default() {
        assert_eq!(parse_timeout_ms(None, 3000), 3000);
        assert_eq!(parse_timeout_ms(Some(""), 3000), 3000);
    }

    #[test]
    fn parse_timeout_valid() {
        assert_eq!(parse_timeout_ms(Some("5000"), 3000), 5000);
    }

    #[test]
    fn parse_timeout_invalid() {
        assert_eq!(parse_timeout_ms(Some("abc"), 3000), 3000);
    }

    #[test]
    fn resolve_targets_default_config() {
        let cfg = OpenAcosmiConfig::default();
        let targets = resolve_targets(&cfg, None);
        assert_eq!(targets.len(), 1);
        assert_eq!(targets[0].kind, TargetKind::LocalLoopback);
        assert!(targets[0].active);
    }

    #[test]
    fn resolve_targets_with_explicit_url() {
        let cfg = OpenAcosmiConfig::default();
        let targets = resolve_targets(&cfg, Some("ws://example.com:9999"));
        assert!(targets.len() >= 2);
        assert_eq!(targets[0].kind, TargetKind::Explicit);
        assert_eq!(targets[0].url, "ws://example.com:9999");
    }

    #[test]
    fn resolve_targets_remote_mode() {
        let cfg = OpenAcosmiConfig {
            gateway: Some(oa_types::gateway::GatewayConfig {
                mode: Some(GatewayMode::Remote),
                remote: Some(oa_types::gateway::GatewayRemoteConfig {
                    url: Some("ws://remote:18789".to_string()),
                    ..Default::default()
                }),
                ..Default::default()
            }),
            ..Default::default()
        };
        let targets = resolve_targets(&cfg, None);
        assert!(targets.len() >= 2);
        let remote = targets.iter().find(|t| t.kind == TargetKind::ConfigRemote);
        assert!(remote.is_some());
        assert!(remote.unwrap_or(&targets[0]).active);
    }

    #[test]
    fn resolve_probe_budget_loopback() {
        assert_eq!(resolve_probe_budget_ms(3000, &TargetKind::LocalLoopback), 800);
    }

    #[test]
    fn resolve_probe_budget_ssh() {
        assert_eq!(resolve_probe_budget_ms(3000, &TargetKind::SshTunnel), 2000);
    }

    #[test]
    fn resolve_probe_budget_remote() {
        assert_eq!(
            resolve_probe_budget_ms(3000, &TargetKind::ConfigRemote),
            1500
        );
    }

    #[test]
    fn resolve_auth_explicit_overrides() {
        let cfg = OpenAcosmiConfig::default();
        let target = GatewayStatusTarget {
            id: "test".to_string(),
            kind: TargetKind::LocalLoopback,
            url: "ws://127.0.0.1:18789".to_string(),
            active: true,
        };
        let overrides = GatewayProbeAuth {
            token: Some("my-token".to_string()),
            password: None,
        };
        let auth = resolve_auth_for_target(&cfg, &target, &overrides);
        assert_eq!(auth.token.as_deref(), Some("my-token"));
    }

    #[test]
    fn render_target_header_loopback() {
        let target = GatewayStatusTarget {
            id: "localLoopback".to_string(),
            kind: TargetKind::LocalLoopback,
            url: "ws://127.0.0.1:18789".to_string(),
            active: true,
        };
        let header = render_target_header(&target);
        assert!(header.contains("Local loopback"));
    }

    #[test]
    fn render_probe_summary_ok() {
        let p = ProbedTarget {
            target: GatewayStatusTarget {
                id: "test".to_string(),
                kind: TargetKind::LocalLoopback,
                url: "ws://127.0.0.1:18789".to_string(),
                active: true,
            },
            ok: true,
            connect_latency_ms: Some(42),
            error: None,
            self_presence: None,
            config_summary: None,
        };
        let line = render_probe_summary_line(&p);
        assert!(line.contains("42ms"));
    }

    #[test]
    fn render_probe_summary_failed() {
        let p = ProbedTarget {
            target: GatewayStatusTarget {
                id: "test".to_string(),
                kind: TargetKind::LocalLoopback,
                url: "ws://127.0.0.1:18789".to_string(),
                active: true,
            },
            ok: false,
            connect_latency_ms: None,
            error: Some("connection refused".to_string()),
            self_presence: None,
            config_summary: None,
        };
        let line = render_probe_summary_line(&p);
        assert!(line.contains("failed"));
        assert!(line.contains("connection refused"));
    }

    #[test]
    fn gateway_warning_serializes() {
        let w = GatewayWarning {
            code: "test".to_string(),
            message: "test warning".to_string(),
        };
        let json = serde_json::to_value(&w).unwrap_or_default();
        assert_eq!(json["code"], "test");
    }

    #[test]
    fn target_kind_serializes() {
        let json = serde_json::to_string(&TargetKind::LocalLoopback).unwrap_or_default();
        assert_eq!(json, "\"localLoopback\"");
    }
}
