/// Agent run context resolution.
///
/// Merges context from the command options (explicit overrides, channel info,
/// account id, group routing) into a unified `AgentRunContext` used by the
/// embedded agent runner.
///
/// Source: `src/commands/agent/run-context.ts`

use crate::types::{AgentCommandOpts, AgentRunContext};

/// Resolve the agent run context from command options.
///
/// Merges `opts.run_context` with top-level option fields, preferring
/// the `run_context` field when both are provided.
///
/// Source: `src/commands/agent/run-context.ts` - `resolveAgentRunContext`
pub fn resolve_agent_run_context(opts: &AgentCommandOpts) -> AgentRunContext {
    let mut merged = opts
        .run_context
        .clone()
        .unwrap_or_default();

    // Resolve message channel.
    let channel_candidate = merged
        .message_channel
        .as_deref()
        .or(opts.message_channel.as_deref())
        .or(opts.reply_channel.as_deref())
        .or(opts.channel.as_deref());
    if let Some(ch) = channel_candidate {
        let trimmed = ch.trim();
        if !trimmed.is_empty() {
            merged.message_channel = Some(trimmed.to_owned());
        }
    }

    // Resolve account id.
    let account_candidate = merged
        .account_id
        .as_deref()
        .or(opts.account_id.as_deref());
    if let Some(acct) = account_candidate {
        let trimmed = acct.trim();
        if !trimmed.is_empty() {
            merged.account_id = Some(trimmed.to_owned());
        }
    }

    // Resolve group id.
    let group_candidate = merged
        .group_id
        .as_deref()
        .or(opts.group_id.as_deref());
    if let Some(g) = group_candidate {
        let trimmed = g.trim();
        if !trimmed.is_empty() {
            merged.group_id = Some(trimmed.to_owned());
        }
    }

    // Resolve group channel.
    let group_channel_candidate = merged
        .group_channel
        .as_deref()
        .or(opts.group_channel.as_deref());
    if let Some(gc) = group_channel_candidate {
        let trimmed = gc.trim();
        if !trimmed.is_empty() {
            merged.group_channel = Some(trimmed.to_owned());
        }
    }

    // Resolve group space.
    let group_space_candidate = merged
        .group_space
        .as_deref()
        .or(opts.group_space.as_deref());
    if let Some(gs) = group_space_candidate {
        let trimmed = gs.trim();
        if !trimmed.is_empty() {
            merged.group_space = Some(trimmed.to_owned());
        }
    }

    // Populate current_thread_ts from thread_id if not already set.
    if merged.current_thread_ts.is_none() {
        if let Some(ref tid) = opts.thread_id {
            let trimmed = tid.trim();
            if !trimmed.is_empty() {
                merged.current_thread_ts = Some(trimmed.to_owned());
            }
        }
    }

    // Populate current_channel_id from outbound target.
    if merged.current_channel_id.is_none() {
        if let Some(ref to) = opts.to {
            let trimmed = to.trim();
            if !trimmed.is_empty() {
                merged.current_channel_id = Some(trimmed.to_owned());
            }
        }
    }

    merged
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn empty_opts_produce_empty_context() {
        let opts = AgentCommandOpts::default();
        let ctx = resolve_agent_run_context(&opts);
        assert!(ctx.message_channel.is_none());
        assert!(ctx.account_id.is_none());
        assert!(ctx.group_id.is_none());
    }

    #[test]
    fn channel_from_opts() {
        let opts = AgentCommandOpts {
            channel: Some("whatsapp".to_owned()),
            ..Default::default()
        };
        let ctx = resolve_agent_run_context(&opts);
        assert_eq!(ctx.message_channel.as_deref(), Some("whatsapp"));
    }

    #[test]
    fn run_context_takes_precedence() {
        let opts = AgentCommandOpts {
            channel: Some("telegram".to_owned()),
            run_context: Some(AgentRunContext {
                message_channel: Some("whatsapp".to_owned()),
                ..Default::default()
            }),
            ..Default::default()
        };
        let ctx = resolve_agent_run_context(&opts);
        assert_eq!(ctx.message_channel.as_deref(), Some("whatsapp"));
    }

    #[test]
    fn thread_id_populates_current_thread_ts() {
        let opts = AgentCommandOpts {
            thread_id: Some("T123".to_owned()),
            ..Default::default()
        };
        let ctx = resolve_agent_run_context(&opts);
        assert_eq!(ctx.current_thread_ts.as_deref(), Some("T123"));
    }

    #[test]
    fn to_populates_current_channel_id() {
        let opts = AgentCommandOpts {
            to: Some("+15551234567".to_owned()),
            ..Default::default()
        };
        let ctx = resolve_agent_run_context(&opts);
        assert_eq!(ctx.current_channel_id.as_deref(), Some("+15551234567"));
    }

    #[test]
    fn group_fields_merge() {
        let opts = AgentCommandOpts {
            group_id: Some("g1".to_owned()),
            group_channel: Some("gc1".to_owned()),
            group_space: Some("gs1".to_owned()),
            ..Default::default()
        };
        let ctx = resolve_agent_run_context(&opts);
        assert_eq!(ctx.group_id.as_deref(), Some("g1"));
        assert_eq!(ctx.group_channel.as_deref(), Some("gc1"));
        assert_eq!(ctx.group_space.as_deref(), Some("gs1"));
    }

    #[test]
    fn blank_values_are_skipped() {
        let opts = AgentCommandOpts {
            channel: Some("  ".to_owned()),
            to: Some("".to_owned()),
            ..Default::default()
        };
        let ctx = resolve_agent_run_context(&opts);
        assert!(ctx.message_channel.is_none());
        assert!(ctx.current_channel_id.is_none());
    }
}
