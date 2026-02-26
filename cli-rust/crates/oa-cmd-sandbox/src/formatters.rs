/// Formatting utilities for sandbox CLI output.
///
/// Source: `src/commands/sandbox-formatters.ts`
use serde::{Deserialize, Serialize};

/// Shared container info fields used by both sandbox and browser containers.
///
/// Source: `src/commands/sandbox-formatters.ts` — `ContainerItem`
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ContainerItem {
    /// Whether the container is currently running.
    pub running: bool,
    /// Whether the container image matches the expected image.
    pub image_match: bool,
    /// Docker container name.
    pub container_name: String,
    /// Session key associated with this container.
    pub session_key: String,
    /// Docker image name.
    pub image: String,
    /// Container creation timestamp (epoch ms).
    pub created_at_ms: u64,
    /// Last-used timestamp (epoch ms).
    pub last_used_at_ms: u64,
}

/// Information about a sandbox compute container.
///
/// Source: `src/agents/sandbox.ts` — `SandboxContainerInfo`
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SandboxContainerInfo {
    /// Whether the container is running.
    pub running: bool,
    /// Whether the image matches expected.
    pub image_match: bool,
    /// Docker container name.
    pub container_name: String,
    /// Session key.
    pub session_key: String,
    /// Docker image.
    pub image: String,
    /// Created timestamp (epoch ms).
    pub created_at_ms: u64,
    /// Last used timestamp (epoch ms).
    pub last_used_at_ms: u64,
}

/// Information about a sandbox browser container.
///
/// Source: `src/agents/sandbox.ts` — `SandboxBrowserInfo`
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SandboxBrowserInfo {
    /// Whether the container is running.
    pub running: bool,
    /// Whether the image matches expected.
    pub image_match: bool,
    /// Docker container name.
    pub container_name: String,
    /// Session key.
    pub session_key: String,
    /// Docker image.
    pub image: String,
    /// Created timestamp (epoch ms).
    pub created_at_ms: u64,
    /// Last used timestamp (epoch ms).
    pub last_used_at_ms: u64,
    /// Chrome DevTools Protocol port.
    pub cdp_port: u16,
    /// noVNC port (optional).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub no_vnc_port: Option<u16>,
}

/// Format a running/stopped status indicator.
///
/// Source: `src/commands/sandbox-formatters.ts` — `formatStatus`
#[must_use]
pub fn format_status(running: bool) -> &'static str {
    if running { "running" } else { "stopped" }
}

/// Format a simple running/stopped status without emoji.
///
/// Source: `src/commands/sandbox-formatters.ts` — `formatSimpleStatus`
#[must_use]
pub fn format_simple_status(running: bool) -> &'static str {
    if running { "running" } else { "stopped" }
}

/// Format an image match indicator.
///
/// Source: `src/commands/sandbox-formatters.ts` — `formatImageMatch`
#[must_use]
pub fn format_image_match(matches: bool) -> &'static str {
    if matches { "[ok]" } else { "[mismatch]" }
}

/// Count the number of running items in a slice.
///
/// Source: `src/commands/sandbox-formatters.ts` — `countRunning`
#[must_use]
pub fn count_running<T: HasRunning>(items: &[T]) -> usize {
    items.iter().filter(|item| item.is_running()).count()
}

/// Count the number of image mismatches in a slice.
///
/// Source: `src/commands/sandbox-formatters.ts` — `countMismatches`
#[must_use]
pub fn count_mismatches<T: HasImageMatch>(items: &[T]) -> usize {
    items.iter().filter(|item| !item.image_matches()).count()
}

/// Trait for items that have a `running` property.
///
/// Source: `src/commands/sandbox-formatters.ts` — `{ running: boolean }`
pub trait HasRunning {
    /// Returns `true` if the item is running.
    fn is_running(&self) -> bool;
}

/// Trait for items that have an `imageMatch` property.
///
/// Source: `src/commands/sandbox-formatters.ts` — `{ imageMatch: boolean }`
pub trait HasImageMatch {
    /// Returns `true` if the image matches.
    fn image_matches(&self) -> bool;
}

impl HasRunning for SandboxContainerInfo {
    fn is_running(&self) -> bool {
        self.running
    }
}

