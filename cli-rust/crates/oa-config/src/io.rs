/// Configuration I/O operations.
///
/// Handles loading, parsing, validating, and writing the OpenAcosmi config file.
/// Supports JSON5 format, `$include` directives, `${VAR}` env var substitution,
/// atomic writes with backup rotation, and SHA-256 content hashing.
///
/// Source: `src/config/io.ts`

use std::path::Path;

use anyhow::{Context, Result};
use sha2::{Digest, Sha256};
use serde_json::Value;
use tracing::warn;

use oa_types::config::{
    ConfigFileSnapshot, ConfigValidationIssue, OpenAcosmiConfig,
};

use crate::defaults::{apply_agent_defaults, apply_model_defaults, apply_session_defaults};
use crate::env_substitution::resolve_config_env_vars;
use crate::includes::resolve_config_includes;
use crate::paths::resolve_config_path;
use crate::validation::validate_config_object;

/// Maximum number of backup files retained during config writes.
const CONFIG_BACKUP_COUNT: usize = 5;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/// Compute the SHA-256 hex digest of a raw config string.
fn hash_config_raw(raw: Option<&str>) -> String {
    let mut hasher = Sha256::new();
    hasher.update(raw.unwrap_or(""));
    format!("{:x}", hasher.finalize())
}

/// Parse a raw string as JSON5 and return the result.
///
/// Returns `Ok(Value)` on success, or an error describing the parse failure.
pub fn parse_config_json5(raw: &str) -> Result<Value> {
    json5::from_str(raw).context("JSON5 parse failed")
}

// ---------------------------------------------------------------------------
// Load config (synchronous)
// ---------------------------------------------------------------------------

/// Load and parse the OpenAcosmi configuration from disk.
///
/// This is the primary synchronous entry point. It resolves the config path,
/// reads the file, processes `$include` directives, substitutes `${VAR}`
/// env var references, validates the result, and applies default values.
///
/// Returns a default (empty) config if the file does not exist or cannot be read.
pub fn load_config() -> Result<OpenAcosmiConfig> {
    let config_path = resolve_config_path();

    if !config_path.exists() {
        return Ok(OpenAcosmiConfig::default());
    }

    let raw = std::fs::read_to_string(&config_path)
        .with_context(|| format!("Failed to read config at {}", config_path.display()))?;

    let parsed = parse_config_json5(&raw)
        .with_context(|| format!("Failed to parse config at {}", config_path.display()))?;

    // Resolve $include directives
    let resolved = resolve_config_includes(parsed, &config_path)
        .with_context(|| "Failed to resolve $include directives")?;

    // Substitute ${VAR} env var references
    let substituted = resolve_config_env_vars(resolved)
        .with_context(|| "Failed to substitute env vars")?;

    // Validate and deserialize
    let cfg = validate_config_object(&substituted)?;

    // Apply defaults
    let cfg = apply_session_defaults(cfg);
    let cfg = apply_agent_defaults(cfg);
    let cfg = apply_model_defaults(cfg);

    Ok(cfg)
}

// ---------------------------------------------------------------------------
// Read config snapshot (async)
// ---------------------------------------------------------------------------

/// Read the config file and return a full snapshot including raw content,
/// parsed value, validation results, and the deserialized config.
///
/// This is used by commands that need to inspect the config state (e.g.,
/// `oa configure`, `oa doctor`).
pub async fn read_config_file_snapshot() -> Result<ConfigFileSnapshot> {
    let config_path = resolve_config_path();
    let path_str = config_path.display().to_string();

    if !config_path.exists() {
        let hash = hash_config_raw(None);
        return Ok(ConfigFileSnapshot {
            path: path_str,
            exists: false,
            raw: None,
            parsed: Some(Value::Object(serde_json::Map::new())),
            valid: true,
            config: OpenAcosmiConfig::default(),
            hash: Some(hash),
            issues: vec![],
            warnings: vec![],
            legacy_issues: vec![],
        });
    }

    let raw = tokio::fs::read_to_string(&config_path)
        .await
        .with_context(|| format!("Failed to read config at {path_str}"))?;

    let hash = hash_config_raw(Some(&raw));

    // Parse JSON5
    let parsed = match parse_config_json5(&raw) {
        Ok(v) => v,
        Err(e) => {
            return Ok(ConfigFileSnapshot {
                path: path_str,
                exists: true,
                raw: Some(raw),
                parsed: Some(Value::Object(serde_json::Map::new())),
                valid: false,
                config: OpenAcosmiConfig::default(),
                hash: Some(hash),
                issues: vec![ConfigValidationIssue {
                    path: String::new(),
                    message: format!("JSON5 parse failed: {e}"),
                }],
                warnings: vec![],
                legacy_issues: vec![],
            });
        }
    };

    // Resolve $include directives
    let resolved = match resolve_config_includes(parsed.clone(), &config_path) {
        Ok(v) => v,
        Err(e) => {
            return Ok(ConfigFileSnapshot {
                path: path_str,
                exists: true,
                raw: Some(raw),
                parsed: Some(parsed),
                valid: false,
                config: OpenAcosmiConfig::default(),
                hash: Some(hash),
                issues: vec![ConfigValidationIssue {
                    path: String::new(),
                    message: format!("Include resolution failed: {e}"),
                }],
                warnings: vec![],
                legacy_issues: vec![],
            });
        }
    };

    // Substitute ${VAR} env var references
    let substituted = match resolve_config_env_vars(resolved.clone()) {
        Ok(v) => v,
        Err(e) => {
            return Ok(ConfigFileSnapshot {
                path: path_str,
                exists: true,
                raw: Some(raw),
                parsed: Some(parsed),
                valid: false,
                config: OpenAcosmiConfig::default(),
                hash: Some(hash),
                issues: vec![ConfigValidationIssue {
                    path: String::new(),
                    message: format!("Env var substitution failed: {e}"),
                }],
                warnings: vec![],
                legacy_issues: vec![],
            });
        }
    };

    // Validate
    let config = match validate_config_object(&substituted) {
        Ok(cfg) => cfg,
        Err(e) => {
            return Ok(ConfigFileSnapshot {
                path: path_str,
                exists: true,
                raw: Some(raw),
                parsed: Some(parsed),
                valid: false,
                config: OpenAcosmiConfig::default(),
                hash: Some(hash),
                issues: vec![ConfigValidationIssue {
                    path: String::new(),
                    message: format!("Validation failed: {e}"),
                }],
                warnings: vec![],
                legacy_issues: vec![],
            });
        }
    };

    // Apply defaults
    let config = apply_session_defaults(config);
    let config = apply_agent_defaults(config);
    let config = apply_model_defaults(config);

    Ok(ConfigFileSnapshot {
        path: path_str,
        exists: true,
        raw: Some(raw),
        parsed: Some(parsed),
        valid: true,
        config,
        hash: Some(hash),
        issues: vec![],
        warnings: vec![],
        legacy_issues: vec![],
    })
}

