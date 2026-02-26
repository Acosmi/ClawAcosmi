//! Cold-start latency benchmarks for oa-sandbox.
//!
//! Measures the end-to-end time from `runner.run()` call to `SandboxOutput` return.
//! This includes process spawn, sandbox setup, command execution, and cleanup.
//!
//! Targets:
//! - Native backend: <10ms cold start (Linux Landlock+Seccomp)
//! - macOS Seatbelt: ~50-70ms (SBPL profile compilation overhead)
//! - Docker fallback: <500ms cold start (container spin-up)
//!
//! Run with: `cargo bench -p oa-sandbox --bench cold_start`

use std::collections::HashMap;

use criterion::{BenchmarkId, Criterion, criterion_group, criterion_main};

use oa_sandbox::config::{
    BackendPreference, NetworkPolicy, OutputFormat, ResourceLimits, SandboxConfig, SecurityLevel,
};

/// Resolve `true` command path per platform.
/// Alpine (Docker) uses `/bin/true`; macOS/Linux host uses `/usr/bin/true`.
fn true_cmd(for_docker: bool) -> String {
    if for_docker {
        "/bin/true".into()
    } else if cfg!(target_os = "windows") {
        "cmd.exe".into()
    } else {
        "/usr/bin/true".into()
    }
}

/// Create a minimal benchmark config that runs `true` (exits immediately).
fn bench_config(security: SecurityLevel, for_docker: bool) -> SandboxConfig {
    SandboxConfig {
        security_level: security,
        command: true_cmd(for_docker),
        args: if !for_docker && cfg!(target_os = "windows") {
            vec!["/C".into(), "exit".into(), "0".into()]
        } else {
            vec![]
        },
        workspace: std::env::temp_dir(),
        mounts: vec![],
        resource_limits: ResourceLimits {
            timeout_secs: Some(30),
            ..ResourceLimits::default()
        },
        network_policy: None,
        env_vars: HashMap::new(),
        format: OutputFormat::Json,
        backend: BackendPreference::Auto,
    }
}

/// Create a config that runs `echo hello` for a slightly heavier workload.
fn echo_config(security: SecurityLevel, for_docker: bool) -> SandboxConfig {
    let (command, args) = if for_docker {
        ("/bin/echo".into(), vec!["hello".into()])
    } else if cfg!(target_os = "windows") {
        (
            "cmd.exe".into(),
            vec!["/C".into(), "echo".into(), "hello".into()],
        )
    } else {
        ("/bin/echo".into(), vec!["hello".into()])
    };

    SandboxConfig {
        security_level: security,
        command,
        args,
        workspace: std::env::temp_dir(),
        mounts: vec![],
        resource_limits: ResourceLimits {
            timeout_secs: Some(30),
            ..ResourceLimits::default()
        },
        network_policy: None,
        env_vars: HashMap::new(),
        format: OutputFormat::Json,
        backend: BackendPreference::Auto,
    }
}

/// Benchmark the native sandbox backend cold start.
fn bench_native_cold_start(c: &mut Criterion) {
    let mut group = c.benchmark_group("native_cold_start");

    let native_config = SandboxConfig {
        backend: BackendPreference::Native,
        ..bench_config(SecurityLevel::L1Sandbox, false)
    };

    let runner = match oa_sandbox::select_runner(&native_config) {
        Ok(r) => r,
        Err(e) => {
            eprintln!("SKIP native benchmarks: {e}");
            group.finish();
            return;
        }
    };

    if !runner.available() {
        eprintln!("SKIP native benchmarks: {} not available", runner.name());
        group.finish();
        return;
    }

    let backend_name = runner.name();

    // Benchmark `true` (minimal command — measures pure sandbox overhead)
    group.bench_with_input(
        BenchmarkId::new("true", backend_name),
        &SecurityLevel::L1Sandbox,
        |b, &sec| {
            let cfg = SandboxConfig {
                backend: BackendPreference::Native,
                ..bench_config(sec, false)
            };
            b.iter(|| {
                let result = runner.run(&cfg);
                assert!(result.is_ok(), "benchmark run failed: {result:?}");
            });
        },
    );

    // Benchmark `echo hello` (light I/O)
    group.bench_with_input(
        BenchmarkId::new("echo", backend_name),
        &SecurityLevel::L1Sandbox,
        |b, &sec| {
            let cfg = SandboxConfig {
                backend: BackendPreference::Native,
                ..echo_config(sec, false)
            };
            b.iter(|| {
                let result = runner.run(&cfg);
                assert!(result.is_ok(), "benchmark run failed: {result:?}");
            });
        },
    );

    // Benchmark L0 (deny) — more restrictive, may have different overhead
    group.bench_with_input(
        BenchmarkId::new("true_L0", backend_name),
        &SecurityLevel::L0Deny,
        |b, &sec| {
            let cfg = SandboxConfig {
                backend: BackendPreference::Native,
                network_policy: Some(NetworkPolicy::None),
                ..bench_config(sec, false)
            };
            b.iter(|| {
                let result = runner.run(&cfg);
                assert!(result.is_ok(), "benchmark run failed: {result:?}");
            });
        },
    );

    group.finish();
}

