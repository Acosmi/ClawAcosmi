//! Persistent sandbox Worker process.
//!
//! The Worker runs inside a sandbox (Seatbelt/Landlock/Seccomp), reads command
//! requests from stdin via JSON-Lines, executes them via `std::process::Command`
//! (child processes inherit the sandbox), and writes responses to stdout.
//!
//! # Architecture
//!
//! ```text
//! Parent (Go/CLI)          Worker process (sandboxed)
//!     │                         │
//!     ├─ write request ────────▶│ stdin BufReader
//!     │                         │   ├─ __ping__  → respond ok
//!     │                         │   ├─ __shutdown__ → respond ok + exit
//!     │                         │   └─ command → fork+exec → collect output
//!     │◀── read response ───────┤ stdout writeln
//!     │                         │
//!     ├─ close stdin ──────────▶│ EOF detected → exit
//! ```

pub mod handle;
pub mod launcher;
pub mod protocol;

use std::io::{self, Read};
use std::process::{Command, Stdio};
use std::time::{Duration, Instant};

use tracing::{debug, info, warn};

use protocol::{WorkerRequest, WorkerResponse, read_request, write_response};

/// Default timeout for commands if not specified per-request or at Worker level.
pub const DEFAULT_TIMEOUT_SECS: u64 = 120;

/// Default idle timeout — 0 means no idle timeout (wait forever for input).
pub const DEFAULT_IDLE_TIMEOUT_SECS: u64 = 0;

/// Configuration for the Worker event loop.
#[derive(Debug, Clone)]
pub struct WorkerConfig {
    /// Default working directory for commands.
    pub workspace: std::path::PathBuf,

    /// Default timeout in seconds for commands without per-request timeout.
    pub default_timeout_secs: u64,

    /// Idle timeout in seconds. If no request arrives within this duration,
    /// the Worker exits gracefully. 0 = no idle timeout.
    pub idle_timeout_secs: u64,
}

/// Run the Worker event loop.
///
/// Reads [`WorkerRequest`] messages from `stdin`, executes commands, and writes
/// [`WorkerResponse`] messages to `stdout`. Exits on EOF, `__shutdown__`, or
/// idle timeout (if configured).
///
/// This function is intended to be called from the Worker binary entry point
/// **after** the sandbox has been applied (Seatbelt/Landlock/Seccomp).
///
/// # Idle timeout
///
/// When `config.idle_timeout_secs > 0`, a background thread reads from stdin
/// and sends parsed requests through a channel. The main thread uses
/// `recv_timeout` to detect idle periods. On timeout, the Worker exits.
///
/// # Errors
///
/// Returns `Err` on unrecoverable I/O errors (stdin/stdout broken).
/// Individual command failures are reported as error responses, not as `Err`.
pub fn run_event_loop(config: &WorkerConfig) -> io::Result<()> {
    let stdout = io::stdout();
    let mut writer = io::BufWriter::new(stdout.lock());

    info!(
        workspace = %config.workspace.display(),
        idle_timeout_secs = config.idle_timeout_secs,
        "worker event loop started"
    );

    // Use a channel-based reader to support idle timeout.
    // A background thread reads stdin lines and sends them to the main loop.
    let (tx, rx) = std::sync::mpsc::channel::<io::Result<Option<WorkerRequest>>>();

    std::thread::spawn(move || {
        let stdin = io::stdin();
        let mut reader = io::BufReader::new(stdin.lock());
        loop {
            let result = read_request(&mut reader);
            let is_eof = matches!(&result, Ok(None));
            // Distinguish fatal I/O errors from recoverable parse errors.
            let is_fatal_io_err = matches!(
                &result,
                Err(e) if e.kind() != io::ErrorKind::InvalidData
            );
            // If send fails, the main thread has exited — stop reading.
            if tx.send(result).is_err() {
                break;
            }
            if is_eof || is_fatal_io_err {
                // EOF or broken pipe — stop reading.
                break;
            }
            // Parse errors (InvalidData) are recoverable — continue reading.
        }
    });

    let idle_timeout = if config.idle_timeout_secs > 0 {
        Some(Duration::from_secs(config.idle_timeout_secs))
    } else {
        None
    };

    loop {
        // Wait for next request, with optional idle timeout
        let recv_result = if let Some(timeout) = idle_timeout {
            match rx.recv_timeout(timeout) {
                Ok(val) => Some(val),
                Err(std::sync::mpsc::RecvTimeoutError::Timeout) => {
                    info!(
                        idle_timeout_secs = config.idle_timeout_secs,
                        "idle timeout reached, worker exiting"
                    );
                    break;
                }
                Err(std::sync::mpsc::RecvTimeoutError::Disconnected) => {
                    info!("stdin reader disconnected, worker exiting");
                    break;
                }
            }
        } else {
            match rx.recv() {
                Ok(val) => Some(val),
                Err(_) => {
                    info!("stdin reader disconnected, worker exiting");
                    break;
                }
            }
        };

        let request = match recv_result {
            Some(Ok(Some(req))) => req,
            Some(Ok(None)) => {
                // EOF — parent closed stdin, exit gracefully
                info!("stdin EOF, worker exiting");
                break;
            }
            Some(Err(e)) => {
                // Parse error — send error response with id=0 and continue
                warn!(error = %e, "failed to parse request");
                let err_resp = WorkerResponse::err(0, format!("parse error: {e}"));
                write_response(&mut writer, &err_resp)?;
                continue;
            }
            None => break,
        };

        debug!(id = request.id, command = %request.command, "received request");

        // Handle special commands
        if request.is_ping() {
            debug!(id = request.id, "ping");
            write_response(&mut writer, &WorkerResponse::ok(request.id))?;
            continue;
        }

        if request.is_shutdown() {
            info!(id = request.id, "shutdown requested");
            write_response(&mut writer, &WorkerResponse::ok(request.id))?;
            break;
        }

        // Execute command
        let response = execute_command(&request, config);
        write_response(&mut writer, &response)?;
    }

    info!("worker event loop exited");
    Ok(())
}

