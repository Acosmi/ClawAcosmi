#ifndef ARGUS_CORE_H
#define ARGUS_CORE_H

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

// ===== Error Codes =====
#define ARGUS_OK 0
#define ARGUS_ERR_NULL_PTR -1
#define ARGUS_ERR_INVALID -2
#define ARGUS_ERR_INTERNAL -3
#define ARGUS_ERR_UNAVAIL -4
#define ARGUS_ERR_OOM -5

// ===== Screen Capture =====

// Capture a single frame from the main display.
// On success, *out_pixels receives BGRA pixel data.
// Caller must free *out_pixels via argus_free_buffer(out_pixels, out_stride *
// out_height).
int32_t argus_capture_frame(uint8_t **out_pixels, int32_t *out_width,
                            int32_t *out_height, int32_t *out_stride);

// Get main display information.
int32_t argus_capture_display_info(int32_t *out_width, int32_t *out_height,
                                   uint32_t *out_display_id);

// ===== Input Injection =====

// Move mouse cursor to (x, y).
int32_t argus_input_mouse_move(int32_t x, int32_t y);

// Click at (x, y). button: 0=left, 1=right, 2=middle.
int32_t argus_input_click(int32_t x, int32_t y, int32_t button);

// Double-click at (x, y) with left button.
int32_t argus_input_double_click(int32_t x, int32_t y);

// Scroll at (x, y) by (delta_x, delta_y) pixels.
int32_t argus_input_scroll(int32_t x, int32_t y, int32_t delta_x,
                           int32_t delta_y);

// Press a key down (key_code is macOS CGKeyCode).
int32_t argus_input_key_down(uint16_t key_code);

// Release a key.
int32_t argus_input_key_up(uint16_t key_code);

// Press and release a key.
int32_t argus_input_key_press(uint16_t key_code);

// Type a single Unicode character.
int32_t argus_input_type_char(uint32_t char_code);

// Get current mouse position.
int32_t argus_input_get_mouse_pos(int32_t *out_x, int32_t *out_y);

// ===== ScreenCaptureKit Capture (macOS 12.3+) =====

// Discover displays. display_index 0 = main.
int32_t argus_sck_discover(int32_t display_index);

// Start SCK capture stream. fps ∈ [1, 60]. show_cursor: 0 or 1.
int32_t argus_sck_start_stream(int32_t fps, int32_t show_cursor);

// Stop SCK capture stream.
int32_t argus_sck_stop_stream(void);

// Get latest SCK frame. Returns 1 if new frame, 0 if none.
// Caller must free *out_pixels via argus_free_buffer(out_pixels,
// out_bytes_per_row * out_height).
int32_t argus_sck_get_frame(uint8_t **out_pixels, int32_t *out_width,
                            int32_t *out_height, int32_t *out_bytes_per_row,
                            uint64_t *out_frame_no);

// Get SCK display info populated by argus_sck_discover.
int32_t argus_sck_display_info(int32_t *out_width, int32_t *out_height,
                               double *out_scale, uint32_t *out_id,
                               int32_t *out_refresh_hz);

// Check Screen Recording permission. Returns 1 if granted, 0 if not.
int32_t argus_sck_has_permission(void);

// ===== Image Processing (Phase 2) =====

// Resize a BGRA image using SIMD-accelerated algorithms.
// algorithm: 0=Lanczos3, 1=Bilinear, 2=Nearest.
// Caller must free *out_pixels via argus_free_buffer(*out_pixels, *out_len).
int32_t argus_image_resize(const uint8_t *src, int32_t src_w, int32_t src_h,
                           int32_t src_stride, int32_t dst_w, int32_t dst_h,
                           int32_t algorithm, uint8_t **out_pixels,
                           size_t *out_len);

// Calculate target dimensions that fit within max_dim on the longest edge.
int32_t argus_image_calc_fit_size(int32_t src_w, int32_t src_h, int32_t max_dim,
                                  int32_t *out_w, int32_t *out_h);

// ===== Keyframe Extraction (Phase 2) =====

// Calculate pixel change ratio between two BGRA frames.
// threshold: per-channel diff threshold (default 20).
int32_t argus_keyframe_diff(const uint8_t *prev, const uint8_t *curr,
                            int32_t width, int32_t height, int32_t stride,
                            int32_t threshold, double *out_ratio);

