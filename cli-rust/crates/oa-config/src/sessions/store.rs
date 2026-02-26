/// Session store persistence.
///
/// Provides loading and saving of the session store (a JSON file mapping
/// session keys to `SessionEntry` records). Supports file-level locking
/// and atomic writes. Currently a stub with basic load/save functionality;
/// the full implementation includes caching, lock timeouts, and migration.
///
/// Source: `src/config/sessions/store.ts`

use std::collections::HashMap;
use std::path::Path;

use anyhow::{Context, Result};

use oa_types::session::SessionEntry;

/// Load the session store from disk.
///
/// Returns a map of session keys to session entries. If the file does not
/// exist or cannot be parsed, returns an empty map.
pub fn load_session_store(store_path: &Path) -> HashMap<String, SessionEntry> {
    let raw = match std::fs::read_to_string(store_path) {
        Ok(r) => r,
        Err(_) => return HashMap::new(),
    };

    match serde_json::from_str::<HashMap<String, SessionEntry>>(&raw) {
        Ok(store) => store,
        Err(_) => HashMap::new(),
    }
}

/// Save the session store to disk.
///
/// Performs an atomic write (write to temp, then rename) with 0o600
/// permissions on Unix.
pub async fn save_session_store(
    store_path: &Path,
    store: &HashMap<String, SessionEntry>,
) -> Result<()> {
    // Ensure parent directory exists
    if let Some(parent) = store_path.parent() {
        tokio::fs::create_dir_all(parent)
            .await
            .with_context(|| {
                format!(
                    "Failed to create session store dir: {}",
                    parent.display()
                )
            })?;
    }

    let json = serde_json::to_string_pretty(store)
        .context("Failed to serialize session store")?;

    // Write to temp file, then atomic rename
    let tmp = store_path.with_extension(format!(
        "{}.{}.tmp",
        std::process::id(),
        uuid::Uuid::new_v4()
    ));

    tokio::fs::write(&tmp, &json)
        .await
        .with_context(|| format!("Failed to write temp session store: {}", tmp.display()))?;

    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        let perms = std::fs::Permissions::from_mode(0o600);
        let _ = tokio::fs::set_permissions(&tmp, perms).await;
    }

    tokio::fs::rename(&tmp, store_path)
        .await
        .with_context(|| {
            format!(
                "Failed to rename session store: {} -> {}",
                tmp.display(),
                store_path.display()
            )
        })?;

    Ok(())
}

/// Read the `updatedAt` timestamp for a specific session.
///
/// Returns `None` if the session key is not found or the store cannot be read.
pub fn read_session_updated_at(store_path: &Path, session_key: &str) -> Option<u64> {
    let store = load_session_store(store_path);
    store.get(session_key).map(|e| e.updated_at)
}

/// Update a session store entry with a mutator function.
///
/// Loads the store, applies the mutator, and saves the result. This is a
/// simplified version; the full implementation uses file locking.
pub async fn update_session_store<F, T>(store_path: &Path, mutator: F) -> Result<T>
where
    F: FnOnce(&mut HashMap<String, SessionEntry>) -> T,
{
    let mut store = load_session_store(store_path);
    let result = mutator(&mut store);
    save_session_store(store_path, &store).await?;
    Ok(result)
}