impl HasImageMatch for SandboxContainerInfo {
    fn image_matches(&self) -> bool {
        self.image_match
    }
}

impl HasRunning for SandboxBrowserInfo {
    fn is_running(&self) -> bool {
        self.running
    }
}

impl HasImageMatch for SandboxBrowserInfo {
    fn image_matches(&self) -> bool {
        self.image_match
    }
}

/// Format a duration in milliseconds as a compact human-readable string.
///
/// Source: `src/infra/format-time/format-duration.ts` — `formatDurationCompact`
#[must_use]
pub fn format_duration_compact(ms: u64) -> String {
    let total_secs = ms / 1000;
    if total_secs < 60 {
        return format!("{total_secs}s");
    }
    let minutes = total_secs / 60;
    if minutes < 60 {
        return format!("{minutes}m");
    }
    let hours = minutes / 60;
    if hours < 24 {
        let remaining_mins = minutes % 60;
        if remaining_mins > 0 {
            return format!("{hours}h {remaining_mins}m");
        }
        return format!("{hours}h");
    }
    let days = hours / 24;
    let remaining_hours = hours % 24;
    if remaining_hours > 0 {
        format!("{days}d {remaining_hours}h")
    } else {
        format!("{days}d")
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn format_status_values() {
        assert_eq!(format_status(true), "running");
        assert_eq!(format_status(false), "stopped");
    }

    #[test]
    fn format_simple_status_values() {
        assert_eq!(format_simple_status(true), "running");
        assert_eq!(format_simple_status(false), "stopped");
    }

    #[test]
    fn format_image_match_values() {
        assert_eq!(format_image_match(true), "[ok]");
        assert_eq!(format_image_match(false), "[mismatch]");
    }

    #[test]
    fn count_running_items() {
        let items = vec![
            SandboxContainerInfo {
                running: true,
                image_match: true,
                container_name: "c1".to_owned(),
                session_key: "s1".to_owned(),
                image: "img".to_owned(),
                created_at_ms: 0,
                last_used_at_ms: 0,
            },
            SandboxContainerInfo {
                running: false,
                image_match: true,
                container_name: "c2".to_owned(),
                session_key: "s2".to_owned(),
                image: "img".to_owned(),
                created_at_ms: 0,
                last_used_at_ms: 0,
            },
            SandboxContainerInfo {
                running: true,
                image_match: false,
                container_name: "c3".to_owned(),
                session_key: "s3".to_owned(),
                image: "img".to_owned(),
                created_at_ms: 0,
                last_used_at_ms: 0,
            },
        ];
        assert_eq!(count_running(&items), 2);
    }

    #[test]
    fn count_mismatches_items() {
        let items = vec![
            SandboxContainerInfo {
                running: true,
                image_match: true,
                container_name: "c1".to_owned(),
                session_key: "s1".to_owned(),
                image: "img".to_owned(),
                created_at_ms: 0,
                last_used_at_ms: 0,
            },
            SandboxContainerInfo {
                running: false,
                image_match: false,
                container_name: "c2".to_owned(),
                session_key: "s2".to_owned(),
                image: "img".to_owned(),
                created_at_ms: 0,
                last_used_at_ms: 0,
            },
        ];
        assert_eq!(count_mismatches(&items), 1);
    }

    #[test]
    fn format_duration_compact_seconds() {
        assert_eq!(format_duration_compact(5000), "5s");
        assert_eq!(format_duration_compact(59_000), "59s");
    }

    #[test]
    fn format_duration_compact_minutes() {
        assert_eq!(format_duration_compact(60_000), "1m");
        assert_eq!(format_duration_compact(3_540_000), "59m");
    }

    #[test]
    fn format_duration_compact_hours() {
        assert_eq!(format_duration_compact(3_600_000), "1h");
        assert_eq!(format_duration_compact(5_400_000), "1h 30m");
    }

    #[test]
    fn format_duration_compact_days() {
        assert_eq!(format_duration_compact(86_400_000), "1d");
        assert_eq!(format_duration_compact(129_600_000), "1d 12h");
    }

    #[test]
    fn format_duration_compact_zero() {
        assert_eq!(format_duration_compact(0), "0s");
    }
}
