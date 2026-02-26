/// Session key standardization: normalization, building, parsing, and classification.
///
/// Source: `src/routing/session-key.ts`, `src/sessions/session-key-utils.ts`

use std::collections::HashMap;
use std::sync::LazyLock;

use oa_types::common::ChatType;
use regex::Regex;

// ── Constants ──

/// Default agent identifier when none is specified.
///
/// Source: `src/routing/session-key.ts` - `DEFAULT_AGENT_ID`
pub const DEFAULT_AGENT_ID: &str = "main";

/// Default main session key.
///
/// Source: `src/routing/session-key.ts` - `DEFAULT_MAIN_KEY`
pub const DEFAULT_MAIN_KEY: &str = "main";

/// Default account identifier when none is specified.
///
/// Source: `src/routing/session-key.ts` - `DEFAULT_ACCOUNT_ID`
pub const DEFAULT_ACCOUNT_ID: &str = "default";

// ── Pre-compiled regex patterns ──

/// Validates that an ID is alphanumeric (with hyphens/underscores), 1-64 chars.
///
/// Source: `src/routing/session-key.ts` - `VALID_ID_RE`
static VALID_ID_RE: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"(?i)^[a-z0-9][a-z0-9_-]{0,63}$").expect("valid regex"));

/// Matches sequences of characters not in `[a-z0-9_-]`.
///
/// Source: `src/routing/session-key.ts` - `INVALID_CHARS_RE`
static INVALID_CHARS_RE: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"[^a-z0-9_-]+").expect("valid regex"));

/// Matches leading dashes.
///
/// Source: `src/routing/session-key.ts` - `LEADING_DASH_RE`
static LEADING_DASH_RE: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"^-+").expect("valid regex"));

/// Matches trailing dashes.
///
/// Source: `src/routing/session-key.ts` - `TRAILING_DASH_RE`
static TRAILING_DASH_RE: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"-+$").expect("valid regex"));

/// Thread session markers used when resolving parent session keys.
///
/// Source: `src/sessions/session-key-utils.ts` - `THREAD_SESSION_MARKERS`
const THREAD_SESSION_MARKERS: &[&str] = &[":thread:", ":topic:"];

// ── Enums ──

/// Shape classification for session keys.
///
/// Source: `src/routing/session-key.ts` - `SessionKeyShape`
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SessionKeyShape {
    /// No session key provided.
    Missing,
    /// Well-formed `agent:<id>:<rest>` key.
    Agent,
    /// Legacy key or alias (not agent-prefixed).
    LegacyOrAlias,
    /// Starts with `agent:` but fails to parse.
    MalformedAgent,
}

/// DM session scope controlling how direct-message sessions are bucketed.
///
/// Source: `src/routing/session-key.ts` - `dmScope` parameter
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum DmScope {
    /// All DMs collapse into the main session key.
    #[default]
    Main,
    /// One session per peer across all channels.
    PerPeer,
    /// One session per (channel, peer) pair.
    PerChannelPeer,
    /// One session per (account, channel, peer) triple.
    PerAccountChannelPeer,
}

// ── Parsed key types ──

/// A parsed agent session key of the form `agent:<agentId>:<rest>`.
///
/// Source: `src/sessions/session-key-utils.ts` - `ParsedAgentSessionKey`
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ParsedAgentSessionKey {
    /// The agent identifier extracted from the key.
    pub agent_id: String,
    /// The remainder of the key after `agent:<agentId>:`.
    pub rest: String,
}

/// Result of resolving thread session keys.
///
/// Source: `src/routing/session-key.ts` - return type of `resolveThreadSessionKeys`
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ResolvedThreadSessionKeys {
    /// The resolved session key (may include thread suffix).
    pub session_key: String,
    /// The parent session key, if threads are involved.
    pub parent_session_key: Option<String>,
}

// ── Internal helpers ──

/// Normalize a token by trimming and lowercasing. Returns empty string for `None`.
///
/// Source: `src/routing/session-key.ts` - `normalizeToken`
fn normalize_token(value: Option<&str>) -> String {
    value.unwrap_or("").trim().to_lowercase()
}

