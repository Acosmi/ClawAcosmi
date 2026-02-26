//! nexus-crypto — AES-256-GCM 加解密 + bcrypt 哈希，通过 C ABI 暴露给 Go。
//!
//! 新增加密能力，供记忆加密等场景使用（不替换现有 Fernet）。

use aes_gcm::aead::{Aead, KeyInit, OsRng};
use aes_gcm::{Aes256Gcm, AeadCore, Nonce};
use libc::{c_char, c_int, c_uchar, size_t};
use std::ffi::{CStr, CString};

// ---------------------------------------------------------------------------
// 1) AES-256-GCM 加密
// ---------------------------------------------------------------------------

/// AES-256-GCM 加密。
///
/// - `key`: 32 字节密钥
/// - `plaintext` / `pt_len`: 明文
/// - `out_buf` / `out_cap`: 输出缓冲区（需 >= pt_len + 12 + 16）
/// - `out_len`: 实际写入长度（nonce 12B + ciphertext + tag 16B）
///
/// 返回 0 成功，-1 失败。
#[no_mangle]
pub unsafe extern "C" fn nexus_aes256gcm_encrypt(
    key: *const c_uchar,
    plaintext: *const c_uchar,
    pt_len: size_t,
    out_buf: *mut c_uchar,
    out_cap: size_t,
    out_len: *mut size_t,
) -> c_int {
    if key.is_null() || plaintext.is_null() || out_buf.is_null() || out_len.is_null() {
        return -1;
    }
    let key_slice = unsafe { std::slice::from_raw_parts(key, 32) };
    let pt = unsafe { std::slice::from_raw_parts(plaintext, pt_len) };

    let cipher = match Aes256Gcm::new_from_slice(key_slice) {
        Ok(c) => c,
        Err(_) => return -1,
    };

    let nonce = Aes256Gcm::generate_nonce(&mut OsRng);
    let ciphertext = match cipher.encrypt(&nonce, pt) {
        Ok(ct) => ct,
        Err(_) => return -1,
    };

    let total = 12 + ciphertext.len(); // nonce + ciphertext+tag
    if total > out_cap {
        return -1;
    }

    let out = unsafe { std::slice::from_raw_parts_mut(out_buf, out_cap) };
    out[..12].copy_from_slice(&nonce);
    out[12..total].copy_from_slice(&ciphertext);
    unsafe { *out_len = total };
    0
}

/// AES-256-GCM 解密。
///
/// - `data` / `data_len`: nonce(12B) + ciphertext + tag(16B)
/// - `out_buf` / `out_cap`: 明文输出缓冲区
/// - `out_len`: 实际明文长度
#[no_mangle]
pub unsafe extern "C" fn nexus_aes256gcm_decrypt(
    key: *const c_uchar,
    data: *const c_uchar,
    data_len: size_t,
    out_buf: *mut c_uchar,
    out_cap: size_t,
    out_len: *mut size_t,
) -> c_int {
    if key.is_null() || data.is_null() || data_len < 28 || out_buf.is_null() {
        return -1;
    }
    let key_slice = unsafe { std::slice::from_raw_parts(key, 32) };
    let d = unsafe { std::slice::from_raw_parts(data, data_len) };

    let cipher = match Aes256Gcm::new_from_slice(key_slice) {
        Ok(c) => c,
        Err(_) => return -1,
    };

    let nonce = Nonce::from_slice(&d[..12]);
    let plaintext = match cipher.decrypt(nonce, &d[12..]) {
        Ok(pt) => pt,
        Err(_) => return -1,
    };

    if plaintext.len() > out_cap {
        return -1;
    }
    let out = unsafe { std::slice::from_raw_parts_mut(out_buf, out_cap) };
    out[..plaintext.len()].copy_from_slice(&plaintext);
    unsafe { *out_len = plaintext.len() };
    0
}

// ---------------------------------------------------------------------------
// 2) bcrypt 哈希验证
// ---------------------------------------------------------------------------

/// bcrypt 哈希。返回 C 字符串，调用方用 nexus_crypto_free 释放。
#[no_mangle]
pub unsafe extern "C" fn nexus_bcrypt_hash(
    password: *const c_char,
    cost: c_int,
) -> *mut c_char {
    let pw = unsafe { CStr::from_ptr(password) }.to_str().unwrap_or("");
    let cost = if cost < 4 || cost > 31 { 12 } else { cost as u32 };

    match bcrypt::hash(pw, cost) {
        Ok(h) => CString::new(h).map_or(std::ptr::null_mut(), |c| c.into_raw()),
        Err(_) => std::ptr::null_mut(),
    }
}

/// bcrypt 验证。返回 1 匹配，0 不匹配，-1 错误。
#[no_mangle]
pub unsafe extern "C" fn nexus_bcrypt_verify(
    password: *const c_char,
    hash: *const c_char,
) -> c_int {
    let pw = unsafe { CStr::from_ptr(password) }.to_str().unwrap_or("");
    let h = unsafe { CStr::from_ptr(hash) }.to_str().unwrap_or("");

    match bcrypt::verify(pw, h) {
        Ok(true) => 1,
        Ok(false) => 0,
        Err(_) => -1,
    }
}

/// 释放 nexus_bcrypt_hash 返回的内存。
#[no_mangle]
pub unsafe extern "C" fn nexus_crypto_free(ptr: *mut c_char) {
    if !ptr.is_null() {
        drop(unsafe { CString::from_raw(ptr) });
    }
}

// ---------------------------------------------------------------------------
// Rust 侧单元测试
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_aes_roundtrip() {
        let key = [0x42u8; 32];
        let plaintext = b"hello nexus";
        let mut buf = [0u8; 256];
        let mut enc_len: usize = 0;

        let ret = unsafe {
            nexus_aes256gcm_encrypt(
                key.as_ptr(), plaintext.as_ptr(), plaintext.len(),
                buf.as_mut_ptr(), buf.len(), &mut enc_len,
            )
        };
        assert_eq!(ret, 0);
        assert!(enc_len > 0);

        let mut dec_buf = [0u8; 256];
        let mut dec_len: usize = 0;
        let ret = unsafe {
            nexus_aes256gcm_decrypt(
                key.as_ptr(), buf.as_ptr(), enc_len,
                dec_buf.as_mut_ptr(), dec_buf.len(), &mut dec_len,
            )
        };
        assert_eq!(ret, 0);
        assert_eq!(&dec_buf[..dec_len], plaintext);
    }

    #[test]
    fn test_bcrypt_roundtrip() {
        let pw = CString::new("test_password").unwrap();
        let hash_ptr = unsafe { nexus_bcrypt_hash(pw.as_ptr(), 4) };
        assert!(!hash_ptr.is_null());

        let result = unsafe { nexus_bcrypt_verify(pw.as_ptr(), hash_ptr) };
        assert_eq!(result, 1);

        unsafe { nexus_crypto_free(hash_ptr) };
    }
}
