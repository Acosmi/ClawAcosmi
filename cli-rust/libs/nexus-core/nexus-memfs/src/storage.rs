//! nexus-memfs — persistence layer for durable VFS storage.
//!
//! Provides `MemoryFSStore`: a thread-safe wrapper around `MemoryFS` instances
//! keyed by `(tenant_id, user_id)`, with JSON-based disk persistence.
//!
//! Each tenant+user pair gets its own independent filesystem that can be
//! flushed to disk and restored on startup.

use crate::vfs::MemoryFS;
use serde_json;
use std::collections::HashMap;
use std::fs;
use std::path::PathBuf;
use std::sync::{Mutex, MutexGuard};

// ---------------------------------------------------------------------------
// MemoryFSStore — global store managing per-(tenant, user) filesystems
// ---------------------------------------------------------------------------

/// Thread-safe store managing multiple `MemoryFS` instances.
///
/// Each `(tenant_id, user_id)` pair maps to an independent virtual filesystem.
/// The store supports JSON-based persistence to a configurable root directory.
pub struct MemoryFSStore {
    /// Root directory for persisted filesystem data.
    root_path: PathBuf,
    /// In-memory registry: key = "tenant_id/user_id", value = MemoryFS.
    filesystems: Mutex<HashMap<String, MemoryFS>>,
}

impl MemoryFSStore {
    /// Create a new store rooted at the given directory.
    ///
    /// The directory will be created if it does not exist.
    pub fn new(root_path: &str) -> Result<Self, String> {
        let path = PathBuf::from(root_path);
        fs::create_dir_all(&path).map_err(|e| format!("create root dir: {}", e))?;

        Ok(Self {
            root_path: path,
            filesystems: Mutex::new(HashMap::new()),
        })
    }

    /// Get or create a MemoryFS for the given tenant+user.
    ///
    /// If a persisted version exists on disk, it will be loaded.
    /// Otherwise, a new scaffolded MemoryFS is created.
    pub fn get_or_create(
        &self,
        tenant_id: &str,
        user_id: &str,
    ) -> Result<MutexGuard<'_, HashMap<String, MemoryFS>>, String> {
        let key = Self::make_key(tenant_id, user_id);
        let mut map = self
            .filesystems
            .lock()
            .map_err(|e| format!("lock poisoned: {}", e))?;

        if !map.contains_key(&key) {
            // Try to load from disk
            let disk_path = self.fs_path(tenant_id, user_id);
            if disk_path.exists() {
                let data =
                    fs::read_to_string(&disk_path).map_err(|e| format!("read fs file: {}", e))?;
                let memfs: MemoryFS =
                    serde_json::from_str(&data).map_err(|e| format!("parse fs: {}", e))?;
                map.insert(key, memfs);
            } else {
                map.insert(key, MemoryFS::new());
            }
        }

        Ok(map)
    }

    /// Execute a closure with the MemoryFS for a given tenant+user.
    ///
    /// This is the primary access pattern — takes a lock, runs the closure,
    /// and releases the lock.
    pub fn with_fs<F, R>(&self, tenant_id: &str, user_id: &str, f: F) -> Result<R, String>
    where
        F: FnOnce(&mut MemoryFS) -> Result<R, String>,
    {
        let key = Self::make_key(tenant_id, user_id);
        let mut map = self.get_or_create(tenant_id, user_id)?;
        let fs = map
            .get_mut(&key)
            .ok_or_else(|| "MemoryFS disappeared after creation".to_string())?;
        f(fs)
    }

    /// Persist a tenant+user filesystem to disk as JSON.
    pub fn flush(&self, tenant_id: &str, user_id: &str) -> Result<(), String> {
        let key = Self::make_key(tenant_id, user_id);
        let map = self
            .filesystems
            .lock()
            .map_err(|e| format!("lock poisoned: {}", e))?;

        if let Some(fs) = map.get(&key) {
            let disk_path = self.fs_path(tenant_id, user_id);
            // Ensure parent directory exists
            if let Some(parent) = disk_path.parent() {
                std::fs::create_dir_all(parent).map_err(|e| format!("create parent dir: {}", e))?;
            }
            let json =
                serde_json::to_string_pretty(fs).map_err(|e| format!("serialize fs: {}", e))?;
            std::fs::write(&disk_path, json).map_err(|e| format!("write fs file: {}", e))?;
            Ok(())
        } else {
            Err(format!("no filesystem for {}", key))
        }
    }

    /// Remove a tenant+user filesystem from memory (does not delete disk file).
    pub fn evict(&self, tenant_id: &str, user_id: &str) -> Result<(), String> {
        let key = Self::make_key(tenant_id, user_id);
        let mut map = self
            .filesystems
            .lock()
            .map_err(|e| format!("lock poisoned: {}", e))?;
        map.remove(&key);
        Ok(())
    }

    // -----------------------------------------------------------------------
    // Helpers
    // -----------------------------------------------------------------------

    fn make_key(tenant_id: &str, user_id: &str) -> String {
        format!("{}/{}", tenant_id, user_id)
    }

    fn fs_path(&self, tenant_id: &str, user_id: &str) -> PathBuf {
        self.root_path
            .join(tenant_id)
            .join(format!("{}.json", user_id))
    }
}

