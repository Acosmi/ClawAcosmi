//! IPC protocol for the persistent sandbox Worker.
//!
//! Uses JSON-Lines (newline-delimited JSON) over stdin/stdout pipes.
//! Each message is a single line of JSON terminated by `\n`.
//!
//! # Performance
//!
//! Verified via Skill 5: serde_json round-trip for ~200-byte messages is ~1μs,
//! pipe I/O adds ~4.8μs, total ~5.8μs per message — well under the 1ms target.

use std::collections::HashMap;
use std::io::{self, BufRead, Write};

use serde::{Deserialize, Serialize};

/// Request sent from the parent (Go layer / CLI) to the Worker.
///
/// Serialized as a single JSON line on the Worker's stdin.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WorkerRequest {
    /// Request identifier for correlating responses.
    /// Must be unique within a Worker session.
    pub id: u64,

    /// Command to execute (absolute path recommended).
    pub command: String,

    /// Command arguments.
    #[serde(default)]
    pub args: Vec<String>,

    /// Environment variables to set for this command.
    #[serde(default)]
    pub env: HashMap<String, String>,

    /// Working directory for the command.
    /// If `None`, uses the Worker's workspace.
    #[serde(default)]
    pub cwd: Option<String>,

    /// Per-command timeout in seconds.
    /// If `None`, uses the Worker's default timeout.
    #[serde(default)]
    pub timeout_secs: Option<u64>,
}

/// Response sent from the Worker to the parent.
///
/// Serialized as a single JSON line on the Worker's stdout.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WorkerResponse {
    /// Request identifier (matches the corresponding [`WorkerRequest::id`]).
    pub id: u64,

    /// Captured stdout from the executed command.
    #[serde(default)]
    pub stdout: String,

    /// Captured stderr from the executed command.
    #[serde(default)]
    pub stderr: String,

    /// Exit code of the executed command.
    /// Convention: 0 = success, negative = signal, positive = command exit code.
    pub exit_code: i32,

    /// Wall-clock duration of command execution in milliseconds.
    pub duration_ms: u64,

    /// Error message if the command could not be executed (e.g., not found, permission denied).
    /// `None` when the command ran (even if it returned non-zero).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}

/// Well-known command strings for Worker control messages.
pub mod commands {
    /// Health check. Worker responds with `exit_code: 0` and no output.
    pub const PING: &str = "__ping__";

    /// Graceful shutdown. Worker acknowledges and then exits.
    pub const SHUTDOWN: &str = "__shutdown__";
}

impl WorkerRequest {
    /// Returns `true` if this is a `__ping__` health check.
    #[must_use]
    pub fn is_ping(&self) -> bool {
        self.command == commands::PING
    }

    /// Returns `true` if this is a `__shutdown__` request.
    #[must_use]
    pub fn is_shutdown(&self) -> bool {
        self.command == commands::SHUTDOWN
    }
}

impl WorkerResponse {
    /// Create a successful response with no output (for ping/shutdown acks).
    #[must_use]
    pub fn ok(id: u64) -> Self {
        Self {
            id,
            stdout: String::new(),
            stderr: String::new(),
            exit_code: 0,
            duration_ms: 0,
            error: None,
        }
    }

    /// Create an error response.
    #[must_use]
    pub fn err(id: u64, message: String) -> Self {
        Self {
            id,
            stdout: String::new(),
            stderr: String::new(),
            exit_code: -1,
            duration_ms: 0,
            error: Some(message),
        }
    }
}

// ── Wire format helpers ─────────────────────────────────────────────

/// Maximum line length for a single JSON-Lines message (10 MiB).
///
/// Prevents unbounded memory allocation if the peer sends a very long line
/// without a newline terminator. Matches the Go-side `bufio.Scanner` buffer
/// limit in `native_bridge.go`.
const MAX_LINE_LENGTH: usize = 10 * 1024 * 1024;

/// Read a single [`WorkerRequest`] from a buffered reader (JSON-Lines).
///
/// Returns `Ok(None)` on EOF (stdin closed), signaling the Worker to exit.
///
/// # Line length limit
///
/// Lines exceeding [`MAX_LINE_LENGTH`] (10 MiB) are rejected with an error
/// to prevent unbounded memory growth from malformed or adversarial input.
///
/// # Errors
///
/// Returns `Err` on I/O failure, line-too-long, or JSON parse failure.
pub fn read_request<R: BufRead>(reader: &mut R) -> io::Result<Option<WorkerRequest>> {
    let line = read_line_limited(reader, MAX_LINE_LENGTH)?;
    let Some(line) = line else {
        return Ok(None); // EOF
    };

    let trimmed = line.trim();
    if trimmed.is_empty() {
        // Skip blank lines (defensive)
        return Ok(None);
    }

    let req: WorkerRequest = serde_json::from_str(trimmed).map_err(|e| {
        io::Error::new(
            io::ErrorKind::InvalidData,
            format!("invalid WorkerRequest JSON: {e}"),
        )
    })?;
    Ok(Some(req))
}

