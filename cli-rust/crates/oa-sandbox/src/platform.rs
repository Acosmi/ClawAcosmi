//! Platform capability detection and sandbox backend selection.
//!
//! Detects available isolation mechanisms at runtime and selects the strongest
//! backend following the degradation chain:
//!
//! - **Linux**:   `Namespace+Seccomp+Landlock` → `Landlock+Seccomp` → Docker fallback
//! - **macOS**:   `Seatbelt FFI` → Docker fallback
//! - **Windows**: `RestrictedToken+JobObject` → `JobObject only` → Docker fallback

use tracing::{debug, info, warn};

use crate::SandboxRunner;
use crate::config::{BackendPreference, SandboxConfig};
use crate::error::SandboxError;

/// Detected sandbox backend variant.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SandboxBackend {
    // ── Linux ──────────────────────────────────────────────────────
    /// Full Linux isolation: User/PID/Mount Namespaces + Seccomp-BPF + Landlock LSM.
    LinuxFull,
    /// Unprivileged-only: Landlock LSM + Seccomp-BPF (no namespace required).
    LinuxLandlockSeccomp,

    // ── macOS ──────────────────────────────────────────────────────
    /// macOS Seatbelt via `sandbox_init_with_parameters` FFI.
    MacosSeatbelt,

    // ── Windows ────────────────────────────────────────────────────
    /// Full Windows isolation: Restricted Token + Job Object.
    WindowsFull,
    /// Job Object only (resource limits + process tree reaping, no token restriction).
    WindowsJobOnly,

    // ── Fallback ───────────────────────────────────────────────────
    /// Docker CLI fallback when native backends are unavailable.
    DockerFallback,
}

impl SandboxBackend {
    /// Human-readable name for this backend (used in `SandboxOutput.sandbox_backend`).
    #[must_use]
    pub const fn name(self) -> &'static str {
        match self {
            Self::LinuxFull => "linux-namespace+seccomp+landlock",
            Self::LinuxLandlockSeccomp => "linux-landlock+seccomp",
            Self::MacosSeatbelt => "macos-seatbelt",
            Self::WindowsFull => "windows-restricted-token+job",
            Self::WindowsJobOnly => "windows-job-object",
            Self::DockerFallback => "docker-fallback",
        }
    }

    /// Whether this backend is a native OS sandbox (not Docker).
    #[must_use]
    pub const fn is_native(self) -> bool {
        !matches!(self, Self::DockerFallback)
    }
}

// ── Linux capability detection ─────────────────────────────────────────────

/// Linux-specific sandbox capabilities.
#[cfg(target_os = "linux")]
#[derive(Debug, Clone)]
pub struct LinuxCapabilities {
    /// Whether unprivileged user namespaces are available.
    /// Ubuntu 24.04+ may block these via AppArmor.
    pub has_user_namespace: bool,

    /// Landlock ABI version (0 = not available, 1-6 = supported version).
    /// ABI 4+ (kernel 6.7) required for TCP network filtering.
    pub landlock_abi_version: u8,

    /// Whether seccomp-BPF is available (kernel 3.5+).
    pub has_seccomp: bool,

    /// Whether cgroups v2 delegation is available (via systemd --user).
    pub has_cgroup_v2_delegation: bool,
}

#[cfg(target_os = "linux")]
impl LinuxCapabilities {
    /// Detect Linux sandbox capabilities from the current system.
    pub fn detect() -> Self {
        let has_user_namespace = detect_user_namespace();
        let landlock_abi_version = detect_landlock_abi();
        let has_seccomp = detect_seccomp();
        let has_cgroup_v2_delegation = detect_cgroup_v2_delegation();

        let caps = Self {
            has_user_namespace,
            landlock_abi_version,
            has_seccomp,
            has_cgroup_v2_delegation,
        };

        info!(
            user_ns = caps.has_user_namespace,
            landlock_abi = caps.landlock_abi_version,
            seccomp = caps.has_seccomp,
            cgroup_v2 = caps.has_cgroup_v2_delegation,
            "Linux sandbox capabilities detected"
        );

        caps
    }

