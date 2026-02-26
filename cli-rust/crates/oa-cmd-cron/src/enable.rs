/// Cron job enable command.
///
/// Enables a previously disabled scheduled job via the `cron.enable`
/// RPC method.
///
/// Source: `src/commands/cron-enable.ts`

use anyhow::Result;

use oa_config::io::load_config;
use oa_gateway_rpc::call::{call_gateway, CallGatewayOptions};

/// Enable a cron job by ID.
///
/// Source: `src/commands/cron-enable.ts` - `cronEnableCommand`
pub async fn cron_enable_command(id: &str) -> Result<()> {
    let cfg = load_config().unwrap_or_default();

    let params = serde_json::json!({
        "id": id,
    });

    let call_opts = CallGatewayOptions {
        method: "cron.enable".to_string(),
        config: Some(cfg),
        params: Some(params),
        ..Default::default()
    };

    let _result: serde_json::Value = call_gateway(call_opts).await?;
    println!("Enabled cron job {id}.");

    Ok(())
}