// Compute a 64-bit perceptual hash (dHash) of a BGRA frame.
int32_t argus_keyframe_hash(const uint8_t *pixels, int32_t width,
                            int32_t height, int32_t stride, uint64_t *out_hash);

// ===== SHM IPC (Phase 3) =====

// Opaque SHM writer handle.
typedef struct ShmWriter ShmWriter;

// Create a shared memory writer for max frame dimensions.
// shm_name, sem_writer, sem_reader: POSIX names (must start with '/').
// On success, *out_handle receives the writer handle.
int32_t argus_shm_create(const char *shm_name, const char *sem_writer,
                         const char *sem_reader, int32_t max_width,
                         int32_t max_height, int32_t channels,
                         ShmWriter **out_handle);

// Write a frame to shared memory. Blocks until reader consumes previous frame.
int32_t argus_shm_write_frame(ShmWriter *handle, int32_t width, int32_t height,
                              int32_t channels, const uint8_t *pixels,
                              size_t pixels_len);

// Destroy the SHM writer and release all resources.
void argus_shm_destroy(ShmWriter *handle);

// Get the current frame number.
int32_t argus_shm_frame_number(const ShmWriter *handle, uint64_t *out_frame_no);

// Cleanup residual SHM and semaphore resources (e.g. after crash).
void argus_shm_cleanup(const char *shm_name, const char *sem_writer,
                       const char *sem_reader);

// Simulate a reader consuming a frame (for testing).
int32_t argus_shm_simulate_reader(ShmWriter *handle);

// ===== PII Filter =====

// Filter PII from text. Returns JSON result via out_ptr/out_len.
// Caller must free the output buffer via argus_free_buffer.
int32_t argus_pii_filter(const uint8_t *text_ptr, size_t text_len,
                         uint8_t **out_ptr, size_t *out_len);

// Quick check: returns 0 if no PII detected, 1 if PII found.
int32_t argus_pii_is_safe(const uint8_t *text_ptr, size_t text_len);

// ===== Crypto (AES-256-GCM) =====

// Encrypt plaintext. Caller must free nonce_out and cipher_out via
// argus_free_buffer.
int32_t argus_aes_encrypt(const uint8_t *key_ptr, const uint8_t *plaintext_ptr,
                          size_t plaintext_len, uint8_t **nonce_out,
                          size_t *nonce_out_len, uint8_t **cipher_out,
                          size_t *cipher_out_len);

// Decrypt ciphertext. Caller must free plain_out via argus_free_buffer.
int32_t argus_aes_decrypt(const uint8_t *key_ptr, const uint8_t *nonce_ptr,
                          const uint8_t *ciphertext_ptr, size_t ciphertext_len,
                          uint8_t **plain_out, size_t *plain_out_len);

// ===== Metrics =====

// Export Rust-side metrics as JSON. Caller must free via argus_free_buffer.
int32_t argus_metrics_get(uint8_t **out_ptr, size_t *out_len);

// Reset all Rust-side metrics counters.
void argus_metrics_reset(void);

// ===== Accessibility (macOS AXUIElement) =====

// List all UI elements for a given process ID.
// Returns JSON array via out_json/out_len.
// Caller must free *out_json via argus_free_buffer(*out_json, *out_len).
int32_t argus_ax_list_elements(int32_t pid, uint8_t **out_json,
                               size_t *out_len);

// Get UI element at screen position (x, y).
// Returns JSON array via out_json/out_len.
// Caller must free *out_json via argus_free_buffer(*out_json, *out_len).
int32_t argus_ax_element_at_position(float x, float y, uint8_t **out_json,
                                     size_t *out_len);

// Get all UI elements of the currently focused application.
// Returns JSON array via out_json/out_len.
// Caller must free *out_json via argus_free_buffer(*out_json, *out_len).
int32_t argus_ax_focused_app(uint8_t **out_json, size_t *out_len);

// Check macOS permissions (Accessibility + Screen Recording).
// Returns JSON: {"accessibility": bool, "screen_recording": bool}
// Caller must free *out_json via argus_free_buffer(*out_json, *out_len).
int32_t argus_check_permissions(uint8_t **out_json, size_t *out_len);

// Requests Screen Recording permission explicitly.
// Returns true if granted, false if not (triggers dialog).
bool argus_request_screen_capture(void);

// ===== Memory Management =====

// Free a buffer allocated by Rust.
// ptr must have been returned by a Rust function. len is the buffer size.
void argus_free_buffer(uint8_t *ptr, size_t len);

#endif // ARGUS_CORE_H
