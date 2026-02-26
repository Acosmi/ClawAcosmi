/// Link/login to a channel account.
///
/// Source: `src/commands/channels-login.ts`

use anyhow::{Context, Result};

use oa_config::io::load_config;
use oa_gateway_rpc::call::{call_gateway, CallGatewayOptions};

/// Options for the `channels login` command.
pub struct ChannelsLoginOptions {
    pub channel: String,
    pub account: Option<String>,
    pub json: bool,
}

/// Execute the `channels login` command.
pub async fn channels_login_command(opts: &ChannelsLoginOptions) -> Result<()> {
    let cfg = load_config().unwrap_or_default();

    let params = serde_json::json!({
        "channel": opts.channel,
        "account": opts.account,
    });

    let call_opts = CallGatewayOptions {
        method: "channels.login".to_string(),
        config: Some(cfg),
        params: Some(params),
        ..Default::default()
    };

    let result: serde_json::Value = call_gateway(call_opts)
        .await
        .context("Failed to login to channel")?;

    if opts.json {
        println!("{}", serde_json::to_string_pretty(&result)?);
    } else {
        let status = result
            .get("status")
            .and_then(|v| v.as_str())
            .unwrap_or("unknown");
        let msg = result
            .get("message")
            .and_then(|v| v.as_str())
            .unwrap_or("");
        println!("Channel login ({}):", opts.channel);
        println!("  Status: {status}");
        if !msg.is_empty() {
            println!("  {msg}");
        }
    }
    Ok(())
}
