/// Skills setup wizard for onboarding.
///
/// Provides the interactive skill discovery, dependency installation, and
/// API key configuration flow during onboarding.
///
/// Source: `src/commands/onboard-skills.ts`

use anyhow::Result;
use tracing::info;

use oa_types::config::OpenAcosmiConfig;
use oa_types::skills::{SkillConfig, SkillsConfig, SkillsInstallConfig, SkillsNodeManager};

/// Information about a skill for display during onboarding.
///
/// Source: `src/commands/onboard-skills.ts` - skill status fields
#[derive(Debug, Clone)]
pub struct SkillStatusInfo {
    /// Skill name identifier.
    pub name: String,
    /// Skill config key.
    pub skill_key: String,
    /// Whether this skill is eligible (all deps met).
    pub eligible: bool,
    /// Whether the skill is disabled.
    pub disabled: bool,
    /// Whether blocked by the allowlist.
    pub blocked_by_allowlist: bool,
    /// Optional description.
    pub description: Option<String>,
    /// Primary environment variable name (for API key prompts).
    pub primary_env: Option<String>,
    /// Missing binary dependencies.
    pub missing_bins: Vec<String>,
    /// Missing environment variables.
    pub missing_env: Vec<String>,
    /// Available install options.
    pub install_options: Vec<SkillInstallOption>,
}

/// An installation option for a skill dependency.
///
/// Source: `src/commands/onboard-skills.ts` - install option fields
#[derive(Debug, Clone)]
pub struct SkillInstallOption {
    /// Unique install identifier.
    pub id: String,
    /// Install kind (e.g., "brew", "npm", "apt").
    pub kind: String,
    /// Display label.
    pub label: String,
}

/// Summarize an install failure message for display.
///
/// Strips common prefixes and truncates long messages.
///
/// Source: `src/commands/onboard-skills.ts` - `summarizeInstallFailure`
pub fn summarize_install_failure(message: &str) -> Option<String> {
    let re_prefix = regex::Regex::new(r"(?i)^Install failed(?:\s*\([^)]*\))?\s*:?\s*")
        .unwrap_or_else(|_| regex::Regex::new("").expect("empty regex"));
    let cleaned = re_prefix.replace(message, "").trim().to_string();
    if cleaned.is_empty() {
        return None;
    }
    let max_len = 140;
    if cleaned.len() > max_len {
        Some(format!("{}...", &cleaned[..max_len - 1]))
    } else {
        Some(cleaned)
    }
}

/// Format a skill hint for display in selection prompts.
///
/// Source: `src/commands/onboard-skills.ts` - `formatSkillHint`
pub fn format_skill_hint(description: Option<&str>, install_label: Option<&str>) -> String {
    let desc = description.map(str::trim).filter(|s| !s.is_empty());
    let label = install_label.map(str::trim).filter(|s| !s.is_empty());

    let combined = match (desc, label) {
        (Some(d), Some(l)) => format!("{d} -- {l}"),
        (Some(d), None) => d.to_string(),
        (None, Some(l)) => l.to_string(),
        (None, None) => "install".to_string(),
    };

    let max_len = 90;
    if combined.len() > max_len {
        format!("{}...", &combined[..max_len - 1])
    } else {
        combined
    }
}

/// Upsert a skill entry in the config, merging with any existing values.
///
/// Source: `src/commands/onboard-skills.ts` - `upsertSkillEntry`
pub fn upsert_skill_entry(
    cfg: OpenAcosmiConfig,
    skill_key: &str,
    api_key: Option<&str>,
) -> OpenAcosmiConfig {
    let skills = cfg.skills.clone().unwrap_or_default();
    let mut entries = skills.entries.clone().unwrap_or_default();

    let existing = entries.get(skill_key).cloned().unwrap_or_default();
    let entry = SkillConfig {
        api_key: api_key.map(str::to_string).or(existing.api_key),
        ..existing
    };
    entries.insert(skill_key.to_string(), entry);

    OpenAcosmiConfig {
        skills: Some(SkillsConfig {
            entries: Some(entries),
            ..skills
        }),
        ..cfg
    }
}

/// Apply the node manager choice to the config.
///
/// Source: `src/commands/onboard-skills.ts` - node manager selection
pub fn apply_node_manager(cfg: OpenAcosmiConfig, manager: SkillsNodeManager) -> OpenAcosmiConfig {
    let skills = cfg.skills.unwrap_or_default();
    let install = skills.install.unwrap_or_default();

    OpenAcosmiConfig {
        skills: Some(SkillsConfig {
            install: Some(SkillsInstallConfig {
                node_manager: Some(manager),
                ..install
            }),
            ..skills
        }),
        ..cfg
    }
}

/// Parse a node manager string to the enum value.
///
/// Source: `src/commands/onboard-skills.ts` - node manager selection
pub fn parse_node_manager(value: &str) -> Option<SkillsNodeManager> {
    match value {
        "npm" => Some(SkillsNodeManager::Npm),
        "pnpm" => Some(SkillsNodeManager::Pnpm),
        "bun" => Some(SkillsNodeManager::Bun),
        "yarn" => Some(SkillsNodeManager::Yarn),
        _ => None,
    }
}

