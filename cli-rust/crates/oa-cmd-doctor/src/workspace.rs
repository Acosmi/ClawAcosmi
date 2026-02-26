/// Memory-system suggestion and legacy workspace detection.
///
/// Checks whether the workspace directory contains a `MEMORY.md` file
/// or references one in `AGENTS.md`.  If not, suggests installing the
/// memory system.  Also detects legacy workspace directories from
/// prior product names.
///
/// Source: `src/commands/doctor-workspace.ts`

use std::path::{Path, PathBuf};

/// Prompt text suggesting the memory system installation.
///
/// Source: `src/commands/doctor-workspace.ts` — `MEMORY_SYSTEM_PROMPT`
pub const MEMORY_SYSTEM_PROMPT: &str = "\
Memory system not found in workspace.
Paste this into your agent:

Install the memory system by applying:
https://github.com/openacosmi/openacosmi/commit/9ffea23f31ca1df5183b25668f8f814bee0fb34e
https://github.com/openacosmi/openacosmi/commit/7d1fee70e76f2f634f1b41fca927ee663914183a";

/// Default agents filename.
///
/// Source: `src/agents/workspace.ts` — `DEFAULT_AGENTS_FILENAME`
const DEFAULT_AGENTS_FILENAME: &str = "AGENTS.md";

/// Check whether the workspace should show the memory system suggestion.
///
/// Returns `true` if:
/// - No `MEMORY.md` / `memory.md` exists in the workspace.
/// - The `AGENTS.md` file does not reference `memory.md`.
///
/// Source: `src/commands/doctor-workspace.ts` — `shouldSuggestMemorySystem`
pub async fn should_suggest_memory_system(workspace_dir: &Path) -> bool {
    let memory_candidates = [
        workspace_dir.join("MEMORY.md"),
        workspace_dir.join("memory.md"),
    ];

    for path in &memory_candidates {
        if tokio::fs::metadata(path).await.is_ok() {
            return false;
        }
    }

    let agents_path = workspace_dir.join(DEFAULT_AGENTS_FILENAME);
    match tokio::fs::read_to_string(&agents_path).await {
        Ok(content) => {
            if regex::Regex::new(r"(?i)memory\.md")
                .ok()
                .is_some_and(|re| re.is_match(&content))
            {
                return false;
            }
        }
        Err(_) => {
            // No AGENTS.md or unreadable; treat as missing memory guidance.
        }
    }

    true
}

/// Result of detecting legacy workspace directories.
///
/// Source: `src/commands/doctor-workspace.ts` — `LegacyWorkspaceDetection`
#[derive(Debug, Clone)]
pub struct LegacyWorkspaceDetection {
    /// The active workspace directory (resolved absolute path).
    pub active_workspace: PathBuf,
    /// Legacy directories that may contain old agent files.
    pub legacy_dirs: Vec<PathBuf>,
}

/// Detect legacy workspace directories from prior product names.
///
/// Source: `src/commands/doctor-workspace.ts` — `detectLegacyWorkspaceDirs`
pub fn detect_legacy_workspace_dirs(workspace_dir: &Path) -> LegacyWorkspaceDetection {
    let active_workspace = workspace_dir
        .canonicalize()
        .unwrap_or_else(|_| workspace_dir.to_path_buf());

    // The TS implementation currently returns an empty list.
    // Future versions may scan for `.clawdbot-workspace` etc.
    LegacyWorkspaceDetection {
        active_workspace,
        legacy_dirs: Vec::new(),
    }
}

/// Shorten a path by replacing the home directory prefix with `~`.
fn shorten_home_path(path: &Path) -> String {
    let home = dirs::home_dir().unwrap_or_default();
    let path_str = path.to_string_lossy();
    let home_str = home.to_string_lossy();
    if path_str.starts_with(home_str.as_ref()) {
        format!("~{}", &path_str[home_str.len()..])
    } else {
        path_str.to_string()
    }
}

/// Format a warning message about detected legacy workspace directories.
///
/// Source: `src/commands/doctor-workspace.ts` — `formatLegacyWorkspaceWarning`
pub fn format_legacy_workspace_warning(detection: &LegacyWorkspaceDetection) -> String {
    let mut lines = vec![
        "Extra workspace directories detected (may contain old agent files):".to_string(),
    ];
    for dir in &detection.legacy_dirs {
        lines.push(format!("- {}", shorten_home_path(dir)));
    }
    lines.push(format!(
        "Active workspace: {}",
        shorten_home_path(&detection.active_workspace)
    ));
    lines.push("If unused, archive or move to Trash.".to_string());
    lines.join("\n")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn suggest_memory_for_empty_workspace() {
        let tmp = std::env::temp_dir().join("oa-doctor-ws-test-empty");
        let _ = std::fs::create_dir_all(&tmp);
        assert!(should_suggest_memory_system(&tmp).await);
        let _ = std::fs::remove_dir_all(&tmp);
    }

    #[tokio::test]
    async fn no_suggest_when_memory_md_exists() {
        let tmp = std::env::temp_dir().join("oa-doctor-ws-test-memory");
        let _ = std::fs::create_dir_all(&tmp);
        let _ = std::fs::write(tmp.join("MEMORY.md"), "# Memory\n");
        assert!(!should_suggest_memory_system(&tmp).await);
        let _ = std::fs::remove_dir_all(&tmp);
    }

    #[tokio::test]
    async fn no_suggest_when_agents_references_memory() {
        let tmp = std::env::temp_dir().join("oa-doctor-ws-test-agents");
        let _ = std::fs::create_dir_all(&tmp);
        let _ = std::fs::write(
            tmp.join("AGENTS.md"),
            "# Agent\nSee memory.md for context.\n",
        );
        assert!(!should_suggest_memory_system(&tmp).await);
        let _ = std::fs::remove_dir_all(&tmp);
    }

    #[test]
    fn detect_legacy_returns_empty() {
        let tmp = std::env::temp_dir().join("oa-doctor-ws-detect-test");
        let _ = std::fs::create_dir_all(&tmp);
        let result = detect_legacy_workspace_dirs(&tmp);
        assert!(result.legacy_dirs.is_empty());
        let _ = std::fs::remove_dir_all(&tmp);
    }

    #[test]
    fn format_warning_includes_active_dir() {
        let detection = LegacyWorkspaceDetection {
            active_workspace: PathBuf::from("/home/user/.openacosmi/workspace"),
            legacy_dirs: vec![PathBuf::from("/home/user/.clawdbot/workspace")],
        };
        let warning = format_legacy_workspace_warning(&detection);
        assert!(warning.contains("Extra workspace"));
        assert!(warning.contains("Active workspace"));
    }
}
