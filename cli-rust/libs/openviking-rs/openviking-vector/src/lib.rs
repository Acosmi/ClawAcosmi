// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Process-in-memory vector store backed by Qdrant's `segment` library.
//!
//! This crate wraps the Qdrant segment engine to provide a high-level
//! vector store API that can be called in-process via FFI, eliminating
//! the need for a separate Qdrant gRPC server.
//!
//! # Architecture
//!
//! ```text
//! Go (调度层)
//!   └── CGo FFI
//!        └── openviking-ffi
//!             └── openviking-vector (this crate)
//!                  └── segment (Qdrant core engine)
//! ```

pub mod segment_store;

// Re-export commonly needed segment types for FFI layer.
pub use segment::types::{Distance, Payload};

/// Parse a JSON object string into a [`Payload`].
///
/// This helper is primarily for FFI callers who cannot construct
/// `Payload` directly.
pub fn payload_from_json_str(
    json_str: &str,
) -> Result<Payload, Box<dyn std::error::Error + Send + Sync>> {
    let val: serde_json::Value = serde_json::from_str(json_str)?;
    let map = val.as_object().ok_or("payload must be a JSON object")?;
    Ok(Payload::from(map.clone()))
}

#[cfg(test)]
mod tests;
