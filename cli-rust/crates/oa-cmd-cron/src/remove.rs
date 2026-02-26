/// Cron job remove command.
///
/// Deletes a scheduled job via the `cron.remove` RPC method.
///
/// Source: `src/commands/cron-remove.ts`

use anyhow::Result;

use oa_config::io::load_config;
use oa_gateway_rpc::call::{call_gateway, CallGatewayOptions};

/// Remove (delete) a cron job by ID.
///
/// Source: `src/commands/cron-remove.ts` - `cronRemoveCommand`
pub async fn cron_remove_command(id: &str) -> Result<()> {
    let cfg = load_config().unwrap_or_default();

    let params = serde_json::json!({
        "id": id,
    });

    let call_opts = CallGatewayOptions {
        method: "cron.remove".to_string(),
        config: Some(cfg),
        params: Some(params),
        ..Default::default()
    };

    let _result: serde_json::Value = call_gateway(call_opts).await?;
    println!("Removed cron job {id}.");

    Ok(())
}
