/// Gateway authentication for the client side.
///
/// Provides types and helpers for resolving explicit gateway auth
/// credentials (token or password) from CLI options and configuration,
/// and validating that required credentials are present when connecting
/// to a remote gateway URL.
///
/// The server-side authorization logic (`authorizeGatewayConnect`) is NOT
/// ported here -- this crate focuses on the client perspective.
///
/// Source: `src/gateway/auth.ts`, `src/gateway/call.ts`

use serde::{Deserialize, Serialize};

// ---------------------------------------------------------------------------
// Auth mode
// ---------------------------------------------------------------------------

/// Resolved gateway authentication mode.
///
/// Determines whether a token or password is used for gateway auth.
///
/// Source: `src/gateway/auth.ts` (`ResolvedGatewayAuthMode`)
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ResolvedGatewayAuthMode {
    /// Token-based authentication.
    Token,
    /// Password-based authentication.
    Password,
}

// ---------------------------------------------------------------------------
// Resolved auth
// ---------------------------------------------------------------------------

/// Fully resolved gateway authentication configuration.
///
/// Built from the config file, environment variables, and CLI overrides.
/// Used on the server side to decide how to authorize incoming connections.
///
/// Source: `src/gateway/auth.ts` (`ResolvedGatewayAuth`)
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ResolvedGatewayAuth {
    /// Authentication mode.
    pub mode: ResolvedGatewayAuthMode,
    /// Token credential (if mode is `Token`).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub token: Option<String>,
    /// Password credential (if mode is `Password`).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub password: Option<String>,
    /// Whether Tailscale identity auth is allowed as a fallback.
    pub allow_tailscale: bool,
}

// ---------------------------------------------------------------------------
// Auth result
// ---------------------------------------------------------------------------

/// Result of a gateway authentication attempt.
///
/// Returned by the server after evaluating the connect handshake credentials.
/// On the client side, this is received as part of the connect error flow
/// (e.g., "token_mismatch" reason).
///
/// Source: `src/gateway/auth.ts` (`GatewayAuthResult`)
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct GatewayAuthResult {
    /// Whether authentication succeeded.
    pub ok: bool,
    /// Authentication method used (e.g., "token", "password", "tailscale", "device-token").
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub method: Option<String>,
    /// Authenticated user identity (e.g., Tailscale login).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub user: Option<String>,
    /// Reason for failure (e.g., "token_missing", "password_mismatch").
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub reason: Option<String>,
}

// ---------------------------------------------------------------------------
// Explicit auth (client side)
// ---------------------------------------------------------------------------

/// Explicit gateway authentication credentials provided by the caller.
///
/// Used on the client side when making `call_gateway()` requests.
/// One of `token` or `password` is typically required when connecting
/// to a non-local gateway.
///
/// Source: `src/gateway/call.ts` (`ExplicitGatewayAuth`)
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ExplicitGatewayAuth {
    /// Bearer token.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub token: Option<String>,
    /// Password credential.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub password: Option<String>,
}

/// Resolve explicit gateway auth from optional user-provided values.
///
/// Trims whitespace and treats empty strings as `None`.
///
/// Source: `src/gateway/call.ts` (`resolveExplicitGatewayAuth`)
#[must_use]
pub fn resolve_explicit_gateway_auth(opts: Option<&ExplicitGatewayAuth>) -> ExplicitGatewayAuth {
    let token = opts
        .and_then(|o| o.token.as_deref())
        .map(str::trim)
        .filter(|s| !s.is_empty())
        .map(String::from);
    let password = opts
        .and_then(|o| o.password.as_deref())
        .map(str::trim)
        .filter(|s| !s.is_empty())
        .map(String::from);
    ExplicitGatewayAuth { token, password }
}

