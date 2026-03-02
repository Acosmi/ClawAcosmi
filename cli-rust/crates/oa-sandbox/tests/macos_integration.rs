//! Integration tests for macOS Seatbelt sandbox backend.
//!
//! These tests spawn real sandboxed child processes and verify isolation behavior.
//! They require macOS to run.

#![cfg(target_os = "macos")]
#![allow(clippy::unwrap_used)]

use std::collections::HashMap;

use oa_sandbox::SandboxRunner;
use oa_sandbox::config::{
    BackendPreference, MountMode, MountSpec, NetworkPolicy, OutputFormat, ResourceLimits,
    SandboxConfig, SecurityLevel,
};
use oa_sandbox::macos::MacosRunner;
use oa_sandbox::platform::MacosCapabilities;

/// Helper to create a test config with sensible defaults.
fn make_config(
    command: &str,
    args: &[&str],
    security_level: SecurityLevel,
    network: Option<NetworkPolicy>,
) -> SandboxConfig {
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
        network_policy: network,
        env_vars: HashMap::new(),
        format: OutputFormat::Json,
        backend: BackendPreference::Native,
    }
}

fn make_runner() -> MacosRunner {
    let caps = MacosCapabilities::detect();
    MacosRunner::new(caps)
}

// ── Basic execution ────────────────────────────────────────────────────────

#[test]
fn echo_hello_in_sandbox() {
    let runner = make_runner();
    if !runner.available() {
        eprintln!("SKIP: Seatbelt not available");
        return;
    }

    let config = make_config(
        "/bin/echo",
        &["hello", "world"],
        SecurityLevel::L1Allowlist,
        None,
    );
    let output = runner.run(&config).unwrap();

    assert_eq!(output.exit_code, 0);
    assert_eq!(output.stdout.trim(), "hello world");
    assert_eq!(output.sandbox_backend, "macos-seatbelt");
    // Seatbelt init can be slow on first run; generous bound
    assert!(output.duration_ms < 15000, "took {}ms", output.duration_ms);
}

#[test]
fn command_with_nonzero_exit() {
    let runner = make_runner();
    if !runner.available() {
        return;
    }

    let config = make_config(
        "/bin/sh",
        &["-c", "exit 42"],
        SecurityLevel::L1Allowlist,
        None,
    );
    let output = runner.run(&config).unwrap();

    assert_eq!(output.exit_code, 42);
}

#[test]
fn command_not_found_returns_error() {
    let runner = make_runner();
    if !runner.available() {
        return;
    }

    let config = make_config("/nonexistent/binary", &[], SecurityLevel::L1Allowlist, None);
    let result = runner.run(&config);

    assert!(result.is_err());
}

// ── Workspace access ───────────────────────────────────────────────────────

#[test]
fn can_read_workspace_files() {
    let runner = make_runner();
    if !runner.available() {
        return;
    }

    // Create a temp file in the workspace
    let tmpdir = tempfile::tempdir().unwrap();
    let test_file = tmpdir.path().join("test.txt");
    std::fs::write(&test_file, "sandbox-content").unwrap();

    let config = SandboxConfig {
        security_level: SecurityLevel::L1Allowlist,
        command: "/bin/cat".into(),
        args: vec![test_file.to_str().unwrap().into()],
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
    assert_eq!(output.stdout.trim(), "sandbox-content");
}

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
        command: "/bin/sh".into(),
        args: vec![
            "-c".into(),
            format!("echo 'written' > '{}'", output_file.display()),
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
    assert_eq!(
        std::fs::read_to_string(&output_file).unwrap().trim(),
        "written"
    );
}

// ── Filesystem isolation ───────────────────────────────────────────────────

#[test]
fn cannot_read_outside_workspace_in_deny_mode() {
    let runner = make_runner();
    if !runner.available() {
        return;
    }

    // Create a file in a location NOT covered by any SBPL allowlist.
    // /tmp and TMPDIR are explicitly allowed, so we use a subdirectory
    // under HOME which is not in the sandbox profile.
    #[allow(clippy::expect_used)]
    let home = std::env::var("HOME").expect("HOME not set");
    let test_dir = std::path::PathBuf::from(&home).join(".oa-sandbox-test");
    std::fs::create_dir_all(&test_dir).unwrap();
    let secret_file = test_dir.join("secret.txt");
    std::fs::write(&secret_file, "secret").unwrap();

    // Use a different directory as workspace
    let workspace = tempfile::tempdir().unwrap();

    let config = SandboxConfig {
        security_level: SecurityLevel::L0Deny,
        command: "/bin/cat".into(),
        args: vec![secret_file.to_str().unwrap().into()],
        workspace: workspace.path().to_path_buf(),
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

    // Cleanup
    let _ = std::fs::remove_dir_all(&test_dir);

    // The command should fail because the file is outside workspace
    assert_ne!(output.exit_code, 0);
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
        command: "/bin/sleep".into(),
        args: vec!["60".into()],
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

// ── Additional mounts ──────────────────────────────────────────────────────

#[test]
fn additional_mount_readonly() {
    let runner = make_runner();
    if !runner.available() {
        return;
    }

    let extra_dir = tempfile::tempdir().unwrap();
    let extra_file = extra_dir.path().join("extra.txt");
    std::fs::write(&extra_file, "extra-content").unwrap();

    let workspace = tempfile::tempdir().unwrap();

    let config = SandboxConfig {
        security_level: SecurityLevel::L1Allowlist,
        command: "/bin/cat".into(),
        args: vec![extra_file.to_str().unwrap().into()],
        workspace: workspace.path().to_path_buf(),
        mounts: vec![MountSpec {
            host_path: extra_dir.path().to_path_buf(),
            sandbox_path: extra_dir.path().to_path_buf(),
            mode: MountMode::ReadOnly,
        }],
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
    assert_eq!(output.stdout.trim(), "extra-content");
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
        command: "/bin/sh".into(),
        args: vec!["-c".into(), "echo $MY_TEST_VAR".into()],
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
    assert_eq!(output.stdout.trim(), "test_value_123");
}
