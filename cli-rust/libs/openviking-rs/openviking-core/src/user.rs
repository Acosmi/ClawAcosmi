// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! User identity — multi-tenant account + user + agent triple.
//!
//! Ported from `openviking_cli/session/user_id.py`.

use serde::{Deserialize, Serialize};

/// A validated triple identifying a user within a tenant.
///
/// All components must be non-empty and match `[a-zA-Z0-9_-]+`.
#[derive(Debug, Clone, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct UserIdentifier {
    /// Tenant account ID.
    pub account_id: String,
    /// Individual user ID within the account.
    pub user_id: String,
    /// Agent ID the user is interacting with.
    pub agent_id: String,
    /// Preferred language code (e.g. "en", "zh-CN", "ja").
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub language: Option<String>,
}

/// Validation error for `UserIdentifier`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct UserIdError(pub String);

impl std::fmt::Display for UserIdError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.0)
    }
}

impl std::error::Error for UserIdError {}

/// Check that a string is non-empty and contains only `[a-zA-Z0-9_-]`.
fn is_valid_component(s: &str) -> bool {
    !s.is_empty()
        && s.bytes()
            .all(|b| b.is_ascii_alphanumeric() || b == b'_' || b == b'-')
}

impl UserIdentifier {
    /// Create and validate a new `UserIdentifier`.
    pub fn new(
        account_id: impl Into<String>,
        user_id: impl Into<String>,
        agent_id: impl Into<String>,
    ) -> Result<Self, UserIdError> {
        let id = Self {
            account_id: account_id.into(),
            user_id: user_id.into(),
            agent_id: agent_id.into(),
            language: None,
        };
        id.validate()?;
        Ok(id)
    }

    /// Default singleton user (`default:default:default`).
    #[must_use]
    pub fn default_user() -> Self {
        Self {
            account_id: "default".into(),
            user_id: "default".into(),
            agent_id: "default".into(),
            language: None,
        }
    }

    fn validate(&self) -> Result<(), UserIdError> {
        if !is_valid_component(&self.account_id) {
            return Err(UserIdError(
                "account_id must be non-empty alpha-numeric".into(),
            ));
        }
        if !is_valid_component(&self.user_id) {
            return Err(UserIdError(
                "user_id must be non-empty alpha-numeric".into(),
            ));
        }
        if !is_valid_component(&self.agent_id) {
            return Err(UserIdError(
                "agent_id must be non-empty alpha-numeric".into(),
            ));
        }
        Ok(())
    }

    /// Anonymized unique space name: `{account_id}_{md5(user+agent)[:8]}`.
    #[must_use]
    pub fn unique_space_name(&self) -> String {
        use std::collections::hash_map::DefaultHasher;
        use std::hash::{Hash, Hasher};
        let mut h = DefaultHasher::new();
        format!("{}{}", self.user_id, self.agent_id).hash(&mut h);
        let hash = format!("{:016x}", h.finish());
        format!("{}_{}", self.account_id, &hash[..8])
    }
}

impl std::fmt::Display for UserIdentifier {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}:{}:{}", self.account_id, self.user_id, self.agent_id)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn valid_creation() {
        let u = UserIdentifier::new("acme", "alice", "planner").unwrap();
        assert_eq!(u.account_id, "acme");
    }

    #[test]
    fn invalid_empty() {
        assert!(UserIdentifier::new("", "alice", "planner").is_err());
    }

    #[test]
    fn serde_roundtrip() {
        let u = UserIdentifier::default_user();
        let json = serde_json::to_string(&u).unwrap();
        let restored: UserIdentifier = serde_json::from_str(&json).unwrap();
        assert_eq!(restored, u);
    }

    #[test]
    fn unique_space_name_format() {
        let u = UserIdentifier::default_user();
        let name = u.unique_space_name();
        assert!(name.starts_with("default_"));
        assert_eq!(name.len(), "default_".len() + 8);
    }
}
