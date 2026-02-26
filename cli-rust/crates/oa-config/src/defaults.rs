/// Default configuration value application.
///
/// Applies sensible defaults to various sections of the OpenAcosmi config.
/// These are intentionally kept as pass-through stubs for now, as the full
/// logic depends on many other modules (model selection, agent limits, etc.).
///
/// Source: `src/config/defaults.ts`

use oa_types::config::OpenAcosmiConfig;

/// Apply session defaults to the configuration.
///
/// Currently a pass-through stub. The full implementation normalizes
/// `session.mainKey` and warns on non-standard values.
pub fn apply_session_defaults(cfg: OpenAcosmiConfig) -> OpenAcosmiConfig {
    cfg
}

/// Apply agent defaults to the configuration.
///
/// Currently a pass-through stub. The full implementation sets
/// `agents.defaults.maxConcurrent` and `agents.defaults.subagents.maxConcurrent`
/// when not explicitly configured.
pub fn apply_agent_defaults(cfg: OpenAcosmiConfig) -> OpenAcosmiConfig {
    cfg
}

/// Apply model defaults to the configuration.
///
/// Currently a pass-through stub. The full implementation normalizes
/// model definitions (reasoning, input, cost, context window, max tokens)
/// and applies default model aliases.
pub fn apply_model_defaults(cfg: OpenAcosmiConfig) -> OpenAcosmiConfig {
    cfg
}

/// Apply message defaults to the configuration.
///
/// Currently a pass-through stub. The full implementation sets
/// `messages.ackReactionScope` to `"group-mentions"` when not specified.
pub fn apply_message_defaults(cfg: OpenAcosmiConfig) -> OpenAcosmiConfig {
    cfg
}

/// Apply logging defaults to the configuration.
///
/// Currently a pass-through stub. The full implementation sets
/// `logging.redactSensitive` to `"tools"` when logging is configured
/// but `redactSensitive` is not set.
pub fn apply_logging_defaults(cfg: OpenAcosmiConfig) -> OpenAcosmiConfig {
    cfg
}

/// Apply compaction defaults to the configuration.
///
/// Currently a pass-through stub. The full implementation sets
/// `agents.defaults.compaction.mode` to `"safeguard"` when not specified.
pub fn apply_compaction_defaults(cfg: OpenAcosmiConfig) -> OpenAcosmiConfig {
    cfg
}

/// Apply context pruning defaults to the configuration.
///
/// Currently a pass-through stub. The full implementation configures
/// context pruning mode, TTL, heartbeat intervals, and cache retention
/// based on the detected authentication mode.
pub fn apply_context_pruning_defaults(cfg: OpenAcosmiConfig) -> OpenAcosmiConfig {
    cfg
}
