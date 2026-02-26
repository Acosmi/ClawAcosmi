// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Viking URI parser and path utilities.
//!
//! Ported from `openviking_cli/utils/uri.py` + `viking_fs.py` static helpers.
//! Zero IO — pure string manipulation only.

use std::fmt;

use serde::{Deserialize, Serialize};

// ---------------------------------------------------------------------------
// Scope enum
// ---------------------------------------------------------------------------

/// Valid scopes in the Viking URI namespace.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Scope {
    /// Independent resource scope.
    Resources,
    /// User scope (long-term memory).
    User,
    /// Agent scope (skills, instructions, patterns).
    Agent,
    /// Session scope (single conversation).
    Session,
    /// Queue scope.
    Queue,
    /// Temporary scope.
    Temp,
}

impl Scope {
    /// Parse a scope string. Returns `None` for invalid scopes.
    #[must_use]
    pub fn from_str_opt(s: &str) -> Option<Self> {
        match s {
            "resources" => Some(Self::Resources),
            "user" => Some(Self::User),
            "agent" => Some(Self::Agent),
            "session" => Some(Self::Session),
            "queue" => Some(Self::Queue),
            "temp" => Some(Self::Temp),
            _ => None,
        }
    }

    /// Scope as lowercase string slice.
    #[must_use]
    pub const fn as_str(&self) -> &'static str {
        match self {
            Self::Resources => "resources",
            Self::User => "user",
            Self::Agent => "agent",
            Self::Session => "session",
            Self::Queue => "queue",
            Self::Temp => "temp",
        }
    }
}

impl fmt::Display for Scope {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

// ---------------------------------------------------------------------------
// Error type
// ---------------------------------------------------------------------------

/// Errors that can occur during URI parsing.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum UriError {
    /// URI does not start with `viking://`.
    MissingScheme,
    /// Scope component is not one of the valid scopes.
    InvalidScope(String),
    /// URI is structurally malformed.
    MalformedUri(String),
}

impl fmt::Display for UriError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::MissingScheme => write!(f, "URI must start with 'viking://'"),
            Self::InvalidScope(s) => write!(f, "Invalid scope '{s}'"),
            Self::MalformedUri(s) => write!(f, "Malformed URI: {s}"),
        }
    }
}

impl std::error::Error for UriError {}

// ---------------------------------------------------------------------------
// VikingUri
// ---------------------------------------------------------------------------

const SCHEME_PREFIX: &str = "viking://";

/// Parsed Viking URI — `viking://<scope>/<path>`.
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub struct VikingUri {
    /// The full original URI string.
    raw: String,
    /// Parsed scope.
    scope: Scope,
}

impl VikingUri {
    /// Parse a Viking URI string.
    pub fn parse(uri: impl Into<String>) -> Result<Self, UriError> {
        let raw = uri.into();
        if !raw.starts_with(SCHEME_PREFIX) {
            return Err(UriError::MissingScheme);
        }
        let after = &raw[SCHEME_PREFIX.len()..];
        let scope_str = after.split('/').next().unwrap_or("");
        if scope_str.is_empty() {
            return Err(UriError::MalformedUri("empty scope".into()));
        }
        let scope = Scope::from_str_opt(scope_str)
            .ok_or_else(|| UriError::InvalidScope(scope_str.into()))?;
        Ok(Self { raw, scope })
    }

    /// Get the scope.
    #[must_use]
    pub fn scope(&self) -> Scope {
        self.scope
    }

    /// Full URI string.
    #[must_use]
    pub fn as_str(&self) -> &str {
        &self.raw
    }

    /// Full path after `viking://` (e.g. `"user/memories/preferences"`).
    #[must_use]
    pub fn full_path(&self) -> &str {
        &self.raw[SCHEME_PREFIX.len()..]
    }

    /// Resource name for resources scope (second component).
    #[must_use]
    pub fn resource_name(&self) -> Option<&str> {
        if self.scope != Scope::Resources {
            return None;
        }
        self.full_path().split('/').nth(1)
    }

    /// Check if this URI starts with the given prefix.
    #[must_use]
    pub fn matches_prefix(&self, prefix: &str) -> bool {
        self.raw.starts_with(prefix)
    }

    /// Get parent URI (one level up). Returns `None` at scope root.
    #[must_use]
    pub fn parent(&self) -> Option<Self> {
        let trimmed = self.raw.trim_end_matches('/');
        let after = &trimmed[SCHEME_PREFIX.len()..];
        if !after.contains('/') {
            return None; // Already at scope root
        }
        let last_slash = trimmed.rfind('/')?;
        Self::parse(&trimmed[..last_slash]).ok()
    }

    /// Join a relative part to this URI.
    pub fn join(&self, part: &str) -> Result<Self, UriError> {
        if part.is_empty() {
            return Ok(self.clone());
        }
        let base = self.raw.trim_end_matches('/');
        let part = part.trim_matches('/');
        if part.is_empty() {
            return Ok(self.clone());
        }
        Self::parse(format!("{base}/{part}"))
    }