/// Best-effort sanitization: lowercase, replace invalid chars with `-`,
/// strip leading/trailing dashes, truncate to 64 chars.
fn sanitize_id(value: &str, fallback: &str) -> String {
    let lowered = value.to_lowercase();
    let replaced = INVALID_CHARS_RE.replace_all(&lowered, "-");
    let no_leading = LEADING_DASH_RE.replace(&replaced, "");
    let no_trailing = TRAILING_DASH_RE.replace(&no_leading, "");
    let truncated = if no_trailing.len() > 64 {
        &no_trailing[..64]
    } else {
        &no_trailing
    };
    if truncated.is_empty() {
        fallback.to_owned()
    } else {
        truncated.to_owned()
    }
}

// ── Public: parsing ──

/// Parse an agent session key of the form `agent:<agentId>:<rest>`.
///
/// Returns `None` if the key is empty, has fewer than 3 non-empty colon-separated
/// parts, or does not start with `"agent"`.
///
/// Source: `src/sessions/session-key-utils.ts` - `parseAgentSessionKey`
pub fn parse_agent_session_key(session_key: Option<&str>) -> Option<ParsedAgentSessionKey> {
    let raw = session_key.unwrap_or("").trim();
    if raw.is_empty() {
        return None;
    }
    let parts: Vec<&str> = raw.split(':').filter(|s| !s.is_empty()).collect();
    if parts.len() < 3 {
        return None;
    }
    if parts[0] != "agent" {
        return None;
    }
    let agent_id = parts[1].trim();
    let rest = parts[2..].join(":");
    if agent_id.is_empty() || rest.is_empty() {
        return None;
    }
    Some(ParsedAgentSessionKey {
        agent_id: agent_id.to_owned(),
        rest,
    })
}

/// Check whether a session key represents a subagent session.
///
/// Source: `src/sessions/session-key-utils.ts` - `isSubagentSessionKey`
pub fn is_subagent_session_key(session_key: Option<&str>) -> bool {
    let raw = session_key.unwrap_or("").trim();
    if raw.is_empty() {
        return false;
    }
    if raw.to_lowercase().starts_with("subagent:") {
        return true;
    }
    parse_agent_session_key(Some(raw))
        .is_some_and(|parsed| parsed.rest.to_lowercase().starts_with("subagent:"))
}

/// Check whether a session key represents an ACP session.
///
/// Source: `src/sessions/session-key-utils.ts` - `isAcpSessionKey`
pub fn is_acp_session_key(session_key: Option<&str>) -> bool {
    let raw = session_key.unwrap_or("").trim();
    if raw.is_empty() {
        return false;
    }
    let normalized = raw.to_lowercase();
    if normalized.starts_with("acp:") {
        return true;
    }
    parse_agent_session_key(Some(raw))
        .is_some_and(|parsed| parsed.rest.to_lowercase().starts_with("acp:"))
}

/// Resolve the parent session key by stripping a trailing `:thread:` or `:topic:` segment.
///
/// Source: `src/sessions/session-key-utils.ts` - `resolveThreadParentSessionKey`
pub fn resolve_thread_parent_session_key(session_key: Option<&str>) -> Option<String> {
    let raw = session_key.unwrap_or("").trim();
    if raw.is_empty() {
        return None;
    }
    let normalized = raw.to_lowercase();
    let mut idx: Option<usize> = None;
    for marker in THREAD_SESSION_MARKERS {
        if let Some(candidate) = normalized.rfind(marker) {
            match idx {
                Some(current) if candidate > current => idx = Some(candidate),
                None => idx = Some(candidate),
                _ => {}
            }
        }
    }
    let idx = idx?;
    if idx == 0 {
        return None;
    }
    let parent = raw[..idx].trim();
    if parent.is_empty() { None } else { Some(parent.to_owned()) }
}

// ── Public: normalization ──

/// Normalize a main session key. Blank values default to [`DEFAULT_MAIN_KEY`].
///
/// Source: `src/routing/session-key.ts` - `normalizeMainKey`
pub fn normalize_main_key(value: Option<&str>) -> String {
    let trimmed = value.unwrap_or("").trim();
    if trimmed.is_empty() {
        DEFAULT_MAIN_KEY.to_owned()
    } else {
        trimmed.to_lowercase()
    }
}

