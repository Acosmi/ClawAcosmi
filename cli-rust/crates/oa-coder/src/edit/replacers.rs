//! 9-layer replacer implementations.
//!
//! Each replacer takes `(content, find)` and returns a `Vec<String>` of
//! candidate strings found in `content` that match `find`. The orchestrator
//! in `mod.rs` handles the actual replacement.
//!
//! Ported from OpenAcosmi's `edit.ts` — each function corresponds to an
//! exported `Replacer` generator in the TypeScript source.

use crate::edit::levenshtein;

// ---------------------------------------------------------------------------
// Layer 1: SimpleReplacer
// ---------------------------------------------------------------------------

/// Exact substring match — yields the find string itself if present.
pub fn simple_replacer(_content: &str, find: &str) -> Vec<String> {
    vec![find.to_owned()]
}

// ---------------------------------------------------------------------------
// Layer 2: LineTrimmedReplacer
// ---------------------------------------------------------------------------

/// Matches by comparing trimmed lines. Yields the original text from content
/// (preserving whitespace) when trimmed lines match.
pub fn line_trimmed_replacer(content: &str, find: &str) -> Vec<String> {
    let original_lines: Vec<&str> = content.split('\n').collect();
    let mut search_lines: Vec<&str> = find.split('\n').collect();

    // Remove trailing empty line (matches OpenAcosmi behavior).
    if search_lines.last() == Some(&"") {
        search_lines.pop();
    }

    if search_lines.is_empty() {
        return Vec::new();
    }

    let mut results = Vec::new();

    for i in 0..=original_lines.len().saturating_sub(search_lines.len()) {
        let mut matches = true;

        for j in 0..search_lines.len() {
            if original_lines[i + j].trim() != search_lines[j].trim() {
                matches = false;
                break;
            }
        }

        if matches {
            // Calculate the byte range in the original content.
            let mut start_idx = 0;
            for k in 0..i {
                start_idx += original_lines[k].len() + 1; // +1 for \n
            }

            let mut end_idx = start_idx;
            for k in 0..search_lines.len() {
                end_idx += original_lines[i + k].len();
                if k < search_lines.len() - 1 {
                    end_idx += 1; // newline between lines, not after last
                }
            }

            results.push(content[start_idx..end_idx].to_owned());
        }
    }

    results
}

// ---------------------------------------------------------------------------
// Layer 3: BlockAnchorReplacer
// ---------------------------------------------------------------------------

/// Matches by anchoring on first and last lines, with variable-length blocks.
/// Scores middle lines via Levenshtein similarity.
pub fn block_anchor_replacer(content: &str, find: &str) -> Vec<String> {
    let original_lines: Vec<&str> = content.split('\n').collect();
    let mut search_lines: Vec<&str> = find.split('\n').collect();

    if search_lines.len() < 3 {
        return Vec::new();
    }

    if search_lines.last() == Some(&"") {
        search_lines.pop();
    }

    let first_line_search = search_lines[0].trim();
    let last_line_search = search_lines[search_lines.len() - 1].trim();
    let search_block_size = search_lines.len();

    // Collect candidate positions where both anchors match.
    let mut candidates: Vec<(usize, usize)> = Vec::new(); // (start_line, end_line)

    for i in 0..original_lines.len() {
        if original_lines[i].trim() != first_line_search {
            continue;
        }

        for j in (i + 2)..original_lines.len() {
            if original_lines[j].trim() == last_line_search {
                candidates.push((i, j));
                break; // Only first match of last line
            }
        }
    }

    if candidates.is_empty() {
        return Vec::new();
    }

    if candidates.len() == 1 {
        // Single candidate — use relaxed threshold.
        let (start_line, end_line) = candidates[0];
        let actual_block_size = end_line - start_line + 1;

        let lines_to_check =
            (search_block_size - 2).min(actual_block_size.saturating_sub(2));

        let similarity = if lines_to_check > 0 {
            let mut sim = 0.0;
            for j in 1..search_block_size.min(actual_block_size) - 1 {
                let original_line = original_lines[start_line + j].trim();
                let search_line = search_lines[j].trim();
                let max_len = original_line.len().max(search_line.len());
                if max_len == 0 {
                    continue;
                }
                let dist = levenshtein::distance(original_line, search_line);
                sim += (1.0 - dist as f64 / max_len as f64) / lines_to_check as f64;

                if sim >= super::SINGLE_CANDIDATE_SIMILARITY_THRESHOLD {
                    break;
                }
            }
            sim
        } else {
            1.0 // No middle lines, accept based on anchors
        };

        if similarity >= super::SINGLE_CANDIDATE_SIMILARITY_THRESHOLD {
            return vec![extract_block(content, &original_lines, start_line, end_line)];
        }
        return Vec::new();
    }

    // Multiple candidates — pick best by similarity.
    let mut best_match: Option<(usize, usize)> = None;
    let mut max_similarity: f64 = -1.0;

    for &(start_line, end_line) in &candidates {
        let actual_block_size = end_line - start_line + 1;
        let lines_to_check =
            (search_block_size - 2).min(actual_block_size.saturating_sub(2));

        let similarity = if lines_to_check > 0 {
            let mut sim = 0.0;
            for j in 1..search_block_size.min(actual_block_size) - 1 {
                let original_line = original_lines[start_line + j].trim();
                let search_line = search_lines[j].trim();
                let max_len = original_line.len().max(search_line.len());
                if max_len == 0 {
                    continue;
                }
                let dist = levenshtein::distance(original_line, search_line);
                sim += 1.0 - dist as f64 / max_len as f64;
            }
            sim / lines_to_check as f64
        } else {
            1.0
        };

        if similarity > max_similarity {
            max_similarity = similarity;
            best_match = Some((start_line, end_line));
        }
    }

    if max_similarity >= super::MULTIPLE_CANDIDATES_SIMILARITY_THRESHOLD {
        if let Some((start_line, end_line)) = best_match {
            return vec![extract_block(content, &original_lines, start_line, end_line)];
        }
    }

    Vec::new()
}

