/// Sandbox configuration types.
///
/// Source: `src/config/types.sandbox.ts`

use std::collections::HashMap;

use serde::{Deserialize, Serialize};

/// Ulimit value: can be a string "soft:hard", a number, or an object { soft, hard }.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(untagged)]
pub enum UlimitValue {
    String(String),
    Number(u64),
    Object {
        #[serde(default, skip_serializing_if = "Option::is_none")]
        soft: Option<u64>,
        #[serde(default, skip_serializing_if = "Option::is_none")]
        hard: Option<u64>,
    },
}

/// Docker memory/memorySwap: can be a string (e.g. "512m") or a number (bytes).
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(untagged)]
pub enum MemoryLimit {
    String(String),
    Number(u64),
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct SandboxDockerSettings {
    /// Docker image to use for sandbox containers.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub image: Option<String>,
    /// Prefix for sandbox container names.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub container_prefix: Option<String>,
    /// Container workdir mount path (default: /workspace).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub workdir: Option<String>,
    /// Run container rootfs read-only.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub read_only_root: Option<bool>,
    /// Extra tmpfs mounts for read-only containers.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tmpfs: Option<Vec<String>>,
    /// Container network mode (bridge|none|custom).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub network: Option<String>,
    /// Container user (uid:gid).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub user: Option<String>,
    /// Drop Linux capabilities.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cap_drop: Option<Vec<String>>,
    /// Extra environment variables for sandbox exec.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub env: Option<HashMap<String, String>>,
    /// Optional setup command run once after container creation.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub setup_command: Option<String>,
    /// Limit container PIDs (0 = Docker default).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub pids_limit: Option<u64>,
    /// Limit container memory (e.g. 512m, 2g, or bytes as number).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub memory: Option<MemoryLimit>,
    /// Limit container memory swap (same format as memory).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub memory_swap: Option<MemoryLimit>,
    /// Limit container CPU shares (e.g. 0.5, 1, 2).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cpus: Option<f64>,
    /// Set ulimit values by name.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub ulimits: Option<HashMap<String, UlimitValue>>,
    /// Seccomp profile (path or profile name).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub seccomp_profile: Option<String>,
    /// AppArmor profile name.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub apparmor_profile: Option<String>,
    /// DNS servers.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub dns: Option<Vec<String>>,
    /// Extra host mappings (e.g. ["api.local:10.0.0.2"]).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub extra_hosts: Option<Vec<String>>,
    /// Additional bind mounts (host:container:mode format).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub binds: Option<Vec<String>>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct SandboxBrowserSettings {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub image: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub container_prefix: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cdp_port: Option<u16>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub vnc_port: Option<u16>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub no_vnc_port: Option<u16>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub headless: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enable_no_vnc: Option<bool>,
    /// Allow sandboxed sessions to target the host browser control server.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_host_control: Option<bool>,
    /// When true, start/reattach to sandbox browser container automatically.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub auto_start: Option<bool>,
    /// Max time to wait for CDP to become reachable after auto-start (ms).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub auto_start_timeout_ms: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct SandboxPruneSettings {
    /// Prune if idle for more than N hours (0 disables).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub idle_hours: Option<u64>,
    /// Prune if older than N days (0 disables).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_age_days: Option<u64>,
}
