//! Windows sandbox backend.
//!
//! Provides process isolation using:
//!
//! | Layer | Required? | What it does |
//! |-------|-----------|-------------|
//! | **Job Object** | Yes | Resource limits (memory, CPU, PIDs) + kill-on-close |
//! | **Restricted Token** | Yes* | Stripped privileges, deny-only groups, Low IL |
//! | **NTFS ACL** | Yes* | Temporary workspace access grant, auto-revoked |
//! | **AppContainer** | Optional | Network + object namespace isolation (opt-in) |
//!
//! *Only with `WindowsFull` backend. `WindowsJobOnly` uses Job Objects alone.
//!
//! # Execution flow
//!
//! ```text
//! WindowsRunner::run(config)
//!        │
//!        ├── 1. Create Job Object (resource limits + KILL_ON_JOB_CLOSE)
//!        ├── 2. Create Restricted Token (strip privileges, Low IL)
//!        ├── 3. Grant workspace NTFS ACL access to restricted SID
//!        ├── 4. CreateProcessAsUserW with restricted token
//!        ├── 5. AssignProcessToJobObject
//!        ├── 6. ResumeThread (process starts suspended)
//!        ├── 7. Timeout thread + WaitForSingleObject
//!        │
//!        ▼
//!        Drop: AclGuard revokes ACL, JobGuard kills processes
//! ```
//!
//! # Degradation chain
//!
//! ```text
//! RestrictedToken + JobObject  →  JobObject only  →  Docker fallback
//! (full isolation)                (resource limits)
//! ```
//!
//! # Key decisions (from Chromium/Electron analysis)
//!
//! - AppContainer has severe compatibility issues with traditional `.exe`
//! - Default to Restricted Token + Job Object (proven in Chromium renderer)
//! - AppContainer is opt-in only when the user explicitly requests it
//! - Process created SUSPENDED to assign Job Object before any code runs

pub mod acl;
pub mod appcontainer;
pub mod job;
pub mod token;

use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};
use std::time::{Duration, Instant};

use tracing::{debug, info, warn};

use windows::Win32::Foundation::{CloseHandle, HANDLE, WAIT_OBJECT_0, WAIT_TIMEOUT};
use windows::Win32::System::Threading::{
    CREATE_SUSPENDED, CREATE_UNICODE_ENVIRONMENT, CreateProcessAsUserW, GetExitCodeProcess,
    PROCESS_INFORMATION, ResumeThread, STARTUPINFOW, TerminateProcess, WaitForSingleObject,
};

use crate::SandboxRunner;
use crate::config::{MountMode, SandboxConfig, SecurityLevel};
use crate::error::SandboxError;
use crate::output::SandboxOutput;
use crate::platform::{SandboxBackend, WindowsCapabilities};

/// Windows sandbox runner.
pub struct WindowsRunner {
    backend: SandboxBackend,
    capabilities: WindowsCapabilities,
}

impl WindowsRunner {
    /// Create a new Windows runner with the given backend selection and capabilities.
    #[must_use]
    pub fn new(backend: SandboxBackend, capabilities: WindowsCapabilities) -> Self {
        Self {
            backend,
            capabilities,
        }
    }
}

