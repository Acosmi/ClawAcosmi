/// Set the default image model.
///
/// Source: `src/commands/models/set-image.ts`

use anyhow::Result;

use oa_config::io::load_config;

use crate::shared::{resolve_model_target, update_config};

/// Set the primary image model.
///
/// Resolves the raw model reference, ensures it's in the allowlist,
/// and writes the updated config.
///
/// Source: `src/commands/models/set-image.ts` - `modelsSetImageCommand`
pub async fn models_set_image_command(model_raw: &str) -> Result<String> {
    let cfg_snapshot = load_config()?;
    let resolved = resolve_model_target(model_raw, &cfg_snapshot)?;
    let key = format!("{}/{}", resolved.provider, resolved.model);

    let updated = update_config(|cfg| {
        let mut next = cfg.clone();
        let agents = next.agents.get_or_insert_with(Default::default);
        let defaults = agents.defaults.get_or_insert_with(Default::default);
        let models = defaults.models.get_or_insert_with(Default::default);
        models.entry(key.clone()).or_default();

        // Preserve existing image fallbacks
        let existing_fallbacks = defaults
            .image_model
            .as_ref()
            .and_then(|m| m.fallbacks.clone());
        let image_model_config = defaults.image_model.get_or_insert_with(Default::default);
        image_model_config.primary = Some(key.clone());
        if let Some(fallbacks) = existing_fallbacks {
            image_model_config.fallbacks = Some(fallbacks);
        }

        Ok(next)
    })
    .await?;

    let display_key = updated
        .agents
        .as_ref()
        .and_then(|a| a.defaults.as_ref())
        .and_then(|d| d.image_model.as_ref())
        .and_then(|m| m.primary.clone())
        .unwrap_or_else(|| model_raw.to_owned());

    Ok(format!("Image model: {display_key}"))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn resolve_image_target_works() {
        let cfg = oa_types::config::OpenAcosmiConfig::default();
        let result = resolve_model_target("openai/gpt-4o", &cfg);
        assert!(result.is_ok());
        let model_ref = result.expect("should resolve");
        assert_eq!(model_ref.provider, "openai");
        assert_eq!(model_ref.model, "gpt-4o");
    }
}
