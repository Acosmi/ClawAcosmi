//! Error types for the MCP install subsystem.

/// Result alias using [`McpInstallError`].
pub type Result<T> = std::result::Result<T, McpInstallError>;

/// Errors that can occur during MCP server installation.
#[derive(Debug, thiserror::Error)]
pub enum McpInstallError {
    #[error("invalid URL: {0}")]
    InvalidUrl(String),

    #[error("unsupported URL scheme: {0}")]
    UnsupportedScheme(String),

    #[error("git clone failed: {0}")]
    GitCloneFailed(String),

    #[error("project type could not be detected in {0}")]
    DetectionFailed(String),

    #[error("unsupported project type: {0}")]
    UnsupportedProjectType(String),

    #[error("manifest parse error: {0}")]
    ManifestParseError(String),

    #[error("build failed: {0}")]
    BuildFailed(String),

    #[error("build command not allowed: {0}")]
    BuildCommandNotAllowed(String),

    #[error("registry error: {0}")]
    RegistryError(String),

    #[error("server not found: {0}")]
    ServerNotFound(String),

    #[error("server already installed: {0}")]
    ServerAlreadyInstalled(String),

    #[error("download failed: {0}")]
    DownloadFailed(String),

    #[error("SHA-256 checksum mismatch: expected {expected}, got {actual}")]
    ChecksumMismatch { expected: String, actual: String },

    #[error("binary path escape: {path} is outside managed directory {managed_dir}")]
    BinaryPathEscape { path: String, managed_dir: String },

    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),

    #[error("{0}")]
    Other(String),
}
