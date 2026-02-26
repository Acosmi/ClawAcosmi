/// Workspace status reporting (skills, plugins, legacy workspace directories).
///
/// Reports eligible skills, missing requirements, blocked-by-allowlist counts,
/// loaded/disabled/errored plugin counts, and plugin diagnostics.
///
/// Source: `src/commands/doctor-workspace-status.ts`

use oa_config::paths::resolve_state_dir;
use oa_terminal::note::note;
use oa_types::config::OpenAcosmiConfig;

use crate::workspace::{detect_legacy_workspace_dirs, format_legacy_workspace_warning};

/// Report workspace status: legacy directories, skills, and plugins.
///
/// Source: `src/commands/doctor-workspace-status.ts` — `noteWorkspaceStatus`
pub fn note_workspace_status(cfg: &OpenAcosmiConfig) {
    let state_dir = resolve_state_dir();
    let workspace_dir = state_dir.join("workspace");

    // ── Legacy workspace directories ──
    let legacy = detect_legacy_workspace_dirs(&workspace_dir);
    if !legacy.legacy_dirs.is_empty() {
        note(
            &format_legacy_workspace_warning(&legacy),
            Some("Extra workspace"),
        );
    }

    // ── Skills status ──
    // Stub: the real implementation calls `buildWorkspaceSkillStatus` from `oa-agents`.
    note(
        &[
            "Eligible: (not yet computed in Rust port)",
            "Missing requirements: (not yet computed)",
            "Blocked by allowlist: (not yet computed)",
        ]
        .join("\n"),
        Some("Skills status"),
    );

    // ── Plugin status ──
    // Stub: the real implementation calls `loadOpenAcosmiPlugins` from the plugin loader.
    let _ = cfg;
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn workspace_status_noop_default_config() {
        let cfg = OpenAcosmiConfig::default();
        // Should not panic.
        note_workspace_status(&cfg);
    }
}
