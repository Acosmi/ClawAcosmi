//! Git URL parsing for MCP server installation.
//!
//! Supports: GitHub HTTPS, GitLab HTTPS, SSH (git@), Release asset URLs.

use regex::Regex;
use std::sync::LazyLock;

use crate::error::{McpInstallError, Result};
use crate::types::{HostingPlatform, ParsedMcpUrl, UrlKind};

// ---------- Regex patterns ----------

/// GitHub/GitLab HTTPS: `https://github.com/owner/repo[.git][/...]`
static RE_HTTPS: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"^https?://(?P<host>[^/]+)/(?P<owner>[^/]+)/(?P<repo>[^/.]+?)(?:\.git)?(?:/.*)?$")
        .expect("RE_HTTPS compile")
});

/// SSH: `git@host:owner/repo[.git]`
static RE_SSH: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"^git@(?P<host>[^:]+):(?P<owner>[^/]+)/(?P<repo>[^/.]+?)(?:\.git)?$")
        .expect("RE_SSH compile")
});

/// GitHub Release asset: `https://github.com/owner/repo/releases/download/tag/asset`
static RE_GITHUB_RELEASE: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"^https?://github\.com/(?P<owner>[^/]+)/(?P<repo>[^/]+)/releases/download/(?P<tag>[^/]+)/(?P<asset>.+)$")
        .expect("RE_GITHUB_RELEASE compile")
});

/// GitLab Release asset (generic packages): `https://gitlab.com/.../releases/permalink/latest/downloads/asset`
static RE_GITLAB_RELEASE: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"^https?://(?P<host>[^/]*gitlab[^/]*)/(?P<owner>[^/]+)/(?P<repo>[^/]+)/-/releases/(?:permalink/latest|(?P<tag>[^/]+))/downloads/(?P<asset>.+)$")
        .expect("RE_GITLAB_RELEASE compile")
});

// ---------- Public API ----------

/// Parse an MCP server URL into structured components.
///
/// Supports:
/// - GitHub HTTPS: `https://github.com/owner/repo`
/// - GitLab HTTPS: `https://gitlab.com/owner/repo`
/// - SSH: `git@github.com:owner/repo.git`
/// - GitHub Release: `https://github.com/owner/repo/releases/download/v1.0/binary`
/// - GitLab Release: `https://gitlab.com/owner/repo/-/releases/v1.0/downloads/binary`
pub fn parse_mcp_url(input: &str) -> Result<ParsedMcpUrl> {
    let input = input.trim();
    if input.is_empty() {
        return Err(McpInstallError::InvalidUrl("empty URL".into()));
    }

    // 1. Check GitHub Release asset URL first (most specific)
    if let Some(caps) = RE_GITHUB_RELEASE.captures(input) {
        return Ok(ParsedMcpUrl {
            original: input.to_string(),
            git_url: Some(format!(
                "https://github.com/{}/{}.git",
                &caps["owner"], &caps["repo"]
            )),
            owner: Some(caps["owner"].to_string()),
            repo: Some(caps["repo"].to_string()),
            platform: HostingPlatform::GitHub,
            kind: UrlKind::ReleaseAsset,
            release_asset_url: Some(input.to_string()),
            release_tag: Some(caps["tag"].to_string()),
        });
    }

    // 2. Check GitLab Release asset URL
    if let Some(caps) = RE_GITLAB_RELEASE.captures(input) {
        return Ok(ParsedMcpUrl {
            original: input.to_string(),
            git_url: Some(format!(
                "https://{}/{}/{}.git",
                &caps["host"], &caps["owner"], &caps["repo"]
            )),
            owner: Some(caps["owner"].to_string()),
            repo: Some(caps["repo"].to_string()),
            platform: HostingPlatform::GitLab,
            kind: UrlKind::ReleaseAsset,
            release_asset_url: Some(input.to_string()),
            release_tag: caps.name("tag").map(|m| m.as_str().to_string()),
        });
    }

    // 3. SSH URL
    if let Some(caps) = RE_SSH.captures(input) {
        let host = &caps["host"];
        let platform = detect_platform(host);
        return Ok(ParsedMcpUrl {
            original: input.to_string(),
            git_url: Some(input.to_string()),
            owner: Some(caps["owner"].to_string()),
            repo: Some(caps["repo"].to_string()),
            platform,
            kind: UrlKind::Ssh,
            release_asset_url: None,
            release_tag: None,
        });
    }

    // 4. HTTPS git URL
    if let Some(caps) = RE_HTTPS.captures(input) {
        let host = &caps["host"];
        let platform = detect_platform(host);
        let clone_url = format!("https://{}/{}/{}.git", host, &caps["owner"], &caps["repo"]);
        return Ok(ParsedMcpUrl {
            original: input.to_string(),
            git_url: Some(clone_url),
            owner: Some(caps["owner"].to_string()),
            repo: Some(caps["repo"].to_string()),
            platform,
            kind: UrlKind::GitRepo,
            release_asset_url: None,
            release_tag: None,
        });
    }

    Err(McpInstallError::InvalidUrl(format!(
        "cannot parse URL: {input}"
    )))
}

