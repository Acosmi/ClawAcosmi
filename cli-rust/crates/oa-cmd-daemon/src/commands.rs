/// Legacy daemon commands -- thin wrappers around gateway service operations.
///
/// These commands exist for backward compatibility. New code should use
/// the `gateway` subcommands directly.
///
/// Source: `src/cli/daemon-cli.ts`

use anyhow::Result;
use tracing::info;

/// Show gateway service status (legacy daemon alias).
///
/// Source: `src/cli/daemon-cli.ts` - `daemonStatus`
pub async fn daemon_status_command(json: bool) -> Result<()> {
    info!("daemon status is a legacy alias for gateway status");
    oa_cmd_gateway::status::gateway_status_command(json).await
}

/// Start the gateway service (legacy daemon alias).
///
/// Source: `src/cli/daemon-cli.ts` - `daemonStart`
pub async fn daemon_start_command() -> Result<()> {
    info!("daemon start is a legacy alias for gateway start");
    oa_cmd_gateway::start::gateway_start_command(None, false).await
}

/// Stop the gateway service (legacy daemon alias).
///
/// Source: `src/cli/daemon-cli.ts` - `daemonStop`
pub async fn daemon_stop_command() -> Result<()> {
    info!("daemon stop is a legacy alias for gateway stop");
    oa_cmd_gateway::stop::gateway_stop_command().await
}

/// Restart the gateway service (legacy daemon alias).
///
/// Performs a stop followed by a start.
///
/// Source: `src/cli/daemon-cli.ts` - `daemonRestart`
pub async fn daemon_restart_command() -> Result<()> {
    info!("daemon restart is a legacy alias for gateway stop + start");
    oa_cmd_gateway::stop::gateway_stop_command().await?;
    oa_cmd_gateway::start::gateway_start_command(None, false).await
}

/// Install the gateway service (legacy daemon alias).
///
/// Source: `src/cli/daemon-cli.ts` - `daemonInstall`
pub async fn daemon_install_command() -> Result<()> {
    info!("daemon install is a legacy alias for gateway install");
    oa_cmd_gateway::install::gateway_install_command(None).await
}

/// Uninstall the gateway service (legacy daemon alias).
///
/// Source: `src/cli/daemon-cli.ts` - `daemonUninstall`
pub async fn daemon_uninstall_command() -> Result<()> {
    info!("daemon uninstall is a legacy alias for gateway uninstall");
    oa_cmd_gateway::uninstall::gateway_uninstall_command().await
}
