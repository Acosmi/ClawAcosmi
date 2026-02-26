//! Atomic file writing via tempfile + rename.
//!
//! Uses [`tempfile::NamedTempFile`] to write to a temporary file in the same
//! directory as the target, then atomically renames it. This prevents partial
//! writes from corrupting files on crash/kill.
//!
//! Reference: VS Code and Claude Code both use write-temp-then-rename.

use std::io::Write;
use std::path::Path;

use anyhow::{Context, Result};

/// Atomically write `content` to `path`.
///
/// Creates a temporary file in the same directory as `path`, writes `content`
/// to it, then renames (persists) it to `path`. The rename is atomic on most
/// filesystems (ext4, APFS, NTFS), ensuring no partial writes.
///
/// # Errors
///
/// Returns an error if the parent directory doesn't exist, writing fails,
/// or the rename fails (e.g., cross-device).
pub fn atomic_write(path: &Path, content: &str) -> Result<()> {
    let parent = path
        .parent()
        .with_context(|| format!("no parent directory for {}", path.display()))?;

    let mut tmp = tempfile::NamedTempFile::new_in(parent)
        .with_context(|| format!("failed to create temp file in {}", parent.display()))?;

    tmp.write_all(content.as_bytes())
        .with_context(|| format!("failed to write to temp file for {}", path.display()))?;

    tmp.flush()
        .with_context(|| format!("failed to flush temp file for {}", path.display()))?;

    tmp.persist(path)
        .with_context(|| format!("failed to atomically replace {}", path.display()))?;

    Ok(())
}
