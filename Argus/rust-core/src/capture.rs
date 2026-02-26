//! Screen capture module using CoreGraphics.
//!
//! Provides C ABI functions for capturing the screen on macOS.
//! Uses `CGDisplay::image()` for broad compatibility.
//! Future: migrate to ScreenCaptureKit for hardware-accelerated capture.

use core_foundation::data::CFData;
use core_graphics::display::CGDisplay;

use crate::{
    ARGUS_ERR_INTERNAL, ARGUS_ERR_NULL_PTR, ARGUS_ERR_UNAVAILABLE, ARGUS_OK,
};

/// Capture a single frame from the main display using CoreGraphics.
///
/// On success, writes BGRA pixel data to `*out_pixels`, dimensions to
/// `*out_width` / `*out_height`, and bytes-per-row to `*out_stride`.
/// The caller **must** free `*out_pixels` via `argus_free_buffer` with
/// the length `(*out_stride) * (*out_height)`.
///
/// # Safety
/// All output pointers must be non-null and properly aligned.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_capture_frame(
    out_pixels: *mut *mut u8,
    out_width: *mut i32,
    out_height: *mut i32,
    out_stride: *mut i32,
) -> i32 {
    // Null-pointer checks
    if out_pixels.is_null()
        || out_width.is_null()
        || out_height.is_null()
        || out_stride.is_null()
    {
        return ARGUS_ERR_NULL_PTR;
    }

    // Capture the full main display
    let display = CGDisplay::main();
    let image = match display.image() {
        Some(img) => img,
        None => return ARGUS_ERR_UNAVAILABLE,
    };

    let width = image.width() as i32;
    let height = image.height() as i32;
    let bpr = image.bytes_per_row() as i32;

    // Extract raw pixel data from the CGImage via CFData
    let cf_data: CFData = image.data();
    let slice = cf_data.bytes();
    let expected_len = (bpr as usize) * (height as usize);

    if slice.len() < expected_len {
        return ARGUS_ERR_INTERNAL;
    }

    // Copy to a Rust-owned Vec and transfer ownership to caller
    let mut pixels = Vec::with_capacity(expected_len);
    pixels.extend_from_slice(&slice[..expected_len]);
    let ptr = pixels.as_mut_ptr();
    std::mem::forget(pixels); // caller frees via argus_free_buffer

    // SAFETY: all pointers verified non-null above
    unsafe {
        *out_pixels = ptr;
        *out_width = width;
        *out_height = height;
        *out_stride = bpr;
    }

    crate::metrics::inc_frames_captured();
    ARGUS_OK
}

/// Get main display information.
///
/// # Safety
/// All output pointers must be non-null and properly aligned.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_capture_display_info(
    out_width: *mut i32,
    out_height: *mut i32,
    out_display_id: *mut u32,
) -> i32 {
    if out_width.is_null() || out_height.is_null() || out_display_id.is_null()
    {
        return ARGUS_ERR_NULL_PTR;
    }

    let display = CGDisplay::main();
    let bounds = display.bounds();

    // SAFETY: pointers verified non-null above
    unsafe {
        *out_width = bounds.size.width as i32;
        *out_height = bounds.size.height as i32;
        *out_display_id = display.id;
    }

    ARGUS_OK
}
