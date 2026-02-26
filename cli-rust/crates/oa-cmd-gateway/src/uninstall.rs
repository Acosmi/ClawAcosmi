/// Gateway uninstall command.
///
/// Removes the gateway system service (launchd on macOS, systemd on
/// Linux), stopping it first if it is currently running.
///
/// Source: `src/commands/gateway-uninstall.ts`

use anyhow::{Context, Result};
use tracing::info;

use oa_daemon::service::{
    is_gateway_service_enabled, service_backend_label, stop_gateway_service,
    uninstall_gateway_service,
};
use oa_terminal::theme::Theme;

/// Uninstall the gateway system service.
///
/// If the service is currently loaded/running, it is stopped first,
/// then the service definition is removed from the platform service
/// manager.
///
/// Source: `src/commands/gateway-uninstall.ts` - `gatewayUninstallCommand`
pub async fn gateway_uninstall_command() -> Result<()> {
    let backend_label = service_backend_label();

    info!(backend = backend_label, "gateway uninstall requested");

    let is_loaded = is_gateway_service_enabled(None).unwrap_or(false);

    if !is_loaded {
        println!(
            "{}: {} service is not installed.",
            Theme::info("Gateway"),
            Theme::muted(backend_label),
        );
        return Ok(());
    }

    // Stop the service before uninstalling.
    println!(
        "{}: stopping {} service...",
        Theme::info("Gateway"),
        Theme::muted(backend_label),
    );

    // Best-effort stop -- the service may already be stopped.
    let _ = stop_gateway_service(None);

    println!(
        "{}: removing {} service...",
        Theme::info("Gateway"),
        Theme::muted(backend_label),
    );

    uninstall_gateway_service(None)
        .context("failed to uninstall gateway service")?;

    println!(
        "{}",
        Theme::success(&format!("Gateway {backend_label} service uninstalled.")),
    );

    Ok(())
}

#[cfg(test)]
mod tests {
    #[test]
    fn module_compiles() {
        // Ensures the module compiles correctly.
    }
}
