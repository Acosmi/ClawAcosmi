/// Gateway usage cost command.
///
/// Fetches usage and cost data from the gateway via the `usage.cost`
/// RPC method and displays the results.
///
/// Source: `src/commands/gateway-usage-cost.ts`

use anyhow::{Context, Result};
use tracing::info;

use oa_cli_shared::progress::with_progress;
use oa_gateway_rpc::call::{call_gateway, CallGatewayOptions};
use oa_terminal::theme::Theme;

/// Fetch and display gateway usage cost information.
///
/// Calls the `usage.cost` RPC method on the gateway and renders the
/// response as either machine-readable JSON or human-friendly output.
///
/// Source: `src/commands/gateway-usage-cost.ts` - `gatewayUsageCostCommand`
pub async fn gateway_usage_cost_command(json: bool) -> Result<()> {
    info!(json, "gateway usage cost requested");

    let opts = CallGatewayOptions {
        method: "usage.cost".to_string(),
        timeout_ms: Some(10_000),
        ..Default::default()
    };

    let result: serde_json::Value = with_progress(
        "Fetching usage cost...",
        call_gateway(opts),
    )
    .await
    .context("failed to fetch usage cost from gateway")?;

    if json {
        println!(
            "{}",
            serde_json::to_string_pretty(&result)
                .context("failed to serialize usage cost response")?
        );
        return Ok(());
    }

    // Human-readable output.
    println!("{}", Theme::heading("Gateway Usage Cost"));
    println!();

    // Display top-level cost fields if present.
    if let Some(obj) = result.as_object() {
        for (key, value) in obj {
            let display_value = match value {
                serde_json::Value::String(s) => s.clone(),
                serde_json::Value::Number(n) => n.to_string(),
                serde_json::Value::Bool(b) => b.to_string(),
                serde_json::Value::Null => "null".to_string(),
                other => serde_json::to_string_pretty(other).unwrap_or_default(),
            };
            println!(
                "  {}: {}",
                Theme::info(key),
                Theme::accent(&display_value),
            );
        }
    } else {
        println!(
            "  {}",
            serde_json::to_string_pretty(&result)
                .context("failed to serialize usage cost response")?
        );
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
