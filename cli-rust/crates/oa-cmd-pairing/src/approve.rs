/// Pairing approve — calls `node.pair.approve` via Gateway RPC.
use anyhow::Result;

use oa_gateway_rpc::call::{CallGatewayOptions, call_gateway};

/// Approve a pairing request via Gateway.
pub async fn pairing_approve_command(channel: &str, code: &str, notify: bool) -> Result<()> {
    let _resp: serde_json::Value = call_gateway(CallGatewayOptions {
        method: "node.pair.approve".to_string(),
        params: Some(serde_json::json!({
            "requestId": code,
            "channel": channel,
            "notify": notify,
        })),
        ..Default::default()
    })
    .await?;

    println!("✅ Pairing approved ({channel}, code={code})");
    Ok(())
}
