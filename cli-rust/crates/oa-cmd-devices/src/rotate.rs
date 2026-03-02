/// Device token rotate command — calls `device.token.rotate` via Gateway RPC.
use anyhow::Result;

use oa_gateway_rpc::call::{CallGatewayOptions, call_gateway};

/// Rotate a device token.
pub async fn devices_rotate_command(device: &str, role: &str) -> Result<()> {
    let resp: serde_json::Value = call_gateway(CallGatewayOptions {
        method: "device.token.rotate".to_string(),
        params: Some(serde_json::json!({
            "deviceId": device,
            "role": role,
        })),
        ..Default::default()
    })
    .await?;

    let new_token = resp
        .get("token")
        .and_then(|v| v.as_str())
        .unwrap_or("(hidden)");
    println!("🔄 Token rotated for device {device} (role: {role})");
    println!("   New token: {new_token}");
    Ok(())
}
