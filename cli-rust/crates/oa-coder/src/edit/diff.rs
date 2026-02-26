//! Diff generation using the `similar` crate.
//!
//! Generates unified diffs for display after edit operations.

use similar::{Algorithm, TextDiff};

/// Generate a unified diff between old and new content.
///
/// Uses the Patience diff algorithm which produces cleaner diffs
/// for source code by preserving structure.
pub fn unified_diff(file_name: &str, old: &str, new: &str) -> String {
    let diff = TextDiff::configure()
        .algorithm(Algorithm::Patience)
        .diff_lines(old, new);

    diff.unified_diff()
        .header(&format!("a/{file_name}"), &format!("b/{file_name}"))
        .to_string()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_no_diff() {
        let result = unified_diff("test.rs", "hello\n", "hello\n");
        // No changes should produce empty or minimal diff.
        assert!(!result.contains('+') || !result.contains('-'));
    }

    #[test]
    fn test_simple_diff() {
        let old = "line1\nline2\nline3\n";
        let new = "line1\nmodified\nline3\n";
        let result = unified_diff("test.rs", old, new);
        assert!(result.contains("-line2"));
        assert!(result.contains("+modified"));
    }
}
