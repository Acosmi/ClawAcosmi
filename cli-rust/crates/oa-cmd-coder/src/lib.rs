//! CLI entry point for the oa-coder MCP server.
//!
//! Provides the `acosmi coder` subcommand that starts the MCP server
//! on stdin/stdout for use as a standalone coding agent or as a
//! sub-process managed by the Gateway's CoderBridge.
//!
//! # Usage
//!
//! ```text
//! # Standalone MCP server
//! acosmi coder start --workspace /path/to/project
//!
//! # With sandbox enabled
//! acosmi coder start --workspace /path/to/project --sandboxed
//! ```

use std::path::PathBuf;

use anyhow::{Context, Result};

/// Options for the `coder start` subcommand.
#[derive(Debug, Clone)]
pub struct CoderStartOptions {
    /// Workspace directory for file operations.
    pub workspace: PathBuf,
    /// Enable sandboxed execution for bash tool.
    pub sandboxed: bool,
}

/// Start the MCP server with the given options.
///
/// This function blocks until stdin is closed (client disconnects).
///
/// # Errors
///
/// Returns an error if the MCP server encounters a fatal I/O error.
pub fn coder_start_command(opts: &CoderStartOptions) -> Result<()> {
    let config = oa_coder::server::McpServerConfig {
        workspace: opts.workspace.clone(),
        sandboxed: opts.sandboxed,
    };

    oa_coder::run_mcp_server(config).context("MCP server failed")
}
