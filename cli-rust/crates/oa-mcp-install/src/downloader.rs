//! Pre-compiled binary downloader for MCP servers.
//!
//! Downloads release assets from GitHub/GitLab, verifies SHA-256,
//! and registers directly (skipping the build step).
//! Inspired by Homebrew bottles: pre-compiled preferred, source build fallback.

use std::path::{Path, PathBuf};

use sha2::{Digest, Sha256};

use crate::error::{McpInstallError, Result};

/// Download a release asset to `dest_path`.
///
/// Uses reqwest to fetch the binary. Follows redirects (GitHub releases
/// redirect to S3/CDN).
pub async fn download_release_asset(url: &str, dest_path: &Path) -> Result<()> {
    tracing::info!(url = url, dest = %dest_path.display(), "downloading release asset");

    // Ensure parent directory exists
    if let Some(parent) = dest_path.parent() {
        std::fs::create_dir_all(parent)?;
    }

    let client = reqwest::Client::builder()
        .redirect(reqwest::redirect::Policy::limited(10))
        .build()
        .map_err(|e| McpInstallError::DownloadFailed(format!("HTTP client: {e}")))?;

    let response = client
        .get(url)
        .header("User-Agent", "openacosmi-mcp-install/0.1")
        .send()
        .await
        .map_err(|e| McpInstallError::DownloadFailed(format!("request: {e}")))?;

    if !response.status().is_success() {
        return Err(McpInstallError::DownloadFailed(format!(
            "HTTP {}: {}",
            response.status(),
            url
        )));
    }

    let bytes = response
        .bytes()
        .await
        .map_err(|e| McpInstallError::DownloadFailed(format!("body: {e}")))?;

    std::fs::write(dest_path, &bytes)?;

    // Make executable on Unix
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        let perms = std::fs::Permissions::from_mode(0o755);
        std::fs::set_permissions(dest_path, perms)?;
    }

    tracing::info!(
        size = bytes.len(),
        dest = %dest_path.display(),
        "download complete"
    );

    Ok(())
}

/// Compute SHA-256 hash of a file.
pub fn compute_sha256(path: &Path) -> Result<String> {
    let data = std::fs::read(path)?;
    let hash = Sha256::digest(&data);
    Ok(format!("{hash:x}"))
}

/// Verify that a file matches the expected SHA-256 hash.
pub fn verify_sha256(path: &Path, expected: &str) -> Result<()> {
    let actual = compute_sha256(path)?;
    if actual != expected {
        return Err(McpInstallError::ChecksumMismatch {
            expected: expected.into(),
            actual,
        });
    }
    Ok(())
}

/// Determine the destination path for a downloaded binary.
///
/// Uses `~/.openacosmi/mcp-servers/<server_name>/bin/<asset_name>`.
pub fn download_dest_path(managed_dir: &Path, server_name: &str, asset_name: &str) -> PathBuf {
    managed_dir
        .join(server_name)
        .join("bin")
        .join(asset_name)
}

/// Extract asset filename from a URL.
pub fn extract_asset_name(url: &str) -> Option<String> {
    url.rsplit('/').next().map(String::from)
}

// ---------- Tests ----------

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;
    use tempfile::TempDir;

    #[test]
    fn test_compute_sha256() {
        let dir = TempDir::new().unwrap();
        let path = dir.path().join("test.bin");
        fs::write(&path, b"hello world").unwrap();
        let hash = compute_sha256(&path).unwrap();
        // SHA-256 of "hello world" is well-known
        assert_eq!(
            hash,
            "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
        );
    }

    #[test]
    fn test_verify_sha256_match() {
        let dir = TempDir::new().unwrap();
        let path = dir.path().join("test.bin");
        fs::write(&path, b"hello world").unwrap();
        assert!(verify_sha256(
            &path,
            "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
        )
        .is_ok());
    }

    #[test]
    fn test_verify_sha256_mismatch() {
        let dir = TempDir::new().unwrap();
        let path = dir.path().join("test.bin");
        fs::write(&path, b"hello world").unwrap();
        let result = verify_sha256(&path, "0000000000000000");
        assert!(result.is_err());
        let err = result.unwrap_err().to_string();
        assert!(err.contains("checksum mismatch"), "got: {err}");
    }

    #[test]
    fn test_extract_asset_name() {
        assert_eq!(
            extract_asset_name(
                "https://github.com/org/repo/releases/download/v1.0/mcp-server-darwin-arm64"
            ),
            Some("mcp-server-darwin-arm64".into())
        );
    }

    #[test]
    fn test_download_dest_path() {
        let managed = PathBuf::from("/home/user/.openacosmi/mcp-servers");
        let dest = download_dest_path(&managed, "my-server", "my-server-darwin-arm64");
        assert_eq!(
            dest,
            PathBuf::from("/home/user/.openacosmi/mcp-servers/my-server/bin/my-server-darwin-arm64")
        );
    }
}
