/// Gateway status command.
///
/// Displays the current gateway service status by querying the
/// platform-specific daemon service manager and optionally probing
/// the gateway health endpoint via HTTP.
///
/// Source: `src/commands/gateway-status.ts`

use std::time::Duration;

use anyhow::{Context, Result};
use tracing::info;

use oa_config::io::load_config;
use oa_config::paths::resolve_gateway_port;
use oa_daemon::service::{
    ServiceStatus, read_gateway_service_runtime, service_backend_label,
};
use oa_terminal::theme::Theme;

/// Show gateway service status and probe the health endpoint.
///
/// Queries the daemon service manager for the service runtime state,
/// then makes an HTTP GET request to the gateway health endpoint.
/// Output can be rendered as human-readable text or JSON.
///
/// Source: `src/commands/gateway-status.ts` - `gatewayStatusCommand`
pub async fn gateway_status_command(json: bool) -> Result<()> {
    let config = load_config().unwrap_or_default();
    let port = resolve_gateway_port(Some(&config));
    let backend_label = service_backend_label();

    info!(
        port,
        backend = backend_label,
        json,
        "gateway status requested"
    );

    // Read daemon service runtime.
    let runtime = read_gateway_service_runtime(None)
        .context("failed to read gateway service runtime")?;

    let service_status = runtime
        .as_ref()
        .map(|rt| rt.status)
        .unwrap_or(ServiceStatus::Unknown);
    let service_pid = runtime.as_ref().and_then(|rt| rt.pid);

    // Probe the gateway health endpoint via HTTP.
    let health_url = format!("http://127.0.0.1:{port}/health");
    let health_result = probe_health_endpoint(&health_url).await;

    let health_ok = health_result.is_ok();
    let health_latency_ms = health_result.as_ref().ok().copied();
    let health_error = health_result.err().map(|e| format!("{e}"));

    if json {
        let output = serde_json::json!({
            "service": {
                "backend": backend_label,
                "status": service_status.to_string(),
                "pid": service_pid,
            },
            "health": {
                "ok": health_ok,
                "url": health_url,
                "latencyMs": health_latency_ms,
                "error": health_error,
            },
            "port": port,
        });
        println!(
            "{}",
            serde_json::to_string_pretty(&output)
                .context("failed to serialize status output")?
        );
        return Ok(());
    }

    // Human-readable output.
    println!("{}", Theme::heading("Gateway Status"));
    println!();

    // Service section.
    println!(
        "  {}: {} ({})",
        Theme::info("Service"),
        match service_status {
            ServiceStatus::Running => Theme::success("running"),
            ServiceStatus::Stopped => Theme::error("stopped"),
            ServiceStatus::Unknown => Theme::muted("unknown"),
        },
        Theme::muted(backend_label),
    );
    if let Some(pid) = service_pid {
        println!(
            "  {}: {}",
            Theme::muted("PID"),
            Theme::accent(&pid.to_string()),
        );
    }

    println!();

    // Health probe section.
    println!(
        "  {}: {}",
        Theme::info("Health"),
        if health_ok {
            let ms = health_latency_ms.unwrap_or(0);
            Theme::success(&format!("reachable ({ms}ms)"))
        } else {
            let err_msg = health_error.as_deref().unwrap_or("unknown error");
            Theme::error(&format!("unreachable - {err_msg}"))
        },
    );
    println!(
        "  {}: {}",
        Theme::muted("Endpoint"),
        Theme::muted(&health_url),
    );

    println!();

    Ok(())
}

/// Probe the gateway health endpoint via HTTP GET.
///
/// Returns the latency in milliseconds on success, or an error on failure.
async fn probe_health_endpoint(url: &str) -> Result<u64> {
    let client = reqwest::Client::builder()
        .timeout(Duration::from_secs(5))
        .build()
        .context("failed to build HTTP client")?;

    let start = std::time::Instant::now();

    let response = client
        .get(url)
        .send()
        .await
        .context("health endpoint request failed")?;

    let latency_ms = start.elapsed().as_millis() as u64;

    if response.status().is_success() {
        Ok(latency_ms)
    } else {
        anyhow::bail!(
            "health endpoint returned HTTP {}",
            response.status().as_u16()
        );
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn service_status_display_values() {
        assert_eq!(ServiceStatus::Running.to_string(), "running");
        assert_eq!(ServiceStatus::Stopped.to_string(), "stopped");
        assert_eq!(ServiceStatus::Unknown.to_string(), "unknown");
    }
}