/// Normalize an agent identifier. Blank values default to [`DEFAULT_AGENT_ID`].
/// Invalid characters are collapsed to `-` and leading/trailing dashes are stripped.
///
/// Source: `src/routing/session-key.ts` - `normalizeAgentId`
pub fn normalize_agent_id(value: Option<&str>) -> String {
    let trimmed = value.unwrap_or("").trim();
    if trimmed.is_empty() {
        return DEFAULT_AGENT_ID.to_owned();
    }
    if VALID_ID_RE.is_match(trimmed) {
        return trimmed.to_lowercase();
    }
    sanitize_id(trimmed, DEFAULT_AGENT_ID)
}

/// Sanitize an agent identifier. Equivalent to [`normalize_agent_id`] -- both
/// perform the same best-effort cleanup in the TypeScript source.
///
/// Source: `src/routing/session-key.ts` - `sanitizeAgentId`
pub fn sanitize_agent_id(value: Option<&str>) -> String {
    normalize_agent_id(value)
}

/// Normalize an account identifier. Blank values default to [`DEFAULT_ACCOUNT_ID`].
///
/// Source: `src/routing/session-key.ts` - `normalizeAccountId`
pub fn normalize_account_id(value: Option<&str>) -> String {
    let trimmed = value.unwrap_or("").trim();
    if trimmed.is_empty() {
        return DEFAULT_ACCOUNT_ID.to_owned();
    }
    if VALID_ID_RE.is_match(trimmed) {
        return trimmed.to_lowercase();
    }
    sanitize_id(trimmed, DEFAULT_ACCOUNT_ID)
}

// ── Public: key building ──

/// Build the canonical main session key: `agent:<agentId>:<mainKey>`.
///
/// Source: `src/routing/session-key.ts` - `buildAgentMainSessionKey`
pub fn build_agent_main_session_key(agent_id: &str, main_key: Option<&str>) -> String {
    let agent_id = normalize_agent_id(Some(agent_id));
    let main_key = normalize_main_key(main_key);
    format!("agent:{agent_id}:{main_key}")
}

/// Parameters for building a peer session key.
///
/// Source: `src/routing/session-key.ts` - `buildAgentPeerSessionKey` params
pub struct PeerSessionKeyParams<'a> {
    /// Agent identifier.
    pub agent_id: &'a str,
    /// Optional main key override.
    pub main_key: Option<&'a str>,
    /// Channel identifier (e.g. `"twilio"`, `"discord"`).
    pub channel: &'a str,
    /// Account identifier.
    pub account_id: Option<&'a str>,
    /// The kind of chat peer.
    pub peer_kind: Option<&'a ChatType>,
    /// Peer identifier.
    pub peer_id: Option<&'a str>,
    /// Identity links: canonical name -> list of aliases.
    pub identity_links: Option<&'a HashMap<String, Vec<String>>>,
    /// DM session scope.
    pub dm_scope: Option<DmScope>,
}

/// Resolve a linked peer ID via identity links.
///
/// Source: `src/routing/session-key.ts` - `resolveLinkedPeerId`
fn resolve_linked_peer_id(
    identity_links: Option<&HashMap<String, Vec<String>>>,
    channel: &str,
    peer_id: &str,
) -> Option<String> {
    let identity_links = identity_links?;
    let peer_id = peer_id.trim();
    if peer_id.is_empty() {
        return None;
    }

    let mut candidates = Vec::new();
    let raw_candidate = normalize_token(Some(peer_id));
    if !raw_candidate.is_empty() {
        candidates.push(raw_candidate);
    }
    let channel_normalized = normalize_token(Some(channel));
    if !channel_normalized.is_empty() {
        let scoped = normalize_token(Some(&format!("{channel_normalized}:{peer_id}")));
        if !scoped.is_empty() && !candidates.contains(&scoped) {
            candidates.push(scoped);
        }
    }
    if candidates.is_empty() {
        return None;
    }

    for (canonical, ids) in identity_links {
        let canonical_name = canonical.trim();
        if canonical_name.is_empty() {
            continue;
        }
        for id in ids {
            let normalized = normalize_token(Some(id));
            if !normalized.is_empty() && candidates.contains(&normalized) {
                return Some(canonical_name.to_owned());
            }
        }
    }
    None
}

