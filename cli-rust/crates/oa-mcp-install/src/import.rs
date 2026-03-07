//! Import MCP server configurations from Claude Desktop, Cursor, and VS Code.
//!
//! All three tools use a similar JSON format with `command` + `args` + `env`.
//! This module parses each format and normalizes into `ImportedMcpServer` entries.

use serde::Deserialize;
use std::collections::HashMap;
use std::path::{Path, PathBuf};

use crate::error::{McpInstallError, Result};

// ---------- Public API ----------

/// A single MCP server configuration parsed from an external tool.
#[derive(Debug, Clone)]
pub struct ImportedMcpServer {
    /// Server name (key from the config object).
    pub name: String,
    /// Command to run the server.
    pub command: String,
    /// Arguments for the command.
    pub args: Vec<String>,
    /// Environment variables.
    pub env: HashMap<String, String>,
    /// Source format where this was imported from.
    pub source_format: ConfigFormat,
    /// Original config file path.
    pub source_path: PathBuf,
}

/// Supported config formats.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ConfigFormat {
    /// Claude Desktop: `~/.claude/claude_desktop_config.json`
    ClaudeDesktop,
    /// Cursor: `.cursor/mcp.json`
    Cursor,
    /// VS Code: `.vscode/mcp.json` or settings.json `mcp.servers`
    VsCode,
    /// Auto-detected from file path or content.
    Unknown,
}

impl std::fmt::Display for ConfigFormat {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::ClaudeDesktop => write!(f, "Claude Desktop"),
            Self::Cursor => write!(f, "Cursor"),
            Self::VsCode => write!(f, "VS Code"),
            Self::Unknown => write!(f, "Unknown"),
        }
    }
}

/// Import MCP server configurations from a config file.
///
/// Auto-detects the format based on file path or content structure.
pub fn import_from_file(path: &Path) -> Result<Vec<ImportedMcpServer>> {
    let content = std::fs::read_to_string(path).map_err(|e| {
        McpInstallError::Other(format!("cannot read config file {}: {e}", path.display()))
    })?;

    let format = detect_format(path, &content);
    parse_config(&content, format, path)
}

/// Import from a specific format (skip auto-detection).
pub fn import_from_file_with_format(
    path: &Path,
    format: ConfigFormat,
) -> Result<Vec<ImportedMcpServer>> {
    let content = std::fs::read_to_string(path).map_err(|e| {
        McpInstallError::Other(format!("cannot read config file {}: {e}", path.display()))
    })?;
    parse_config(&content, format, path)
}

/// Discover well-known config file paths on this system.
pub fn discover_config_files() -> Vec<(PathBuf, ConfigFormat)> {
    let mut found = Vec::new();

    if let Some(home) = dirs::home_dir() {
        // Claude Desktop
        let claude = home.join(".claude").join("claude_desktop_config.json");
        if claude.exists() {
            found.push((claude, ConfigFormat::ClaudeDesktop));
        }

        // Cursor — user-level
        let cursor = home.join(".cursor").join("mcp.json");
        if cursor.exists() {
            found.push((cursor, ConfigFormat::Cursor));
        }

        // VS Code — user-level
        #[cfg(target_os = "macos")]
        {
            let vscode = home
                .join("Library/Application Support/Code/User/settings.json");
            if vscode.exists() {
                found.push((vscode, ConfigFormat::VsCode));
            }
        }
        #[cfg(target_os = "linux")]
        {
            let vscode = home.join(".config/Code/User/settings.json");
            if vscode.exists() {
                found.push((vscode, ConfigFormat::VsCode));
            }
        }
        #[cfg(target_os = "windows")]
        {
            if let Some(appdata) = dirs::config_dir() {
                let vscode = appdata.join("Code/User/settings.json");
                if vscode.exists() {
                    found.push((vscode, ConfigFormat::VsCode));
                }
            }
        }
    }

    found
}

// ---------- Parsing ----------

/// Detect format from file path heuristics.
fn detect_format(path: &Path, content: &str) -> ConfigFormat {
    let filename = path
        .file_name()
        .and_then(|n| n.to_str())
        .unwrap_or("");

    // Check parent directory for hints
    let parent = path
        .parent()
        .and_then(|p| p.file_name())
        .and_then(|n| n.to_str())
        .unwrap_or("");

    if filename == "claude_desktop_config.json" || parent == ".claude" {
        return ConfigFormat::ClaudeDesktop;
    }
    if parent == ".cursor" {
        return ConfigFormat::Cursor;
    }
    if parent == ".vscode" || filename == "settings.json" {
        return ConfigFormat::VsCode;
    }

    // Try content-based detection
    if content.contains("\"mcpServers\"") {
        // Both Claude Desktop and Cursor use "mcpServers"
        return ConfigFormat::ClaudeDesktop; // same parse path
    }
    if content.contains("\"mcp.servers\"") || content.contains("\"servers\"") {
        return ConfigFormat::VsCode;
    }

    ConfigFormat::Unknown
}

