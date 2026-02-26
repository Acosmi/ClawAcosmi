/// List available log files.
///
/// Enumerates `.log` files in the state directory, sorted by modification
/// time (most recent first). Supports JSON output for scripting.
///
/// Source: `src/commands/logs-list.ts`

use anyhow::{Context, Result};

use oa_config::paths::resolve_state_dir;

/// List all log files in the state log directory.
///
/// Source: `src/commands/logs-list.ts` - `logsListCommand`
pub async fn logs_list_command(json: bool) -> Result<()> {
    let state_dir = resolve_state_dir();
    let log_dir = state_dir.join("logs");

    let mut entries: Vec<_> = std::fs::read_dir(&log_dir)
        .context("Failed to read log directory")?
        .filter_map(|e| e.ok())
        .filter(|e| e.path().extension().map_or(false, |ext| ext == "log"))
        .collect();
    entries.sort_by_key(|e| {
        std::cmp::Reverse(e.metadata().ok().and_then(|m| m.modified().ok()))
    });

    if json {
        let files: Vec<String> = entries
            .iter()
            .map(|e| e.path().display().to_string())
            .collect();
        println!("{}", serde_json::to_string_pretty(&files)?);
    } else if entries.is_empty() {
        println!("No log files found in {}", log_dir.display());
    } else {
        println!("Log files:");
        for entry in &entries {
            let meta = entry.metadata().ok();
            let size = meta.as_ref().map_or(0, |m| m.len());
            println!("  {} ({} bytes)", entry.path().display(), size);
        }
    }

    Ok(())
}
