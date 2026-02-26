// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! FFI exports for BPE token counting.
//!
//! Uses tiktoken-rs cl100k_base encoding (compatible with GPT-4/Claude).
//! O(n) complexity, 10x faster than HuggingFace tokenizers.
//!
//! # Thread Safety
//!
//! The tokenizer is initialized once via `LazyLock` and shared across threads.
//! All functions are safe to call concurrently.

use std::sync::LazyLock;

use tiktoken_rs::CoreBPE;

use crate::error::{fail, FfiErrorCode};

/// Global tokenizer instance (cl100k_base encoding).
/// LazyLock ensures one-time initialization on first use.
///
// SAFETY: cl100k_base BPE data is embedded at compile time by tiktoken-rs.
// `expect()` is infallible here — the only failure mode would be a bug in
// the tiktoken-rs crate itself (corrupt embedded data), not runtime conditions.
static TOKENIZER: LazyLock<CoreBPE> = LazyLock::new(|| {
    tiktoken_rs::cl100k_base().expect("failed to load cl100k_base BPE data")
});

/// Count tokens in a UTF-8 text string using cl100k_base BPE encoding.
///
/// # Parameters
/// - `text` / `text_len`: UTF-8 text bytes.
///
/// # Returns
/// - `>= 0`: token count on success.
/// - `-1`: invalid UTF-8 input.
///
/// # Safety
/// `text` must point to `text_len` valid bytes.
#[no_mangle]
pub unsafe extern "C" fn ovk_token_count(text: *const u8, text_len: usize) -> i32 {
    if text.is_null() || text_len == 0 {
        return 0;
    }

    let slice = unsafe { std::slice::from_raw_parts(text, text_len) };
    match std::str::from_utf8(slice) {
        Ok(s) => {
            let tokens = TOKENIZER.encode_ordinary(s);
            // Saturate at i32::MAX to avoid overflow for extreme inputs (>6GB text).
            // Go caller treats negative as error, so clamping is safer than wrapping.
            i32::try_from(tokens.len()).unwrap_or(i32::MAX)
        }
        Err(_) => {
            fail(FfiErrorCode::InvalidArgument, "ovk_token_count: invalid UTF-8");
            -1
        }
    }
}

/// Truncate text to fit within max_tokens, respecting UTF-8 char boundaries.
///
/// # Parameters
/// - `text` / `text_len`: UTF-8 text bytes.
/// - `max_tokens`: maximum number of BPE tokens to keep.
/// - `out_byte_len`: receives the byte length of the truncated text.
///
/// # Returns
/// - `0`: success. `*out_byte_len` contains the truncated byte length.
/// - `-1`: error (null pointers or invalid UTF-8).
///
/// The truncated text is the input `text[..out_byte_len]` — always valid UTF-8.
///
/// # Safety
/// `text` must point to `text_len` valid bytes. `out_byte_len` must be a valid pointer.
#[no_mangle]
pub unsafe extern "C" fn ovk_token_truncate(
    text: *const u8,
    text_len: usize,
    max_tokens: usize,
    out_byte_len: *mut usize,
) -> i32 {
    if text.is_null() || out_byte_len.is_null() {
        fail(FfiErrorCode::InvalidArgument, "ovk_token_truncate: null pointer");
        return -1;
    }

    if text_len == 0 || max_tokens == 0 {
        unsafe { *out_byte_len = 0 };
        return 0;
    }

    let slice = unsafe { std::slice::from_raw_parts(text, text_len) };
    let s = match std::str::from_utf8(slice) {
        Ok(s) => s,
        Err(_) => {
            fail(FfiErrorCode::InvalidArgument, "ovk_token_truncate: invalid UTF-8");
            return -1;
        }
    };

    let tokens = TOKENIZER.encode_ordinary(s);
    if tokens.len() <= max_tokens {
        // No truncation needed
        unsafe { *out_byte_len = text_len };
        return 0;
    }

    // Decode the first max_tokens tokens back to text to find the byte boundary
    let truncated_tokens = &tokens[..max_tokens];
    let truncated_text = TOKENIZER.decode(truncated_tokens.to_vec());
    match truncated_text {
        Ok(decoded) => {
            unsafe { *out_byte_len = decoded.len() };
            0
        }
        Err(_) => {
            // Fallback: linear scan by char boundaries to find max prefix with <= max_tokens.
            // Binary search is unsafe here because find_char_boundary can map
            // different byte offsets to the same boundary, causing non-convergence.
            let mut last_good = 0usize;
            for (i, _) in s.char_indices() {
                if i == 0 {
                    continue;
                }
                let count = TOKENIZER.encode_ordinary(&s[..i]).len();
                if count > max_tokens {
                    break;
                }
                last_good = i;
            }
            unsafe { *out_byte_len = last_good };
            0
        }
    }
}

/// Find the nearest valid UTF-8 char boundary at or before `pos`.
fn find_char_boundary(s: &str, pos: usize) -> usize {
    if pos >= s.len() {
        return s.len();
    }
    let mut p = pos;
    while p > 0 && !s.is_char_boundary(p) {
        p -= 1;
    }
    p
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_token_count_basic() {
        let text = b"Hello world";
        let count = unsafe { ovk_token_count(text.as_ptr(), text.len()) };
        assert_eq!(count, 2, "\"Hello world\" should be 2 tokens");
    }

    #[test]
    fn test_token_count_chinese() {
        let text = "你好世界".as_bytes();
        let count = unsafe { ovk_token_count(text.as_ptr(), text.len()) };
        assert!(count > 0, "Chinese text should have positive token count");
    }

    #[test]
    fn test_token_count_empty() {
        let count = unsafe { ovk_token_count(std::ptr::null(), 0) };
        assert_eq!(count, 0);
    }

    #[test]
    fn test_token_truncate_no_truncation() {
        let text = b"Hello";
        let mut out_len: usize = 0;
        let rc = unsafe {
            ovk_token_truncate(text.as_ptr(), text.len(), 100, &mut out_len)
        };
        assert_eq!(rc, 0);
        assert_eq!(out_len, text.len());
    }

    #[test]
    fn test_token_truncate_actual() {
        let text = b"Hello world, how are you doing today?";
        let mut out_len: usize = 0;
        let rc = unsafe {
            ovk_token_truncate(text.as_ptr(), text.len(), 2, &mut out_len)
        };
        assert_eq!(rc, 0);
        assert!(out_len < text.len(), "should truncate");
        assert!(out_len > 0, "should keep something");
        // Verify valid UTF-8
        let truncated = std::str::from_utf8(&text[..out_len]);
        assert!(truncated.is_ok());
    }
}
