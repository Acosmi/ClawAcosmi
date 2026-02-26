/// Memory search command.
///
/// Searches the memory store for entries matching a query string via
/// the `memory.search` RPC method.
///
/// Source: `src/commands/memory-search.ts`

use anyhow::Result;

use oa_config::io::load_config;
use oa_gateway_rpc::call::{call_gateway, CallGatewayOptions};

/// Search memory entries by query.
///
/// Source: `src/commands/memory-search.ts` - `memorySearchCommand`
pub async fn memory_search_command(
    query: &str,
    limit: Option<usize>,
    json: bool,
) -> Result<()> {
    let cfg = load_config().unwrap_or_default();

    let params = serde_json::json!({
        "query": query,
        "limit": limit.unwrap_or(10),
    });

    let call_opts = CallGatewayOptions {
        method: "memory.search".to_string(),
        config: Some(cfg),
        params: Some(params),
        ..Default::default()
    };

    let result: serde_json::Value = call_gateway(call_opts).await?;

    if json {
        println!("{}", serde_json::to_string_pretty(&result)?);
    } else if let Some(results) = result.as_array() {
        if results.is_empty() {
            println!("No results found for \"{query}\".");
        } else {
            println!("Found {} result(s) for \"{query}\":", results.len());
            for (i, r) in results.iter().enumerate() {
                let text = r
                    .get("text")
                    .and_then(|v| v.as_str())
                    .unwrap_or("(no text)");
                let score = r.get("score").and_then(|v| v.as_f64()).unwrap_or(0.0);
                println!("  {}. [{:.3}] {}", i + 1, score, text);
            }
        }
    }

    Ok(())
}