/// Execute a single command and return a [`WorkerResponse`].
///
/// Uses `std::process::Command` which fork+exec's a child process.
/// The child inherits the Worker's sandbox constraints (verified by Skill 5).
///
/// Timeout handling: takes stdout/stderr from the Child, reads them in
/// background threads, then polls `child.try_wait()` with a deadline.
/// On timeout, calls `child.kill()` to send SIGKILL, then `child.wait()`
/// to reap. This avoids the pitfalls of `wait_with_output()` + raw PID kill.
fn execute_command(request: &WorkerRequest, config: &WorkerConfig) -> WorkerResponse {
    let start = Instant::now();

    // Determine working directory
    let cwd = request
        .cwd
        .as_ref()
        .map_or_else(|| config.workspace.clone(), std::path::PathBuf::from);

    // Build command
    let mut cmd = Command::new(&request.command);
    cmd.args(&request.args)
        .current_dir(&cwd)
        .stdin(Stdio::null())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());

    // Set per-command environment variables
    for (key, value) in &request.env {
        cmd.env(key, value);
    }

    // Spawn child
    let mut child = match cmd.spawn() {
        Ok(c) => c,
        Err(e) => {
            let duration_ms = elapsed_ms(&start);
            return if e.kind() == io::ErrorKind::NotFound {
                WorkerResponse {
                    id: request.id,
                    stdout: String::new(),
                    stderr: String::new(),
                    exit_code: -1,
                    duration_ms,
                    error: Some(format!("command not found: {}", request.command)),
                }
            } else {
                WorkerResponse {
                    id: request.id,
                    stdout: String::new(),
                    stderr: String::new(),
                    exit_code: -1,
                    duration_ms,
                    error: Some(format!("spawn failed: {e}")),
                }
            };
        }
    };

    let child_pid = child.id();
    debug!(id = request.id, pid = child_pid, "child spawned");

    // Take stdout/stderr handles BEFORE waiting — this is critical for
    // correct timeout behavior. We read them in background threads so
    // the main thread can poll try_wait() with a deadline.
    let child_stdout = child.stdout.take();
    let child_stderr = child.stderr.take();

    // Spawn reader threads
    let stdout_thread = std::thread::spawn(move || {
        let mut buf = Vec::new();
        if let Some(mut out) = child_stdout {
            let _ = out.read_to_end(&mut buf);
        }
        buf
    });
    let stderr_thread = std::thread::spawn(move || {
        let mut buf = Vec::new();
        if let Some(mut err) = child_stderr {
            let _ = err.read_to_end(&mut buf);
        }
        buf
    });

    // Timeout handling: poll try_wait with deadline
    let timeout_secs = request
        .timeout_secs
        .unwrap_or(config.default_timeout_secs);
    let deadline = Instant::now() + Duration::from_secs(timeout_secs);
    let mut timed_out = false;

    let exit_status = loop {
        match child.try_wait() {
            Ok(Some(status)) => break Ok(status),
            Ok(None) => {
                if Instant::now() >= deadline {
                    // Timeout — kill the child
                    warn!(
                        id = request.id,
                        pid = child_pid,
                        timeout_secs,
                        "killing timed-out child"
                    );
                    timed_out = true;
                    let _ = child.kill();
                    // Reap the process to prevent zombie
                    break child.wait();
                }
                std::thread::sleep(Duration::from_millis(20));
            }
            Err(e) => break Err(e),
        }
    };

    let duration_ms = elapsed_ms(&start);

    // Join reader threads (they will finish once the child's pipes close)
    let stdout_data = stdout_thread.join().unwrap_or_default();
    let stderr_data = stderr_thread.join().unwrap_or_default();

    if timed_out {
        return WorkerResponse {
            id: request.id,
            stdout: String::new(),
            stderr: String::new(),
            exit_code: -1,
            duration_ms,
            error: Some(format!("command timed out after {timeout_secs}s")),
        };
    }

    match exit_status {
        Ok(status) => {
            let exit_code = status.code().unwrap_or(-1);
            let stdout = String::from_utf8_lossy(&stdout_data).into_owned();
            let stderr = String::from_utf8_lossy(&stderr_data).into_owned();

            debug!(
                id = request.id,
                pid = child_pid,
                exit_code,
                duration_ms,
                "child completed"
            );

            WorkerResponse {
                id: request.id,
                stdout,
                stderr,
                exit_code,
                duration_ms,
                error: None,
            }
        }
        Err(e) => WorkerResponse {
            id: request.id,
            stdout: String::new(),
            stderr: String::new(),
            exit_code: -1,
            duration_ms,
            error: Some(format!("wait failed: {e}")),
        },
    }
}

