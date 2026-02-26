/// Agent command types: options, run context, image content, and stream params.
///
/// Source: `src/commands/agent/types.ts`

use serde::{Deserialize, Serialize};

/// Image content block for multimodal messages.
///
/// Source: `src/commands/agent/types.ts` - `ImageContent`
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ImageContent {
    /// Content type discriminant.
    pub r#type: String,
    /// Base64-encoded image data.
    pub data: String,
    /// MIME type of the image.
    pub mime_type: String,
}

/// Per-call stream parameter overrides (best-effort).
///
/// Source: `src/commands/agent/types.ts` - `AgentStreamParams`
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AgentStreamParams {
    /// Provider temperature override.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub temperature: Option<f64>,
    /// Provider max tokens override.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_tokens: Option<u64>,
}

/// Agent run context for embedded execution routing (channel, account, thread).
///
/// Source: `src/commands/agent/types.ts` - `AgentRunContext`
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AgentRunContext {
    /// Message channel context (webchat, voicewake, whatsapp, etc.).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub message_channel: Option<String>,
    /// Account identifier for multi-account routing.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub account_id: Option<String>,
    /// Group identifier for channel-level tool policy resolution.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub group_id: Option<String>,
    /// Group channel label for channel-level tool policy resolution.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub group_channel: Option<String>,
    /// Group space label for channel-level tool policy resolution.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub group_space: Option<String>,
    /// Current channel identifier for delivery context resolution.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub current_channel_id: Option<String>,
    /// Current thread timestamp (Slack-style).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub current_thread_ts: Option<String>,
    /// Reply-to mode: off, first, or all.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub reply_to_mode: Option<String>,
}

/// Full set of options for the agent command.
///
/// Source: `src/commands/agent/types.ts` - `AgentCommandOpts`
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AgentCommandOpts {
    /// The message body to send to the agent.
    pub message: String,
    /// Optional image attachments for multimodal messages.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub images: Option<Vec<ImageContent>>,
    /// Agent id override (must exist in config).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub agent_id: Option<String>,
    /// Delivery target phone number / address.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub to: Option<String>,
    /// Explicit session identifier.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub session_id: Option<String>,
    /// Explicit session key.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub session_key: Option<String>,
    /// Thinking level override (off, minimal, low, medium, high, xhigh).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub thinking: Option<String>,
    /// One-shot thinking level override (resets after this run).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub thinking_once: Option<String>,
    /// Verbose level override (on, full, off).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub verbose: Option<String>,
    /// Output as JSON.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub json: Option<bool>,
    /// Timeout in seconds.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timeout: Option<String>,
    /// Whether to deliver the result to the channel.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub deliver: Option<bool>,
    /// Override delivery target.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub reply_to: Option<String>,
    /// Override delivery channel.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub reply_channel: Option<String>,
    /// Override delivery account id.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub reply_account_id: Option<String>,
    /// Override delivery thread/topic id.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub thread_id: Option<String>,
    /// Message channel context (webchat, voicewake, whatsapp, etc.).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub message_channel: Option<String>,
    /// Delivery channel (whatsapp, telegram, etc.).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub channel: Option<String>,
    /// Account ID for multi-account channel routing.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub account_id: Option<String>,
    /// Context for embedded run routing.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub run_context: Option<AgentRunContext>,
    /// Group id for channel-level tool policy resolution.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub group_id: Option<String>,
    /// Group channel label for channel-level tool policy resolution.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub group_channel: Option<String>,
    /// Group space label for channel-level tool policy resolution.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub group_space: Option<String>,
    /// Parent session key for subagent policy inheritance.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub spawned_by: Option<String>,
    /// Delivery target mode (explicit or implicit).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub delivery_target_mode: Option<String>,
    /// Best-effort delivery (log errors instead of failing).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub best_effort_deliver: Option<bool>,
    /// Agent lane (e.g., "nested" for subagent runs).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub lane: Option<String>,
    /// Run identifier for idempotency.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub run_id: Option<String>,
    /// Extra system prompt to inject.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub extra_system_prompt: Option<String>,
    /// Per-call stream param overrides (best-effort).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub stream_params: Option<AgentStreamParams>,
}

