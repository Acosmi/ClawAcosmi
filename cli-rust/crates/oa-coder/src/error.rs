//! Error types for the oa-coder crate.

use std::path::PathBuf;

/// Coder-specific error types.
#[derive(Debug, thiserror::Error)]
pub enum CoderError {
    /// File not found at the specified path.
    #[error("file not found: {path}")]
    FileNotFound { path: PathBuf },

    /// File is binary and cannot be edited as text.
    #[error("binary file cannot be edited: {path}")]
    BinaryFile { path: PathBuf },

    /// The old_string was not found in the file (no fuzzy match succeeded).
    #[error("no match found for replacement in {path}")]
    NoMatchFound { path: PathBuf },

    /// Ambiguous match — multiple candidates with similar scores.
    #[error("ambiguous match: {count} candidates in {path}")]
    AmbiguousMatch { path: PathBuf, count: usize },

    /// File was modified externally since last read (timestamp conflict).
    #[error("file modified externally: {path} (expected mtime {expected}, got {actual})")]
    FileModified {
        path: PathBuf,
        expected: String,
        actual: String,
    },

    /// ripgrep binary not found on PATH.
    #[error("ripgrep (rg) not found on PATH — install via: brew install ripgrep / apt install ripgrep")]
    RipgrepNotFound,

    /// Subprocess execution failed.
    #[error("subprocess failed: {command}: {reason}")]
    SubprocessFailed { command: String, reason: String },

    /// MCP protocol error.
    #[error("MCP protocol error: {0}")]
    Protocol(String),

    /// JSON serialization/deserialization error.
    #[error("JSON error: {0}")]
    Json(#[from] serde_json::Error),

    /// I/O error with context.
    #[error("I/O error on {path}: {source}")]
    Io {
        path: PathBuf,
        #[source]
        source: std::io::Error,
    },

    /// Sandbox execution error.
    #[error("sandbox error: {0}")]
    Sandbox(String),
}

/// Convenience result type for oa-coder operations.
pub type CoderResult<T> = Result<T, CoderError>;
