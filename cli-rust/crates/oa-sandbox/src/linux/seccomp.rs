//! Seccomp-BPF syscall filtering.
//!
//! Uses the `libseccomp` crate (Rust bindings over the C libseccomp library)
//! to install a BPF filter restricting which syscalls the process can make.
//!
//! # Strategy
//!
//! We use a **default-allow** filter with explicit deny rules for:
//! 1. **Network syscalls** — controlled by `NetworkPolicy` (None/Restricted/Host)
//! 2. **Dangerous syscalls** — always blocked (ptrace, mount, reboot, etc.)
//!
//! A strict whitelist would be more secure but breaks compatibility with
//! arbitrary user programs (Python, Node, shell scripts need many syscalls).
//! The Landlock layer handles filesystem isolation.
//!
//! # Network policy mapping
//!
//! | Policy | socket | connect | Unix domain |
//! |--------|--------|---------|-------------|
//! | None | Block all | Block all | Block all |
//! | Restricted | Allow INET/INET6 | Allow INET/INET6 | Block |
//! | Host | Allow all | Allow all | Allow all |
//!
//! # System requirement
//!
//! Requires `libseccomp-dev` >= 2.5.0 installed on the build system.

use libseccomp::{ScmpAction, ScmpArgCompare, ScmpCompareOp, ScmpFilterContext, ScmpSyscall};
use tracing::{debug, info};

use crate::config::{NetworkPolicy, SandboxConfig};
use crate::error::SandboxError;

/// Dangerous syscalls that are always blocked regardless of security level.
///
/// These can escape sandboxes, modify system state, or elevate privileges.
const DANGEROUS_SYSCALLS: &[&str] = &[
    "ptrace",           // process tracing — sandbox escape vector
    "process_vm_readv", // cross-process memory access
    "process_vm_writev",
    "personality",     // change execution domain — bypass ASLR
    "mount",           // mount filesystems
    "umount2",         // unmount filesystems
    "pivot_root",      // change root filesystem
    "swapon",          // enable swap
    "swapoff",         // disable swap
    "reboot",          // reboot the system
    "sethostname",     // change hostname
    "setdomainname",   // change domain name
    "kexec_load",      // load new kernel
    "kexec_file_load", // load new kernel (file variant)
    "init_module",     // load kernel module
    "finit_module",    // load kernel module (file variant)
    "delete_module",   // unload kernel module
    "acct",            // process accounting
    "settimeofday",    // set system clock
    "clock_settime",   // set clock
    "adjtimex",        // adjust system clock
    "bpf",             // BPF operations — can load arbitrary BPF programs
    "userfaultfd",     // exploit primitive — used in many kernel exploits
    "perf_event_open", // performance monitoring — info leak
    "lookup_dcookie",  // kernel tracing
    "add_key",         // kernel keyring manipulation
    "request_key",     // kernel keyring
    "keyctl",          // kernel keyring control
    "io_setup",        // AIO — rarely needed, potential exploit surface
    "io_destroy",
    "io_submit",
    "io_cancel",
    "io_getevents",
    "move_pages",    // NUMA memory migration — privilege escalation vector
    "mbind",         // NUMA memory policy
    "set_mempolicy", // NUMA memory policy
    "migrate_pages", // NUMA page migration
    "unshare",       // create namespaces — prevent nested sandbox escape
    "setns",         // join namespaces — prevent sandbox escape
    // S-01 audit fix: mount-related syscalls (kernel 5.2+ mount API)
    "open_tree",  // create file handle for mount operations
    "move_mount", // move mount points — container escape vector
    "fsopen",     // open filesystem configuration context
    "fspick",     // pick filesystem for reconfiguration
    "fsconfig",   // configure a filesystem context
    "fsmount",    // create a mount from filesystem context
    // S-01 audit fix: clone3 can create new namespaces
    "clone3", // newer clone() — can create namespaces, prevent nested escape
];

/// Network-related syscalls blocked in `NetworkPolicy::None` mode.
const NETWORK_SYSCALLS: &[&str] = &[
    "socket",
    "socketpair",
    "connect",
    "bind",
    "listen",
    "accept",
    "accept4",
    "sendto",
    "recvfrom",
    "sendmsg",
    "recvmsg",
    "sendmmsg",
    "recvmmsg",
    "shutdown",
    "getsockopt",
    "setsockopt",
    "getsockname",
    "getpeername",
];