impl SandboxRunner for WindowsRunner {
    fn name(&self) -> &'static str {
        self.backend.name()
    }

    fn available(&self) -> bool {
        self.capabilities.has_job_objects
    }

    #[allow(clippy::too_many_lines)]
    fn run(&self, config: &SandboxConfig) -> Result<SandboxOutput, SandboxError> {
        let start = Instant::now();
        let use_restricted_token = self.backend == SandboxBackend::WindowsFull;

        info!(
            backend = self.name(),
            use_restricted_token,
            command = %config.command,
            "starting sandboxed execution"
        );

        // ── 1. Create Job Object with resource limits ────────────────────
        let job_guard = job::create_job_object(&config.resource_limits)?;

        // ── 2. Create Restricted Token (if full isolation) ───────────────
        let restricted_token = if use_restricted_token {
            Some(token::create_restricted_token()?)
        } else {
            None
        };

        // ── 3. Grant workspace ACL access ────────────────────────────────
        // The restricted token runs at Low Integrity Level and may not have
        // access to the workspace directory. We grant temporary access.
        let _acl_guards: Vec<acl::AclGuard> = if use_restricted_token {
            let mut guards = Vec::new();

            // Workspace: read-only for L0, read-write for L1+
            let workspace_mode = match config.security_level {
                SecurityLevel::L0Deny => MountMode::ReadOnly,
                SecurityLevel::L1Sandbox | SecurityLevel::L2Full => MountMode::ReadWrite,
            };

            // Get the token's user SID for ACL entries.
            // For restricted tokens, we use the original user SID (not the logon SID)
            // because the DACL check uses the enabled SIDs from the token.
            let token_handle = restricted_token
                .as_ref()
                .map_or(HANDLE::default(), |t| t.handle());
            let user_sid = get_token_user_sid(token_handle)?;

            guards.push(acl::grant_workspace_access(
                &config.workspace,
                user_sid.sid,
                workspace_mode,
            )?);

            // Additional mounts
            for mount in &config.mounts {
                if mount.host_path.exists() {
                    guards.push(acl::grant_workspace_access(
                        &mount.host_path,
                        user_sid.sid,
                        mount.mode,
                    )?);
                }
            }

            guards
        } else {
            Vec::new()
        };

        // ── 4. Build command line ────────────────────────────────────────
        let command_line = build_command_line(&config.command, &config.args);
        let mut command_line_wide: Vec<u16> = command_line
            .encode_utf16()
            .chain(std::iter::once(0))
            .collect();

        // S-7 audit fix: When no custom env vars, pass None to inherit parent environment.
        // Previously, empty env_vars produced an empty env block (double-null), which
        // gave the child process NO environment variables at all.
        let env_block = if config.env_vars.is_empty() {
            None
        } else {
            Some(build_environment_block(&config.env_vars))
        };

        // Working directory
        let working_dir: Vec<u16> = config
            .workspace
            .to_string_lossy()
            .encode_utf16()
            .chain(std::iter::once(0))
            .collect();

        // ── 5. Create process (suspended) ────────────────────────────────
        let mut si = STARTUPINFOW::default();
        si.cb = u32::try_from(std::mem::size_of::<STARTUPINFOW>()).unwrap_or(u32::MAX);

        let mut pi = PROCESS_INFORMATION::default();

        let creation_flags = CREATE_SUSPENDED | CREATE_UNICODE_ENVIRONMENT;

        let token_handle = restricted_token
            .as_ref()
            .map_or(HANDLE::default(), |t| t.handle());

        // SAFETY: All strings are valid null-terminated UTF-16.
        // token_handle is either a valid restricted token or HANDLE::default().
        // CREATE_SUSPENDED ensures the process doesn't run until ResumeThread.
        // pi receives the process and thread handles we must close.
        // S-7: env_ptr is None (inherit parent env) or Some(block pointer).
        let env_ptr = env_block
            .as_ref()
            .map(|b| b.as_ptr().cast::<std::ffi::c_void>());
        unsafe {
            CreateProcessAsUserW(
                token_handle,
                None, // application name (use command line instead)
                Some(command_line_wide.as_mut_ptr()),
                None,  // process security attributes
                None,  // thread security attributes
                false, // don't inherit handles
                creation_flags,
                env_ptr,
                windows::core::PCWSTR(working_dir.as_ptr()),
                &si,
                &mut pi,
            )
            .map_err(|e| {
                if e.code().0 as u32 == 2 {
                    // ERROR_FILE_NOT_FOUND
                    SandboxError::CommandNotFound {
                        command: config.command.clone(),
                    }
                } else {
                    SandboxError::Win32 {
                        operation: format!("CreateProcessAsUserW({})", config.command),
                        error_code: e.code().0 as u32,
                    }
                }
            })?;
        }

        // S-1 audit fix: Wrap both handles in RAII guards immediately.
        // This prevents handle leaks on any error path below.
        let _thread_guard = HandleGuard(pi.hThread);
        let process_guard = HandleGuard(pi.hProcess);

        debug!(
            pid = pi.dwProcessId,
            "sandboxed process created (suspended)"
        );

        // ── 6. Assign to Job Object (before resuming) ────────────────────
        job_guard.assign_process(process_guard.0)?;

        // ── 7. Resume the process ────────────────────────────────────────
        // SAFETY: pi.hThread is a valid thread handle from CreateProcessAsUserW.
        // ResumeThread decrements the suspend count; the thread starts running.
        unsafe {
            ResumeThread(pi.hThread);
        }

        debug!(pid = pi.dwProcessId, "sandboxed process resumed");

        // ── 8. Timeout + wait ────────────────────────────────────────────
        let timeout_ms = config
            .resource_limits
            .timeout_secs
            .map(|s| s.saturating_mul(1000))
            .unwrap_or(u64::MAX);

        let done = Arc::new(AtomicBool::new(false));
        let timed_out = Arc::new(AtomicBool::new(false));

        // S-2 audit fix: Copy the raw handle value for the timeout thread.
        // The HandleGuard on the main thread owns the handle and will close it
        // only after joining the timeout thread, preventing use-after-close.
        let timeout_handle = config.resource_limits.timeout_secs.map(|secs| {
            let done = done.clone();
            let timed_out = timed_out.clone();
            let proc_handle = process_guard.0;
            std::thread::spawn(move || {
                let deadline = Instant::now() + Duration::from_secs(secs);
                while Instant::now() < deadline {
                    if done.load(Ordering::Relaxed) {
                        return;
                    }
                    std::thread::sleep(Duration::from_millis(50));
                }
                if !done.load(Ordering::SeqCst) {
                    warn!(timeout_secs = secs, "killing timed out process");
                    timed_out.store(true, Ordering::SeqCst);
                    // SAFETY: proc_handle is a valid process handle.
                    // TerminateProcess with exit code 1 is safe for our child.
                    unsafe {
                        let _ = TerminateProcess(proc_handle, 1);
                    }
                }
            })
        });

        // Wait for process completion
        // SAFETY: process_guard.0 is a valid handle from CreateProcessAsUserW.
        // WaitForSingleObject blocks until the process exits or timeout.
        let wait_result = unsafe {
            WaitForSingleObject(
                process_guard.0,
                u32::try_from(timeout_ms).unwrap_or(u32::MAX),
            )
        };

        done.store(true, Ordering::SeqCst);
        if let Some(handle) = timeout_handle {
            let _ = handle.join();
        }

        let duration_ms = u64::try_from(start.elapsed().as_millis()).unwrap_or(u64::MAX);

        // Check for timeout
        // S-3 audit fix: No manual CloseHandle — process_guard Drop handles it.
        if timed_out.load(Ordering::SeqCst) {
            let timeout_secs = config.resource_limits.timeout_secs.unwrap_or(0);
            info!(timeout_secs, duration_ms, "process timed out");
            return Err(SandboxError::Timeout { timeout_secs });
        }

        // ── 9. Get exit code ─────────────────────────────────────────────
        let mut exit_code: u32 = 0;
        // SAFETY: process_guard.0 is valid and the process has exited
        // (WaitForSingleObject returned). exit_code receives the value.
        unsafe {
            GetExitCodeProcess(process_guard.0, &mut exit_code).map_err(|e| {
                SandboxError::Win32 {
                    operation: "GetExitCodeProcess".into(),
                    error_code: e.code().0 as u32,
                }
            })?;
        }
        // S-3 audit fix: No manual CloseHandle — process_guard Drop handles it.

        info!(exit_code, duration_ms, "sandboxed process completed");

        // Note: On Windows, we don't capture stdout/stderr through pipes in this
        // implementation because CreateProcessAsUserW doesn't use Rust's Command.
        // For full stdout/stderr capture, we'd need to create pipes manually and
        // pass them via STARTUPINFOW.hStdOutput/hStdError.
        // TODO: Implement pipe-based stdout/stderr capture in a follow-up.
        Ok(SandboxOutput {
            stdout: String::new(),
            stderr: String::new(),
            exit_code: exit_code as i32,
            error: None,
            duration_ms,
            sandbox_backend: self.name().into(),
        })
    }
}

