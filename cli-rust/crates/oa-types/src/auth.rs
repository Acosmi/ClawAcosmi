/// Authentication configuration types.
///
/// Source: `src/config/types.auth.ts`

use std::collections::HashMap;

use serde::{Deserialize, Serialize};

/// Credential type expected in auth-profiles.json for a profile id.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum AuthProfileMode {
    ApiKey,
    Oauth,
    Token,
}

/// A single auth profile configuration.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AuthProfileConfig {
    pub provider: String,
    pub mode: AuthProfileMode,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub email: Option<String>,
}

/// Cooldown settings for billing/failure backoff.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct AuthCooldownsConfig {
    /// Default billing backoff (hours). Default: 5.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub billing_backoff_hours: Option<f64>,
    /// Optional per-provider billing backoff (hours).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub billing_backoff_hours_by_provider: Option<HashMap<String, f64>>,
    /// Billing backoff cap (hours). Default: 24.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub billing_max_hours: Option<f64>,
    /// Failure window for backoff counters (hours). Default: 24.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub failure_window_hours: Option<f64>,
}

/// Top-level auth configuration.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct AuthConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub profiles: Option<HashMap<String, AuthProfileConfig>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub order: Option<HashMap<String, Vec<String>>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cooldowns: Option<AuthCooldownsConfig>,
}
