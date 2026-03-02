// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! FFI exports for VFS semantic index operations.
//!
//! Provides a dedicated API for the "vfs_semantic" collection, wrapping
//! the generic `SegmentVectorStore` operations with VFS-specific semantics.
//!
//! The collection name is hardcoded to "vfs_semantic" to ensure consistency
//! between Go and Rust sides.

use std::slice;

use openviking_vector::segment_store::{CollectionConfig, SegmentVectorStore};
use openviking_vector::Distance;

use crate::error::{fail, FfiErrorCode};
use crate::session_api::cstr_to_str;

/// Hardcoded collection name for VFS semantic index.
const VFS_SEMANTIC_COLLECTION: &str = "vfs_semantic";

// ---------------------------------------------------------------------------
// Init — create the vfs_semantic collection
// ---------------------------------------------------------------------------

/// Initialize the VFS semantic index collection.
///
/// Creates a "vfs_semantic" collection with the specified embedding dimension.
/// Safe to call multiple times — existing collection is retained.
///
/// # Parameters
/// - `handle`: SegmentVectorStore handle (from `ovk_segment_store_new`).
/// - `dim`: embedding vector dimension.
///
/// # Returns
/// `0` on success, non-zero on error.
///
/// # Safety
/// `handle` must be a valid `SegmentVectorStore` pointer.
#[no_mangle]
pub unsafe extern "C" fn ovk_vfs_semantic_init(handle: *mut SegmentVectorStore, dim: usize) -> i32 {
    if handle.is_null() {
        return fail(FfiErrorCode::NullPointer, "segment store handle is null");
    }

    let store = unsafe { &*handle };
    let cfg = CollectionConfig {
        dimension: dim,
        distance: Distance::Cosine,
        sparse_vectors: false,
        ..Default::default()
    };

    match store.create_collection(VFS_SEMANTIC_COLLECTION, &cfg) {
        Ok(_) => FfiErrorCode::Ok.as_i32(),
        Err(e) => fail(
            FfiErrorCode::VectorStoreError,
            format!("vfs_semantic init: {e}"),
        ),
    }
}

// ---------------------------------------------------------------------------
// Index — upsert a VFS memory into the semantic index
// ---------------------------------------------------------------------------

/// Index a VFS memory entry for semantic search.
///
/// Upserts a point into the "vfs_semantic" collection with the given
/// embedding vector and metadata payload.
///
/// # Parameters
/// - `handle`: SegmentVectorStore handle.
/// - `id` / `id_len`: memory UUID string (UTF-8).
/// - `dense_vec` / `dense_len`: embedding vector.
/// - `payload_json` / `payload_json_len`: JSON metadata payload (UTF-8).
///   Expected keys: "content", "tenant_id", "user_id", "section", "category",
///   "vfs_uri", "memory_id".
///
/// # Returns
/// `0` on success, non-zero on error.
///
/// # Safety
/// All pointers must be valid.
#[no_mangle]
pub unsafe extern "C" fn ovk_vfs_semantic_index(
    handle: *mut SegmentVectorStore,
    id: *const u8,
    id_len: usize,
    dense_vec: *const f32,
    dense_len: usize,
    payload_json: *const u8,
    payload_json_len: usize,
) -> i32 {
    if handle.is_null() {
        return fail(FfiErrorCode::NullPointer, "segment store handle is null");
    }

    let result = (|| -> Result<(), i32> {
        let point_id = unsafe { cstr_to_str(id, id_len)? };
        let store = unsafe { &*handle };

        let vec_slice = if dense_vec.is_null() || dense_len == 0 {
            return Err(fail(
                FfiErrorCode::InvalidUtf8,
                "embedding vector is null or empty",
            ));
        } else {
            unsafe { slice::from_raw_parts(dense_vec, dense_len) }
        };

        let payload = if !payload_json.is_null() && payload_json_len > 0 {
            let json_str = unsafe { cstr_to_str(payload_json, payload_json_len)? };
            Some(
                openviking_vector::payload_from_json_str(json_str)
                    .map_err(|e| fail(FfiErrorCode::Other, format!("payload parse: {e}")))?,
            )
        } else {
            None
        };

        store
            .upsert(
                VFS_SEMANTIC_COLLECTION,
                point_id,
                vec_slice,
                payload.as_ref(),
            )
            .map_err(|e| {
                fail(
                    FfiErrorCode::VectorStoreError,
                    format!("vfs semantic index: {e}"),
                )
            })
    })();

    match result {
        Ok(()) => FfiErrorCode::Ok.as_i32(),
        Err(code) => code,
    }
}

