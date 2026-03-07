//! McpInstallService — full installation pipeline orchestration.
//!
//! Coordinates: URL parse → clone/download → detect → build → register.
//! This is the main entry point for `openacosmi mcp install <url>`.

use std::collections::HashMap;
use std::path::Path;
use std::process::Command;

use crate::builder;
use crate::detect;
use crate::downloader;
use crate::error::{McpInstallError, Result};
use crate::manifest;
use crate::registry;
use crate::types::*;
use crate::url_parser;

/// Installation options.
#[derive(Debug, Clone, Default)]
pub struct InstallOptions {
    /// Pin to a specific git ref (tag/branch/commit). Defaults to latest tag if available.
    pub pinned_ref: Option<String>,
    /// Pre-configured environment variables (from interactive prompt or config).
    pub env: HashMap<String, String>,
    /// Expected SHA-256 for pre-compiled binary (optional).
    pub expected_sha256: Option<String>,
    /// Custom server name override (defaults to repo name).
    pub name_override: Option<String>,
    /// Force reinstall even if already installed.
    pub force: bool,
}

/// Full installation pipeline result.
#[derive(Debug)]
pub struct InstallResult {
    /// Registered server entry.
    pub server: InstalledMcpServer,
    /// Whether the server was downloaded (true) or built from source (false).
    pub was_downloaded: bool,
}

/// Run the complete installation pipeline.
///
/// Flow: URL parse → clone/download → detect → build → register
pub async fn install(url: &str, opts: &InstallOptions) -> Result<InstallResult> {
    let parsed = url_parser::parse_mcp_url(url)?;
    let registry_path = registry::default_registry_path()?;
    let managed = registry::managed_dir()?;
    let mut reg = registry::load_registry(&registry_path)?;

    // Determine server name
    let server_name = opts
        .name_override
        .clone()
        .or_else(|| parsed.repo.clone())
        .ok_or_else(|| McpInstallError::InvalidUrl("cannot determine server name".into()))?;

    // Check if already installed
    if !opts.force && reg.servers.contains_key(&server_name) {
        return Err(McpInstallError::ServerAlreadyInstalled(server_name));
    }

    // Route: Release asset download vs git clone + build
    let result = match parsed.kind {
        UrlKind::ReleaseAsset => {
            install_from_release(&parsed, &server_name, &managed, opts).await?
        }
        UrlKind::GitRepo | UrlKind::Ssh => {
            install_from_source(&parsed, &server_name, &managed, opts)?
        }
    };

    // Register
    registry::upsert_server(&mut reg, result.server.clone())?;
    registry::save_registry(&registry_path, &reg)?;

    tracing::info!(
        name = &server_name,
        binary = %result.server.binary_path.display(),
        "MCP server installed"
    );

    Ok(result)
}

/// Install from a release asset URL (download → verify → register).
async fn install_from_release(
    parsed: &ParsedMcpUrl,
    server_name: &str,
    managed: &Path,
    opts: &InstallOptions,
) -> Result<InstallResult> {
    let asset_url = parsed
        .release_asset_url
        .as_deref()
        .ok_or_else(|| McpInstallError::InvalidUrl("no release asset URL".into()))?;

    let asset_name = downloader::extract_asset_name(asset_url)
        .ok_or_else(|| McpInstallError::InvalidUrl("cannot extract asset name".into()))?;

    let dest = downloader::download_dest_path(managed, server_name, &asset_name);

    // Download
    downloader::download_release_asset(asset_url, &dest).await?;

    // SHA-256 verification
    let binary_sha256 = downloader::compute_sha256(&dest)?;
    if let Some(ref expected) = opts.expected_sha256 {
        downloader::verify_sha256(&dest, expected)?;
    }

    let now = chrono::Utc::now().to_rfc3339();
    let server = InstalledMcpServer {
        name: server_name.to_string(),
        source_url: parsed.original.clone(),
        source_kind: UrlKind::ReleaseAsset,
        project_type: ProjectType::Unknown, // binary download, type not applicable
        transport: TransportMode::Stdio,
        binary_path: dest,
        command: None,
        args: vec![],
        clone_dir: None,
        env: opts.env.clone(),
        pinned_ref: parsed.release_tag.clone(),
        source_commit: None,
        binary_sha256: Some(binary_sha256),
        installed_at: now,
        updated_at: None,
    };

    Ok(InstallResult {
        server,
        was_downloaded: true,
    })
}

