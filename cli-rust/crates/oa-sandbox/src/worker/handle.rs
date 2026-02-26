//! Client-side handle for communicating with a persistent sandbox Worker.
//!
//! [`WorkerHandle`] owns the Worker child process and provides a typed API
//! for sending commands and receiving responses over JSON-Lines IPC.
//!
//! The `Drop` implementation ensures the Worker is cleaned up (stdin closed →
//! SIGTERM → wait) to prevent zombie processes.

use std::io::{self, BufReader};
use std::process::{Child, ChildStdin, ChildStdout};
use std::time::{Duration, Instant};

use tracing::{debug, info, warn};

use super::protocol::{
    WorkerRequest, WorkerResponse, commands, read_response, write_request,
};

/// Handle to a running Worker process.
///
/// Provides `exec`, `ping`, and `shutdown` methods for interacting with the
/// Worker over stdin/stdout pipes.
///
/// # Drop behavior
///
/// On drop, the handle:
/// 1. Closes stdin (signals EOF to the Worker)
/// 2. Waits up to 5 seconds for graceful exit
/// 3. Sends SIGKILL if still running (Child::kill() uses SIGKILL on Unix)
/// 4. Waits for the process to reap (prevents zombies)
pub struct WorkerHandle {
    child: Option<Child>,
    stdin: Option<ChildStdin>,
    stdout: Option<BufReader<ChildStdout>>,
    next_id: u64,
}

impl WorkerHandle {
    /// Create a new handle from a spawned Worker child process.
    ///
    /// The child process must have `stdin` and `stdout` piped.
    ///
    /// # Errors
    ///
    /// Returns `Err` if the child process does not have piped stdin/stdout.
    pub fn new(mut child: Child) -> io::Result<Self> {
        let stdin = child.stdin.take().ok_or_else(|| {
            io::Error::new(io::ErrorKind::InvalidInput, "worker child has no stdin pipe")
        })?;
        let stdout = child.stdout.take().ok_or_else(|| {
            io::Error::new(
                io::ErrorKind::InvalidInput,
                "worker child has no stdout pipe",
            )
        })?;

        Ok(Self {
            child: Some(child),
            stdin: Some(stdin),
            stdout: Some(BufReader::new(stdout)),
            next_id: 1,
        })
    }

    /// Execute a command in the Worker sandbox and return the response.
    ///
    /// This is the primary API for sending commands. The Worker fork+exec's
    /// the command inside its sandbox, collects output, and responds.
    ///
    /// # Errors
    ///
    /// Returns `Err` on I/O failure (broken pipe, Worker crashed).
    /// Command execution failures are returned as `Ok(response)` with
    /// `response.error` set.
    pub fn exec(&mut self, request: WorkerRequest) -> io::Result<WorkerResponse> {
        let stdin = self.stdin.as_mut().ok_or_else(|| {
            io::Error::new(io::ErrorKind::BrokenPipe, "worker stdin already closed")
        })?;
        let stdout = self.stdout.as_mut().ok_or_else(|| {
            io::Error::new(io::ErrorKind::BrokenPipe, "worker stdout already closed")
        })?;

        write_request(stdin, &request)?;

        read_response(stdout)?.ok_or_else(|| {
            io::Error::new(
                io::ErrorKind::UnexpectedEof,
                "worker closed stdout unexpectedly",
            )
        })
    }

    /// Send a command by specifying command string and args.
    ///
    /// Convenience wrapper around [`exec`](Self::exec) that auto-assigns an ID.
    ///
    /// # Errors
    ///
    /// Returns `Err` on I/O failure.
    pub fn execute(
        &mut self,
        command: &str,
        args: &[&str],
    ) -> io::Result<WorkerResponse> {
        let id = self.next_id;
        self.next_id += 1;

        let request = WorkerRequest {
            id,
            command: command.into(),
            args: args.iter().map(|&s| s.into()).collect(),
            env: std::collections::HashMap::new(),
            cwd: None,
            timeout_secs: None,
        };

        self.exec(request)
    }

