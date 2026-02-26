/// Gateway configuration prompts and result types.
///
/// Handles interactive prompting for gateway port, bind mode, auth mode,
/// tailscale exposure settings, and custom bind hosts.
///
/// Source: `src/commands/configure.gateway.ts`

use oa_types::config::OpenAcosmiConfig;
use oa_types::gateway::{
    GatewayBindMode, GatewayConfig, GatewayTailscaleConfig, GatewayTailscaleMode,
};
use serde::{Deserialize, Serialize};

use crate::gateway_auth::{build_gateway_auth_config, BuildGatewayAuthParams, GatewayAuthChoice};

/// Result of the gateway configuration prompt.
///
/// Source: `src/commands/configure.gateway.ts` - `promptGatewayConfig` return type
pub struct GatewayConfigResult {
    /// Updated config with gateway settings applied.
    pub config: OpenAcosmiConfig,
    /// The configured gateway port.
    pub port: u16,
    /// The gateway token (if token auth was selected).
    pub token: Option<String>,
}

/// Bind mode selection options for the gateway.
///
/// Source: `src/commands/configure.gateway.ts` - bind mode options
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum BindModeChoice {
    /// Bind to 127.0.0.1 (secure, local-only access).
    Loopback,
    /// Bind to Tailscale IP only (100.x.x.x).
    Tailnet,
    /// Prefer loopback; fall back to all interfaces.
    Auto,
    /// Bind to 0.0.0.0 (accessible from anywhere on the network).
    Lan,
    /// Specify a specific IP address.
    Custom,
}

impl From<BindModeChoice> for GatewayBindMode {
    fn from(choice: BindModeChoice) -> Self {
        match choice {
            BindModeChoice::Loopback => Self::Loopback,
            BindModeChoice::Tailnet => Self::Tailnet,
            BindModeChoice::Auto => Self::Auto,
            BindModeChoice::Lan => Self::Lan,
            BindModeChoice::Custom => Self::Custom,
        }
    }
}

/// Tailscale exposure mode selection.
///
/// Source: `src/commands/configure.gateway.ts` - tailscale mode options
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum TailscaleModeChoice {
    /// No Tailscale exposure.
    Off,
    /// Private HTTPS for your tailnet (devices on Tailscale).
    Serve,
    /// Public HTTPS via Tailscale Funnel (internet).
    Funnel,
}

impl From<TailscaleModeChoice> for GatewayTailscaleMode {
    fn from(choice: TailscaleModeChoice) -> Self {
        match choice {
            TailscaleModeChoice::Off => Self::Off,
            TailscaleModeChoice::Serve => Self::Serve,
            TailscaleModeChoice::Funnel => Self::Funnel,
        }
    }
}

/// Apply gateway configuration choices to produce a `GatewayConfigResult`.
///
/// Enforces constraints: tailscale requires bind=loopback, funnel requires
/// password auth. Constructs the complete `GatewayConfig` including auth,
/// tailscale, and optional custom bind host.
///
/// Source: `src/commands/configure.gateway.ts` - `promptGatewayConfig`
pub fn apply_gateway_config(
    cfg: OpenAcosmiConfig,
    port: u16,
    mut bind: BindModeChoice,
    mut auth_mode: GatewayAuthChoice,
    tailscale_mode: TailscaleModeChoice,
    tailscale_reset_on_exit: bool,
    custom_bind_host: Option<String>,
    gateway_token: Option<String>,
    gateway_password: Option<String>,
) -> GatewayConfigResult {
    // Tailscale requires bind=loopback
    if tailscale_mode != TailscaleModeChoice::Off && bind != BindModeChoice::Loopback {
        bind = BindModeChoice::Loopback;
    }

    // Tailscale funnel requires password auth
    if tailscale_mode == TailscaleModeChoice::Funnel && auth_mode != GatewayAuthChoice::Password {
        auth_mode = GatewayAuthChoice::Password;
    }

    let auth_config = build_gateway_auth_config(BuildGatewayAuthParams {
        existing: cfg.gateway.as_ref().and_then(|g| g.auth.clone()),
        mode: auth_mode,
        token: gateway_token.clone(),
        password: gateway_password,
    });

    let tailscale = GatewayTailscaleConfig {
        mode: Some(GatewayTailscaleMode::from(tailscale_mode)),
        reset_on_exit: Some(tailscale_reset_on_exit),
    };

    let custom_host = if bind == BindModeChoice::Custom {
        custom_bind_host.filter(|h| !h.is_empty())
    } else {
        None
    };

    let gateway = GatewayConfig {
        mode: Some(oa_types::gateway::GatewayMode::Local),
        port: Some(port),
        bind: Some(GatewayBindMode::from(bind)),
        auth: Some(auth_config),
        tailscale: Some(tailscale),
        custom_bind_host: custom_host,
        ..cfg.gateway.unwrap_or_default()
    };

    let next = OpenAcosmiConfig {
        gateway: Some(gateway),
        ..cfg
    };

    GatewayConfigResult {
        config: next,
        port,
        token: gateway_token,
    }
}

