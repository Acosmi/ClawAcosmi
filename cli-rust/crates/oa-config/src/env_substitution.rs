/// Environment variable substitution for config values.
///
/// Supports `${VAR_NAME}` syntax in string values, substituted at config load time.
/// - Only uppercase env vars are matched: `[A-Z_][A-Z0-9_]*`
/// - Escape with `$${VAR}` to output literal `${VAR}`
/// - Missing env vars produce an error with context
///
/// Source: `src/config/env-substitution.ts`

use std::env;

use anyhow::{Result, bail};
use serde_json::Value;

/// Pattern for valid uppercase env var names.
///
/// Starts with an uppercase letter or underscore, followed by uppercase letters,
/// digits, or underscores.
fn is_valid_env_var_name(name: &str) -> bool {
    // Equivalent to /^[A-Z_][A-Z0-9_]*$/
    if name.is_empty() {
        return false;
    }
    let mut chars = name.chars();
    let first = chars.next().unwrap_or(' ');
    if !first.is_ascii_uppercase() && first != '_' {
        return false;
    }
    chars.all(|c| c.is_ascii_uppercase() || c.is_ascii_digit() || c == '_')
}

/// Substitute `${VAR}` references in a single string value.
///
/// Returns the string with all valid env var references replaced by their values.
/// Escaped references (`$${VAR}`) are converted to literal `${VAR}`.
fn substitute_string(value: &str, config_path: &str) -> Result<String> {
    if !value.contains('$') {
        return Ok(value.to_string());
    }

    let mut chunks = Vec::new();
    let chars: Vec<char> = value.chars().collect();
    let len = chars.len();
    let mut i = 0;

    while i < len {
        let ch = chars[i];
        if ch != '$' {
            chunks.push(ch.to_string());
            i += 1;
            continue;
        }

        let next = chars.get(i + 1).copied();
        let after_next = chars.get(i + 2).copied();

        // Escaped: $${VAR} -> ${VAR}
        if next == Some('$') && after_next == Some('{') {
            let start = i + 3;
            if let Some(end) = find_closing_brace(&chars, start) {
                let name: String = chars[start..end].iter().collect();
                if is_valid_env_var_name(&name) {
                    chunks.push(format!("${{{name}}}"));
                    i = end + 1;
                    continue;
                }
            }
        }

        // Substitution: ${VAR} -> value
        if next == Some('{') {
            let start = i + 2;
            if let Some(end) = find_closing_brace(&chars, start) {
                let name: String = chars[start..end].iter().collect();
                if is_valid_env_var_name(&name) {
                    let env_value = env::var(&name).ok().filter(|v| !v.is_empty());
                    match env_value {
                        Some(val) => {
                            chunks.push(val);
                            i = end + 1;
                            continue;
                        }
                        None => {
                            bail!(
                                "Missing env var \"{name}\" referenced at config path: {config_path}"
                            );
                        }
                    }
                }
            }
        }

        // Not a recognized pattern, leave untouched
        chunks.push(ch.to_string());
        i += 1;
    }

    Ok(chunks.join(""))
}

/// Find the index of the closing `}` brace starting from `start`.
fn find_closing_brace(chars: &[char], start: usize) -> Option<usize> {
    for i in start..chars.len() {
        if chars[i] == '}' {
            return Some(i);
        }
    }
    None
}

/// Recursively substitute env vars in any JSON value.
fn substitute_any(value: Value, config_path: &str) -> Result<Value> {
    match value {
        Value::String(s) => {
            let substituted = substitute_string(&s, config_path)?;
            Ok(Value::String(substituted))
        }
        Value::Array(arr) => {
            let mut result = Vec::with_capacity(arr.len());
            for (idx, item) in arr.into_iter().enumerate() {
                let child_path = format!("{config_path}[{idx}]");
                result.push(substitute_any(item, &child_path)?);
            }
            Ok(Value::Array(result))
        }
        Value::Object(map) => {
            let mut result = serde_json::Map::new();
            for (key, val) in map {
                let child_path = if config_path.is_empty() {
                    key.clone()
                } else {
                    format!("{config_path}.{key}")
                };
                result.insert(key, substitute_any(val, &child_path)?);
            }
            Ok(Value::Object(result))
        }
        // Primitives (number, boolean, null) pass through unchanged
        other => Ok(other),
    }
}

/// Resolve `${VAR_NAME}` environment variable references in config values.
///
/// Walks the entire JSON value tree and replaces `${VAR}` references in
/// string values with the corresponding environment variable. Only uppercase
/// env var names matching `[A-Z_][A-Z0-9_]*` are recognized.
///
/// # Errors
///
/// Returns an error if a referenced env var is not set or empty.
pub fn resolve_config_env_vars(obj: Value) -> Result<Value> {
    substitute_any(obj, "")
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn no_vars_passthrough() {
        let val = json!({"key": "value"});
        let result = resolve_config_env_vars(val.clone()).expect("passthrough");
        assert_eq!(result, val);
    }

    #[test]
    fn non_string_passthrough() {
        let val = json!({"num": 42, "bool": true, "null": null});
        let result = resolve_config_env_vars(val.clone()).expect("passthrough");
        assert_eq!(result, val);
    }

    #[test]
    fn valid_env_var_name_check() {
        assert!(is_valid_env_var_name("MY_VAR"));
        assert!(is_valid_env_var_name("_PRIVATE"));
        assert!(is_valid_env_var_name("A"));
        assert!(is_valid_env_var_name("ABC123"));
        assert!(!is_valid_env_var_name("lowercase"));
        assert!(!is_valid_env_var_name("123ABC"));
        assert!(!is_valid_env_var_name(""));
    }

    #[test]
    fn substitute_with_existing_var() {
        // Use HOME which is always set on macOS/Linux
        let home = env::var("HOME").expect("HOME should be set");
        let val = json!({"path": "${HOME}/config"});
        let result = resolve_config_env_vars(val).expect("should substitute");
        assert_eq!(result, json!({"path": format!("{home}/config")}));
    }

    #[test]
    fn substitute_string_internal() {
        // Test the internal substitute_string with a var we know exists (PATH)
        let path_val = env::var("PATH").expect("PATH should be set");
        let result = substitute_string("prefix-${PATH}-suffix", "test").expect("should work");
        assert_eq!(result, format!("prefix-{path_val}-suffix"));
    }

    #[test]
    fn missing_var_error() {
        // Use a var name that is extremely unlikely to exist
        let val = json!({"key": "${OPENACOSMI_NONEXISTENT_VAR_FOR_TESTING_XYZ}"});
        let result = resolve_config_env_vars(val);
        assert!(result.is_err());
        let err = result.unwrap_err().to_string();
        assert!(err.contains("OPENACOSMI_NONEXISTENT_VAR_FOR_TESTING_XYZ"));
    }

    #[test]
    fn escaped_var_literal() {
        let val = json!({"key": "$${MY_VAR}"});
        let result = resolve_config_env_vars(val).expect("escape");
        assert_eq!(result, json!({"key": "${MY_VAR}"}));
    }

    #[test]
    fn nested_object_substitution() {
        let home = env::var("HOME").expect("HOME should be set");
        let val = json!({
            "outer": {
                "inner": "${HOME}"
            },
            "arr": ["${HOME}", "plain"]
        });
        let result = resolve_config_env_vars(val).expect("nested");
        assert_eq!(result, json!({
            "outer": {
                "inner": home
            },
            "arr": [&home, "plain"]
        }));
    }
}
