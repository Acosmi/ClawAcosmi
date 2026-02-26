/// Queue configuration types.
///
/// Source: `src/config/types.queue.ts`

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum QueueMode {
    Steer,
    Followup,
    Collect,
    SteerBacklog,
    #[serde(rename = "steer+backlog")]
    SteerPlusBacklog,
    Queue,
    Interrupt,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum QueueDropPolicy {
    Old,
    New,
    Summarize,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct QueueModeByProvider {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub whatsapp: Option<QueueMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub telegram: Option<QueueMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub discord: Option<QueueMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub googlechat: Option<QueueMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub slack: Option<QueueMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub signal: Option<QueueMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub imessage: Option<QueueMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub msteams: Option<QueueMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub webchat: Option<QueueMode>,
}
