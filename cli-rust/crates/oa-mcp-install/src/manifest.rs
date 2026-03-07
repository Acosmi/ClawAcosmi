//! `mcp.json` manifest discovery, parsing, and auto-inference.
//!
//! An MCP server repository may contain an `mcp.json` file declaring build/run
//! configuration. If absent, the system infers defaults from the detected
//! project type.

use std::path::Path;

use crate::error::{McpInstallError, Result};
use crate::types::{ManifestBuild, McpServerManifest, ProjectType, TransportMode};

/// Well-known manifest filename.
const MANIFEST_FILENAME: &str = "mcp.json";

/// Try to load `mcp.json` from the project directory.
///
/// Returns `Ok(None)` if the file does not exist (non-error: manifest is optional).
pub fn load_manifest(project_dir: &Path) -> Result<Option<McpServerManifest>> {
    let manifest_path = project_dir.join(MANIFEST_FILENAME);
    if !manifest_path.exists() {
        tracing::debug!(dir = %project_dir.display(), "no mcp.json found, will auto-infer");
        return Ok(None);
    }

    let content = std::fs::read_to_string(&manifest_path)?;
    let manifest: McpServerManifest = serde_json::from_str(&content).map_err(|e| {
        McpInstallError::ManifestParseError(format!("{}: {e}", manifest_path.display()))
    })?;

    tracing::info!(name = %manifest.name, "loaded mcp.json manifest");
    Ok(Some(manifest))
}

/// Infer a manifest from the detected project type when no `mcp.json` exists.
///
/// Uses sensible defaults:
/// - **Rust**: `cargo build --release`, output at `target/release/<repo_name>`
/// - **Go**: `go build -o <repo_name>`, output at `./<repo_name>`
pub fn infer_manifest(project_type: ProjectType, repo_name: &str) -> Result<McpServerManifest> {
    let (build, command) = match project_type {
        ProjectType::Rust => {
            let build = ManifestBuild {
                command: "cargo".into(),
                args: vec!["build".into(), "--release".into()],
                output: Some(format!("target/release/{repo_name}")),
            };
            (Some(build), None)
        }
        ProjectType::Go => {
            let build = ManifestBuild {
                command: "go".into(),
                args: vec!["build".into(), "-o".into(), repo_name.to_string(), ".".into()],
                output: Some(repo_name.to_string()),
            };
            (Some(build), None)
        }
        ProjectType::JavaScript => {
            // Phase 2: JS/TS support via Docker or QuickJS
            return Err(McpInstallError::UnsupportedProjectType(
                "JavaScript/TypeScript (Phase 2)".into(),
            ));
        }
        ProjectType::Python => {
            // Phase 3: Python support via Docker
            return Err(McpInstallError::UnsupportedProjectType(
                "Python (Phase 3)".into(),
            ));
        }
        ProjectType::Unknown => {
            return Err(McpInstallError::DetectionFailed(
                "cannot infer build config for unknown project type".into(),
            ));
        }
    };

    Ok(McpServerManifest {
        name: repo_name.to_string(),
        version: String::new(),
        transport: TransportMode::Stdio,
        build,
        command,
        args: Vec::new(),
        env: std::collections::HashMap::new(),
    })
}

/// Build command allowlist — only these base commands are permitted.
const ALLOWED_BUILD_COMMANDS: &[&str] = &["cargo", "go", "npm", "npx", "pip", "uv"];

/// Validate that the manifest build command is in the allowlist.
pub fn validate_build_command(manifest: &McpServerManifest) -> Result<()> {
    if let Some(ref build) = manifest.build {
        let base_cmd = build
            .command
            .split('/')
            .last()
            .unwrap_or(&build.command);

        if !ALLOWED_BUILD_COMMANDS.contains(&base_cmd) {
            return Err(McpInstallError::BuildCommandNotAllowed(format!(
                "{:?} — allowed: {:?}",
                build.command, ALLOWED_BUILD_COMMANDS
            )));
        }
    }
    Ok(())
}

