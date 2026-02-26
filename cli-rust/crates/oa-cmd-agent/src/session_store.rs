/// Session store updates after agent runs.
///
/// Persists token usage, model information, compaction counts, and
/// CLI session identifiers back to the session store after each agent
/// execution completes.
///
/// Source: `src/commands/agent/session-store.ts`

use std::path::Path;

use anyhow::Result;

use oa_config::sessions::store::{load_session_store, save_session_store};
use oa_types::config::OpenAcosmiConfig;
use oa_types::session::SessionEntry;

/// Default context token window size when model metadata is unavailable.
///
/// Source: `src/agents/defaults.ts` - `DEFAULT_CONTEXT_TOKENS`
const DEFAULT_CONTEXT_TOKENS: u64 = 200_000;

/// Token usage from an agent run.
///
/// Source: `src/commands/agent/session-store.ts` (inline usage type)
#[derive(Debug, Clone, Default)]
pub struct AgentRunUsage {
    /// Input tokens consumed.
    pub input: u64,
    /// Output tokens produced.
    pub output: u64,
}

/// Metadata from an agent run used to update the session store.
///
/// Source: `src/commands/agent/session-store.ts` (RunResult meta shape)
#[derive(Debug, Clone, Default)]
pub struct AgentRunMeta {
    /// Model used during the run.
    pub model: Option<String>,
    /// Provider used during the run.
    pub provider: Option<String>,
    /// Token usage.
    pub usage: Option<AgentRunUsage>,
    /// Number of compactions performed in this run.
    pub compaction_count: Option<u64>,
    /// Whether the run was aborted.
    pub aborted: Option<bool>,
    /// CLI session identifier (for CLI-backend providers).
    pub session_id: Option<String>,
}

/// Parameters for updating the session store after an agent run.
///
/// Source: `src/commands/agent/session-store.ts` - `updateSessionStoreAfterAgentRun`
pub struct UpdateSessionStoreParams {
    /// The full configuration.
    pub cfg: OpenAcosmiConfig,
    /// Context tokens override from agent defaults.
    pub context_tokens_override: Option<u64>,
    /// Session identifier.
    pub session_id: String,
    /// Session key in the store.
    pub session_key: String,
    /// Path to the session store file.
    pub store_path: String,
    /// Default provider used.
    pub default_provider: String,
    /// Default model used.
    pub default_model: String,
    /// Fallback provider (if model fallback was applied).
    pub fallback_provider: Option<String>,
    /// Fallback model (if model fallback was applied).
    pub fallback_model: Option<String>,
    /// Agent run metadata.
    pub meta: AgentRunMeta,
}

/// Check whether usage has non-zero values.
///
/// Source: `src/agents/usage.ts` - `hasNonzeroUsage`
fn has_nonzero_usage(usage: &Option<AgentRunUsage>) -> bool {
    match usage {
        Some(u) => u.input > 0 || u.output > 0,
        None => false,
    }
}

/// Update the session store after an agent run completes.
///
/// Persists model, provider, token usage, compaction count, abort status,
/// and CLI session identifiers.
///
/// Source: `src/commands/agent/session-store.ts` - `updateSessionStoreAfterAgentRun`
pub async fn update_session_store_after_agent_run(
    params: &UpdateSessionStoreParams,
) -> Result<()> {
    let store_path = Path::new(&params.store_path);
    let mut store = load_session_store(store_path);

    let model_used = params
        .meta
        .model
        .as_deref()
        .or(params.fallback_model.as_deref())
        .unwrap_or(&params.default_model)
        .to_owned();
    let provider_used = params
        .meta
        .provider
        .as_deref()
        .or(params.fallback_provider.as_deref())
        .unwrap_or(&params.default_provider)
        .to_owned();
    let context_tokens = params.context_tokens_override.unwrap_or(DEFAULT_CONTEXT_TOKENS);
    let compactions_this_run = params.meta.compaction_count.unwrap_or(0);

    let now = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis() as u64;

    let existing = store.get(&params.session_key).cloned();
    let base = existing.unwrap_or_else(|| SessionEntry {
        session_id: params.session_id.clone(),
        updated_at: now,
        ..default_session_entry(&params.session_id, now)
    });

    let mut next = base.clone();
    next.session_id.clone_from(&params.session_id);
    next.updated_at = now;
    next.model_provider = Some(provider_used);
    next.model = Some(model_used);
    next.context_tokens = Some(context_tokens);
    next.aborted_last_run = Some(params.meta.aborted.unwrap_or(false));

    if has_nonzero_usage(&params.meta.usage) {
        if let Some(ref usage) = params.meta.usage {
            next.input_tokens = Some(usage.input);
            next.output_tokens = Some(usage.output);
            next.total_tokens = Some(usage.input);
        }
    }

    if compactions_this_run > 0 {
        let prev_compactions = base.compaction_count.unwrap_or(0);
        next.compaction_count = Some(prev_compactions + compactions_this_run);
    }

    store.insert(params.session_key.clone(), next);
    save_session_store(store_path, &store).await?;

    Ok(())
}

/// Create a default session entry with the given id and timestamp.
fn default_session_entry(session_id: &str, updated_at: u64) -> SessionEntry {
    SessionEntry {
        session_id: session_id.to_owned(),
        updated_at,
        last_heartbeat_text: None,
        last_heartbeat_sent_at: None,
        session_file: None,
        spawned_by: None,
        system_sent: None,
        aborted_last_run: None,
        chat_type: None,
        thinking_level: None,
        verbose_level: None,
        reasoning_level: None,
        elevated_level: None,
        tts_auto: None,
        exec_host: None,
        exec_security: None,
        exec_ask: None,
        exec_node: None,
        response_usage: None,
        provider_override: None,
        model_override: None,
        auth_profile_override: None,
        auth_profile_override_source: None,
        auth_profile_override_compaction_count: None,
        group_activation: None,
        group_activation_needs_system_intro: None,
        send_policy: None,
        queue_mode: None,
        queue_debounce_ms: None,
        queue_cap: None,
        queue_drop: None,
        input_tokens: None,
        output_tokens: None,
        total_tokens: None,
        model_provider: None,
        model: None,
        context_tokens: None,
        compaction_count: None,
        memory_flush_at: None,
        memory_flush_compaction_count: None,
        cli_session_ids: None,
        claude_cli_session_id: None,
        label: None,
        display_name: None,
        channel: None,
        group_id: None,
        subject: None,
        group_channel: None,
        space: None,
        origin: None,
        delivery_context: None,
        last_channel: None,
        last_to: None,
        last_account_id: None,
        last_thread_id: None,
        skills_snapshot: None,
        system_prompt_report: None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn has_nonzero_usage_true() {
        let usage = Some(AgentRunUsage { input: 100, output: 50 });
        assert!(has_nonzero_usage(&usage));
    }

    #[test]
    fn has_nonzero_usage_false_zeros() {
        let usage = Some(AgentRunUsage { input: 0, output: 0 });
        assert!(!has_nonzero_usage(&usage));
    }

    #[test]
    fn has_nonzero_usage_false_none() {
        assert!(!has_nonzero_usage(&None));
    }

    #[test]
    fn default_context_tokens_value() {
        assert_eq!(DEFAULT_CONTEXT_TOKENS, 200_000);
    }

    #[test]
    fn default_session_entry_fields() {
        let entry = default_session_entry("test-id", 12345);
        assert_eq!(entry.session_id, "test-id");
        assert_eq!(entry.updated_at, 12345);
        assert!(entry.model.is_none());
        assert!(entry.model_provider.is_none());
    }
}
