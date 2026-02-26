/// Health check types.
///
/// Source: `src/commands/health.ts` types

use serde::{Deserialize, Serialize};

/// Health check result.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct HealthResult {
    /// Whether the system is healthy
    pub healthy: bool,
    /// Optional message
    #[serde(default)]
    pub message: Option<String>,
}
