/// Onboarding helper utilities.
///
/// Provides token generation, path resolution, config summarization,
/// browser detection, gateway probing, reset handling, and workspace
/// initialization helpers used throughout the onboarding flow.
///
/// Source: `src/commands/onboard-helpers.ts`

use std::path::Path;

use anyhow::Result;
use tracing::{info, warn};

use crate::types::ResetScope;

/// Default workspace directory.
///
/// Source: `src/commands/onboard-helpers.ts` - `DEFAULT_WORKSPACE`
pub const DEFAULT_WORKSPACE: &str = "~/openacosmi";

/// Default local gateway WebSocket URL.
///
/// Source: `src/commands/onboard-remote.ts` - `DEFAULT_GATEWAY_URL`
pub const DEFAULT_GATEWAY_URL: &str = "ws://127.0.0.1:18789";

/// Generate a random hex token (48 hex chars = 24 random bytes).
///
/// Source: `src/commands/onboard-helpers.ts` - `randomToken`
pub fn random_token() -> String {
    use std::fmt::Write;
    let mut bytes = [0u8; 24];
    #[cfg(not(test))]
    {
        use std::io::Read;
        if let Ok(mut f) = std::fs::File::open("/dev/urandom") {
            let _ = f.read_exact(&mut bytes);
        }
    }
    #[cfg(test)]
    {
        // Deterministic for tests
        for (i, b) in bytes.iter_mut().enumerate() {
            *b = (i as u8).wrapping_mul(7).wrapping_add(42);
        }
    }
    let mut hex = String::with_capacity(48);
    for b in &bytes {
        let _ = write!(hex, "{b:02x}");
    }
    hex
}

/// Normalize a gateway token input string, trimming whitespace.
///
/// Source: `src/commands/onboard-helpers.ts` - `normalizeGatewayTokenInput`
pub fn normalize_gateway_token_input(value: &str) -> String {
    value.trim().to_string()
}

/// Resolve a user path, expanding `~` to the home directory.
///
/// Source: `src/commands/onboard-helpers.ts` (via `src/utils.ts` - `resolveUserPath`)
pub fn resolve_user_path(input: &str) -> String {
    let trimmed = input.trim();
    if trimmed.is_empty() {
        return trimmed.to_string();
    }
    if let Some(rest) = trimmed.strip_prefix('~') {
        if let Some(home) = dirs::home_dir() {
            let home_str = home.to_string_lossy();
            if rest.is_empty() {
                return home_str.to_string();
            }
            let rest = rest.strip_prefix('/').unwrap_or(rest);
            return format!("{home_str}/{rest}");
        }
    }
    trimmed.to_string()
}

/// Shorten a path by replacing the home directory prefix with `~`.
///
/// Source: `src/commands/onboard-helpers.ts` (via `src/utils.ts` - `shortenHomePath`)
pub fn shorten_home_path(path: &str) -> String {
    if let Some(home) = dirs::home_dir() {
        let home_str = home.to_string_lossy();
        if let Some(rest) = path.strip_prefix(home_str.as_ref()) {
            return format!("~{rest}");
        }
    }
    path.to_string()
}

/// Print the wizard header banner.
///
/// Source: `src/commands/onboard-helpers.ts` - `printWizardHeader`
pub fn print_wizard_header() {
    let header = [
        "\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}\u{2584}",
        "\u{2588}\u{2588}\u{2591}\u{2584}\u{2584}\u{2584}\u{2591}\u{2588}\u{2588}\u{2591}\u{2584}\u{2584}\u{2591}\u{2588}\u{2588}\u{2591}\u{2584}\u{2584}\u{2584}\u{2588}\u{2588}\u{2591}\u{2580}\u{2588}\u{2588}\u{2591}\u{2588}\u{2588}\u{2591}\u{2584}\u{2584}\u{2580}\u{2588}\u{2588}\u{2591}\u{2588}\u{2588}\u{2588}\u{2588}\u{2591}\u{2584}\u{2584}\u{2580}\u{2588}\u{2588}\u{2591}\u{2588}\u{2588}\u{2588}\u{2591}\u{2588}\u{2588}",
        "                  OPENACOSMI                    ",
        " ",
    ];
    for line in &header {
        info!("{line}");
    }
}

/// Probe whether a gateway is reachable at the given URL.
///
/// Returns `Ok(true)` if reachable, `Ok(false)` otherwise.
///
/// Source: `src/commands/onboard-helpers.ts` - `probeGatewayReachable`
pub async fn probe_gateway_reachable(
    url: &str,
    _token: Option<&str>,
    _password: Option<&str>,
    timeout_ms: Option<u64>,
) -> ProbeResult {
    let timeout_ms = timeout_ms.unwrap_or(1500);
    let url = url.trim();

    // Attempt a basic TCP connection to check reachability
    let parsed = match url::Url::parse(url) {
        Ok(u) => u,
        Err(e) => {
            return ProbeResult {
                ok: false,
                detail: Some(format!("Invalid URL: {e}")),
            };
        }
    };

    let host = parsed.host_str().unwrap_or("127.0.0.1");
    let port = parsed.port().unwrap_or(18789);
    let addr = format!("{host}:{port}");

    let timeout = std::time::Duration::from_millis(timeout_ms);
    match tokio::time::timeout(timeout, tokio::net::TcpStream::connect(&addr)).await {
        Ok(Ok(_)) => ProbeResult {
            ok: true,
            detail: None,
        },
        Ok(Err(e)) => ProbeResult {
            ok: false,
            detail: Some(format!("Connection refused: {e}")),
        },
        Err(_) => ProbeResult {
            ok: false,
            detail: Some("Connection timed out".to_string()),
        },
    }
}