/// Convert elapsed duration to milliseconds, clamping to u64::MAX.
fn elapsed_ms(start: &Instant) -> u64 {
    u64::try_from(start.elapsed().as_millis()).unwrap_or(u64::MAX)
}

#[cfg(test)]
#[allow(clippy::unwrap_used)]
mod tests {
    use super::*;
    use std::io::Cursor;

    /// Helper: run the event loop with simulated stdin/stdout.
    fn run_with_input(input: &str, config: &WorkerConfig) -> Vec<WorkerResponse> {
        let mut reader = io::BufReader::new(Cursor::new(input.as_bytes().to_vec()));
        let mut output_buf = Vec::new();

        // Inline event loop for testing (avoids locking real stdin/stdout)
        loop {
            let request = match read_request(&mut reader) {
                Ok(Some(req)) => req,
                Ok(None) => break,
                Err(e) => {
                    let err_resp = WorkerResponse::err(0, format!("parse error: {e}"));
                    write_response(&mut output_buf, &err_resp).unwrap();
                    continue;
                }
            };

            if request.is_ping() {
                write_response(&mut output_buf, &WorkerResponse::ok(request.id)).unwrap();
                continue;
            }
            if request.is_shutdown() {
                write_response(&mut output_buf, &WorkerResponse::ok(request.id)).unwrap();
                break;
            }

            let response = execute_command(&request, config);
            write_response(&mut output_buf, &response).unwrap();
        }

        // Parse responses from output buffer
        let output_str = String::from_utf8(output_buf).unwrap();
        output_str
            .lines()
            .filter(|l| !l.is_empty())
            .map(|l| serde_json::from_str(l).unwrap())
            .collect()
    }

    fn test_config() -> WorkerConfig {
        WorkerConfig {
            workspace: std::env::temp_dir(),
            default_timeout_secs: DEFAULT_TIMEOUT_SECS,
            idle_timeout_secs: DEFAULT_IDLE_TIMEOUT_SECS,
        }
    }

