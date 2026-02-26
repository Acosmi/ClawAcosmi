// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! `LocalFs` — concrete `FileSystem` implementation backed by local disk
//! via `tokio::fs`.
//!
//! URI convention: `viking://resources/foo` maps to `{root}/resources/foo`.
//! A plain path (no `viking://` prefix) is treated relative to `root`.

use std::path::{Path, PathBuf};

use async_trait::async_trait;
use tokio::fs;
use tokio::io::AsyncWriteExt;

use openviking_session::traits::{BoxError, FileSystem, FsEntry, FsStat, GrepMatch};

/// A `FileSystem` implementation that maps Viking URIs to real paths under a
/// root directory and delegates all I/O to `tokio::fs`.
pub struct LocalFs {
    /// Root directory for all file operations.
    root: PathBuf,
}

impl LocalFs {
    /// Create a new `LocalFs` rooted at the given directory.
    ///
    /// The directory is **not** created automatically; callers should ensure
    /// it exists before issuing file operations.
    pub fn new(root: impl Into<PathBuf>) -> Self {
        Self { root: root.into() }
    }

    /// Resolve a Viking URI to an absolute path under `self.root`.
    ///
    /// Strips the `viking://` prefix if present, then joins the remainder
    /// onto `self.root`.
    fn resolve(&self, uri: &str) -> PathBuf {
        let stripped = uri
            .strip_prefix("viking://")
            .unwrap_or(uri)
            .trim_start_matches('/');
        self.root.join(stripped)
    }

    /// Ensure that the parent directory of `path` exists.
    async fn ensure_parent(path: &Path) -> Result<(), BoxError> {
        if let Some(parent) = path.parent() {
            if !parent.exists() {
                fs::create_dir_all(parent).await?;
            }
        }
        Ok(())
    }
}

#[async_trait]
impl FileSystem for LocalFs {
    // -------------------------------------------------------------------
    // read / write
    // -------------------------------------------------------------------

    async fn read(&self, uri: &str) -> Result<String, BoxError> {
        let path = self.resolve(uri);
        let content = fs::read_to_string(&path).await.map_err(|e| {
            let msg = format!("read {}: {e}", path.display());
            Box::<dyn std::error::Error + Send + Sync>::from(msg)
        })?;
        Ok(content)
    }

    async fn read_bytes(&self, uri: &str) -> Result<Vec<u8>, BoxError> {
        let path = self.resolve(uri);
        let bytes = fs::read(&path).await.map_err(|e| {
            let msg = format!("read_bytes {}: {e}", path.display());
            Box::<dyn std::error::Error + Send + Sync>::from(msg)
        })?;
        Ok(bytes)
    }

    async fn write(&self, uri: &str, content: &str) -> Result<(), BoxError> {
        let path = self.resolve(uri);
        Self::ensure_parent(&path).await?;
        fs::write(&path, content).await.map_err(|e| {
            let msg = format!("write {}: {e}", path.display());
            Box::<dyn std::error::Error + Send + Sync>::from(msg)
        })?;
        Ok(())
    }

    async fn write_bytes(&self, uri: &str, content: &[u8]) -> Result<(), BoxError> {
        let path = self.resolve(uri);
        Self::ensure_parent(&path).await?;
        fs::write(&path, content).await.map_err(|e| {
            let msg = format!("write_bytes {}: {e}", path.display());
            Box::<dyn std::error::Error + Send + Sync>::from(msg)
        })?;
        Ok(())
    }

    // -------------------------------------------------------------------
    // mkdir
    // -------------------------------------------------------------------

    async fn mkdir(&self, uri: &str) -> Result<(), BoxError> {
        let path = self.resolve(uri);
        fs::create_dir_all(&path).await.map_err(|e| {
            let msg = format!("mkdir {}: {e}", path.display());
            Box::<dyn std::error::Error + Send + Sync>::from(msg)
        })?;
        Ok(())
    }

    // -------------------------------------------------------------------
    // ls
    // -------------------------------------------------------------------

    async fn ls(&self, uri: &str) -> Result<Vec<FsEntry>, BoxError> {
        let path = self.resolve(uri);
        let mut entries = Vec::new();
        let mut dir = fs::read_dir(&path).await.map_err(|e| {
            let msg = format!("ls {}: {e}", path.display());
            Box::<dyn std::error::Error + Send + Sync>::from(msg)
        })?;

        while let Some(entry) = dir.next_entry().await? {
            let meta = entry.metadata().await?;
            entries.push(FsEntry {
                name: entry.file_name().to_string_lossy().into_owned(),
                is_dir: meta.is_dir(),
                size: meta.len(),
            });
        }

        Ok(entries)
    }