/// Benchmark the Docker fallback cold start.
fn bench_docker_cold_start(c: &mut Criterion) {
    let mut group = c.benchmark_group("docker_cold_start");

    // Docker benchmarks are slower — reduce sample count
    group.sample_size(10);
    group.warm_up_time(std::time::Duration::from_secs(3));
    group.measurement_time(std::time::Duration::from_secs(20));

    let config = SandboxConfig {
        backend: BackendPreference::Docker,
        ..bench_config(SecurityLevel::L1Sandbox, true)
    };

    let runner = match oa_sandbox::select_runner(&config) {
        Ok(r) => r,
        Err(e) => {
            eprintln!("SKIP Docker benchmarks: {e}");
            group.finish();
            return;
        }
    };

    if !runner.available() {
        eprintln!("SKIP Docker benchmarks: Docker not available");
        group.finish();
        return;
    }

    // Benchmark `true` via Docker (measures container spin-up + sandbox overhead)
    group.bench_function("true", |b| {
        let cfg = SandboxConfig {
            backend: BackendPreference::Docker,
            ..bench_config(SecurityLevel::L1Sandbox, true)
        };
        b.iter(|| {
            let result = runner.run(&cfg);
            assert!(result.is_ok(), "Docker benchmark run failed: {result:?}");
        });
    });

    // Benchmark `echo hello` via Docker
    group.bench_function("echo", |b| {
        let cfg = SandboxConfig {
            backend: BackendPreference::Docker,
            ..echo_config(SecurityLevel::L1Sandbox, true)
        };
        b.iter(|| {
            let result = runner.run(&cfg);
            assert!(result.is_ok(), "Docker benchmark run failed: {result:?}");
        });
    });

    group.finish();
}

/// Benchmark raw `Command::new` without sandbox (baseline reference).
fn bench_baseline_no_sandbox(c: &mut Criterion) {
    let mut group = c.benchmark_group("baseline_no_sandbox");

    group.bench_function("true", |b| {
        b.iter(|| {
            let output = std::process::Command::new(if cfg!(target_os = "windows") {
                "cmd.exe"
            } else {
                "/usr/bin/true"
            })
            .args(if cfg!(target_os = "windows") {
                vec!["/C", "exit", "0"]
            } else {
                vec![]
            })
            .output()
            .expect("failed to run baseline command");
            assert!(output.status.success());
        });
    });

    group.bench_function("echo", |b| {
        b.iter(|| {
            let output = std::process::Command::new(if cfg!(target_os = "windows") {
                "cmd.exe"
            } else {
                "/bin/echo"
            })
            .args(if cfg!(target_os = "windows") {
                vec!["/C", "echo", "hello"]
            } else {
                vec!["hello"]
            })
            .output()
            .expect("failed to run baseline command");
            assert!(output.status.success());
        });
    });

    group.finish();
}

criterion_group!(
    benches,
    bench_baseline_no_sandbox,
    bench_native_cold_start,
    bench_docker_cold_start,
);
criterion_main!(benches);
