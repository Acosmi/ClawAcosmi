/// Cron job run history command.
///
/// Queries the gateway for the execution history of a specific job via
/// the `cron.runs` RPC method.
///
/// Source: `src/commands/cron-runs.ts`

use anyhow::Result;

use oa_config::io::load_config;
use oa_gateway_rpc::call::{call_gateway, CallGatewayOptions};

/// Display the run history for a cron job.
///
/// Source: `src/commands/cron-runs.ts` - `cronRunsCommand`
pub async fn cron_runs_command(id: &str, limit: Option<usize>, json: bool) -> Result<()> {
    let cfg = load_config().unwrap_or_default();

    let params = serde_json::json!({
        "id": id,
        "limit": limit.unwrap_or(20),
    });

    let call_opts = CallGatewayOptions {
        method: "cron.runs".to_string(),
        config: Some(cfg),
        params: Some(params),
        ..Default::default()
    };

    let result: serde_json::Value = call_gateway(call_opts).await?;

    if json {
        println!("{}", serde_json::to_string_pretty(&result)?);
    } else if let Some(runs) = result.as_array() {
        if runs.is_empty() {
            println!("No runs found for job {id}.");
        } else {
            println!(
                "{:<24} {:<12} {:<10} {}",
                "STARTED", "DURATION", "STATUS", "MESSAGE"
            );
            for run in runs {
                let started = run
                    .get("started_at")
                    .and_then(|v| v.as_str())
                    .unwrap_or("-");
                let duration = run
                    .get("duration_ms")
                    .and_then(|v| v.as_u64())
                    .map(|ms| format!("{ms}ms"))
                    .unwrap_or_else(|| "-".to_string());
                let status = run
                    .get("status")
                    .and_then(|v| v.as_str())
                    .unwrap_or("unknown");
                let message = run
                    .get("message")
                    .and_then(|v| v.as_str())
                    .unwrap_or("");
                println!("{:<24} {:<12} {:<10} {}", started, duration, status, message);
            }
        }
    }

    Ok(())
}
