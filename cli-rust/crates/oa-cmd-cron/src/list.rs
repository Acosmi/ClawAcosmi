/// Cron job list command.
///
/// Queries the gateway for all scheduled jobs via the `cron.list` RPC
/// method and displays them in a table format.
///
/// Source: `src/commands/cron-list.ts`

use anyhow::Result;

use oa_config::io::load_config;
use oa_gateway_rpc::call::{call_gateway, CallGatewayOptions};

/// List all scheduled cron jobs.
///
/// Displays a table with columns: name, schedule, enabled, last run.
///
/// Source: `src/commands/cron-list.ts` - `cronListCommand`
pub async fn cron_list_command(json: bool) -> Result<()> {
    let cfg = load_config().unwrap_or_default();

    let call_opts = CallGatewayOptions {
        method: "cron.list".to_string(),
        config: Some(cfg),
        ..Default::default()
    };

    let result: serde_json::Value = call_gateway(call_opts).await?;

    if json {
        println!("{}", serde_json::to_string_pretty(&result)?);
    } else if let Some(jobs) = result.as_array() {
        if jobs.is_empty() {
            println!("No cron jobs configured.");
        } else {
            println!(
                "{:<20} {:<20} {:<10} {}",
                "NAME", "SCHEDULE", "ENABLED", "LAST RUN"
            );
            for job in jobs {
                let name = job
                    .get("name")
                    .and_then(|v| v.as_str())
                    .unwrap_or("(unnamed)");
                let schedule = job
                    .get("schedule")
                    .and_then(|v| v.as_str())
                    .unwrap_or("-");
                let enabled = job
                    .get("enabled")
                    .and_then(|v| v.as_bool())
                    .unwrap_or(false);
                let last_run = job
                    .get("last_run")
                    .and_then(|v| v.as_str())
                    .unwrap_or("never");
                println!(
                    "{:<20} {:<20} {:<10} {}",
                    name,
                    schedule,
                    if enabled { "yes" } else { "no" },
                    last_run
                );
            }
        }
    }

    Ok(())
}
