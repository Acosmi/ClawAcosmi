// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Math utility functions — pure computation, zero IO.
//!
//! Ported from `openviking/session/memory_deduplicator.py`.

/// Calculate cosine similarity between two vectors.
///
/// Returns 0.0 if either vector is zero-length or all-zeros.
#[must_use]
pub fn cosine_similarity(a: &[f32], b: &[f32]) -> f32 {
    if a.len() != b.len() || a.is_empty() {
        return 0.0;
    }
    let mut dot = 0.0_f64;
    let mut norm_a = 0.0_f64;
    let mut norm_b = 0.0_f64;
    for (x, y) in a.iter().zip(b.iter()) {
        let x = f64::from(*x);
        let y = f64::from(*y);
        dot += x * y;
        norm_a += x * x;
        norm_b += y * y;
    }
    let denom = norm_a.sqrt() * norm_b.sqrt();
    if denom < f64::EPSILON {
        return 0.0;
    }
    (dot / denom) as f32
}

/// Extract normalized facet key from a memory abstract.
///
/// Mirrors Python's `MemoryDeduplicator._extract_facet_key()`:
/// 1. Normalize whitespace.
/// 2. Split on first separator (`：`, `:`, `-`, `—`).
/// 3. If no separator, take first 24 chars.
#[must_use]
pub fn extract_facet_key(text: &str) -> String {
    if text.is_empty() {
        return String::new();
    }
    // Normalize whitespace (collapse runs of whitespace to single space).
    let normalized: String = text.split_whitespace().collect::<Vec<_>>().join(" ");
    // Try each separator in order (matching Python's priority).
    for sep in ['：', ':', '-', '—'] {
        if let Some(pos) = normalized.find(sep) {
            let left = normalized[..pos].trim().to_lowercase();
            if !left.is_empty() {
                return left;
            }
        }
    }
    // Fallback: first 24 characters, break on word boundary if possible.
    let lower = normalized.to_lowercase();
    if lower.len() <= 24 {
        return lower.trim().to_owned();
    }
    // Find last space within first 24 chars.
    let prefix = &lower[..lower
        .char_indices()
        .take_while(|(i, _)| *i < 24)
        .last()
        .map_or(0, |(i, c)| i + c.len_utf8())];
    if let Some(space_pos) = prefix.rfind(' ') {
        prefix[..space_pos].trim().to_owned()
    } else {
        prefix.trim().to_owned()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn cosine_identical() {
        let v = vec![1.0, 2.0, 3.0];
        let sim = cosine_similarity(&v, &v);
        assert!((sim - 1.0).abs() < 1e-5);
    }

    #[test]
    fn cosine_orthogonal() {
        let a = vec![1.0, 0.0, 0.0];
        let b = vec![0.0, 1.0, 0.0];
        let sim = cosine_similarity(&a, &b);
        assert!(sim.abs() < 1e-5);
    }

    #[test]
    fn cosine_empty() {
        assert_eq!(cosine_similarity(&[], &[]), 0.0);
    }

    #[test]
    fn cosine_different_length() {
        assert_eq!(cosine_similarity(&[1.0], &[1.0, 2.0]), 0.0);
    }

    #[test]
    fn facet_key_with_separator() {
        assert_eq!(extract_facet_key("Code Style — prefers Rust"), "code style");
        assert_eq!(extract_facet_key("IDE: VSCode"), "ide");
    }

    #[test]
    fn facet_key_no_separator() {
        assert_eq!(extract_facet_key("simple text"), "simple text");
    }
}
