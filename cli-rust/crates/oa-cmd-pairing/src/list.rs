/// Pairing list — calls `node.pair.list` via Gateway RPC.
use std::collections::HashMap;

use anyhow::Result;
use serde_json::Value;

use oa_gateway_rpc::call::{CallGatewayOptions, call_gateway};

/// List pending pairing requests via Gateway.
pub async fn pairing_list_command(channel: &str, json_out: bool) -> Result<()> {
    let resp: HashMap<String, Value> = call_gateway(CallGatewayOptions {
        method: "node.pair.list".to_string(),
        params: Some(serde_json::json!({ "channel": channel })),
        ..Default::default()
    })
    .await?;

    if json_out {
        println!("{}", serde_json::to_string_pretty(&resp)?);
        return Ok(());
    }

    let pending = resp.get("pending").and_then(|v| v.as_array());
    let paired = resp.get("paired").and_then(|v| v.as_array());

    println!("📋 Pairing ({channel}):");
    if let Some(pending) = pending {
        println!("   Pending: {} requests", pending.len());
        for req in pending {
            let id = req.get("requestId").and_then(|v| v.as_str()).unwrap_or("?");
            let name = req
                .get("displayName")
                .and_then(|v| v.as_str())
                .unwrap_or("unknown");
            println!("     ⏳ {id}  {name}");
        }
    }
    if let Some(paired) = paired {
        println!("   Paired: {} nodes", paired.len());
    }
    Ok(())
}