// ---------------------------------------------------------------------------
// Write config (async)
// ---------------------------------------------------------------------------

/// Rotate backup files for the config, keeping up to `CONFIG_BACKUP_COUNT`.
///
/// Shifts `.bak.N` files up by one, removing the oldest.
async fn rotate_config_backups(config_path: &Path) {
    if CONFIG_BACKUP_COUNT <= 1 {
        return;
    }
    let backup_base = format!("{}.bak", config_path.display());
    let max_index = CONFIG_BACKUP_COUNT - 1;

    // Remove the oldest backup
    let _ = tokio::fs::remove_file(format!("{backup_base}.{max_index}")).await;

    // Shift backups up by one
    for index in (1..max_index).rev() {
        let _ = tokio::fs::rename(
            format!("{backup_base}.{index}"),
            format!("{backup_base}.{}", index + 1),
        )
        .await;
    }

    // Move current backup to .bak.1
    let _ = tokio::fs::rename(&backup_base, format!("{backup_base}.1")).await;
}

/// Write the configuration to disk using an atomic write strategy.
///
/// Steps:
/// 1. Serialize the config as pretty JSON
/// 2. Write to a temporary file
/// 3. Rotate existing backups
/// 4. Rename the temp file to the final path
///
/// File permissions are set to 0o600 (owner read/write only) on Unix.
pub async fn write_config_file(cfg: &OpenAcosmiConfig) -> Result<()> {
    let config_path = resolve_config_path();
    let dir = config_path
        .parent()
        .unwrap_or_else(|| Path::new("."));

    // Ensure directory exists
    tokio::fs::create_dir_all(dir)
        .await
        .with_context(|| format!("Failed to create config dir: {}", dir.display()))?;

    // Apply model defaults before writing
    let cfg_with_defaults = apply_model_defaults(cfg.clone());

    // Serialize as pretty JSON (not JSON5 -- we write standard JSON)
    let json = serde_json::to_string_pretty(&cfg_with_defaults)
        .context("Failed to serialize config")?;
    let json = json.trim_end().to_string() + "\n";

    // Write to temporary file
    let tmp = dir.join(format!(
        "{}.{}.{}.tmp",
        config_path
            .file_name()
            .unwrap_or_default()
            .to_string_lossy(),
        std::process::id(),
        uuid::Uuid::new_v4()
    ));

    tokio::fs::write(&tmp, &json)
        .await
        .with_context(|| format!("Failed to write temp config: {}", tmp.display()))?;

    // Set permissions on Unix
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        let perms = std::fs::Permissions::from_mode(0o600);
        let _ = tokio::fs::set_permissions(&tmp, perms).await;
    }

    // Rotate backups if config already exists
    if config_path.exists() {
        rotate_config_backups(&config_path).await;
        let backup_path = format!("{}.bak", config_path.display());
        let _ = tokio::fs::copy(&config_path, &backup_path).await;
    }

    // Atomic rename
    match tokio::fs::rename(&tmp, &config_path).await {
        Ok(()) => Ok(()),
        Err(e) => {
            // Fallback: copy + cleanup (e.g., on Windows when dest exists)
            warn!("Atomic rename failed, falling back to copy: {e}");
            tokio::fs::copy(&tmp, &config_path)
                .await
                .with_context(|| "Failed to copy temp config to final path")?;

            #[cfg(unix)]
            {
                use std::os::unix::fs::PermissionsExt;
                let perms = std::fs::Permissions::from_mode(0o600);
                let _ = tokio::fs::set_permissions(&config_path, perms).await;
            }

            let _ = tokio::fs::remove_file(&tmp).await;
            Ok(())
        }
    }
}
