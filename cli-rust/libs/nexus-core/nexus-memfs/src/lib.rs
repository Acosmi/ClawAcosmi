//! nexus-memfs — Virtual filesystem-based memory store for UHMS.
//!
//! Inspired by ByteDance's OpenViking file system paradigm, this crate provides:
//!
//! - **Virtual Filesystem (VFS)**: Hierarchical directory structure for organizing
//!   permanent memories by category (decisions, facts, emotions, todos).
//! - **L0/L1/L2 Tiered Context**: Three-level content abstraction for reducing
//!   token consumption — abstract (~100 tokens), overview (~2k tokens), full detail.
//! - **Directory Recursive Search**: Keyword-based search with tiered scoring weights.
//! - **C ABI Exports**: Functions exposed via `extern "C"` for Go FFI integration.
//!
//! ## Architecture
//!
//! ```text
//! Go (memory_fs_store.go)
//!   ↓ CGO
//! C ABI (nexus_memfs_*)
//!   ↓
//! MemoryFSStore → MemoryFS → VfsNode (Dir/File)
//!                              ├── TieredContent (L0/L1/L2)
//!                              └── SearchHit / DirEntry
//! ```

pub mod node;
pub mod storage;
pub mod vfs;

use libc::{c_char, c_int, size_t};
use std::ffi::CStr;
use std::sync::OnceLock;

use crate::node::Tier;
use crate::storage::MemoryFSStore;

// ---------------------------------------------------------------------------
// Global store singleton
// ---------------------------------------------------------------------------

static GLOBAL_STORE: OnceLock<MemoryFSStore> = OnceLock::new();

fn get_store() -> Result<&'static MemoryFSStore, c_int> {
    GLOBAL_STORE.get().ok_or(-100) // -100 = not initialized
}

// ---------------------------------------------------------------------------
// Helper: C string ↔ Rust string
// ---------------------------------------------------------------------------

unsafe fn cstr_to_str<'a>(ptr: *const c_char) -> Result<&'a str, c_int> {
    if ptr.is_null() {
        return Err(-1);
    }
    CStr::from_ptr(ptr).to_str().map_err(|_| -2)
}

/// Write a Rust string into a C output buffer.
/// Returns the actual byte length written (excluding null terminator).
fn write_to_buf(s: &str, out_buf: *mut c_char, buf_len: size_t, out_len: *mut size_t) -> c_int {
    if out_buf.is_null() || buf_len == 0 {
        return -1;
    }
    let bytes = s.as_bytes();
    let copy_len = bytes.len().min(buf_len - 1); // reserve 1 for null terminator

    unsafe {
        std::ptr::copy_nonoverlapping(bytes.as_ptr(), out_buf as *mut u8, copy_len);
        *out_buf.add(copy_len) = 0; // null terminator
        if !out_len.is_null() {
            *out_len = copy_len;
        }
    }
    0
}

// ---------------------------------------------------------------------------
// C ABI Exports
// ---------------------------------------------------------------------------

/// Initialize the global MemoryFS store at a given root directory.
///
/// Must be called once before any other `nexus_memfs_*` function.
/// Returns 0 on success, negative on error.
#[no_mangle]
pub unsafe extern "C" fn nexus_memfs_init(root_path: *const c_char) -> c_int {
    let root = match cstr_to_str(root_path) {
        Ok(s) => s,
        Err(e) => return e,
    };

    match MemoryFSStore::new(root) {
        Ok(store) => {
            if GLOBAL_STORE.set(store).is_err() {
                return -3; // already initialized
            }
            0
        }
        Err(_) => -4,
    }
}