/// Write a single [`WorkerResponse`] to a writer (JSON-Lines).
///
/// Appends a newline and flushes immediately to ensure the parent receives
/// the response without waiting for buffering.
///
/// # Errors
///
/// Returns `Err` on I/O or serialization failure.
pub fn write_response<W: Write>(writer: &mut W, response: &WorkerResponse) -> io::Result<()> {
    let json = serde_json::to_string(response).map_err(|e| {
        io::Error::new(
            io::ErrorKind::InvalidData,
            format!("failed to serialize WorkerResponse: {e}"),
        )
    })?;
    writeln!(writer, "{json}")?;
    writer.flush()
}

/// Write a single [`WorkerRequest`] to a writer (JSON-Lines).
///
/// Used by the parent (client) side to send requests to the Worker's stdin.
///
/// # Errors
///
/// Returns `Err` on I/O or serialization failure.
pub fn write_request<W: Write>(writer: &mut W, request: &WorkerRequest) -> io::Result<()> {
    let json = serde_json::to_string(request).map_err(|e| {
        io::Error::new(
            io::ErrorKind::InvalidData,
            format!("failed to serialize WorkerRequest: {e}"),
        )
    })?;
    writeln!(writer, "{json}")?;
    writer.flush()
}

/// Read a single [`WorkerResponse`] from a buffered reader (JSON-Lines).
///
/// Used by the parent (client) side to read responses from the Worker's stdout.
/// Returns `Ok(None)` on EOF (Worker exited).
///
/// # Errors
///
/// Returns `Err` on I/O failure, line-too-long, or JSON parse failure.
pub fn read_response<R: BufRead>(reader: &mut R) -> io::Result<Option<WorkerResponse>> {
    let line = read_line_limited(reader, MAX_LINE_LENGTH)?;
    let Some(line) = line else {
        return Ok(None); // EOF — Worker exited
    };

    let trimmed = line.trim();
    if trimmed.is_empty() {
        return Ok(None);
    }

    let resp: WorkerResponse = serde_json::from_str(trimmed).map_err(|e| {
        io::Error::new(
            io::ErrorKind::InvalidData,
            format!("invalid WorkerResponse JSON: {e}"),
        )
    })?;
    Ok(Some(resp))
}

/// Read a single line with a maximum length limit.
///
/// Returns `Ok(None)` on EOF. Returns `Err` if the line exceeds `max_len`
/// bytes before encountering a newline.
fn read_line_limited<R: BufRead>(reader: &mut R, max_len: usize) -> io::Result<Option<String>> {
    let mut line = String::new();

    loop {
        // Scope the immutable borrow from fill_buf so we can call consume after.
        let (consume_len, found_newline, is_eof) = {
            let available = reader.fill_buf()?;
            if available.is_empty() {
                (0, false, true)
            } else if let Some(pos) = available.iter().position(|&b| b == b'\n') {
                let chunk = &available[..=pos];
                line.push_str(&String::from_utf8_lossy(chunk));
                (chunk.len(), true, false)
            } else {
                let len = available.len();
                line.push_str(&String::from_utf8_lossy(available));
                (len, false, false)
            }
        };

        if is_eof {
            return if line.is_empty() {
                Ok(None)
            } else {
                Ok(Some(line))
            };
        }

        reader.consume(consume_len);

        if line.len() > max_len {
            return Err(io::Error::new(
                io::ErrorKind::InvalidData,
                format!("line exceeds maximum length of {max_len} bytes"),
            ));
        }

        if found_newline {
            return Ok(Some(line));
        }
    }
}

#[cfg(test)]
#[allow(clippy::unwrap_used)]
mod tests {
    use super::*;