    /// Send a health check ping to the Worker.
    ///
    /// Returns `Ok(())` if the Worker responds with `exit_code: 0`.
    ///
    /// # Errors
    ///
    /// Returns `Err` if the Worker is unreachable or responds with an error.
    pub fn ping(&mut self) -> io::Result<()> {
        let id = self.next_id;
        self.next_id += 1;

        let request = WorkerRequest {
            id,
            command: commands::PING.into(),
            args: vec![],
            env: std::collections::HashMap::new(),
            cwd: None,
            timeout_secs: None,
        };

        let response = self.exec(request)?;
        if response.exit_code == 0 {
            Ok(())
        } else {
            Err(io::Error::new(
                io::ErrorKind::Other,
                format!("ping failed: exit_code={}", response.exit_code),
            ))
        }
    }

    /// Request the Worker to shut down gracefully.
    ///
    /// Sends a `__shutdown__` command, reads the acknowledgment, closes stdin,
    /// and waits for the process to exit.
    ///
    /// # Errors
    ///
    /// Returns `Err` on I/O failure or if the process cannot be reaped.
    pub fn shutdown(mut self) -> io::Result<()> {
        let id = self.next_id;
        self.next_id += 1;

        let request = WorkerRequest {
            id,
            command: commands::SHUTDOWN.into(),
            args: vec![],
            env: std::collections::HashMap::new(),
            cwd: None,
            timeout_secs: None,
        };

        // Send shutdown command (best-effort — Worker might already be gone)
        if let (Some(stdin), Some(stdout)) = (self.stdin.as_mut(), self.stdout.as_mut()) {
            if write_request(stdin, &request).is_ok() {
                let _ = read_response(stdout);
            }
        }

        // Close stdin to signal EOF
        drop(self.stdin.take());
        drop(self.stdout.take());

        // Wait for process to exit
        if let Some(mut child) = self.child.take() {
            child.wait().map_err(|e| {
                io::Error::new(
                    io::ErrorKind::Other,
                    format!("failed to wait for worker process: {e}"),
                )
            })?;
        }

        info!("worker shut down gracefully");
        Ok(())
    }

    /// Returns the PID of the Worker process, if still tracked.
    #[must_use]
    pub fn pid(&self) -> Option<u32> {
        self.child.as_ref().map(Child::id)
    }

    /// Check if the Worker process has exited (non-blocking).
    ///
    /// Returns `Some(status)` if the process has exited, `None` if still running.
    ///
    /// # Errors
    ///
    /// Returns `Err` on I/O failure.
    pub fn try_wait(&mut self) -> io::Result<Option<std::process::ExitStatus>> {
        if let Some(child) = self.child.as_mut() {
            child.try_wait()
        } else {
            Err(io::Error::new(
                io::ErrorKind::NotFound,
                "worker process already reaped",
            ))
        }
    }
}

impl Drop for WorkerHandle {
    fn drop(&mut self) {
        // 1. Close stdin to signal EOF
        drop(self.stdin.take());
        drop(self.stdout.take());

        // 2. Wait for graceful exit (up to 5 seconds)
        if let Some(mut child) = self.child.take() {
            let start = Instant::now();
            let grace_period = Duration::from_secs(5);

            loop {
                match child.try_wait() {
                    Ok(Some(status)) => {
                        debug!(
                            pid = child.id(),
                            ?status,
                            "worker exited during drop"
                        );
                        return;
                    }
                    Ok(None) => {
                        if start.elapsed() >= grace_period {
                            break;
                        }
                        std::thread::sleep(Duration::from_millis(50));
                    }
                    Err(e) => {
                        warn!(error = %e, "try_wait failed during drop");
                        break;
                    }
                }
            }

            // 3. Grace period expired — send SIGKILL then wait
            warn!("worker did not exit within grace period, sending SIGKILL");
            let _ = child.kill();
            let _ = child.wait(); // reap to prevent zombie
        }
    }
}
