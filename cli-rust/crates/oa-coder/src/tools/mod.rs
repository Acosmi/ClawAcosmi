//! Tool router — registers and dispatches MCP tool calls.
//!
//! Each tool is a function that takes JSON arguments and returns a
//! [`ToolCallResult`]. The router maintains the tool registry and
//! provides `list_tools()` / `call_tool()` for the MCP server.

pub mod bash;
pub mod edit;
pub mod glob;
pub mod grep;
pub mod read;
pub mod write;

use std::path::{Path, PathBuf};

use anyhow::{bail, Result};
use tracing::debug;

use crate::server::{ContentItem, ToolCallResult, ToolDefinition};

/// Resolve and validate a file path, ensuring it stays within the workspace.
///
/// Rejects null bytes, `..` traversal escapes, and symlinks that resolve outside
/// the workspace boundary. Returns the canonicalized (or safely joined) path.
pub fn validate_path(workspace: &Path, file_path: &str) -> Result<PathBuf> {
    // Reject null bytes (can bypass C-based path APIs).
    if file_path.contains('\0') {
        bail!("path contains null byte");
    }

    let raw_path = if Path::new(file_path).is_absolute() {
        PathBuf::from(file_path)
    } else {
        workspace.join(file_path)
    };

    // Canonicalize workspace first (resolves symlinks like /var → /private/var on macOS).
    let canonical_workspace = workspace.canonicalize().unwrap_or_else(|_| workspace.to_path_buf());

    // For existing paths, canonicalize to resolve symlinks and `..`.
    // For non-existing paths (write/edit create), normalize manually.
    let canonical_path = if raw_path.exists() {
        raw_path.canonicalize()?
    } else {
        // Find the deepest existing ancestor and canonicalize that.
        let mut ancestor = raw_path.clone();
        let mut suffix_parts = Vec::new();
        loop {
            if ancestor.exists() {
                let canon_ancestor = ancestor.canonicalize()?;
                let mut result = canon_ancestor;
                for part in suffix_parts.iter().rev() {
                    result = result.join(part);
                }
                break result;
            }
            if let Some(name) = ancestor.file_name() {
                suffix_parts.push(name.to_os_string());
                if let Some(parent) = ancestor.parent() {
                    ancestor = parent.to_path_buf();
                } else {
                    break raw_path;
                }
            } else {
                break raw_path;
            }
        }
    };

    // Verify the resolved path is within the workspace.
    if !canonical_path.starts_with(&canonical_workspace) {
        bail!(
            "path escapes workspace boundary: {}",
            file_path
        );
    }

    Ok(canonical_path)
}

/// Resolve a directory path for grep/glob operations, ensuring it stays within workspace.
pub fn validate_dir_path(workspace: &Path, dir_path: &str) -> Result<PathBuf> {
    validate_path(workspace, dir_path)
}

/// Tool router that dispatches MCP tool calls to implementations.
pub struct ToolRouter {
    /// Working directory for file operations.
    workspace: PathBuf,
    /// Whether bash tool uses sandbox.
    sandboxed: bool,
}

impl ToolRouter {
    /// Create a new tool router.
    pub fn new(workspace: PathBuf, sandboxed: bool) -> Self {
        Self {
            workspace,
            sandboxed,
        }
    }

    /// List all available tools with their JSON Schema definitions.
    pub fn list_tools(&self) -> Vec<ToolDefinition> {
        vec![
            edit::tool_definition(),
            read::tool_definition(),
            write::tool_definition(),
            grep::tool_definition(),
            glob::tool_definition(),
            bash::tool_definition(),
        ]
    }

    /// Call a tool by name with the given JSON arguments.
    ///
    /// # Errors
    ///
    /// Returns an error if the tool name is unknown or the tool execution fails.
    pub fn call_tool(
        &self,
        name: &str,
        arguments: serde_json::Value,
    ) -> Result<ToolCallResult> {
        debug!(tool = name, "dispatching tool call");

        match name {
            "edit" => edit::execute(&self.workspace, arguments),
            "read" => read::execute(&self.workspace, arguments),
            "write" => write::execute(&self.workspace, arguments),
            "grep" => grep::execute(&self.workspace, arguments),
            "glob" => glob::execute(&self.workspace, arguments),
            "bash" => bash::execute(&self.workspace, self.sandboxed, arguments),
            _ => {
                let result = ToolCallResult {
                    content: vec![ContentItem {
                        content_type: "text".to_owned(),
                        text: format!("Unknown tool: {name}"),
                    }],
                    is_error: true,
                };
                Ok(result)
            }
        }
    }
}