/// Apply a seccomp-BPF filter based on the sandbox configuration.
///
/// Must be called after Landlock (seccomp is more restrictive and harder to debug).
/// The filter is irrevocable and inherited by exec'd processes.
pub fn apply_seccomp_filter(config: &SandboxConfig) -> Result<(), SandboxError> {
    let network_policy = config.effective_network_policy();

    // Default action: Allow — we deny specific dangerous operations
    let mut filter = ScmpFilterContext::new_filter(ScmpAction::Allow)
        .map_err(|e| seccomp_err("new_filter", e))?;

    // ── Always-blocked dangerous syscalls ──────────────────────────────────
    for name in DANGEROUS_SYSCALLS {
        if let Ok(syscall) = ScmpSyscall::from_name(name) {
            filter
                .add_rule(ScmpAction::Errno(libc::EPERM), syscall)
                .map_err(|e| seccomp_err(&format!("block {name}"), e))?;
        }
        // If syscall name not recognized (e.g., arch-specific), skip silently
    }

    // ── Network policy ────────────────────────────────────────────────────
    match network_policy {
        NetworkPolicy::None => {
            // Block ALL network syscalls
            for name in NETWORK_SYSCALLS {
                if let Ok(syscall) = ScmpSyscall::from_name(name) {
                    filter
                        .add_rule(ScmpAction::Errno(libc::EPERM), syscall)
                        .map_err(|e| seccomp_err(&format!("block network {name}"), e))?;
                }
            }
            debug!("seccomp: all network syscalls blocked (NetworkPolicy::None)");
        }

        NetworkPolicy::Restricted => {
            // Block Unix domain sockets to prevent proxy bypass.
            // Allow AF_INET (2) and AF_INET6 (10) for TCP/UDP.
            //
            // socket(domain, type, protocol):
            //   Block if domain == AF_UNIX (1)
            //   Block if domain == AF_LOCAL (1, alias for AF_UNIX)
            if let Ok(socket_sc) = ScmpSyscall::from_name("socket") {
                filter
                    .add_rule_conditional(
                        ScmpAction::Errno(libc::EPERM),
                        socket_sc,
                        &[ScmpArgCompare::new(
                            0,
                            ScmpCompareOp::Equal,
                            libc::AF_UNIX as u64,
                        )],
                    )
                    .map_err(|e| seccomp_err("block socket(AF_UNIX)", e))?;
            }

            // socketpair is always AF_UNIX — block entirely
            if let Ok(sc) = ScmpSyscall::from_name("socketpair") {
                filter
                    .add_rule(ScmpAction::Errno(libc::EPERM), sc)
                    .map_err(|e| seccomp_err("block socketpair", e))?;
            }

            // S-02 audit fix: Block AF_NETLINK (can manipulate routing/iptables)
            if let Ok(socket_sc) = ScmpSyscall::from_name("socket") {
                filter
                    .add_rule_conditional(
                        ScmpAction::Errno(libc::EPERM),
                        socket_sc,
                        &[ScmpArgCompare::new(
                            0,
                            ScmpCompareOp::Equal,
                            libc::AF_NETLINK as u64,
                        )],
                    )
                    .map_err(|e| seccomp_err("block socket(AF_NETLINK)", e))?;
            }

            // S-02 audit fix: Block AF_PACKET (raw packet access — can sniff traffic)
            if let Ok(socket_sc) = ScmpSyscall::from_name("socket") {
                filter
                    .add_rule_conditional(
                        ScmpAction::Errno(libc::EPERM),
                        socket_sc,
                        &[ScmpArgCompare::new(
                            0,
                            ScmpCompareOp::Equal,
                            libc::AF_PACKET as u64,
                        )],
                    )
                    .map_err(|e| seccomp_err("block socket(AF_PACKET)", e))?;
            }

            // S-02 audit fix: Block AF_VSOCK (VM socket — potential escape in VM environments)
            // AF_VSOCK = 40 (not always defined in libc crate)
            if let Ok(socket_sc) = ScmpSyscall::from_name("socket") {
                const AF_VSOCK: u64 = 40;
                filter
                    .add_rule_conditional(
                        ScmpAction::Errno(libc::EPERM),
                        socket_sc,
                        &[ScmpArgCompare::new(0, ScmpCompareOp::Equal, AF_VSOCK)],
                    )
                    .map_err(|e| seccomp_err("block socket(AF_VSOCK)", e))?;
            }

            // Block connect to Unix domain sockets.
            // connect(fd, addr, addrlen): addr->sa_family == AF_UNIX
            // Note: libseccomp cannot inspect pointed-to memory, so we rely on
            // blocking socket(AF_UNIX) above to prevent Unix socket creation.
            // If somehow a Unix socket fd exists, connect would still work.
            // Full mitigation requires Landlock ABI 6+ scope or network namespace.

            debug!(
                "seccomp: Unix/NETLINK/PACKET/VSOCK sockets blocked (NetworkPolicy::Restricted)"
            );
        }

        NetworkPolicy::Host => {
            // No network restrictions
            debug!("seccomp: no network restrictions (NetworkPolicy::Host)");
        }
    }

    // ── Load filter ───────────────────────────────────────────────────────
    filter.load().map_err(|e| seccomp_err("load_filter", e))?;

    info!(
        network_policy = ?network_policy,
        dangerous_blocked = DANGEROUS_SYSCALLS.len(),
        "seccomp filter loaded"
    );

    Ok(())
}

/// Convert a libseccomp error to `SandboxError::Seccomp`.
fn seccomp_err(
    operation: &str,
    err: impl std::error::Error + Send + Sync + 'static,
) -> SandboxError {
    SandboxError::Seccomp {
        operation: operation.into(),
        source: std::io::Error::other(err),
    }
}
