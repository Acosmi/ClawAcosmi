//! PII (Personally Identifiable Information) detection and masking.
//!
//! Provides high-performance regex-based PII detection using the `regex` crate's
//! DFA/NFA hybrid engine. Supports 7 entity types with partial/full masking.

use regex::Regex;
use serde::Serialize;
use std::sync::LazyLock;

// ===== Types =====

#[derive(Serialize)]
struct PIIMatch {
    entity_type: String,
    original: String,
    masked: String,
    start: usize,
    end: usize,
    confidence: f64,
}

#[derive(Serialize)]
struct PIIFilterResult {
    original_text: String,
    filtered_text: String,
    matches: Vec<PIIMatch>,
    pii_detected: bool,
}

struct PIIPattern {
    regex: Regex,
    mask_style: &'static str, // "partial" or "full"
    entity_type: &'static str,
    needs_trim: bool, // whether boundary chars need trimming
}

// ===== Compiled Patterns (singleton) =====

static PATTERNS: LazyLock<Vec<PIIPattern>> = LazyLock::new(|| {
    vec![
        PIIPattern {
            regex: Regex::new(r"(?:^|[^\d])\d{17}[\dXx](?:[^\d]|$)").unwrap(),
            mask_style: "partial",
            entity_type: "cn_id_card",
            needs_trim: true,
        },
        PIIPattern {
            regex: Regex::new(r"(?:^|[^\d])1[3-9]\d{9}(?:[^\d]|$)").unwrap(),
            mask_style: "partial",
            entity_type: "cn_phone",
            needs_trim: true,
        },
        PIIPattern {
            regex: Regex::new(r"\+\d{1,3}[-\s]?\d{6,14}").unwrap(),
            mask_style: "partial",
            entity_type: "intl_phone",
            needs_trim: false,
        },
        PIIPattern {
            regex: Regex::new(r"[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}").unwrap(),
            mask_style: "partial",
            entity_type: "email",
            needs_trim: false,
        },
        PIIPattern {
            regex: Regex::new(r"(?:^|[^\d])\d{4}[\s\-]?\d{4}[\s\-]?\d{4}[\s\-]?\d{4}(?:[^\d]|$)").unwrap(),
            mask_style: "partial",
            entity_type: "bank_card",
            needs_trim: true,
        },
        PIIPattern {
            regex: Regex::new(r"(?:^|[^\d])\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(?:[^\d]|$)").unwrap(),
            mask_style: "full",
            entity_type: "ip_address",
            needs_trim: true,
        },
        PIIPattern {
            regex: Regex::new(r"(?:^|[^A-Z])[A-Z]\d{8}(?:[^\d]|$)").unwrap(),
            mask_style: "full",
            entity_type: "passport",
            needs_trim: true,
        },
    ]
});


/// Trim boundary characters that leaked through lookaround emulation.
/// This is UTF-8 safe — it skips entire characters, not single bytes.
fn trim_boundary(s: &str) -> (&str, usize) {
    let bytes = s.as_bytes();
    if bytes.is_empty() {
        return (s, 0);
    }

    let mut start_byte = 0usize;
    let mut end_byte = bytes.len();

    // Check first char: if it's not a digit/plus/upper-letter, skip it
    if let Some(first_char) = s.chars().next()
        && !first_char.is_ascii_digit() && first_char != '+' && !first_char.is_ascii_uppercase()
    {
        start_byte = first_char.len_utf8();
    }

    // Check last char: if it's not a digit/x/upper-letter, skip it
    if s.len() > start_byte
        && let Some(last_char) = s.chars().next_back()
        && !last_char.is_ascii_digit() && last_char != 'X' && last_char != 'x' && !last_char.is_ascii_uppercase()
    {
        end_byte -= last_char.len_utf8();
    }

    if start_byte >= end_byte {
        return (s, 0); // safety: don't produce empty/invalid slice
    }

    (&s[start_byte..end_byte], start_byte)
}

fn mask_text(text: &str, style: &str) -> String {
    let chars: Vec<char> = text.chars().collect();
    if style == "partial" && chars.len() > 6 {
        let prefix: String = chars[..3].iter().collect();
        let suffix: String = chars[chars.len() - 3..].iter().collect();
        let mid = "*".repeat(chars.len() - 6);
        format!("{}{}{}", prefix, mid, suffix)
    } else {
        "*".repeat(chars.len())
    }
}

