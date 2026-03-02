/// Node status — calls `node.list` via Gateway RPC.
use std::collections::HashMap;

use anyhow::Result;
use serde_json::Value;

use oa_gateway_rpc::call::{CallGatewayOptions, call_gateway};

/// Show node service status via Gateway RPC.
pub async fn node_status_command(json_out: bool) -> Result<()> {
    let resp: HashMap<String, Value> = call_gateway(CallGatewayOptions {
        method: "node.list".to_string(),
        ..Default::default()
    })
    .await?;

    if json_out {
        println!("{}", serde_json::to_string_pretty(&resp)?);
        return Ok(());
    }

    let nodes = resp.get("nodes").and_then(|v| v.as_array());
    if let Some(nodes) = nodes {
        if nodes.is_empty() {
            println!("📊 No nodes registered.");
        } else {
            println!("📊 Nodes ({}):", nodes.len());
            for node in nodes {
                let id = node.get("id").and_then(|v| v.as_str()).unwrap_or("?");
                let name = node
                    .get("displayName")
                    .and_then(|v| v.as_str())
                    .unwrap_or("unknown");
                let online = node
                    .get("online")
                    .and_then(|v| v.as_bool())
                    .unwrap_or(false);
                let status = if online {
                    "🟢 online"
                } else {
                    "🔴 offline"
                };
                println!("  {status} {id}  {name}");
            }
        }
    }
    Ok(())
}
