/// Update run — calls `update.run` via Gateway RPC.
use std::collections::HashMap;

use anyhow::Result;
use serde_json::Value;

use oa_gateway_rpc::call::{CallGatewayOptions, call_gateway};

/// Trigger an update via Gateway.
pub async fn update_run_command(
    channel: Option<&str>,
    no_restart: bool,
    json_out: bool,
) -> Result<()> {
    let mut params = serde_json::json!({});
    if no_restart {
        params["scheduleRestartMs"] = Value::Number(0.into());
    }
    if let Some(ch) = channel {
        params["channel"] = Value::String(ch.to_string());
    }

    let resp: HashMap<String, Value> = call_gateway(CallGatewayOptions {
        method: "update.run".to_string(),
        params: Some(params),
        timeout_ms: Some(60_000),
        ..Default::default()
    })
    .await?;

    if json_out {
        println!("{}", serde_json::to_string_pretty(&resp)?);
        return Ok(());
    }

    let action = resp
        .get("action")
        .and_then(|v| v.as_str())
        .unwrap_or("unknown");
    let ok = resp.get("ok").and_then(|v| v.as_bool()).unwrap_or(false);
    if ok {
        println!("🔄 Update: {action}");
    } else {
        println!("❌ Update failed");
    }
    Ok(())
}
