/// Unlink/logout from a channel account.
///
/// Source: `src/commands/channels-logout.ts`

use anyhow::{Context, Result};

use oa_config::io::load_config;
use oa_gateway_rpc::call::{call_gateway, CallGatewayOptions};

/// Options for the `channels logout` command.
pub struct ChannelsLogoutOptions {
    pub channel: String,
    pub account: Option<String>,
    pub json: bool,
}

/// Execute the `channels logout` command.
pub async fn channels_logout_command(opts: &ChannelsLogoutOptions) -> Result<()> {
    let cfg = load_config().unwrap_or_default();

    let params = serde_json::json!({
        "channel": opts.channel,
        "account": opts.account,
    });

    let call_opts = CallGatewayOptions {
        method: "channels.logout".to_string(),
        config: Some(cfg),
        params: Some(params),
        ..Default::default()
    };

    let result: serde_json::Value = call_gateway(call_opts)
        .await
        .context("Failed to logout from channel")?;

    if opts.json {
        println!("{}", serde_json::to_string_pretty(&result)?);
    } else {
        let status = result
            .get("status")
            .and_then(|v| v.as_str())
            .unwrap_or("unknown");
        println!("Channel logout ({}):", opts.channel);
        println!("  Status: {status}");
    }
    Ok(())
}
