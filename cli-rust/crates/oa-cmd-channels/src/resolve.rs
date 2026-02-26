/// `channels resolve` command: resolves user/group identifiers to channel-
/// specific target IDs.
///
/// Accepts one or more entries on the command line, auto-detects whether each
/// represents a user (DM) or group target, and prints the resolved ID.
///
/// Source: `src/commands/channels/resolve.ts`

use anyhow::{bail, Result};
use serde::{Deserialize, Serialize};

use regex::Regex;
use std::sync::LazyLock;

/// Options for the `channels resolve` subcommand.
///
/// Source: `src/commands/channels/resolve.ts` — `ChannelsResolveOptions`
#[derive(Debug, Clone, Default)]
pub struct ChannelsResolveOptions {
    /// Channel identifier.
    pub channel: Option<String>,
    /// Account identifier override.
    pub account: Option<String>,
    /// Resolution kind: "auto", "user", "group", or "channel".
    pub kind: Option<String>,
    /// Output in JSON format.
    pub json: bool,
    /// Entries to resolve.
    pub entries: Vec<String>,
}

/// Kind of resolve target.
///
/// Source: `src/commands/channels/resolve.ts` — `ChannelResolveKind`
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ResolveKind {
    /// Resolve as a user (DM target).
    User,
    /// Resolve as a group/channel target.
    Group,
}

/// Result of resolving a single entry.
///
/// Source: `src/commands/channels/resolve.ts` — `ResolveResult`
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ResolveResult {
    /// The original input string.
    pub input: String,
    /// Whether resolution succeeded.
    pub resolved: bool,
    /// Resolved identifier.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,
    /// Resolved display name.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    /// Error message.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
    /// Additional note.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub note: Option<String>,
}

/// Regex patterns for user-like targets.
///
/// Source: `src/commands/channels/resolve.ts` — `detectAutoKind`
static MENTION_RE: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"^<@!?").expect("valid regex"));

/// Source: `src/commands/channels/resolve.ts` — `detectAutoKind`
static EMAIL_RE: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"^[^\s@]+@[^\s@]+\.[^\s@]+$").expect("valid regex"));

/// Source: `src/commands/channels/resolve.ts` — `detectAutoKind`
static USER_PREFIX_RE: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"(?i)^(user|discord|slack|matrix|msteams|teams|zalo|zalouser|googlechat|google-chat|gchat):").expect("valid regex")
});

/// Parse the preferred resolve kind from user input.
///
/// Source: `src/commands/channels/resolve.ts` — `resolvePreferredKind`
#[must_use]
pub fn resolve_preferred_kind(kind: Option<&str>) -> Option<ResolveKind> {
    match kind {
        None | Some("auto") | Some("") => None,
        Some("user") => Some(ResolveKind::User),
        Some("group") | Some("channel") => Some(ResolveKind::Group),
        _ => None,
    }
}

/// Auto-detect whether an input looks like a user or group target.
///
/// Source: `src/commands/channels/resolve.ts` — `detectAutoKind`
#[must_use]
pub fn detect_auto_kind(input: &str) -> ResolveKind {
    let trimmed = input.trim();
    if trimmed.is_empty() {
        return ResolveKind::Group;
    }
    if trimmed.starts_with('@') {
        return ResolveKind::User;
    }
    if MENTION_RE.is_match(trimmed) {
        return ResolveKind::User;
    }
    if EMAIL_RE.is_match(trimmed) {
        return ResolveKind::User;
    }
    if USER_PREFIX_RE.is_match(trimmed) {
        return ResolveKind::User;
    }
    ResolveKind::Group
}

/// Format a resolved result for human display.
///
/// Source: `src/commands/channels/resolve.ts` — `formatResolveResult`
#[must_use]
pub fn format_resolve_result(result: &ResolveResult) -> String {
    if !result.resolved || result.id.is_none() {
        return format!("{} -> unresolved", result.input);
    }
    let id = result.id.as_deref().unwrap_or("?");
    let name = result
        .name
        .as_deref()
        .map(|n| format!(" ({n})"))
        .unwrap_or_default();
    let note = result
        .note
        .as_deref()
        .map(|n| format!(" [{n}]"))
        .unwrap_or_default();
    format!("{} -> {id}{name}{note}", result.input)
}

