/// `channels logs` command: tails the gateway log file and filters entries
/// by channel.
///
/// Source: `src/commands/channels/logs.ts`

use std::collections::HashSet;
use std::path::Path;

use anyhow::Result;
use serde::{Deserialize, Serialize};

use oa_channels::registry::CHAT_CHANNEL_ORDER;
use oa_terminal::theme::Theme;

/// Options for the `channels logs` subcommand.
///
/// Source: `src/commands/channels/logs.ts` — `ChannelsLogsOptions`
#[derive(Debug, Clone, Default)]
pub struct ChannelsLogsOptions {
    /// Channel filter (e.g. "discord", "all").
    pub channel: Option<String>,
    /// Number of log lines to display.
    pub lines: Option<String>,
    /// Output in JSON format.
    pub json: bool,
}

/// Default number of log lines to display.
///
/// Source: `src/commands/channels/logs.ts` — `DEFAULT_LIMIT`
const DEFAULT_LIMIT: usize = 200;

/// Maximum bytes to read from the end of the log file.
///
/// Source: `src/commands/channels/logs.ts` — `MAX_BYTES`
const MAX_BYTES: u64 = 1_000_000;

/// A parsed log line with structured fields.
///
/// Source: `src/commands/channels/logs.ts` — `LogLine`
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct LogLine {
    /// ISO timestamp.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub time: Option<String>,
    /// Log level (e.g. "INFO", "ERROR").
    #[serde(skip_serializing_if = "Option::is_none")]
    pub level: Option<String>,
    /// Log message text.
    pub message: String,
    /// Subsystem name.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub subsystem: Option<String>,
    /// Module name.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub module: Option<String>,
}

/// Build the set of known channel IDs (plus "all").
///
/// Source: `src/commands/channels/logs.ts` — `getChannelSet`
fn get_channel_set() -> HashSet<String> {
    let mut set: HashSet<String> = CHAT_CHANNEL_ORDER
        .iter()
        .map(|id| id.as_str().to_owned())
        .collect();
    set.insert("all".to_owned());
    set
}

/// Parse and normalize the channel filter from user input.
///
/// Source: `src/commands/channels/logs.ts` — `parseChannelFilter`
fn parse_channel_filter(raw: Option<&str>) -> String {
    let trimmed = raw.unwrap_or("").trim().to_lowercase();
    if trimmed.is_empty() {
        return "all".to_owned();
    }
    if get_channel_set().contains(&trimmed) {
        trimmed
    } else {
        "all".to_owned()
    }
}

/// Check whether a log line matches the channel filter.
///
/// Source: `src/commands/channels/logs.ts` — `matchesChannel`
fn matches_channel(line: &LogLine, channel: &str) -> bool {
    if channel == "all" {
        return true;
    }
    let needle = format!("gateway/channels/{channel}");
    if let Some(ref subsystem) = line.subsystem {
        if subsystem.contains(&needle) {
            return true;
        }
    }
    if let Some(ref module) = line.module {
        if module.contains(channel) {
            return true;
        }
    }
    false
}

/// Try to parse a raw log line string into a structured `LogLine`.
///
/// Attempts JSON parsing first; falls back to treating the whole line as a
/// plain message.
///
/// Source: `src/commands/channels/logs.ts` — `parseLogLine`
fn parse_log_line(raw: &str) -> Option<LogLine> {
    let trimmed = raw.trim();
    if trimmed.is_empty() {
        return None;
    }
    // Try JSON
    if let Ok(parsed) = serde_json::from_str::<serde_json::Value>(trimmed) {
        let obj = parsed.as_object()?;
        let time = obj
            .get("time")
            .or_else(|| obj.get("timestamp"))
            .and_then(|v| v.as_str())
            .map(String::from);
        let level = obj
            .get("level")
            .and_then(|v| v.as_str())
            .map(String::from);
        let message = obj
            .get("msg")
            .or_else(|| obj.get("message"))
            .and_then(|v| v.as_str())
            .unwrap_or("")
            .to_owned();
        let subsystem = obj
            .get("subsystem")
            .or_else(|| obj.get("module"))
            .and_then(|v| v.as_str())
            .map(String::from);
        let module = obj
            .get("module")
            .and_then(|v| v.as_str())
            .map(String::from);
        return Some(LogLine {
            time,
            level,
            message,
            subsystem,
            module,
        });
    }
    // Fallback: plain text
    Some(LogLine {
        time: None,
        level: None,
        message: trimmed.to_owned(),
        subsystem: None,
        module: None,
    })
}

