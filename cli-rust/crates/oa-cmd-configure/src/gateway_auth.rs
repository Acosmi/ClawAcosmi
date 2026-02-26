/// Gateway authentication configuration builder.
///
/// Builds `GatewayAuthConfig` objects from wizard selections, preserving
/// existing tailscale settings while applying new auth mode and credentials.
///
/// Source: `src/commands/configure.gateway-auth.ts`

use oa_types::gateway::{GatewayAuthConfig, GatewayAuthMode};

/// The auth mode choice for gateway access.
///
/// Source: `src/commands/configure.gateway-auth.ts` - `GatewayAuthChoice`
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum GatewayAuthChoice {
    /// Token-based authentication (recommended default).
    Token,
    /// Password-based authentication.
    Password,
}

/// Model keys eligible for Anthropic OAuth model allowlist.
///
/// Source: `src/commands/configure.gateway-auth.ts` - `ANTHROPIC_OAUTH_MODEL_KEYS`
pub const ANTHROPIC_OAUTH_MODEL_KEYS: &[&str] = &[
    "anthropic/claude-opus-4-6",
    "anthropic/claude-opus-4-5",
    "anthropic/claude-sonnet-4-5",
    "anthropic/claude-haiku-4-5",
];

/// Parameters for building a gateway auth config.
///
/// Source: `src/commands/configure.gateway-auth.ts` - `buildGatewayAuthConfig` params
pub struct BuildGatewayAuthParams {
    /// Existing auth config to preserve tailscale settings from.
    pub existing: Option<GatewayAuthConfig>,
    /// Chosen authentication mode.
    pub mode: GatewayAuthChoice,
    /// Token value (used when mode is `Token`).
    pub token: Option<String>,
    /// Password value (used when mode is `Password`).
    pub password: Option<String>,
}

/// Build a `GatewayAuthConfig` from wizard selections, preserving tailscale settings.
///
/// Merges the `allow_tailscale` flag from the existing config (if any) with
/// the new auth mode and corresponding credential.
///
/// Source: `src/commands/configure.gateway-auth.ts` - `buildGatewayAuthConfig`
pub fn build_gateway_auth_config(params: BuildGatewayAuthParams) -> GatewayAuthConfig {
    let allow_tailscale = params
        .existing
        .as_ref()
        .and_then(|existing| existing.allow_tailscale);

    match params.mode {
        GatewayAuthChoice::Token => GatewayAuthConfig {
            mode: Some(GatewayAuthMode::Token),
            token: params.token,
            password: None,
            allow_tailscale,
        },
        GatewayAuthChoice::Password => GatewayAuthConfig {
            mode: Some(GatewayAuthMode::Password),
            token: None,
            password: params.password,
            allow_tailscale,
        },
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn build_token_auth_config() {
        let config = build_gateway_auth_config(BuildGatewayAuthParams {
            existing: None,
            mode: GatewayAuthChoice::Token,
            token: Some("my-token".to_string()),
            password: None,
        });
        assert_eq!(config.mode, Some(GatewayAuthMode::Token));
        assert_eq!(config.token.as_deref(), Some("my-token"));
        assert!(config.password.is_none());
        assert!(config.allow_tailscale.is_none());
    }

    #[test]
    fn build_password_auth_config() {
        let config = build_gateway_auth_config(BuildGatewayAuthParams {
            existing: None,
            mode: GatewayAuthChoice::Password,
            token: None,
            password: Some("secret".to_string()),
        });
        assert_eq!(config.mode, Some(GatewayAuthMode::Password));
        assert!(config.token.is_none());
        assert_eq!(config.password.as_deref(), Some("secret"));
    }

    #[test]
    fn preserves_allow_tailscale_from_existing() {
        let existing = GatewayAuthConfig {
            mode: Some(GatewayAuthMode::Token),
            token: Some("old-token".to_string()),
            password: None,
            allow_tailscale: Some(true),
        };
        let config = build_gateway_auth_config(BuildGatewayAuthParams {
            existing: Some(existing),
            mode: GatewayAuthChoice::Password,
            token: None,
            password: Some("new-pass".to_string()),
        });
        assert_eq!(config.allow_tailscale, Some(true));
        assert_eq!(config.mode, Some(GatewayAuthMode::Password));
    }

    #[test]
    fn no_existing_config_leaves_tailscale_none() {
        let config = build_gateway_auth_config(BuildGatewayAuthParams {
            existing: None,
            mode: GatewayAuthChoice::Token,
            token: Some("t".to_string()),
            password: None,
        });
        assert!(config.allow_tailscale.is_none());
    }

    #[test]
    fn anthropic_oauth_model_keys_count() {
        assert_eq!(ANTHROPIC_OAUTH_MODEL_KEYS.len(), 4);
        assert!(ANTHROPIC_OAUTH_MODEL_KEYS.contains(&"anthropic/claude-opus-4-6"));
    }
}
