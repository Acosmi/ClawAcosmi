//! Bash tool — command execution, optionally sandboxed via oa-sandbox.

use std::path::Path;
use std::process::Command;

use anyhow::{Context, Result};
use serde::Deserialize;

use crate::server::{ContentItem, ToolCallResult, ToolDefinition};

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct BashParams {
    /// The bash command to execute.
    pub command: String,
    /// Execution timeout in seconds (default: 120).
    #[serde(default = "default_timeout")]
    pub timeout: u64,
}

const fn default_timeout() -> u64 { 120 }

pub fn tool_definition() -> ToolDefinition {
    ToolDefinition {
        name: "bash".to_owned(),
        description: "Execute a bash command. When sandboxed, the command runs inside an \
            OS-native sandbox (Seatbelt/Landlock/AppContainer) or Docker container."
            .to_owned(),
        input_schema: serde_json::json!({
            "type": "object",
            "properties": {
                "command": {
                    "type": "string",
                    "description": "The bash command to execute"
                },
                "timeout": {
                    "type": "integer",
                    "description": "Timeout in seconds (default: 120)",
                    "default": 120
                }
            },
            "required": ["command"]
        }),
    }
}

/// Execute the bash tool.
///
/// When `sandboxed` is true, delegates to oa-sandbox for isolated execution.
/// When false, executes directly via `sh -c`.
pub fn execute(
    workspace: &Path,
    sandboxed: bool,
    arguments: serde_json::Value,
) -> Result<ToolCallResult> {
    let params: BashParams =
        serde_json::from_value(arguments).context("invalid bash parameters")?;

    if sandboxed {
        #[cfg(feature = "sandbox")]
        {
            return execute_sandboxed(workspace, &params);
        }

        #[cfg(not(feature = "sandbox"))]
        {
            tracing::warn!("sandbox feature not enabled, falling back to direct execution");
            // fall through to execute_direct
        }
    }

    execute_direct(workspace, &params)
}

/// Direct execution without sandbox (for development/trusted contexts).
fn execute_direct(workspace: &Path, params: &BashParams) -> Result<ToolCallResult> {
    let mut child = Command::new("sh")
        .arg("-c")
        .arg(&params.command)
        .current_dir(workspace)
        .stdout(std::process::Stdio::piped())
        .stderr(std::process::Stdio::piped())
        .spawn()
        .with_context(|| format!("failed to spawn: {}", params.command))?;

    let timeout = std::time::Duration::from_secs(params.timeout);
    let start = std::time::Instant::now();

    // Poll with short sleeps until process exits or timeout.
    let status = loop {
        match child.try_wait().context("failed to check process status")? {
            Some(status) => break status,
            None if start.elapsed() >= timeout => {
                // Kill the process on timeout.
                let _ = child.kill();
                let _ = child.wait();
                return Ok(ToolCallResult {
                    content: vec![ContentItem {
                        content_type: "text".to_owned(),
                        text: format!(
                            "Command timed out after {}s: {}",
                            params.timeout, params.command
                        ),
                    }],
                    is_error: true,
                });
            }
            None => std::thread::sleep(std::time::Duration::from_millis(50)),
        }
    };

    let mut stdout_buf = Vec::new();
    let mut stderr_buf = Vec::new();
    if let Some(mut out) = child.stdout.take() {
        std::io::Read::read_to_end(&mut out, &mut stdout_buf).ok();
    }
    if let Some(mut err) = child.stderr.take() {
        std::io::Read::read_to_end(&mut err, &mut stderr_buf).ok();
    }

    let stdout = String::from_utf8_lossy(&stdout_buf);
    let stderr = String::from_utf8_lossy(&stderr_buf);
    let exit_code = status.code().unwrap_or(-1);

    let mut text = String::new();
    if !stdout.is_empty() {
        text.push_str(&stdout);
    }
    if !stderr.is_empty() {
        if !text.is_empty() {
            text.push('\n');
        }
        text.push_str("STDERR:\n");
        text.push_str(&stderr);
    }
    if text.is_empty() {
        text = format!("(exit code: {exit_code})");
    } else {
        text.push_str(&format!("\n(exit code: {exit_code})"));
    }

    Ok(ToolCallResult {
        content: vec![ContentItem {
            content_type: "text".to_owned(),
            text,
        }],
        is_error: exit_code != 0,
    })
}

/// Sandboxed execution via oa-sandbox.
///
/// Uses `oa_sandbox::platform::select_runner()` to pick the best available
/// backend (native Seatbelt/Landlock/AppContainer or Docker fallback).
#[cfg(feature = "sandbox")]
fn execute_sandboxed(workspace: &Path, params: &BashParams) -> Result<ToolCallResult> {
    use std::collections::HashMap;

    use oa_sandbox::config::{
        BackendPreference, NetworkPolicy, OutputFormat, ResourceLimits, SandboxConfig,
        SecurityLevel,
    };

    let config = SandboxConfig {
        command: "sh".to_owned(),
        args: vec!["-c".to_owned(), params.command.clone()],
        workspace: workspace.to_path_buf(),
        security_level: SecurityLevel::L1Sandbox,
        network_policy: Some(NetworkPolicy::None),
        resource_limits: ResourceLimits {
            timeout_secs: Some(params.timeout),
            ..ResourceLimits::default()
        },
        format: OutputFormat::Json,
        backend: BackendPreference::Auto,
        mounts: Vec::new(),
        env_vars: HashMap::new(),
    };

    let runner = oa_sandbox::select_runner(&config)
        .map_err(|e| anyhow::anyhow!("sandbox setup failed: {e}"))?;
    let output = runner
        .run(&config)
        .map_err(|e| anyhow::anyhow!("sandbox execution failed: {e}"))?;

    Ok(ToolCallResult {
        content: vec![ContentItem {
            content_type: "text".to_owned(),
            text: format!(
                "{}\n{}(exit code: {})",
                output.stdout,
                if output.stderr.is_empty() {
                    String::new()
                } else {
                    format!("STDERR:\n{}\n", output.stderr)
                },
                output.exit_code
            ),
        }],
        is_error: output.exit_code != 0,
    })
}
