/// Update status — calls `update.check` via Gateway RPC.
use std::collections::HashMap;

use anyhow::Result;
use serde_json::Value;

use oa_gateway_rpc::call::{CallGatewayOptions, call_gateway};

/// Check update status via Gateway.
pub async fn update_status_command(json_out: bool) -> Result<()> {
    let resp: HashMap<String, Value> = call_gateway(CallGatewayOptions {
        method: "update.check".to_string(),
        ..Default::default()
    })
    .await?;

    if json_out {
        println!("{}", serde_json::to_string_pretty(&resp)?);
        return Ok(());
    }

    let version = resp
        .get("currentVersion")
        .and_then(|v| v.as_str())
        .unwrap_or("?");
    let platform = resp.get("platform").and_then(|v| v.as_str()).unwrap_or("?");
    let arch = resp.get("arch").and_then(|v| v.as_str()).unwrap_or("?");
    let available = resp
        .get("updateAvailable")
        .and_then(|v| v.as_bool())
        .unwrap_or(false);

    println!("📊 Update Status:");
    println!("   Version:  {version}");
    println!("   Platform: {platform}/{arch}");
    if available {
        println!("   ⬆️  Update available!");
    } else {
        println!("   ✅ Up to date");
    }
    Ok(())
}