/// Result of a gateway probe.
///
/// Source: `src/commands/onboard-helpers.ts` - `probeGatewayReachable` return type
#[derive(Debug, Clone)]
pub struct ProbeResult {
    /// Whether the gateway was reachable.
    pub ok: bool,
    /// Optional detail message on failure.
    pub detail: Option<String>,
}

/// Wait for a gateway to become reachable, polling periodically.
///
/// Source: `src/commands/onboard-helpers.ts` - `waitForGatewayReachable`
pub async fn wait_for_gateway_reachable(
    url: &str,
    token: Option<&str>,
    password: Option<&str>,
    deadline_ms: Option<u64>,
    poll_ms: Option<u64>,
) -> ProbeResult {
    let deadline_ms = deadline_ms.unwrap_or(15_000);
    let poll_ms = poll_ms.unwrap_or(400);
    let started_at = std::time::Instant::now();
    let mut last_detail: Option<String> = None;

    while started_at.elapsed().as_millis() < u128::from(deadline_ms) {
        let probe = probe_gateway_reachable(url, token, password, Some(1500)).await;
        if probe.ok {
            return probe;
        }
        last_detail = probe.detail;
        tokio::time::sleep(std::time::Duration::from_millis(poll_ms)).await;
    }

    ProbeResult {
        ok: false,
        detail: last_detail,
    }
}

/// Handle a reset operation by removing config, credentials, sessions, and workspace.
///
/// Source: `src/commands/onboard-helpers.ts` - `handleReset`
pub async fn handle_reset(scope: ResetScope, workspace_dir: &str) -> Result<()> {
    let config_path = oa_config::paths::resolve_config_path();
    move_to_trash(&config_path.to_string_lossy()).await;

    if scope == ResetScope::Config {
        return Ok(());
    }

    let state_dir = oa_config::paths::resolve_state_dir();
    let creds_dir = state_dir.join("credentials");
    move_to_trash(&creds_dir.to_string_lossy()).await;

    // Sessions dir
    let sessions_dir = state_dir.join("sessions");
    move_to_trash(&sessions_dir.to_string_lossy()).await;

    if scope == ResetScope::Full {
        move_to_trash(workspace_dir).await;
    }

    Ok(())
}

/// Attempt to move a path to the trash using the `trash` command.
///
/// Source: `src/commands/onboard-helpers.ts` - `moveToTrash`
async fn move_to_trash(pathname: &str) {
    if pathname.is_empty() {
        return;
    }
    let path = Path::new(pathname);
    if !path.exists() {
        return;
    }
    match tokio::process::Command::new("trash")
        .arg(pathname)
        .output()
        .await
    {
        Ok(output) if output.status.success() => {
            info!("Moved to Trash: {}", shorten_home_path(pathname));
        }
        _ => {
            warn!(
                "Failed to move to Trash (manual delete): {}",
                shorten_home_path(pathname)
            );
        }
    }
}

/// Detect whether a binary is available in PATH.
///
/// Source: `src/commands/onboard-helpers.ts` - `detectBinary`
pub async fn detect_binary(name: &str) -> bool {
    let name = name.trim();
    if name.is_empty() {
        return false;
    }
    which::which(name).is_ok()
}

/// Resolve the node manager options for interactive selection.
///
/// Source: `src/commands/onboard-helpers.ts` - `resolveNodeManagerOptions`
pub fn resolve_node_manager_options() -> Vec<NodeManagerOption> {
    vec![
        NodeManagerOption {
            value: "npm",
            label: "npm",
        },
        NodeManagerOption {
            value: "pnpm",
            label: "pnpm",
        },
        NodeManagerOption {
            value: "bun",
            label: "bun",
        },
    ]
}

/// Node manager option for interactive selection.
///
/// Source: `src/commands/onboard-helpers.ts` - `resolveNodeManagerOptions`
pub struct NodeManagerOption {
    /// The value identifier.
    pub value: &'static str,
    /// Display label.
    pub label: &'static str,
}

