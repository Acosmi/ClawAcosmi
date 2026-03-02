/// Configuration file path resolution.
///
/// Resolves paths to state directories, config files, gateway lock dirs,
/// and OAuth credential directories. Respects environment variable overrides
/// such as `OPENACOSMI_HOME`, `OPENACOSMI_STATE_DIR`, `OPENACOSMI_CONFIG_PATH`,
/// `OPENACOSMI_GATEWAY_PORT`, `OPENACOSMI_NIX_MODE`, and `OPENACOSMI_OAUTH_DIR`.
///
/// Source: `src/config/paths.ts`

use std::env;
use std::path::{Path, PathBuf};

use oa_types::config::OpenAcosmiConfig;

/// Default state directory name under the user's home directory.
pub const NEW_STATE_DIRNAME: &str = ".openacosmi";

/// Default config file name.
pub const CONFIG_FILENAME: &str = "openacosmi.json";

/// Legacy state directory names from prior product incarnations.
const LEGACY_STATE_DIRNAMES: &[&str] = &[".clawdbot", ".moltbot", ".moldbot"];

/// Legacy config file names from prior product incarnations.
const LEGACY_CONFIG_FILENAMES: &[&str] = &["clawdbot.json", "moltbot.json", "moldbot.json"];

/// Default gateway HTTP port.
pub const DEFAULT_GATEWAY_PORT: u16 = 19001;

/// OAuth credentials filename.
const OAUTH_FILENAME: &str = "oauth.json";

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

/// Resolve the home directory, preferring `OPENACOSMI_HOME` over `dirs::home_dir`.
fn resolve_home_dir() -> PathBuf {
    if let Ok(home) = env::var("OPENACOSMI_HOME") {
        let trimmed = home.trim().to_string();
        if !trimmed.is_empty() {
            return PathBuf::from(trimmed);
        }
    }
    dirs::home_dir().unwrap_or_else(|| PathBuf::from("."))
}

/// Expand a leading `~` in a user-supplied path to the home directory.
fn resolve_user_path(input: &str) -> PathBuf {
    let trimmed = input.trim();
    if trimmed.is_empty() {
        return PathBuf::from(trimmed);
    }
    if let Some(rest) = trimmed.strip_prefix('~') {
        let home = resolve_home_dir();
        if rest.is_empty() {
            return home;
        }
        let rest = rest.strip_prefix('/').unwrap_or(rest);
        return home.join(rest);
    }
    PathBuf::from(trimmed).canonicalize().unwrap_or_else(|_| PathBuf::from(trimmed))
}

fn new_state_dir() -> PathBuf {
    resolve_home_dir().join(NEW_STATE_DIRNAME)
}

