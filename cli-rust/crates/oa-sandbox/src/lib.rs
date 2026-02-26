//! OS-native sandbox runtime for `OpenAcosmi`.
//!
//! Provides platform-specific process isolation with automatic degradation:
//!
//! | Platform | Primary Backend | Fallback |
//! |----------|----------------|----------|
//! | Linux    | Landlock + Seccomp (+ Namespaces) | Docker |
//! | macOS    | Seatbelt FFI (`sandbox_init_with_parameters`) | Docker |
//! | Windows  | Restricted Token + Job Object | Docker |
//!
//! # Usage
//!
//! ```rust,no_run
//! use oa_sandbox::config::{SandboxConfig, SecurityLevel};
//! use oa_sandbox::select_runner;
//!
//! # fn example(config: SandboxConfig) -> anyhow::Result<()> {
//! let runner = select_runner(&config)?;
//! let output = runner.run(&config)?;
//! println!("{}", serde_json::to_string(&output)?);
//! # Ok(())
//! # }
//! ```
//!
//! # Safety
//!
//! This crate overrides the workspace `unsafe_code = "forbid"` lint because it
//! requires `unsafe` for platform FFI calls:
//! - Linux: `libseccomp` BPF filter installation
//! - macOS: `sandbox_init_with_parameters` via `extern "C"`
//! - Windows: Win32 API calls via the `windows` crate
//!
//! All `unsafe` blocks include mandatory `// SAFETY:` comments and are subject
//! to Skill 4 line-level audit.
#![allow(unsafe_code)]

pub mod config;
pub mod error;
pub mod output;
pub mod platform;
pub mod worker;

#[cfg(target_os = "linux")]
pub mod linux;

#[cfg(target_os = "macos")]
pub mod macos;

#[cfg(target_os = "windows")]
pub mod windows;

pub mod docker;

use crate::config::SandboxConfig;
use crate::error::SandboxError;
use crate::output::SandboxOutput;

/// Core sandbox execution trait.
///
/// Each platform backend implements this trait. The [`platform`] module
/// selects the best available backend based on runtime capability detection.
pub trait SandboxRunner: Send + Sync {
    /// Human-readable name of this backend (e.g., `"linux-landlock+seccomp"`).
    fn name(&self) -> &'static str;

    /// Returns `true` if this backend's prerequisites are met on the current system.
    fn available(&self) -> bool;

    /// Execute a command within the sandbox.
    ///
    /// Returns a [`SandboxOutput`] with captured stdout/stderr, exit code, and timing.
    /// The output is serialized to JSON for the Go orchestration layer.
    fn run(&self, config: &SandboxConfig) -> Result<SandboxOutput, SandboxError>;
}

/// Select the best available sandbox runner for the current platform and config.
///
/// Follows the degradation chain per platform (see module-level docs).
/// With [`BackendPreference::Auto`](config::BackendPreference::Auto) (default),
/// tries native first, then Docker fallback.
///
/// # Errors
///
/// Returns [`SandboxError::NoBackendAvailable`] if no backend (native or Docker) is usable.
pub fn select_runner(config: &SandboxConfig) -> Result<Box<dyn SandboxRunner>, SandboxError> {
    platform::select_runner(config)
}
