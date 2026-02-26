/// Cron job add command.
///
/// Creates a new scheduled job via the `cron.add` RPC method.
///
/// Source: `src/commands/cron-add.ts`

use anyhow::Result;

use oa_config::io::load_config;
use oa_gateway_rpc::call::{call_gateway, CallGatewayOptions};

/// Add a new cron job.
///
/// Source: `src/commands/cron-add.ts` - `cronAddCommand`
pub async fn cron_add_command(
    name: &str,
    schedule: &str,
    agent_id: &str,
    message: &str,
) -> Result<()> {
    let cfg = load_config().unwrap_or_default();

    let params = serde_json::json!({
        "name": name,
        "schedule": schedule,
        "agent_id": agent_id,
        "message": message,
    });

    let call_opts = CallGatewayOptions {
        method: "cron.add".to_string(),
        config: Some(cfg),
        params: Some(params),
        ..Default::default()
    };

    let result: serde_json::Value = call_gateway(call_opts).await?;

    let id = result
        .get("id")
        .and_then(|v| v.as_str())
        .unwrap_or("(unknown)");
    println!("Created cron job \"{name}\" (id: {id})");

    Ok(())
}