/// Build a peer session key for an agent, applying DM scope rules.
///
/// For `ChatType::Direct`, the session may collapse to the main key or include
/// peer/channel/account segments depending on the [`DmScope`].
///
/// For `ChatType::Group` or `ChatType::Channel`, the key is always
/// `agent:<agentId>:<channel>:<peerKind>:<peerId>`.
///
/// Source: `src/routing/session-key.ts` - `buildAgentPeerSessionKey`
pub fn build_agent_peer_session_key(params: &PeerSessionKeyParams<'_>) -> String {
    let peer_kind = params.peer_kind.unwrap_or(&ChatType::Direct);
    let agent_id = normalize_agent_id(Some(params.agent_id));

    if *peer_kind == ChatType::Direct {
        let dm_scope = params.dm_scope.unwrap_or(DmScope::Main);
        let mut peer_id = params.peer_id.unwrap_or("").trim().to_owned();

        let linked = if dm_scope == DmScope::Main {
            None
        } else {
            resolve_linked_peer_id(params.identity_links, params.channel, &peer_id)
        };
        if let Some(linked_id) = linked {
            peer_id = linked_id;
        }
        peer_id = peer_id.to_lowercase();

        if dm_scope == DmScope::PerAccountChannelPeer && !peer_id.is_empty() {
            let channel = {
                let c = params.channel.trim().to_lowercase();
                if c.is_empty() { "unknown".to_owned() } else { c }
            };
            let account_id = normalize_account_id(params.account_id);
            return format!("agent:{agent_id}:{channel}:{account_id}:direct:{peer_id}");
        }
        if dm_scope == DmScope::PerChannelPeer && !peer_id.is_empty() {
            let channel = {
                let c = params.channel.trim().to_lowercase();
                if c.is_empty() { "unknown".to_owned() } else { c }
            };
            return format!("agent:{agent_id}:{channel}:direct:{peer_id}");
        }
        if dm_scope == DmScope::PerPeer && !peer_id.is_empty() {
            return format!("agent:{agent_id}:direct:{peer_id}");
        }
        return build_agent_main_session_key(params.agent_id, params.main_key);
    }

    // Group or Channel
    let channel = {
        let c = params.channel.trim().to_lowercase();
        if c.is_empty() { "unknown".to_owned() } else { c }
    };
    let peer_id = {
        let p = params.peer_id.unwrap_or("").trim().to_lowercase();
        if p.is_empty() { "unknown".to_owned() } else { p }
    };
    let kind_str = match peer_kind {
        ChatType::Group => "group",
        ChatType::Channel => "channel",
        ChatType::Direct => unreachable!(),
    };
    format!("agent:{agent_id}:{channel}:{kind_str}:{peer_id}")
}

// ── Public: key classification / resolution ──

/// Extract the agent ID from a session key. Falls back to [`DEFAULT_AGENT_ID`].
///
/// Source: `src/routing/session-key.ts` - `resolveAgentIdFromSessionKey`
pub fn resolve_agent_id_from_session_key(session_key: Option<&str>) -> String {
    let parsed = parse_agent_session_key(session_key);
    let raw_id = parsed.map_or(DEFAULT_AGENT_ID.to_owned(), |p| p.agent_id);
    normalize_agent_id(Some(&raw_id))
}

/// Classify the shape of a session key.
///
/// Source: `src/routing/session-key.ts` - `classifySessionKeyShape`
pub fn classify_session_key_shape(session_key: Option<&str>) -> SessionKeyShape {
    let raw = session_key.unwrap_or("").trim();
    if raw.is_empty() {
        return SessionKeyShape::Missing;
    }
    if parse_agent_session_key(Some(raw)).is_some() {
        return SessionKeyShape::Agent;
    }
    if raw.to_lowercase().starts_with("agent:") {
        SessionKeyShape::MalformedAgent
    } else {
        SessionKeyShape::LegacyOrAlias
    }
}

/// Convert a store key to a request key by stripping the `agent:<id>:` prefix.
///
/// Returns `None` if the input is blank.
///
/// Source: `src/routing/session-key.ts` - `toAgentRequestSessionKey`
pub fn to_agent_request_session_key(store_key: Option<&str>) -> Option<String> {
    let raw = store_key.unwrap_or("").trim();
    if raw.is_empty() {
        return None;
    }
    Some(
        parse_agent_session_key(Some(raw))
            .map_or_else(|| raw.to_owned(), |parsed| parsed.rest),
    )
}