/// Install from git source (clone → detect → build → register).
fn install_from_source(
    parsed: &ParsedMcpUrl,
    server_name: &str,
    managed: &Path,
    opts: &InstallOptions,
) -> Result<InstallResult> {
    let git_url = parsed
        .git_url
        .as_deref()
        .ok_or_else(|| McpInstallError::InvalidUrl("no git URL".into()))?;

    let clone_dir = managed.join(server_name).join("src");

    // Clone
    git_clone(git_url, &clone_dir, opts.pinned_ref.as_deref())?;

    // Detect project type
    let project_type = detect::detect_project_type(&clone_dir);
    if !detect::is_buildable(project_type) {
        return Err(McpInstallError::UnsupportedProjectType(format!(
            "{project_type} (only Rust and Go are supported in Phase 1)"
        )));
    }

    // Load or infer manifest
    let mcp_manifest = manifest::load_manifest(&clone_dir)?
        .unwrap_or(manifest::infer_manifest(project_type, server_name)?);

    // Validate build command
    manifest::validate_build_command(&mcp_manifest)?;

    // Build
    let mcp_builder = builder::select_builder(&project_type);
    let build_result = mcp_builder.build(&clone_dir, &mcp_manifest)?;

    // Copy binary to managed bin/ directory
    let bin_dir = managed.join(server_name).join("bin");
    std::fs::create_dir_all(&bin_dir)?;
    let final_binary = bin_dir.join(server_name);
    std::fs::copy(&build_result.binary_path, &final_binary)?;

    // Make executable
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        let perms = std::fs::Permissions::from_mode(0o755);
        std::fs::set_permissions(&final_binary, perms)?;
    }

    let now = chrono::Utc::now().to_rfc3339();
    let server = InstalledMcpServer {
        name: server_name.to_string(),
        source_url: parsed.original.clone(),
        source_kind: parsed.kind,
        project_type,
        transport: mcp_manifest.transport,
        binary_path: final_binary,
        command: mcp_manifest.command,
        args: mcp_manifest.args,
        clone_dir: Some(clone_dir),
        env: opts.env.clone(),
        pinned_ref: opts.pinned_ref.clone(),
        source_commit: build_result.source_commit,
        binary_sha256: Some(build_result.binary_sha256),
        installed_at: now,
        updated_at: None,
    };

    Ok(InstallResult {
        server,
        was_downloaded: false,
    })
}

/// Environment variable names to strip from clone and build subprocesses.
/// Prevents credential leakage and limits supply-chain attack surface.
const SANITIZED_ENV_VARS: &[&str] = &[
    // Cloud credentials
    "AWS_ACCESS_KEY_ID",
    "AWS_SECRET_ACCESS_KEY",
    "AWS_SESSION_TOKEN",
    "AZURE_CLIENT_SECRET",
    "GOOGLE_APPLICATION_CREDENTIALS",
    "GCP_SERVICE_ACCOUNT_KEY",
    // Source control tokens
    "GITHUB_TOKEN",
    "GH_TOKEN",
    "GITLAB_TOKEN",
    "BITBUCKET_TOKEN",
    // API keys
    "OPENAI_API_KEY",
    "ANTHROPIC_API_KEY",
    "HF_TOKEN",
    // SSH (builds should not need SSH agent)
    "SSH_AUTH_SOCK",
    "SSH_AGENT_PID",
    // NPM/Cargo/Go publishing tokens
    "NPM_TOKEN",
    "CARGO_REGISTRY_TOKEN",
    "GONOSUMCHECK",
    "GONOSUMDB",
    "GONOPROXY",
    "GOFLAGS",
];

/// Build a sanitized environment for subprocess execution.
/// Used by clone and build commands to prevent credential leakage.
pub fn sanitized_env() -> Vec<(String, String)> {
    std::env::vars()
        .filter(|(k, _)| !SANITIZED_ENV_VARS.contains(&k.as_str()))
        .collect()
}

/// Clone a git repository.
fn git_clone(url: &str, dest: &Path, pinned_ref: Option<&str>) -> Result<()> {
    // Remove existing clone dir if present (force reinstall)
    if dest.exists() {
        std::fs::remove_dir_all(dest)?;
    }

    let mut args = vec!["clone".to_string(), "--depth".to_string(), "1".to_string()];
    if let Some(git_ref) = pinned_ref {
        args.push("--branch".into());
        args.push(git_ref.into());
    }
    args.push(url.to_string());
    args.push(dest.display().to_string());

    tracing::info!(url = url, dest = %dest.display(), "cloning repository");

    let output = Command::new("git")
        .args(&args)
        .env_clear()
        .envs(sanitized_env())
        .output()
        .map_err(|e| McpInstallError::GitCloneFailed(format!("git: {e}")))?;

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        return Err(McpInstallError::GitCloneFailed(format!(
            "git clone exited with {}: {}",
            output.status,
            stderr.chars().take(2000).collect::<String>()
        )));
    }

    Ok(())
}

/// Uninstall an MCP server by name.
pub fn uninstall(name: &str) -> Result<()> {
    let registry_path = registry::default_registry_path()?;
    let managed = registry::managed_dir()?;
    let mut reg = registry::load_registry(&registry_path)?;

    let removed = registry::remove_server(&mut reg, name)?;
    registry::save_registry(&registry_path, &reg)?;

    // Clean up files
    let server_dir = managed.join(name);
    if server_dir.exists() {
        std::fs::remove_dir_all(&server_dir)?;
    }

    // Also try to remove clone dir if it was elsewhere
    if let Some(ref clone_dir) = removed.clone_dir {
        if clone_dir.exists() && !clone_dir.starts_with(&server_dir) {
            let _ = std::fs::remove_dir_all(clone_dir);
        }
    }

    tracing::info!(name = name, "MCP server uninstalled");
    Ok(())
}

/// List all installed servers.
pub fn list() -> Result<Vec<InstalledMcpServer>> {
    let registry_path = registry::default_registry_path()?;
    let reg = registry::load_registry(&registry_path)?;
    let mut servers: Vec<InstalledMcpServer> = reg.servers.into_values().collect();
    servers.sort_by(|a, b| a.name.cmp(&b.name));
    Ok(servers)
}

/// Get a specific installed server.
pub fn get(name: &str) -> Result<InstalledMcpServer> {
    let registry_path = registry::default_registry_path()?;
    let reg = registry::load_registry(&registry_path)?;
    reg.servers
        .get(name)
        .cloned()
        .ok_or_else(|| McpInstallError::ServerNotFound(name.into()))
}