/// Validate an IPv4 address string.
///
/// Source: `src/commands/configure.gateway.ts` - custom IP validation
pub fn validate_ipv4(input: &str) -> bool {
    let parts: Vec<&str> = input.split('.').collect();
    if parts.len() != 4 {
        return false;
    }
    parts.iter().all(|part| {
        part.parse::<u16>()
            .ok()
            .is_some_and(|n| n <= 255 && part == &n.to_string())
    })
}

/// Validate a port string.
///
/// Source: `src/commands/configure.gateway.ts` - port validation
pub fn validate_port(input: &str) -> bool {
    input.parse::<u16>().ok().is_some_and(|p| p > 0)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn validate_ipv4_valid_addresses() {
        assert!(validate_ipv4("192.168.1.100"));
        assert!(validate_ipv4("0.0.0.0"));
        assert!(validate_ipv4("255.255.255.255"));
        assert!(validate_ipv4("127.0.0.1"));
    }

    #[test]
    fn validate_ipv4_invalid_addresses() {
        assert!(!validate_ipv4("256.0.0.1"));
        assert!(!validate_ipv4("192.168.1"));
        assert!(!validate_ipv4("192.168.1.1.1"));
        assert!(!validate_ipv4("abc.def.ghi.jkl"));
        assert!(!validate_ipv4(""));
        assert!(!validate_ipv4("01.02.03.04")); // Leading zeros
    }

    #[test]
    fn validate_port_valid() {
        assert!(validate_port("18789"));
        assert!(validate_port("1"));
        assert!(validate_port("65535"));
    }

    #[test]
    fn validate_port_invalid() {
        assert!(!validate_port("0"));
        assert!(!validate_port("-1"));
        assert!(!validate_port("abc"));
        assert!(!validate_port(""));
        assert!(!validate_port("99999"));
    }

    #[test]
    fn apply_gateway_config_basic() {
        let cfg = OpenAcosmiConfig::default();
        let result = apply_gateway_config(
            cfg,
            18789,
            BindModeChoice::Loopback,
            GatewayAuthChoice::Token,
            TailscaleModeChoice::Off,
            false,
            None,
            Some("test-token".to_string()),
            None,
        );
        assert_eq!(result.port, 18789);
        assert_eq!(result.token.as_deref(), Some("test-token"));
        let gw = result.config.gateway.as_ref().expect("gateway config");
        assert_eq!(gw.bind, Some(GatewayBindMode::Loopback));
    }

    #[test]
    fn apply_gateway_config_tailscale_forces_loopback() {
        let cfg = OpenAcosmiConfig::default();
        let result = apply_gateway_config(
            cfg,
            18789,
            BindModeChoice::Lan, // will be overridden
            GatewayAuthChoice::Token,
            TailscaleModeChoice::Serve,
            false,
            None,
            Some("tok".to_string()),
            None,
        );
        let gw = result.config.gateway.as_ref().expect("gateway config");
        assert_eq!(gw.bind, Some(GatewayBindMode::Loopback));
    }

    #[test]
    fn apply_gateway_config_funnel_forces_password() {
        let cfg = OpenAcosmiConfig::default();
        let result = apply_gateway_config(
            cfg,
            18789,
            BindModeChoice::Loopback,
            GatewayAuthChoice::Token, // will be overridden
            TailscaleModeChoice::Funnel,
            true,
            None,
            None,
            Some("pass".to_string()),
        );
        let gw = result.config.gateway.as_ref().expect("gateway config");
        let auth = gw.auth.as_ref().expect("auth config");
        assert_eq!(
            auth.mode,
            Some(oa_types::gateway::GatewayAuthMode::Password)
        );
    }

    #[test]
    fn bind_mode_choice_converts_to_gateway_bind_mode() {
        assert_eq!(GatewayBindMode::from(BindModeChoice::Loopback), GatewayBindMode::Loopback);
        assert_eq!(GatewayBindMode::from(BindModeChoice::Lan), GatewayBindMode::Lan);
        assert_eq!(GatewayBindMode::from(BindModeChoice::Auto), GatewayBindMode::Auto);
        assert_eq!(GatewayBindMode::from(BindModeChoice::Custom), GatewayBindMode::Custom);
        assert_eq!(GatewayBindMode::from(BindModeChoice::Tailnet), GatewayBindMode::Tailnet);
    }
}