/// Convert a request key to a store key by prepending the `agent:<agentId>:` prefix.
///
/// If the request key is blank or equals [`DEFAULT_MAIN_KEY`], falls back to
/// [`build_agent_main_session_key`].
///
/// Source: `src/routing/session-key.ts` - `toAgentStoreSessionKey`
pub fn to_agent_store_session_key(
    agent_id: &str,
    request_key: Option<&str>,
    main_key: Option<&str>,
) -> String {
    let raw = request_key.unwrap_or("").trim();
    if raw.is_empty() || raw == DEFAULT_MAIN_KEY {
        return build_agent_main_session_key(agent_id, main_key);
    }
    let lowered = raw.to_lowercase();
    if lowered.starts_with("agent:") {
        return lowered;
    }
    let normalized_agent = normalize_agent_id(Some(agent_id));
    format!("agent:{normalized_agent}:{lowered}")
}

/// Build a group/channel history key: `<channel>:<accountId>:<peerKind>:<peerId>`.
///
/// Source: `src/routing/session-key.ts` - `buildGroupHistoryKey`
pub fn build_group_history_key(
    channel: &str,
    account_id: Option<&str>,
    peer_kind: &str,
    peer_id: &str,
) -> String {
    let channel = {
        let c = normalize_token(Some(channel));
        if c.is_empty() { "unknown".to_owned() } else { c }
    };
    let account_id = normalize_account_id(account_id);
    let peer_id = {
        let p = peer_id.trim().to_lowercase();
        if p.is_empty() { "unknown".to_owned() } else { p }
    };
    format!("{channel}:{account_id}:{peer_kind}:{peer_id}")
}

/// Resolve thread session keys by optionally appending a `:thread:<id>` suffix.
///
/// Source: `src/routing/session-key.ts` - `resolveThreadSessionKeys`
pub fn resolve_thread_session_keys(
    base_session_key: &str,
    thread_id: Option<&str>,
    parent_session_key: Option<String>,
    use_suffix: Option<bool>,
) -> ResolvedThreadSessionKeys {
    let thread_id = thread_id.unwrap_or("").trim();
    if thread_id.is_empty() {
        return ResolvedThreadSessionKeys {
            session_key: base_session_key.to_owned(),
            parent_session_key: None,
        };
    }
    let normalized_thread_id = thread_id.to_lowercase();
    let use_suffix = use_suffix.unwrap_or(true);
    let session_key = if use_suffix {
        format!("{base_session_key}:thread:{normalized_thread_id}")
    } else {
        base_session_key.to_owned()
    };
    ResolvedThreadSessionKeys {
        session_key,
        parent_session_key,
    }
}

// ── Tests ──

#[cfg(test)]
mod tests {
    use super::*;

    // ── parse_agent_session_key ──

    #[test]
    fn parse_valid_agent_key() {
        let result = parse_agent_session_key(Some("agent:mybot:main"));
        assert_eq!(
            result,
            Some(ParsedAgentSessionKey {
                agent_id: "mybot".to_owned(),
                rest: "main".to_owned(),
            })
        );
    }

    #[test]
    fn parse_agent_key_with_multiple_colons() {
        let result = parse_agent_session_key(Some("agent:bot:twilio:direct:+1234"));
        assert_eq!(
            result,
            Some(ParsedAgentSessionKey {
                agent_id: "bot".to_owned(),
                rest: "twilio:direct:+1234".to_owned(),
            })
        );
    }

    #[test]
    fn parse_agent_key_none() {
        assert_eq!(parse_agent_session_key(None), None);
    }

    #[test]
    fn parse_agent_key_empty() {
        assert_eq!(parse_agent_session_key(Some("")), None);
        assert_eq!(parse_agent_session_key(Some("  ")), None);
    }

    #[test]
    fn parse_agent_key_too_few_parts() {
        assert_eq!(parse_agent_session_key(Some("agent:bot")), None);
        assert_eq!(parse_agent_session_key(Some("agent")), None);
    }

    #[test]
    fn parse_agent_key_wrong_prefix() {
        assert_eq!(parse_agent_session_key(Some("subagent:bot:main")), None);
    }

    // ── is_subagent_session_key ──

    #[test]
    fn subagent_key_direct() {
        assert!(is_subagent_session_key(Some("subagent:helper:main")));
    }

    #[test]
    fn subagent_key_nested() {
        assert!(is_subagent_session_key(Some(
            "agent:bot:subagent:helper:main"
        )));
    }

    #[test]
    fn not_subagent_key() {
        assert!(!is_subagent_session_key(Some("agent:bot:main")));
        assert!(!is_subagent_session_key(None));
    }

