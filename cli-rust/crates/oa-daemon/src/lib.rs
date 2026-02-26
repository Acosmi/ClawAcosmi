/// Daemon service management for OpenAcosmi CLI.
///
/// Handles launchd (macOS) and systemd (Linux) service management
/// for the OpenAcosmi gateway and node services. Uses conditional
/// compilation for platform-specific code.
///
/// Source: `src/daemon/*.ts`

pub mod constants;
pub mod paths;
pub mod service;

#[cfg(target_os = "macos")]
pub mod launchd;

#[cfg(target_os = "linux")]
pub mod systemd;
