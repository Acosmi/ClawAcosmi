/// Show the contents of a specific log file.
///
/// Reads a named log file (or the most recent one) and prints its contents,
/// optionally limited to the last N lines. Supports JSON output.
///
/// Source: `src/commands/logs-show.ts`

use anyhow::{Context, Result};

use oa_config::paths::resolve_state_dir;

/// Show a log file's contents.
///
/// If `file` is `None`, the most recently modified log file is used.
/// If `lines` is provided, only the last N lines are shown.
///
/// Source: `src/commands/logs-show.ts` - `logsShowCommand`
pub async fn logs_show_command(
    file: Option<&str>,
    lines: Option<usize>,
    json: bool,
) -> Result<()> {
    let state_dir = resolve_state_dir();
    let log_dir = state_dir.join("logs");

    let path = if let Some(f) = file {
        log_dir.join(f)
    } else {
        // Find most recent log file.
        let mut entries: Vec<_> = std::fs::read_dir(&log_dir)
            .context("Failed to read log directory")?
            .filter_map(|e| e.ok())
            .filter(|e| e.path().extension().map_or(false, |ext| ext == "log"))
            .collect();
        entries.sort_by_key(|e| {
            std::cmp::Reverse(e.metadata().ok().and_then(|m| m.modified().ok()))
        });
        entries.first().context("No log files found")?.path()
    };

    let content = std::fs::read_to_string(&path)
        .with_context(|| format!("Failed to read {}", path.display()))?;

    let output: Vec<&str> = if let Some(n) = lines {
        content
            .lines()
            .rev()
            .take(n)
            .collect::<Vec<_>>()
            .into_iter()
            .rev()
            .collect()
    } else {
        content.lines().collect()
    };

    if json {
        println!("{}", serde_json::to_string_pretty(&output)?);
    } else {
        for line in &output {
            println!("{line}");
        }
    }

    Ok(())
}
