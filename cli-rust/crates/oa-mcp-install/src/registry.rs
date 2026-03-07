//! MCP server registry — persistent storage for installed servers.
//!
//! Stores configuration in `~/.openacosmi/mcp-servers/registry.json`
//! with 0600 permissions (P5-5 security requirement).

use std::fs;
use std::path::{Path, PathBuf};

use crate::error::{McpInstallError, Result};
use crate::types::{InstalledMcpServer, McpServerRegistry};

/// Default registry directory relative to home.
const REGISTRY_DIR: &str = ".openacosmi/mcp-servers";
/// Registry filename.
const REGISTRY_FILE: &str = "registry.json";

/// Resolve the default registry file path.
pub fn default_registry_path() -> Result<PathBuf> {
    let home = dirs::home_dir().ok_or_else(|| {
        McpInstallError::RegistryError("cannot determine home directory".into())
    })?;
    Ok(home.join(REGISTRY_DIR).join(REGISTRY_FILE))
}

/// Managed server binaries/clones directory.
pub fn managed_dir() -> Result<PathBuf> {
    let home = dirs::home_dir().ok_or_else(|| {
        McpInstallError::RegistryError("cannot determine home directory".into())
    })?;
    Ok(home.join(REGISTRY_DIR))
}

/// Load the registry from disk. Returns empty registry if file doesn't exist.
pub fn load_registry(path: &Path) -> Result<McpServerRegistry> {
    if !path.exists() {
        return Ok(McpServerRegistry::default());
    }
    let content = fs::read_to_string(path)?;
    let registry: McpServerRegistry = serde_json::from_str(&content).map_err(|e| {
        McpInstallError::RegistryError(format!("parse {}: {e}", path.display()))
    })?;
    Ok(registry)
}

/// Save the registry to disk with 0600 permissions.
pub fn save_registry(path: &Path, registry: &McpServerRegistry) -> Result<()> {
    // Ensure parent directory exists
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)?;
    }

    let content = serde_json::to_string_pretty(registry).map_err(|e| {
        McpInstallError::RegistryError(format!("serialize: {e}"))
    })?;

    fs::write(path, &content)?;

    // Set 0600 permissions (owner read/write only)
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        let perms = fs::Permissions::from_mode(0o600);
        fs::set_permissions(path, perms)?;
    }

    tracing::debug!(path = %path.display(), servers = registry.servers.len(), "registry saved");
    Ok(())
}

/// Add or update a server in the registry.
pub fn upsert_server(
    registry: &mut McpServerRegistry,
    server: InstalledMcpServer,
) -> Result<()> {
    let name = server.name.clone();
    registry.servers.insert(name, server);
    Ok(())
}

/// Remove a server from the registry.
pub fn remove_server(
    registry: &mut McpServerRegistry,
    name: &str,
) -> Result<InstalledMcpServer> {
    registry.servers.remove(name).ok_or_else(|| {
        McpInstallError::ServerNotFound(name.into())
    })
}

/// Get a server by name.
pub fn get_server<'a>(
    registry: &'a McpServerRegistry,
    name: &str,
) -> Option<&'a InstalledMcpServer> {
    registry.servers.get(name)
}

/// List all installed server names.
pub fn list_servers(registry: &McpServerRegistry) -> Vec<&str> {
    let mut names: Vec<&str> = registry.servers.keys().map(String::as_str).collect();
    names.sort_unstable();
    names
}

/// Validate that a binary path is within the managed directory (P5-4).
pub fn validate_binary_path(binary_path: &Path) -> Result<()> {
    let managed = managed_dir()?;
    let canonical_managed = managed.canonicalize().unwrap_or(managed);

    // For new installs the binary may not exist yet — check the parent.
    let check_path = if binary_path.exists() {
        binary_path
            .canonicalize()
            .unwrap_or_else(|_| binary_path.to_path_buf())
    } else {
        binary_path.to_path_buf()
    };

    if !check_path.starts_with(&canonical_managed) {
        return Err(McpInstallError::BinaryPathEscape {
            path: check_path.display().to_string(),
            managed_dir: canonical_managed.display().to_string(),
        });
    }

    Ok(())
}