/// Write a permanent memory entry to the VFS.
///
/// # Parameters
/// - `tenant_id`, `user_id`: scope identifiers
/// - `memory_id`: unique memory UUID
/// - `category`: one of "decisions", "facts", "emotions", "todos"
/// - `content`: L2 full content
/// - `l0_abstract`: L0 one-sentence summary
/// - `l1_overview`: L1 structured overview
///
/// Returns 0 on success, negative on error.
#[no_mangle]
pub unsafe extern "C" fn nexus_memfs_write_memory(
    tenant_id: *const c_char,
    user_id: *const c_char,
    memory_id: *const c_char,
    category: *const c_char,
    content: *const c_char,
    l0_abstract: *const c_char,
    l1_overview: *const c_char,
) -> c_int {
    let store = match get_store() {
        Ok(s) => s,
        Err(e) => return e,
    };

    let tid = match cstr_to_str(tenant_id) {
        Ok(s) => s,
        Err(e) => return e,
    };
    let uid = match cstr_to_str(user_id) {
        Ok(s) => s,
        Err(e) => return e,
    };
    let mid = match cstr_to_str(memory_id) {
        Ok(s) => s,
        Err(e) => return e,
    };
    let cat = match cstr_to_str(category) {
        Ok(s) => s,
        Err(e) => return e,
    };
    let cnt = match cstr_to_str(content) {
        Ok(s) => s,
        Err(e) => return e,
    };
    let l0 = match cstr_to_str(l0_abstract) {
        Ok(s) => s,
        Err(e) => return e,
    };
    let l1 = match cstr_to_str(l1_overview) {
        Ok(s) => s,
        Err(e) => return e,
    };

    // Parse category: supports "section/category" (e.g. "episodic/dialogues")
    // or plain category name (backward compat → "permanent/{category}")
    let (section, sub_cat) = if let Some(idx) = cat.find('/') {
        (&cat[..idx], &cat[idx + 1..])
    } else {
        ("permanent", cat)
    };

    match store.with_fs(tid, uid, |fs| {
        fs.write_memory_to(section, sub_cat, mid, cnt, l0, l1)
    }) {
        Ok(()) => {
            // Auto-flush after write
            let _ = store.flush(tid, uid);
            0
        }
        Err(_) => -10,
    }
}

/// Read content from the VFS at a specific path and tier level.
///
/// # Parameters
/// - `path`: relative path, e.g. "permanent/decisions/{id}.md"
/// - `level`: 0=L0, 1=L1, 2=L2
/// - `out_buf`: output buffer for the content string
/// - `buf_len`: size of `out_buf`
/// - `out_len`: actual bytes written (excluding null terminator)
///
/// Returns 0 on success, negative on error.
#[no_mangle]
pub unsafe extern "C" fn nexus_memfs_read(
    tenant_id: *const c_char,
    user_id: *const c_char,
    path: *const c_char,
    level: c_int,
    out_buf: *mut c_char,
    buf_len: size_t,
    out_len: *mut size_t,
) -> c_int {
    let store = match get_store() {
        Ok(s) => s,
        Err(e) => return e,
    };

    let tid = match cstr_to_str(tenant_id) {
        Ok(s) => s,
        Err(e) => return e,
    };
    let uid = match cstr_to_str(user_id) {
        Ok(s) => s,
        Err(e) => return e,
    };
    let p = match cstr_to_str(path) {
        Ok(s) => s,
        Err(e) => return e,
    };
    let tier = match Tier::from_int(level) {
        Some(t) => t,
        None => return -5,
    };

    match store.with_fs(tid, uid, |fs| fs.read(p, tier)) {
        Ok(content) => write_to_buf(&content, out_buf, buf_len, out_len),
        Err(_) => -11,
    }
}

/// List entries in a VFS directory. Returns JSON array of `DirEntry`.
///
/// # Parameters
/// - `path`: relative directory path. Use empty string for root.
/// - `out_buf`: output buffer for JSON string
/// - `buf_len`, `out_len`: as above
///
/// Returns 0 on success, negative on error.
#[no_mangle]
pub unsafe extern "C" fn nexus_memfs_list_dir(
    tenant_id: *const c_char,
    user_id: *const c_char,
    path: *const c_char,
    out_buf: *mut c_char,
    buf_len: size_t,
    out_len: *mut size_t,
) -> c_int {
    let store = match get_store() {
        Ok(s) => s,
        Err(e) => return e,
    };

    let tid = match cstr_to_str(tenant_id) {
        Ok(s) => s,
        Err(e) => return e,
    };
    let uid = match cstr_to_str(user_id) {
        Ok(s) => s,
        Err(e) => return e,
    };
    let p = match cstr_to_str(path) {
        Ok(s) => s,
        Err(e) => return e,
    };

    let result = store.with_fs(tid, uid, |fs| {
        let entries = if p.is_empty() {
            fs.list_root()
        } else {
            fs.list_dir(p)?
        };
        serde_json::to_string(&entries).map_err(|e| format!("json: {}", e))
    });

    match result {
        Ok(json) => write_to_buf(&json, out_buf, buf_len, out_len),
        Err(_) => -12,
    }
}

