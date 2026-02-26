//! Keyframe extraction helpers: pixel-diff ratio and perceptual hash.
//!
//! Provides C ABI functions used by the Go pipeline to accelerate
//! frame-change detection and deduplication.

use crate::{ARGUS_ERR_INVALID_PARAM, ARGUS_ERR_NULL_PTR, ARGUS_OK};

/// Calculate the pixel change ratio between two BGRA frames.
///
/// A pixel is considered "changed" if any RGB channel differs by more
/// than `threshold` (typically 20). Returns `changed_pixels / total`.
///
/// # Safety
/// `prev` and `curr` must point to at least `height * stride` bytes.
/// `out_ratio` must be non-null.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_keyframe_diff(
    prev: *const u8,
    curr: *const u8,
    width: i32,
    height: i32,
    stride: i32,
    threshold: i32,
    out_ratio: *mut f64,
) -> i32 {
    if prev.is_null() || curr.is_null() || out_ratio.is_null() {
        return ARGUS_ERR_NULL_PTR;
    }
    if width <= 0 || height <= 0 || threshold < 0 {
        return ARGUS_ERR_INVALID_PARAM;
    }

    let w = width as usize;
    let h = height as usize;
    let s = if stride <= 0 { w * 4 } else { stride as usize };
    let thresh = threshold as u8;
    let total_pixels = w * h;

    if total_pixels == 0 {
        // SAFETY: checked non-null above.
        unsafe { *out_ratio = 0.0; }
        return ARGUS_OK;
    }

    let mut changed: usize = 0;

    for y in 0..h {
        let row_off = y * s;
        for x in 0..w {
            let off = row_off + x * 4;
            // SAFETY: caller guarantees buffers are large enough.
            unsafe {
                let dr = abs_diff(*prev.add(off), *curr.add(off));
                let dg = abs_diff(*prev.add(off + 1), *curr.add(off + 1));
                let db = abs_diff(*prev.add(off + 2), *curr.add(off + 2));
                if dr > thresh || dg > thresh || db > thresh {
                    changed += 1;
                }
            }
        }
    }

    let ratio = changed as f64 / total_pixels as f64;
    // SAFETY: checked non-null above.
    unsafe { *out_ratio = ratio; }
    crate::metrics::inc_keyframe_diffs();
    ARGUS_OK
}

/// Compute a 64-bit perceptual hash (dHash) of a BGRA frame.
///
/// Algorithm: downsample to 9×8 grayscale → compare horizontal gradients
/// → 64-bit hash. Two frames with hamming distance < 10 are visually
/// similar.
///
/// # Safety
/// `pixels` must point to at least `height * stride` bytes.
/// `out_hash` must be non-null.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_keyframe_hash(
    pixels: *const u8,
    width: i32,
    height: i32,
    stride: i32,
    out_hash: *mut u64,
) -> i32 {
    if pixels.is_null() || out_hash.is_null() {
        return ARGUS_ERR_NULL_PTR;
    }
    if width <= 0 || height <= 0 {
        return ARGUS_ERR_INVALID_PARAM;
    }

    let w = width as usize;
    let h = height as usize;
    let s = if stride <= 0 { w * 4 } else { stride as usize };

    // Downsample to 9×8 grayscale using nearest-neighbor.
    let dw: usize = 9;
    let dh: usize = 8;
    let mut gray = [0u8; 9 * 8]; // 72 bytes

    for dy in 0..dh {
        let sy = dy * h / dh;
        for dx in 0..dw {
            let sx = dx * w / dw;
            let off = sy * s + sx * 4;
            // SAFETY: caller guarantees buffer is large enough.
            // BGRA layout: B=off, G=off+1, R=off+2
            let (b, g, r) = unsafe {
                (*pixels.add(off), *pixels.add(off + 1), *pixels.add(off + 2))
            };
            // ITU-R BT.601 luma: 0.299R + 0.587G + 0.114B
            let luma = (r as u32 * 299 + g as u32 * 587 + b as u32 * 114) / 1000;
            gray[dy * dw + dx] = luma as u8;
        }
    }

    // dHash: compare each pixel to its right neighbor.
    let mut hash: u64 = 0;
    let mut bit = 0u32;
    for y in 0..dh {
        for x in 0..(dw - 1) {
            if gray[y * dw + x] < gray[y * dw + x + 1] {
                hash |= 1u64 << bit;
            }
            bit += 1;
            if bit >= 64 {
                break;
            }
        }
    }

    // SAFETY: checked non-null above.
    unsafe { *out_hash = hash; }
    ARGUS_OK
}

/// Absolute difference between two bytes.
#[inline(always)]
fn abs_diff(a: u8, b: u8) -> u8 {
    a.abs_diff(b)
}
