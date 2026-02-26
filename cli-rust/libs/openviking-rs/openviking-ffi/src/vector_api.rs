// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! FFI exports for in-process vector store operations.
//!
//! All functions follow the same conventions as `vfs_api.rs` and `session_api.rs`:
//! - Return `i32` error codes (0 = success).
//! - Thread-local error message via `ovk_last_error`.
//! - Caller owns buffers, Rust writes into them.

use std::slice;

use openviking_vector::segment_store::{CollectionConfig, SegmentVectorStore};
use openviking_vector::Distance;

use crate::error::{fail, FfiErrorCode};
use crate::session_api::cstr_to_str;

// ---------------------------------------------------------------------------
// SegmentVectorStore handle
// ---------------------------------------------------------------------------

/// Create a new [`SegmentVectorStore`].
///
/// # Parameters
/// - `data_dir` / `data_dir_len`: UTF-8 path to data root directory.
///
/// # Returns
/// Opaque `*mut SegmentVectorStore` on success, `null` on failure.
///
/// # Safety
/// `data_dir` must be valid UTF-8 bytes.
#[no_mangle]
pub unsafe extern "C" fn ovk_segment_store_new(
    data_dir: *const u8,
    data_dir_len: usize,
) -> *mut SegmentVectorStore {
    let result = (|| -> Result<*mut SegmentVectorStore, i32> {
        let dir = unsafe { cstr_to_str(data_dir, data_dir_len)? };
        let store = SegmentVectorStore::new(dir)
            .map_err(|e| fail(FfiErrorCode::IoError, format!("segment store init: {e}")))?;
        Ok(Box::into_raw(Box::new(store)))
    })();

    match result {
        Ok(ptr) => ptr,
        Err(_) => std::ptr::null_mut(),
    }
}

/// Free a [`SegmentVectorStore`] handle.
///
/// # Safety
/// `handle` must have been returned by [`ovk_segment_store_new`].
#[no_mangle]
pub unsafe extern "C" fn ovk_segment_store_free(handle: *mut SegmentVectorStore) {
    if !handle.is_null() {
        unsafe { drop(Box::from_raw(handle)) };
    }
}

// ---------------------------------------------------------------------------
// Collection management
// ---------------------------------------------------------------------------

/// Create a collection in the segment vector store.
///
/// # Parameters
/// - `handle`: SegmentVectorStore handle.
/// - `name` / `name_len`: collection name (UTF-8).
/// - `dim`: embedding vector dimension.
///
/// # Returns
/// `0` on success (created or already exists), non-zero on error.
///
/// # Safety
/// All pointers must be valid.
#[no_mangle]
pub unsafe extern "C" fn ovk_segment_create_collection(
    handle: *mut SegmentVectorStore,
    name: *const u8,
    name_len: usize,
    dim: usize,
) -> i32 {
    if handle.is_null() {
        return fail(FfiErrorCode::NullPointer, "segment store handle is null");
    }

    let result = (|| -> Result<(), i32> {
        let col_name = unsafe { cstr_to_str(name, name_len)? };
        let store = unsafe { &*handle };

        let cfg = CollectionConfig {
            dimension: dim,
            distance: Distance::Cosine,
            sparse_vectors: false,
        };

        store.create_collection(col_name, &cfg).map_err(|e| {
            fail(
                FfiErrorCode::VectorStoreError,
                format!("create collection: {e}"),
            )
        })?;

        Ok(())
    })();

    match result {
        Ok(()) => FfiErrorCode::Ok.as_i32(),
        Err(code) => code,
    }
}

// ---------------------------------------------------------------------------
// Upsert
// ---------------------------------------------------------------------------

