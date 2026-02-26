/// Clear (delete) all log files.
///
/// Removes all `.log` files from the state directory. Requires `--yes`
/// confirmation to actually delete.
///
/// Source: `src/commands/logs-clear.ts`

use anyhow::{Context, Result};

use oa_config::paths::resolve_state_dir;

/// Delete all log files from the state log directory.
///
/// If `yes` is false, prints a confirmation prompt and exits without
/// deleting anything.
///
/// Source: `src/commands/logs-clear.ts` - `logsClearCommand`
pub async fn logs_clear_command(yes: bool) -> Result<()> {
    let state_dir = resolve_state_dir();
    let log_dir = state_dir.join("logs");

    let entries: Vec<_> = std::fs::read_dir(&log_dir)
        .context("Failed to read log directory")?
        .filter_map(|e| e.ok())
        .filter(|e| e.path().extension().map_or(false, |ext| ext == "log"))
        .collect();

    if entries.is_empty() {
        println!("No log files to clear.");
        return Ok(());
    }

    println!("Found {} log file(s) to clear.", entries.len());

    if !yes {
        println!("Use --yes to confirm deletion.");
        return Ok(());
    }

    let mut removed = 0;
    for entry in &entries {
        if std::fs::remove_file(entry.path()).is_ok() {
            removed += 1;
        }
    }
    println!("Cleared {removed} log file(s).");

    Ok(())
}
