/// Agent result delivery.
///
/// After an agent run completes, this module handles formatting and
/// delivering the result payloads. Payloads can be logged to the
/// terminal (default), output as JSON, or delivered to an outbound
/// channel (WhatsApp, Telegram, etc.).
///
/// Source: `src/commands/agent/delivery.ts`

use serde::{Deserialize, Serialize};

/// Nested agent log line prefix.
///
/// Source: `src/commands/agent/delivery.ts` - `NESTED_LOG_PREFIX`
pub const NESTED_LOG_PREFIX: &str = "[agent:nested]";

/// Agent lane identifier for nested (subagent) runs.
///
/// Source: `src/agents/lanes.ts` - `AGENT_LANE_NESTED`
pub const AGENT_LANE_NESTED: &str = "nested";

/// Normalized outbound payload for delivery or logging.
///
/// Source: `src/infra/outbound/payloads.ts` - `NormalizedOutboundPayload`
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct NormalizedOutboundPayload {
    /// Text content.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub text: Option<String>,
    /// Media URLs.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub media_urls: Option<Vec<String>>,
}

/// Result envelope returned from delivery.
///
/// Source: `src/infra/outbound/envelope.ts` - `OutboundResultEnvelope`
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct OutboundResultEnvelope {
    /// Normalized payloads.
    pub payloads: Vec<NormalizedOutboundPayload>,
    /// Agent run metadata.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub meta: Option<serde_json::Value>,
}

/// Format the nested log prefix for a subagent run.
///
/// Includes session, run, channel, and target information when available.
///
/// Source: `src/commands/agent/delivery.ts` - `formatNestedLogPrefix`
pub fn format_nested_log_prefix(
    session: Option<&str>,
    run_id: Option<&str>,
    channel: Option<&str>,
    to: Option<&str>,
    account_id: Option<&str>,
) -> String {
    let mut parts = vec![NESTED_LOG_PREFIX.to_owned()];
    if let Some(s) = session {
        if !s.is_empty() {
            parts.push(format!("session={s}"));
        }
    }
    if let Some(r) = run_id {
        if !r.is_empty() {
            parts.push(format!("run={r}"));
        }
    }
    if let Some(c) = channel {
        if !c.is_empty() {
            parts.push(format!("channel={c}"));
        }
    }
    if let Some(t) = to {
        if !t.is_empty() {
            parts.push(format!("to={t}"));
        }
    }
    if let Some(a) = account_id {
        if !a.is_empty() {
            parts.push(format!("account={a}"));
        }
    }
    parts.join(" ")
}

/// Format a payload for human-readable log output.
///
/// Outputs the text content followed by media URLs prefixed with `MEDIA:`.
///
/// Source: `src/infra/outbound/payloads.ts` - `formatOutboundPayloadLog`
pub fn format_outbound_payload_log(payload: &NormalizedOutboundPayload) -> Option<String> {
    let mut lines: Vec<String> = Vec::new();

    if let Some(ref text) = payload.text {
        let trimmed = text.trim_end();
        if !trimmed.is_empty() {
            lines.push(trimmed.to_owned());
        }
    }

    if let Some(ref urls) = payload.media_urls {
        for url in urls {
            let trimmed = url.trim();
            if !trimmed.is_empty() {
                lines.push(format!("MEDIA:{trimmed}"));
            }
        }
    }

    if lines.is_empty() {
        None
    } else {
        Some(lines.join("\n").trim_end().to_owned())
    }
}

/// Normalize raw payloads into the standard outbound format.
///
/// Source: `src/infra/outbound/payloads.ts` - `normalizeOutboundPayloads`
pub fn normalize_outbound_payloads(
    raw: &[serde_json::Value],
) -> Vec<NormalizedOutboundPayload> {
    raw.iter()
        .map(|v| {
            let text = v
                .get("text")
                .and_then(serde_json::Value::as_str)
                .map(String::from);
            let media_url = v
                .get("mediaUrl")
                .and_then(serde_json::Value::as_str)
                .map(|s| s.trim().to_owned())
                .filter(|s| !s.is_empty());
            let media_urls_array = v
                .get("mediaUrls")
                .and_then(serde_json::Value::as_array)
                .map(|arr| {
                    arr.iter()
                        .filter_map(serde_json::Value::as_str)
                        .map(String::from)
                        .collect::<Vec<_>>()
                });
            let all_media = media_urls_array
                .or_else(|| media_url.map(|u| vec![u]));

            NormalizedOutboundPayload {
                text,
                media_urls: all_media,
            }
        })
        .collect()
}

