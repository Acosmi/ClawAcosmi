//! Rust-side metrics counters for Argus-Core.
//!
//! Global atomic counters tracking operations performed by the Rust library.
//! Exported via C ABI as JSON for integration into Go's `/metrics` endpoint.

use std::sync::atomic::{AtomicU64, Ordering};
use serde::Serialize;

// ===== Global Counters =====

static FRAMES_CAPTURED: AtomicU64 = AtomicU64::new(0);
static RESIZES_TOTAL: AtomicU64 = AtomicU64::new(0);
static SHM_WRITES_TOTAL: AtomicU64 = AtomicU64::new(0);
static KEYFRAME_DIFFS_TOTAL: AtomicU64 = AtomicU64::new(0);
static PII_SCANS_TOTAL: AtomicU64 = AtomicU64::new(0);
static CRYPTO_OPS_TOTAL: AtomicU64 = AtomicU64::new(0);

// ===== Increment Functions (called from other modules) =====

pub fn inc_frames_captured() {
    FRAMES_CAPTURED.fetch_add(1, Ordering::Relaxed);
}

pub fn inc_resizes() {
    RESIZES_TOTAL.fetch_add(1, Ordering::Relaxed);
}

pub fn inc_shm_writes() {
    SHM_WRITES_TOTAL.fetch_add(1, Ordering::Relaxed);
}

pub fn inc_keyframe_diffs() {
    KEYFRAME_DIFFS_TOTAL.fetch_add(1, Ordering::Relaxed);
}

pub fn inc_pii_scans() {
    PII_SCANS_TOTAL.fetch_add(1, Ordering::Relaxed);
}

pub fn inc_crypto_ops() {
    CRYPTO_OPS_TOTAL.fetch_add(1, Ordering::Relaxed);
}

// ===== JSON Export =====

#[derive(Serialize)]
struct RustMetrics {
    rust_frames_captured_total: u64,
    rust_resizes_total: u64,
    rust_shm_writes_total: u64,
    rust_keyframe_diffs_total: u64,
    rust_pii_scans_total: u64,
    rust_crypto_ops_total: u64,
}

fn collect_metrics() -> RustMetrics {
    RustMetrics {
        rust_frames_captured_total: FRAMES_CAPTURED.load(Ordering::Relaxed),
        rust_resizes_total: RESIZES_TOTAL.load(Ordering::Relaxed),
        rust_shm_writes_total: SHM_WRITES_TOTAL.load(Ordering::Relaxed),
        rust_keyframe_diffs_total: KEYFRAME_DIFFS_TOTAL.load(Ordering::Relaxed),
        rust_pii_scans_total: PII_SCANS_TOTAL.load(Ordering::Relaxed),
        rust_crypto_ops_total: CRYPTO_OPS_TOTAL.load(Ordering::Relaxed),
    }
}

// ===== C ABI Exports =====

/// Export all Rust-side metrics as JSON.
///
/// # Safety
/// - `out_ptr` and `out_len` must be valid pointers.
/// - Caller must free the buffer via `argus_free_buffer`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_metrics_get(
    out_ptr: *mut *mut u8,
    out_len: *mut usize,
) -> i32 {
    if out_ptr.is_null() || out_len.is_null() {
        return crate::ARGUS_ERR_NULL_PTR;
    }

    let metrics = collect_metrics();
    let json = match serde_json::to_vec(&metrics) {
        Ok(j) => j,
        Err(_) => return crate::ARGUS_ERR_INTERNAL,
    };

    // into_boxed_slice guarantees capacity == len for argus_free_buffer
    let json_len = json.len();
    let json_ptr = Box::into_raw(json.into_boxed_slice()) as *mut u8;

    unsafe {
        *out_ptr = json_ptr;
        *out_len = json_len;
    }

    crate::ARGUS_OK
}

/// Reset all Rust-side metrics counters to zero.
#[unsafe(no_mangle)]
pub extern "C" fn argus_metrics_reset() {
    FRAMES_CAPTURED.store(0, Ordering::Relaxed);
    RESIZES_TOTAL.store(0, Ordering::Relaxed);
    SHM_WRITES_TOTAL.store(0, Ordering::Relaxed);
    KEYFRAME_DIFFS_TOTAL.store(0, Ordering::Relaxed);
    PII_SCANS_TOTAL.store(0, Ordering::Relaxed);
    CRYPTO_OPS_TOTAL.store(0, Ordering::Relaxed);
}