/// CLI-level agent options (simplified for direct CLI invocation).
///
/// Source: `src/commands/agent-via-gateway.ts` - `AgentCliOpts`
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AgentCliOpts {
    /// The message body.
    pub message: String,
    /// Agent id override.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub agent: Option<String>,
    /// Delivery target phone number / address.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub to: Option<String>,
    /// Explicit session identifier.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub session_id: Option<String>,
    /// Thinking level override.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub thinking: Option<String>,
    /// Verbose level override.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub verbose: Option<String>,
    /// Output as JSON.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub json: Option<bool>,
    /// Timeout in seconds.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timeout: Option<String>,
    /// Whether to deliver the result.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub deliver: Option<bool>,
    /// Delivery channel.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub channel: Option<String>,
    /// Reply-to target.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub reply_to: Option<String>,
    /// Reply channel.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub reply_channel: Option<String>,
    /// Reply account.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub reply_account: Option<String>,
    /// Best-effort delivery.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub best_effort_deliver: Option<bool>,
    /// Agent lane.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub lane: Option<String>,
    /// Run id for idempotency.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub run_id: Option<String>,
    /// Extra system prompt.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub extra_system_prompt: Option<String>,
    /// Force local execution (skip gateway).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub local: Option<bool>,
}

/// Payload in a gateway agent response.
///
/// Source: `src/commands/agent-via-gateway.ts` - `AgentGatewayResult`
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AgentGatewayPayload {
    /// Text content of the payload.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub text: Option<String>,
    /// Single media URL (legacy).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub media_url: Option<String>,
    /// List of media URLs.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub media_urls: Option<Vec<String>>,
}

/// Result payload from a gateway agent call.
///
/// Source: `src/commands/agent-via-gateway.ts` - `AgentGatewayResult`
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AgentGatewayResult {
    /// Output payloads.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub payloads: Option<Vec<AgentGatewayPayload>>,
    /// Run metadata.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub meta: Option<serde_json::Value>,
}

/// Full gateway agent response envelope.
///
/// Source: `src/commands/agent-via-gateway.ts` - `GatewayAgentResponse`
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct GatewayAgentResponse {
    /// Run identifier.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub run_id: Option<String>,
    /// Status string.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub status: Option<String>,
    /// Summary text.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub summary: Option<String>,
    /// Agent result payload.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub result: Option<AgentGatewayResult>,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn agent_command_opts_default_has_empty_message() {
        let opts = AgentCommandOpts::default();
        assert!(opts.message.is_empty());
        assert!(opts.agent_id.is_none());
        assert!(opts.to.is_none());
    }

    #[test]
    fn agent_cli_opts_serializes_camel_case() {
        let opts = AgentCliOpts {
            message: "hello".to_owned(),
            best_effort_deliver: Some(true),
            ..Default::default()
        };
        let json = serde_json::to_string(&opts).expect("should serialize");
        assert!(json.contains("\"bestEffortDeliver\":true"));
        assert!(json.contains("\"message\":\"hello\""));
    }

    #[test]
    fn gateway_response_deserializes() {
        let raw = r#"{"runId":"abc","status":"ok","summary":"done"}"#;
        let resp: GatewayAgentResponse =
            serde_json::from_str(raw).expect("should deserialize");
        assert_eq!(resp.run_id.as_deref(), Some("abc"));
        assert_eq!(resp.status.as_deref(), Some("ok"));
        assert_eq!(resp.summary.as_deref(), Some("done"));
    }

    #[test]
    fn gateway_payload_with_media() {
        let raw = r#"{"text":"hi","mediaUrls":["https://a.com/img.png"]}"#;
        let payload: AgentGatewayPayload =
            serde_json::from_str(raw).expect("should deserialize");
        assert_eq!(payload.text.as_deref(), Some("hi"));
        assert_eq!(
            payload.media_urls,
            Some(vec!["https://a.com/img.png".to_owned()])
        );
    }

    #[test]
    fn image_content_serializes() {
        let img = ImageContent {
            r#type: "image".to_owned(),
            data: "base64data".to_owned(),
            mime_type: "image/png".to_owned(),
        };
        let json = serde_json::to_string(&img).expect("should serialize");
        assert!(json.contains("\"mimeType\":\"image/png\""));
    }

    #[test]
    fn agent_stream_params_default() {
        let p = AgentStreamParams::default();
        assert!(p.temperature.is_none());
        assert!(p.max_tokens.is_none());
    }
}