// ---------------------------------------------------------------------------
// Search — semantic search over VFS index
// ---------------------------------------------------------------------------

/// Search the VFS semantic index for nearest neighbours.
///
/// # Parameters
/// - `handle`: SegmentVectorStore handle.
/// - `dense_vec` / `dense_len`: query embedding vector.
/// - `limit`: max results.
/// - `out_json` / `out_cap`: caller-provided buffer for JSON result.
///
/// # Returns
/// - `0` if no results.
/// - Positive: number of bytes written to `out_json`.
/// - Negative error code on failure.
///
/// # Safety
/// All pointers must be valid.
#[no_mangle]
pub unsafe extern "C" fn ovk_vfs_semantic_search(
    handle: *mut SegmentVectorStore,
    dense_vec: *const f32,
    dense_len: usize,
    limit: usize,
    out_json: *mut u8,
    out_cap: usize,
) -> i32 {
    if handle.is_null() {
        return fail(FfiErrorCode::NullPointer, "segment store handle is null");
    }

    let result = (|| -> Result<i32, i32> {
        let store = unsafe { &*handle };
        let vec_slice = if dense_vec.is_null() || dense_len == 0 {
            return Err(fail(
                FfiErrorCode::InvalidUtf8,
                "query vector is null or empty",
            ));
        } else {
            unsafe { slice::from_raw_parts(dense_vec, dense_len) }
        };

        let hits = store
            .search(VFS_SEMANTIC_COLLECTION, vec_slice, None, limit, None)
            .map_err(|e| {
                fail(
                    FfiErrorCode::VectorStoreError,
                    format!("vfs semantic search: {e}"),
                )
            })?;

        if hits.is_empty() {
            return Ok(0);
        }

        let json_bytes = serde_json::to_vec(&hits)
            .map_err(|e| fail(FfiErrorCode::Other, format!("JSON serialise: {e}")))?;

        let needed = json_bytes.len();
        if out_json.is_null() || out_cap < needed {
            return Err(fail(
                FfiErrorCode::BufferTooSmall,
                format!("need {needed} bytes"),
            ));
        }

        unsafe {
            std::ptr::copy_nonoverlapping(json_bytes.as_ptr(), out_json, needed);
        }
        Ok(needed as i32)
    })();

    match result {
        Ok(n) => n,
        Err(code) => code,
    }
}

// ---------------------------------------------------------------------------
// Delete — remove a point from VFS semantic index
// ---------------------------------------------------------------------------

/// Delete a memory entry from the VFS semantic index.
///
/// # Parameters
/// - `handle`: SegmentVectorStore handle.
/// - `id` / `id_len`: memory UUID string (UTF-8).
///
/// # Returns
/// `0` on success, non-zero on error.
///
/// # Safety
/// All pointers must be valid.
#[no_mangle]
pub unsafe extern "C" fn ovk_vfs_semantic_delete(
    handle: *mut SegmentVectorStore,
    id: *const u8,
    id_len: usize,
) -> i32 {
    if handle.is_null() {
        return fail(FfiErrorCode::NullPointer, "segment store handle is null");
    }

    let result = (|| -> Result<(), i32> {
        let point_id = unsafe { cstr_to_str(id, id_len)? };
        let store = unsafe { &*handle };

        store
            .delete(VFS_SEMANTIC_COLLECTION, point_id)
            .map_err(|e| {
                fail(
                    FfiErrorCode::VectorStoreError,
                    format!("vfs semantic delete: {e}"),
                )
            })?;
        Ok(())
    })();

    match result {
        Ok(()) => FfiErrorCode::Ok.as_i32(),
        Err(code) => code,
    }
}
