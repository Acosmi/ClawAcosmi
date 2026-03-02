/// Device token revoke command — calls `device.token.revoke` via Gateway RPC.
use anyhow::Result;

use oa_gateway_rpc::call::{CallGatewayOptions, call_gateway};

/// Revoke a device token.
pub async fn devices_revoke_command(device: &str, role: &str) -> Result<()> {
    let _resp: serde_json::Value = call_gateway(CallGatewayOptions {
        method: "device.token.revoke".to_string(),
        params: Some(serde_json::json!({
            "deviceId": device,
            "role": role,
        })),
        ..Default::default()
    })
    .await?;

    println!("🚫 Token revoked for device {device} (role: {role})");
    Ok(())
}