/// Detect hosting platform from hostname.
fn detect_platform(host: &str) -> HostingPlatform {
    let h = host.to_lowercase();
    if h.contains("github") {
        HostingPlatform::GitHub
    } else if h.contains("gitlab") {
        HostingPlatform::GitLab
    } else if h.contains("bitbucket") {
        HostingPlatform::Bitbucket
    } else {
        HostingPlatform::Other
    }
}

// ---------- Tests ----------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_github_https() {
        let result = parse_mcp_url("https://github.com/modelcontextprotocol/servers").unwrap();
        assert_eq!(result.owner.as_deref(), Some("modelcontextprotocol"));
        assert_eq!(result.repo.as_deref(), Some("servers"));
        assert_eq!(result.platform, HostingPlatform::GitHub);
        assert_eq!(result.kind, UrlKind::GitRepo);
        assert!(result.git_url.is_some());
        assert!(result.release_asset_url.is_none());
    }

    #[test]
    fn test_github_https_with_git_suffix() {
        let result =
            parse_mcp_url("https://github.com/modelcontextprotocol/servers.git").unwrap();
        assert_eq!(result.repo.as_deref(), Some("servers"));
        assert_eq!(result.kind, UrlKind::GitRepo);
    }

    #[test]
    fn test_ssh_url() {
        let result = parse_mcp_url("git@github.com:owner/my-mcp-server.git").unwrap();
        assert_eq!(result.owner.as_deref(), Some("owner"));
        assert_eq!(result.repo.as_deref(), Some("my-mcp-server"));
        assert_eq!(result.platform, HostingPlatform::GitHub);
        assert_eq!(result.kind, UrlKind::Ssh);
    }

    #[test]
    fn test_gitlab_https() {
        let result = parse_mcp_url("https://gitlab.com/mygroup/mcp-tool").unwrap();
        assert_eq!(result.platform, HostingPlatform::GitLab);
        assert_eq!(result.kind, UrlKind::GitRepo);
    }

    #[test]
    fn test_github_release_asset() {
        let url = "https://github.com/anthropics/mcp-server/releases/download/v1.2.0/mcp-server-darwin-arm64";
        let result = parse_mcp_url(url).unwrap();
        assert_eq!(result.owner.as_deref(), Some("anthropics"));
        assert_eq!(result.repo.as_deref(), Some("mcp-server"));
        assert_eq!(result.platform, HostingPlatform::GitHub);
        assert_eq!(result.kind, UrlKind::ReleaseAsset);
        assert_eq!(result.release_tag.as_deref(), Some("v1.2.0"));
        assert_eq!(result.release_asset_url.as_deref(), Some(url));
        // git_url is also populated for potential source fallback
        assert!(result.git_url.is_some());
    }

    #[test]
    fn test_gitlab_release_asset() {
        let url = "https://gitlab.com/mygroup/mcp-tool/-/releases/v2.0/downloads/mcp-tool-linux";
        let result = parse_mcp_url(url).unwrap();
        assert_eq!(result.platform, HostingPlatform::GitLab);
        assert_eq!(result.kind, UrlKind::ReleaseAsset);
        assert_eq!(result.release_tag.as_deref(), Some("v2.0"));
    }

    #[test]
    fn test_empty_url() {
        assert!(parse_mcp_url("").is_err());
    }

    #[test]
    fn test_invalid_url() {
        assert!(parse_mcp_url("not-a-url").is_err());
    }

    #[test]
    fn test_self_hosted_gitlab() {
        let result = parse_mcp_url("https://gitlab.mycompany.com/team/mcp-server").unwrap();
        assert_eq!(result.platform, HostingPlatform::GitLab);
        assert_eq!(result.kind, UrlKind::GitRepo);
    }
}
