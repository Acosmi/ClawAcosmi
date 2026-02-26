/// Source-install integrity checks.
///
/// When running from a development checkout (pnpm workspace), verifies
/// that `node_modules` was installed by pnpm, that no stray `package-lock.json`
/// exists, and that the `tsx` binary is present.
///
/// Source: `src/commands/doctor-install.ts`

use std::path::Path;

use oa_terminal::note::note;

/// Check for common source-install issues at the given package root.
///
/// `root` is the resolved OpenAcosmi package root (may be `None` if
/// it could not be determined).
///
/// Source: `src/commands/doctor-install.ts` — `noteSourceInstallIssues`
pub fn note_source_install_issues(root: Option<&str>) {
    let Some(root) = root else {
        return;
    };
    let root_path = Path::new(root);

    let workspace_marker = root_path.join("pnpm-workspace.yaml");
    if !workspace_marker.exists() {
        return;
    }

    let mut warnings: Vec<String> = Vec::new();

    let node_modules = root_path.join("node_modules");
    let pnpm_store = node_modules.join(".pnpm");
    let tsx_bin = node_modules.join(".bin").join("tsx");
    let src_entry = root_path.join("src").join("entry.ts");

    if node_modules.exists() && !pnpm_store.exists() {
        warnings.push(
            "- node_modules was not installed by pnpm (missing node_modules/.pnpm). Run: pnpm install"
                .to_string(),
        );
    }

    if root_path.join("package-lock.json").exists() {
        warnings.push(
            "- package-lock.json present in a pnpm workspace. If you ran npm install, remove it and reinstall with pnpm."
                .to_string(),
        );
    }

    if src_entry.exists() && !tsx_bin.exists() {
        warnings.push("- tsx binary is missing for source runs. Run: pnpm install".to_string());
    }

    if !warnings.is_empty() {
        note(&warnings.join("\n"), Some("Install"));
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn noop_when_root_is_none() {
        // Should not panic.
        note_source_install_issues(None);
    }

    #[test]
    fn noop_when_no_workspace_marker() {
        let tmp = std::env::temp_dir().join("oa-doctor-install-test-no-ws");
        let _ = std::fs::create_dir_all(&tmp);
        note_source_install_issues(tmp.to_str());
        let _ = std::fs::remove_dir_all(&tmp);
    }

    #[test]
    fn detects_missing_pnpm_store() {
        let tmp = std::env::temp_dir().join("oa-doctor-install-test-pnpm");
        let _ = std::fs::remove_dir_all(&tmp);
        let _ = std::fs::create_dir_all(tmp.join("node_modules"));
        // Create workspace marker.
        let _ = std::fs::write(tmp.join("pnpm-workspace.yaml"), "packages:\n  - crates/*\n");
        // Should detect the missing .pnpm dir (but we can't easily capture note output).
        note_source_install_issues(tmp.to_str());
        let _ = std::fs::remove_dir_all(&tmp);
    }
}