/// Read the last N lines of a file, reading at most `MAX_BYTES` from the end.
///
/// Source: `src/commands/channels/logs.ts` — `readTailLines`
async fn read_tail_lines(file: &Path, limit: usize) -> Vec<String> {
    let meta = match tokio::fs::metadata(file).await {
        Ok(m) => m,
        Err(_) => return Vec::new(),
    };

    let size = meta.len();
    let start = if size > MAX_BYTES { size - MAX_BYTES } else { 0 };

    let content = match tokio::fs::read_to_string(file).await {
        Ok(c) => c,
        Err(_) => return Vec::new(),
    };

    let text = if start > 0 {
        // Skip first line (may be partial)
        let bytes = content.as_bytes();
        let skip_to = start as usize;
        if skip_to < bytes.len() {
            let sub = &content[skip_to..];
            match sub.find('\n') {
                Some(idx) => &sub[idx + 1..],
                None => sub,
            }
        } else {
            ""
        }
    } else {
        &content
    };

    let mut lines: Vec<String> = text
        .lines()
        .map(String::from)
        .collect();

    // Remove trailing empty line
    if lines.last().is_some_and(|l| l.is_empty()) {
        lines.pop();
    }

    // Keep only the last `limit` lines
    if lines.len() > limit {
        lines = lines.split_off(lines.len() - limit);
    }

    lines
}

/// Resolve the log file path.
///
/// In the full implementation this reads from the logging config;
/// here we use a sensible default based on the state directory.
///
/// Source: `src/commands/channels/logs.ts` — `getResolvedLoggerSettings().file`
fn resolve_log_file_path() -> std::path::PathBuf {
    // Use XDG_DATA_HOME if set, otherwise fall back to platform defaults
    if let Ok(xdg) = std::env::var("XDG_DATA_HOME") {
        return std::path::PathBuf::from(xdg)
            .join("openacosmi")
            .join("gateway.log");
    }
    if let Ok(home) = std::env::var("HOME") {
        let base = if cfg!(target_os = "macos") {
            std::path::PathBuf::from(&home).join("Library/Application Support")
        } else {
            std::path::PathBuf::from(&home).join(".local/share")
        };
        return base.join("openacosmi").join("gateway.log");
    }
    std::path::PathBuf::from(".").join("openacosmi").join("gateway.log")
}

