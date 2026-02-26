/// Memory check command.
///
/// Performs a health check on the memory subsystem via the `memory.check`
/// RPC method.
///
/// Source: `src/commands/memory-check.ts`

use anyhow::Result;

use oa_config::io::load_config;
use oa_gateway_rpc::call::{call_gateway, CallGatewayOptions};

/// Run a memory subsystem health check.
///
/// Source: `src/commands/memory-check.ts` - `memoryCheckCommand`
pub async fn memory_check_command(json: bool) -> Result<()> {
    let cfg = load_config().unwrap_or_default();

    let call_opts = CallGatewayOptions {
        method: "memory.check".to_string(),
        config: Some(cfg),
        ..Default::default()
    };

    let result: serde_json::Value = call_gateway(call_opts).await?;

    if json {
        println!("{}", serde_json::to_string_pretty(&result)?);
    } else {
        println!("Memory check: {result}");
    }

    Ok(())
}
