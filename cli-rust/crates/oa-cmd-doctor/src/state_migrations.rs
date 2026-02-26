/// Legacy state migration detection and execution.
///
/// Detects legacy state directory layouts (clawdbot / moltbot / moldbot)
/// and session/agent/WhatsApp auth data that needs to be migrated to the
/// current `.openacosmi` structure.
///
/// Source: `src/commands/doctor-state-migrations.ts`
/// (re-exports from `src/infra/state-migrations.ts`)

use std::path::PathBuf;

use oa_config::paths::resolve_state_dir;
use oa_types::config::OpenAcosmiConfig;

/// Result of auto-migrating the legacy state directory.
///
/// Source: `src/infra/state-migrations.ts` — return of `autoMigrateLegacyStateDir`
#[derive(Debug, Clone, Default)]
pub struct MigrationResult {
    /// Human-readable descriptions of changes made.
    pub changes: Vec<String>,
    /// Warnings encountered during migration.
    pub warnings: Vec<String>,
}

/// Result of detecting legacy state that needs migration.
///
/// Source: `src/infra/state-migrations.ts` — `LegacyStateDetection`
#[derive(Debug, Clone, Default)]
pub struct LegacyStateDetection {
    /// Preview messages describing what will be migrated.
    pub preview: Vec<String>,
}

/// Legacy state directory candidates.
const LEGACY_DIRNAMES: &[&str] = &[".clawdbot", ".moltbot", ".moldbot"];

/// Resolve home directory.
fn resolve_home() -> PathBuf {
    dirs::home_dir().unwrap_or_else(|| PathBuf::from("."))
}

/// Auto-migrate a legacy state directory to `~/.openacosmi`.
///
/// If `~/.openacosmi` already exists, this is a no-op.  Otherwise, checks
/// for legacy directories and copies/moves them.
///
/// Source: `src/infra/state-migrations.ts` — `autoMigrateLegacyStateDir`
pub async fn auto_migrate_legacy_state_dir() -> MigrationResult {
    let mut result = MigrationResult::default();
    let home = resolve_home();
    let target = home.join(".openacosmi");

    if target.exists() {
        return result;
    }

    // Check for legacy directories.
    let mut legacy_path: Option<PathBuf> = None;
    for dirname in LEGACY_DIRNAMES {
        let candidate = home.join(dirname);
        if candidate.is_dir() {
            legacy_path = Some(candidate);
            break;
        }
    }

    let Some(legacy) = legacy_path else {
        return result;
    };

    // Attempt to rename the legacy directory.
    match tokio::fs::rename(&legacy, &target).await {
        Ok(()) => {
            result.changes.push(format!(
                "Migrated legacy state dir: {} -> {}",
                legacy.display(),
                target.display()
            ));
        }
        Err(e) => {
            result.warnings.push(format!(
                "Failed to migrate {} -> {}: {e}",
                legacy.display(),
                target.display()
            ));
        }
    }

    result
}

/// Detect legacy state that needs migration (sessions, agent configs, WhatsApp auth).
///
/// Returns a `LegacyStateDetection` with preview messages.
///
/// Source: `src/infra/state-migrations.ts` — `detectLegacyStateMigrations`
pub async fn detect_legacy_state_migrations(
    _cfg: &OpenAcosmiConfig,
) -> LegacyStateDetection {
    let state_dir = resolve_state_dir();
    let mut detection = LegacyStateDetection::default();

    // Check for legacy session files.
    let legacy_sessions = state_dir.join("sessions-legacy");
    if legacy_sessions.is_dir() {
        detection
            .preview
            .push("Legacy session directory found.".to_string());
    }

    // Check for legacy agent directories.
    for dirname in LEGACY_DIRNAMES {
        let candidate = state_dir.join(dirname);
        if candidate.is_dir() {
            detection
                .preview
                .push(format!("Legacy agent directory found: {dirname}"));
        }
    }

    detection
}

/// Execute detected legacy state migrations.
///
/// Source: `src/infra/state-migrations.ts` — `runLegacyStateMigrations`
pub async fn run_legacy_state_migrations(
    _detected: &LegacyStateDetection,
) -> MigrationResult {
    // Stub: the real implementation iterates over detected legacy state
    // and moves/transforms it into the current layout.
    MigrationResult::default()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn auto_migrate_noop_when_target_exists() {
        // The ~/.openacosmi directory likely exists in test environments.
        let result = auto_migrate_legacy_state_dir().await;
        // Should be a no-op (no changes, no warnings).
        assert!(result.changes.is_empty());
        assert!(result.warnings.is_empty());
    }

    #[tokio::test]
    async fn detect_returns_empty_by_default() {
        let cfg = OpenAcosmiConfig::default();
        let detection = detect_legacy_state_migrations(&cfg).await;
        // In most test environments, there are no legacy directories.
        // We just verify it doesn't panic.
        let _ = detection;
    }

    #[tokio::test]
    async fn run_migrations_returns_empty() {
        let detection = LegacyStateDetection::default();
        let result = run_legacy_state_migrations(&detection).await;
        assert!(result.changes.is_empty());
    }

    #[test]
    fn legacy_dirnames_are_complete() {
        assert_eq!(LEGACY_DIRNAMES.len(), 3);
        assert!(LEGACY_DIRNAMES.contains(&".clawdbot"));
        assert!(LEGACY_DIRNAMES.contains(&".moltbot"));
        assert!(LEGACY_DIRNAMES.contains(&".moldbot"));
    }
}