    /// Check if a URI string is valid.
    #[must_use]
    pub fn is_valid(uri: &str) -> bool {
        Self::parse(uri.to_owned()).is_ok()
    }

    /// Build a URI from scope and path parts.
    pub fn build(scope: Scope, parts: &[&str]) -> Result<Self, UriError> {
        let mut segments: Vec<&str> = vec![scope.as_str()];
        segments.extend(parts.iter().filter(|p| !p.is_empty()));
        Self::parse(format!("{SCHEME_PREFIX}{}", segments.join("/")))
    }

    /// Build a semantic URI under the given parent.
    pub fn build_semantic(
        parent: &str,
        semantic_name: &str,
        node_id: Option<&str>,
        is_leaf: bool,
    ) -> Result<Self, UriError> {
        let safe = sanitize_segment(semantic_name);
        if is_leaf {
            let nid = node_id
                .ok_or_else(|| UriError::MalformedUri("Leaf node must have a node_id".into()))?;
            Self::parse(format!("{}/{}/{}", parent.trim_end_matches('/'), safe, nid))
        } else {
            Self::parse(format!("{}/{}", parent.trim_end_matches('/'), safe))
        }
    }

    /// Normalize a URI string — ensure `viking://` prefix.
    #[must_use]
    pub fn normalize(uri: &str) -> String {
        if uri.starts_with(SCHEME_PREFIX) {
            return uri.to_owned();
        }
        let stripped = uri.trim_start_matches('/');
        format!("{SCHEME_PREFIX}{stripped}")
    }

    /// Create a temporary URI: `viking://temp/MMDDHHMM_XXXXXX`.
    ///
    /// Port of `VikingURI.create_temp_uri()`.
    #[must_use]
    pub fn create_temp_uri() -> Self {
        use chrono::Utc;
        use uuid::Uuid;
        let now = Utc::now();
        let ts = now.format("%m%d%H%M");
        let id = &Uuid::new_v4().simple().to_string()[..6];
        // Safety: "temp" is a valid scope, so unwrap is safe.
        Self::parse(format!("{SCHEME_PREFIX}temp/{ts}_{id}")).unwrap()
    }
}

impl fmt::Display for VikingUri {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.raw)
    }
}

impl Serialize for VikingUri {
    fn serialize<S: serde::Serializer>(&self, serializer: S) -> Result<S::Ok, S::Error> {
        serializer.serialize_str(&self.raw)
    }
}

impl<'de> Deserialize<'de> for VikingUri {
    fn deserialize<D: serde::Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        let s = String::deserialize(deserializer)?;
        Self::parse(s).map_err(serde::de::Error::custom)
    }
}

// ---------------------------------------------------------------------------
// Sanitize segment
// ---------------------------------------------------------------------------

/// Sanitize text for use as a URI segment.
///
/// Preserves letters, digits, underscores, hyphens, and CJK characters.
/// Replaces everything else with `_`, merges consecutive underscores,
/// strips leading/trailing underscores, and truncates to 50 chars.
#[must_use]
pub fn sanitize_segment(text: &str) -> String {
    let mut out = String::with_capacity(text.len());
    for ch in text.chars() {
        if ch.is_alphanumeric() || ch == '_' || ch == '-' || is_cjk(ch) {
            out.push(ch);
        } else {
            out.push('_');
        }
    }
    // Merge consecutive underscores
    let mut merged = String::with_capacity(out.len());
    let mut prev_underscore = false;
    for ch in out.chars() {
        if ch == '_' {
            if !prev_underscore {
                merged.push('_');
            }
            prev_underscore = true;
        } else {
            merged.push(ch);
            prev_underscore = false;
        }
    }
    // Strip leading/trailing underscores and limit to 50 chars
    let trimmed = merged.trim_matches('_');
    let result: String = trimmed.chars().take(50).collect();
    if result.is_empty() {
        "unnamed".to_owned()
    } else {
        result
    }
}

/// Check if a character is CJK (Chinese/Japanese/Korean).
fn is_cjk(ch: char) -> bool {
    matches!(ch,
        '\u{4e00}'..='\u{9fff}'   // CJK Unified Ideographs
        | '\u{3040}'..='\u{309f}' // Hiragana
        | '\u{30a0}'..='\u{30ff}' // Katakana
        | '\u{ac00}'..='\u{d7af}' // Hangul Syllables
        | '\u{3400}'..='\u{4dbf}' // CJK Extension A
    )
}

// ---------------------------------------------------------------------------
// URI ↔ filesystem path conversion
// ---------------------------------------------------------------------------

/// Convert a Viking URI to a filesystem path (`/local/...`).
///
/// Port of `VikingFS._uri_to_path()`.
#[must_use]
pub fn uri_to_fs_path(uri: &str) -> String {
    let remainder = uri[SCHEME_PREFIX.len()..].trim_matches('/');
    if remainder.is_empty() {
        return "/local".to_owned();
    }
    let safe_parts: Vec<String> = remainder
        .split('/')
        .map(|p| shorten_component(p, 255))
        .collect();
    format!("/local/{}", safe_parts.join("/"))
}