/// Extract a block of lines from content as a single string.
fn extract_block(content: &str, lines: &[&str], start_line: usize, end_line: usize) -> String {
    let mut start_idx = 0;
    for k in 0..start_line {
        start_idx += lines[k].len() + 1;
    }
    let mut end_idx = start_idx;
    for k in start_line..=end_line {
        end_idx += lines[k].len();
        if k < end_line {
            end_idx += 1;
        }
    }
    content[start_idx..end_idx].to_owned()
}

// ---------------------------------------------------------------------------
// Layer 4: WhitespaceNormalizedReplacer
// ---------------------------------------------------------------------------

/// Matches after normalizing whitespace. Yields the original text from content.
pub fn whitespace_normalized_replacer(content: &str, find: &str) -> Vec<String> {
    let normalize = |s: &str| -> String {
        s.split_whitespace().collect::<Vec<&str>>().join(" ")
    };

    let normalized_find = normalize(find);
    let lines: Vec<&str> = content.split('\n').collect();
    let mut results = Vec::new();

    // Pre-compile the whitespace-flexible regex once, outside the loop.
    let ws_regex = {
        let words: Vec<&str> = find.split_whitespace().collect();
        if words.is_empty() {
            None
        } else {
            let pattern = words
                .iter()
                .map(|w| regex::escape(w))
                .collect::<Vec<_>>()
                .join(r"\s+");
            regex::Regex::new(&pattern).ok()
        }
    };

    // Single-line matches.
    for line in &lines {
        if normalize(line) == normalized_find {
            results.push((*line).to_owned());
        } else {
            let normalized_line = normalize(line);
            if normalized_line.contains(&normalized_find) {
                if let Some(ref re) = ws_regex {
                    if let Some(m) = re.find(line) {
                        results.push(m.as_str().to_owned());
                    }
                }
            }
        }
    }

    // Multi-line matches.
    let find_lines: Vec<&str> = find.split('\n').collect();
    if find_lines.len() > 1 {
        for i in 0..=lines.len().saturating_sub(find_lines.len()) {
            let block = lines[i..i + find_lines.len()].join("\n");
            if normalize(&block) == normalized_find {
                results.push(block);
            }
        }
    }

    results
}

// ---------------------------------------------------------------------------
// Layer 5: IndentationFlexibleReplacer
// ---------------------------------------------------------------------------

