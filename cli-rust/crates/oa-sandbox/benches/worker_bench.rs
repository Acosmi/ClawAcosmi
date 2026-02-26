//! Persistent Worker IPC latency benchmarks.
//!
//! Measures the round-trip time for executing commands via the persistent
//! sandbox Worker's JSON-Lines IPC, compared to cold-start (fork per call).
//!
//! Targets:
//! - Persistent Worker IPC round-trip: <1ms for simple commands
//! - Cold-start (baseline comparison): ~65ms (macOS Seatbelt), ~215ms (Docker)
//!
//! Run with: `cargo bench -p oa-sandbox --bench worker_bench`
//!
//! **Requires**: `OA_CLI_BINARY` environment variable pointing to the compiled CLI.

use criterion::{BenchmarkId, Criterion, criterion_group, criterion_main};

use oa_sandbox::worker::launcher::{WorkerLaunchConfig, launch_worker_unsandboxed};

/// Launch an unsandboxed Worker for benchmarking.
///
/// Returns `None` if `OA_CLI_BINARY` is not set (benchmark will be skipped).
fn spawn_bench_worker() -> Option<oa_sandbox::worker::handle::WorkerHandle> {
    if std::env::var("OA_CLI_BINARY").is_err() {
        return None;
    }
    let config = WorkerLaunchConfig {
        workspace: std::env::temp_dir(),
        default_timeout_secs: 30,
        ..WorkerLaunchConfig::default()
    };
    launch_worker_unsandboxed(&config).ok()
}

/// Benchmark: persistent Worker IPC round-trip (echo command).
fn bench_persistent_worker_exec(c: &mut Criterion) {
    let mut group = c.benchmark_group("persistent_worker");

    let mut worker = match spawn_bench_worker() {
        Some(w) => w,
        None => {
            eprintln!("SKIP: OA_CLI_BINARY not set, cannot benchmark worker");
            return;
        }
    };

    // Warm up — first call includes event-loop startup latency
    let _ = worker.execute("/bin/echo", &["warmup"]);

    group.bench_function(BenchmarkId::new("echo", "ipc"), |b| {
        b.iter(|| {
            let resp = worker.execute("/bin/echo", &["hello"]).unwrap();
            assert_eq!(resp.exit_code, 0);
        });
    });

    group.bench_function(BenchmarkId::new("true", "ipc"), |b| {
        b.iter(|| {
            let resp = worker.execute("/usr/bin/true", &[]).unwrap();
            assert_eq!(resp.exit_code, 0);
        });
    });

    // Ping round-trip (no fork+exec, pure IPC)
    group.bench_function(BenchmarkId::new("ping", "ipc"), |b| {
        b.iter(|| {
            worker.ping().unwrap();
        });
    });

    group.finish();
    let _ = worker.shutdown();
}

/// Benchmark: persistent Worker vs fork-per-call comparison.
///
/// Spawns a new Worker for each "cold start" call to measure the overhead
/// difference between persistent and per-call execution models.
fn bench_persistent_vs_cold_start(c: &mut Criterion) {
    if std::env::var("OA_CLI_BINARY").is_err() {
        eprintln!("SKIP: OA_CLI_BINARY not set, cannot benchmark worker");
        return;
    }

    let mut group = c.benchmark_group("persistent_vs_cold");
    group.sample_size(10); // cold start is slow

    // Cold start: spawn worker + execute + shutdown per iteration
    group.bench_function("cold_start_echo", |b| {
        b.iter(|| {
            let mut worker = spawn_bench_worker().unwrap();
            let resp = worker.execute("/bin/echo", &["hello"]).unwrap();
            assert_eq!(resp.exit_code, 0);
            let _ = worker.shutdown();
        });
    });

    // Persistent: single worker, multiple calls
    let mut worker = spawn_bench_worker().unwrap();
    let _ = worker.execute("/bin/echo", &["warmup"]);

    group.bench_function("persistent_echo", |b| {
        b.iter(|| {
            let resp = worker.execute("/bin/echo", &["hello"]).unwrap();
            assert_eq!(resp.exit_code, 0);
        });
    });

    group.finish();
    let _ = worker.shutdown();
}

criterion_group!(
    benches,
    bench_persistent_worker_exec,
    bench_persistent_vs_cold_start,
);
criterion_main!(benches);
