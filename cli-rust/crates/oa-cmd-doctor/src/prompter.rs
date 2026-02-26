/// Doctor prompter — interactive / yes / non-interactive confirmation modes.
///
/// Encapsulates the decision logic for whether to auto-apply repairs, prompt
/// the user, or silently skip interactive steps.
///
/// Source: `src/commands/doctor-prompter.ts`

use serde::{Deserialize, Serialize};

/// Options that control doctor behaviour.
///
/// Source: `src/commands/doctor-prompter.ts` — `DoctorOptions`
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DoctorOptions {
    /// When `Some(false)`, workspace suggestions are suppressed.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub workspace_suggestions: Option<bool>,

    /// Accept all prompts automatically.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub yes: Option<bool>,

    /// Run without any interactive prompts.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub non_interactive: Option<bool>,

    /// Enable deep scanning for extra gateway services.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub deep: Option<bool>,

    /// Apply recommended repairs automatically.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub repair: Option<bool>,

    /// Allow aggressive / destructive repairs.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub force: Option<bool>,

    /// Pre-generate a gateway auth token (skip prompt).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub generate_gateway_token: Option<bool>,
}

/// The doctor prompter state machine.
///
/// Tracks whether repairs have been triggered and exposes confirmation
/// methods that respect the current `DoctorOptions`.
///
/// Source: `src/commands/doctor-prompter.ts` — `DoctorPrompter`
pub struct DoctorPrompter {
    /// Whether the user requested `--fix` / `--yes` (auto-repair).
    pub should_repair: bool,

    /// Whether the user passed `--force` (aggressive overwrite).
    pub should_force: bool,

    /// True when running in non-interactive mode (no TTY or `--non-interactive`).
    non_interactive: bool,

    /// True when we can actually display interactive prompts.
    can_prompt: bool,
}

/// Create a new [`DoctorPrompter`] from the given options.
///
/// Source: `src/commands/doctor-prompter.ts` — `createDoctorPrompter`
pub fn create_doctor_prompter(options: &DoctorOptions) -> DoctorPrompter {
    let yes = options.yes == Some(true);
    let requested_non_interactive = options.non_interactive == Some(true);
    let should_repair = options.repair == Some(true) || yes;
    let should_force = options.force == Some(true);
    let is_tty = atty_stdout();
    let non_interactive = requested_non_interactive || (!is_tty && !yes);
    let can_prompt = is_tty && !yes && !non_interactive;

    DoctorPrompter {
        should_repair,
        should_force,
        non_interactive,
        can_prompt,
    }
}

/// Check whether stdout is a TTY (best-effort heuristic).
fn atty_stdout() -> bool {
    // In a Rust CLI we check stdin for interactive TTY.
    std::io::IsTerminal::is_terminal(&std::io::stdin())
}

impl DoctorPrompter {
    /// Standard confirm — returns `true` if the user accepts or we are in auto-repair mode.
    ///
    /// Source: `src/commands/doctor-prompter.ts` — `confirmDefault`
    pub async fn confirm(&self, _message: &str, initial_value: bool) -> bool {
        if self.non_interactive {
            return false;
        }
        if self.should_repair {
            return true;
        }
        if !self.can_prompt {
            return initial_value;
        }
        // In a real CLI we would call `dialoguer::Confirm` here.
        // For now, return the initial value in non-TTY contexts.
        initial_value
    }

    /// Confirm for a repair step — skips in non-interactive mode.
    ///
    /// Source: `src/commands/doctor-prompter.ts` — `confirmRepair`
    pub async fn confirm_repair(&self, _message: &str, initial_value: bool) -> bool {
        if self.non_interactive {
            return false;
        }
        self.confirm(_message, initial_value).await
    }

    /// Confirm for an aggressive (destructive) repair.
    ///
    /// Requires both `--fix` and `--force` to auto-accept.
    ///
    /// Source: `src/commands/doctor-prompter.ts` — `confirmAggressive`
    pub async fn confirm_aggressive(&self, _message: &str, initial_value: bool) -> bool {
        if self.non_interactive {
            return false;
        }
        if self.should_repair && self.should_force {
            return true;
        }
        if self.should_repair && !self.should_force {
            return false;
        }
        if !self.can_prompt {
            return initial_value;
        }
        initial_value
    }

    /// Confirm but skip entirely in non-interactive mode.
    ///
    /// Source: `src/commands/doctor-prompter.ts` — `confirmSkipInNonInteractive`
    pub async fn confirm_skip_in_non_interactive(
        &self,
        _message: &str,
        initial_value: bool,
    ) -> bool {
        if self.non_interactive {
            return false;
        }
        if self.should_repair {
            return true;
        }
        self.confirm(_message, initial_value).await
    }

    /// Select with a fallback value when prompting is not possible.
    ///
    /// Source: `src/commands/doctor-prompter.ts` — `select`
    pub async fn select<T: Clone>(&self, _message: &str, fallback: T) -> T {
        // In auto-repair mode or when we cannot prompt, return the fallback.
        fallback
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn default_options_produce_non_repair_prompter() {
        let options = DoctorOptions::default();
        let p = create_doctor_prompter(&options);
        assert!(!p.should_repair);
        assert!(!p.should_force);
    }

    #[test]
    fn yes_option_enables_repair() {
        let options = DoctorOptions {
            yes: Some(true),
            ..Default::default()
        };
        let p = create_doctor_prompter(&options);
        assert!(p.should_repair);
    }

    #[test]
    fn repair_option_enables_repair() {
        let options = DoctorOptions {
            repair: Some(true),
            ..Default::default()
        };
        let p = create_doctor_prompter(&options);
        assert!(p.should_repair);
    }

    #[test]
    fn force_option_enables_force() {
        let options = DoctorOptions {
            force: Some(true),
            ..Default::default()
        };
        let p = create_doctor_prompter(&options);
        assert!(p.should_force);
    }

    #[tokio::test]
    async fn confirm_non_interactive_returns_false() {
        let options = DoctorOptions {
            non_interactive: Some(true),
            ..Default::default()
        };
        let p = create_doctor_prompter(&options);
        assert!(!p.confirm("Test?", true).await);
    }

    #[tokio::test]
    async fn confirm_repair_mode_returns_true() {
        let options = DoctorOptions {
            yes: Some(true),
            ..Default::default()
        };
        let p = create_doctor_prompter(&options);
        // yes → should_repair, and non_interactive is false when yes=true
        assert!(p.confirm("Test?", false).await);
    }

    #[tokio::test]
    async fn confirm_aggressive_needs_force() {
        // repair=true without force → aggressive returns false
        let repair_only = DoctorOptions {
            yes: Some(true),
            repair: Some(true),
            ..Default::default()
        };
        let p = create_doctor_prompter(&repair_only);
        assert!(!p.confirm_aggressive("Overwrite?", true).await);

        // repair=true + force=true → aggressive returns true
        let repair_force = DoctorOptions {
            yes: Some(true),
            repair: Some(true),
            force: Some(true),
            ..Default::default()
        };
        let p2 = create_doctor_prompter(&repair_force);
        assert!(p2.confirm_aggressive("Overwrite?", false).await);
    }
}
