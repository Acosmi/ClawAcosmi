/// Legacy config value normalization.
///
/// Migrates deprecated `messages.ackReaction` / `messages.ackReactionScope`
/// into the per-channel `channels.whatsapp.ackReaction` structure when the
/// WhatsApp channel is configured but does not yet have its own ack config.
///
/// Source: `src/commands/doctor-legacy-config.ts`

use oa_types::config::OpenAcosmiConfig;

/// Result of normalizing legacy config values.
///
/// Source: `src/commands/doctor-legacy-config.ts` — return of `normalizeLegacyConfigValues`
pub struct NormalizeLegacyResult {
    /// The (possibly updated) config.
    pub config: OpenAcosmiConfig,
    /// Human-readable descriptions of what changed.
    pub changes: Vec<String>,
}

/// Normalize deprecated config values into their current equivalents.
///
/// Currently handles:
/// - `messages.ackReaction` + `messages.ackReactionScope` → `channels.whatsapp.ackReaction`
///
/// Source: `src/commands/doctor-legacy-config.ts` — `normalizeLegacyConfigValues`
pub fn normalize_legacy_config_values(cfg: &OpenAcosmiConfig) -> NormalizeLegacyResult {
    let mut changes = Vec::new();
    let mut next = cfg.clone();

    // ── ackReaction migration ──
    let legacy_ack = cfg
        .messages
        .as_ref()
        .and_then(|m| m.ack_reaction.as_ref())
        .map(|s| s.trim().to_string())
        .filter(|s| !s.is_empty());

    let has_whatsapp = cfg.channels.as_ref().and_then(|c| c.whatsapp.as_ref()).is_some();

    if let Some(legacy_ack_emoji) = legacy_ack {
        if has_whatsapp {
            // Check if the WhatsApp channel already has an ackReaction.
            let whatsapp_has_ack = cfg
                .channels
                .as_ref()
                .and_then(|c| c.whatsapp.as_ref())
                .and_then(|w| w.get("ackReaction"))
                .is_some();

            if !whatsapp_has_ack {
                let legacy_scope = cfg
                    .messages
                    .as_ref()
                    .and_then(|m| m.ack_reaction_scope.as_ref());

                let scope_str = legacy_scope
                    .map(|s| format!("{s:?}"))
                    .unwrap_or_else(|| "group-mentions".to_string());

                let (direct, group) = match scope_str.as_str() {
                    "All" => (true, "always"),
                    "Direct" => (true, "never"),
                    "GroupAll" => (false, "always"),
                    _ => (false, "mentions"), // GroupMentions is the default
                };

                // Build the ackReaction value.
                let ack_value = serde_json::json!({
                    "emoji": legacy_ack_emoji,
                    "direct": direct,
                    "group": group,
                });

                // Merge into the WhatsApp channel config.
                if let Some(ref mut channels) = next.channels {
                    if let Some(ref mut wa) = channels.whatsapp {
                        if let Some(obj) = wa.as_object_mut() {
                            obj.insert("ackReaction".to_string(), ack_value);
                        }
                    }
                }

                changes.push(format!(
                    "Copied messages.ackReaction -> channels.whatsapp.ackReaction (scope: {scope_str})."
                ));
            }
        }
    }

    NormalizeLegacyResult {
        config: next,
        changes,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn noop_when_no_legacy_ack() {
        let cfg = OpenAcosmiConfig::default();
        let result = normalize_legacy_config_values(&cfg);
        assert!(result.changes.is_empty());
    }

    #[test]
    fn noop_when_no_whatsapp() {
        let mut cfg = OpenAcosmiConfig::default();
        cfg.messages = Some(oa_types::messages::MessagesConfig {
            ack_reaction: Some("thumbsup".to_string()),
            ..Default::default()
        });
        // No whatsapp channel configured.
        let result = normalize_legacy_config_values(&cfg);
        assert!(result.changes.is_empty());
    }

    #[test]
    fn migrates_ack_reaction_to_whatsapp() {
        let mut cfg = OpenAcosmiConfig::default();
        cfg.messages = Some(oa_types::messages::MessagesConfig {
            ack_reaction: Some("thumbsup".to_string()),
            ..Default::default()
        });
        cfg.channels = Some(oa_types::channels::ChannelsConfig {
            whatsapp: Some(serde_json::json!({"enabled": true})),
            ..Default::default()
        });

        let result = normalize_legacy_config_values(&cfg);
        assert_eq!(result.changes.len(), 1);
        assert!(result.changes[0].contains("ackReaction"));

        // Verify the ackReaction was set.
        let wa = result
            .config
            .channels
            .as_ref()
            .and_then(|c| c.whatsapp.as_ref())
            .and_then(|w| w.get("ackReaction"));
        assert!(wa.is_some());
    }

    #[test]
    fn noop_when_whatsapp_already_has_ack() {
        let mut cfg = OpenAcosmiConfig::default();
        cfg.messages = Some(oa_types::messages::MessagesConfig {
            ack_reaction: Some("thumbsup".to_string()),
            ..Default::default()
        });
        cfg.channels = Some(oa_types::channels::ChannelsConfig {
            whatsapp: Some(serde_json::json!({
                "enabled": true,
                "ackReaction": {"emoji": "already-set"}
            })),
            ..Default::default()
        });

        let result = normalize_legacy_config_values(&cfg);
        assert!(result.changes.is_empty());
    }
}
