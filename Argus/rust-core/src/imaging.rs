//! SIMD-accelerated image resize via `fast_image_resize`.
//!
//! Provides C ABI functions for BGRA image scaling with multiple
//! algorithm choices: Lanczos3, Bilinear, Nearest.


use fast_image_resize as fir;

use crate::{ARGUS_ERR_INTERNAL, ARGUS_ERR_INVALID_PARAM, ARGUS_ERR_NULL_PTR, ARGUS_OK};

/// Algorithm selector constants (maps to `fir::ResizeAlg`).
const ALG_LANCZOS3: i32 = 0;
const ALG_BILINEAR: i32 = 1;
const ALG_NEAREST: i32 = 2;

/// Resize a BGRA image using SIMD-accelerated algorithms.
///
/// # Parameters
/// - `src`: pointer to source BGRA pixel data
/// - `src_w`, `src_h`: source dimensions (pixels)
/// - `src_stride`: source row stride in bytes (0 = src_w * 4)
/// - `dst_w`, `dst_h`: target dimensions (pixels)
/// - `algorithm`: 0=Lanczos3, 1=Bilinear, 2=Nearest
/// - `out_pixels`: receives pointer to resized BGRA data
/// - `out_len`: receives length of output buffer
///
/// # Safety
/// All pointers must be valid. Caller must free `*out_pixels` via
/// `argus_free_buffer(*out_pixels, *out_len)`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_image_resize(
    src: *const u8,
    src_w: i32,
    src_h: i32,
    src_stride: i32,
    dst_w: i32,
    dst_h: i32,
    algorithm: i32,
    out_pixels: *mut *mut u8,
    out_len: *mut usize,
) -> i32 {
    // --- null checks ---
    if src.is_null() || out_pixels.is_null() || out_len.is_null() {
        return ARGUS_ERR_NULL_PTR;
    }
    if src_w <= 0 || src_h <= 0 || dst_w <= 0 || dst_h <= 0 {
        return ARGUS_ERR_INVALID_PARAM;
    }

    let sw = src_w as u32;
    let sh = src_h as u32;
    let dw = dst_w as u32;
    let dh = dst_h as u32;
    let stride = if src_stride <= 0 { sw * 4 } else { src_stride as u32 };

    // Select resize algorithm.
    let alg = match algorithm {
        ALG_LANCZOS3 => fir::ResizeAlg::Convolution(fir::FilterType::Lanczos3),
        ALG_BILINEAR => fir::ResizeAlg::Convolution(fir::FilterType::Bilinear),
        ALG_NEAREST => fir::ResizeAlg::Nearest,
        _ => return ARGUS_ERR_INVALID_PARAM,
    };

    // --- copy source, handling stride ---
    let src_buf = if stride == sw * 4 {
        // SAFETY: caller guarantees src points to valid BGRA data.
        unsafe { std::slice::from_raw_parts(src, (sh * stride) as usize) }.to_vec()
    } else {
        // Row-by-row copy stripping padding.
        let row_bytes = (sw * 4) as usize;
        let mut buf = Vec::with_capacity((sw * sh * 4) as usize);
        for y in 0..sh as usize {
            let offset = y * stride as usize;
            // SAFETY: caller guarantees src has at least src_h rows of stride bytes.
            let row = unsafe { std::slice::from_raw_parts(src.add(offset), row_bytes) };
            buf.extend_from_slice(row);
        }
        buf
    };

    // BGRA → RGBA swap (channels 0 and 2).
    let mut rgba_buf = src_buf;
    for chunk in rgba_buf.chunks_exact_mut(4) {
        chunk.swap(0, 2);
    }

    // Wrap as fir::Image.
    let src_image = match fir::images::Image::from_vec_u8(
        sw,
        sh,
        rgba_buf,
        fir::PixelType::U8x4,
    ) {
        Ok(img) => img,
        Err(_) => return ARGUS_ERR_INTERNAL,
    };

    // Destination buffer.
    let mut dst_image = fir::images::Image::new(dw, dh, fir::PixelType::U8x4);

    // Resize with options.
    let mut resizer = fir::Resizer::new();
    let options = fir::ResizeOptions::new().resize_alg(alg);
    if resizer.resize(&src_image, &mut dst_image, &options).is_err() {
        return ARGUS_ERR_INTERNAL;
    }

    // RGBA → BGRA swap back.
    let mut result = dst_image.into_vec();
    for chunk in result.chunks_exact_mut(4) {
        chunk.swap(0, 2);
    }

    let len = result.len();
    let ptr = result.as_mut_ptr();
    std::mem::forget(result);

    // SAFETY: out_pixels and out_len checked non-null above.
    unsafe {
        *out_pixels = ptr;
        *out_len = len;
    }
    crate::metrics::inc_resizes();
    ARGUS_OK
}

/// Calculate target dimensions that fit within `max_dim` on the longest
/// edge while preserving aspect ratio.
///
/// # Safety
/// `out_w` and `out_h` must be non-null.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_image_calc_fit_size(
    src_w: i32,
    src_h: i32,
    max_dim: i32,
    out_w: *mut i32,
    out_h: *mut i32,
) -> i32 {
    if out_w.is_null() || out_h.is_null() {
        return ARGUS_ERR_NULL_PTR;
    }
    if src_w <= 0 || src_h <= 0 || max_dim <= 0 {
        return ARGUS_ERR_INVALID_PARAM;
    }

    // Already fits.
    if src_w <= max_dim && src_h <= max_dim {
        // SAFETY: checked non-null above.
        unsafe {
            *out_w = src_w;
            *out_h = src_h;
        }
        return ARGUS_OK;
    }

    let (nw, nh) = if src_w >= src_h {
        (max_dim, src_h * max_dim / src_w)
    } else {
        (src_w * max_dim / src_h, max_dim)
    };

    // SAFETY: checked non-null above.
    unsafe {
        *out_w = if nw < 1 { 1 } else { nw };
        *out_h = if nh < 1 { 1 } else { nh };
    }
    ARGUS_OK
}