// ---------- JSON Schema Types ----------

/// Claude Desktop / Cursor schema: `{ "mcpServers": { "<name>": { ... } } }`
#[derive(Debug, Deserialize)]
struct ClaudeDesktopConfig {
    #[serde(default, rename = "mcpServers")]
    mcp_servers: HashMap<String, McpServerEntry>,
}

/// VS Code schema variant 1: `{ "servers": { "<name>": { ... } } }`
#[derive(Debug, Deserialize)]
struct VsCodeMcpConfig {
    #[serde(default)]
    servers: HashMap<String, McpServerEntry>,
}

/// Individual server entry (shared across all formats).
#[derive(Debug, Deserialize)]
struct McpServerEntry {
    #[serde(default)]
    command: String,
    #[serde(default)]
    args: Vec<String>,
    #[serde(default)]
    env: HashMap<String, String>,
    /// VS Code uses "type" field (stdio/sse).
    #[serde(default, rename = "type")]
    _transport_type: Option<String>,
}

fn parse_config(
    content: &str,
    format: ConfigFormat,
    source_path: &Path,
) -> Result<Vec<ImportedMcpServer>> {
    match format {
        ConfigFormat::ClaudeDesktop | ConfigFormat::Cursor => {
            parse_claude_cursor(content, format, source_path)
        }
        ConfigFormat::VsCode => parse_vscode(content, source_path),
        ConfigFormat::Unknown => {
            // Try Claude/Cursor first, then VS Code
            if let Ok(servers) = parse_claude_cursor(content, ConfigFormat::Unknown, source_path) {
                if !servers.is_empty() {
                    return Ok(servers);
                }
            }
            if let Ok(servers) = parse_vscode(content, source_path) {
                if !servers.is_empty() {
                    return Ok(servers);
                }
            }
            Ok(Vec::new())
        }
    }
}

fn parse_claude_cursor(
    content: &str,
    format: ConfigFormat,
    source_path: &Path,
) -> Result<Vec<ImportedMcpServer>> {
    let config: ClaudeDesktopConfig = serde_json::from_str(content).map_err(|e| {
        McpInstallError::Other(format!("JSON parse error: {e}"))
    })?;

    Ok(config
        .mcp_servers
        .into_iter()
        .filter(|(_, entry)| !entry.command.is_empty())
        .map(|(name, entry)| ImportedMcpServer {
            name,
            command: entry.command,
            args: entry.args,
            env: entry.env,
            source_format: format,
            source_path: source_path.to_path_buf(),
        })
        .collect())
}

fn parse_vscode(content: &str, source_path: &Path) -> Result<Vec<ImportedMcpServer>> {
    // Try direct mcp.json format first
    if let Ok(config) = serde_json::from_str::<VsCodeMcpConfig>(content) {
        if !config.servers.is_empty() {
            return Ok(config
                .servers
                .into_iter()
                .filter(|(_, entry)| !entry.command.is_empty())
                .map(|(name, entry)| ImportedMcpServer {
                    name,
                    command: entry.command,
                    args: entry.args,
                    env: entry.env,
                    source_format: ConfigFormat::VsCode,
                    source_path: source_path.to_path_buf(),
                })
                .collect());
        }
    }

    // Try settings.json with nested "mcp.servers" key
    if let Ok(settings) = serde_json::from_str::<serde_json::Value>(content) {
        if let Some(mcp_servers) = settings.get("mcp.servers") {
            if let Ok(servers) =
                serde_json::from_value::<HashMap<String, McpServerEntry>>(mcp_servers.clone())
            {
                return Ok(servers
                    .into_iter()
                    .filter(|(_, entry)| !entry.command.is_empty())
                    .map(|(name, entry)| ImportedMcpServer {
                        name,
                        command: entry.command,
                        args: entry.args,
                        env: entry.env,
                        source_format: ConfigFormat::VsCode,
                        source_path: source_path.to_path_buf(),
                    })
                    .collect());
            }
        }
    }

    Ok(Vec::new())
}

