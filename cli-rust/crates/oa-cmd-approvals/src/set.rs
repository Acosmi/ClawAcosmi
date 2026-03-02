/// Approvals set — calls `exec.approvals.set` via Gateway RPC.
use anyhow::{Context, Result};
use serde_json::Value;

use oa_gateway_rpc::call::{CallGatewayOptions, call_gateway};

/// Set exec approvals from a file, with optimistic concurrency control.
pub async fn approvals_set_command(file: &str, gateway: bool, node: Option<&str>) -> Result<()> {
    // Read the approvals file
    let content = std::fs::read_to_string(file)
        .with_context(|| format!("Failed to read approvals file: {file}"))?;
    let file_value: Value = serde_json::from_str(&content)
        .with_context(|| format!("Failed to parse approvals file as JSON: {file}"))?;

    // First, get current hash for optimistic concurrency
    let get_method = if node.is_some() {
        "exec.approvals.node.get"
    } else {
        "exec.approvals.get"
    };
    let mut get_params = serde_json::Map::new();
    if let Some(node_id) = node {
        get_params.insert("nodeId".to_string(), Value::String(node_id.to_string()));
    }

    let current: std::collections::HashMap<String, Value> = call_gateway(CallGatewayOptions {
        method: get_method.to_string(),
        params: if get_params.is_empty() {
            None
        } else {
            Some(Value::Object(get_params))
        },
        ..Default::default()
    })
    .await?;

    let base_hash = current.get("hash").and_then(|v| v.as_str()).unwrap_or("");

    // Now set with the base hash
    let set_method = if node.is_some() {
        "exec.approvals.node.set"
    } else {
        "exec.approvals.set"
    };
    let mut set_params = serde_json::json!({
        "file": file_value,
        "baseHash": base_hash,
    });
    if let Some(node_id) = node {
        set_params["nodeId"] = Value::String(node_id.to_string());
    }

    let _resp: std::collections::HashMap<String, Value> = call_gateway(CallGatewayOptions {
        method: set_method.to_string(),
        params: Some(set_params),
        ..Default::default()
    })
    .await?;

    println!("✅ Exec approvals updated from {file}");
    let _ = gateway;
    Ok(())
}