    // ── is_acp_session_key ──

    #[test]
    fn acp_key_direct() {
        assert!(is_acp_session_key(Some("acp:session123")));
    }

    #[test]
    fn acp_key_nested() {
        assert!(is_acp_session_key(Some("agent:bot:acp:session123")));
    }

    #[test]
    fn not_acp_key() {
        assert!(!is_acp_session_key(Some("agent:bot:main")));
        assert!(!is_acp_session_key(None));
    }

    // ── normalize_main_key ──

    #[test]
    fn main_key_defaults() {
        assert_eq!(normalize_main_key(None), "main");
        assert_eq!(normalize_main_key(Some("")), "main");
        assert_eq!(normalize_main_key(Some("  ")), "main");
    }

    #[test]
    fn main_key_lowercases() {
        assert_eq!(normalize_main_key(Some("MyKey")), "mykey");
        assert_eq!(normalize_main_key(Some("  Main  ")), "main");
    }

    // ── normalize_agent_id ──

    #[test]
    fn agent_id_defaults() {
        assert_eq!(normalize_agent_id(None), "main");
        assert_eq!(normalize_agent_id(Some("")), "main");
    }

    #[test]
    fn agent_id_valid() {
        assert_eq!(normalize_agent_id(Some("bot1")), "bot1");
        assert_eq!(normalize_agent_id(Some("My-Bot")), "my-bot");
    }

    #[test]
    fn agent_id_sanitizes_invalid() {
        assert_eq!(normalize_agent_id(Some("my bot!")), "my-bot");
        assert_eq!(normalize_agent_id(Some("---")), "main");
    }

    // ── sanitize_agent_id ──

    #[test]
    fn sanitize_matches_normalize() {
        assert_eq!(sanitize_agent_id(Some("My Bot!")), normalize_agent_id(Some("My Bot!")));
        assert_eq!(sanitize_agent_id(None), normalize_agent_id(None));
    }

    // ── normalize_account_id ──

    #[test]
    fn account_id_defaults() {
        assert_eq!(normalize_account_id(None), "default");
        assert_eq!(normalize_account_id(Some("")), "default");
    }

    #[test]
    fn account_id_valid() {
        assert_eq!(normalize_account_id(Some("acct1")), "acct1");
        assert_eq!(normalize_account_id(Some("MyAcct")), "myacct");
    }

    #[test]
    fn account_id_sanitizes_invalid() {
        assert_eq!(normalize_account_id(Some("my account!")), "my-account");
    }

    // ── build_agent_main_session_key ──

    #[test]
    fn build_main_key_default() {
        assert_eq!(
            build_agent_main_session_key("bot", None),
            "agent:bot:main"
        );
    }

    #[test]
    fn build_main_key_custom() {
        assert_eq!(
            build_agent_main_session_key("bot", Some("work")),
            "agent:bot:work"
        );
    }

    // ── build_agent_peer_session_key ──

    #[test]
    fn peer_key_group() {
        let params = PeerSessionKeyParams {
            agent_id: "bot",
            main_key: None,
            channel: "discord",
            account_id: None,
            peer_kind: Some(&ChatType::Group),
            peer_id: Some("general"),
            identity_links: None,
            dm_scope: None,
        };
        assert_eq!(
            build_agent_peer_session_key(&params),
            "agent:bot:discord:group:general"
        );
    }

    #[test]
    fn peer_key_channel_type() {
        let params = PeerSessionKeyParams {
            agent_id: "bot",
            main_key: None,
            channel: "slack",
            account_id: None,
            peer_kind: Some(&ChatType::Channel),
            peer_id: Some("C123"),
            identity_links: None,
            dm_scope: None,
        };
        assert_eq!(
            build_agent_peer_session_key(&params),
            "agent:bot:slack:channel:c123"
        );
    }

    #[test]
    fn peer_key_direct_main_scope() {
        let params = PeerSessionKeyParams {
            agent_id: "bot",
            main_key: None,
            channel: "twilio",
            account_id: None,
            peer_kind: Some(&ChatType::Direct),
            peer_id: Some("+1234"),
            identity_links: None,
            dm_scope: Some(DmScope::Main),
        };
        assert_eq!(
            build_agent_peer_session_key(&params),
            "agent:bot:main"
        );
    }

