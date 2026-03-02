//! Integration tests for the Docker fallback sandbox backend.
//!
//! These tests require Docker to be installed and running.
//! They are skipped automatically if Docker is not available.

#![allow(clippy::unwrap_used)]

use std::collections::HashMap;

use oa_sandbox::SandboxRunner;
use oa_sandbox::config::{
    BackendPreference, NetworkPolicy, OutputFormat, ResourceLimits, SandboxConfig, SecurityLevel,
};
use oa_sandbox::docker::DockerFallbackRunner;

/// Check if Docker is available and return the runner, or skip the test.
fn make_runner() -> DockerFallbackRunner {
    DockerFallbackRunner::new()
}

fn docker_available() -> bool {
    which::which("docker").is_ok()
        && std::process::Command::new("docker")
            .arg("info")
            .output()
            .map(|o| o.status.success())
            .unwrap_or(false)
}

fn make_config(command: &str, args: &[&str], level: SecurityLevel) -> SandboxConfig {
    SandboxConfig {
        security_level: level,
        command: command.into(),
        args: args.iter().map(|s| (*s).into()).collect(),
        workspace: std::env::temp_dir(),
        mounts: vec![],
        resource_limits: ResourceLimits {
            timeout_secs: Some(30),
            ..ResourceLimits::default()
        },
        network_policy: None,
        env_vars: HashMap::new(),
        format: OutputFormat::Json,
        backend: BackendPreference::Docker,
    }
}

// ── Basic execution ────────────────────────────────────────────────────────

#[test]
fn docker_echo_hello() {
    if !docker_available() {
        eprintln!("SKIP: Docker not available");
        return;
    }

    let runner = make_runner();
    let config = make_config("echo", &["hello", "world"], SecurityLevel::L1Allowlist);
    let output = runner.run(&config).unwrap();

    assert_eq!(output.exit_code, 0);
    assert!(
        output.stdout.trim().contains("hello world"),
        "stdout: {:?}",
        output.stdout
    );
    assert_eq!(output.sandbox_backend, "docker-fallback");
}

#[test]
fn docker_nonzero_exit() {
    if !docker_available() {
        eprintln!("SKIP: Docker not available");
        return;
    }

    let runner = make_runner();
    let config = make_config("sh", &["-c", "exit 42"], SecurityLevel::L1Allowlist);
    let output = runner.run(&config).unwrap();

    assert_eq!(output.exit_code, 42);
}

#[test]
fn docker_command_not_found() {
    if !docker_available() {
        eprintln!("SKIP: Docker not available");
        return;
    }

    let runner = make_runner();
    let config = make_config("/nonexistent/binary", &[], SecurityLevel::L1Allowlist);
    let result = runner.run(&config);

    assert!(result.is_err());
}

// ── Timeout ────────────────────────────────────────────────────────────────

#[test]
fn docker_timeout() {
    if !docker_available() {
        eprintln!("SKIP: Docker not available");
        return;
    }

    let runner = make_runner();
    let config = SandboxConfig {
        security_level: SecurityLevel::L1Allowlist,
        command: "sleep".into(),
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
        backend: BackendPreference::Docker,
    };

    let start = std::time::Instant::now();
    let result = runner.run(&config);
    let elapsed = start.elapsed();

    assert!(result.is_err());
    let err = result.unwrap_err();
    assert!(err.to_string().contains("timed out"), "error: {err}");
    assert!(
        elapsed.as_secs() < 15,
        "should timeout around 2s, took {elapsed:?}"
    );
}

// ── Environment variables ──────────────────────────────────────────────────

#[test]
fn docker_env_vars() {
    if !docker_available() {
        eprintln!("SKIP: Docker not available");
        return;
    }

    let runner = make_runner();
    let mut config = make_config("sh", &["-c", "echo $MY_VAR"], SecurityLevel::L1Allowlist);
    config.env_vars.insert("MY_VAR".into(), "test_value".into());

    let output = runner.run(&config).unwrap();
    assert_eq!(output.exit_code, 0);
    assert!(
        output.stdout.trim().contains("test_value"),
        "stdout: {:?}",
        output.stdout
    );
}

// ── Workspace mount ────────────────────────────────────────────────────────

#[test]
fn docker_workspace_mounted() {
    if !docker_available() {
        eprintln!("SKIP: Docker not available");
        return;
    }

    let tmpdir = tempfile::tempdir().unwrap();
    let marker = tmpdir.path().join("marker.txt");
    std::fs::write(&marker, "hello from host").unwrap();

    let runner = make_runner();
    let config = SandboxConfig {
        security_level: SecurityLevel::L1Allowlist,
        command: "cat".into(),
        args: vec!["/workspace/marker.txt".into()],
        workspace: tmpdir.path().to_path_buf(),
        mounts: vec![],
        resource_limits: ResourceLimits {
            timeout_secs: Some(10),
            ..ResourceLimits::default()
        },
        network_policy: None,
        env_vars: HashMap::new(),
        format: OutputFormat::Json,
        backend: BackendPreference::Docker,
    };

    let output = runner.run(&config).unwrap();
    assert_eq!(output.exit_code, 0);
    assert!(
        output.stdout.trim().contains("hello from host"),
        "stdout: {:?}",
        output.stdout
    );
}

// ── Network policy ─────────────────────────────────────────────────────────

#[test]
fn docker_network_none_blocks_connectivity() {
    if !docker_available() {
        eprintln!("SKIP: Docker not available");
        return;
    }

    let runner = make_runner();
    let mut config = make_config(
        "sh",
        &["-c", "wget -q -O /dev/null http://1.1.1.1 2>&1; echo $?"],
        SecurityLevel::L0Deny,
    );
    config.network_policy = Some(NetworkPolicy::None);
    config.resource_limits.timeout_secs = Some(10);

    let output = runner.run(&config).unwrap();
    // The wget should fail (non-zero exit from the sh script)
    // because --network=none blocks all connectivity
    assert!(
        output.stdout.trim() != "0",
        "wget should fail with --network=none, stdout: {:?}",
        output.stdout
    );
}

// ── Backend selection via select_runner ─────────────────────────────────────

#[test]
fn select_runner_docker_backend() {
    if !docker_available() {
        eprintln!("SKIP: Docker not available");
        return;
    }

    let config = SandboxConfig {
        security_level: SecurityLevel::L1Allowlist,
        command: "echo".into(),
        args: vec!["test".into()],
        workspace: std::env::temp_dir(),
        mounts: vec![],
        resource_limits: ResourceLimits::default(),
        network_policy: None,
        env_vars: HashMap::new(),
        format: OutputFormat::Json,
        backend: BackendPreference::Docker,
    };

    let runner = oa_sandbox::select_runner(&config).unwrap();
    assert_eq!(runner.name(), "docker-fallback");
    assert!(runner.available());
}
