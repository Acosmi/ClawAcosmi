/// Common types shared across modules.
///
/// Source: `src/channels/chat-type.ts` and shared utility types.

use serde::{Deserialize, Serialize};

/// Standard JSON output wrapper for `--json` flag.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct JsonOutput<T: Serialize> {
    /// Whether the operation succeeded
    pub success: bool,
    /// The data payload
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub data: Option<T>,
    /// Error message if failed
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}

/// Chat type enum.
///
/// Source: `src/channels/chat-type.ts`
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq, Hash)]
#[serde(rename_all = "lowercase")]
pub enum ChatType {
    Direct,
    Group,
    Channel,
}
