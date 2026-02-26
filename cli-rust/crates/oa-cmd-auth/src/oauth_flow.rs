/// OAuth flow helpers (VPS-aware handlers).
///
/// Provides the [`VpsAwareOAuthHandlers`] abstraction that adapts OAuth
/// flows for both local (browser-opens-automatically) and remote/VPS
/// (user must manually copy a URL and paste back a redirect URL) environments.
///
/// Source: `src/commands/oauth-flow.ts`

/// An OAuth authentication event containing the URL to open.
///
/// Source: `src/commands/oauth-flow.ts` - `onAuth` parameter
#[derive(Debug, Clone)]
pub struct OAuthAuthEvent {
    /// The authorization URL the user should open.
    pub url: String,
}

/// An OAuth prompt requesting text input from the user.
///
/// Source: `src/commands/oauth-flow.ts` - `OAuthPrompt`
#[derive(Debug, Clone)]
pub struct OAuthPrompt {
    /// The prompt message to display.
    pub message: String,
    /// Optional placeholder text.
    pub placeholder: Option<String>,
}

/// Configuration for creating VPS-aware OAuth handlers.
///
/// Source: `src/commands/oauth-flow.ts` - `createVpsAwareOAuthHandlers` params
#[derive(Debug, Clone)]
pub struct VpsAwareOAuthConfig {
    /// Whether the environment is remote/VPS.
    pub is_remote: bool,
    /// Message shown while browser is opening in local mode.
    pub local_browser_message: String,
    /// Custom prompt message for manual code entry (remote mode).
    pub manual_prompt_message: Option<String>,
}

impl Default for VpsAwareOAuthConfig {
    fn default() -> Self {
        Self {
            is_remote: false,
            local_browser_message: "Complete sign-in in browser\u{2026}".to_owned(),
            manual_prompt_message: None,
        }
    }
}

/// The resolved manual prompt message, using the configured override or the default.
///
/// Source: `src/commands/oauth-flow.ts` - `manualPromptMessage`
#[must_use]
pub fn resolve_manual_prompt_message(config: &VpsAwareOAuthConfig) -> String {
    config
        .manual_prompt_message
        .clone()
        .unwrap_or_else(|| "Paste the redirect URL (or authorization code)".to_owned())
}

/// Validate that a required input string is non-empty.
///
/// Returns `None` if the input is valid, or `Some("Required")` if it is empty.
///
/// Source: `src/commands/oauth-flow.ts` - `validateRequiredInput`
#[must_use]
pub fn validate_required_input(value: &str) -> Option<&'static str> {
    if value.trim().is_empty() {
        Some("Required")
    } else {
        None
    }
}

/// Instructions for performing an OAuth flow in a remote/VPS environment.
///
/// In remote mode, the user must manually open the URL in their local
/// browser and paste back the redirect URL. This struct captures the
/// formatted instructions.
///
/// Source: `src/commands/oauth-flow.ts` - `createVpsAwareOAuthHandlers` (remote path)
#[derive(Debug, Clone)]
pub struct RemoteOAuthInstructions {
    /// The authorization URL the user should open in their local browser.
    pub url: String,
    /// The prompt message for pasting the redirect URL back.
    pub prompt_message: String,
}

/// Build remote OAuth instructions from an auth event and config.
///
/// Source: `src/commands/oauth-flow.ts` - `createVpsAwareOAuthHandlers` (remote path)
#[must_use]
pub fn build_remote_instructions(
    event: &OAuthAuthEvent,
    config: &VpsAwareOAuthConfig,
) -> RemoteOAuthInstructions {
    RemoteOAuthInstructions {
        url: event.url.clone(),
        prompt_message: resolve_manual_prompt_message(config),
    }
}

/// Represents the two possible handler strategies.
///
/// In local mode, the browser is opened automatically and callbacks
/// are captured via a local HTTP server. In remote mode, the user
/// copies a URL and pastes back the redirect.
///
/// Source: `src/commands/oauth-flow.ts` - `createVpsAwareOAuthHandlers` return
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum OAuthHandlerMode {
    /// Open browser locally and capture callback automatically.
    Local,
    /// Show URL to user, who must paste redirect URL back manually.
    Remote,
}

/// Determine the handler mode based on environment detection.
///
/// Source: `src/commands/oauth-flow.ts` - `createVpsAwareOAuthHandlers`
#[must_use]
pub fn resolve_handler_mode(config: &VpsAwareOAuthConfig) -> OAuthHandlerMode {
    if config.is_remote {
        OAuthHandlerMode::Remote
    } else {
        OAuthHandlerMode::Local
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn validate_required_input_empty() {
        assert_eq!(validate_required_input(""), Some("Required"));
        assert_eq!(validate_required_input("   "), Some("Required"));
    }

    #[test]
    fn validate_required_input_nonempty() {
        assert_eq!(validate_required_input("http://localhost:1455/callback?code=abc"), None);
    }

    #[test]
    fn resolve_manual_prompt_default() {
        let config = VpsAwareOAuthConfig::default();
        let msg = resolve_manual_prompt_message(&config);
        assert!(msg.contains("redirect URL"));
    }

    #[test]
    fn resolve_manual_prompt_custom() {
        let config = VpsAwareOAuthConfig {
            manual_prompt_message: Some("Enter code".to_owned()),
            ..VpsAwareOAuthConfig::default()
        };
        assert_eq!(resolve_manual_prompt_message(&config), "Enter code");
    }

    #[test]
    fn handler_mode_local() {
        let config = VpsAwareOAuthConfig {
            is_remote: false,
            ..VpsAwareOAuthConfig::default()
        };
        assert_eq!(resolve_handler_mode(&config), OAuthHandlerMode::Local);
    }

    #[test]
    fn handler_mode_remote() {
        let config = VpsAwareOAuthConfig {
            is_remote: true,
            ..VpsAwareOAuthConfig::default()
        };
        assert_eq!(resolve_handler_mode(&config), OAuthHandlerMode::Remote);
    }

    #[test]
    fn build_remote_instructions_captures_url() {
        let event = OAuthAuthEvent {
            url: "https://oauth.example.com/auth".to_owned(),
        };
        let config = VpsAwareOAuthConfig {
            is_remote: true,
            ..VpsAwareOAuthConfig::default()
        };
        let instructions = build_remote_instructions(&event, &config);
        assert_eq!(instructions.url, "https://oauth.example.com/auth");
        assert!(instructions.prompt_message.contains("redirect"));
    }
}