/// Ensure explicit gateway auth is present when a URL override is provided.
///
/// When connecting to a non-local (overridden) gateway URL, explicit
/// credentials are required. This function returns an error message
/// if the URL is overridden but no credentials are present.
///
/// Source: `src/gateway/call.ts` (`ensureExplicitGatewayAuth`)
pub fn ensure_explicit_gateway_auth(
    url_override: Option<&str>,
    auth: &ExplicitGatewayAuth,
    error_hint: &str,
    config_path: Option<&str>,
) -> Result<(), String> {
    // Only enforce when a URL override is present
    let url = match url_override {
        Some(u) if !u.trim().is_empty() => u,
        _ => return Ok(()),
    };
    let _ = url; // Used for the check above

    if auth.token.is_some() || auth.password.is_some() {
        return Ok(());
    }

    let mut parts = vec![
        "gateway url override requires explicit credentials".to_string(),
        error_hint.to_string(),
    ];
    if let Some(path) = config_path {
        parts.push(format!("Config: {path}"));
    }

    Err(parts.into_iter().filter(|s| !s.is_empty()).collect::<Vec<_>>().join("\n"))
}

// ---------------------------------------------------------------------------
// Loopback check (re-export from net for convenience)
// ---------------------------------------------------------------------------

/// Check whether an IP address string represents a loopback address.
///
/// Convenience re-export of [`crate::net::is_loopback_address`] for backward
/// compatibility with code that imported this from the auth module.
///
/// Source: `src/gateway/auth.ts` (`isLoopbackAddress`)
#[must_use]
pub fn is_loopback_address(ip: Option<&str>) -> bool {
    crate::net::is_loopback_address(ip)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn resolve_explicit_auth_trims() {
        let input = ExplicitGatewayAuth {
            token: Some("  my-token  ".to_string()),
            password: Some("".to_string()),
        };
        let resolved = resolve_explicit_gateway_auth(Some(&input));
        assert_eq!(resolved.token.as_deref(), Some("my-token"));
        assert!(resolved.password.is_none());
    }

    #[test]
    fn resolve_explicit_auth_none() {
        let resolved = resolve_explicit_gateway_auth(None);
        assert!(resolved.token.is_none());
        assert!(resolved.password.is_none());
    }

    #[test]
    fn ensure_auth_no_url_override() {
        let auth = ExplicitGatewayAuth::default();
        let result = ensure_explicit_gateway_auth(None, &auth, "hint", None);
        assert!(result.is_ok());
    }

    #[test]
    fn ensure_auth_url_override_no_creds() {
        let auth = ExplicitGatewayAuth::default();
        let result = ensure_explicit_gateway_auth(
            Some("wss://remote.example.com"),
            &auth,
            "Fix: pass --token or --password.",
            Some("/path/to/config.json"),
        );
        assert!(result.is_err());
        let msg = result.err().unwrap_or_default();
        assert!(msg.contains("explicit credentials"));
        assert!(msg.contains("Config:"));
    }

    #[test]
    fn ensure_auth_url_override_with_token() {
        let auth = ExplicitGatewayAuth {
            token: Some("secret".to_string()),
            password: None,
        };
        let result = ensure_explicit_gateway_auth(
            Some("wss://remote.example.com"),
            &auth,
            "hint",
            None,
        );
        assert!(result.is_ok());
    }

    #[test]
    fn ensure_auth_url_override_with_password() {
        let auth = ExplicitGatewayAuth {
            token: None,
            password: Some("pass".to_string()),
        };
        let result = ensure_explicit_gateway_auth(
            Some("wss://remote.example.com"),
            &auth,
            "hint",
            None,
        );
        assert!(result.is_ok());
    }

    #[test]
    fn loopback_check_delegates() {
        assert!(is_loopback_address(Some("127.0.0.1")));
        assert!(!is_loopback_address(Some("10.0.0.1")));
    }

    #[test]
    fn auth_result_serializes() {
        let result = GatewayAuthResult {
            ok: false,
            method: None,
            user: None,
            reason: Some("token_mismatch".to_string()),
        };
        let json = serde_json::to_value(&result).ok();
        assert!(json.is_some());
        let val = json.unwrap_or_default();
        assert_eq!(val["ok"], false);
        assert_eq!(val["reason"], "token_mismatch");
    }

    #[test]
    fn resolved_auth_mode_serializes() {
        let mode = ResolvedGatewayAuthMode::Token;
        let json = serde_json::to_string(&mode).ok();
        assert_eq!(json.as_deref(), Some("\"token\""));

        let mode2 = ResolvedGatewayAuthMode::Password;
        let json2 = serde_json::to_string(&mode2).ok();
        assert_eq!(json2.as_deref(), Some("\"password\""));
    }
}
