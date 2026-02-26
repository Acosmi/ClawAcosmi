//! JSON IPC output contract.
//!
//! [`SandboxOutput`] is serialized to stdout for the Go orchestration layer
//! (`attempt_runner.go` → `tool_executor.go`) to parse via `exec.Command`.

use serde::{Deserialize, Serialize};

/// Result of a sandboxed command execution.
///
/// # Exit code conventions
///
/// | Code | Meaning |
/// |------|---------|
/// | 0    | Command succeeded |
/// | 1    | Command failed (non-zero exit from the sandboxed process) |
/// | 2    | Sandbox configuration error |
/// | 3    | Execution timeout |
/// | 4    | Resource limit exceeded |
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SandboxOutput {
    /// Captured stdout from the sandboxed command.
    pub stdout: String,

    /// Captured stderr from the sandboxed command.
    pub stderr: String,

    /// Exit code (see conventions above).
    pub exit_code: i32,

    /// Error message if sandbox setup or execution failed.
    /// `None` when the sandboxed command ran successfully (even if it returned non-zero).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,

    /// Wall-clock execution duration in milliseconds.
    pub duration_ms: u64,

    /// Name of the sandbox backend that was used (e.g., "linux-landlock+seccomp",
    /// "macos-seatbelt", "windows-restricted-token+job", "docker-fallback").
    pub sandbox_backend: String,
}

/// Well-known exit codes for sandbox-level errors (distinct from the sandboxed command's exit code).
pub mod exit_codes {
    /// Sandbox configuration error (invalid args, missing workspace, etc.).
    pub const CONFIG_ERROR: i32 = 2;
    /// The sandboxed command exceeded its timeout.
    pub const TIMEOUT: i32 = 3;
    /// The sandboxed command exceeded resource limits (memory, PIDs, etc.).
    pub const RESOURCE_EXCEEDED: i32 = 4;
}