/// Search for memories matching query keywords.
///
/// Returns JSON array of `SearchHit` sorted by relevance score.
///
/// # Parameters
/// - `query`: search keywords (space-separated)
/// - `max_results`: maximum results to return
///
/// Returns 0 on success, negative on error.
#[no_mangle]
pub unsafe extern "C" fn nexus_memfs_search(
    tenant_id: *const c_char,
    user_id: *const c_char,
    query: *const c_char,
    max_results: c_int,
    out_buf: *mut c_char,
    buf_len: size_t,
    out_len: *mut size_t,
) -> c_int {
    let store = match get_store() {
        Ok(s) => s,
        Err(e) => return e,
    };

    let tid = match cstr_to_str(tenant_id) {
        Ok(s) => s,
        Err(e) => return e,
    };
    let uid = match cstr_to_str(user_id) {
        Ok(s) => s,
        Err(e) => return e,
    };
    let q = match cstr_to_str(query) {
        Ok(s) => s,
        Err(e) => return e,
    };

    let max = if max_results < 1 {
        10
    } else {
        max_results as usize
    };

    let result = store.with_fs(tid, uid, |fs| {
        let hits = fs.search(q, max);
        serde_json::to_string(&hits).map_err(|e| format!("json: {}", e))
    });

    match result {
        Ok(json) => write_to_buf(&json, out_buf, buf_len, out_len),
        Err(_) => -13,
    }
}

/// Search with full retrieval trace for visualization.
///
/// Returns JSON of `SearchTrace` containing steps, scores, and hits.
///
/// # Parameters
/// - `query`: search keywords (space-separated)
/// - `max_results`: maximum results to return
///
/// Returns 0 on success, negative on error.
#[no_mangle]
pub unsafe extern "C" fn nexus_memfs_search_with_trace(
    tenant_id: *const c_char,
    user_id: *const c_char,
    query: *const c_char,
    max_results: c_int,
    out_buf: *mut c_char,
    buf_len: size_t,
    out_len: *mut size_t,
) -> c_int {
    let store = match get_store() {
        Ok(s) => s,
        Err(e) => return e,
    };

    let tid = match cstr_to_str(tenant_id) {
        Ok(s) => s,
        Err(e) => return e,
    };
    let uid = match cstr_to_str(user_id) {
        Ok(s) => s,
        Err(e) => return e,
    };
    let q = match cstr_to_str(query) {
        Ok(s) => s,
        Err(e) => return e,
    };

    let max = if max_results < 1 {
        10
    } else {
        max_results as usize
    };

    let result = store.with_fs(tid, uid, |fs| {
        let trace = fs.search_with_trace(q, max);
        serde_json::to_string(&trace).map_err(|e| format!("json: {}", e))
    });

    match result {
        Ok(json) => write_to_buf(&json, out_buf, buf_len, out_len),
        Err(_) => -13,
    }
}

/// Delete a memory by ID from the VFS.
///
/// Returns 0 on success, negative on error.
#[no_mangle]
pub unsafe extern "C" fn nexus_memfs_delete(
    tenant_id: *const c_char,
    user_id: *const c_char,
    memory_id: *const c_char,
) -> c_int {
    let store = match get_store() {
        Ok(s) => s,
        Err(e) => return e,
    };

    let tid = match cstr_to_str(tenant_id) {
        Ok(s) => s,
        Err(e) => return e,
    };
    let uid = match cstr_to_str(user_id) {
        Ok(s) => s,
        Err(e) => return e,
    };
    let mid = match cstr_to_str(memory_id) {
        Ok(s) => s,
        Err(e) => return e,
    };

    match store.with_fs(tid, uid, |fs| fs.delete_memory(mid)) {
        Ok(()) => {
            let _ = store.flush(tid, uid);
            0
        }
        Err(_) => -14,
    }
}