    /// Select the strongest available backend.
    fn select_backend(&self) -> Option<SandboxBackend> {
        if !self.has_seccomp {
            warn!("seccomp not available — no native sandbox possible");
            return None;
        }

        if self.landlock_abi_version == 0 {
            warn!("landlock not available — no native sandbox possible");
            return None;
        }

        if self.has_user_namespace {
            debug!("full Linux sandbox available (namespace+seccomp+landlock)");
            Some(SandboxBackend::LinuxFull)
        } else {
            debug!("user namespace not available, using landlock+seccomp only");
            Some(SandboxBackend::LinuxLandlockSeccomp)
        }
    }
}

/// Check if unprivileged user namespaces are available.
#[cfg(target_os = "linux")]
fn detect_user_namespace() -> bool {
    // Method 1: Check sysctl (not all kernels expose this)
    if let Ok(content) = std::fs::read_to_string("/proc/sys/kernel/unprivileged_userns_clone") {
        if content.trim() == "0" {
            debug!("unprivileged user namespaces disabled via sysctl");
            return false;
        }
    }

    // Method 2: Check AppArmor restriction (Ubuntu 24.04+)
    if let Ok(content) =
        std::fs::read_to_string("/proc/sys/kernel/apparmor_restrict_unprivileged_userns")
    {
        if content.trim() == "1" {
            debug!("unprivileged user namespaces restricted by AppArmor");
            return false;
        }
    }

    // Method 3: Try to create a user namespace (definitive test)
    // This is the most reliable check but has a small cost.
    // For Phase 1, we rely on the file-based checks above.
    // Phase 2 will add an actual unshare(CLONE_NEWUSER) test.

    debug!("user namespace appears available");
    true
}

/// Detect the Landlock ABI version by checking the LSM list.
#[cfg(target_os = "linux")]
fn detect_landlock_abi() -> u8 {
    // Check if Landlock is listed as an active LSM
    let lsm_list = match std::fs::read_to_string("/sys/kernel/security/lsm") {
        Ok(content) => content,
        Err(e) => {
            debug!("cannot read /sys/kernel/security/lsm: {e}");
            return 0;
        }
    };

    if !lsm_list.contains("landlock") {
        debug!("landlock not listed in active LSMs");
        return 0;
    }

    // Landlock is available; determine ABI version.
    // The actual ABI version is determined by creating a ruleset with
    // landlock_create_ruleset(NULL, 0, LANDLOCK_CREATE_RULESET_VERSION).
    // For Phase 1, we report ABI 1 as minimum if Landlock is active.
    // Phase 2 will use the `landlock` crate's `Compatible` trait for precise detection.
    debug!("landlock is active in LSM list");
    1
}

/// Check if seccomp-BPF is available.
#[cfg(target_os = "linux")]
fn detect_seccomp() -> bool {
    // Check /proc/sys/kernel/seccomp/actions_avail for BPF support
    if std::fs::read_to_string("/proc/sys/kernel/seccomp/actions_avail").is_ok() {
        debug!("seccomp BPF actions available");
        return true;
    }

    // Fallback: check kernel config
    if let Ok(content) = std::fs::read_to_string("/proc/config.gz") {
        // This is compressed; skip for now
        let _ = content;
    }

    // Most modern kernels (3.17+) have seccomp
    debug!("assuming seccomp available (modern kernel)");
    true
}

/// Check if cgroups v2 delegation is available.
#[cfg(target_os = "linux")]
fn detect_cgroup_v2_delegation() -> bool {
    // Check if cgroups v2 unified hierarchy is mounted
    let controllers = match std::fs::read_to_string("/sys/fs/cgroup/cgroup.controllers") {
        Ok(content) => content,
        Err(_) => {
            debug!("cgroups v2 unified hierarchy not found");
            return false;
        }
    };

    // Check for user-level delegation (systemd)
    // A user-delegated cgroup should exist at /sys/fs/cgroup/user.slice/user-{UID}.slice/
    let uid = unsafe { libc::getuid() };
    let user_cgroup = format!("/sys/fs/cgroup/user.slice/user-{uid}.slice");
    let delegated = std::path::Path::new(&user_cgroup).is_dir();

    debug!(
        controllers = controllers.trim(),
        delegated, "cgroups v2 detection"
    );

    delegated
}

