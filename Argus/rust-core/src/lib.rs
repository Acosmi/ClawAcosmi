//! Argus-Core: Rust core library for Argus-Compound.
//!
//! Provides C ABI exports for:
//! - Screen capture (CoreGraphics / SCK)
//! - Input injection (CGEvent) [Phase 1 Batch B]
//! - Image processing (SIMD) [Phase 2]
//! - SHM IPC [Phase 3]

pub mod capture;
pub mod capture_sck;
pub mod input;
pub mod imaging;
pub mod keyframe;
pub mod shm;
pub mod pii;
pub mod crypto;
pub mod metrics;
pub mod accessibility;

// ===== Error Codes =====
pub const ARGUS_OK: i32 = 0;
pub const ARGUS_ERR_NULL_PTR: i32 = -1;
pub const ARGUS_ERR_INVALID_PARAM: i32 = -2;
pub const ARGUS_ERR_INTERNAL: i32 = -3;
pub const ARGUS_ERR_UNAVAILABLE: i32 = -4;
pub const ARGUS_ERR_OOM: i32 = -5;

// ===== Memory Management =====

/// Free a buffer allocated by Rust.
///
/// # Safety
/// `ptr` must have been allocated by Rust via `Vec::into_raw_parts` or
/// equivalent. `len` must be the original allocation length. Passing
/// invalid pointers or wrong lengths is undefined behaviour.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_free_buffer(ptr: *mut u8, len: usize) {
    if ptr.is_null() {
        return;
    }
    // SAFETY: caller guarantees ptr was allocated by Vec with the given len.
    unsafe {
        let _ = Vec::from_raw_parts(ptr, len, len);
    }
}
