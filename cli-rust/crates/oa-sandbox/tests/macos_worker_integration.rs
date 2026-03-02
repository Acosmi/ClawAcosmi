//! Integration tests for macOS Seatbelt-sandboxed persistent Worker.
//!
//! These tests verify that commands executed through the persistent Worker
//! inherit the Seatbelt sandbox constraints. This confirms the Skill 5
//! verification: fork()+exec() children inherit sandbox_init() restrictions.
//!
//! Requires macOS to run. Requires the CLI binary to be built first.
//! Run with: `OA_CLI_BINARY=target/debug/openacosmi cargo test --test macos_worker_integration`

#![cfg(target_os = "macos")]
#![allow(clippy::unwrap_used)]

use std::collections::HashMap;

use oa_sandbox::config::SecurityLevel;
use oa_sandbox::worker::handle::WorkerHandle;
use oa_sandbox::worker::launcher::{WorkerLaunchConfig, launch_worker};
use oa_sandbox::worker::protocol::WorkerRequest;

/// Helper: launch a Seatbelt-sandboxed Worker.
fn spawn_sandboxed_worker(workspace: std::path::PathBuf, security: SecurityLevel) -> WorkerHandle {
    let config = WorkerLaunchConfig {
        security_level: security,
        workspace,
        network_policy: None,
        mounts: vec![],
        default_timeout_secs: 30,
        idle_timeout_secs: 0,
    };
    launch_worker(&config).expect("failed to launch sandboxed worker")
}

// ── Basic sandboxed execution ──────────────────────────────────────────

#[test]
fn sandboxed_worker_echo() {
    let workspace = std::env::temp_dir();
    let mut handle = spawn_sandboxed_worker(workspace, SecurityLevel::L1Allowlist);

    let resp = handle.execute("/bin/echo", &["hello", "sandboxed"]).unwrap();
    assert_eq!(resp.exit_code, 0);
    assert_eq!(resp.stdout.trim(), "hello sandboxed");
    assert!(resp.error.is_none());

    handle.shutdown().unwrap();
}

#[test]
fn sandboxed_worker_ping() {
    let workspace = std::env::temp_dir();
    let mut handle = spawn_sandboxed_worker(workspace, SecurityLevel::L1Allowlist);
    handle.ping().unwrap();
    handle.shutdown().unwrap();
}

#[test]
fn sandboxed_worker_multiple_commands() {
    let workspace = std::env::temp_dir();
    let mut handle = spawn_sandboxed_worker(workspace, SecurityLevel::L1Allowlist);

    for i in 0..5 {
        let resp = handle.execute("/bin/echo", &[&format!("cmd-{i}")]).unwrap();
        assert_eq!(resp.exit_code, 0);
        assert_eq!(resp.stdout.trim(), format!("cmd-{i}"));
    }

    handle.shutdown().unwrap();
}

// ── Workspace access (sandboxed) ──────────────────────────────────────

#[test]
fn sandboxed_worker_can_read_workspace() {
    let tmpdir = tempfile::tempdir().unwrap();
    let test_file = tmpdir.path().join("worker-test.txt");
    std::fs::write(&test_file, "worker-content").unwrap();

    let mut handle = spawn_sandboxed_worker(tmpdir.path().to_path_buf(), SecurityLevel::L1Allowlist);

    let resp = handle.execute("/bin/cat", &[test_file.to_str().unwrap()]).unwrap();
    assert_eq!(resp.exit_code, 0);
    assert_eq!(resp.stdout.trim(), "worker-content");

    handle.shutdown().unwrap();
}

#[test]
fn sandboxed_worker_can_write_workspace() {
    let tmpdir = tempfile::tempdir().unwrap();
    let output_file = tmpdir.path().join("output.txt");

    let mut handle = spawn_sandboxed_worker(tmpdir.path().to_path_buf(), SecurityLevel::L1Allowlist);

    let cmd = format!("echo 'written-by-worker' > '{}'", output_file.display());
    let resp = handle.execute("/bin/sh", &["-c", &cmd]).unwrap();
    assert_eq!(resp.exit_code, 0);

    let content = std::fs::read_to_string(&output_file).unwrap();
    assert_eq!(content.trim(), "written-by-worker");

    handle.shutdown().unwrap();
}

// ── Filesystem isolation (sandboxed) ─────────────────────────────────

