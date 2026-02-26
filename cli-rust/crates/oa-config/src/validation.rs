/// Configuration validation.
///
/// Validates a raw JSON value against the `OpenAcosmiConfig` schema.
/// Currently implemented as a simple deserialization stub; full validation
/// with plugin support will be added in a later pass.
///
/// Source: `src/config/validation.ts`

use anyhow::{Context, Result};
use serde_json::Value;

use oa_types::config::OpenAcosmiConfig;

/// Validate a raw JSON value and deserialize it into an `OpenAcosmiConfig`.
///
/// Currently performs only structural deserialization. The full implementation
/// will include plugin-based validation, field-level checks, and warning
/// collection.
pub fn validate_config_object(raw: &Value) -> Result<OpenAcosmiConfig> {
    serde_json::from_value(raw.clone()).context("Config validation failed: invalid structure")
}