/// Resolve control UI links (HTTP and WS URLs) based on gateway configuration.
///
/// Source: `src/commands/onboard-helpers.ts` - `resolveControlUiLinks`
pub fn resolve_control_ui_links(
    port: u16,
    bind: Option<&str>,
    custom_bind_host: Option<&str>,
    base_path: Option<&str>,
) -> ControlUiLinks {
    let host = match bind.unwrap_or("loopback") {
        "custom" => custom_bind_host
            .filter(|h| !h.is_empty())
            .unwrap_or("127.0.0.1"),
        "lan" => "0.0.0.0",
        _ => "127.0.0.1",
    };

    let base_path = base_path
        .map(|p| p.trim_matches('/'))
        .filter(|p| !p.is_empty());
    let ui_path = base_path
        .map(|p| format!("/{p}/"))
        .unwrap_or_else(|| "/".to_string());
    let ws_path = base_path
        .map(|p| format!("/{p}"))
        .unwrap_or_default();

    ControlUiLinks {
        http_url: format!("http://{host}:{port}{ui_path}"),
        ws_url: format!("ws://{host}:{port}{ws_path}"),
    }
}

/// Control UI URL links.
///
/// Source: `src/commands/onboard-helpers.ts` - `resolveControlUiLinks` return type
#[derive(Debug, Clone)]
pub struct ControlUiLinks {
    /// HTTP URL for the web UI.
    pub http_url: String,
    /// WebSocket URL for gateway connections.
    pub ws_url: String,
}

/// Validate an IPv4 address string.
///
/// Source: `src/commands/onboard-helpers.ts` - `isValidIPv4`
pub fn is_valid_ipv4(host: &str) -> bool {
    let parts: Vec<&str> = host.split('.').collect();
    if parts.len() != 4 {
        return false;
    }
    parts.iter().all(|part| {
        part.parse::<u16>()
            .ok()
            .is_some_and(|n| n <= 255 && *part == n.to_string())
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn random_token_length() {
        let token = random_token();
        assert_eq!(token.len(), 48);
        // Should be hex characters only
        assert!(token.chars().all(|c| c.is_ascii_hexdigit()));
    }

    #[test]
    fn normalize_gateway_token_trims() {
        assert_eq!(normalize_gateway_token_input("  abc  "), "abc");
        assert_eq!(normalize_gateway_token_input(""), "");
    }

    #[test]
    fn resolve_user_path_tilde() {
        let path = resolve_user_path("~/test");
        assert!(!path.starts_with('~'));
        assert!(path.ends_with("/test") || path.ends_with("\\test"));
    }

    #[test]
    fn resolve_user_path_no_tilde() {
        assert_eq!(resolve_user_path("/absolute/path"), "/absolute/path");
        assert_eq!(resolve_user_path("relative"), "relative");
    }

    #[test]
    fn resolve_user_path_empty() {
        assert_eq!(resolve_user_path(""), "");
        assert_eq!(resolve_user_path("  "), "");
    }

    #[test]
    fn is_valid_ipv4_tests() {
        assert!(is_valid_ipv4("127.0.0.1"));
        assert!(is_valid_ipv4("192.168.1.100"));
        assert!(is_valid_ipv4("0.0.0.0"));
        assert!(is_valid_ipv4("255.255.255.255"));
        assert!(!is_valid_ipv4("256.0.0.1"));
        assert!(!is_valid_ipv4("1.2.3"));
        assert!(!is_valid_ipv4("abc"));
        assert!(!is_valid_ipv4(""));
    }

    #[test]
    fn control_ui_links_loopback() {
        let links = resolve_control_ui_links(18789, Some("loopback"), None, None);
        assert_eq!(links.http_url, "http://127.0.0.1:18789/");
        assert_eq!(links.ws_url, "ws://127.0.0.1:18789");
    }

    #[test]
    fn control_ui_links_lan() {
        let links = resolve_control_ui_links(9999, Some("lan"), None, None);
        assert_eq!(links.http_url, "http://0.0.0.0:9999/");
        assert_eq!(links.ws_url, "ws://0.0.0.0:9999");
    }

    #[test]
    fn control_ui_links_custom() {
        let links =
            resolve_control_ui_links(18789, Some("custom"), Some("192.168.1.100"), None);
        assert_eq!(links.http_url, "http://192.168.1.100:18789/");
    }

    #[test]
    fn control_ui_links_with_base_path() {
        let links = resolve_control_ui_links(18789, None, None, Some("gateway"));
        assert_eq!(links.http_url, "http://127.0.0.1:18789/gateway/");
        assert_eq!(links.ws_url, "ws://127.0.0.1:18789/gateway");
    }

    #[test]
    fn default_workspace_value() {
        assert_eq!(DEFAULT_WORKSPACE, "~/openacosmi");
    }

    #[test]
    fn default_gateway_url_value() {
        assert_eq!(DEFAULT_GATEWAY_URL, "ws://127.0.0.1:18789");
    }

    #[test]
    fn node_manager_options_count() {
        let options = resolve_node_manager_options();
        assert_eq!(options.len(), 3);
        assert_eq!(options[0].value, "npm");
        assert_eq!(options[1].value, "pnpm");
        assert_eq!(options[2].value, "bun");
    }
}