    // -------------------------------------------------------------------
    // rm — auto-detect file vs. directory
    // -------------------------------------------------------------------

    async fn rm(&self, uri: &str) -> Result<(), BoxError> {
        let path = self.resolve(uri);
        let meta = fs::metadata(&path).await.map_err(|e| {
            let msg = format!("rm stat {}: {e}", path.display());
            Box::<dyn std::error::Error + Send + Sync>::from(msg)
        })?;

        if meta.is_dir() {
            fs::remove_dir_all(&path).await.map_err(|e| {
                let msg = format!("rm dir {}: {e}", path.display());
                Box::<dyn std::error::Error + Send + Sync>::from(msg)
            })?;
        } else {
            fs::remove_file(&path).await.map_err(|e| {
                let msg = format!("rm file {}: {e}", path.display());
                Box::<dyn std::error::Error + Send + Sync>::from(msg)
            })?;
        }
        Ok(())
    }

    // -------------------------------------------------------------------
    // mv — rename with cross-device fallback (copy + delete)
    // -------------------------------------------------------------------

    async fn mv(&self, from_uri: &str, to_uri: &str) -> Result<(), BoxError> {
        let from = self.resolve(from_uri);
        let to = self.resolve(to_uri);

        Self::ensure_parent(&to).await?;

        // Try atomic rename first.
        match fs::rename(&from, &to).await {
            Ok(()) => Ok(()),
            Err(e) => {
                // On cross-device links, rename returns EXDEV (errno 18 on
                // macOS/Linux). Fall back to copy + delete.
                if e.raw_os_error() == Some(libc_exdev()) {
                    let meta = fs::metadata(&from).await?;
                    if meta.is_dir() {
                        copy_dir_recursive(&from, &to).await?;
                        fs::remove_dir_all(&from).await?;
                    } else {
                        fs::copy(&from, &to).await?;
                        fs::remove_file(&from).await?;
                    }
                    Ok(())
                } else {
                    let msg = format!("mv {} → {}: {e}", from.display(), to.display());
                    Err(Box::<dyn std::error::Error + Send + Sync>::from(msg))
                }
            }
        }
    }

    // -------------------------------------------------------------------
    // stat
    // -------------------------------------------------------------------

    async fn stat(&self, uri: &str) -> Result<FsStat, BoxError> {
        let path = self.resolve(uri);
        let meta = fs::metadata(&path).await.map_err(|e| {
            let msg = format!("stat {}: {e}", path.display());
            Box::<dyn std::error::Error + Send + Sync>::from(msg)
        })?;

        let mod_time = meta
            .modified()
            .map(|t| {
                let dt: chrono::DateTime<chrono::Utc> = t.into();
                dt.to_rfc3339()
            })
            .unwrap_or_default();

        let name = path
            .file_name()
            .map(|n| n.to_string_lossy().into_owned())
            .unwrap_or_default();

        Ok(FsStat {
            name,
            size: meta.len(),
            is_dir: meta.is_dir(),
            mod_time,
        })
    }

    // -------------------------------------------------------------------
    // grep — line-by-line substring match (no regex crate)
    // -------------------------------------------------------------------

    async fn grep(
        &self,
        uri: &str,
        pattern: &str,
        recursive: bool,
        case_insensitive: bool,
    ) -> Result<Vec<GrepMatch>, BoxError> {
        let path = self.resolve(uri);
        let meta = fs::metadata(&path).await.map_err(|e| {
            let msg = format!("grep stat {}: {e}", path.display());
            Box::<dyn std::error::Error + Send + Sync>::from(msg)
        })?;

        let mut results = Vec::new();

        if meta.is_file() {
            grep_file(&path, uri, pattern, case_insensitive, &mut results).await?;
        } else if meta.is_dir() && recursive {
            grep_dir_recursive(&path, uri, pattern, case_insensitive, &mut results).await?;
        }

        Ok(results)
    }

    // -------------------------------------------------------------------
    // exists
    // -------------------------------------------------------------------

    async fn exists(&self, uri: &str) -> Result<bool, BoxError> {
        let path = self.resolve(uri);
        Ok(fs::try_exists(&path).await.unwrap_or(false))
    }

