/// Cron job disable command.
///
/// Disables a scheduled job via the `cron.disable` RPC method. The job
/// remains configured but will not run until re-enabled.
///
/// Source: `src/commands/cron-disable.ts`

use anyhow::Result;

use oa_config::io::load_config;
use oa_gateway_rpc::call::{call_gateway, CallGatewayOptions};

/// Disable a cron job by ID.
///
/// Source: `src/commands/cron-disable.ts` - `cronDisableCommand`
pub async fn cron_disable_command(id: &str) -> Result<()> {
    let cfg = load_config().unwrap_or_default();

    let params = serde_json::json!({
        "id": id,
    });

    let call_opts = CallGatewayOptions {
        method: "cron.disable".to_string(),
        config: Some(cfg),
        params: Some(params),
        ..Default::default()
    };

    let _result: serde_json::Value = call_gateway(call_opts).await?;
    println!("Disabled cron job {id}.");

    Ok(())
}