// ---------- Tests ----------

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;
    use tempfile::TempDir;

    #[test]
    fn test_load_manifest_missing() {
        let dir = TempDir::new().unwrap();
        let result = load_manifest(dir.path()).unwrap();
        assert!(result.is_none());
    }

    #[test]
    fn test_load_manifest_valid() {
        let dir = TempDir::new().unwrap();
        let manifest = r#"{
            "name": "test-mcp-server",
            "version": "1.0.0",
            "transport": "stdio",
            "build": {
                "command": "cargo",
                "args": ["build", "--release"],
                "output": "target/release/test-mcp-server"
            },
            "command": null,
            "args": []
        }"#;
        fs::write(dir.path().join("mcp.json"), manifest).unwrap();
        let result = load_manifest(dir.path()).unwrap().unwrap();
        assert_eq!(result.name, "test-mcp-server");
        assert_eq!(result.transport, TransportMode::Stdio);
        assert!(result.build.is_some());
    }

    #[test]
    fn test_load_manifest_invalid_json() {
        let dir = TempDir::new().unwrap();
        fs::write(dir.path().join("mcp.json"), "not json").unwrap();
        assert!(load_manifest(dir.path()).is_err());
    }

    #[test]
    fn test_infer_rust() {
        let m = infer_manifest(ProjectType::Rust, "my-server").unwrap();
        assert_eq!(m.name, "my-server");
        let build = m.build.unwrap();
        assert_eq!(build.command, "cargo");
        assert_eq!(build.output.as_deref(), Some("target/release/my-server"));
    }

    #[test]
    fn test_infer_go() {
        let m = infer_manifest(ProjectType::Go, "my-server").unwrap();
        let build = m.build.unwrap();
        assert_eq!(build.command, "go");
    }

    #[test]
    fn test_infer_javascript_unsupported() {
        assert!(infer_manifest(ProjectType::JavaScript, "x").is_err());
    }

    #[test]
    fn test_infer_python_unsupported() {
        assert!(infer_manifest(ProjectType::Python, "x").is_err());
    }

    #[test]
    fn test_validate_allowed_commands() {
        for &cmd in &["cargo", "go", "npm", "pip"] {
            let m = McpServerManifest {
                name: "test".into(),
                version: String::new(),
                transport: TransportMode::Stdio,
                build: Some(ManifestBuild {
                    command: cmd.into(),
                    args: vec![],
                    output: None,
                }),
                command: None,
                args: vec![],
                env: std::collections::HashMap::new(),
            };
            assert!(validate_build_command(&m).is_ok(), "expected {cmd} to be allowed");
        }
    }

    #[test]
    fn test_validate_disallowed_command() {
        let m = McpServerManifest {
            name: "test".into(),
            version: String::new(),
            transport: TransportMode::Stdio,
            build: Some(ManifestBuild {
                command: "rm".into(),
                args: vec!["-rf".into()],
                output: None,
            }),
            command: None,
            args: vec![],
            env: std::collections::HashMap::new(),
        };
        assert!(validate_build_command(&m).is_err());
    }

    #[test]
    fn test_manifest_with_env() {
        let dir = TempDir::new().unwrap();
        let manifest = r#"{
            "name": "api-server",
            "transport": "stdio",
            "env": {
                "API_KEY": {
                    "description": "API key for the service",
                    "required": true
                },
                "LOG_LEVEL": {
                    "description": "Log verbosity",
                    "default": "info"
                }
            }
        }"#;
        fs::write(dir.path().join("mcp.json"), manifest).unwrap();
        let result = load_manifest(dir.path()).unwrap().unwrap();
        assert_eq!(result.env.len(), 2);
        assert!(result.env["API_KEY"].required);
        assert_eq!(result.env["LOG_LEVEL"].default.as_deref(), Some("info"));
    }
}
