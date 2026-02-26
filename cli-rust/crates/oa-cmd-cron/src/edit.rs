/// Cron job edit command.
///
/// Updates an existing scheduled job via the `cron.edit` RPC method.
///
/// Source: `src/commands/cron-edit.ts`

use anyhow::Result;

use oa_config::io::load_config;
use oa_gateway_rpc::call::{call_gateway, CallGatewayOptions};

/// Edit an existing cron job.
///
/// Only the provided fields are updated; others remain unchanged.
///
/// Source: `src/commands/cron-edit.ts` - `cronEditCommand`
pub async fn cron_edit_command(
    id: &str,
    name: Option<&str>,
    schedule: Option<&str>,
    agent_id: Option<&str>,
    message: Option<&str>,
) -> Result<()> {
    let cfg = load_config().unwrap_or_default();

    let mut fields = serde_json::Map::new();
    fields.insert("id".to_string(), serde_json::Value::String(id.to_string()));
    if let Some(n) = name {
        fields.insert("name".to_string(), serde_json::Value::String(n.to_string()));
    }
    if let Some(s) = schedule {
        fields.insert(
            "schedule".to_string(),
            serde_json::Value::String(s.to_string()),
        );
    }
    if let Some(a) = agent_id {
        fields.insert(
            "agent_id".to_string(),
            serde_json::Value::String(a.to_string()),
        );
    }
    if let Some(m) = message {
        fields.insert(
            "message".to_string(),
            serde_json::Value::String(m.to_string()),
        );
    }

    let params = serde_json::Value::Object(fields);

    let call_opts = CallGatewayOptions {
        method: "cron.edit".to_string(),
        config: Some(cfg),
        params: Some(params),
        ..Default::default()
    };

    let _result: serde_json::Value = call_gateway(call_opts).await?;
    println!("Updated cron job {id}.");

    Ok(())
}