/// Matches after removing common indentation. Yields the original text.
pub fn indentation_flexible_replacer(content: &str, find: &str) -> Vec<String> {
    let remove_indentation = |text: &str| -> String {
        let lines: Vec<&str> = text.split('\n').collect();
        let non_empty: Vec<&&str> = lines.iter().filter(|l| !l.trim().is_empty()).collect();

        if non_empty.is_empty() {
            return text.to_owned();
        }

        let min_indent = non_empty
            .iter()
            .map(|l| l.len() - l.trim_start().len())
            .min()
            .unwrap_or(0);

        lines
            .iter()
            .map(|line| {
                if line.trim().is_empty() {
                    *line
                } else if line.len() > min_indent && line.is_char_boundary(min_indent) {
                    &line[min_indent..]
                } else {
                    line.trim_start()
                }
            })
            .collect::<Vec<&str>>()
            .join("\n")
    };

    let normalized_find = remove_indentation(find);
    let content_lines: Vec<&str> = content.split('\n').collect();
    let find_lines: Vec<&str> = find.split('\n').collect();
    let mut results = Vec::new();

    for i in 0..=content_lines.len().saturating_sub(find_lines.len()) {
        let block = content_lines[i..i + find_lines.len()].join("\n");
        if remove_indentation(&block) == normalized_find {
            results.push(block);
        }
    }

    results
}

// ---------------------------------------------------------------------------
// Layer 6: EscapeNormalizedReplacer
// ---------------------------------------------------------------------------

/// Matches after normalizing escape sequences in the find string.
pub fn escape_normalized_replacer(content: &str, find: &str) -> Vec<String> {
    let unescape = |s: &str| -> String {
        let mut result = String::with_capacity(s.len());
        let mut chars = s.chars().peekable();

        while let Some(ch) = chars.next() {
            if ch == '\\' {
                match chars.peek() {
                    Some('n') => {
                        chars.next();
                        result.push('\n');
                    }
                    Some('t') => {
                        chars.next();
                        result.push('\t');
                    }
                    Some('r') => {
                        chars.next();
                        result.push('\r');
                    }
                    Some('\\') => {
                        chars.next();
                        result.push('\\');
                    }
                    Some('\'') => {
                        chars.next();
                        result.push('\'');
                    }
                    Some('"') => {
                        chars.next();
                        result.push('"');
                    }
                    Some('`') => {
                        chars.next();
                        result.push('`');
                    }
                    Some('$') => {
                        chars.next();
                        result.push('$');
                    }
                    Some('\n') => {
                        chars.next();
                        result.push('\n');
                    }
                    _ => result.push(ch),
                }
            } else {
                result.push(ch);
            }
        }

        result
    };

    let unescaped_find = unescape(find);
    let mut results = Vec::new();

    // Direct match with unescaped find.
    if content.contains(&unescaped_find) {
        results.push(unescaped_find.clone());
    }

    // Also try finding escaped versions in content that match.
    let lines: Vec<&str> = content.split('\n').collect();
    let find_lines: Vec<&str> = unescaped_find.split('\n').collect();

    if find_lines.len() <= lines.len() {
        for i in 0..=lines.len() - find_lines.len() {
            let block = lines[i..i + find_lines.len()].join("\n");
            let unescaped_block = unescape(&block);
            if unescaped_block == unescaped_find && !results.contains(&block) {
                results.push(block);
            }
        }
    }

    results
}

// ---------------------------------------------------------------------------
// Layer 7: TrimmedBoundaryReplacer
// ---------------------------------------------------------------------------

/// Matches after trimming leading/trailing whitespace from the find string.
pub fn trimmed_boundary_replacer(content: &str, find: &str) -> Vec<String> {
    let trimmed_find = find.trim();

    if trimmed_find == find {
        // Already trimmed, no point in trying.
        return Vec::new();
    }

    let mut results = Vec::new();

    // Direct substring match.
    if content.contains(trimmed_find) {
        results.push(trimmed_find.to_owned());
    }

    // Block matching where trimmed content matches.
    let lines: Vec<&str> = content.split('\n').collect();
    let find_lines: Vec<&str> = find.split('\n').collect();

    if find_lines.len() <= lines.len() {
        for i in 0..=lines.len() - find_lines.len() {
            let block = lines[i..i + find_lines.len()].join("\n");
            if block.trim() == trimmed_find && !results.contains(&block) {
                results.push(block);
            }
        }
    }

    results
}

// ---------------------------------------------------------------------------
// Layer 8: ContextAwareReplacer
// ---------------------------------------------------------------------------

