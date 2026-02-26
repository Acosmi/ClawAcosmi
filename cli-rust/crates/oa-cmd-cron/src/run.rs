/// Cron job immediate run command.
///
/// Triggers an immediate execution of a scheduled job via the `cron.run`
/// RPC method, bypassing the normal schedule.
///
/// Source: `src/commands/cron-run.ts`

use anyhow::Result;

use oa_config::io::load_config;
use oa_gateway_rpc::call::{call_gateway, CallGatewayOptions};

/// Trigger an immediate execution of a cron job.
///
/// Source: `src/commands/cron-run.ts` - `cronRunCommand`
pub async fn cron_run_command(id: &str) -> Result<()> {
    let cfg = load_config().unwrap_or_default();

    let params = serde_json::json!({
        "id": id,
    });

    let call_opts = CallGatewayOptions {
        method: "cron.run".to_string(),
        config: Some(cfg),
        params: Some(params),
        ..Default::default()
    };

    let result: serde_json::Value = call_gateway(call_opts).await?;

    let run_id = result
        .get("run_id")
        .and_then(|v| v.as_str())
        .unwrap_or("(unknown)");
    println!("Triggered cron job {id} (run: {run_id}).");

    Ok(())
}