    #[test]
    fn peer_key_direct_per_peer() {
        let params = PeerSessionKeyParams {
            agent_id: "bot",
            main_key: None,
            channel: "twilio",
            account_id: None,
            peer_kind: Some(&ChatType::Direct),
            peer_id: Some("+1234567890"),
            identity_links: None,
            dm_scope: Some(DmScope::PerPeer),
        };
        assert_eq!(
            build_agent_peer_session_key(&params),
            "agent:bot:direct:+1234567890"
        );
    }

    #[test]
    fn peer_key_direct_per_channel_peer() {
        let params = PeerSessionKeyParams {
            agent_id: "bot",
            main_key: None,
            channel: "twilio",
            account_id: None,
            peer_kind: Some(&ChatType::Direct),
            peer_id: Some("+1234567890"),
            identity_links: None,
            dm_scope: Some(DmScope::PerChannelPeer),
        };
        assert_eq!(
            build_agent_peer_session_key(&params),
            "agent:bot:twilio:direct:+1234567890"
        );
    }

    #[test]
    fn peer_key_direct_per_account_channel_peer() {
        let params = PeerSessionKeyParams {
            agent_id: "bot",
            main_key: None,
            channel: "twilio",
            account_id: Some("acct1"),
            peer_kind: Some(&ChatType::Direct),
            peer_id: Some("+1234567890"),
            identity_links: None,
            dm_scope: Some(DmScope::PerAccountChannelPeer),
        };
        assert_eq!(
            build_agent_peer_session_key(&params),
            "agent:bot:twilio:acct1:direct:+1234567890"
        );
    }

    #[test]
    fn peer_key_direct_no_peer_id_falls_to_main() {
        let params = PeerSessionKeyParams {
            agent_id: "bot",
            main_key: None,
            channel: "twilio",
            account_id: None,
            peer_kind: Some(&ChatType::Direct),
            peer_id: Some(""),
            identity_links: None,
            dm_scope: Some(DmScope::PerPeer),
        };
        assert_eq!(
            build_agent_peer_session_key(&params),
            "agent:bot:main"
        );
    }

    #[test]
    fn peer_key_group_unknown_peer() {
        let params = PeerSessionKeyParams {
            agent_id: "bot",
            main_key: None,
            channel: "discord",
            account_id: None,
            peer_kind: Some(&ChatType::Group),
            peer_id: None,
            identity_links: None,
            dm_scope: None,
        };
        assert_eq!(
            build_agent_peer_session_key(&params),
            "agent:bot:discord:group:unknown"
        );
    }

    // ── resolve_agent_id_from_session_key ──

    #[test]
    fn resolve_agent_id_from_valid() {
        assert_eq!(
            resolve_agent_id_from_session_key(Some("agent:mybot:main")),
            "mybot"
        );
    }

    #[test]
    fn resolve_agent_id_from_missing() {
        assert_eq!(
            resolve_agent_id_from_session_key(None),
            DEFAULT_AGENT_ID
        );
    }

    #[test]
    fn resolve_agent_id_from_legacy() {
        assert_eq!(
            resolve_agent_id_from_session_key(Some("some-legacy-key")),
            DEFAULT_AGENT_ID
        );
    }

    // ── classify_session_key_shape ──

    #[test]
    fn classify_missing() {
        assert_eq!(classify_session_key_shape(None), SessionKeyShape::Missing);
        assert_eq!(
            classify_session_key_shape(Some("")),
            SessionKeyShape::Missing
        );
    }

    #[test]
    fn classify_agent() {
        assert_eq!(
            classify_session_key_shape(Some("agent:bot:main")),
            SessionKeyShape::Agent
        );
    }

    #[test]
    fn classify_malformed_agent() {
        assert_eq!(
            classify_session_key_shape(Some("agent:oops")),
            SessionKeyShape::MalformedAgent
        );
    }

    #[test]
    fn classify_legacy() {
        assert_eq!(
            classify_session_key_shape(Some("some-old-key")),
            SessionKeyShape::LegacyOrAlias
        );
    }

    // ── to_agent_request_session_key ──

    #[test]
    fn request_key_from_store() {
        assert_eq!(
            to_agent_request_session_key(Some("agent:bot:main")),
            Some("main".to_owned())
        );
    }

    #[test]
    fn request_key_from_legacy() {
        assert_eq!(
            to_agent_request_session_key(Some("legacy-key")),
            Some("legacy-key".to_owned())
        );
    }