fn run_pii_filter(text: &str) -> PIIFilterResult {
    let mut matches: Vec<PIIMatch> = Vec::new();

    for pat in PATTERNS.iter() {
        for m in pat.regex.find_iter(text) {
            let raw = m.as_str();
            let (trimmed, trim_offset) = if pat.needs_trim {
                trim_boundary(raw)
            } else {
                (raw, 0)
            };

            let start = m.start() + trim_offset;
            let end = start + trimmed.len();
            let masked = mask_text(trimmed, pat.mask_style);

            matches.push(PIIMatch {
                entity_type: pat.entity_type.to_string(),
                original: trimmed.to_string(),
                masked,
                start,
                end,
                confidence: 1.0,
            });
        }
    }

    // Sort descending by start for safe replacement
    matches.sort_by(|a, b| b.start.cmp(&a.start));

    // Build filtered text using byte-level replacement (safe since all
    // match boundaries align to UTF-8 char boundaries from regex).
    let mut filtered_bytes = text.as_bytes().to_vec();
    for m in &matches {
        let masked_bytes = m.masked.as_bytes();
        let mut result = Vec::with_capacity(filtered_bytes.len() - (m.end - m.start) + masked_bytes.len());
        result.extend_from_slice(&filtered_bytes[..m.start]);
        result.extend_from_slice(masked_bytes);
        result.extend_from_slice(&filtered_bytes[m.end..]);
        filtered_bytes = result;
    }
    let filtered = String::from_utf8(filtered_bytes).unwrap_or_else(|_| text.to_string());

    // Re-sort ascending for output
    matches.sort_by(|a, b| a.start.cmp(&b.start));

    let pii_detected = !matches.is_empty();
    PIIFilterResult {
        original_text: text.to_string(),
        filtered_text: filtered,
        matches,
        pii_detected,
    }
}

fn is_safe(text: &str) -> bool {
    for pat in PATTERNS.iter() {
        if pat.regex.is_match(text) {
            return false;
        }
    }
    true
}

// ===== C ABI Exports =====

/// Filter PII from text. Returns JSON result via out_ptr/out_len.
///
/// # Safety
/// - `text_ptr` must point to valid UTF-8 of length `text_len`.
/// - `out_ptr` and `out_len` must be valid pointers.
/// - Caller must free the output buffer via `argus_free_buffer`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_pii_filter(
    text_ptr: *const u8,
    text_len: usize,
    out_ptr: *mut *mut u8,
    out_len: *mut usize,
) -> i32 {
    if text_ptr.is_null() || out_ptr.is_null() || out_len.is_null() {
        return crate::ARGUS_ERR_NULL_PTR;
    }

    let text = unsafe {
        match std::str::from_utf8(std::slice::from_raw_parts(text_ptr, text_len)) {
            Ok(s) => s,
            Err(_) => return crate::ARGUS_ERR_INVALID_PARAM,
        }
    };

    crate::metrics::inc_pii_scans();

    let result = run_pii_filter(text);
    let json = match serde_json::to_vec(&result) {
        Ok(j) => j,
        Err(_) => return crate::ARGUS_ERR_INTERNAL,
    };

    // into_boxed_slice guarantees capacity == len for argus_free_buffer
    let json_len = json.len();
    let json_ptr = Box::into_raw(json.into_boxed_slice()) as *mut u8;

    unsafe {
        *out_ptr = json_ptr;
        *out_len = json_len;
    }

    crate::ARGUS_OK
}

/// Quick check: returns ARGUS_OK (0) if no PII detected, 1 if PII found.
///
/// # Safety
/// - `text_ptr` must point to valid UTF-8 of length `text_len`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_pii_is_safe(
    text_ptr: *const u8,
    text_len: usize,
) -> i32 {
    if text_ptr.is_null() {
        return crate::ARGUS_ERR_NULL_PTR;
    }

    let text = unsafe {
        match std::str::from_utf8(std::slice::from_raw_parts(text_ptr, text_len)) {
            Ok(s) => s,
            Err(_) => return crate::ARGUS_ERR_INVALID_PARAM,
        }
    };

    if is_safe(text) { 0 } else { 1 }
}
