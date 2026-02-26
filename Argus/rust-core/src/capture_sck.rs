//! ScreenCaptureKit capture module.
//!
//! Provides C ABI functions for event-driven screen capture using Apple's
//! ScreenCaptureKit framework (macOS 12.3+). Frame data is delivered via
//! callback instead of polling, offering lower latency and CPU usage
//! compared to the legacy CoreGraphics capture path.

use screencapturekit::cv::CVPixelBufferLockFlags;
use screencapturekit::prelude::*;
use std::sync::{Arc, Mutex, OnceLock};

use crate::{ARGUS_ERR_INTERNAL, ARGUS_ERR_NULL_PTR, ARGUS_OK};

// ─── Global State ────────────────────────────────────────────────

/// Shared state for the latest captured frame.
struct SckState {
    // Display metadata
    display_width: i32,
    display_height: i32,
    scale_factor: f64,
    display_id: u32,
    refresh_rate_hz: i32,

    // Latest frame data
    pixels: Option<Vec<u8>>,
    frame_width: i32,
    frame_height: i32,
    frame_bytes_per_row: i32,
    frame_no: u64,
    has_new_frame: bool,
}

impl SckState {
    fn new() -> Self {
        Self {
            display_width: 0,
            display_height: 0,
            scale_factor: 2.0,
            display_id: 0,
            refresh_rate_hz: 60,
            pixels: None,
            frame_width: 0,
            frame_height: 0,
            frame_bytes_per_row: 0,
            frame_no: 0,
            has_new_frame: false,
        }
    }
}

static SCK_STATE: OnceLock<Arc<Mutex<SckState>>> = OnceLock::new();

fn get_state() -> Arc<Mutex<SckState>> {
    SCK_STATE
        .get_or_init(|| Arc::new(Mutex::new(SckState::new())))
        .clone()
}

// Keep the stream alive in a global so it isn't dropped.
static SCK_STREAM: OnceLock<Mutex<Option<SCStream>>> = OnceLock::new();

fn get_stream_slot() -> &'static Mutex<Option<SCStream>> {
    SCK_STREAM.get_or_init(|| Mutex::new(None))
}

// ─── Frame Handler ───────────────────────────────────────────────

/// Receives frames from ScreenCaptureKit and caches the latest one.
struct FrameHandler {
    state: Arc<Mutex<SckState>>,
}

impl SCStreamOutputTrait for FrameHandler {
    fn did_output_sample_buffer(
        &self,
        sample: CMSampleBuffer,
        _of_type: SCStreamOutputType,
    ) {
        // Extract pixel buffer from the sample
        let Some(pixel_buffer) = sample.image_buffer() else {
            return;
        };

        // Lock pixel buffer for CPU read access via guard pattern
        let Ok(guard) = pixel_buffer.lock(CVPixelBufferLockFlags::READ_ONLY)
        else {
            return;
        };

        let width = guard.width() as i32;
        let height = guard.height() as i32;
        let bytes_per_row = guard.bytes_per_row() as i32;

        // Direct slice access — zero-copy read from locked buffer
        let slice = guard.as_slice();
        if !slice.is_empty() {
            let pixel_data = slice.to_vec();

            if let Ok(mut state) = self.state.lock() {
                state.pixels = Some(pixel_data);
                state.frame_width = width;
                state.frame_height = height;
                state.frame_bytes_per_row = bytes_per_row;
                state.frame_no += 1;
                state.has_new_frame = true;
            }
        }
        // guard dropped → pixel buffer auto-unlocked
    }
}

// ─── C ABI Exports ───────────────────────────────────────────────

/// Discover available displays and cache metadata.
/// Returns 0 on success, negative on error.
#[unsafe(no_mangle)]
pub extern "C" fn argus_sck_discover(display_index: i32) -> i32 {
    let content = match SCShareableContent::get() {
        Ok(c) => c,
        Err(_) => return -1,
    };

    let displays = content.displays();
    if displays.is_empty() {
        return -1;
    }

    let idx = if (display_index as usize) < displays.len() {
        display_index as usize
    } else {
        0
    };
    let display = &displays[idx];

    let state = get_state();
    if let Ok(mut s) = state.lock() {
        s.display_width = display.width() as i32;
        s.display_height = display.height() as i32;
        s.display_id = display.display_id();
        // Default values — actual scale/refresh can be refined later
        s.scale_factor = 2.0;
        s.refresh_rate_hz = 60;
    } else {
        return ARGUS_ERR_INTERNAL;
    }

    ARGUS_OK
}

