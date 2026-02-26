//! Integration tests for the persistent sandbox Worker.
//!
//! These tests verify the full end-to-end flow: launcher → Worker process →
//! IPC protocol → WorkerHandle client API.
//!
//! Tests use `launch_worker_unsandboxed` to avoid requiring sandbox privileges.
//! Sandboxed Worker tests are in `macos_worker_integration.rs` (macOS).

#![allow(clippy::unwrap_used)]

use std::collections::HashMap;

use oa_sandbox::worker::handle::WorkerHandle;
use oa_sandbox::worker::launcher::{WorkerLaunchConfig, launch_worker_unsandboxed};
use oa_sandbox::worker::protocol::{WorkerRequest, commands};

/// Helper: launch an unsandboxed Worker for testing.
fn spawn_test_worker() -> WorkerHandle {
    let config = WorkerLaunchConfig {
        workspace: std::env::temp_dir(),
        default_timeout_secs: 30,
        ..WorkerLaunchConfig::default()
    };
    launch_worker_unsandboxed(&config).expect("failed to launch test worker")
}

#[test]
fn worker_ping() {
    let mut handle = spawn_test_worker();
    handle.ping().expect("ping failed");
    handle.shutdown().expect("shutdown failed");
}

#[test]
fn worker_echo() {
    let mut handle = spawn_test_worker();
    let resp = handle.execute("/bin/echo", &["hello", "worker"]).expect("exec failed");
    assert_eq!(resp.exit_code, 0);
    assert_eq!(resp.stdout.trim(), "hello worker");
    assert!(resp.error.is_none());
    handle.shutdown().expect("shutdown failed");
}

#[test]
fn worker_multiple_commands() {
    let mut handle = spawn_test_worker();

    let r1 = handle.execute("/bin/echo", &["first"]).expect("exec 1 failed");
    assert_eq!(r1.stdout.trim(), "first");

    let r2 = handle.execute("/bin/echo", &["second"]).expect("exec 2 failed");
    assert_eq!(r2.stdout.trim(), "second");

    let r3 = handle.execute("/bin/echo", &["third"]).expect("exec 3 failed");
    assert_eq!(r3.stdout.trim(), "third");

    handle.shutdown().expect("shutdown failed");
}

#[test]
fn worker_command_not_found() {
    let mut handle = spawn_test_worker();
    let resp = handle
        .execute("/nonexistent/binary_xyz_123", &[])
        .expect("exec should succeed even for missing commands");
    assert_eq!(resp.exit_code, -1);
    assert!(resp.error.as_ref().unwrap().contains("not found"));
    handle.shutdown().expect("shutdown failed");
}

#[test]
fn worker_nonzero_exit() {
    let mut handle = spawn_test_worker();
    let resp = handle.execute("/bin/sh", &["-c", "exit 42"]).expect("exec failed");
    assert_eq!(resp.exit_code, 42);
    assert!(resp.error.is_none()); // command ran, just non-zero exit
    handle.shutdown().expect("shutdown failed");
}

#[test]
fn worker_stderr_capture() {
    let mut handle = spawn_test_worker();
    let resp = handle
        .execute("/bin/sh", &["-c", "echo err_msg >&2"])
        .expect("exec failed");
    assert_eq!(resp.exit_code, 0);
    assert_eq!(resp.stderr.trim(), "err_msg");
    handle.shutdown().expect("shutdown failed");
}

#[test]
fn worker_env_vars() {
    let mut handle = spawn_test_worker();

    let req = WorkerRequest {
        id: 1,
        command: "/bin/sh".into(),
        args: vec!["-c".into(), "echo $MY_TEST_VAR".into()],
        env: HashMap::from([("MY_TEST_VAR".into(), "hello_env".into())]),
        cwd: None,
        timeout_secs: None,
    };

    let resp = handle.exec(req).expect("exec failed");
    assert_eq!(resp.exit_code, 0);
    assert_eq!(resp.stdout.trim(), "hello_env");
    handle.shutdown().expect("shutdown failed");
}

#[test]
fn worker_custom_cwd() {
    let mut handle = spawn_test_worker();

    let req = WorkerRequest {
        id: 1,
        command: "/bin/pwd".into(),
        args: vec![],
        env: HashMap::new(),
        cwd: Some("/tmp".into()),
        timeout_secs: None,
    };

    let resp = handle.exec(req).expect("exec failed");
    assert_eq!(resp.exit_code, 0);
    // macOS: /tmp → /private/tmp
    assert!(
        resp.stdout.trim() == "/tmp" || resp.stdout.trim() == "/private/tmp",
        "unexpected cwd: {}",
        resp.stdout.trim()
    );
    handle.shutdown().expect("shutdown failed");
}