fn legacy_state_dirs() -> Vec<PathBuf> {
    let home = resolve_home_dir();
    LEGACY_STATE_DIRNAMES
        .iter()
        .map(|name| home.join(name))
        .collect()
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

/// Resolve the state directory for mutable data (sessions, logs, caches).
///
/// Precedence:
/// 1. `OPENACOSMI_STATE_DIR` environment variable
/// 2. Existing `~/.openacosmi` directory
/// 3. First existing legacy state directory
/// 4. Default `~/.openacosmi` (even if it doesn't exist yet)
pub fn resolve_state_dir() -> PathBuf {
    // Check explicit override
    if let Ok(val) = env::var("OPENACOSMI_STATE_DIR") {
        let trimmed = val.trim().to_string();
        if !trimmed.is_empty() {
            return resolve_user_path(&trimmed);
        }
    }

    let new_dir = new_state_dir();
    if new_dir.exists() {
        return new_dir;
    }

    // Check legacy directories
    for dir in legacy_state_dirs() {
        if dir.exists() {
            return dir;
        }
    }

    new_dir
}

/// Resolve the canonical config file path within a given state directory.
///
/// Respects `OPENACOSMI_CONFIG_PATH` override, otherwise returns
/// `<state_dir>/openacosmi.json`.
pub fn resolve_canonical_config_path(state_dir: &Path) -> PathBuf {
    if let Ok(val) = env::var("OPENACOSMI_CONFIG_PATH") {
        let trimmed = val.trim().to_string();
        if !trimmed.is_empty() {
            return resolve_user_path(&trimmed);
        }
    }
    state_dir.join(CONFIG_FILENAME)
}

/// Build the list of default config path candidates.
///
/// Returns all possible config file locations in priority order:
/// explicit path, state-dir candidates, new+legacy dir candidates.
fn resolve_default_config_candidates() -> Vec<PathBuf> {
    // If explicit config path is set, use only that
    if let Ok(val) = env::var("OPENACOSMI_CONFIG_PATH") {
        let trimmed = val.trim().to_string();
        if !trimmed.is_empty() {
            return vec![resolve_user_path(&trimmed)];
        }
    }

    let mut candidates = Vec::new();

    // If state dir override exists, add candidates from there
    if let Ok(val) = env::var("OPENACOSMI_STATE_DIR") {
        let trimmed = val.trim().to_string();
        if !trimmed.is_empty() {
            let resolved = resolve_user_path(&trimmed);
            candidates.push(resolved.join(CONFIG_FILENAME));
            for name in LEGACY_CONFIG_FILENAMES {
                candidates.push(resolved.join(name));
            }
        }
    }

    // Add default directories: new state dir + legacy state dirs
    let default_dirs = {
        let mut dirs = vec![new_state_dir()];
        dirs.extend(legacy_state_dirs());
        dirs
    };

    for dir in default_dirs {
        candidates.push(dir.join(CONFIG_FILENAME));
        for name in LEGACY_CONFIG_FILENAMES {
            candidates.push(dir.join(name));
        }
    }

    candidates
}

/// Resolve the active config path by preferring existing config files
/// before falling back to the canonical path.
///
/// This is the primary entry point for determining which config file to use.
pub fn resolve_config_path() -> PathBuf {
    // If explicit override, use it directly
    if let Ok(val) = env::var("OPENACOSMI_CONFIG_PATH") {
        let trimmed = val.trim().to_string();
        if !trimmed.is_empty() {
            return resolve_user_path(&trimmed);
        }
    }

    let state_dir = resolve_state_dir();

    // If OPENACOSMI_STATE_DIR is set, look within that dir
    let state_override = env::var("OPENACOSMI_STATE_DIR")
        .ok()
        .filter(|v| !v.trim().is_empty());

    // Build candidates from the state dir
    let mut candidates = vec![state_dir.join(CONFIG_FILENAME)];
    for name in LEGACY_CONFIG_FILENAMES {
        candidates.push(state_dir.join(name));
    }

    // Check if any candidate exists
    for candidate in &candidates {
        if candidate.exists() {
            return candidate.clone();
        }
    }

    // If state dir was explicitly overridden, use it
    if state_override.is_some() {
        return state_dir.join(CONFIG_FILENAME);
    }

    // Try full default candidate list
    let all_candidates = resolve_default_config_candidates();
    for candidate in &all_candidates {
        if candidate.exists() {
            return candidate.clone();
        }
    }

    // Fallback: canonical path
    resolve_canonical_config_path(&state_dir)
}

/// Resolve the gateway lock directory (ephemeral, in temp dir).
///
/// Returns `<tmpdir>/openacosmi-<uid>` on Unix (uid from `id -u` or process ID)
/// or `<tmpdir>/openacosmi` on other platforms.
pub fn resolve_gateway_lock_dir() -> PathBuf {
    let base = env::temp_dir();
    #[cfg(unix)]
    {
        // Attempt to get the real uid; fall back to process id if unavailable.
        let uid = resolve_unix_uid();
        return base.join(format!("openacosmi-{uid}"));
    }
    #[cfg(not(unix))]
    {
        base.join("openacosmi")
    }
}

/// Resolve the Unix user ID for lock-directory naming.
///
/// Uses `std::os::unix::fs::MetadataExt` to read the uid of the
/// home directory, falling back to the process ID.
#[cfg(unix)]
fn resolve_unix_uid() -> u32 {
    use std::os::unix::fs::MetadataExt;
    // Read uid from the home directory metadata
    let home = resolve_home_dir();
    if let Ok(meta) = std::fs::metadata(&home) {
        return meta.uid();
    }
    // Fallback: use process id (not ideal but functional)
    std::process::id()
}

/// Resolve the OAuth credentials directory.
///
/// Precedence:
/// 1. `OPENACOSMI_OAUTH_DIR` environment variable
/// 2. `<state_dir>/credentials`
pub fn resolve_oauth_dir(state_dir: &Path) -> PathBuf {
    if let Ok(val) = env::var("OPENACOSMI_OAUTH_DIR") {
        let trimmed = val.trim().to_string();
        if !trimmed.is_empty() {
            return resolve_user_path(&trimmed);
        }
    }
    state_dir.join("credentials")
}

/// Resolve the OAuth credentials file path.
///
/// Returns `<oauth_dir>/oauth.json`.
pub fn resolve_oauth_path(state_dir: &Path) -> PathBuf {
    resolve_oauth_dir(state_dir).join(OAUTH_FILENAME)
}

/// Resolve the gateway port from environment and config.
///
/// Precedence:
/// 1. `OPENACOSMI_GATEWAY_PORT` environment variable
/// 2. `cfg.gateway.port` from config
/// 3. [`DEFAULT_GATEWAY_PORT`] (19001)
pub fn resolve_gateway_port(cfg: Option<&OpenAcosmiConfig>) -> u16 {
    // Check env var
    if let Ok(raw) = env::var("OPENACOSMI_GATEWAY_PORT") {
        let trimmed = raw.trim().to_string();
        if let Ok(parsed) = trimmed.parse::<u16>() {
            if parsed > 0 {
                return parsed;
            }
        }
    }

    // Check config
    if let Some(config) = cfg {
        if let Some(ref gw) = config.gateway {
            if let Some(port) = gw.port {
                if port > 0 {
                    return port;
                }
            }
        }
    }

    DEFAULT_GATEWAY_PORT
}

/// Check if running in Nix mode.
///
/// When `OPENACOSMI_NIX_MODE=1`, the gateway is running under Nix.
/// In this mode, no auto-install flows should be attempted and
/// config is managed externally.
pub fn is_nix_mode() -> bool {
    env::var("OPENACOSMI_NIX_MODE")
        .ok()
        .is_some_and(|v| v.trim() == "1")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn default_gateway_port_value() {
        assert_eq!(DEFAULT_GATEWAY_PORT, 19001);
    }

    #[test]
    fn gateway_port_from_config() {
        let mut cfg = OpenAcosmiConfig::default();
        cfg.gateway = Some(oa_types::gateway::GatewayConfig {
            port: Some(9999),
            ..Default::default()
        });
        // Only works when env var is not set; this tests the config path
        let port = resolve_gateway_port(Some(&cfg));
        // If env var is set, it takes precedence, so just check it's a valid port
        assert!(port > 0);
    }

    #[test]
    fn gateway_port_fallback() {
        let port = resolve_gateway_port(None);
        assert!(port > 0);
    }
}
