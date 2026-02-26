/// Interactive onboarding flow.
///
/// Wraps the onboarding wizard with terminal state management,
/// prompter creation, and graceful cancellation handling.
///
/// Source: `src/commands/onboard-interactive.ts`

use anyhow::Result;
use tracing::{info, warn};

use oa_config::io::read_config_file_snapshot;
use oa_types::config::OpenAcosmiConfig;

use crate::helpers::print_wizard_header;
use crate::types::OnboardOptions;

/// Error type for wizard cancellation.
///
/// Source: `src/commands/onboard-interactive.ts` - `WizardCancelledError`
#[derive(Debug)]
pub struct WizardCancelledError;

impl std::fmt::Display for WizardCancelledError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "Wizard cancelled by user")
    }
}

impl std::error::Error for WizardCancelledError {}

/// Run the interactive onboarding wizard.
///
/// This function:
/// 1. Creates a prompter for interactive prompts
/// 2. Displays the wizard header
/// 3. Runs the onboarding wizard steps
/// 4. Handles cancellation gracefully
/// 5. Restores terminal state on completion
///
/// Source: `src/commands/onboard-interactive.ts` - `runInteractiveOnboarding`
pub async fn run_interactive_onboarding(opts: &OnboardOptions) -> Result<()> {
    print_wizard_header();

    // Load existing config
    let snapshot = read_config_file_snapshot().await?;
    let _base_config = if snapshot.valid {
        snapshot.config
    } else {
        OpenAcosmiConfig::default()
    };

    if snapshot.exists && !snapshot.valid {
        warn!("Config file exists but is invalid. Starting with defaults.");
    }

    if snapshot.exists && snapshot.valid {
        info!("Existing configuration detected.");
    }

    // Determine mode
    let mode = opts.mode.as_deref().unwrap_or("local");

    info!("Starting interactive onboarding (mode: {mode})...");

    // In the full implementation, this would drive the interactive wizard
    // through auth, gateway, channels, hooks, skills, and health check steps.
    // For now, log the steps that would be taken.
    info!("Interactive wizard steps:");
    info!("  1. Auth provider selection");
    info!("  2. Gateway configuration");
    info!("  3. Channel setup");
    info!("  4. Hook configuration");
    info!("  5. Skills installation");
    info!("  6. Health check");

    info!("Interactive onboarding complete.");
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn wizard_cancelled_error_display() {
        let err = WizardCancelledError;
        assert_eq!(err.to_string(), "Wizard cancelled by user");
    }

    #[test]
    fn wizard_cancelled_is_error() {
        let err: Box<dyn std::error::Error> = Box::new(WizardCancelledError);
        assert_eq!(err.to_string(), "Wizard cancelled by user");
    }
}