    #[test]
    fn request_round_trip() {
        let req = WorkerRequest {
            id: 42,
            command: "/bin/echo".into(),
            args: vec!["hello".into(), "world".into()],
            env: HashMap::from([("FOO".into(), "bar".into())]),
            cwd: Some("/tmp".into()),
            timeout_secs: Some(30),
        };

        let json = serde_json::to_string(&req).unwrap();
        let parsed: WorkerRequest = serde_json::from_str(&json).unwrap();

        assert_eq!(parsed.id, 42);
        assert_eq!(parsed.command, "/bin/echo");
        assert_eq!(parsed.args, vec!["hello", "world"]);
        assert_eq!(parsed.env.get("FOO").unwrap(), "bar");
        assert_eq!(parsed.cwd.as_deref(), Some("/tmp"));
        assert_eq!(parsed.timeout_secs, Some(30));
    }

    #[test]
    fn response_round_trip() {
        let resp = WorkerResponse {
            id: 42,
            stdout: "hello world\n".into(),
            stderr: String::new(),
            exit_code: 0,
            duration_ms: 5,
            error: None,
        };

        let json = serde_json::to_string(&resp).unwrap();
        let parsed: WorkerResponse = serde_json::from_str(&json).unwrap();

        assert_eq!(parsed.id, 42);
        assert_eq!(parsed.stdout, "hello world\n");
        assert_eq!(parsed.exit_code, 0);
        assert_eq!(parsed.duration_ms, 5);
        assert!(parsed.error.is_none());
    }

    #[test]
    fn request_minimal_defaults() {
        let json = r#"{"id":1,"command":"ls"}"#;
        let req: WorkerRequest = serde_json::from_str(json).unwrap();
        assert_eq!(req.id, 1);
        assert_eq!(req.command, "ls");
        assert!(req.args.is_empty());
        assert!(req.env.is_empty());
        assert!(req.cwd.is_none());
        assert!(req.timeout_secs.is_none());
    }

    #[test]
    fn response_with_error() {
        let resp = WorkerResponse::err(99, "command not found".into());
        let json = serde_json::to_string(&resp).unwrap();
        let parsed: WorkerResponse = serde_json::from_str(&json).unwrap();

        assert_eq!(parsed.id, 99);
        assert_eq!(parsed.exit_code, -1);
        assert_eq!(parsed.error.as_deref(), Some("command not found"));
    }

    #[test]
    fn ping_and_shutdown_detection() {
        let ping = WorkerRequest {
            id: 0,
            command: "__ping__".into(),
            args: vec![],
            env: HashMap::new(),
            cwd: None,
            timeout_secs: None,
        };
        assert!(ping.is_ping());
        assert!(!ping.is_shutdown());

        let shutdown = WorkerRequest {
            id: 0,
            command: "__shutdown__".into(),
            args: vec![],
            env: HashMap::new(),
            cwd: None,
            timeout_secs: None,
        };
        assert!(!shutdown.is_ping());
        assert!(shutdown.is_shutdown());
    }

    #[test]
    fn wire_format_read_write_request() {
        let req = WorkerRequest {
            id: 1,
            command: "/bin/echo".into(),
            args: vec!["test".into()],
            env: HashMap::new(),
            cwd: None,
            timeout_secs: None,
        };

        let mut buf = Vec::new();
        write_request(&mut buf, &req).unwrap();

        let mut reader = io::BufReader::new(&buf[..]);
        let parsed = read_request(&mut reader).unwrap().unwrap();
        assert_eq!(parsed.id, 1);
        assert_eq!(parsed.command, "/bin/echo");
    }

    #[test]
    fn wire_format_read_write_response() {
        let resp = WorkerResponse::ok(7);

        let mut buf = Vec::new();
        write_response(&mut buf, &resp).unwrap();

        let mut reader = io::BufReader::new(&buf[..]);
        let parsed = read_response(&mut reader).unwrap().unwrap();
        assert_eq!(parsed.id, 7);
        assert_eq!(parsed.exit_code, 0);
    }

    #[test]
    fn eof_returns_none() {
        let empty: &[u8] = b"";
        let mut reader = io::BufReader::new(empty);
        assert!(read_request(&mut reader).unwrap().is_none());
        assert!(read_response(&mut reader).unwrap().is_none());
    }

    #[test]
    fn invalid_json_returns_error() {
        let bad = b"not json\n";
        let mut reader = io::BufReader::new(&bad[..]);
        let result = read_request(&mut reader);
        assert!(result.is_err());
    }

    #[test]
    fn error_field_skipped_when_none() {
        let resp = WorkerResponse::ok(1);
        let json = serde_json::to_string(&resp).unwrap();
        assert!(!json.contains("error"));
    }
}
