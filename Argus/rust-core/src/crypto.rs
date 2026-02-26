//! AES-256-GCM authenticated encryption / decryption.
//!
//! Uses the `aes-gcm` crate for AEAD operations with 96-bit nonces.

use aes_gcm::{
    Aes256Gcm, Nonce,
    aead::{Aead, KeyInit, OsRng},
    AeadCore,
};

// ===== C ABI Exports =====

/// Encrypt plaintext using AES-256-GCM.
///
/// - `key_ptr`: 32-byte AES-256 key.
/// - `plaintext_ptr`, `plaintext_len`: data to encrypt.
/// - `nonce_out`: pointer to receive 12-byte nonce (caller must `argus_free_buffer`).
/// - `nonce_out_len`: receives 12.
/// - `cipher_out`: pointer to receive ciphertext+tag (caller must `argus_free_buffer`).
/// - `cipher_out_len`: receives ciphertext length.
///
/// # Safety
/// All pointers must be valid. Key must be exactly 32 bytes.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_aes_encrypt(
    key_ptr: *const u8,
    plaintext_ptr: *const u8,
    plaintext_len: usize,
    nonce_out: *mut *mut u8,
    nonce_out_len: *mut usize,
    cipher_out: *mut *mut u8,
    cipher_out_len: *mut usize,
) -> i32 {
    if key_ptr.is_null()
        || plaintext_ptr.is_null()
        || nonce_out.is_null()
        || nonce_out_len.is_null()
        || cipher_out.is_null()
        || cipher_out_len.is_null()
    {
        return crate::ARGUS_ERR_NULL_PTR;
    }

    let key_bytes = unsafe { std::slice::from_raw_parts(key_ptr, 32) };
    let plaintext = unsafe { std::slice::from_raw_parts(plaintext_ptr, plaintext_len) };

    let cipher = match Aes256Gcm::new_from_slice(key_bytes) {
        Ok(c) => c,
        Err(_) => return crate::ARGUS_ERR_INVALID_PARAM,
    };

    let nonce = Aes256Gcm::generate_nonce(&mut OsRng);

    let ciphertext = match cipher.encrypt(&nonce, plaintext) {
        Ok(ct) => ct,
        Err(_) => return crate::ARGUS_ERR_INTERNAL,
    };

    crate::metrics::inc_crypto_ops();

    // Output nonce (12 bytes)
    let nonce_vec: Vec<u8> = nonce.to_vec();
    let nonce_len = nonce_vec.len();
    let nonce_ptr = Box::into_raw(nonce_vec.into_boxed_slice()) as *mut u8;
    unsafe {
        *nonce_out = nonce_ptr;
        *nonce_out_len = nonce_len;
    }

    // Output ciphertext
    let ct_len = ciphertext.len();
    let ct_ptr = Box::into_raw(ciphertext.into_boxed_slice()) as *mut u8;
    unsafe {
        *cipher_out = ct_ptr;
        *cipher_out_len = ct_len;
    }

    crate::ARGUS_OK
}

/// Decrypt ciphertext using AES-256-GCM.
///
/// - `key_ptr`: 32-byte AES-256 key.
/// - `nonce_ptr`: 12-byte nonce from encryption.
/// - `ciphertext_ptr`, `ciphertext_len`: encrypted data (includes 16-byte GCM tag).
/// - `plain_out`, `plain_out_len`: receives decrypted plaintext.
///
/// # Safety
/// All pointers must be valid.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_aes_decrypt(
    key_ptr: *const u8,
    nonce_ptr: *const u8,
    ciphertext_ptr: *const u8,
    ciphertext_len: usize,
    plain_out: *mut *mut u8,
    plain_out_len: *mut usize,
) -> i32 {
    if key_ptr.is_null()
        || nonce_ptr.is_null()
        || ciphertext_ptr.is_null()
        || plain_out.is_null()
        || plain_out_len.is_null()
    {
        return crate::ARGUS_ERR_NULL_PTR;
    }

    let key_bytes = unsafe { std::slice::from_raw_parts(key_ptr, 32) };
    let nonce_bytes = unsafe { std::slice::from_raw_parts(nonce_ptr, 12) };
    let ciphertext = unsafe { std::slice::from_raw_parts(ciphertext_ptr, ciphertext_len) };

    let cipher = match Aes256Gcm::new_from_slice(key_bytes) {
        Ok(c) => c,
        Err(_) => return crate::ARGUS_ERR_INVALID_PARAM,
    };

    let nonce = Nonce::from_slice(nonce_bytes);

    let plaintext = match cipher.decrypt(nonce, ciphertext) {
        Ok(pt) => pt,
        Err(_) => return crate::ARGUS_ERR_INTERNAL,
    };

    crate::metrics::inc_crypto_ops();

    let pt_len = plaintext.len();
    let pt_ptr = Box::into_raw(plaintext.into_boxed_slice()) as *mut u8;
    unsafe {
        *plain_out = pt_ptr;
        *plain_out_len = pt_len;
    }

    crate::ARGUS_OK
}