/// Create a session archive directory under `session/archives/archive_{index}/`.
///
/// # Parameters
/// - `tenant_id`, `user_id`: scope identifiers
/// - `index`: archive index (0, 1, 2, ...)
/// - `l0_summary`: one-line summary for L0
/// - `l1_overview`: structured overview for L1
///
/// Returns 0 on success, negative on error.
#[no_mangle]
pub unsafe extern "C" fn nexus_memfs_create_archive_dir(
    tenant_id: *const c_char,
    user_id: *const c_char,
    index: c_int,
    l0_summary: *const c_char,
    l1_overview: *const c_char,
) -> c_int {
    let store = match get_store() {
        Ok(s) => s,
        Err(e) => return e,
    };

    let tid = match cstr_to_str(tenant_id) {
        Ok(s) => s,
        Err(e) => return e,
    };
    let uid = match cstr_to_str(user_id) {
        Ok(s) => s,
        Err(e) => return e,
    };
    let l0 = match cstr_to_str(l0_summary) {
        Ok(s) => s,
        Err(e) => return e,
    };
    let l1 = match cstr_to_str(l1_overview) {
        Ok(s) => s,
        Err(e) => return e,
    };

    match store.with_fs(tid, uid, |fs| fs.create_archive_dir(index as u32, l0, l1)) {
        Ok(()) => {
            let _ = store.flush(tid, uid);
            0
        }
        Err(_) => -17,
    }
}

/// Get the next archive index for a tenant+user.
///
/// Returns the index (>= 0) on success, negative on error.
#[no_mangle]
pub unsafe extern "C" fn nexus_memfs_next_archive_index(
    tenant_id: *const c_char,
    user_id: *const c_char,
) -> c_int {
    let store = match get_store() {
        Ok(s) => s,
        Err(e) => return e,
    };

    let tid = match cstr_to_str(tenant_id) {
        Ok(s) => s,
        Err(e) => return e,
    };
    let uid = match cstr_to_str(user_id) {
        Ok(s) => s,
        Err(e) => return e,
    };

    match store.with_fs(tid, uid, |fs| Ok(fs.next_archive_index())) {
        Ok(idx) => idx as c_int,
        Err(_) => -18,
    }
}

/// Evict a tenant+user filesystem from memory.
///
/// Call this to free memory when a user session ends.
/// Persisted data on disk is not affected.
///
/// Returns 0 on success, negative on error.
#[no_mangle]
pub unsafe extern "C" fn nexus_memfs_cleanup(
    tenant_id: *const c_char,
    user_id: *const c_char,
) -> c_int {
    let store = match get_store() {
        Ok(s) => s,
        Err(e) => return e,
    };

    let tid = match cstr_to_str(tenant_id) {
        Ok(s) => s,
        Err(e) => return e,
    };
    let uid = match cstr_to_str(user_id) {
        Ok(s) => s,
        Err(e) => return e,
    };

    match store.evict(tid, uid) {
        Ok(()) => 0,
        Err(_) => -15,
    }
}

/// Get the total memory count for a tenant+user.
///
/// Returns the count (>= 0) on success, negative on error.
#[no_mangle]
pub unsafe extern "C" fn nexus_memfs_memory_count(
    tenant_id: *const c_char,
    user_id: *const c_char,
) -> c_int {
    let store = match get_store() {
        Ok(s) => s,
        Err(e) => return e,
    };

    let tid = match cstr_to_str(tenant_id) {
        Ok(s) => s,
        Err(e) => return e,
    };
    let uid = match cstr_to_str(user_id) {
        Ok(s) => s,
        Err(e) => return e,
    };

    match store.with_fs(tid, uid, |fs| Ok(fs.memory_count())) {
        Ok(count) => count as c_int,
        Err(_) => -16,
    }
}
