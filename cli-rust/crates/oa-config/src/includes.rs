/// Config `$include` directive processing.
///
/// Supports merging modular config files via a `$include` key that can
/// reference a single file path or an array of paths. Included files are
/// resolved relative to the including file, parsed (with JSON5 support),
/// and deep-merged into the parent config.
///
/// Source: `src/config/includes.ts`

use std::collections::HashSet;
use std::path::{Path, PathBuf};

use anyhow::{Context, Result, bail};
use serde_json::Value;

/// The JSON key used to trigger include resolution.
pub const INCLUDE_KEY: &str = "$include";

/// Maximum nesting depth for `$include` directives.
pub const MAX_INCLUDE_DEPTH: usize = 10;

// ---------------------------------------------------------------------------
// Deep merge
// ---------------------------------------------------------------------------

/// Deep merge two `serde_json::Value` trees.
///
/// Merge semantics:
/// - Arrays: concatenated (target first, then source)
/// - Objects: recursively merged (source wins for conflicting leaf keys)
/// - All other types: source wins
pub fn deep_merge(target: Value, source: Value) -> Value {
    match (target, source) {
        (Value::Array(mut t), Value::Array(s)) => {
            t.extend(s);
            Value::Array(t)
        }
        (Value::Object(mut t), Value::Object(s)) => {
            for (key, s_val) in s {
                let merged = if let Some(t_val) = t.remove(&key) {
                    deep_merge(t_val, s_val)
                } else {
                    s_val
                };
                t.insert(key, merged);
            }
            Value::Object(t)
        }
        (_, source) => source,
    }
}

// ---------------------------------------------------------------------------
// Include processor
// ---------------------------------------------------------------------------

/// Internal state for recursive include resolution.
struct IncludeProcessor {
    base_path: PathBuf,
    visited: HashSet<PathBuf>,
    depth: usize,
}

impl IncludeProcessor {
    fn new(base_path: &Path) -> Self {
        let normalized = normalize_path(base_path);
        let mut visited = HashSet::new();
        visited.insert(normalized.clone());
        Self {
            base_path: normalized,
            visited,
            depth: 0,
        }
    }

    fn process(&self, value: Value) -> Result<Value> {
        match value {
            Value::Array(arr) => {
                let mut result = Vec::with_capacity(arr.len());
                for item in arr {
                    result.push(self.process(item)?);
                }
                Ok(Value::Array(result))
            }
            Value::Object(ref map) => {
                if map.contains_key(INCLUDE_KEY) {
                    self.process_include(value)
                } else {
                    self.process_object(value)
                }
            }
            other => Ok(other),
        }
    }

    fn process_object(&self, value: Value) -> Result<Value> {
        let Value::Object(map) = value else {
            return Ok(value);
        };
        let mut result = serde_json::Map::new();
        for (key, val) in map {
            result.insert(key, self.process(val)?);
        }
        Ok(Value::Object(result))
    }

    fn process_include(&self, value: Value) -> Result<Value> {
        let Value::Object(map) = value else {
            return Ok(value);
        };

        let include_value = map
            .get(INCLUDE_KEY)
            .cloned()
            .unwrap_or(Value::Null);

        let other_keys: Vec<(String, Value)> = map
            .into_iter()
            .filter(|(k, _)| k != INCLUDE_KEY)
            .collect();

        let included = self.resolve_include(&include_value)?;

        if other_keys.is_empty() {
            return Ok(included);
        }

        // Sibling keys require included content to be an object
        if !included.is_object() {
            bail!(
                "Sibling keys require included content to be an object"
            );
        }

        // Build the "rest" object from sibling keys
        let mut rest = serde_json::Map::new();
        for (key, val) in other_keys {
            rest.insert(key, self.process(val)?);
        }

        Ok(deep_merge(included, Value::Object(rest)))
    }

    fn resolve_include(&self, value: &Value) -> Result<Value> {
        match value {
            Value::String(path) => self.load_file(path),
            Value::Array(arr) => {
                let mut merged = Value::Object(serde_json::Map::new());
                for item in arr {
                    let Value::String(path) = item else {
                        bail!(
                            "Invalid $include array item: expected string, got {}",
                            value_type_name(item)
                        );
                    };
                    let loaded = self.load_file(path)?;
                    merged = deep_merge(merged, loaded);
                }
                Ok(merged)
            }
            other => {
                bail!(
                    "Invalid $include value: expected string or array of strings, got {}",
                    value_type_name(other)
                );
            }
        }
    }

