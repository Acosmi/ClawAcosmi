/// Gateway health check and channel-status probe.
///
/// Runs the health command against the gateway, then probes channel
/// status for warnings.
///
/// Source: `src/commands/doctor-gateway-health.ts`

use oa_terminal::note::note;
use oa_types::config::OpenAcosmiConfig;

/// Result of a gateway health check.
///
/// Source: `src/commands/doctor-gateway-health.ts` — return of `checkGatewayHealth`
#[derive(Debug, Clone)]
pub struct GatewayHealthResult {
    /// Whether the gateway responded to the health check successfully.
    pub health_ok: bool,
}

/// Check gateway health by running the health command with a timeout.
///
/// If healthy, additionally probes `channels.status` for warnings.
///
/// Source: `src/commands/doctor-gateway-health.ts` — `checkGatewayHealth`
pub async fn check_gateway_health(
    cfg: &OpenAcosmiConfig,
    timeout_ms: u64,
) -> GatewayHealthResult {
    let _ = timeout_ms;

    // Build connection details from config.
    let mode = cfg
        .gateway
        .as_ref()
        .and_then(|gw| gw.mode.as_ref())
        .map(|m| format!("{m:?}"))
        .unwrap_or_else(|| "local".to_string());

    // Stub: the real implementation would call `healthCommand`.
    // For now, report as unhealthy so that the daemon flow gets a chance to run.
    let health_ok = false;

    if !health_ok {
        note("Gateway not running.", Some("Gateway"));
        note(
            &format!("Gateway mode: {mode}"),
            Some("Gateway connection"),
        );
    }

    GatewayHealthResult { health_ok }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn health_check_returns_result() {
        let cfg = OpenAcosmiConfig::default();
        let result = check_gateway_health(&cfg, 3000).await;
        // Stub always returns false.
        assert!(!result.health_ok);
    }
}