/// Start the SCK capture stream.
/// Must be called after `argus_sck_discover`.
/// fps ∈ [1, 60].
#[unsafe(no_mangle)]
pub extern "C" fn argus_sck_start_stream(
    fps: i32,
    show_cursor: i32,
) -> i32 {
    let state = get_state();
    let (width, height) = {
        let s = match state.lock() {
            Ok(s) => s,
            Err(_) => return ARGUS_ERR_INTERNAL,
        };
        let w = (s.display_width as f64 * s.scale_factor) as u32;
        let h = (s.display_height as f64 * s.scale_factor) as u32;
        (w, h)
    };

    if width == 0 || height == 0 {
        return -1; // discover not called
    }

    // Get display for filter
    let content = match SCShareableContent::get() {
        Ok(c) => c,
        Err(_) => return -2,
    };
    let displays = content.displays();
    if displays.is_empty() {
        return -2;
    }

    let display_id = state.lock().map(|s| s.display_id).unwrap_or(0);
    let display = displays
        .iter()
        .find(|d| d.display_id() == display_id)
        .unwrap_or(&displays[0]);

    // Build content filter (full display, no exclusions for now)
    let filter = SCContentFilter::create()
        .with_display(display)
        .with_excluding_windows(&[])
        .build();

    // Build stream configuration
    let mut config = SCStreamConfiguration::new()
        .with_width(width)
        .with_height(height)
        .with_pixel_format(PixelFormat::BGRA)
        .with_minimum_frame_interval(&CMTime {
            value: 1,
            timescale: fps,
            flags: 0,
            epoch: 0,
        })
        .with_queue_depth(3);

    if show_cursor != 0 {
        config = config.with_shows_cursor(true);
    } else {
        config = config.with_shows_cursor(false);
    }

    // Create stream and add handler
    let mut stream = SCStream::new(&filter, &config);

    let handler = FrameHandler {
        state: state.clone(),
    };
    stream.add_output_handler(handler, SCStreamOutputType::Screen);

    // Start capture
    if stream.start_capture().is_err() {
        return -3;
    }

    // Store stream globally to keep it alive
    let slot = get_stream_slot();
    if let Ok(mut s) = slot.lock() {
        *s = Some(stream);
    }

    ARGUS_OK
}

/// Stop the SCK capture stream and release resources.
#[unsafe(no_mangle)]
pub extern "C" fn argus_sck_stop_stream() -> i32 {
    let slot = get_stream_slot();
    if let Ok(mut s) = slot.lock() {
        if let Some(ref stream) = *s {
            let _ = stream.stop_capture();
        }
        *s = None;
    }
    ARGUS_OK
}

/// Get the latest captured frame.
///
/// Returns 1 if a new frame is available, 0 otherwise.
/// Caller must free *out_pixels via `argus_free_buffer`.
///
/// # Safety
/// All output pointers must be valid and non-null.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_sck_get_frame(
    out_pixels: *mut *mut u8,
    out_width: *mut i32,
    out_height: *mut i32,
    out_bytes_per_row: *mut i32,
    out_frame_no: *mut u64,
) -> i32 {
    if out_pixels.is_null()
        || out_width.is_null()
        || out_height.is_null()
        || out_bytes_per_row.is_null()
        || out_frame_no.is_null()
    {
        return ARGUS_ERR_NULL_PTR;
    }

    let state = get_state();
    let mut s = match state.lock() {
        Ok(s) => s,
        Err(_) => return ARGUS_ERR_INTERNAL,
    };

    if !s.has_new_frame {
        return 0; // no new frame
    }

    let pixels = match s.pixels.as_ref() {
        Some(p) => p,
        None => return 0,
    };

    // Clone into a Vec — caller frees via argus_free_buffer(ptr, len)
    let mut cloned = pixels.clone();
    let ptr = cloned.as_mut_ptr();
    std::mem::forget(cloned);
    unsafe {
        *out_pixels = ptr;
        *out_width = s.frame_width;
        *out_height = s.frame_height;
        *out_bytes_per_row = s.frame_bytes_per_row;
        *out_frame_no = s.frame_no;
    }

    s.has_new_frame = false;
    crate::metrics::inc_frames_captured();
    1 // new frame available
}

/// Get display info populated by `argus_sck_discover`.
///
/// # Safety
/// All output pointers must be valid and non-null.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_sck_display_info(
    out_width: *mut i32,
    out_height: *mut i32,
    out_scale: *mut f64,
    out_id: *mut u32,
    out_refresh_hz: *mut i32,
) -> i32 {
    if out_width.is_null()
        || out_height.is_null()
        || out_scale.is_null()
        || out_id.is_null()
        || out_refresh_hz.is_null()
    {
        return ARGUS_ERR_NULL_PTR;
    }

    let state = get_state();
    let s = match state.lock() {
        Ok(s) => s,
        Err(_) => return ARGUS_ERR_INTERNAL,
    };

    unsafe {
        *out_width = s.display_width;
        *out_height = s.display_height;
        *out_scale = s.scale_factor;
        *out_id = s.display_id;
        *out_refresh_hz = s.refresh_rate_hz;
    }

    ARGUS_OK
}

/// Check if Screen Recording permission is granted.
/// Returns 1 if granted, 0 if not.
///
/// Uses `SCShareableContent::get()` as a proxy check — it will fail
/// without permission.
#[unsafe(no_mangle)]
pub extern "C" fn argus_sck_has_permission() -> i32 {
    match SCShareableContent::get() {
        Ok(c) => {
            if c.displays().is_empty() {
                0
            } else {
                1
            }
        }
        Err(_) => 0,
    }
}