/// Execute the `channels logs` command.
///
/// Source: `src/commands/channels/logs.ts` — `channelsLogsCommand`
pub async fn channels_logs_command(opts: &ChannelsLogsOptions) -> Result<()> {
    let channel = parse_channel_filter(opts.channel.as_deref());

    let limit_raw = opts
        .lines
        .as_deref()
        .unwrap_or("")
        .parse::<usize>()
        .ok();
    let limit = match limit_raw {
        Some(n) if n > 0 => n,
        _ => DEFAULT_LIMIT,
    };

    let file = resolve_log_file_path();
    let raw_lines = read_tail_lines(&file, limit * 4).await;

    let parsed: Vec<LogLine> = raw_lines
        .iter()
        .filter_map(|line| parse_log_line(line))
        .collect();

    let filtered: Vec<&LogLine> = parsed
        .iter()
        .filter(|line| matches_channel(line, &channel))
        .collect();

    let display: Vec<&&LogLine> = if filtered.len() > limit {
        filtered[filtered.len() - limit..].iter().collect()
    } else {
        filtered.iter().collect()
    };

    if opts.json {
        let payload = serde_json::json!({
            "file": file.display().to_string(),
            "channel": channel,
            "lines": display,
        });
        println!("{}", serde_json::to_string_pretty(&payload)?);
        return Ok(());
    }

    println!("{}", Theme::info(&format!("Log file: {}", file.display())));
    if channel != "all" {
        println!("{}", Theme::info(&format!("Channel: {channel}")));
    }

    if display.is_empty() {
        println!("{}", Theme::muted("No matching log lines."));
        return Ok(());
    }

    for line in &display {
        let ts = line
            .time
            .as_deref()
            .map(|t| format!("{t} "))
            .unwrap_or_default();
        let level = line
            .level
            .as_deref()
            .map(|l| format!("{} ", l.to_lowercase()))
            .unwrap_or_default();
        let text = format!("{ts}{level}{}", line.message).trim().to_owned();
        println!("{text}");
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_channel_filter_all() {
        assert_eq!(parse_channel_filter(None), "all");
        assert_eq!(parse_channel_filter(Some("")), "all");
        assert_eq!(parse_channel_filter(Some("  ")), "all");
    }

    #[test]
    fn parse_channel_filter_known() {
        assert_eq!(parse_channel_filter(Some("discord")), "discord");
        assert_eq!(parse_channel_filter(Some("TELEGRAM")), "telegram");
        assert_eq!(parse_channel_filter(Some("slack")), "slack");
    }

    #[test]
    fn parse_channel_filter_unknown_falls_to_all() {
        assert_eq!(parse_channel_filter(Some("matrix")), "all");
        assert_eq!(parse_channel_filter(Some("unknown")), "all");
    }

    #[test]
    fn matches_channel_all_matches_everything() {
        let line = LogLine {
            time: None,
            level: None,
            message: "hello".to_owned(),
            subsystem: None,
            module: None,
        };
        assert!(matches_channel(&line, "all"));
    }

    #[test]
    fn matches_channel_subsystem() {
        let line = LogLine {
            time: None,
            level: None,
            message: "msg".to_owned(),
            subsystem: Some("gateway/channels/discord".to_owned()),
            module: None,
        };
        assert!(matches_channel(&line, "discord"));
        assert!(!matches_channel(&line, "telegram"));
    }

    #[test]
    fn matches_channel_module() {
        let line = LogLine {
            time: None,
            level: None,
            message: "msg".to_owned(),
            subsystem: None,
            module: Some("slack-handler".to_owned()),
        };
        assert!(matches_channel(&line, "slack"));
        assert!(!matches_channel(&line, "discord"));
    }

    #[test]
    fn parse_log_line_json() {
        let raw = r#"{"time":"2024-01-01","level":"INFO","msg":"hello","subsystem":"test"}"#;
        let line = parse_log_line(raw).expect("should parse");
        assert_eq!(line.time.as_deref(), Some("2024-01-01"));
        assert_eq!(line.level.as_deref(), Some("INFO"));
        assert_eq!(line.message, "hello");
        assert_eq!(line.subsystem.as_deref(), Some("test"));
    }

    #[test]
    fn parse_log_line_plain_text() {
        let line = parse_log_line("just a plain message").expect("should parse");
        assert_eq!(line.message, "just a plain message");
        assert!(line.time.is_none());
    }

    #[test]
    fn parse_log_line_empty() {
        assert!(parse_log_line("").is_none());
        assert!(parse_log_line("  ").is_none());
    }

    #[test]
    fn get_channel_set_includes_all_channels() {
        let set = get_channel_set();
        assert!(set.contains("all"));
        assert!(set.contains("discord"));
        assert!(set.contains("telegram"));
        assert!(set.contains("slack"));
        assert!(set.contains("signal"));
        assert!(set.contains("whatsapp"));
        assert!(set.contains("googlechat"));
        assert!(set.contains("imessage"));
    }
}