// ---------------------------------------------------------------------------
// Serialization support for MemoryFS
// ---------------------------------------------------------------------------

// MemoryFS needs Serialize/Deserialize for persistence.
// We implement it by delegating to the root DirNode.

impl serde::Serialize for MemoryFS {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        // Access via the public API — MemoryFS exposes root indirectly
        // We need to add a serialization helper
        self.root_ref().serialize(serializer)
    }
}

impl<'de> serde::Deserialize<'de> for MemoryFS {
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        use crate::node::DirNode;
        let root = DirNode::deserialize(deserializer)?;
        Ok(MemoryFS::from_root(root))
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use std::env;
    use std::path::Path;

    fn test_root() -> PathBuf {
        env::temp_dir().join("nexus-memfs-test")
    }

    fn cleanup(path: &Path) {
        let _ = fs::remove_dir_all(path);
    }

    #[test]
    fn test_store_write_and_flush() {
        let root = test_root().join("store_write");
        cleanup(&root);

        let store = MemoryFSStore::new(root.to_str().unwrap()).unwrap();

        // Write a memory
        store
            .with_fs("tenant1", "user1", |fs| {
                fs.write_memory("m1", "facts", "content", "abstract", "overview")
            })
            .unwrap();

        // Flush to disk
        store.flush("tenant1", "user1").unwrap();

        // Verify file exists
        let file_path = root.join("tenant1").join("user1.json");
        assert!(file_path.exists());

        cleanup(&root);
    }

    #[test]
    fn test_store_persist_and_reload() {
        let root = test_root().join("store_reload");
        cleanup(&root);

        // Write and flush
        {
            let store = MemoryFSStore::new(root.to_str().unwrap()).unwrap();
            store
                .with_fs("t1", "u1", |fs| {
                    fs.write_memory("m1", "decisions", "full", "abs", "ovw")
                })
                .unwrap();
            store.flush("t1", "u1").unwrap();
        }

        // Reload from new store instance
        {
            let store = MemoryFSStore::new(root.to_str().unwrap()).unwrap();
            let content = store
                .with_fs("t1", "u1", |fs| {
                    fs.read("permanent/decisions/m1.md", crate::node::Tier::L2)
                })
                .unwrap();
            assert_eq!(content, "full");
        }

        cleanup(&root);
    }

    #[test]
    fn test_store_evict() {
        let root = test_root().join("store_evict");
        cleanup(&root);

        let store = MemoryFSStore::new(root.to_str().unwrap()).unwrap();
        store
            .with_fs("t1", "u1", |fs| {
                fs.write_memory("m1", "facts", "c", "a", "o")
            })
            .unwrap();

        store.evict("t1", "u1").unwrap();

        // After evict, accessing again should create a fresh FS
        let count = store
            .with_fs("t1", "u1", |fs| Ok(fs.memory_count()))
            .unwrap();
        assert_eq!(count, 0); // Fresh FS has no memories

        cleanup(&root);
    }
}
