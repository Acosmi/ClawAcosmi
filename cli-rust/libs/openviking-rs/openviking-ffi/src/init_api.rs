// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! FFI exports for collection initialization and directory setup.
//!
//! **Resolves deferred item D-A3-1**: The Go caller passes `collection_name`
//! and `vector_dim` as plain parameters — no config file / env-var reading
//! happens on the Rust side.

use openviking_session::collection_schemas::init_context_collection;
use openviking_session::InMemoryVectorStore;
use openviking_vfs::{DirectoryInitializer, LocalFs, VikingFs};

use crate::callbacks::{EmbedFn, FfiEmbedder};
use crate::error::{fail, FfiErrorCode};
use crate::runtime::block_on_ffi;
use crate::session_api::cstr_to_str;

// ---------------------------------------------------------------------------
// InMemoryVectorStore handle
// ---------------------------------------------------------------------------

/// Create a new [`InMemoryVectorStore`].
///
/// # Returns
/// Opaque `*mut InMemoryVectorStore` on success, `null` on failure.
#[no_mangle]
pub extern "C" fn ovk_vector_store_new() -> *mut InMemoryVectorStore {
    Box::into_raw(Box::new(InMemoryVectorStore::new()))
}

/// Free an [`InMemoryVectorStore`] handle.
///
/// # Safety
/// `handle` must have been returned by [`ovk_vector_store_new`].
#[no_mangle]
pub unsafe extern "C" fn ovk_vector_store_free(handle: *mut InMemoryVectorStore) {
    if !handle.is_null() {
        unsafe {
            drop(Box::from_raw(handle));
        }
    }
}

// ---------------------------------------------------------------------------
// ovk_init_context_collection
// ---------------------------------------------------------------------------

/// Initialise the context collection in the vector store.
///
/// This is the C-ABI entry point for
/// [`init_context_collection`](openviking_session::collection_schemas::init_context_collection).
///
/// # Parameters
/// - `vs`: `InMemoryVectorStore` handle.
/// - `name` / `name_len`: collection name (UTF-8).
/// - `vector_dim`: embedding vector dimension.
///
/// # Returns
/// - `0`: success (collection created or already existed).
/// - Non-zero: error code (check `ovk_last_error`).
///
/// # Safety
/// All pointers must be valid.
#[no_mangle]
pub unsafe extern "C" fn ovk_init_context_collection(
    vs: *mut InMemoryVectorStore,
    name: *const u8,
    name_len: usize,
    vector_dim: usize,
) -> i32 {
    if vs.is_null() {
        return fail(FfiErrorCode::NullPointer, "vector store handle is null");
    }

    let result = (|| -> Result<(), i32> {
        let col_name = unsafe { cstr_to_str(name, name_len)? };
        let vs_ref = unsafe { &*vs };

        block_on_ffi(async {
            init_context_collection(vs_ref, col_name, vector_dim)
                .await
                .map(|_created| ())
                .map_err(|e| {
                    let msg = e.to_string();
                    if msg.contains("collection not found") {
                        fail(FfiErrorCode::CollectionNotFound, msg)
                    } else {
                        fail(FfiErrorCode::VectorStoreError, msg)
                    }
                })
        })
    })();

    match result {
        Ok(()) => FfiErrorCode::Ok.as_i32(),
        Err(code) => code,
    }
}

// ---------------------------------------------------------------------------
// VikingFs handle (composed of LocalFs + VectorStore + Embedder)
// ---------------------------------------------------------------------------

/// Opaque handle for the fully-composed VikingFs.
pub type FfiVikingFs = VikingFs<LocalFs, InMemoryVectorStore, FfiEmbedder>;

/// Create a fully configured [`VikingFs`] (file system + vector store + embedder).
///
/// # Parameters
/// - `root_path` / `root_len`: UTF-8 path to local storage root.
/// - `vs`: `InMemoryVectorStore` handle (ownership moves into VikingFs).
/// - `embed_cb`: C embedding callback.
/// - `embed_dim`: embedding vector dimension.
///
/// # Returns
/// Opaque `*mut FfiVikingFs` handle on success, `null` on failure.
///
/// # Safety
/// All pointers must be valid. `vs` handle is consumed and must not be
/// used or freed separately after this call.
#[no_mangle]
pub unsafe extern "C" fn ovk_vikingfs_new(
    root_path: *const u8,
    root_len: usize,
    vs: *mut InMemoryVectorStore,
    embed_cb: EmbedFn,
    embed_dim: usize,
) -> *mut FfiVikingFs {
    let result = (|| -> Result<*mut FfiVikingFs, i32> {
        let root = unsafe { cstr_to_str(root_path, root_len)? };
        if vs.is_null() {
            return Err(fail(FfiErrorCode::NullPointer, "vector store is null"));
        }
        let vs_owned = unsafe { *Box::from_raw(vs) };
        let fs = LocalFs::new(root);
        let embedder = FfiEmbedder::new(embed_cb, embed_dim);
        let vfs = VikingFs::with_backends(fs, vs_owned, embedder);
        Ok(Box::into_raw(Box::new(vfs)))
    })();

    match result {
        Ok(ptr) => ptr,
        Err(_) => std::ptr::null_mut(),
    }
}

/// Free a [`VikingFs`] handle.
///
/// # Safety
/// `handle` must have been returned by [`ovk_vikingfs_new`].
#[no_mangle]
pub unsafe extern "C" fn ovk_vikingfs_free(handle: *mut FfiVikingFs) {
    if !handle.is_null() {
        unsafe {
            drop(Box::from_raw(handle));
        }
    }
}

// ---------------------------------------------------------------------------
// ovk_directory_init
// ---------------------------------------------------------------------------

/// Initialise the preset directory structure (global scope).
///
/// # Parameters
/// - `vfs`: `FfiVikingFs` handle.
/// - `col_name` / `col_name_len`: collection name (UTF-8).
///
/// # Returns
/// `0` on success, non-zero error code on failure.
///
/// # Safety
/// All pointers must be valid.
#[no_mangle]
pub unsafe extern "C" fn ovk_directory_init(
    vfs: *mut FfiVikingFs,
    col_name: *const u8,
    col_name_len: usize,
) -> i32 {
    if vfs.is_null() {
        return fail(FfiErrorCode::NullPointer, "vfs handle is null");
    }

    let result = (|| -> Result<(), i32> {
        let name = unsafe { cstr_to_str(col_name, col_name_len)? };
        let vfs_ref = unsafe { &*vfs };

        block_on_ffi(async {
            let init = DirectoryInitializer::new(vfs_ref, name);
            init.initialize_all().await.map(|_| ()).map_err(|e| {
                fail(FfiErrorCode::IoError, format!("directory init failed: {e}"))
            })
        })
    })();

    match result {
        Ok(()) => FfiErrorCode::Ok.as_i32(),
        Err(code) => code,
    }
}