/// Run the skills setup wizard (non-interactive stub).
///
/// In the full implementation, this discovers available skills, checks for
/// missing dependencies, prompts for installations and API keys, and
/// updates the config accordingly.
///
/// Source: `src/commands/onboard-skills.ts` - `setupSkills`
pub async fn setup_skills(
    cfg: OpenAcosmiConfig,
    _workspace_dir: &str,
) -> Result<OpenAcosmiConfig> {
    info!("Skills configuration available via interactive mode.");
    Ok(cfg)
}

#[cfg(test)]
mod tests {
    use std::collections::HashMap;

    use super::*;

    #[test]
    fn summarize_failure_strips_prefix() {
        let msg = "Install failed (exit 1): package not found";
        let result = summarize_install_failure(msg);
        assert_eq!(result.as_deref(), Some("package not found"));
    }

    #[test]
    fn summarize_failure_empty_message() {
        assert!(summarize_install_failure("Install failed").is_none());
    }

    #[test]
    fn summarize_failure_truncates_long_message() {
        let long_msg = "a".repeat(200);
        let result = summarize_install_failure(&long_msg);
        assert!(result.is_some());
        let text = result.expect("should have value");
        assert!(text.len() <= 143); // 139 chars + "..."
    }

    #[test]
    fn format_hint_both() {
        let hint = format_skill_hint(Some("A great skill"), Some("brew install foo"));
        assert_eq!(hint, "A great skill -- brew install foo");
    }

    #[test]
    fn format_hint_description_only() {
        let hint = format_skill_hint(Some("A great skill"), None);
        assert_eq!(hint, "A great skill");
    }

    #[test]
    fn format_hint_label_only() {
        let hint = format_skill_hint(None, Some("brew install foo"));
        assert_eq!(hint, "brew install foo");
    }

    #[test]
    fn format_hint_neither() {
        let hint = format_skill_hint(None, None);
        assert_eq!(hint, "install");
    }

    #[test]
    fn format_hint_truncates() {
        let long_desc = "x".repeat(100);
        let hint = format_skill_hint(Some(&long_desc), None);
        assert!(hint.len() <= 93); // 89 + "..."
    }

    #[test]
    fn upsert_skill_entry_creates_new() {
        let cfg = OpenAcosmiConfig::default();
        let result = upsert_skill_entry(cfg, "web-search", Some("sk-test"));

        let entries = result
            .skills
            .as_ref()
            .and_then(|s| s.entries.as_ref())
            .expect("entries");
        let entry = entries.get("web-search").expect("entry");
        assert_eq!(entry.api_key.as_deref(), Some("sk-test"));
    }

    #[test]
    fn upsert_skill_entry_merges_existing() {
        let mut cfg = OpenAcosmiConfig::default();
        let mut entries = HashMap::new();
        entries.insert(
            "web-search".to_string(),
            SkillConfig {
                enabled: Some(true),
                ..Default::default()
            },
        );
        cfg.skills = Some(SkillsConfig {
            entries: Some(entries),
            ..Default::default()
        });

        let result = upsert_skill_entry(cfg, "web-search", Some("sk-new"));

        let entry = result
            .skills
            .as_ref()
            .and_then(|s| s.entries.as_ref())
            .and_then(|e| e.get("web-search"))
            .expect("entry");
        assert_eq!(entry.api_key.as_deref(), Some("sk-new"));
        assert_eq!(entry.enabled, Some(true)); // preserved
    }

    #[test]
    fn apply_node_manager_npm() {
        let cfg = OpenAcosmiConfig::default();
        let result = apply_node_manager(cfg, SkillsNodeManager::Npm);

        let nm = result
            .skills
            .as_ref()
            .and_then(|s| s.install.as_ref())
            .and_then(|i| i.node_manager.as_ref())
            .expect("node_manager");
        assert_eq!(*nm, SkillsNodeManager::Npm);
    }

    #[test]
    fn apply_node_manager_preserves_existing() {
        let mut cfg = OpenAcosmiConfig::default();
        cfg.skills = Some(SkillsConfig {
            install: Some(SkillsInstallConfig {
                prefer_brew: Some(true),
                ..Default::default()
            }),
            ..Default::default()
        });

        let result = apply_node_manager(cfg, SkillsNodeManager::Pnpm);

        let install = result
            .skills
            .as_ref()
            .and_then(|s| s.install.as_ref())
            .expect("install");
        assert_eq!(install.node_manager, Some(SkillsNodeManager::Pnpm));
        assert_eq!(install.prefer_brew, Some(true));
    }

    #[test]
    fn parse_node_manager_values() {
        assert_eq!(parse_node_manager("npm"), Some(SkillsNodeManager::Npm));
        assert_eq!(parse_node_manager("pnpm"), Some(SkillsNodeManager::Pnpm));
        assert_eq!(parse_node_manager("bun"), Some(SkillsNodeManager::Bun));
        assert_eq!(parse_node_manager("yarn"), Some(SkillsNodeManager::Yarn));
        assert!(parse_node_manager("unknown").is_none());
    }

    #[test]
    fn skill_status_info_fields() {
        let info = SkillStatusInfo {
            name: "web-search".to_string(),
            skill_key: "webSearch".to_string(),
            eligible: true,
            disabled: false,
            blocked_by_allowlist: false,
            description: Some("Search the web".to_string()),
            primary_env: Some("SERP_API_KEY".to_string()),
            missing_bins: vec![],
            missing_env: vec!["SERP_API_KEY".to_string()],
            install_options: vec![],
        };
        assert!(info.eligible);
        assert!(!info.missing_env.is_empty());
    }
}
