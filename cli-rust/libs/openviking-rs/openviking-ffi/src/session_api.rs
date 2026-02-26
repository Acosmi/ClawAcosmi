// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! FFI exports for [`Session`] lifecycle management.
//!
//! Provides opaque-handle based API for creating sessions, adding messages,
//! committing, and freeing resources.

use openviking_core::message::{Message, Role};
use openviking_session::session::Session;
use openviking_vfs::LocalFs;

use crate::callbacks::{FfiLlmProvider, LlmCompletionFn};
use crate::error::{fail, FfiErrorCode};
use crate::runtime::block_on_ffi;

// ---------------------------------------------------------------------------
// Type alias for the concrete Session used through FFI
// ---------------------------------------------------------------------------

/// Concrete session type exposed through FFI handles.
pub type FfiSession = Session<LocalFs, FfiLlmProvider>;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/// Convert a C string pointer + length to a `&str`, returning an error code
/// on null or invalid UTF-8.
pub(crate) unsafe fn cstr_to_str<'a>(ptr: *const u8, len: usize) -> Result<&'a str, i32> {
    if ptr.is_null() {
        return Err(fail(FfiErrorCode::NullPointer, "string pointer is null"));
    }
    let slice = unsafe { std::slice::from_raw_parts(ptr, len) };
    std::str::from_utf8(slice)
        .map_err(|e| fail(FfiErrorCode::InvalidUtf8, format!("invalid UTF-8: {e}")))
}

// ---------------------------------------------------------------------------
// ovk_session_new
// ---------------------------------------------------------------------------

/// Create a new [`Session`] backed by a local file system at `root_path`.
///
/// # Parameters
/// - `root_path` / `root_len`: UTF-8 path to the local storage root.
/// - `user_id` / `user_id_len`: user identifier string.
/// - `session_id` / `session_id_len`: session identifier string.
/// - `llm_cb`: LLM completion callback (may be called synchronously).
///
/// # Returns
/// Opaque `*mut FfiSession` handle on success, `null` on failure
/// (check [`ovk_last_error`](crate::error::ovk_last_error)).
///
/// # Safety
/// All pointer arguments must be valid for their declared lengths.
#[no_mangle]
pub unsafe extern "C" fn ovk_session_new(
    root_path: *const u8,
    root_len: usize,
    user_id: *const u8,
    user_id_len: usize,
    session_id: *const u8,
    session_id_len: usize,
    llm_cb: LlmCompletionFn,
) -> *mut FfiSession {
    let result = (|| -> Result<*mut FfiSession, i32> {
        let root = unsafe { cstr_to_str(root_path, root_len)? };
        let uid = unsafe { cstr_to_str(user_id, user_id_len)? };
        let sid = unsafe { cstr_to_str(session_id, session_id_len)? };

        let fs = LocalFs::new(root);
        let llm = FfiLlmProvider::new(llm_cb);

        let user = openviking_core::user::UserIdentifier {
            account_id: uid.to_string(),
            user_id: uid.to_string(),
            agent_id: String::new(),
            language: None,
        };

        let session = Session::new(fs, llm, user, sid.to_string());

        Ok(Box::into_raw(Box::new(session)))
    })();

    match result {
        Ok(ptr) => ptr,
        Err(_) => std::ptr::null_mut(), // error already recorded
    }
}

// ---------------------------------------------------------------------------
// ovk_session_add_message
// ---------------------------------------------------------------------------

/// Add a message to the session.
///
/// # Parameters
/// - `handle`: session handle from [`ovk_session_new`].
/// - `role`: `0` = User, `1` = Assistant.
/// - `content` / `content_len`: UTF-8 message content.
///
/// # Returns
/// `0` on success, non-zero error code on failure.
///
/// # Safety
/// `handle` must be a valid pointer from `ovk_session_new`.
#[no_mangle]
pub unsafe extern "C" fn ovk_session_add_message(
    handle: *mut FfiSession,
    role: i32,
    content: *const u8,
    content_len: usize,
) -> i32 {
    if handle.is_null() {
        return fail(FfiErrorCode::NullPointer, "session handle is null");
    }

    let result = (|| -> Result<(), i32> {
        let text = unsafe { cstr_to_str(content, content_len)? };

        let role = match role {
            0 => Role::User,
            1 => Role::Assistant,
            _ => return Err(fail(FfiErrorCode::Other, format!("unknown role: {role}"))),
        };

        let msg = Message::create_user(text);
        // Override role if assistant.
        let msg = if role == Role::Assistant {
            Message::create_assistant(Some(text.to_owned()), vec![], vec![])
        } else {
            msg
        };

        let session = unsafe { &mut *handle };
        block_on_ffi(async {
            session.add_message(msg).await.map_err(|e| {
                fail(FfiErrorCode::SessionError, format!("add_message failed: {e}"))
            })?;
            Ok(())
        })
    })();

    match result {
        Ok(()) => FfiErrorCode::Ok.as_i32(),
        Err(code) => code,
    }
}

// ---------------------------------------------------------------------------
// ovk_session_commit
// ---------------------------------------------------------------------------

/// Commit the session (archive messages, write AGFS, update stats).
///
/// The JSON-serialised commit result is written to the output buffer.
///
/// # Parameters
/// - `handle`: session handle.
/// - `out_json` / `out_cap`: caller-allocated buffer for the JSON result.
///
/// # Returns
/// - Positive value: bytes written to `out_json`.
/// - `0`: commit succeeded but no JSON output (empty result).
/// - Negative value: error code (check `ovk_last_error`).
///
/// # Safety
/// `handle` must be a valid session pointer. `out_json` must be writeable.
#[no_mangle]
pub unsafe extern "C" fn ovk_session_commit(
    handle: *mut FfiSession,
    out_json: *mut u8,
    out_cap: usize,
) -> i32 {
    if handle.is_null() {
        return fail(FfiErrorCode::NullPointer, "session handle is null");
    }

    let session = unsafe { &mut *handle };

    let result = block_on_ffi(async {
        let cr = session.commit().await.map_err(|e| {
            fail(FfiErrorCode::SessionError, format!("commit failed: {e}"))
        })?;

        let json = serde_json::to_string(&serde_json::json!({
            "session_id": cr.session_id,
            "status": cr.status,
            "memories_extracted": cr.memories_extracted,
            "archived": cr.archived,
        }))
        .map_err(|e| fail(FfiErrorCode::Other, format!("JSON serialise: {e}")))?;

        if !out_json.is_null() && out_cap > 0 {
            let write_len = json.len().min(out_cap);
            unsafe {
                std::ptr::copy_nonoverlapping(json.as_ptr(), out_json, write_len);
            }
            Ok(write_len as i32)
        } else {
            Ok(0i32)
        }
    });

    match result {
        Ok(n) => n,
        Err(code) => code,
    }
}

// ---------------------------------------------------------------------------
// ovk_session_free
// ---------------------------------------------------------------------------

/// Free a session handle.
///
/// # Safety
/// `handle` must have been returned by [`ovk_session_new`] and must not be
/// used after this call.
#[no_mangle]
pub unsafe extern "C" fn ovk_session_free(handle: *mut FfiSession) {
    if !handle.is_null() {
        unsafe {
            drop(Box::from_raw(handle));
        }
    }
}