#[test]
fn worker_shutdown_command() {
    let mut handle = spawn_test_worker();

    // Send a ping first
    handle.ping().expect("ping failed");

    // Send shutdown via exec (not the shutdown() method)
    let req = WorkerRequest {
        id: 99,
        command: commands::SHUTDOWN.into(),
        args: vec![],
        env: HashMap::new(),
        cwd: None,
        timeout_secs: None,
    };
    let resp = handle.exec(req).expect("shutdown exec failed");
    assert_eq!(resp.id, 99);
    assert_eq!(resp.exit_code, 0);

    // Worker should have exited — next read should fail or return EOF
    // Just drop the handle; Drop will clean up
}

#[test]
fn worker_drop_cleanup() {
    // Just spawn and drop — should not leak zombie processes
    let handle = spawn_test_worker();
    let pid = handle.pid();
    assert!(pid.is_some());
    drop(handle);

    // Give the OS a moment to clean up
    std::thread::sleep(std::time::Duration::from_millis(200));

    // On Unix, check the process is gone
    #[cfg(unix)]
    {
        if let Some(pid) = pid {
            if let Ok(p) = libc::pid_t::try_from(pid) {
                // SAFETY: Sending signal 0 to check if process exists.
                // Returns 0 if process exists, -1 with ESRCH if not.
                let exists = unsafe { libc::kill(p, 0) };
                assert_eq!(
                    exists, -1,
                    "worker process (pid {pid}) should not exist after drop"
                );
            }
        }
    }
}

#[test]
fn worker_duration_tracking() {
    let mut handle = spawn_test_worker();

    // Run a command that takes a measurable amount of time
    let resp = handle
        .execute("/bin/sh", &["-c", "sleep 0.1 && echo done"])
        .expect("exec failed");
    assert_eq!(resp.exit_code, 0);
    assert_eq!(resp.stdout.trim(), "done");
    // Duration should be at least 100ms
    assert!(
        resp.duration_ms >= 50, // Allow some slack
        "duration_ms should be >= 50, got {}",
        resp.duration_ms
    );

    handle.shutdown().expect("shutdown failed");
}

#[test]
fn worker_command_timeout() {
    let config = WorkerLaunchConfig {
        workspace: std::env::temp_dir(),
        default_timeout_secs: 2, // Short default timeout
        ..WorkerLaunchConfig::default()
    };
    let mut handle =
        launch_worker_unsandboxed(&config).expect("failed to launch test worker");

    // Run a command that exceeds the default timeout
    let start = std::time::Instant::now();
    let resp = handle.execute("/bin/sleep", &["60"]).expect("exec failed");
    let elapsed = start.elapsed();

    // Should have timed out
    assert_eq!(resp.exit_code, -1);
    assert!(resp.error.as_ref().unwrap().contains("timed out"));
    assert!(
        elapsed.as_secs() < 10,
        "should timeout around 2s, took {elapsed:?}"
    );

    // Worker should still be alive and responsive after a timed-out command
    handle.ping().expect("worker should still be alive after timeout");
    handle.shutdown().expect("shutdown failed");
}

#[test]
fn worker_per_request_timeout() {
    let mut handle = spawn_test_worker();

    // Send a request with per-request timeout override
    let req = WorkerRequest {
        id: 1,
        command: "/bin/sleep".into(),
        args: vec!["60".into()],
        env: HashMap::new(),
        cwd: None,
        timeout_secs: Some(1), // 1 second timeout
    };

    let start = std::time::Instant::now();
    let resp = handle.exec(req).expect("exec failed");
    let elapsed = start.elapsed();

    assert_eq!(resp.exit_code, -1);
    assert!(resp.error.as_ref().unwrap().contains("timed out"));
    assert!(elapsed.as_secs() < 5);

    handle.shutdown().expect("shutdown failed");
}

#[test]
fn worker_large_output() {
    let mut handle = spawn_test_worker();

    // Generate 10KB of output
    let resp = handle
        .execute("/bin/sh", &["-c", "yes hello | head -1000"])
        .expect("exec failed");
    assert_eq!(resp.exit_code, 0);
    let lines: Vec<&str> = resp.stdout.lines().collect();
    assert_eq!(lines.len(), 1000);
    assert!(lines.iter().all(|l| *l == "hello"));

    handle.shutdown().expect("shutdown failed");
}