/// Convert a filesystem path back to a Viking URI.
///
/// Port of `VikingFS._path_to_uri()`.
#[must_use]
pub fn fs_path_to_uri(path: &str) -> String {
    if path.starts_with(SCHEME_PREFIX) {
        path.to_owned()
    } else if let Some(rest) = path.strip_prefix("/local/") {
        format!("{SCHEME_PREFIX}{rest}")
    } else if let Some(rest) = path.strip_prefix('/') {
        format!("viking:/{rest}")
    } else {
        format!("{SCHEME_PREFIX}{path}")
    }
}

/// Shorten a path component if its UTF-8 bytes exceed `max_bytes`.
///
/// Appends a `_<sha256[:8]>` suffix when truncation is needed.
/// Port of `VikingFS._shorten_component()`.
#[must_use]
pub fn shorten_component(component: &str, max_bytes: usize) -> String {
    if component.len() <= max_bytes {
        return component.to_owned();
    }
    // Build hash suffix
    use std::collections::hash_map::DefaultHasher;
    use std::hash::{Hash, Hasher};
    let mut hasher = DefaultHasher::new();
    component.hash(&mut hasher);
    let hash_suffix = format!("{:016x}", hasher.finish());
    let suffix = format!("_{}", &hash_suffix[..8]);
    let target = max_bytes - suffix.len();
    // Trim component to fit, respecting char boundaries
    let mut prefix = String::new();
    for ch in component.chars() {
        if prefix.len() + ch.len_utf8() > target {
            break;
        }
        prefix.push(ch);
    }
    format!("{prefix}{suffix}")
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_valid_scopes() {
        for scope_str in &["resources", "user", "agent", "session", "queue", "temp"] {
            let uri = format!("viking://{scope_str}/test");
            let parsed = VikingUri::parse(&uri).unwrap();
            assert_eq!(parsed.scope().as_str(), *scope_str);
        }
    }

    #[test]
    fn parse_invalid() {
        assert!(VikingUri::parse("http://user/test").is_err());
        assert!(VikingUri::parse("viking://invalid_scope/x").is_err());
        assert!(VikingUri::parse("viking://").is_err());
    }

    #[test]
    fn parent_navigation() {
        let uri = VikingUri::parse("viking://user/memories/preferences/code").unwrap();
        let p1 = uri.parent().unwrap();
        assert_eq!(p1.as_str(), "viking://user/memories/preferences");
        let p2 = p1.parent().unwrap();
        assert_eq!(p2.as_str(), "viking://user/memories");
        let p3 = p2.parent().unwrap();
        assert_eq!(p3.as_str(), "viking://user");
        assert!(p3.parent().is_none()); // scope root
    }

    #[test]
    fn join_parts() {
        let base = VikingUri::parse("viking://user/memories").unwrap();
        let joined = base.join("preferences/code").unwrap();
        assert_eq!(joined.as_str(), "viking://user/memories/preferences/code");
        // Empty join returns same
        let same = base.join("").unwrap();
        assert_eq!(same.as_str(), base.as_str());
    }

    #[test]
    fn build_uri() {
        let uri = VikingUri::build(Scope::Agent, &["skills", "search"]).unwrap();
        assert_eq!(uri.as_str(), "viking://agent/skills/search");
    }

    #[test]
    fn sanitize_cjk() {
        assert_eq!(sanitize_segment("你好世界"), "你好世界");
        assert_eq!(sanitize_segment("hello world!@#"), "hello_world");
        assert_eq!(sanitize_segment("   "), "unnamed");
    }

    #[test]
    fn uri_path_roundtrip() {
        let uri = "viking://user/memories/preferences/code-style";
        let path = uri_to_fs_path(uri);
        assert_eq!(path, "/local/user/memories/preferences/code-style");
        let back = fs_path_to_uri(&path);
        assert_eq!(back, uri);
    }

    #[test]
    fn shorten_long_component() {
        let short = "hello";
        assert_eq!(shorten_component(short, 255), "hello");
        // A very long component should be shortened
        let long: String = "x".repeat(300);
        let result = shorten_component(&long, 255);
        assert!(result.len() <= 255);
        assert!(result.contains('_')); // has hash suffix
    }

    #[test]
    fn normalize_uri() {
        assert_eq!(
            VikingUri::normalize("viking://user/test"),
            "viking://user/test"
        );
        assert_eq!(
            VikingUri::normalize("/resources/docs"),
            "viking://resources/docs"
        );
        assert_eq!(
            VikingUri::normalize("resources/docs"),
            "viking://resources/docs"
        );
    }

    #[test]
    fn serde_roundtrip() {
        let uri = VikingUri::parse("viking://agent/skills/search").unwrap();
        let json = serde_json::to_string(&uri).unwrap();
        assert_eq!(json, "\"viking://agent/skills/search\"");
        let restored: VikingUri = serde_json::from_str(&json).unwrap();
        assert_eq!(restored, uri);
    }
}
