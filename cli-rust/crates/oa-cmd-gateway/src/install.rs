/// Gateway install command.
///
/// Installs the gateway as a system-managed service (launchd on macOS,
/// systemd on Linux) so it starts automatically on boot.
///
/// Source: `src/commands/gateway-install.ts`

use std::collections::HashMap;

use anyhow::{Context, Result};
use tracing::info;

use oa_config::io::load_config;
use oa_config::paths::resolve_gateway_port;
use oa_daemon::constants::format_gateway_service_description;
use oa_daemon::service::{install_gateway_service, service_backend_label};
use oa_terminal::theme::Theme;

/// Install the gateway as a system service.
///
/// Creates the platform-specific service definition (LaunchAgent plist
/// on macOS, systemd unit on Linux) and enables/loads it. The service
/// is configured to run the current executable with `gateway run
/// --port <port>`.
///
/// Source: `src/commands/gateway-install.ts` - `gatewayInstallCommand`
pub async fn gateway_install_command(port: Option<u16>) -> Result<()> {
    let config = load_config().unwrap_or_default();
    let effective_port = port.unwrap_or_else(|| resolve_gateway_port(Some(&config)));
    let backend_label = service_backend_label();

    info!(
        port = effective_port,
        backend = backend_label,
        "gateway install requested"
    );

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
            "Gateway service installed as {backend_label} on port {effective_port}."
        )),
    );
    println!(
        "  {}",
        Theme::muted("The gateway will start automatically on login/boot."),
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
