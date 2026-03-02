/// Device approve command — calls `device.pair.approve` via Gateway RPC.
use anyhow::Result;

use oa_gateway_rpc::call::{CallGatewayOptions, call_gateway};

/// Approve a pending pairing request.
pub async fn devices_approve_command(request_id: &str) -> Result<()> {
    let resp: serde_json::Value = call_gateway(CallGatewayOptions {
        method: "device.pair.approve".to_string(),
        params: Some(serde_json::json!({ "requestId": request_id })),
        ..Default::default()
    })
    .await?;

    let device_id = resp.get("deviceId").and_then(|v| v.as_str()).unwrap_or("?");
    println!("✅ Device approved: {device_id}");
    Ok(())
}
