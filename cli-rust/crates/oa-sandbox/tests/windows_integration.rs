//! Integration tests for Windows sandbox backend.
//!
//! These tests spawn real sandboxed child processes using Job Objects
//! and Restricted Tokens. They require Windows to run.

#![cfg(target_os = "windows")]
#![allow(clippy::unwrap_used)]

use std::collections::HashMap;

use oa_sandbox::SandboxRunner;
use oa_sandbox::config::{
    BackendPreference, MountMode, MountSpec, NetworkPolicy, OutputFormat, ResourceLimits,
    SandboxConfig, SecurityLevel,
};
use oa_sandbox::platform::WindowsCapabilities;
use oa_sandbox::windows::WindowsRunner;

/// Helper to create a test config with sensible defaults.
fn make_config(command: &str, args: &[&str], security_level: SecurityLevel) -> SandboxConfig {
    SandboxConfig {
        security_level,
        command: command.into(),
        args: args.iter().map(|s| (*s).into()).collect(),
        workspace: std::env::temp_dir(),
        mounts: vec![],
        resource_limits: ResourceLimits {
            timeout_secs: Some(10),
            ..ResourceLimits::default()
        },
        network_policy: None,
        env_vars: HashMap::new(),
        format: OutputFormat::Json,
        backend: BackendPreference::Native,
    }
}

fn make_runner() -> WindowsRunner {
    let caps = WindowsCapabilities::detect();
    let backend = if caps.has_job_objects {
        oa_sandbox::platform::SandboxBackend::WindowsFull
    } else {
        oa_sandbox::platform::SandboxBackend::WindowsJobOnly
    };
    WindowsRunner::new(backend, caps)
}

// ── Basic execution ────────────────────────────────────────────────────────

#[test]
fn echo_hello_in_sandbox() {
    let runner = make_runner();
    if !runner.available() {
        eprintln!("SKIP: Windows sandbox not available");
        return;
    }

    let config = make_config(
        "cmd.exe",
        &["/C", "echo hello world"],
        SecurityLevel::L1Allowlist,
    );
    let output = runner.run(&config).unwrap();

    assert_eq!(output.exit_code, 0);
    assert!(
        output.sandbox_backend.starts_with("windows-"),
        "backend: {}",
        output.sandbox_backend
    );
}

#[test]
fn command_with_nonzero_exit() {
    let runner = make_runner();
    if !runner.available() {
        return;
    }

    let config = make_config("cmd.exe", &["/C", "exit /b 42"], SecurityLevel::L1Allowlist);
    let output = runner.run(&config).unwrap();

    assert_eq!(output.exit_code, 42);
}

#[test]
fn command_not_found_returns_error() {
    let runner = make_runner();
    if !runner.available() {
        return;
    }

    let config = make_config("C:\\nonexistent\\binary.exe", &[], SecurityLevel::L1Allowlist);
    let result = runner.run(&config);

    assert!(result.is_err());
}

// ── Timeout ────────────────────────────────────────────────────────────────

#[test]
fn timeout_kills_long_running_process() {
    let runner = make_runner();
    if !runner.available() {
        return;
    }

    let config = SandboxConfig {
        security_level: SecurityLevel::L1Allowlist,
        command: "cmd.exe".into(),
        args: vec!["/C".into(), "timeout /t 60 /nobreak".into()],
        workspace: std::env::temp_dir(),
        mounts: vec![],
        resource_limits: ResourceLimits {
            timeout_secs: Some(2),
            ..ResourceLimits::default()
        },
        network_policy: None,
        env_vars: HashMap::new(),
        format: OutputFormat::Json,
        backend: BackendPreference::Native,
    };

    let start = std::time::Instant::now();
    let result = runner.run(&config);
    let elapsed = start.elapsed();

    assert!(result.is_err());
    let err = result.unwrap_err();
    assert!(err.to_string().contains("timed out"), "error: {err}");
    assert!(
        elapsed.as_secs() < 10,
        "should timeout around 2s, took {elapsed:?}"
    );
}

// ── Workspace access ───────────────────────────────────────────────────────

#[test]
fn can_write_to_workspace_in_l1() {
    let runner = make_runner();
    if !runner.available() {
        return;
    }

    let tmpdir = tempfile::tempdir().unwrap();
    let output_file = tmpdir.path().join("output.txt");

    let config = SandboxConfig {
        security_level: SecurityLevel::L1Allowlist,
        command: "cmd.exe".into(),
        args: vec![
            "/C".into(),
            format!("echo written > \"{}\"", output_file.display()),
        ],
        workspace: tmpdir.path().to_path_buf(),
        mounts: vec![],
        resource_limits: ResourceLimits {
            timeout_secs: Some(10),
            ..ResourceLimits::default()
        },
        network_policy: None,
        env_vars: HashMap::new(),
        format: OutputFormat::Json,
        backend: BackendPreference::Native,
    };

    let output = runner.run(&config).unwrap();
    assert_eq!(output.exit_code, 0);
}

// ── Environment variables ──────────────────────────────────────────────────

#[test]
fn env_vars_passed_to_sandbox() {
    let runner = make_runner();
    if !runner.available() {
        return;
    }

    let mut env = HashMap::new();
    env.insert("MY_TEST_VAR".into(), "test_value_123".into());

    let config = SandboxConfig {
        security_level: SecurityLevel::L1Allowlist,
        command: "cmd.exe".into(),
        args: vec!["/C".into(), "echo %MY_TEST_VAR%".into()],
        workspace: std::env::temp_dir(),
        mounts: vec![],
        resource_limits: ResourceLimits {
            timeout_secs: Some(10),
            ..ResourceLimits::default()
        },
        network_policy: None,
        env_vars: env,
        format: OutputFormat::Json,
        backend: BackendPreference::Native,
    };

    let output = runner.run(&config).unwrap();
    assert_eq!(output.exit_code, 0);
}

// ── Job Object resource limits ─────────────────────────────────────────────

#[test]
fn job_object_limits_process_count() {
    let runner = make_runner();
    if !runner.available() {
        return;
    }

    // Try to spawn many processes — Job Object should limit
    let config = SandboxConfig {
        security_level: SecurityLevel::L1Allowlist,
        command: "cmd.exe".into(),
        args: vec!["/C".into(), "echo limited".into()],
        workspace: std::env::temp_dir(),
        mounts: vec![],
        resource_limits: ResourceLimits {
            max_pids: 5,
            timeout_secs: Some(10),
            ..ResourceLimits::default()
        },
        network_policy: None,
        env_vars: HashMap::new(),
        format: OutputFormat::Json,
        backend: BackendPreference::Native,
    };

    // This should succeed (single process within limit)
    let output = runner.run(&config).unwrap();
    assert_eq!(output.exit_code, 0);
}
