/// Set the default model.
///
/// Source: `src/commands/models/set.ts`

use anyhow::Result;

use oa_config::io::load_config;

use crate::shared::{resolve_model_target, update_config};

/// Set the primary default model.
///
/// Resolves the raw model reference, ensures it's in the allowlist,
/// and writes the updated config.
///
/// Source: `src/commands/models/set.ts` - `modelsSetCommand`
pub async fn models_set_command(model_raw: &str) -> Result<String> {
    let cfg_snapshot = load_config()?;
    let resolved = resolve_model_target(model_raw, &cfg_snapshot)?;
    let key = format!("{}/{}", resolved.provider, resolved.model);

    let updated = update_config(|cfg| {
        let mut next = cfg.clone();
        let agents = next.agents.get_or_insert_with(Default::default);
        let defaults = agents.defaults.get_or_insert_with(Default::default);
        let models = defaults.models.get_or_insert_with(Default::default);
        models.entry(key.clone()).or_default();

        // Preserve existing fallbacks
        let existing_fallbacks = defaults
            .model
            .as_ref()
            .and_then(|m| m.fallbacks.clone());
        let model_config = defaults.model.get_or_insert_with(Default::default);
        model_config.primary = Some(key.clone());
        if let Some(fallbacks) = existing_fallbacks {
            model_config.fallbacks = Some(fallbacks);
        }

        Ok(next)
    })
    .await?;

    let display_key = updated
        .agents
        .as_ref()
        .and_then(|a| a.defaults.as_ref())
        .and_then(|d| d.model.as_ref())
        .and_then(|m| m.primary.clone())
        .unwrap_or_else(|| model_raw.to_owned());

    Ok(format!("Default model: {display_key}"))
}

#[cfg(test)]
mod tests {
    use super::*;
    use oa_agents::model_selection::parse_model_ref;

    #[test]
    fn resolve_target_works_for_full_ref() {
        let cfg = oa_types::config::OpenAcosmiConfig::default();
        let result = resolve_model_target("anthropic/claude-opus-4-6", &cfg);
        assert!(result.is_ok());
        let model_ref = result.expect("should resolve");
        assert_eq!(model_ref.provider, "anthropic");
        assert_eq!(model_ref.model, "claude-opus-4-6");
    }

    #[test]
    fn resolve_target_works_for_short_ref() {
        let cfg = oa_types::config::OpenAcosmiConfig::default();
        let result = resolve_model_target("gpt-4o", &cfg);
        assert!(result.is_ok());
    }

    #[test]
    fn parse_model_ref_validates() {
        assert!(parse_model_ref("", "anthropic").is_none());
        assert!(parse_model_ref("anthropic/claude-opus-4-6", "anthropic").is_some());
    }
}