/// Execute the `channels resolve` command.
///
/// Note: In the Rust implementation, actual channel plugin resolve calls are
/// not yet wired up (requires runtime gateway integration). This command
/// currently validates inputs and reports them as unresolved with a note.
///
/// Source: `src/commands/channels/resolve.ts` — `channelsResolveCommand`
pub async fn channels_resolve_command(opts: &ChannelsResolveOptions) -> Result<()> {
    let entries: Vec<String> = opts
        .entries
        .iter()
        .map(|e| e.trim().to_owned())
        .filter(|e| !e.is_empty())
        .collect();

    if entries.is_empty() {
        bail!("At least one entry is required.");
    }

    // In the full implementation this would call into channel plugin resolvers.
    // For now, we create stub results to match the command interface.
    let results: Vec<ResolveResult> = entries
        .iter()
        .map(|input| {
            let _kind = resolve_preferred_kind(opts.kind.as_deref())
                .unwrap_or_else(|| detect_auto_kind(input));
            ResolveResult {
                input: input.clone(),
                resolved: false,
                id: None,
                name: None,
                error: None,
                note: Some("resolver not connected (stub)".to_owned()),
            }
        })
        .collect();

    if opts.json {
        println!("{}", serde_json::to_string_pretty(&results)?);
        return Ok(());
    }

    for result in &results {
        if result.resolved && result.id.is_some() {
            println!("{}", format_resolve_result(result));
        } else {
            let suffix = result
                .error
                .as_deref()
                .or(result.note.as_deref())
                .map(|s| format!(" ({s})"))
                .unwrap_or_default();
            eprintln!("{} -> unresolved{suffix}", result.input);
        }
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn resolve_preferred_kind_auto() {
        assert_eq!(resolve_preferred_kind(None), None);
        assert_eq!(resolve_preferred_kind(Some("auto")), None);
        assert_eq!(resolve_preferred_kind(Some("")), None);
    }

    #[test]
    fn resolve_preferred_kind_user() {
        assert_eq!(resolve_preferred_kind(Some("user")), Some(ResolveKind::User));
    }

    #[test]
    fn resolve_preferred_kind_group() {
        assert_eq!(
            resolve_preferred_kind(Some("group")),
            Some(ResolveKind::Group)
        );
        assert_eq!(
            resolve_preferred_kind(Some("channel")),
            Some(ResolveKind::Group)
        );
    }

    #[test]
    fn detect_auto_kind_at_mention() {
        assert_eq!(detect_auto_kind("@alice"), ResolveKind::User);
    }

    #[test]
    fn detect_auto_kind_discord_mention() {
        assert_eq!(detect_auto_kind("<@123456>"), ResolveKind::User);
        assert_eq!(detect_auto_kind("<@!123456>"), ResolveKind::User);
    }

    #[test]
    fn detect_auto_kind_email() {
        assert_eq!(detect_auto_kind("user@example.com"), ResolveKind::User);
    }

    #[test]
    fn detect_auto_kind_user_prefix() {
        assert_eq!(detect_auto_kind("user:123"), ResolveKind::User);
        assert_eq!(detect_auto_kind("discord:123"), ResolveKind::User);
        assert_eq!(detect_auto_kind("slack:U123"), ResolveKind::User);
    }

    #[test]
    fn detect_auto_kind_group_by_default() {
        assert_eq!(detect_auto_kind("general"), ResolveKind::Group);
        assert_eq!(detect_auto_kind("#general"), ResolveKind::Group);
        assert_eq!(detect_auto_kind("C123456"), ResolveKind::Group);
    }

    #[test]
    fn detect_auto_kind_empty() {
        assert_eq!(detect_auto_kind(""), ResolveKind::Group);
        assert_eq!(detect_auto_kind("  "), ResolveKind::Group);
    }

    #[test]
    fn format_resolve_result_resolved() {
        let result = ResolveResult {
            input: "alice".to_owned(),
            resolved: true,
            id: Some("U123".to_owned()),
            name: Some("Alice".to_owned()),
            error: None,
            note: None,
        };
        assert_eq!(format_resolve_result(&result), "alice -> U123 (Alice)");
    }

    #[test]
    fn format_resolve_result_unresolved() {
        let result = ResolveResult {
            input: "unknown".to_owned(),
            resolved: false,
            id: None,
            name: None,
            error: None,
            note: None,
        };
        assert_eq!(format_resolve_result(&result), "unknown -> unresolved");
    }

    #[test]
    fn format_resolve_result_with_note() {
        let result = ResolveResult {
            input: "alice".to_owned(),
            resolved: true,
            id: Some("U123".to_owned()),
            name: None,
            error: None,
            note: Some("cached".to_owned()),
        };
        assert_eq!(format_resolve_result(&result), "alice -> U123 [cached]");
    }
}