#[test]
fn sandboxed_worker_cannot_read_outside_workspace_l0() {
    // Create a secret file outside the workspace
    #[allow(clippy::expect_used)]
    let home = std::env::var("HOME").expect("HOME not set");
    let test_dir = std::path::PathBuf::from(&home).join(".oa-worker-test");
    std::fs::create_dir_all(&test_dir).unwrap();
    let secret_file = test_dir.join("secret.txt");
    std::fs::write(&secret_file, "top-secret").unwrap();

    // Use a different directory as workspace
    let workspace = tempfile::tempdir().unwrap();
    let mut handle = spawn_sandboxed_worker(workspace.path().to_path_buf(), SecurityLevel::L0Deny);

    let resp = handle.execute("/bin/cat", &[secret_file.to_str().unwrap()]).unwrap();

    // Cleanup
    let _ = std::fs::remove_dir_all(&test_dir);

    // Command should fail — file is outside sandbox
    assert_ne!(resp.exit_code, 0, "should not be able to read {}", secret_file.display());
}

#[test]
fn sandboxed_worker_cannot_write_outside_workspace_l0() {
    let workspace = tempfile::tempdir().unwrap();

    // Try to write to a location outside workspace
    #[allow(clippy::expect_used)]
    let home = std::env::var("HOME").expect("HOME not set");
    let target = std::path::PathBuf::from(&home).join(".oa-worker-test-write");
    // Ensure cleanup
    let _ = std::fs::remove_file(&target);

    let mut handle = spawn_sandboxed_worker(workspace.path().to_path_buf(), SecurityLevel::L0Deny);

    let cmd = format!("echo 'escape' > '{}'", target.display());
    let resp = handle.execute("/bin/sh", &["-c", &cmd]).unwrap();

    // Cleanup
    let _ = std::fs::remove_file(&target);

    // Command should fail — cannot write outside sandbox
    assert_ne!(resp.exit_code, 0, "should not be able to write outside workspace");
}

// ── Multiple isolation checks in one Worker session ──────────────────

#[test]
fn sandboxed_worker_isolation_persists_across_commands() {
    let workspace = tempfile::tempdir().unwrap();
    let workspace_file = workspace.path().join("allowed.txt");
    std::fs::write(&workspace_file, "ok").unwrap();

    #[allow(clippy::expect_used)]
    let home = std::env::var("HOME").expect("HOME not set");
    let forbidden_dir = std::path::PathBuf::from(&home).join(".oa-worker-persist-test");
    std::fs::create_dir_all(&forbidden_dir).unwrap();
    let forbidden_file = forbidden_dir.join("forbidden.txt");
    std::fs::write(&forbidden_file, "nope").unwrap();

    let mut handle = spawn_sandboxed_worker(workspace.path().to_path_buf(), SecurityLevel::L0Deny);

    // 1. Allowed: read workspace file
    let r1 = handle.execute("/bin/cat", &[workspace_file.to_str().unwrap()]).unwrap();
    assert_eq!(r1.exit_code, 0);
    assert_eq!(r1.stdout.trim(), "ok");

    // 2. Blocked: read outside workspace
    let r2 = handle.execute("/bin/cat", &[forbidden_file.to_str().unwrap()]).unwrap();
    assert_ne!(r2.exit_code, 0);

    // 3. Allowed: still works after blocked attempt
    let r3 = handle.execute("/bin/echo", &["still-alive"]).unwrap();
    assert_eq!(r3.exit_code, 0);
    assert_eq!(r3.stdout.trim(), "still-alive");

    // Cleanup
    let _ = std::fs::remove_dir_all(&forbidden_dir);

    handle.shutdown().unwrap();
}

// ── Environment variables in sandboxed Worker ────────────────────────

#[test]
fn sandboxed_worker_env_vars() {
    let workspace = std::env::temp_dir();
    let mut handle = spawn_sandboxed_worker(workspace, SecurityLevel::L1Allowlist);

    let req = WorkerRequest {
        id: 1,
        command: "/bin/sh".into(),
        args: vec!["-c".into(), "echo $SANDBOX_VAR".into()],
        env: HashMap::from([("SANDBOX_VAR".into(), "sandbox_value".into())]),
        cwd: None,
        timeout_secs: None,
    };

    let resp = handle.exec(req).unwrap();
    assert_eq!(resp.exit_code, 0);
    assert_eq!(resp.stdout.trim(), "sandbox_value");

    handle.shutdown().unwrap();
}
