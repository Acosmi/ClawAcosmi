//! Go builder — builds MCP servers using `go build`.

use std::path::Path;
use std::process::Command;

use sha2::{Digest, Sha256};

use crate::error::{McpInstallError, Result};
use crate::service::sanitized_env;
use crate::types::{BuildResult, McpServerManifest, ProjectType};

use super::McpBuilder;

/// Builds Go MCP servers via `go build`.
pub struct GoBuilder;

impl McpBuilder for GoBuilder {
    fn can_build(&self, project_type: &ProjectType) -> bool {
        *project_type == ProjectType::Go
    }

    fn build(&self, clone_dir: &Path, manifest: &McpServerManifest) -> Result<BuildResult> {
        tracing::info!(dir = %clone_dir.display(), "building Go MCP server");

        // Determine build args from manifest or use defaults
        let (cmd, args) = if let Some(ref build) = manifest.build {
            (build.command.as_str(), build.args.clone())
        } else {
            (
                "go",
                vec![
                    "build".into(),
                    "-o".into(),
                    manifest.name.clone(),
                    ".".into(),
                ],
            )
        };

        // Execute build with sanitized environment
        let output = Command::new(cmd)
            .args(&args)
            .current_dir(clone_dir)
            .env_clear()
            .envs(sanitized_env())
            .output()
            .map_err(|e| McpInstallError::BuildFailed(format!("{cmd}: {e}")))?;

        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            return Err(McpInstallError::BuildFailed(format!(
                "{cmd} exited with {}: {}",
                output.status,
                stderr.chars().take(2000).collect::<String>()
            )));
        }

        // Resolve output binary path
        let binary_path = resolve_binary_path(clone_dir, manifest)?;

        if !binary_path.exists() {
            return Err(McpInstallError::BuildFailed(format!(
                "expected binary not found at {}",
                binary_path.display()
            )));
        }

        // Compute SHA-256
        let data = std::fs::read(&binary_path)?;
        let hash = Sha256::digest(&data);
        let binary_sha256 = format!("{hash:x}");

        // Try to get source commit
        let source_commit = get_git_commit(clone_dir);

        tracing::info!(
            binary = %binary_path.display(),
            sha256 = &binary_sha256[..16],
            "Go build succeeded"
        );

        Ok(BuildResult {
            binary_path,
            binary_sha256,
            source_commit,
        })
    }
}

/// Resolve the output binary path from manifest or default Go convention.
fn resolve_binary_path(
    clone_dir: &Path,
    manifest: &McpServerManifest,
) -> Result<std::path::PathBuf> {
    if let Some(ref build) = manifest.build {
        if let Some(ref output) = build.output {
            return Ok(clone_dir.join(output));
        }
    }
    // Default: ./<name> in the clone dir
    Ok(clone_dir.join(&manifest.name))
}

/// Try to get the current git HEAD commit.
fn get_git_commit(dir: &Path) -> Option<String> {
    let output = Command::new("git")
        .args(["rev-parse", "HEAD"])
        .current_dir(dir)
        .output()
        .ok()?;

    if output.status.success() {
        Some(String::from_utf8_lossy(&output.stdout).trim().to_string())
    } else {
        None
    }
}