    // -------------------------------------------------------------------
    // append
    // -------------------------------------------------------------------

    async fn append(&self, uri: &str, content: &str) -> Result<(), BoxError> {
        let path = self.resolve(uri);
        Self::ensure_parent(&path).await?;

        let mut file = tokio::fs::OpenOptions::new()
            .create(true)
            .append(true)
            .open(&path)
            .await
            .map_err(|e| {
                let msg = format!("append {}: {e}", path.display());
                Box::<dyn std::error::Error + Send + Sync>::from(msg)
            })?;

        file.write_all(content.as_bytes()).await.map_err(|e| {
            let msg = format!("append write {}: {e}", path.display());
            Box::<dyn std::error::Error + Send + Sync>::from(msg)
        })?;

        Ok(())
    }

    // -------------------------------------------------------------------
    // link — symbolic link (macOS / Linux)
    // -------------------------------------------------------------------

    async fn link(&self, source_uri: &str, target_uri: &str) -> Result<(), BoxError> {
        let source = self.resolve(source_uri);
        let target = self.resolve(target_uri);

        Self::ensure_parent(&target).await?;

        #[cfg(unix)]
        {
            tokio::fs::symlink(&source, &target).await.map_err(|e| {
                let msg = format!(
                    "link {} → {}: {e}",
                    source.display(),
                    target.display()
                );
                Box::<dyn std::error::Error + Send + Sync>::from(msg)
            })?;
        }

        #[cfg(not(unix))]
        {
            return Err("symlink is only supported on Unix-like systems".into());
        }

        Ok(())
    }
}

// ===========================================================================
// Helpers
// ===========================================================================

/// Cross-device move errno.
#[cfg(unix)]
fn libc_exdev() -> i32 {
    18 // EXDEV on macOS and Linux
}

/// Cross-device move errno (non-Unix stub — always returns -1).
#[cfg(not(unix))]
fn libc_exdev() -> i32 {
    -1 // Windows: rename across drives returns a different error code
}

/// Recursively copy a directory.
async fn copy_dir_recursive(src: &Path, dst: &Path) -> Result<(), BoxError> {
    fs::create_dir_all(dst).await?;
    let mut dir = fs::read_dir(src).await?;
    while let Some(entry) = dir.next_entry().await? {
        let src_child = entry.path();
        let dst_child = dst.join(entry.file_name());
        if entry.metadata().await?.is_dir() {
            Box::pin(copy_dir_recursive(&src_child, &dst_child)).await?;
        } else {
            fs::copy(&src_child, &dst_child).await?;
        }
    }
    Ok(())
}

/// Grep a single file for `pattern`, appending matches to `out`.
async fn grep_file(
    path: &Path,
    uri: &str,
    pattern: &str,
    case_insensitive: bool,
    out: &mut Vec<GrepMatch>,
) -> Result<(), BoxError> {
    let content = match fs::read_to_string(path).await {
        Ok(c) => c,
        Err(_) => return Ok(()), // skip binary / unreadable files
    };

    let pat_lower = pattern.to_lowercase();

    for (idx, line) in content.lines().enumerate() {
        let matched = if case_insensitive {
            line.to_lowercase().contains(&pat_lower)
        } else {
            line.contains(pattern)
        };

        if matched {
            out.push(GrepMatch {
                uri: uri.to_string(),
                line: (idx + 1) as u64,
                content: line.to_string(),
            });
        }
    }
    Ok(())
}

/// Recursively grep a directory.
async fn grep_dir_recursive(
    dir_path: &Path,
    base_uri: &str,
    pattern: &str,
    case_insensitive: bool,
    out: &mut Vec<GrepMatch>,
) -> Result<(), BoxError> {
    let mut dir = fs::read_dir(dir_path).await?;
    while let Some(entry) = dir.next_entry().await? {
        let child_path = entry.path();
        let child_name = entry.file_name().to_string_lossy().into_owned();
        let child_uri = format!("{}/{}", base_uri.trim_end_matches('/'), child_name);

        let meta = entry.metadata().await?;
        if meta.is_file() {
            grep_file(&child_path, &child_uri, pattern, case_insensitive, out).await?;
        } else if meta.is_dir() {
            Box::pin(grep_dir_recursive(
                &child_path,
                &child_uri,
                pattern,
                case_insensitive,
                out,
            ))
            .await?;
        }
    }
    Ok(())
}