/// Build a Windows command line string from command and arguments.
///
/// Windows command lines require proper quoting for arguments with spaces.
fn build_command_line(command: &str, args: &[String]) -> String {
    let mut cmd = quote_arg(command);
    for arg in args {
        cmd.push(' ');
        cmd.push_str(&quote_arg(arg));
    }
    cmd
}

/// Quote a single command-line argument for Windows.
///
/// If the argument contains spaces, quotes, or is empty, wraps it in double quotes
/// and escapes internal backslashes and quotes per Windows conventions.
fn quote_arg(arg: &str) -> String {
    if arg.is_empty() {
        return "\"\"".into();
    }

    if !arg.contains(' ') && !arg.contains('"') && !arg.contains('\t') {
        return arg.into();
    }

    let mut quoted = String::with_capacity(arg.len() + 2);
    quoted.push('"');

    let mut backslash_count = 0u32;
    for c in arg.chars() {
        match c {
            '\\' => backslash_count += 1,
            '"' => {
                // Double the backslashes before a quote
                for _ in 0..backslash_count {
                    quoted.push('\\');
                }
                backslash_count = 0;
                quoted.push('\\');
                quoted.push('"');
            }
            _ => {
                backslash_count = 0;
                quoted.push(c);
            }
        }
    }

    // Double backslashes at the end (before closing quote)
    for _ in 0..backslash_count {
        quoted.push('\\');
    }
    quoted.push('"');
    quoted
}