// ---------- Tests ----------

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Write;

    #[test]
    fn test_parse_claude_desktop_config() {
        let json = r#"{
            "mcpServers": {
                "filesystem": {
                    "command": "npx",
                    "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"],
                    "env": { "API_KEY": "test123" }
                },
                "github": {
                    "command": "npx",
                    "args": ["-y", "@modelcontextprotocol/server-github"],
                    "env": {}
                }
            }
        }"#;

        let servers =
            parse_claude_cursor(json, ConfigFormat::ClaudeDesktop, Path::new("/test")).unwrap();
        assert_eq!(servers.len(), 2);

        let fs_server = servers.iter().find(|s| s.name == "filesystem").unwrap();
        assert_eq!(fs_server.command, "npx");
        assert_eq!(fs_server.args.len(), 3);
        assert_eq!(fs_server.env.get("API_KEY"), Some(&"test123".to_string()));
    }

    #[test]
    fn test_parse_cursor_config() {
        let json = r#"{
            "mcpServers": {
                "my-server": {
                    "command": "node",
                    "args": ["server.js"],
                    "env": {}
                }
            }
        }"#;

        let servers =
            parse_claude_cursor(json, ConfigFormat::Cursor, Path::new("/test")).unwrap();
        assert_eq!(servers.len(), 1);
        assert_eq!(servers[0].name, "my-server");
        assert_eq!(servers[0].command, "node");
    }

    #[test]
    fn test_parse_vscode_mcp_json() {
        let json = r#"{
            "servers": {
                "sqlite": {
                    "type": "stdio",
                    "command": "uvx",
                    "args": ["mcp-server-sqlite", "--db-path", "/tmp/test.db"],
                    "env": {}
                }
            }
        }"#;

        let servers = parse_vscode(json, Path::new("/test")).unwrap();
        assert_eq!(servers.len(), 1);
        assert_eq!(servers[0].name, "sqlite");
        assert_eq!(servers[0].command, "uvx");
    }

    #[test]
    fn test_parse_vscode_settings_json() {
        let json = r#"{
            "editor.fontSize": 14,
            "mcp.servers": {
                "fetch": {
                    "command": "uvx",
                    "args": ["mcp-server-fetch"],
                    "env": {}
                }
            }
        }"#;

        let servers = parse_vscode(json, Path::new("/test")).unwrap();
        assert_eq!(servers.len(), 1);
        assert_eq!(servers[0].name, "fetch");
    }

    #[test]
    fn test_detect_format_from_path() {
        assert_eq!(
            detect_format(
                Path::new("/home/user/.claude/claude_desktop_config.json"),
                ""
            ),
            ConfigFormat::ClaudeDesktop
        );
        assert_eq!(
            detect_format(Path::new("/home/user/.cursor/mcp.json"), ""),
            ConfigFormat::Cursor
        );
        assert_eq!(
            detect_format(Path::new("/project/.vscode/mcp.json"), ""),
            ConfigFormat::VsCode
        );
    }

    #[test]
    fn test_detect_format_from_content() {
        assert_eq!(
            detect_format(
                Path::new("/some/random/file.json"),
                r#"{ "mcpServers": {} }"#
            ),
            ConfigFormat::ClaudeDesktop
        );
    }

    #[test]
    fn test_empty_config() {
        let json = r#"{ "mcpServers": {} }"#;
        let servers =
            parse_claude_cursor(json, ConfigFormat::ClaudeDesktop, Path::new("/test")).unwrap();
        assert!(servers.is_empty());
    }

    #[test]
    fn test_server_without_command_is_skipped() {
        let json = r#"{
            "mcpServers": {
                "broken": {
                    "command": "",
                    "args": [],
                    "env": {}
                },
                "good": {
                    "command": "node",
                    "args": ["server.js"],
                    "env": {}
                }
            }
        }"#;
        let servers =
            parse_claude_cursor(json, ConfigFormat::ClaudeDesktop, Path::new("/test")).unwrap();
        assert_eq!(servers.len(), 1);
        assert_eq!(servers[0].name, "good");
    }

    #[test]
    fn test_import_from_file_roundtrip() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("claude_desktop_config.json");
        let mut f = std::fs::File::create(&path).unwrap();
        writeln!(
            f,
            r#"{{
            "mcpServers": {{
                "test": {{
                    "command": "echo",
                    "args": ["hello"],
                    "env": {{}}
                }}
            }}
        }}"#
        )
        .unwrap();

        let servers = import_from_file(&path).unwrap();
        assert_eq!(servers.len(), 1);
        assert_eq!(servers[0].name, "test");
        assert_eq!(servers[0].source_format, ConfigFormat::ClaudeDesktop);
    }
}