    #[test]
    fn ping_response() {
        let input = r#"{"id":1,"command":"__ping__"}"#.to_string() + "\n";
        let responses = run_with_input(&input, &test_config());
        assert_eq!(responses.len(), 1);
        assert_eq!(responses[0].id, 1);
        assert_eq!(responses[0].exit_code, 0);
    }

    #[test]
    fn shutdown_exits_loop() {
        let input = format!(
            "{}\n{}\n{}\n",
            r#"{"id":1,"command":"__ping__"}"#,
            r#"{"id":2,"command":"__shutdown__"}"#,
            r#"{"id":3,"command":"__ping__"}"#, // should not be reached
        );
        let responses = run_with_input(&input, &test_config());
        assert_eq!(responses.len(), 2); // ping + shutdown ack, no third
        assert_eq!(responses[0].id, 1);
        assert_eq!(responses[1].id, 2);
    }

    #[test]
    fn echo_command() {
        let input = r#"{"id":1,"command":"/bin/echo","args":["hello","world"]}"#.to_string() + "\n";
        let responses = run_with_input(&input, &test_config());
        assert_eq!(responses.len(), 1);
        assert_eq!(responses[0].id, 1);
        assert_eq!(responses[0].exit_code, 0);
        assert_eq!(responses[0].stdout.trim(), "hello world");
        assert!(responses[0].error.is_none());
    }

    #[test]
    fn command_not_found() {
        let input =
            r#"{"id":1,"command":"/nonexistent/binary_that_does_not_exist"}"#.to_string() + "\n";
        let responses = run_with_input(&input, &test_config());
        assert_eq!(responses.len(), 1);
        assert_eq!(responses[0].id, 1);
        assert_eq!(responses[0].exit_code, -1);
        assert!(responses[0].error.as_ref().unwrap().contains("not found"));
    }

    #[test]
    fn multiple_commands_sequential() {
        let input = format!(
            "{}\n{}\n{}\n",
            r#"{"id":1,"command":"/bin/echo","args":["first"]}"#,
            r#"{"id":2,"command":"/bin/echo","args":["second"]}"#,
            r#"{"id":3,"command":"/bin/echo","args":["third"]}"#,
        );
        let responses = run_with_input(&input, &test_config());
        assert_eq!(responses.len(), 3);
        assert_eq!(responses[0].stdout.trim(), "first");
        assert_eq!(responses[1].stdout.trim(), "second");
        assert_eq!(responses[2].stdout.trim(), "third");
    }

    #[test]
    fn eof_exits_gracefully() {
        // Empty input = immediate EOF
        let responses = run_with_input("", &test_config());
        assert!(responses.is_empty());
    }

    #[test]
    fn invalid_json_returns_error_and_continues() {
        let input = format!(
            "{}\n{}\n",
            "not valid json",
            r#"{"id":2,"command":"__ping__"}"#,
        );
        let responses = run_with_input(&input, &test_config());
        assert_eq!(responses.len(), 2);
        // First response: parse error with id=0
        assert_eq!(responses[0].id, 0);
        assert!(responses[0].error.is_some());
        // Second response: successful ping
        assert_eq!(responses[1].id, 2);
        assert_eq!(responses[1].exit_code, 0);
    }

    #[test]
    fn command_with_nonzero_exit() {
        let input = r#"{"id":1,"command":"/bin/sh","args":["-c","exit 42"]}"#.to_string() + "\n";
        let responses = run_with_input(&input, &test_config());
        assert_eq!(responses.len(), 1);
        assert_eq!(responses[0].exit_code, 42);
        assert!(responses[0].error.is_none()); // command ran, just non-zero exit
    }

    #[test]
    fn command_stderr_capture() {
        let input =
            r#"{"id":1,"command":"/bin/sh","args":["-c","echo err >&2"]}"#.to_string() + "\n";
        let responses = run_with_input(&input, &test_config());
        assert_eq!(responses.len(), 1);
        assert_eq!(responses[0].stderr.trim(), "err");
    }

    #[test]
    fn duration_is_positive() {
        let input = r#"{"id":1,"command":"/bin/echo","args":["hi"]}"#.to_string() + "\n";
        let responses = run_with_input(&input, &test_config());
        assert_eq!(responses.len(), 1);
        // Duration should be >= 0 (echo is fast, but not negative)
        assert!(responses[0].duration_ms < 5000);
    }
}