/// Matches using first/last line anchors with middle-line similarity scoring.
/// Requires at least 3 lines and 50% middle-line exact match rate.
pub fn context_aware_replacer(content: &str, find: &str) -> Vec<String> {
    let mut find_lines: Vec<&str> = find.split('\n').collect();

    if find_lines.len() < 3 {
        return Vec::new();
    }

    if find_lines.last() == Some(&"") {
        find_lines.pop();
    }

    let content_lines: Vec<&str> = content.split('\n').collect();
    let first_line = find_lines[0].trim();
    let last_line = find_lines[find_lines.len() - 1].trim();

    for i in 0..content_lines.len() {
        if content_lines[i].trim() != first_line {
            continue;
        }

        for j in (i + 2)..content_lines.len() {
            if content_lines[j].trim() != last_line {
                continue;
            }

            let block_lines = &content_lines[i..=j];

            // Check block size matches.
            if block_lines.len() != find_lines.len() {
                break;
            }

            // Score middle lines: 50% exact match threshold.
            let mut matching_lines = 0;
            let mut total_non_empty = 0;

            for k in 1..block_lines.len() - 1 {
                let block_line = block_lines[k].trim();
                let find_line = find_lines[k].trim();

                if !block_line.is_empty() || !find_line.is_empty() {
                    total_non_empty += 1;
                    if block_line == find_line {
                        matching_lines += 1;
                    }
                }
            }

            if total_non_empty == 0
                || matching_lines as f64 / total_non_empty as f64 >= 0.5
            {
                let block = block_lines.join("\n");
                return vec![block];
            }

            break;
        }
    }

    Vec::new()
}

// ---------------------------------------------------------------------------
// Layer 9: MultiOccurrenceReplacer
// ---------------------------------------------------------------------------

/// Yields all exact occurrences of find in content.
/// Used for `replace_all` mode.
pub fn multi_occurrence_replacer(content: &str, find: &str) -> Vec<String> {
    let mut results = Vec::new();
    let mut start = 0;

    while let Some(idx) = content[start..].find(find) {
        results.push(find.to_owned());
        start += idx + find.len();
    }

    results
}

// ===========================================================================
// Tests
// ===========================================================================

#[cfg(test)]
mod tests {
    use super::*;

    // -- Layer 1: SimpleReplacer --
    #[test]
    fn test_simple_exact_match() {
        let candidates = simple_replacer("hello world", "world");
        assert_eq!(candidates, vec!["world"]);
    }

    // -- Layer 2: LineTrimmedReplacer --
    #[test]
    fn test_line_trimmed_whitespace_diff() {
        let content = "  function foo() {\n    return 1;\n  }";
        let find = "function foo() {\n  return 1;\n}";
        let candidates = line_trimmed_replacer(content, find);
        assert_eq!(candidates.len(), 1);
        assert_eq!(candidates[0], content);
    }

    #[test]
    fn test_line_trimmed_no_match() {
        let content = "function foo() {\n  return 1;\n}";
        let find = "function bar() {\n  return 2;\n}";
        let candidates = line_trimmed_replacer(content, find);
        assert!(candidates.is_empty());
    }

    #[test]
    fn test_line_trimmed_trailing_newline() {
        let content = "line1\nline2\nline3";
        let find = "line1\nline2\n"; // trailing newline
        let candidates = line_trimmed_replacer(content, find);
        assert_eq!(candidates.len(), 1);
        assert_eq!(candidates[0], "line1\nline2");
    }

    // -- Layer 3: BlockAnchorReplacer --
    #[test]
    fn test_block_anchor_exact() {
        let content = "start\n  middle1\n  middle2\nend\nother";
        let find = "start\nmiddle1\nmiddle2\nend";
        let candidates = block_anchor_replacer(content, find);
        assert_eq!(candidates.len(), 1);
        assert!(candidates[0].starts_with("start"));
        assert!(candidates[0].ends_with("end"));
    }

    #[test]
    fn test_block_anchor_too_few_lines() {
        let content = "hello\nworld";
        let find = "hello\nworld";
        let candidates = block_anchor_replacer(content, find);
        assert!(candidates.is_empty()); // Need >= 3 lines
    }

    // -- Layer 4: WhitespaceNormalizedReplacer --
    #[test]
    fn test_whitespace_normalized_single_line() {
        let content = "let   x   =   1;";
        let find = "let x = 1;";
        let candidates = whitespace_normalized_replacer(content, find);
        assert!(!candidates.is_empty());
    }

