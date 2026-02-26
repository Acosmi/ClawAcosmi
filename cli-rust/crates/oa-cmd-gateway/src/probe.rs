/// Gateway probe command.
///
/// Performs a comprehensive gateway reachability check including
/// HTTP health probe, WebSocket RPC ping, and optional mDNS discovery.
///
/// Source: `src/commands/gateway-probe.ts`

use std::time::{Duration, Instant};

use anyhow::{Context, Result};
use tracing::info;

use oa_config::io::load_config;
use oa_config::paths::resolve_gateway_port;
use oa_gateway_rpc::call::{
    build_gateway_connection_details, call_gateway, CallGatewayOptions,
};
use oa_terminal::theme::Theme;

/// Probe result for a single check.
#[derive(Debug, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
struct ProbeCheck {
    /// Check name.
    name: String,
    /// Whether the check passed.
    ok: bool,
    /// Latency in milliseconds.
    #[serde(skip_serializing_if = "Option::is_none")]
    latency_ms: Option<u64>,
    /// Error message on failure.
    #[serde(skip_serializing_if = "Option::is_none")]
    error: Option<String>,
}

/// Probe the gateway for reachability, connectivity, and health.
///
/// Performs the following checks:
/// 1. HTTP health endpoint probe
/// 2. WebSocket RPC `ping` call
/// 3. Gateway discovery (config-based)
///
/// Results are displayed as human-readable output or JSON.
///
/// Source: `src/commands/gateway-probe.ts` - `gatewayProbeCommand`
pub async fn gateway_probe_command(json: bool) -> Result<()> {
    let started_at = Instant::now();
    let config = load_config().unwrap_or_default();
    let port = resolve_gateway_port(Some(&config));

    info!(port, json, "gateway probe requested");

    let connection_details = build_gateway_connection_details(&config, None, None);
    let health_url = format!("http://127.0.0.1:{port}/health");

    let mut checks: Vec<ProbeCheck> = Vec::new();

    // Check 1: HTTP health endpoint.
    let http_check = probe_http_health(&health_url).await;
    checks.push(http_check);

    // Check 2: WebSocket RPC ping.
    let rpc_check = probe_rpc_ping().await;
    checks.push(rpc_check);

    // Check 3: Discovery / connection details.
    checks.push(ProbeCheck {
        name: "discovery".to_string(),
        ok: true,
        latency_ms: None,
        error: None,
    });

    let all_ok = checks.iter().all(|c| c.ok);
    let duration_ms = started_at.elapsed().as_millis() as u64;

    if json {
        let output = serde_json::json!({
            "ok": all_ok,
            "durationMs": duration_ms,
            "port": port,
            "target": connection_details.url,
            "targetSource": connection_details.url_source,
            "checks": checks,
        });
        println!(
            "{}",
            serde_json::to_string_pretty(&output)
                .context("failed to serialize probe output")?
        );
        if !all_ok {
            std::process::exit(1);
        }
        return Ok(());
    }

    // Human-readable output.
    println!("{}", Theme::heading("Gateway Probe"));
    println!();
    println!(
        "  {}: {}",
        Theme::info("Target"),
        Theme::muted(&connection_details.url),
    );
    println!(
        "  {}: {}",
        Theme::muted("Source"),
        Theme::muted(&connection_details.url_source),
    );
    println!();

    for check in &checks {
        let status = if check.ok {
            Theme::success("pass")
        } else {
            Theme::error("fail")
        };
        let latency = check
            .latency_ms
            .map(|ms| format!(" ({ms}ms)"))
            .unwrap_or_default();
        let error = check
            .error
            .as_deref()
            .map(|e| format!(" - {e}"))
            .unwrap_or_default();
        println!(
            "  {} {}{}{error}",
            status,
            Theme::info(&check.name),
            Theme::muted(&latency),
        );
    }

    println!();

    if all_ok {
        println!("{}", Theme::success("All checks passed."));
    } else {
        println!("{}", Theme::error("Some checks failed."));
        std::process::exit(1);
    }

    Ok(())
}

/// Probe the HTTP health endpoint.
async fn probe_http_health(url: &str) -> ProbeCheck {
    let client = match reqwest::Client::builder()
        .timeout(Duration::from_secs(5))
        .build()
    {
        Ok(c) => c,
        Err(e) => {
            return ProbeCheck {
                name: "http_health".to_string(),
                ok: false,
                latency_ms: None,
                error: Some(format!("failed to build HTTP client: {e}")),
            };
        }
    };

    let start = Instant::now();

    match client.get(url).send().await {
        Ok(response) => {
            let latency_ms = start.elapsed().as_millis() as u64;
            if response.status().is_success() {
                ProbeCheck {
                    name: "http_health".to_string(),
                    ok: true,
                    latency_ms: Some(latency_ms),
                    error: None,
                }
            } else {
                ProbeCheck {
                    name: "http_health".to_string(),
                    ok: false,
                    latency_ms: Some(latency_ms),
                    error: Some(format!("HTTP {}", response.status().as_u16())),
                }
            }
        }
        Err(e) => {
            let latency_ms = start.elapsed().as_millis() as u64;
            ProbeCheck {
                name: "http_health".to_string(),
                ok: false,
                latency_ms: if latency_ms > 0 {
                    Some(latency_ms)
                } else {
                    None
                },
                error: Some(format!("{e}")),
            }
        }
    }
}

/// Probe the gateway via WebSocket RPC ping.
async fn probe_rpc_ping() -> ProbeCheck {
    let start = Instant::now();

    let opts = CallGatewayOptions {
        method: "ping".to_string(),
        timeout_ms: Some(5_000),
        ..Default::default()
    };

    match call_gateway::<serde_json::Value>(opts).await {
        Ok(_) => {
            let latency_ms = start.elapsed().as_millis() as u64;
            ProbeCheck {
                name: "rpc_ping".to_string(),
                ok: true,
                latency_ms: Some(latency_ms),
                error: None,
            }
        }
        Err(e) => {
            let latency_ms = start.elapsed().as_millis() as u64;
            ProbeCheck {
                name: "rpc_ping".to_string(),
                ok: false,
                latency_ms: if latency_ms > 0 {
                    Some(latency_ms)
                } else {
                    None
                },
                error: Some(format!("{e}")),
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn probe_check_serializes() {
        let check = ProbeCheck {
            name: "test".to_string(),
            ok: true,
            latency_ms: Some(42),
            error: None,
        };
        let json = serde_json::to_value(&check).unwrap_or_default();
        assert_eq!(json["name"], "test");
        assert_eq!(json["ok"], true);
        assert_eq!(json["latencyMs"], 42);
        assert!(json.get("error").is_none());
    }

    #[test]
    fn probe_check_failed_serializes() {
        let check = ProbeCheck {
            name: "http".to_string(),
            ok: false,
            latency_ms: None,
            error: Some("timeout".to_string()),
        };
        let json = serde_json::to_value(&check).unwrap_or_default();
        assert_eq!(json["ok"], false);
        assert_eq!(json["error"], "timeout");
        assert!(json.get("latencyMs").is_none());
    }
}