/// Build the JSON result envelope.
///
/// Source: `src/infra/outbound/envelope.ts` - `buildOutboundResultEnvelope`
pub fn build_outbound_result_envelope(
    payloads: Vec<NormalizedOutboundPayload>,
    meta: Option<serde_json::Value>,
) -> OutboundResultEnvelope {
    OutboundResultEnvelope { payloads, meta }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn nested_log_prefix_minimal() {
        let prefix = format_nested_log_prefix(None, None, None, None, None);
        assert_eq!(prefix, NESTED_LOG_PREFIX);
    }

    #[test]
    fn nested_log_prefix_full() {
        let prefix = format_nested_log_prefix(
            Some("agent:bot:main"),
            Some("run-1"),
            Some("whatsapp"),
            Some("+15551234567"),
            Some("acct-1"),
        );
        assert!(prefix.starts_with(NESTED_LOG_PREFIX));
        assert!(prefix.contains("session=agent:bot:main"));
        assert!(prefix.contains("run=run-1"));
        assert!(prefix.contains("channel=whatsapp"));
        assert!(prefix.contains("to=+15551234567"));
        assert!(prefix.contains("account=acct-1"));
    }

    #[test]
    fn format_payload_text_only() {
        let payload = NormalizedOutboundPayload {
            text: Some("Hello world".to_owned()),
            media_urls: None,
        };
        assert_eq!(
            format_outbound_payload_log(&payload),
            Some("Hello world".to_owned())
        );
    }

    #[test]
    fn format_payload_with_media() {
        let payload = NormalizedOutboundPayload {
            text: Some("Check this out".to_owned()),
            media_urls: Some(vec!["https://example.com/img.png".to_owned()]),
        };
        let result = format_outbound_payload_log(&payload);
        assert!(result.is_some());
        let output = result.unwrap_or_default();
        assert!(output.contains("Check this out"));
        assert!(output.contains("MEDIA:https://example.com/img.png"));
    }

    #[test]
    fn format_payload_empty() {
        let payload = NormalizedOutboundPayload::default();
        assert!(format_outbound_payload_log(&payload).is_none());
    }

    #[test]
    fn normalize_payloads_from_json() {
        let raw = vec![serde_json::json!({
            "text": "hi",
            "mediaUrls": ["https://a.com/1.png"]
        })];
        let normalized = normalize_outbound_payloads(&raw);
        assert_eq!(normalized.len(), 1);
        assert_eq!(normalized[0].text.as_deref(), Some("hi"));
        assert_eq!(
            normalized[0].media_urls,
            Some(vec!["https://a.com/1.png".to_owned()])
        );
    }

    #[test]
    fn normalize_payloads_single_media_url() {
        let raw = vec![serde_json::json!({
            "text": "photo",
            "mediaUrl": "https://a.com/photo.jpg"
        })];
        let normalized = normalize_outbound_payloads(&raw);
        assert_eq!(normalized.len(), 1);
        assert_eq!(
            normalized[0].media_urls,
            Some(vec!["https://a.com/photo.jpg".to_owned()])
        );
    }

    #[test]
    fn build_envelope() {
        let payloads = vec![NormalizedOutboundPayload {
            text: Some("done".to_owned()),
            media_urls: None,
        }];
        let envelope = build_outbound_result_envelope(payloads, None);
        assert_eq!(envelope.payloads.len(), 1);
        assert!(envelope.meta.is_none());
    }

    #[test]
    fn envelope_serializes_camel_case() {
        let envelope = OutboundResultEnvelope {
            payloads: vec![NormalizedOutboundPayload {
                text: Some("test".to_owned()),
                media_urls: Some(vec!["https://x.com/img.png".to_owned()]),
            }],
            meta: None,
        };
        let json = serde_json::to_string(&envelope).expect("should serialize");
        assert!(json.contains("\"mediaUrls\""));
    }
}
