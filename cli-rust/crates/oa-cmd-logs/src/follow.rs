/// Follow the most recent gateway log file.
///
/// Reads the last N lines of the most recently modified `.log` file
/// in the state directory and optionally filters by channel keyword.
///
/// Source: `src/commands/logs-follow.ts`

use anyhow::{Context, Result};

use oa_config::paths::resolve_state_dir;

/// Follow the latest log file, printing the last `lines` lines.
///
/// If `channel` is provided, only lines containing that substring are printed.
///
/// Source: `src/commands/logs-follow.ts` - `logsFollowCommand`
pub async fn logs_follow_command(lines: Option<usize>, channel: Option<&str>) -> Result<()> {
    let state_dir = resolve_state_dir();
    let log_dir = state_dir.join("logs");

    // Find the most recent log file.
    let mut entries: Vec<_> = std::fs::read_dir(&log_dir)
        .context("Failed to read log directory")?
        .filter_map(|e| e.ok())
        .filter(|e| e.path().extension().map_or(false, |ext| ext == "log"))
        .collect();
    entries.sort_by_key(|e| {
        std::cmp::Reverse(e.metadata().ok().and_then(|m| m.modified().ok()))
    });

    let log_file = entries.first().context("No log files found")?;
    println!("Following: {}", log_file.path().display());

    // Read last N lines then print.
    let content = std::fs::read_to_string(log_file.path())?;
    let lines_to_show = lines.unwrap_or(20);
    let tail: Vec<&str> = content.lines().rev().take(lines_to_show).collect();

    for line in tail.into_iter().rev() {
        if let Some(ch) = channel {
            if line.contains(ch) {
                println!("{line}");
            }
        } else {
            println!("{line}");
        }
    }

    Ok(())
}
