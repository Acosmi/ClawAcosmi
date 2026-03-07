//! Core types for MCP server installation and management.

use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::path::PathBuf;

// ---------- Project Type ----------

/// Detected project type from repository contents.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ProjectType {
    Rust,
    Go,
    JavaScript,
    Python,
    Unknown,
}

impl std::fmt::Display for ProjectType {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Rust => write!(f, "rust"),
            Self::Go => write!(f, "go"),
            Self::JavaScript => write!(f, "javascript"),
            Self::Python => write!(f, "python"),
            Self::Unknown => write!(f, "unknown"),
        }
    }
}

// ---------- URL Parse Result ----------

/// Result of parsing an MCP server URL.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ParsedMcpUrl {
    /// Original input URL.
    pub original: String,
    /// Normalized git clone URL (None for release asset URLs).
    pub git_url: Option<String>,
    /// Repository owner (e.g. "modelcontextprotocol").
    pub owner: Option<String>,
    /// Repository name (e.g. "servers").
    pub repo: Option<String>,
    /// Hosting platform.
    pub platform: HostingPlatform,
    /// URL kind classification.
    pub kind: UrlKind,
    /// Release asset direct download URL (for release URLs).
    pub release_asset_url: Option<String>,
    /// Tag name from release URL.
    pub release_tag: Option<String>,
}

/// Hosting platform.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum HostingPlatform {
    GitHub,
    GitLab,
    Bitbucket,
    Other,
}

/// URL kind classification.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum UrlKind {
    /// Standard git repository URL (clone path).
    GitRepo,
    /// GitHub/GitLab Release asset URL (download path).
    ReleaseAsset,
    /// SSH URL (git@host:owner/repo).
    Ssh,
}

// ---------- MCP Server Manifest ----------

/// `mcp.json` manifest schema — optional metadata in the repository root.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct McpServerManifest {
    /// Server display name.
    pub name: String,
    /// Version string.
    #[serde(default)]
    pub version: String,
    /// Transport mode.
    #[serde(default = "default_transport")]
    pub transport: TransportMode,
    /// Build configuration.
    #[serde(default)]
    pub build: Option<ManifestBuild>,
    /// Command to run the server.
    #[serde(default)]
    pub command: Option<String>,
    /// Arguments for the server command.
    #[serde(default)]
    pub args: Vec<String>,
    /// Environment variables required by the server.
    #[serde(default)]
    pub env: HashMap<String, EnvVarSpec>,
}

/// Transport mode for the MCP server.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum TransportMode {
    Stdio,
    Sse,
    Http,
}

fn default_transport() -> TransportMode {
    TransportMode::Stdio
}

/// Build configuration in mcp.json.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ManifestBuild {
    /// Build command (must be in allowlist: cargo, go, npm, pip).
    pub command: String,
    /// Build arguments.
    #[serde(default)]
    pub args: Vec<String>,
    /// Output binary path (relative to project root).
    #[serde(default)]
    pub output: Option<String>,
}

/// Environment variable specification.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EnvVarSpec {
    /// Human-readable description.
    #[serde(default)]
    pub description: String,
    /// Whether this variable is required.
    #[serde(default)]
    pub required: bool,
    /// Default value if not provided.
    #[serde(default)]
    pub default: Option<String>,
}

// ---------- Installed Server Entry ----------

/// An installed MCP server entry in `registry.json`.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InstalledMcpServer {
    /// Unique server name (derived from repo name or manifest).
    pub name: String,
    /// Original install source URL.
    pub source_url: String,
    /// URL kind at install time.
    pub source_kind: UrlKind,
    /// Detected or declared project type.
    pub project_type: ProjectType,
    /// Transport mode.
    pub transport: TransportMode,
    /// Absolute path to the built binary.
    pub binary_path: PathBuf,
    /// Command to run (if different from binary_path).
    #[serde(default)]
    pub command: Option<String>,
    /// Arguments for the server command.
    #[serde(default)]
    pub args: Vec<String>,
    /// Clone directory (for git-cloned servers).
    #[serde(default)]
    pub clone_dir: Option<PathBuf>,
    /// Environment variables (name → value).
    #[serde(default)]
    pub env: HashMap<String, String>,
    /// Version locking: pinned git ref (tag/branch/commit).
    #[serde(default)]
    pub pinned_ref: Option<String>,
    /// Version locking: actual source commit SHA at build time.
    #[serde(default)]
    pub source_commit: Option<String>,
    /// Version locking: SHA-256 of the built/downloaded binary.
    #[serde(default)]
    pub binary_sha256: Option<String>,
    /// ISO 8601 timestamp of installation.
    pub installed_at: String,
    /// ISO 8601 timestamp of last update.
    #[serde(default)]
    pub updated_at: Option<String>,
}

// ---------- Registry ----------

/// Top-level registry structure stored in `~/.openacosmi/mcp-servers/registry.json`.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct McpServerRegistry {
    /// Schema version for forward compatibility.
    #[serde(default = "default_schema_version")]
    pub schema_version: u32,
    /// Installed servers keyed by name.
    #[serde(default)]
    pub servers: HashMap<String, InstalledMcpServer>,
}

fn default_schema_version() -> u32 {
    1
}

impl Default for McpServerRegistry {
    fn default() -> Self {
        Self {
            schema_version: default_schema_version(),
            servers: HashMap::new(),
        }
    }
}

// ---------- Build Result ----------

/// Result of a successful build.
#[derive(Debug, Clone)]
pub struct BuildResult {
    /// Absolute path to the built binary.
    pub binary_path: PathBuf,
    /// SHA-256 hash of the binary.
    pub binary_sha256: String,
    /// Source commit SHA (if available).
    pub source_commit: Option<String>,
}
