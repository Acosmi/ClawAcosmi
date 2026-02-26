//! 9-layer fuzzy matching edit engine.
//!
//! Port of OpenAcosmi's `edit.ts` replacer chain. Each replacer is a function
//! that yields candidate strings found in the original content. The `replace()`
//! orchestrator then locates the candidate and performs the substitution.
//!
//! # Architecture
//!
//! Each replacer takes `(content, find)` and returns a `Vec<String>` of
//! candidate matches — the **actual text found in `content`** that corresponds
//! to the search string. The orchestrator then:
//! 1. For each candidate, finds its position in content via `str::find()`
//! 2. If `replace_all` → `content.replace(candidate, new)`
//! 3. If not `replace_all` → verifies uniqueness (single occurrence), else skips
//!
//! # Layers
//!
//! 1. `SimpleReplacer` — exact substring match
//! 2. `LineTrimmedReplacer` — trim each line before comparing
//! 3. `BlockAnchorReplacer` — anchor on first/last lines + Levenshtein
//! 4. `WhitespaceNormalizedReplacer` — collapse whitespace
//! 5. `IndentationFlexibleReplacer` — normalize indentation
//! 6. `EscapeNormalizedReplacer` — normalize escape sequences
//! 7. `TrimmedBoundaryReplacer` — trim boundary blank lines
//! 8. `ContextAwareReplacer` — context-line anchoring + similarity
//! 9. `MultiOccurrenceReplacer` — yields all exact matches for `replace_all`

pub mod diff;
pub mod levenshtein;
pub mod replacers;

use tracing::debug;

/// Similarity thresholds for block anchor matching (matches OpenAcosmi).
const SINGLE_CANDIDATE_SIMILARITY_THRESHOLD: f64 = 0.0;
const MULTIPLE_CANDIDATES_SIMILARITY_THRESHOLD: f64 = 0.3;

/// A replacer function signature: takes (content, find) and returns
/// candidate strings found in content that match the search.
type Replacer = fn(&str, &str) -> Vec<String>;

/// The ordered chain of replacers, matching OpenAcosmi's exact order.
const REPLACER_CHAIN: &[(&str, Replacer)] = &[
    ("SimpleReplacer", replacers::simple_replacer),
    ("LineTrimmedReplacer", replacers::line_trimmed_replacer),
    ("BlockAnchorReplacer", replacers::block_anchor_replacer),
    (
        "WhitespaceNormalizedReplacer",
        replacers::whitespace_normalized_replacer,
    ),
    (
        "IndentationFlexibleReplacer",
        replacers::indentation_flexible_replacer,
    ),
    (
        "EscapeNormalizedReplacer",
        replacers::escape_normalized_replacer,
    ),
    (
        "TrimmedBoundaryReplacer",
        replacers::trimmed_boundary_replacer,
    ),
    (
        "ContextAwareReplacer",
        replacers::context_aware_replacer,
    ),
    (
        "MultiOccurrenceReplacer",
        replacers::multi_occurrence_replacer,
    ),
];

/// Try to replace `old` with `new` in `content` using the 9-layer chain.
///
/// Returns `Some(new_content)` if a match was found, `None` otherwise.
///
/// Matches OpenAcosmi's `replace()` function logic exactly:
/// - For each replacer, for each yielded candidate:
///   - Find the candidate in content
///   - If `replace_all` → replace all occurrences and return
///   - If not `replace_all` → check uniqueness; if unique, replace; if ambiguous, skip
/// - If all replacers exhausted with no replacement → `None`
pub fn replace(content: &str, old: &str, new: &str, replace_all: bool) -> Option<String> {
    let mut any_found = false;

    for &(name, replacer) in REPLACER_CHAIN {
        let candidates = replacer(content, old);

        for search in &candidates {
            let Some(index) = content.find(search.as_str()) else {
                continue;
            };
            any_found = true;

            if replace_all {
                debug!(replacer = name, "replace_all match");
                return Some(content.replace(search.as_str(), new));
            }

            // Check uniqueness: last occurrence must equal first occurrence.
            let last_index = content.rfind(search.as_str());
            if last_index != Some(index) {
                // Multiple occurrences — skip this candidate, try next.
                debug!(replacer = name, "ambiguous match, skipping");
                continue;
            }

            // Unique match — perform replacement.
            debug!(replacer = name, "unique match found");
            let mut result = String::with_capacity(content.len() + new.len());
            result.push_str(&content[..index]);
            result.push_str(new);
            result.push_str(&content[index + search.len()..]);
            return Some(result);
        }
    }

    if any_found {
        debug!("found matches but all were ambiguous");
    }

    None
}