    #[test]
    fn request_key_from_empty() {
        assert_eq!(to_agent_request_session_key(None), None);
        assert_eq!(to_agent_request_session_key(Some("")), None);
    }

    // ── to_agent_store_session_key ──

    #[test]
    fn store_key_from_blank_request() {
        assert_eq!(
            to_agent_store_session_key("bot", None, None),
            "agent:bot:main"
        );
    }

    #[test]
    fn store_key_from_main_request() {
        assert_eq!(
            to_agent_store_session_key("bot", Some("main"), None),
            "agent:bot:main"
        );
    }

    #[test]
    fn store_key_from_agent_prefixed() {
        assert_eq!(
            to_agent_store_session_key("bot", Some("agent:other:work"), None),
            "agent:other:work"
        );
    }

    #[test]
    fn store_key_from_subagent_prefixed() {
        assert_eq!(
            to_agent_store_session_key("bot", Some("subagent:helper:task"), None),
            "agent:bot:subagent:helper:task"
        );
    }

    #[test]
    fn store_key_from_plain_request() {
        assert_eq!(
            to_agent_store_session_key("bot", Some("custom-key"), None),
            "agent:bot:custom-key"
        );
    }

    // ── build_group_history_key ──

    #[test]
    fn group_history_key() {
        assert_eq!(
            build_group_history_key("discord", Some("acct1"), "group", "general"),
            "discord:acct1:group:general"
        );
    }

    #[test]
    fn group_history_key_defaults() {
        assert_eq!(
            build_group_history_key("", None, "channel", ""),
            "unknown:default:channel:unknown"
        );
    }

    // ── resolve_thread_session_keys ──

    #[test]
    fn thread_keys_no_thread() {
        let result = resolve_thread_session_keys("agent:bot:main", None, None, None);
        assert_eq!(result.session_key, "agent:bot:main");
        assert_eq!(result.parent_session_key, None);
    }

    #[test]
    fn thread_keys_with_suffix() {
        let result = resolve_thread_session_keys(
            "agent:bot:main",
            Some("T123"),
            Some("agent:bot:main".to_owned()),
            None,
        );
        assert_eq!(result.session_key, "agent:bot:main:thread:t123");
        assert_eq!(
            result.parent_session_key,
            Some("agent:bot:main".to_owned())
        );
    }

    #[test]
    fn thread_keys_without_suffix() {
        let result = resolve_thread_session_keys(
            "agent:bot:main",
            Some("T123"),
            Some("agent:bot:main".to_owned()),
            Some(false),
        );
        assert_eq!(result.session_key, "agent:bot:main");
        assert_eq!(
            result.parent_session_key,
            Some("agent:bot:main".to_owned())
        );
    }

    // ── resolve_thread_parent_session_key ──

    #[test]
    fn thread_parent_with_thread_marker() {
        assert_eq!(
            resolve_thread_parent_session_key(Some("agent:bot:main:thread:t123")),
            Some("agent:bot:main".to_owned())
        );
    }

    #[test]
    fn thread_parent_with_topic_marker() {
        assert_eq!(
            resolve_thread_parent_session_key(Some("agent:bot:main:topic:t456")),
            Some("agent:bot:main".to_owned())
        );
    }

    #[test]
    fn thread_parent_no_marker() {
        assert_eq!(
            resolve_thread_parent_session_key(Some("agent:bot:main")),
            None
        );
    }

    #[test]
    fn thread_parent_empty() {
        assert_eq!(resolve_thread_parent_session_key(None), None);
        assert_eq!(resolve_thread_parent_session_key(Some("")), None);
    }

    // ── identity links ──

    #[test]
    fn peer_key_with_identity_link() {
        let mut links = HashMap::new();
        links.insert(
            "alice".to_owned(),
            vec!["+1234567890".to_owned(), "twilio:+1234567890".to_owned()],
        );
        let params = PeerSessionKeyParams {
            agent_id: "bot",
            main_key: None,
            channel: "twilio",
            account_id: None,
            peer_kind: Some(&ChatType::Direct),
            peer_id: Some("+1234567890"),
            identity_links: Some(&links),
            dm_scope: Some(DmScope::PerPeer),
        };
        assert_eq!(
            build_agent_peer_session_key(&params),
            "agent:bot:direct:alice"
        );
    }
}
