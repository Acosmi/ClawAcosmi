/// Gateway raw RPC call command.
///
/// Sends an arbitrary RPC method call to the gateway and displays the
/// response. Useful for debugging and ad-hoc gateway interactions.
///
/// Source: `src/commands/gateway-call.ts`

use anyhow::{Context, Result};
use tracing::info;

use oa_cli_shared::progress::with_progress;
use oa_gateway_rpc::call::{call_gateway, CallGatewayOptions};
use oa_terminal::theme::Theme;

/// Make a raw RPC call to the gateway.
///
/// Parses `params` as a JSON string (if provided), sends the RPC
/// request, and renders the response as either formatted JSON or
/// human-readable output.
///
/// Source: `src/commands/gateway-call.ts` - `gatewayCallCommand`
pub async fn gateway_call_command(
    method: &str,
    params: Option<&str>,
    json: bool,
) -> Result<()> {
    info!(method, params, json, "gateway call requested");

    // Parse the params string as JSON if provided.
    let parsed_params: Option<serde_json::Value> = match params {
        Some(raw) => {
            let trimmed = raw.trim();
            if trimmed.is_empty() {
                None
            } else {
                Some(
                    serde_json::from_str(trimmed)
                        .with_context(|| format!("failed to parse params as JSON: {trimmed}"))?,
                )
            }
        }
        None => None,
    };

    let opts = CallGatewayOptions {
        method: method.to_string(),
        params: parsed_params,
        timeout_ms: Some(15_000),
        ..Default::default()
    };

    let result: serde_json::Value = with_progress(
        &format!("Calling gateway method '{method}'..."),
        call_gateway(opts),
    )
    .await
    .with_context(|| format!("gateway RPC call to '{method}' failed"))?;

    if json {
        println!(
            "{}",
            serde_json::to_string_pretty(&result)
                .context("failed to serialize gateway response")?
        );
    } else {
        println!("{} {}", Theme::info("Method:"), Theme::accent(method));
        println!(
            "{} {}",
            Theme::info("Response:"),
            serde_json::to_string_pretty(&result)
                .context("failed to serialize gateway response")?
        );
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    #[test]
    fn parse_empty_params() {
        let raw = "   ";
        let trimmed = raw.trim();
        assert!(trimmed.is_empty());
    }

    #[test]
    fn parse_valid_json_params() {
        let raw = r#"{"key": "value"}"#;
        let parsed: Result<serde_json::Value, _> = serde_json::from_str(raw);
        assert!(parsed.is_ok());
    }

    #[test]
    fn parse_invalid_json_params() {
        let raw = "not-json";
        let parsed: Result<serde_json::Value, _> = serde_json::from_str(raw);
        assert!(parsed.is_err());
    }
}