/// Upsert a point into a collection.
///
/// # Parameters
/// - `handle`: SegmentVectorStore handle.
/// - `col` / `col_len`: collection name (UTF-8).
/// - `id` / `id_len`: point UUID string (UTF-8).
/// - `dense_vec`: pointer to dense vector float data.
/// - `dense_len`: number of float elements in dense vector.
/// - `payload_json` / `payload_json_len`: optional JSON payload (UTF-8). Pass null/0 for no payload.
///
/// # Returns
/// `0` on success, non-zero on error.
///
/// # Safety
/// All pointers must be valid.
#[no_mangle]
pub unsafe extern "C" fn ovk_segment_upsert(
    handle: *mut SegmentVectorStore,
    col: *const u8,
    col_len: usize,
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
        let col_name = unsafe { cstr_to_str(col, col_len)? };
        let point_id = unsafe { cstr_to_str(id, id_len)? };
        let store = unsafe { &*handle };

        let vec_slice = if dense_vec.is_null() || dense_len == 0 {
            return Err(fail(
                FfiErrorCode::InvalidUtf8,
                "dense vector is null or empty",
            ));
        } else {
            unsafe { slice::from_raw_parts(dense_vec, dense_len) }
        };

        // Parse optional payload.
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
            .upsert(col_name, point_id, vec_slice, payload.as_ref())
            .map_err(|e| fail(FfiErrorCode::VectorStoreError, format!("upsert: {e}")))
    })();

    match result {
        Ok(()) => FfiErrorCode::Ok.as_i32(),
        Err(code) => code,
    }
}

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

/// Search for nearest neighbours.
///
/// # Parameters
/// - `handle`: SegmentVectorStore handle.
/// - `col` / `col_len`: collection name (UTF-8).
/// - `dense_vec` / `dense_len`: query dense vector.
/// - `limit`: max results.
/// - `out_json` / `out_cap`: caller-provided buffer for JSON result.
///
/// # Returns
/// - `0` if no results.
/// - Positive: number of bytes written to `out_json`.
/// - Negative: required buffer size (absolute value) when `out_cap` is too small.
///
/// # Safety
/// All pointers must be valid.
#[no_mangle]
pub unsafe extern "C" fn ovk_segment_search(
    handle: *mut SegmentVectorStore,
    col: *const u8,
    col_len: usize,
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
        let col_name = unsafe { cstr_to_str(col, col_len)? };
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
            .search(col_name, vec_slice, None, limit)
            .map_err(|e| fail(FfiErrorCode::VectorStoreError, format!("search: {e}")))?;

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
// Delete
// ---------------------------------------------------------------------------

/// Delete a point from a collection.
///
/// # Parameters
/// - `handle`: SegmentVectorStore handle.
/// - `col` / `col_len`: collection name (UTF-8).
/// - `id` / `id_len`: point UUID string (UTF-8).
///
/// # Returns
/// `0` on success, non-zero on error.
///
/// # Safety
/// All pointers must be valid.
#[no_mangle]
pub unsafe extern "C" fn ovk_segment_delete(
    handle: *mut SegmentVectorStore,
    col: *const u8,
    col_len: usize,
    id: *const u8,
    id_len: usize,
) -> i32 {
    if handle.is_null() {
        return fail(FfiErrorCode::NullPointer, "segment store handle is null");
    }

    let result = (|| -> Result<(), i32> {
        let col_name = unsafe { cstr_to_str(col, col_len)? };
        let point_id = unsafe { cstr_to_str(id, id_len)? };
        let store = unsafe { &*handle };

        store
            .delete(col_name, point_id)
            .map_err(|e| fail(FfiErrorCode::VectorStoreError, format!("delete: {e}")))?;
        Ok(())
    })();

    match result {
        Ok(()) => FfiErrorCode::Ok.as_i32(),
        Err(code) => code,
    }
}

// ---------------------------------------------------------------------------
// Flush
// ---------------------------------------------------------------------------

/// Flush a collection's segment data to disk.
///
/// # Parameters
/// - `handle`: SegmentVectorStore handle.
/// - `col` / `col_len`: collection name (UTF-8).
///
/// # Returns
/// `0` on success, non-zero on error.
///
/// # Safety
/// All pointers must be valid.
#[no_mangle]
pub unsafe extern "C" fn ovk_segment_flush(
    handle: *mut SegmentVectorStore,
    col: *const u8,
    col_len: usize,
) -> i32 {
    if handle.is_null() {
        return fail(FfiErrorCode::NullPointer, "segment store handle is null");
    }

    let result = (|| -> Result<(), i32> {
        let col_name = unsafe { cstr_to_str(col, col_len)? };
        let store = unsafe { &*handle };

        store
            .flush(col_name)
            .map_err(|e| fail(FfiErrorCode::VectorStoreError, format!("flush: {e}")))
    })();

    match result {
        Ok(()) => FfiErrorCode::Ok.as_i32(),
        Err(code) => code,
    }
}