/// Build a Windows environment block (null-separated, double-null terminated).
///
/// Caller should pass `None` to `CreateProcessAsUserW` for empty env_vars
/// (to inherit parent environment). This function is only called with non-empty maps.
fn build_environment_block(env_vars: &std::collections::HashMap<String, String>) -> Vec<u16> {
    let mut block = Vec::new();

    for (key, value) in env_vars {
        let entry = format!("{key}={value}");
        block.extend(entry.encode_utf16());
        block.push(0);
    }
    block.push(0); // Double null terminator

    block
}

/// Token user SID extracted from a token.
struct TokenUserSid {
    sid: windows::Win32::Foundation::PSID,
    _buffer: Vec<u8>,
}

/// Get the user SID from a token handle.
fn get_token_user_sid(token: HANDLE) -> Result<TokenUserSid, SandboxError> {
    use windows::Win32::Security::{GetTokenInformation, TOKEN_USER, TokenUser};

    let mut size = 0u32;
    // SAFETY: First call with null buffer to get required size.
    unsafe {
        let _ = GetTokenInformation(token, TokenUser, None, 0, &mut size);
    }

    if size == 0 {
        return Err(SandboxError::Win32 {
            operation: "GetTokenInformation(TokenUser) size query".into(),
            error_code: 0,
        });
    }

    let mut buffer = vec![0u8; size as usize];

    // SAFETY: buffer is properly sized. token is a valid token handle.
    unsafe {
        GetTokenInformation(
            token,
            TokenUser,
            Some(buffer.as_mut_ptr().cast()),
            size,
            &mut size,
        )
        .map_err(|e| SandboxError::Win32 {
            operation: "GetTokenInformation(TokenUser)".into(),
            error_code: e.code().0 as u32,
        })?;
    }

    // SAFETY: GetTokenInformation succeeded and buffer contains TOKEN_USER.
    let user = unsafe { &*(buffer.as_ptr().cast::<TOKEN_USER>()) };

    Ok(TokenUserSid {
        sid: user.User.Sid,
        _buffer: buffer, // Keep buffer alive — SID points into it
    })
}

/// RAII guard that closes a HANDLE on drop.
struct HandleGuard(HANDLE);

impl Drop for HandleGuard {
    fn drop(&mut self) {
        if !self.0.is_null() {
            // SAFETY: Handle is valid — from CreateProcessAsUserW.
            unsafe {
                let _ = CloseHandle(self.0);
            }
        }
    }
}

#[cfg(test)]
#[allow(clippy::unwrap_used)]
mod tests {
    use super::*;

    #[test]
    fn test_quote_arg_simple() {
        assert_eq!(quote_arg("hello"), "hello");
    }

    #[test]
    fn test_quote_arg_with_spaces() {
        assert_eq!(quote_arg("hello world"), "\"hello world\"");
    }

    #[test]
    fn test_quote_arg_empty() {
        assert_eq!(quote_arg(""), "\"\"");
    }

    #[test]
    fn test_quote_arg_with_quotes() {
        assert_eq!(quote_arg("say \"hi\""), "\"say \\\"hi\\\"\"");
    }

    #[test]
    fn test_build_command_line() {
        let cmd = build_command_line("cmd.exe", &["/C".into(), "echo hello".into()]);
        assert_eq!(cmd, "cmd.exe /C \"echo hello\"");
    }

    #[test]
    fn test_build_environment_block_with_vars() {
        let mut vars = std::collections::HashMap::new();
        vars.insert("FOO".into(), "bar".into());
        let block = build_environment_block(&vars);
        // Should contain "FOO=bar\0\0"
        let expected: Vec<u16> = "FOO=bar"
            .encode_utf16()
            .chain(std::iter::once(0)) // null after entry
            .chain(std::iter::once(0)) // double null terminator
            .collect();
        assert_eq!(block, expected);
    }
}
