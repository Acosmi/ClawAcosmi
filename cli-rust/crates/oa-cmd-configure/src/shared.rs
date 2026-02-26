/// Shared types, constants, and prompt wrappers for the configure wizard.
///
/// Defines wizard sections, section selection options, and channels wizard modes.
///
/// Source: `src/commands/configure.shared.ts`

use serde::{Deserialize, Serialize};

/// All available wizard sections for the configure command.
///
/// Source: `src/commands/configure.shared.ts` - `CONFIGURE_WIZARD_SECTIONS`
pub const CONFIGURE_WIZARD_SECTIONS: &[WizardSection] = &[
    WizardSection::Workspace,
    WizardSection::Model,
    WizardSection::Web,
    WizardSection::Gateway,
    WizardSection::Daemon,
    WizardSection::Channels,
    WizardSection::Skills,
    WizardSection::Health,
];

/// A wizard section that can be configured independently.
///
/// Source: `src/commands/configure.shared.ts` - `WizardSection`
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum WizardSection {
    /// Set workspace + sessions directory.
    Workspace,
    /// Pick provider + credentials.
    Model,
    /// Configure Brave search + fetch.
    Web,
    /// Port, bind, auth, tailscale.
    Gateway,
    /// Install/manage the background service.
    Daemon,
    /// Link WhatsApp/Telegram/etc and defaults.
    Channels,
    /// Install/enable workspace skills.
    Skills,
    /// Run gateway + channel checks.
    Health,
}

impl std::fmt::Display for WizardSection {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Workspace => write!(f, "workspace"),
            Self::Model => write!(f, "model"),
            Self::Web => write!(f, "web"),
            Self::Gateway => write!(f, "gateway"),
            Self::Daemon => write!(f, "daemon"),
            Self::Channels => write!(f, "channels"),
            Self::Skills => write!(f, "skills"),
            Self::Health => write!(f, "health"),
        }
    }
}

/// Mode for the channels sub-wizard.
///
/// Source: `src/commands/configure.shared.ts` - `ChannelsWizardMode`
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ChannelsWizardMode {
    /// Add/update channels; disable unselected accounts.
    Configure,
    /// Delete channel tokens/settings from config.
    Remove,
}

/// A single option in the section-selection menu.
///
/// Source: `src/commands/configure.shared.ts` - `CONFIGURE_SECTION_OPTIONS`
pub struct SectionOption {
    /// The section value.
    pub value: WizardSection,
    /// Human-readable label.
    pub label: &'static str,
    /// Hint text shown next to the option.
    pub hint: &'static str,
}

/// All section options with labels and hints for the interactive menu.
///
/// Source: `src/commands/configure.shared.ts` - `CONFIGURE_SECTION_OPTIONS`
pub const CONFIGURE_SECTION_OPTIONS: &[SectionOption] = &[
    SectionOption {
        value: WizardSection::Workspace,
        label: "Workspace",
        hint: "Set workspace + sessions",
    },
    SectionOption {
        value: WizardSection::Model,
        label: "Model",
        hint: "Pick provider + credentials",
    },
    SectionOption {
        value: WizardSection::Web,
        label: "Web tools",
        hint: "Configure Brave search + fetch",
    },
    SectionOption {
        value: WizardSection::Gateway,
        label: "Gateway",
        hint: "Port, bind, auth, tailscale",
    },
    SectionOption {
        value: WizardSection::Daemon,
        label: "Daemon",
        hint: "Install/manage the background service",
    },
    SectionOption {
        value: WizardSection::Channels,
        label: "Channels",
        hint: "Link WhatsApp/Telegram/etc and defaults",
    },
    SectionOption {
        value: WizardSection::Skills,
        label: "Skills",
        hint: "Install/enable workspace skills",
    },
    SectionOption {
        value: WizardSection::Health,
        label: "Health check",
        hint: "Run gateway + channel checks",
    },
];

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn wizard_sections_count_matches_options() {
        assert_eq!(CONFIGURE_WIZARD_SECTIONS.len(), CONFIGURE_SECTION_OPTIONS.len());
    }

    #[test]
    fn section_options_ordered_same_as_sections() {
        for (i, section) in CONFIGURE_WIZARD_SECTIONS.iter().enumerate() {
            assert_eq!(*section, CONFIGURE_SECTION_OPTIONS[i].value);
        }
    }

    #[test]
    fn wizard_section_display() {
        assert_eq!(WizardSection::Workspace.to_string(), "workspace");
        assert_eq!(WizardSection::Health.to_string(), "health");
        assert_eq!(WizardSection::Gateway.to_string(), "gateway");
    }

    #[test]
    fn channels_wizard_mode_variants() {
        let configure = ChannelsWizardMode::Configure;
        let remove = ChannelsWizardMode::Remove;
        assert_ne!(configure, remove);
    }
}