    #[test]
    fn test_whitespace_normalized_multiline() {
        let content = "if  (true)  {\n    return  1;\n}";
        let find = "if (true) {\n  return 1;\n}";
        let candidates = whitespace_normalized_replacer(content, find);
        assert!(!candidates.is_empty());
    }

    // -- Layer 5: IndentationFlexibleReplacer --
    #[test]
    fn test_indentation_flexible() {
        let content = "    function test() {\n        return 1;\n    }";
        let find = "function test() {\n    return 1;\n}";
        let candidates = indentation_flexible_replacer(content, find);
        assert_eq!(candidates.len(), 1);
        assert_eq!(candidates[0], content);
    }

    // -- Layer 6: EscapeNormalizedReplacer --
    #[test]
    fn test_escape_normalized_direct() {
        // Content has a literal newline; find uses the escape sequence \n.
        let content = "console.log(\"hello\nworld\")";
        let find = "console.log(\"hello\\nworld\")";
        let candidates = escape_normalized_replacer(content, find);
        assert!(!candidates.is_empty());
    }

    #[test]
    fn test_escape_normalized_identical() {
        // Both content and find are identical — direct match via unescaping.
        let content = "hello world";
        let find = "hello world";
        let candidates = escape_normalized_replacer(content, find);
        assert!(!candidates.is_empty());
    }

    // -- Layer 7: TrimmedBoundaryReplacer --
    #[test]
    fn test_trimmed_boundary() {
        let content = "function test() {}";
        let find = "\n  function test() {}  \n";
        let candidates = trimmed_boundary_replacer(content, find);
        assert!(!candidates.is_empty());
    }

    #[test]
    fn test_trimmed_boundary_already_trimmed() {
        let candidates = trimmed_boundary_replacer("hello", "hello");
        assert!(candidates.is_empty()); // Already trimmed, skip
    }

    // -- Layer 8: ContextAwareReplacer --
    #[test]
    fn test_context_aware_exact() {
        let content = "function foo() {\n  let x = 1;\n  return x;\n}";
        let find = "function foo() {\n  let x = 1;\n  return x;\n}";
        let candidates = context_aware_replacer(content, find);
        assert_eq!(candidates.len(), 1);
    }

    #[test]
    fn test_context_aware_with_diff() {
        let content = "function foo() {\n  let x = 1;\n  let y = 2;\n  return x + y;\n}";
        let find = "function foo() {\n  let x = 1;\n  let y = 2;\n  return x + y;\n}";
        let candidates = context_aware_replacer(content, find);
        assert_eq!(candidates.len(), 1);
    }

    // -- Layer 9: MultiOccurrenceReplacer --
    #[test]
    fn test_multi_occurrence() {
        let content = "aaa bbb aaa ccc aaa";
        let find = "aaa";
        let candidates = multi_occurrence_replacer(content, find);
        assert_eq!(candidates.len(), 3);
    }

    // -- Integration: replace() orchestrator --
    #[test]
    fn test_replace_exact() {
        let result = super::super::replace("hello world", "world", "rust", false);
        assert_eq!(result, Some("hello rust".to_owned()));
    }

    #[test]
    fn test_replace_no_match() {
        let result = super::super::replace("hello world", "missing", "rust", false);
        assert_eq!(result, None);
    }

    #[test]
    fn test_replace_all() {
        let result = super::super::replace("aaa bbb aaa", "aaa", "ccc", true);
        assert_eq!(result, Some("ccc bbb ccc".to_owned()));
    }

    #[test]
    fn test_replace_ambiguous_skips() {
        // "aaa" appears twice, not replace_all — should fail to replace via
        // SimpleReplacer but succeed via MultiOccurrenceReplacer? No —
        // MultiOccurrenceReplacer also yields multiple, and without replaceAll
        // the orchestrator will skip. So it should return None.
        let result = super::super::replace("aaa bbb aaa", "aaa", "ccc", false);
        assert_eq!(result, None);
    }

    #[test]
    fn test_replace_with_trimmed_whitespace() {
        let content = "  function foo() {\n    return 1;\n  }";
        let find = "function foo() {\n  return 1;\n}";
        let new = "function bar() {\n  return 2;\n}";
        let result = super::super::replace(content, find, new, false);
        assert!(result.is_some());
    }
}
