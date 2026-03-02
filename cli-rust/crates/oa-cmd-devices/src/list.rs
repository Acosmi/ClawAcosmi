/// Device list command — calls `device.pair.list` via Gateway RPC.
use std::collections::HashMap;

use anyhow::Result;
use serde_json::Value;

use oa_gateway_rpc::call::{CallGatewayOptions, call_gateway};

/// List all paired devices and pending requests via Gateway RPC.
pub async fn devices_list_command(json: bool) -> Result<()> {
    let resp: HashMap<String, Value> = call_gateway(CallGatewayOptions {
        method: "device.pair.list".to_string(),
        ..Default::default()
    })
    .await?;

    if json {
        println!("{}", serde_json::to_string_pretty(&resp)?);
        return Ok(());
    }

    // Human-readable output
    let pending = resp.get("pending").and_then(|v| v.as_array());
    let paired = resp.get("paired").and_then(|v| v.as_array());

    if let Some(pending) = pending {
        if !pending.is_empty() {
            println!("📋 Pending requests ({}):", pending.len());
            for req in pending {
                let id = req.get("requestId").and_then(|v| v.as_str()).unwrap_or("?");
                let name = req
                    .get("displayName")
                    .and_then(|v| v.as_str())
                    .unwrap_or("unknown");
                println!("  ⏳ {id}  {name}");
            }
        }
    }

    if let Some(paired) = paired {
        if paired.is_empty() {
            println!("📱 No paired devices.");
        } else {
            println!("📱 Paired devices ({}):", paired.len());
            for dev in paired {
                let id = dev.get("deviceId").and_then(|v| v.as_str()).unwrap_or("?");
                let name = dev
                    .get("displayName")
                    .and_then(|v| v.as_str())
                    .unwrap_or("unknown");
                let role = dev.get("role").and_then(|v| v.as_str()).unwrap_or("?");
                println!("  ✅ {id}  {name}  (role: {role})");
            }
        }
    }

    Ok(())
}
