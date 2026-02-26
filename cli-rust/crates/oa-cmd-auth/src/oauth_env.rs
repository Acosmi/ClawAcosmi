/// OAuth environment detection (remote/VPS vs local).
///
/// Detects whether the current environment is remote (SSH, containers,
/// headless Linux) so that OAuth flows can adapt to show manual URL
/// instructions instead of attempting to open a local browser.
///
/// Source: `src/commands/oauth-env.ts`

/// Check whether the current process is running in a remote/VPS environment.
///
/// Returns `true` when any of the following conditions are met:
/// - SSH session indicators (`SSH_CLIENT`, `SSH_TTY`, `SSH_CONNECTION`)
/// - Container indicators (`REMOTE_CONTAINERS`, `CODESPACES`)
/// - Headless Linux (no `DISPLAY` or `WAYLAND_DISPLAY`) that is not WSL
///
/// Source: `src/commands/oauth-env.ts` - `isRemoteEnvironment`
#[must_use]
pub fn is_remote_environment() -> bool {
    // SSH session detection
    if std::env::var("SSH_CLIENT").is_ok()
        || std::env::var("SSH_TTY").is_ok()
        || std::env::var("SSH_CONNECTION").is_ok()
    {
        return true;
    }

    // Container / cloud IDE detection
    if std::env::var("REMOTE_CONTAINERS").is_ok() || std::env::var("CODESPACES").is_ok() {
        return true;
    }

    // Headless Linux (no display server, not WSL)
    if cfg!(target_os = "linux")
        && std::env::var("DISPLAY").is_err()
        && std::env::var("WAYLAND_DISPLAY").is_err()
        && !is_wsl()
    {
        return true;
    }

    false
}

/// Check if we are running in Windows Subsystem for Linux.
///
/// Source: `src/infra/wsl.ts` - `isWSLEnv`
fn is_wsl() -> bool {
    if !cfg!(target_os = "linux") {
        return false;
    }

    // Check WSL-specific environment variables
    if std::env::var("WSL_DISTRO_NAME").is_ok() || std::env::var("WSL_INTEROP").is_ok() {
        return true;
    }

    // Check /proc/version for WSL indicator
    std::fs::read_to_string("/proc/version")
        .ok()
        .map_or(false, |content| {
            let lower = content.to_lowercase();
            lower.contains("microsoft") || lower.contains("wsl")
        })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn is_remote_returns_bool() {
        // We cannot control the environment in unit tests, but we can at
        // least verify the function runs without panicking.
        let _result = is_remote_environment();
    }

    #[test]
    fn is_wsl_returns_bool() {
        let _result = is_wsl();
    }

    #[test]
    fn remote_detection_does_not_panic() {
        // Ensure the function handles missing env vars gracefully
        let result = is_remote_environment();
        assert!(result || !result); // always true, just ensuring no panic
    }
}
