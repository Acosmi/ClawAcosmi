/// Device reject command — calls `device.pair.reject` via Gateway RPC.
use anyhow::Result;

use oa_gateway_rpc::call::{CallGatewayOptions, call_gateway};

/// Reject a pending pairing request.
pub async fn devices_reject_command(request_id: &str) -> Result<()> {
    let _resp: serde_json::Value = call_gateway(CallGatewayOptions {
        method: "device.pair.reject".to_string(),
        params: Some(serde_json::json!({ "requestId": request_id })),
        ..Default::default()
    })
    .await?;

    println!("❌ Device rejected: {request_id}");
    Ok(())
}