// ── macOS capability detection ─────────────────────────────────────────────

/// macOS-specific sandbox capabilities.
#[cfg(target_os = "macos")]
#[derive(Debug, Clone)]
pub struct MacosCapabilities {
    /// Whether Seatbelt (`sandbox_init`) is available.
    pub has_seatbelt: bool,

    /// macOS version (major, minor). E.g., (15, 0) for macOS 15.0.
    pub os_version: (u32, u32),
}

#[cfg(target_os = "macos")]
impl MacosCapabilities {
    /// Detect macOS sandbox capabilities.
    pub fn detect() -> Self {
        let os_version = detect_macos_version();
        let has_seatbelt = detect_seatbelt();

        let caps = Self {
            has_seatbelt,
            os_version,
        };

        info!(
            seatbelt = caps.has_seatbelt,
            os_major = caps.os_version.0,
            os_minor = caps.os_version.1,
            "macOS sandbox capabilities detected"
        );

        caps
    }

    /// Select the available backend.
    fn select_backend(&self) -> Option<SandboxBackend> {
        if self.has_seatbelt {
            debug!("macOS Seatbelt sandbox available");
            Some(SandboxBackend::MacosSeatbelt)
        } else {
            warn!("Seatbelt not available on this macOS version");
            None
        }
    }
}

/// Detect macOS version from sysctl.
#[cfg(target_os = "macos")]
fn detect_macos_version() -> (u32, u32) {
    // Use sw_vers or sysctl to get version
    let output = std::process::Command::new("sw_vers")
        .arg("-productVersion")
        .output();

    match output {
        Ok(out) if out.status.success() => {
            let version_str = String::from_utf8_lossy(&out.stdout);
            let parts: Vec<&str> = version_str.trim().split('.').collect();
            let major = parts.first().and_then(|s| s.parse().ok()).unwrap_or(0);
            let minor = parts.get(1).and_then(|s| s.parse().ok()).unwrap_or(0);
            debug!(major, minor, "detected macOS version");
            (major, minor)
        }
        _ => {
            warn!("failed to detect macOS version, defaulting to (0, 0)");
            (0, 0)
        }
    }
}

/// Check if Seatbelt sandbox is available.
#[cfg(target_os = "macos")]
fn detect_seatbelt() -> bool {
    // sandbox-exec CLI exists (deprecated but still functional through macOS 15)
    let cli_exists = std::path::Path::new("/usr/bin/sandbox-exec").exists();

    // The real check: sandbox_init_with_parameters should be available via libSystem.
    // For Phase 1, we check the CLI as a proxy. Phase 3 will verify FFI availability
    // via dlsym at runtime.
    if cli_exists {
        debug!("sandbox-exec found at /usr/bin/sandbox-exec");
    } else {
        debug!("sandbox-exec not found");
    }

    // Seatbelt is available on macOS 10.5+ and still functional through macOS 15
    cli_exists
}

// ── Windows capability detection ───────────────────────────────────────────

/// Windows-specific sandbox capabilities.
#[cfg(target_os = "windows")]
#[derive(Debug, Clone)]
pub struct WindowsCapabilities {
    /// Whether Job Objects are available (always true on modern Windows).
    pub has_job_objects: bool,

    /// Whether AppContainer is supported (Windows 8+).
    pub has_appcontainer: bool,
}

#[cfg(target_os = "windows")]
impl WindowsCapabilities {
    /// Detect Windows sandbox capabilities.
    pub fn detect() -> Self {
        let caps = Self {
            // Job Objects are available on all supported Windows versions
            has_job_objects: true,
            // AppContainer is available on Windows 8+ but has compatibility issues
            has_appcontainer: detect_appcontainer_support(),
        };

        info!(
            job_objects = caps.has_job_objects,
            appcontainer = caps.has_appcontainer,
            "Windows sandbox capabilities detected"
        );

        caps
    }

