/// Memory status command.
///
/// Queries the gateway for the current memory subsystem status via the
/// `memory.status` RPC method.
///
/// Source: `src/commands/memory-status.ts`

use anyhow::Result;

use oa_config::io::load_config;
use oa_gateway_rpc::call::{call_gateway, CallGatewayOptions};

/// Display the current memory subsystem status.
///
/// Source: `src/commands/memory-status.ts` - `memoryStatusCommand`
pub async fn memory_status_command(json: bool) -> Result<()> {
    let cfg = load_config().unwrap_or_default();

    let call_opts = CallGatewayOptions {
        method: "memory.status".to_string(),
        config: Some(cfg),
        ..Default::default()
    };

    let result: serde_json::Value = call_gateway(call_opts).await?;

    if json {
        println!("{}", serde_json::to_string_pretty(&result)?);
    } else {
        println!("Memory Status:");
        if let Some(obj) = result.as_object() {
            for (k, v) in obj {
                println!("  {k}: {v}");
            }
        }
    }

    Ok(())
}
