/// Cron scheduler status command.
///
/// Queries the gateway for the current scheduler status via the
/// `cron.status` RPC method.
///
/// Source: `src/commands/cron-status.ts`

use anyhow::Result;

use oa_config::io::load_config;
use oa_gateway_rpc::call::{call_gateway, CallGatewayOptions};

/// Display the current cron scheduler status.
///
/// Shows whether the scheduler is running or stopped, and the next
/// scheduled job if applicable.
///
/// Source: `src/commands/cron-status.ts` - `cronStatusCommand`
pub async fn cron_status_command(json: bool) -> Result<()> {
    let cfg = load_config().unwrap_or_default();

    let call_opts = CallGatewayOptions {
        method: "cron.status".to_string(),
        config: Some(cfg),
        ..Default::default()
    };

    let result: serde_json::Value = call_gateway(call_opts).await?;

    if json {
        println!("{}", serde_json::to_string_pretty(&result)?);
    } else {
        let running = result
            .get("running")
            .and_then(|v| v.as_bool())
            .unwrap_or(false);
        let status_label = if running { "running" } else { "stopped" };
        println!("Scheduler: {status_label}");

        if let Some(next) = result.get("next_job") {
            let name = next
                .get("name")
                .and_then(|v| v.as_str())
                .unwrap_or("(unknown)");
            let at = next
                .get("next_run")
                .and_then(|v| v.as_str())
                .unwrap_or("(unknown)");
            println!("Next job: {name} at {at}");
        }
    }

    Ok(())
}