    /// Select the strongest available backend.
    fn select_backend(&self) -> Option<SandboxBackend> {
        if self.has_job_objects {
            // Restricted Token + Job Object is the default on Windows.
            // AppContainer is opt-in due to compatibility issues.
            debug!("Windows full sandbox available (restricted token + job object)");
            Some(SandboxBackend::WindowsFull)
        } else {
            warn!("Job Objects not available — unexpected on modern Windows");
            None
        }
    }
}

/// Check if AppContainer is supported (Windows 8+).
#[cfg(target_os = "windows")]
fn detect_appcontainer_support() -> bool {
    // AppContainer was introduced in Windows 8 (NT 6.2).
    // On modern Windows 10/11 it's always present, but has compatibility issues
    // with traditional .exe (Chromium/Electron don't use it for renderer processes).
    // We report it as available but leave enablement to user opt-in.
    debug!("AppContainer assumed available on modern Windows");
    true
}

// ── Backend selection ──────────────────────────────────────────────────────

/// Check if Docker CLI is available.
fn docker_available() -> bool {
    which::which("docker").is_ok()
}

/// Select the best available sandbox runner for the given configuration.
///
/// This is the main entry point for the degradation chain.
pub fn select_runner(config: &SandboxConfig) -> Result<Box<dyn SandboxRunner>, SandboxError> {
    // Validate config before selecting a runner
    config.validate()?;

    match config.backend {
        BackendPreference::Native => select_native_runner(),
        BackendPreference::Docker => select_docker_runner(),
        BackendPreference::Auto => select_auto_runner(),
    }
}

/// Select the best native runner, or error if none available.
fn select_native_runner() -> Result<Box<dyn SandboxRunner>, SandboxError> {
    #[cfg(target_os = "linux")]
    {
        let caps = LinuxCapabilities::detect();
        if let Some(backend) = caps.select_backend() {
            return Ok(Box::new(crate::linux::LinuxRunner::new(backend, caps)));
        }
        return Err(SandboxError::PlatformNotSupported {
            platform: "linux".into(),
            reason: "neither landlock nor seccomp available".into(),
        });
    }

    #[cfg(target_os = "macos")]
    {
        let caps = MacosCapabilities::detect();
        if let Some(_backend) = caps.select_backend() {
            return Ok(Box::new(crate::macos::MacosRunner::new(caps)));
        }
        Err(SandboxError::PlatformNotSupported {
            platform: "macos".into(),
            reason: "seatbelt not available".into(),
        })
    }

    #[cfg(target_os = "windows")]
    {
        let caps = WindowsCapabilities::detect();
        if let Some(backend) = caps.select_backend() {
            return Ok(Box::new(crate::windows::WindowsRunner::new(backend, caps)));
        }
        return Err(SandboxError::PlatformNotSupported {
            platform: "windows".into(),
            reason: "job objects not available".into(),
        });
    }

    #[cfg(not(any(target_os = "linux", target_os = "macos", target_os = "windows")))]
    Err(SandboxError::PlatformNotSupported {
        platform: std::env::consts::OS.into(),
        reason: "unsupported operating system".into(),
    })
}

/// Select Docker fallback runner, or error if Docker is not available.
fn select_docker_runner() -> Result<Box<dyn SandboxRunner>, SandboxError> {
    if docker_available() {
        Ok(Box::new(crate::docker::DockerFallbackRunner::new()))
    } else {
        Err(SandboxError::PlatformNotSupported {
            platform: "docker".into(),
            reason: "docker CLI not found in PATH".into(),
        })
    }
}

/// Auto-select: try native first, then Docker fallback.
fn select_auto_runner() -> Result<Box<dyn SandboxRunner>, SandboxError> {
    match select_native_runner() {
        Ok(runner) => {
            info!(backend = runner.name(), "selected native sandbox backend");
            Ok(runner)
        }
        Err(native_err) => {
            info!(
                native_error = %native_err,
                "native sandbox unavailable, trying Docker fallback"
            );

            match select_docker_runner() {
                Ok(runner) => {
                    warn!("using Docker fallback — native sandbox unavailable");
                    Ok(runner)
                }
                Err(docker_err) => Err(SandboxError::NoBackendAvailable {
                    native_reason: native_err.to_string(),
                    docker_reason: docker_err.to_string(),
                }),
            }
        }
    }
}
