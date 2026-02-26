//! Levenshtein edit distance algorithm.
//!
//! Used by the fuzzy replacers to score similarity between candidate
//! text blocks and the search string.

/// Maximum character count for Levenshtein inputs.
///
/// Inputs longer than this are rejected with a pessimistic distance estimate
/// to prevent O(m*n) allocation/computation DoS. VS Code uses 128; we use
/// 10,000 which covers any reasonable code block.
const MAX_LEVENSHTEIN_INPUT: usize = 10_000;

/// Compute the Levenshtein edit distance between two strings.
///
/// Returns the minimum number of single-character edits (insertions,
/// deletions, substitutions) required to transform `a` into `b`.
///
/// If either input exceeds [`MAX_LEVENSHTEIN_INPUT`] characters, returns
/// `max(m, n)` as a pessimistic upper bound without computing the full matrix.
/// Also applies a quick-reject heuristic: if `|m - n| > max(m, n) / 3`, the
/// strings are too different in length for a meaningful edit distance, so
/// `max(m, n)` is returned immediately.
pub fn distance(a: &str, b: &str) -> usize {
    let a_chars: Vec<char> = a.chars().collect();
    let b_chars: Vec<char> = b.chars().collect();
    let m = a_chars.len();
    let n = b_chars.len();

    if m == 0 {
        return n;
    }
    if n == 0 {
        return m;
    }

    let max_len = m.max(n);

    // DoS guard: reject oversized inputs with pessimistic estimate.
    if m > MAX_LEVENSHTEIN_INPUT || n > MAX_LEVENSHTEIN_INPUT {
        return max_len;
    }

    // Quick reject: length difference alone implies high distance.
    let diff = if m > n { m - n } else { n - m };
    if diff > max_len / 3 {
        return max_len;
    }

    // Use two rows instead of full matrix for O(min(m,n)) space.
    let mut prev = vec![0usize; n + 1];
    let mut curr = vec![0usize; n + 1];

    for j in 0..=n {
        prev[j] = j;
    }

    for i in 1..=m {
        curr[0] = i;
        for j in 1..=n {
            let cost = if a_chars[i - 1] == b_chars[j - 1] {
                0
            } else {
                1
            };
            curr[j] = (prev[j] + 1) // deletion
                .min(curr[j - 1] + 1) // insertion
                .min(prev[j - 1] + cost); // substitution
        }
        std::mem::swap(&mut prev, &mut curr);
    }

    prev[n]
}

/// Compute similarity ratio between two strings (0.0 = completely different,
/// 1.0 = identical).
pub fn similarity(a: &str, b: &str) -> f64 {
    let max_len = a.len().max(b.len());
    if max_len == 0 {
        return 1.0;
    }
    let dist = distance(a, b);
    1.0 - (dist as f64 / max_len as f64)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_identical() {
        assert_eq!(distance("hello", "hello"), 0);
        assert!((similarity("hello", "hello") - 1.0).abs() < f64::EPSILON);
    }

    #[test]
    fn test_empty() {
        assert_eq!(distance("", "abc"), 3);
        assert_eq!(distance("abc", ""), 3);
        assert_eq!(distance("", ""), 0);
    }

    #[test]
    fn test_single_edit() {
        assert_eq!(distance("kitten", "sitten"), 1); // substitution
        assert_eq!(distance("cat", "cats"), 1); // insertion
        assert_eq!(distance("cats", "cat"), 1); // deletion
    }

    #[test]
    fn test_classic() {
        assert_eq!(distance("kitten", "sitting"), 3);
    }

    #[test]
    fn test_similarity_range() {
        let s = similarity("hello", "world");
        assert!(s >= 0.0 && s <= 1.0);
    }
}
