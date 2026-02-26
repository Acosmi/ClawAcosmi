---
document_type: Tracking
status: In Progress
created: 2026-02-25
last_updated: 2026-02-25
audit_report: Pending
skill5_verified: true
---

# Windows Sandbox API Verification

Pre-implementation research for OS-native sandbox on Windows.
Covers AppContainer, Job Objects, NTFS ACLs, Restricted Tokens, Rust `windows` crate, and Chromium reference implementation.

## Online Verification Log

### 1. Windows AppContainer Isolation

- **Query**: "Windows AppContainer isolation SID model"
- **Source**: https://learn.microsoft.com/en-us/windows/win32/secauthz/appcontainer-isolation
- **Source**: https://learn.microsoft.com/en-us/windows/win32/secauthz/implementing-an-appcontainer
- **Source**: https://learn.microsoft.com/en-us/windows/win32/secauthz/appcontainer-for-legacy-applications-
- **Key finding**: AppContainer uses a dual-principal SID model (Package SID + Capability SIDs orthogonal to user/group SIDs). Both must grant access for resource access to succeed. Runs at Low Integrity Level. Provides file/registry virtualization, network isolation, device isolation, process isolation, and window isolation. For unpackaged apps, requires manual CreateAppContainerProfile + SECURITY_CAPABILITIES in CreateProcess via PROC_THREAD_ATTRIBUTE_SECURITY_CAPABILITIES.
- **Verified date**: 2026-02-25

### 2. Windows Job Objects

- **Query**: "Job Objects Windows process management JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE"
- **Source**: https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects
- **Source**: https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_basic_limit_information
- **Source**: https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_cpu_rate_control_information
- **Key finding**: Job Objects can limit memory (per-process and per-job), CPU (rate control with hard cap, weight-based, or min/max), process count (ActiveProcessLimit), working set, priority class, and CPU affinity. JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE terminates all associated processes when last job handle closes. Nested jobs supported since Windows 8. Child processes auto-inherit job association by default. CPU rate control uses percentage*100 (e.g., 5000 = 50%).
- **Verified date**: 2026-02-25

### 3. NTFS ACL Modification

- **Query**: "NTFS ACL programmatic modification Win32 API SetNamedSecurityInfo"
- **Source**: https://learn.microsoft.com/en-us/windows/win32/api/aclapi/nf-aclapi-setnamedsecurityinfoa
- **Source**: https://learn.microsoft.com/en-us/windows/win32/secauthz/modifying-the-acls-of-an-object-in-c--
- **Key finding**: Workflow is GetNamedSecurityInfo -> build EXPLICIT_ACCESS -> SetEntriesInAcl -> SetNamedSecurityInfo. Supports automatic propagation of inheritable ACEs to subdirectories. In Rust `windows` crate, SetSecurityInfo and GetSecurityInfo are in Win32::Security::Authorization module.
- **Verified date**: 2026-02-25

### 4. Restricted Tokens

- **Query**: "Windows restricted tokens CreateRestrictedToken"
- **Source**: https://learn.microsoft.com/en-us/windows/win32/secauthz/restricted-tokens
- **Source**: https://learn.microsoft.com/en-us/windows/win32/api/securitybaseapi/nf-securitybaseapi-createrestrictedtoken
- **Key finding**: CreateRestrictedToken can: (1) remove privileges, (2) apply deny-only attribute to SIDs, (3) add restricting SIDs. System performs dual access check: one against enabled SIDs, one against restricting SIDs; access granted only if both pass. Can be used with CreateProcessAsUser without SE_ASSIGNPRIMARYTOKEN_NAME privilege if it's a restricted version of the caller's token. Applications using restricted tokens should run on alternate desktops to prevent SendMessage/PostMessage attacks.
- **Verified date**: 2026-02-25

### 5. Rust `windows` Crate

- **Query**: "windows crate Rust Microsoft Win32 API"
- **Source**: https://crates.io/crates/windows (v0.62.2)
- **Source**: https://microsoft.github.io/windows-docs-rs/doc/windows/Win32/Security/
- **Source**: https://microsoft.github.io/windows-docs-rs/doc/windows/Win32/Security/Isolation/fn.CreateAppContainerProfile.html
- **Source**: https://microsoft.github.io/windows-docs-rs/doc/windows/Win32/Security/Authorization/fn.SetSecurityInfo.html
- **Key finding**: The `windows` crate (v0.62.2, 184M+ downloads) covers all required APIs. Key modules: Win32::Security (CreateRestrictedToken, token manipulation), Win32::Security::Authorization (SetSecurityInfo, GetSecurityInfo, ACL operations), Win32::Security::Isolation (CreateAppContainerProfile, DeleteAppContainerProfile), Win32::System::JobObjects (job object APIs). Feature-gated; enable specific Win32 feature flags in Cargo.toml.
- **Verified date**: 2026-02-25

### 6. Chromium Sandbox on Windows

- **Query**: "chromium sandbox windows design AppContainer restricted token"
- **Source**: https://chromium.googlesource.com/chromium/src/+/HEAD/docs/design/sandbox.md
- **Source**: https://bugzilla.mozilla.org/show_bug.cgi?id=1783669 (Mozilla LPAC reference)
- **Key finding**: Chromium combines four mechanisms: (1) restricted token with near-zero privileges at Untrusted integrity level, (2) Job Object preventing child process creation/clipboard/desktop switching, (3) alternate desktop preventing window message attacks, (4) integrity levels for mandatory access control. Uses broker/target model: browser is broker, renderers are targets. Resources are acquired by broker and handle-duplicated into renderer. LPAC (Less Privileged AppContainer) is being adopted for even stronger isolation but faces compatibility issues with filesystem/registry ACLs.
- **Verified date**: 2026-02-25

### 7. AppContainer Legacy Compatibility Issues

- **Query**: "AppContainer compatibility issues legacy desktop exe"
- **Source**: https://learn.microsoft.com/en-us/windows/win32/secauthz/appcontainer-for-legacy-applications-
- **Source**: https://github.com/electron/electron/issues/14548
- **Source**: https://github.com/chromiumembedded/cef/issues/3791
- **Key finding**: Verified -- AppContainer does have significant compatibility limitations for traditional .exe files. Key issues: (a) file/registry virtualization means apps cannot access arbitrary paths; only explicitly granted or virtualized locations work, (b) shell APIs (e.g., Electron's shell.* APIs) do not function inside AppContainer, (c) most system files have ALL APPLICATION PACKAGES ACE but third-party files/DLLs do not, requiring manual ACL setup, (d) LPAC is even more restrictive -- all filesystem/registry locations need explicit ACLs with ALL RESTRICTED APPLICATION PACKAGES, (e) DLL delay loading can fail if the DLL's location lacks proper ACLs, (f) COM access requires specific capabilities in LPAC. The Electron project found that every API needed auditing for AppContainer compatibility.
- **Verified date**: 2026-02-25

## Task Checklist

- [x] Research AppContainer isolation model
- [x] Research Job Objects for process tree management
- [x] Research NTFS ACL modification APIs
- [x] Research Restricted Tokens
- [x] Research Rust `windows` crate coverage
- [x] Research Chromium sandbox reference implementation
- [x] Verify AppContainer legacy compatibility issues
- [ ] Write verification record (this document)
- [ ] Integrate findings into sandbox design