// ---------- Tests ----------

#[cfg(test)]
mod tests {
    use super::*;
    use crate::types::{ProjectType, TransportMode, UrlKind};
    use std::collections::HashMap;
    use tempfile::TempDir;

    fn make_test_server(name: &str) -> InstalledMcpServer {
        InstalledMcpServer {
            name: name.to_string(),
            source_url: format!("https://github.com/test/{name}"),
            source_kind: UrlKind::GitRepo,
            project_type: ProjectType::Rust,
            transport: TransportMode::Stdio,
            binary_path: PathBuf::from(format!("/tmp/mcp/{name}")),
            command: None,
            args: vec![],
            clone_dir: None,
            env: HashMap::new(),
            pinned_ref: None,
            source_commit: None,
            binary_sha256: None,
            installed_at: "2026-03-08T00:00:00Z".into(),
            updated_at: None,
        }
    }

    #[test]
    fn test_load_missing_registry() {
        let dir = TempDir::new().unwrap();
        let path = dir.path().join("registry.json");
        let reg = load_registry(&path).unwrap();
        assert!(reg.servers.is_empty());
        assert_eq!(reg.schema_version, 1);
    }

    #[test]
    fn test_save_and_load() {
        let dir = TempDir::new().unwrap();
        let path = dir.path().join("registry.json");

        let mut reg = McpServerRegistry::default();
        upsert_server(&mut reg, make_test_server("server-a")).unwrap();
        upsert_server(&mut reg, make_test_server("server-b")).unwrap();
        save_registry(&path, &reg).unwrap();

        let loaded = load_registry(&path).unwrap();
        assert_eq!(loaded.servers.len(), 2);
        assert!(loaded.servers.contains_key("server-a"));
        assert!(loaded.servers.contains_key("server-b"));
    }

    #[test]
    fn test_upsert_overwrites() {
        let mut reg = McpServerRegistry::default();
        let mut s = make_test_server("my-server");
        s.source_url = "https://github.com/test/old".into();
        upsert_server(&mut reg, s).unwrap();

        let mut s2 = make_test_server("my-server");
        s2.source_url = "https://github.com/test/new".into();
        upsert_server(&mut reg, s2).unwrap();

        assert_eq!(reg.servers.len(), 1);
        assert_eq!(
            reg.servers["my-server"].source_url,
            "https://github.com/test/new"
        );
    }

    #[test]
    fn test_remove_existing() {
        let mut reg = McpServerRegistry::default();
        upsert_server(&mut reg, make_test_server("x")).unwrap();
        let removed = remove_server(&mut reg, "x").unwrap();
        assert_eq!(removed.name, "x");
        assert!(reg.servers.is_empty());
    }

    #[test]
    fn test_remove_not_found() {
        let mut reg = McpServerRegistry::default();
        assert!(remove_server(&mut reg, "missing").is_err());
    }

    #[test]
    fn test_list_sorted() {
        let mut reg = McpServerRegistry::default();
        upsert_server(&mut reg, make_test_server("charlie")).unwrap();
        upsert_server(&mut reg, make_test_server("alpha")).unwrap();
        upsert_server(&mut reg, make_test_server("bravo")).unwrap();
        let names = list_servers(&reg);
        assert_eq!(names, vec!["alpha", "bravo", "charlie"]);
    }

    #[cfg(unix)]
    #[test]
    fn test_file_permissions() {
        use std::os::unix::fs::PermissionsExt;
        let dir = TempDir::new().unwrap();
        let path = dir.path().join("registry.json");
        let reg = McpServerRegistry::default();
        save_registry(&path, &reg).unwrap();
        let meta = fs::metadata(&path).unwrap();
        assert_eq!(meta.permissions().mode() & 0o777, 0o600);
    }
}
