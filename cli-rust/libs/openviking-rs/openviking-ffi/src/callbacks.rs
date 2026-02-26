// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! C function-pointer → Rust async trait adapters.
//!
//! Go/C callers supply plain function pointers for LLM completion and text
//! embedding. This module wraps them into types that implement
//! [`LlmProvider`](openviking_session::LlmProvider) and
//! [`Embedder`](openviking_session::Embedder), so the rest of the Rust
//! library can use them transparently.

use std::collections::HashMap;

use async_trait::async_trait;
use openviking_session::{BoxError, EmbedResult, Embedder, LlmProvider};

// ---------------------------------------------------------------------------
// C function-pointer types
// ---------------------------------------------------------------------------

/// Synchronous LLM completion callback.
///
/// # Parameters
/// - `prompt` / `prompt_len`: UTF-8 prompt bytes.
/// - `out_buf` / `out_cap`: caller-allocated buffer for the completion text.
///
/// # Returns
/// - Positive value: number of bytes written to `out_buf`.
/// - Negative value: error (message should be set via caller-side logging).
/// - `0`: empty completion (valid).
pub type LlmCompletionFn = unsafe extern "C" fn(
    prompt: *const u8,
    prompt_len: usize,
    out_buf: *mut u8,
    out_cap: usize,
) -> i32;

/// Synchronous text embedding callback.
///
/// # Parameters
/// - `text` / `text_len`: UTF-8 text bytes.
/// - `out_vec`: pre-allocated `f32` buffer of size `out_dim`.
/// - `out_dim`: expected vector dimension.
///
/// # Returns
/// - `0`: success (vector written to `out_vec`).
/// - Negative value: error.
pub type EmbedFn = unsafe extern "C" fn(
    text: *const u8,
    text_len: usize,
    out_vec: *mut f32,
    out_dim: usize,
) -> i32;

// ---------------------------------------------------------------------------
// FfiLlmProvider
// ---------------------------------------------------------------------------

/// Wrapper that implements [`LlmProvider`] by calling a C function pointer.
pub struct FfiLlmProvider {
    cb: LlmCompletionFn,
}

// Safety: The C callback is expected to be thread-safe (Go runtime guarantees this).
unsafe impl Send for FfiLlmProvider {}
unsafe impl Sync for FfiLlmProvider {}

impl FfiLlmProvider {
    /// Create from a C callback.
    pub fn new(cb: LlmCompletionFn) -> Self {
        Self { cb }
    }
}

#[async_trait]
impl LlmProvider for FfiLlmProvider {
    async fn completion(&self, prompt: &str) -> Result<String, BoxError> {
        // Allocate a reasonably large buffer for the response.
        const BUF_CAP: usize = 64 * 1024; // 64 KiB
        let mut buf = vec![0u8; BUF_CAP];

        let n = unsafe {
            (self.cb)(
                prompt.as_ptr(),
                prompt.len(),
                buf.as_mut_ptr(),
                BUF_CAP,
            )
        };

        if n < 0 {
            return Err(format!("LLM callback returned error code {n}").into());
        }

        let len = n as usize;
        if len > BUF_CAP {
            return Err("LLM callback wrote beyond buffer capacity".into());
        }

        buf.truncate(len);
        String::from_utf8(buf).map_err(|e| format!("LLM response is not valid UTF-8: {e}").into())
    }
}

// ---------------------------------------------------------------------------
// FfiEmbedder
// ---------------------------------------------------------------------------

/// Wrapper that implements [`Embedder`] by calling a C function pointer.
pub struct FfiEmbedder {
    cb: EmbedFn,
    dim: usize,
}

// Safety: The C callback is expected to be thread-safe.
unsafe impl Send for FfiEmbedder {}
unsafe impl Sync for FfiEmbedder {}

impl FfiEmbedder {
    /// Create from a C callback and the expected vector dimension.
    pub fn new(cb: EmbedFn, dim: usize) -> Self {
        Self { cb, dim }
    }
}

#[async_trait]
impl Embedder for FfiEmbedder {
    async fn embed(&self, text: &str) -> Result<EmbedResult, BoxError> {
        let mut vec = vec![0.0f32; self.dim];

        let code =
            unsafe { (self.cb)(text.as_ptr(), text.len(), vec.as_mut_ptr(), self.dim) };

        if code < 0 {
            return Err(format!("Embed callback returned error code {code}").into());
        }

        Ok(EmbedResult {
            dense_vector: vec,
            sparse_vector: Some(HashMap::new()),
        })
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    // A trivial LLM callback that echoes the prompt.
    unsafe extern "C" fn echo_llm(
        prompt: *const u8,
        prompt_len: usize,
        out_buf: *mut u8,
        out_cap: usize,
    ) -> i32 {
        let len = prompt_len.min(out_cap);
        unsafe {
            std::ptr::copy_nonoverlapping(prompt, out_buf, len);
        }
        len as i32
    }

    // A trivial embed callback that fills with 1.0.
    unsafe extern "C" fn ones_embed(
        _text: *const u8,
        _text_len: usize,
        out_vec: *mut f32,
        out_dim: usize,
    ) -> i32 {
        for i in 0..out_dim {
            unsafe {
                *out_vec.add(i) = 1.0;
            }
        }
        0
    }

    #[tokio::test]
    async fn ffi_llm_provider_echo() {
        let provider = FfiLlmProvider::new(echo_llm);
        let result = provider.completion("hello").await.unwrap();
        assert_eq!(result, "hello");
    }

    #[tokio::test]
    async fn ffi_embedder_ones() {
        let embedder = FfiEmbedder::new(ones_embed, 4);
        let result = embedder.embed("test").await.unwrap();
        assert_eq!(result.dense_vector, vec![1.0; 4]);
    }
}
