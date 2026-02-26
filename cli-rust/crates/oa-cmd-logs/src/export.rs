/// Export all log files into a single combined output file.
///
/// Concatenates all `.log` files from the state directory (sorted by
/// modification time) into the specified output path, with per-file headers.
///
/// Source: `src/commands/logs-export.ts`

use anyhow::{Context, Result};

use oa_config::paths::resolve_state_dir;

/// Export all log files to a combined output file.
///
/// Source: `src/commands/logs-export.ts` - `logsExportCommand`
pub async fn logs_export_command(output: &str) -> Result<()> {
    let state_dir = resolve_state_dir();
    let log_dir = state_dir.join("logs");

    let mut combined = String::new();
    let mut entries: Vec<_> = std::fs::read_dir(&log_dir)
        .context("Failed to read log directory")?
        .filter_map(|e| e.ok())
        .filter(|e| e.path().extension().map_or(false, |ext| ext == "log"))
        .collect();
    entries.sort_by_key(|e| e.metadata().ok().and_then(|m| m.modified().ok()));

    for entry in &entries {
        let content = std::fs::read_to_string(entry.path()).unwrap_or_default();
        combined.push_str(&format!("=== {} ===\n", entry.path().display()));
        combined.push_str(&content);
        combined.push('\n');
    }

    std::fs::write(output, &combined)
        .with_context(|| format!("Failed to write to {output}"))?;
    println!("Exported {} log file(s) to {output}", entries.len());

    Ok(())
}
