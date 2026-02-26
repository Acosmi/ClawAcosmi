/// Gateway health command.
///
/// Fetches health information from the gateway via the `health` RPC
/// method and displays the results.
///
/// Source: `src/commands/gateway-health.ts`

use anyhow::{Context, Result};
use tracing::info;

use oa_cli_shared::progress::with_progress;
use oa_gateway_rpc::call::{call_gateway, CallGatewayOptions};
use oa_terminal::theme::Theme;

/// Fetch and display gateway health information.
///
/// Calls the `health` RPC method on the gateway and renders the
/// response as either machine-readable JSON or human-friendly output.
///
/// Source: `src/commands/gateway-health.ts` - `gatewayHealthCommand`
pub async fn gateway_health_command(json: bool) -> Result<()> {
    info!(json, "gateway health requested");

    let opts = CallGatewayOptions {
        method: "health".to_string(),
        timeout_ms: Some(10_000),
        ..Default::default()
    };

    let result: serde_json::Value = with_progress(
        "Checking gateway health...",
        call_gateway(opts),
    )
    .await
    .context("failed to fetch health from gateway")?;

    if json {
        println!(
            "{}",
            serde_json::to_string_pretty(&result)
                .context("failed to serialize health response")?
        );
        return Ok(());
    }

    // Human-readable output.
    let status = result
        .get("status")
        .and_then(|v| v.as_str())
        .unwrap_or("unknown");

    let uptime_ms = result
        .get("uptimeMs")
        .and_then(|v| v.as_u64());

    let version = result
        .get("version")
        .and_then(|v| v.as_str());

    println!("{}", Theme::heading("Gateway Health"));
    println!();

    println!(
        "  {}: {}",
        Theme::info("Status"),
        if status == "ok" || status == "healthy" {
            Theme::success(status)
        } else {
            Theme::error(status)
        },
    );

    if let Some(v) = version {
        println!(
            "  {}: {}",
            Theme::info("Version"),
            Theme::accent(v),
        );
    }

    if let Some(ms) = uptime_ms {
        let seconds = ms / 1000;
        let minutes = seconds / 60;
        let hours = minutes / 60;
        let uptime_str = if hours > 0 {
            format!("{hours}h {m}m", m = minutes % 60)
        } else if minutes > 0 {
            format!("{minutes}m {s}s", s = seconds % 60)
        } else {
            format!("{seconds}s")
        };
        println!(
            "  {}: {}",
            Theme::info("Uptime"),
            Theme::muted(&uptime_str),
        );
    }

    // Show additional fields if present.
    if let Some(obj) = result.as_object() {
        let skip_keys = ["status", "uptimeMs", "version"];
        let extras: Vec<_> = obj
            .iter()
            .filter(|(k, _)| !skip_keys.contains(&k.as_str()))
            .collect();
        if !extras.is_empty() {
            println!();
            for (key, value) in extras {
                let display_value = match value {
                    serde_json::Value::String(s) => s.clone(),
                    serde_json::Value::Number(n) => n.to_string(),
                    serde_json::Value::Bool(b) => b.to_string(),
                    serde_json::Value::Null => "null".to_string(),
                    other => serde_json::to_string(other).unwrap_or_default(),
                };
                println!(
                    "  {}: {}",
                    Theme::muted(key),
                    display_value,
                );
            }
        }
    }

    println!();

    Ok(())
}

#[cfg(test)]
mod tests {
    #[test]
    fn module_compiles() {
        // Ensures the module compiles correctly.
    }
}
