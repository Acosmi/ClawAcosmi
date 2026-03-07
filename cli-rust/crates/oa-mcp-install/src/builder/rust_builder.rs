//! Rust builder — builds MCP servers using `cargo build --release`.

use std::path::Path;
use std::process::Command;

use sha2::{Digest, Sha256};

use crate::error::{McpInstallError, Result};
use crate::service::sanitized_env;
use crate::types::{BuildResult, McpServerManifest, ProjectType};

use super::McpBuilder;

/// Builds Rust MCP servers via `cargo build --release`.
pub struct RustBuilder;

impl McpBuilder for RustBuilder {
    fn can_build(&self, project_type: &ProjectType) -> bool {
        *project_type == ProjectType::Rust
    }

    fn build(&self, clone_dir: &Path, manifest: &McpServerManifest) -> Result<BuildResult> {
        tracing::info!(dir = %clone_dir.display(), "building Rust MCP server");

        // Determine build args from manifest or use defaults
        let (cmd, args) = if let Some(ref build) = manifest.build {
            (build.command.as_str(), build.args.clone())
        } else {
            ("cargo", vec!["build".into(), "--release".into()])
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
        let binary_sha256 = compute_sha256(&binary_path)?;

        // Try to get source commit
        let source_commit = get_git_commit(clone_dir);

        tracing::info!(
            binary = %binary_path.display(),
            sha256 = &binary_sha256[..16],
            "Rust build succeeded"
        );

        Ok(BuildResult {
            binary_path,
            binary_sha256,
            source_commit,
        })
    }
}

/// Resolve the output binary path from manifest or default Cargo convention.
fn resolve_binary_path(
    clone_dir: &Path,
    manifest: &McpServerManifest,
) -> Result<std::path::PathBuf> {
    if let Some(ref build) = manifest.build {
        if let Some(ref output) = build.output {
            return Ok(clone_dir.join(output));
        }
    }
    // Default: target/release/<name>
    Ok(clone_dir.join("target").join("release").join(&manifest.name))
}

/// Compute SHA-256 hash of a file.
fn compute_sha256(path: &Path) -> Result<String> {
    let data = std::fs::read(path)?;
    let hash = Sha256::digest(&data);
    Ok(format!("{hash:x}"))
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
