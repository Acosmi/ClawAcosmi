/// Gateway background start command.
///
/// Starts the gateway as a background service by delegating to the
/// platform-specific daemon service manager (launchd on macOS,
/// systemd on Linux).
///
/// Source: `src/commands/gateway-start.ts`

use std::collections::HashMap;

use anyhow::{Context, Result};
use tracing::info;

use oa_config::io::load_config;
use oa_config::paths::resolve_gateway_port;
use oa_daemon::constants::format_gateway_service_description;
use oa_daemon::service::{
    install_gateway_service, is_gateway_service_enabled, restart_gateway_service,
    service_backend_label,
};
use oa_terminal::theme::Theme;

/// Start the gateway as a background daemon service.
///
/// If the service is not yet installed, it will be installed first.
/// If `force` is true and the service is already running, it will be
/// restarted.
///
/// Source: `src/commands/gateway-start.ts` - `gatewayStartCommand`
pub async fn gateway_start_command(port: Option<u16>, force: bool) -> Result<()> {
    let config = load_config().unwrap_or_default();
    let effective_port = port.unwrap_or_else(|| resolve_gateway_port(Some(&config)));
    let backend_label = service_backend_label();

    info!(
        port = effective_port,
        force,
        backend = backend_label,
        "gateway start requested"
    );

    // Check whether the service is already installed.
    let is_loaded = is_gateway_service_enabled(None).unwrap_or(false);

    if is_loaded && !force {
        // Service already exists and running -- just restart.
        println!(
            "{}: {} service already installed, restarting...",
            Theme::info("Gateway"),
            Theme::muted(backend_label),
        );
        restart_gateway_service(None)
            .context("failed to restart gateway service")?;
        println!("{}", Theme::success("Gateway restarted."));
        return Ok(());
    }

    if is_loaded && force {
        println!(
            "{}: force-restarting {} service...",
            Theme::info("Gateway"),
            Theme::muted(backend_label),
        );
        restart_gateway_service(None)
            .context("failed to force-restart gateway service")?;
        println!("{}", Theme::success("Gateway force-restarted."));
        return Ok(());
    }

    // Service is not installed -- install and start.
    println!(
        "{}: installing {} service on port {}...",
        Theme::info("Gateway"),
        Theme::muted(backend_label),
        Theme::accent(&effective_port.to_string()),
    );

    let exe_path = std::env::current_exe()
        .context("failed to determine executable path")?
        .to_string_lossy()
        .to_string();

    let program_arguments = vec![
        exe_path,
        "gateway".to_string(),
        "run".to_string(),
        "--port".to_string(),
        effective_port.to_string(),
    ];

    let description = format_gateway_service_description(None, None);
    let environment: HashMap<String, String> = HashMap::new();

    install_gateway_service(
        None,
        &program_arguments,
        None,
        &environment,
        Some(&description),
    )
    .context("failed to install gateway service")?;

    println!(
        "{}",
        Theme::success(&format!(
            "Gateway service installed and started on port {effective_port}."
        )),
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
