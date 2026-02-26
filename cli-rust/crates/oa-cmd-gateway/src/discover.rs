/// Gateway discover command.
///
/// Discovers gateway instances on the local network using mDNS/Bonjour
/// service browsing, and displays found instances.
///
/// Source: `src/commands/gateway-discover.ts`

use std::time::Instant;

use anyhow::{Context, Result};
use tracing::info;

use oa_config::io::load_config;
use oa_config::paths::resolve_gateway_port;
use oa_gateway_rpc::call::{build_gateway_connection_details, call_gateway, CallGatewayOptions};
use oa_terminal::theme::Theme;

/// A discovered gateway instance.
#[derive(Debug, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
struct DiscoveredGateway {
    /// Display name or hostname.
    name: String,
    /// WebSocket URL.
    url: String,
    /// How the gateway was discovered.
    source: String,
    /// Whether a ping was successful.
    reachable: bool,
    /// Ping latency in milliseconds.
    #[serde(skip_serializing_if = "Option::is_none")]
    latency_ms: Option<u64>,
    /// Gateway version (if reachable).
    #[serde(skip_serializing_if = "Option::is_none")]
    version: Option<String>,
}

/// Discover gateways on the local network.
///
/// Currently discovers the local gateway via config and probes it. In the
/// future this will be extended with mDNS/Bonjour-based LAN discovery.
///
/// Source: `src/commands/gateway-discover.ts` - `gatewayDiscoverCommand`
pub async fn gateway_discover_command(json: bool) -> Result<()> {
    let started_at = Instant::now();

    info!(json, "gateway discover requested");

    let config = load_config().unwrap_or_default();
    let port = resolve_gateway_port(Some(&config));
    let connection_details = build_gateway_connection_details(&config, None, None);

    let mut discovered: Vec<DiscoveredGateway> = Vec::new();

    // Discover the local gateway.
    let local_url = format!("ws://127.0.0.1:{port}");
    let local_result = probe_gateway_for_discovery(&local_url).await;
    discovered.push(DiscoveredGateway {
        name: "localhost".to_string(),
        url: local_url,
        source: "local config".to_string(),
        reachable: local_result.is_ok(),
        latency_ms: local_result.as_ref().ok().map(|(ms, _)| *ms),
        version: local_result.ok().and_then(|(_, v)| v),
    });

    // If config points to a different URL, probe that too.
    if connection_details.url != format!("ws://127.0.0.1:{port}") {
        let remote_result = probe_gateway_for_discovery(&connection_details.url).await;
        discovered.push(DiscoveredGateway {
            name: "configured".to_string(),
            url: connection_details.url.clone(),
            source: connection_details.url_source.clone(),
            reachable: remote_result.is_ok(),
            latency_ms: remote_result.as_ref().ok().map(|(ms, _)| *ms),
            version: remote_result.ok().and_then(|(_, v)| v),
        });
    }

    // TODO: mDNS/Bonjour discovery would go here.
    // Future implementation: browse for _openacosmi._tcp services on the
    // local network and probe each discovered instance.

    let duration_ms = started_at.elapsed().as_millis() as u64;
    let reachable_count = discovered.iter().filter(|d| d.reachable).count();

    if json {
        let output = serde_json::json!({
            "durationMs": duration_ms,
            "gateways": discovered,
            "total": discovered.len(),
            "reachable": reachable_count,
        });
        println!(
            "{}",
            serde_json::to_string_pretty(&output)
                .context("failed to serialize discover output")?
        );
        return Ok(());
    }

    // Human-readable output.
    println!("{}", Theme::heading("Gateway Discovery"));
    println!();

    if discovered.is_empty() {
        println!("  {}", Theme::muted("No gateways discovered."));
    } else {
        for gw in &discovered {
            let status = if gw.reachable {
                Theme::success("reachable")
            } else {
                Theme::error("unreachable")
            };
            let latency = gw
                .latency_ms
                .map(|ms| format!(" ({ms}ms)"))
                .unwrap_or_default();
            let version = gw
                .version
                .as_deref()
                .map(|v| format!(" v{v}"))
                .unwrap_or_default();

            println!(
                "  {} {}{}{}",
                status,
                Theme::info(&gw.name),
                Theme::muted(&latency),
                Theme::muted(&version),
            );
            println!(
                "    {}: {}",
                Theme::muted("URL"),
                Theme::muted(&gw.url),
            );
            println!(
                "    {}: {}",
                Theme::muted("Source"),
                Theme::muted(&gw.source),
            );
            println!();
        }

        println!(
            "  {} {} found, {} reachable",
            Theme::info("Total:"),
            discovered.len(),
            reachable_count,
        );
    }

    println!();

    Ok(())
}

/// Probe a gateway for discovery -- returns (latency_ms, version).
async fn probe_gateway_for_discovery(url: &str) -> Result<(u64, Option<String>)> {
    let start = Instant::now();

    let opts = CallGatewayOptions {
        url: Some(url.to_string()),
        method: "ping".to_string(),
        timeout_ms: Some(3_000),
        ..Default::default()
    };

    let result: serde_json::Value = call_gateway(opts)
        .await
        .context("discovery ping failed")?;

    let latency_ms = start.elapsed().as_millis() as u64;

    let version = result
        .get("version")
        .and_then(|v| v.as_str())
        .map(String::from);

    Ok((latency_ms, version))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn discovered_gateway_serializes() {
        let gw = DiscoveredGateway {
            name: "test".to_string(),
            url: "ws://127.0.0.1:18789".to_string(),
            source: "local".to_string(),
            reachable: true,
            latency_ms: Some(5),
            version: Some("1.0.0".to_string()),
        };
        let json = serde_json::to_value(&gw).unwrap_or_default();
        assert_eq!(json["name"], "test");
        assert_eq!(json["reachable"], true);
        assert_eq!(json["latencyMs"], 5);
        assert_eq!(json["version"], "1.0.0");
    }

    #[test]
    fn discovered_gateway_unreachable_serializes() {
        let gw = DiscoveredGateway {
            name: "test".to_string(),
            url: "ws://10.0.0.1:18789".to_string(),
            source: "mdns".to_string(),
            reachable: false,
            latency_ms: None,
            version: None,
        };
        let json = serde_json::to_value(&gw).unwrap_or_default();
        assert_eq!(json["reachable"], false);
        assert!(json.get("latencyMs").is_none());
        assert!(json.get("version").is_none());
    }
}