    fn load_file(&self, include_path: &str) -> Result<Value> {
        let resolved = self.resolve_path(include_path);

        self.check_circular(&resolved)?;
        self.check_depth(include_path)?;

        let raw = std::fs::read_to_string(&resolved)
            .with_context(|| {
                format!(
                    "Failed to read include file: {include_path} (resolved: {})",
                    resolved.display()
                )
            })?;

        let parsed: Value = json5::from_str(&raw)
            .with_context(|| {
                format!(
                    "Failed to parse include file: {include_path} (resolved: {})",
                    resolved.display()
                )
            })?;

        self.process_nested(&resolved, parsed)
    }

    fn resolve_path(&self, include_path: &str) -> PathBuf {
        let path = Path::new(include_path);
        if path.is_absolute() {
            normalize_path(path)
        } else {
            let parent = self
                .base_path
                .parent()
                .unwrap_or_else(|| Path::new("."));
            normalize_path(&parent.join(include_path))
        }
    }

    fn check_circular(&self, resolved: &Path) -> Result<()> {
        if self.visited.contains(resolved) {
            let chain: Vec<String> = self
                .visited
                .iter()
                .map(|p| p.display().to_string())
                .chain(std::iter::once(resolved.display().to_string()))
                .collect();
            bail!("Circular include detected: {}", chain.join(" -> "));
        }
        Ok(())
    }

    fn check_depth(&self, include_path: &str) -> Result<()> {
        if self.depth >= MAX_INCLUDE_DEPTH {
            bail!(
                "Maximum include depth ({MAX_INCLUDE_DEPTH}) exceeded at: {include_path}"
            );
        }
        Ok(())
    }

    fn process_nested(&self, resolved: &Path, parsed: Value) -> Result<Value> {
        let mut nested = IncludeProcessor {
            base_path: resolved.to_path_buf(),
            visited: {
                let mut v = self.visited.clone();
                v.insert(resolved.to_path_buf());
                v
            },
            depth: self.depth + 1,
        };
        let _ = &mut nested; // suppress unused mut warning
        nested.process(parsed)
    }
}

/// Normalize a path by cleaning up `.` and `..` components without requiring
/// the path to exist on disk (unlike `canonicalize`).
fn normalize_path(path: &Path) -> PathBuf {
    use std::path::Component;
    let mut components = Vec::new();
    for component in path.components() {
        match component {
            Component::ParentDir => {
                if !components.is_empty() {
                    components.pop();
                }
            }
            Component::CurDir => {}
            other => components.push(other),
        }
    }
    components.iter().collect()
}

/// Return a human-readable type name for a JSON value.
fn value_type_name(v: &Value) -> &'static str {
    match v {
        Value::Null => "null",
        Value::Bool(_) => "boolean",
        Value::Number(_) => "number",
        Value::String(_) => "string",
        Value::Array(_) => "array",
        Value::Object(_) => "object",
    }
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

/// Resolve all `$include` directives in a parsed config value.
///
/// Included files are resolved relative to `config_path`, parsed as JSON5,
/// and deep-merged into the parent object. Circular includes and excessive
/// nesting (> [`MAX_INCLUDE_DEPTH`]) are detected and reported as errors.
pub fn resolve_config_includes(obj: Value, config_path: &Path) -> Result<Value> {
    let processor = IncludeProcessor::new(config_path);
    processor.process(obj)
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn deep_merge_objects() {
        let a = json!({"x": 1, "y": {"a": 1}});
        let b = json!({"y": {"b": 2}, "z": 3});
        let merged = deep_merge(a, b);
        assert_eq!(merged, json!({"x": 1, "y": {"a": 1, "b": 2}, "z": 3}));
    }

    #[test]
    fn deep_merge_arrays() {
        let a = json!([1, 2]);
        let b = json!([3, 4]);
        let merged = deep_merge(a, b);
        assert_eq!(merged, json!([1, 2, 3, 4]));
    }

    #[test]
    fn deep_merge_primitive_source_wins() {
        let a = json!(42);
        let b = json!("hello");
        let merged = deep_merge(a, b);
        assert_eq!(merged, json!("hello"));
    }

    #[test]
    fn no_includes_passthrough() {
        let val = json!({"key": "value", "nested": {"a": 1}});
        let result = resolve_config_includes(val.clone(), Path::new("/tmp/test.json"));
        assert!(result.is_ok());
        assert_eq!(result.unwrap(), val);
    }

    #[test]
    fn invalid_include_type() {
        let val = json!({"$include": 42});
        let result = resolve_config_includes(val, Path::new("/tmp/test.json"));
        assert!(result.is_err());
        assert!(result.unwrap_err().to_string().contains("Invalid $include value"));
    }
}
