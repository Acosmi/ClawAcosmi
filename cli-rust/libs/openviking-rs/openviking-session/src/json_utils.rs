// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Tolerant JSON parsing utilities.
//!
//! FIX-JSON: Ported from Python's `parse_json_from_response`.

use regex::Regex;
use serde::de::DeserializeOwned;

/// Parse JSON from a string that may contain markdown fences, extra text,
/// or other non-JSON content around the actual JSON payload.
///
/// This tolerant parser:
/// 1. Tries strict `serde_json::from_str` first.
/// 2. Strips markdown code fences (```json ... ```).
/// 3. Finds the first `{...}` or `[...]` block.
/// 4. Returns `None` if nothing works.
pub fn parse_json_from_response<T: DeserializeOwned>(raw: &str) -> Option<T> {
    let trimmed = raw.trim();

    // 1. Try strict parse
    if let Ok(val) = serde_json::from_str::<T>(trimmed) {
        return Some(val);
    }

    // 2. Strip markdown code fences
    let stripped = strip_markdown_fences(trimmed);
    if let Ok(val) = serde_json::from_str::<T>(&stripped) {
        return Some(val);
    }

    // 3. Find first JSON object or array
    if let Some(json_str) = extract_json_block(&stripped) {
        if let Ok(val) = serde_json::from_str::<T>(&json_str) {
            return Some(val);
        }
    }

    // 4. Last resort: try to find JSON in original text
    if let Some(json_str) = extract_json_block(trimmed) {
        if let Ok(val) = serde_json::from_str::<T>(&json_str) {
            return Some(val);
        }
    }

    None
}

/// Strip markdown code fences from a string.
fn strip_markdown_fences(input: &str) -> String {
    if let Ok(re) = Regex::new(r"(?s)```(?:json|JSON)?\s*\n?(.*?)\n?\s*```") {
        if let Some(caps) = re.captures(input) {
            if let Some(m) = caps.get(1) {
                return m.as_str().trim().to_owned();
            }
        }
    }
    input.to_owned()
}

/// Find the first balanced `{...}` or `[...]` block in the input.
fn extract_json_block(input: &str) -> Option<String> {
    let (open, close) = if let Some(idx) = input.find('{') {
        if let Some(arr_idx) = input.find('[') {
            if arr_idx < idx {
                ('[', ']')
            } else {
                ('{', '}')
            }
        } else {
            ('{', '}')
        }
    } else if input.find('[').is_some() {
        ('[', ']')
    } else {
        return None;
    };

    let start = input.find(open)?;
    let mut depth = 0;
    let mut in_string = false;
    let mut escape = false;

    for (i, ch) in input[start..].char_indices() {
        if escape {
            escape = false;
            continue;
        }
        if ch == '\\' && in_string {
            escape = true;
            continue;
        }
        if ch == '"' {
            in_string = !in_string;
            continue;
        }
        if in_string {
            continue;
        }
        if ch == open {
            depth += 1;
        } else if ch == close {
            depth -= 1;
            if depth == 0 {
                return Some(input[start..start + i + 1].to_owned());
            }
        }
    }
    None
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use serde::Deserialize;

    #[derive(Debug, Deserialize, PartialEq)]
    struct TestPayload {
        action: String,
        value: i32,
    }

    #[test]
    fn parse_strict_json() {
        let raw = r#"{"action": "create", "value": 42}"#;
        let result: Option<TestPayload> = parse_json_from_response(raw);
        assert_eq!(
            result,
            Some(TestPayload {
                action: "create".into(),
                value: 42
            })
        );
    }

    #[test]
    fn parse_markdown_fenced_json() {
        let raw = "Here is the result:\n```json\n{\"action\": \"merge\", \"value\": 7}\n```\nDone.";
        let result: Option<TestPayload> = parse_json_from_response(raw);
        assert_eq!(
            result,
            Some(TestPayload {
                action: "merge".into(),
                value: 7
            })
        );
    }

    #[test]
    fn parse_json_with_prefix() {
        let raw = "The answer is: {\"action\": \"delete\", \"value\": 0} end.";
        let result: Option<TestPayload> = parse_json_from_response(raw);
        assert_eq!(
            result,
            Some(TestPayload {
                action: "delete".into(),
                value: 0
            })
        );
    }

    #[test]
    fn parse_garbage_returns_none() {
        let raw = "This is not JSON at all";
        let result: Option<TestPayload> = parse_json_from_response(raw);
        assert_eq!(result, None);
    }

    #[test]
    fn parse_json_array() {
        let raw = "Results: [{\"a\":1},{\"a\":2}]";
        let result: Option<Vec<serde_json::Value>> = parse_json_from_response(raw);
        assert!(result.is_some());
        assert_eq!(result.unwrap().len(), 2);
    }
}
