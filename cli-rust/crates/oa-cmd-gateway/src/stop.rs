/// Gateway stop command.
///
/// Stops the gateway background service by delegating to the
/// platform-specific daemon service manager.
///
/// Source: `src/commands/gateway-stop.ts`

use anyhow::{Context, Result};
use tracing::info;

use oa_daemon::service::{
    is_gateway_service_enabled, service_backend_label, stop_gateway_service,
};
use oa_terminal::theme::Theme;

/// Stop the gateway background service.
///
/// If the service is not installed or not running, prints an informational
/// message and returns successfully.
///
/// Source: `src/commands/gateway-stop.ts` - `gatewayStopCommand`
pub async fn gateway_stop_command() -> Result<()> {
    let backend_label = service_backend_label();

    info!(backend = backend_label, "gateway stop requested");

    let is_loaded = is_gateway_service_enabled(None).unwrap_or(false);

    if !is_loaded {
        println!(
            "{}: {} service is not installed or not running.",
            Theme::info("Gateway"),
            Theme::muted(backend_label),
        );
        return Ok(());
    }

    println!(
        "{}: stopping {} service...",
        Theme::info("Gateway"),
        Theme::muted(backend_label),
    );

    stop_gateway_service(None)
        .context("failed to stop gateway service")?;

    println!("{}", Theme::success("Gateway stopped."));

    Ok(())
}

#[cfg(test)]
mod tests {
    #[test]
    fn module_compiles() {
        // Ensures the module compiles correctly.
    }
}
