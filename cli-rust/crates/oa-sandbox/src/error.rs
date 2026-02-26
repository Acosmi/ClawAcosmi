//! Sandbox error types.
//!
//! Uses `thiserror` for typed, matchable errors per Skill 3 conventions.
//! Every variant includes enough context (operation, path, PID) for diagnostics.

use std::path::PathBuf;

use thiserror::Error;

/// Errors that can occur during sandbox configuration, setup, or execution.
#[derive(Debug, Error)]
pub enum SandboxError {
    // ── Configuration ──────────────────────────────────────────────
    /// Invalid sandbox configuration.
    #[error("invalid sandbox config: {message}")]
    InvalidConfig { message: String },

    /// The requested command was not found in the sandbox.
    #[error("command not found in sandbox: {command}")]
    CommandNotFound { command: String },

    /// Filesystem path validation failed.
    #[error("path error: {path:?} — {reason}")]
    PathError { path: PathBuf, reason: String },

    // ── Platform support ───────────────────────────────────────────
    /// Current platform does not support the requested sandbox backend.
    #[error("platform not supported: {platform} — {reason}")]
    PlatformNotSupported { platform: String, reason: String },

    /// No sandbox backend available (native and Docker both unavailable).
    #[error("no sandbox backend available — native: {native_reason}; docker: {docker_reason}")]
    NoBackendAvailable {
        native_reason: String,
        docker_reason: String,
    },

    // ── Linux-specific ─────────────────────────────────────────────
    /// Linux namespace operation failed.
    #[error("namespace {operation} failed")]
    Namespace {
        operation: String,
        #[source]
        source: std::io::Error,
    },

    /// Landlock LSM operation failed.
    #[error("landlock {operation} failed")]
    Landlock {
        operation: String,
        #[source]
        source: std::io::Error,
    },

    /// Seccomp-BPF filter operation failed.
    #[error("seccomp {operation} failed")]
    Seccomp {
        operation: String,
        #[source]
        source: std::io::Error,
    },

    // ── macOS-specific ─────────────────────────────────────────────
    /// macOS Seatbelt sandbox operation failed.
    #[error("seatbelt error: {message}")]
    Seatbelt { message: String },

    // ── Windows-specific ───────────────────────────────────────────
    /// Windows API call failed.
    #[error("win32 {operation} failed (error code: {error_code})")]
    Win32 { operation: String, error_code: u32 },

    // ── Execution ──────────────────────────────────────────────────
    /// Sandboxed process exceeded resource limits.
    #[error("resource limit exceeded: {resource} (limit: {limit}, actual: {actual})")]
    ResourceExceeded {
        resource: String,
        limit: String,
        actual: String,
    },

    /// Sandboxed process timed out.
    #[error("sandbox execution timed out after {timeout_secs}s")]
    Timeout { timeout_secs: u64 },

    // ── Degradation ────────────────────────────────────────────────
    /// Native backend unavailable, degradation triggered.
    /// This is informational — the caller should retry with the `to` backend.
    #[error("degrading from {from} to {to}: {reason}")]
    Degraded {
        from: String,
        to: String,
        reason: String,
    },

    // ── Generic I/O ────────────────────────────────────────────────
    /// Generic I/O error with context.
    #[error("{context}")]
    Io {
        context: String,
        #[source]
        source: std::io::Error,
    },
}
