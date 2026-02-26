/// Plugin-based provider auth choice handler.
///
/// Handles auth choices that delegate to a plugin provider for the actual
/// credential acquisition (OAuth, device flow, etc.). This includes
/// Google Antigravity, Google Gemini CLI, Copilot Proxy, Qwen Portal,
/// and MiniMax Portal.
///
/// Source: `src/commands/auth-choice.apply.plugin-provider.ts`

use serde_json::Value;

use oa_types::config::OpenAcosmiConfig;

/// Options for a plugin-based auth choice.
///
/// Source: `src/commands/auth-choice.apply.plugin-provider.ts` - `PluginProviderAuthChoiceOptions`
#[derive(Debug, Clone)]
pub struct PluginProviderAuthChoiceOptions {
    /// The auth choice string identifier.
    pub auth_choice: String,
    /// The plugin identifier.
    pub plugin_id: String,
    /// The provider identifier.
    pub provider_id: String,
    /// The auth method identifier within the provider.
    pub method_id: Option<String>,
    /// Human-readable label for display.
    pub label: String,
}

/// Check if a value is a plain JSON object (not an array).
///
/// Source: `src/commands/auth-choice.apply.plugin-provider.ts` - `isPlainRecord`
#[must_use]
pub fn is_plain_record(value: &Value) -> bool {
    value.is_object()
}

/// Deep-merge a config patch into a base configuration.
///
/// Recursively merges objects; non-object values in the patch overwrite
/// the base value.
///
/// Source: `src/commands/auth-choice.apply.plugin-provider.ts` - `mergeConfigPatch`
#[must_use]
pub fn merge_config_patch(base: &Value, patch: &Value) -> Value {
    if let (Some(base_obj), Some(patch_obj)) = (base.as_object(), patch.as_object()) {
        let mut merged = base_obj.clone();
        for (key, patch_value) in patch_obj {
            let existing = merged.get(key);
            if let Some(existing_value) = existing {
                if existing_value.is_object() && patch_value.is_object() {
                    merged.insert(key.clone(), merge_config_patch(existing_value, patch_value));
                } else {
                    merged.insert(key.clone(), patch_value.clone());
                }
            } else {
                merged.insert(key.clone(), patch_value.clone());
            }
        }
        Value::Object(merged)
    } else {
        patch.clone()
    }
}

/// Apply a default model to the configuration.
///
/// Sets `agents.defaults.model.primary` and ensures the model is in
/// the allowlist.
///
/// Source: `src/commands/auth-choice.apply.plugin-provider.ts` - `applyDefaultModel`
#[must_use]
pub fn apply_default_model(config: OpenAcosmiConfig, model: &str) -> OpenAcosmiConfig {
    crate::default_model::set_default_model_primary(config, model)
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn is_plain_record_object() {
        assert!(is_plain_record(&json!({"key": "value"})));
    }

    #[test]
    fn is_plain_record_not_array() {
        assert!(!is_plain_record(&json!([1, 2, 3])));
    }

    #[test]
    fn is_plain_record_not_string() {
        assert!(!is_plain_record(&json!("hello")));
    }

    #[test]
    fn merge_config_patch_simple() {
        let base = json!({"a": 1, "b": 2});
        let patch = json!({"b": 3, "c": 4});
        let result = merge_config_patch(&base, &patch);
        assert_eq!(result, json!({"a": 1, "b": 3, "c": 4}));
    }

    #[test]
    fn merge_config_patch_nested() {
        let base = json!({"a": {"x": 1, "y": 2}});
        let patch = json!({"a": {"y": 3, "z": 4}});
        let result = merge_config_patch(&base, &patch);
        assert_eq!(result, json!({"a": {"x": 1, "y": 3, "z": 4}}));
    }

    #[test]
    fn merge_config_patch_overwrite_non_object() {
        let base = json!({"a": "string"});
        let patch = json!({"a": {"nested": true}});
        let result = merge_config_patch(&base, &patch);
        assert_eq!(result, json!({"a": {"nested": true}}));
    }

    #[test]
    fn merge_config_patch_non_object_base() {
        let base = json!("not an object");
        let patch = json!({"a": 1});
        let result = merge_config_patch(&base, &patch);
        assert_eq!(result, json!({"a": 1}));
    }

    #[test]
    fn apply_default_model_sets_primary() {
        let config = OpenAcosmiConfig::default();
        let updated = apply_default_model(config, "google/gemini-2.5-pro");
        let primary = updated.agents.as_ref()
            .and_then(|a| a.defaults.as_ref())
            .and_then(|d| d.model.as_ref())
            .and_then(|m| m.primary.as_deref());
        assert_eq!(primary, Some("google/gemini-2.5-pro"));
    }
}
