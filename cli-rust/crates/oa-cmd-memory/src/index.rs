/// Memory index command.
///
/// Triggers a full memory re-index via the `memory.index` RPC method.
/// Shows a progress spinner while the indexing operation runs.
///
/// Source: `src/commands/memory-index.ts`

use anyhow::Result;

use oa_cli_shared::progress::with_progress;
use oa_config::io::load_config;
use oa_gateway_rpc::call::{call_gateway, CallGatewayOptions};

/// Trigger a memory re-index operation.
///
/// Source: `src/commands/memory-index.ts` - `memoryIndexCommand`
pub async fn memory_index_command() -> Result<()> {
    let cfg = load_config().unwrap_or_default();

    let call_opts = CallGatewayOptions {
        method: "memory.index".to_string(),
        config: Some(cfg),
        ..Default::default()
    };

    with_progress("Indexing memory...", async {
        let result: serde_json::Value = call_gateway(call_opts).await?;
        if let Some(count) = result.get("indexed") {
            println!("Indexed {count} entries.");
        } else {
            println!("Memory indexing complete.");
        }
        Ok(())
    })
    .await
}
