// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Unit tests for the openviking-transaction crate.

use std::collections::HashMap;
use std::sync::Mutex;

use async_trait::async_trait;
use openviking_session::traits::{
    BoxError, FileSystem, FsEntry, FsStat, GrepMatch,
};

use crate::path_lock::PathLock;
use crate::transaction_manager::TransactionManager;
use crate::transaction_record::{TransactionRecord, TransactionStatus};

// ---------------------------------------------------------------------------
// Mock FileSystem
// ---------------------------------------------------------------------------

/// In-memory file system for testing.
#[derive(Debug, Clone)]
struct MockFs {
    files: std::sync::Arc<Mutex<HashMap<String, String>>>,
}

impl MockFs {
    fn new() -> Self {
        Self {
            files: std::sync::Arc::new(Mutex::new(HashMap::new())),
        }
    }

    fn with_dirs(dirs: &[&str]) -> Self {
        let fs = Self::new();
        {
            let mut files = fs.files.lock().unwrap();
            for d in dirs {
                // Store directory marker
                files.insert(format!("{d}/__dir__"), String::new());
            }
        }
        fs
    }
}

#[async_trait]
impl FileSystem for MockFs {
    async fn read(&self, uri: &str) -> Result<String, BoxError> {
        let files = self.files.lock().unwrap();
        files
            .get(uri)
            .cloned()
            .ok_or_else(|| format!("not found: {uri}").into())
    }

    async fn read_bytes(&self, uri: &str) -> Result<Vec<u8>, BoxError> {
        let content = self.read(uri).await?;
        Ok(content.into_bytes())
    }

    async fn write(&self, uri: &str, content: &str) -> Result<(), BoxError> {
        self.files
            .lock()
            .unwrap()
            .insert(uri.to_owned(), content.to_owned());
        Ok(())
    }

    async fn write_bytes(&self, uri: &str, content: &[u8]) -> Result<(), BoxError> {
        let s = String::from_utf8_lossy(content).into_owned();
        self.write(uri, &s).await
    }

    async fn mkdir(&self, uri: &str) -> Result<(), BoxError> {
        self.files
            .lock()
            .unwrap()
            .insert(format!("{uri}/__dir__"), String::new());
        Ok(())
    }

    async fn ls(&self, _uri: &str) -> Result<Vec<FsEntry>, BoxError> {
        Ok(vec![])
    }

    async fn rm(&self, uri: &str) -> Result<(), BoxError> {
        self.files.lock().unwrap().remove(uri);
        Ok(())
    }

    async fn mv(&self, from: &str, to: &str) -> Result<(), BoxError> {
        let mut files = self.files.lock().unwrap();
        if let Some(content) = files.remove(from) {
            files.insert(to.to_owned(), content);
        }
        Ok(())
    }

    async fn stat(&self, uri: &str) -> Result<FsStat, BoxError> {
        let files = self.files.lock().unwrap();
        // Check if the URI exists as a file or as a directory marker.
        let dir_marker = format!("{uri}/__dir__");
        if files.contains_key(uri) || files.contains_key(&dir_marker) {
            Ok(FsStat {
                name: uri.rsplit('/').next().unwrap_or(uri).to_owned(),
                size: 0,
                is_dir: files.contains_key(&dir_marker),
                mod_time: "2026-01-01T00:00:00Z".to_owned(),
            })
        } else {
            Err(format!("not found: {uri}").into())
        }
    }

    async fn grep(
        &self,
        _uri: &str,
        _pattern: &str,
        _recursive: bool,
        _case_insensitive: bool,
    ) -> Result<Vec<GrepMatch>, BoxError> {
        Ok(vec![])
    }

    async fn exists(&self, uri: &str) -> Result<bool, BoxError> {
        let files = self.files.lock().unwrap();
        let dir_marker = format!("{uri}/__dir__");
        Ok(files.contains_key(uri) || files.contains_key(&dir_marker))
    }

    async fn append(&self, uri: &str, content: &str) -> Result<(), BoxError> {
        let mut files = self.files.lock().unwrap();
        let entry = files.entry(uri.to_owned()).or_default();
        entry.push_str(content);
        Ok(())
    }

    async fn link(&self, _source: &str, _target: &str) -> Result<(), BoxError> {
        Ok(())
    }
}

// ===========================================================================
// TransactionStatus tests
// ===========================================================================

#[test]
fn transaction_status_display() {
    assert_eq!(TransactionStatus::Init.to_string(), "INIT");
    assert_eq!(TransactionStatus::Acquire.to_string(), "ACQUIRE");
    assert_eq!(TransactionStatus::Exec.to_string(), "EXEC");
    assert_eq!(TransactionStatus::Commit.to_string(), "COMMIT");
    assert_eq!(TransactionStatus::Fail.to_string(), "FAIL");
    assert_eq!(TransactionStatus::Releasing.to_string(), "RELEASING");
    assert_eq!(TransactionStatus::Released.to_string(), "RELEASED");
}

#[test]
fn transaction_status_serde_roundtrip() {
    let status = TransactionStatus::Exec;
    let json = serde_json::to_string(&status).unwrap();
    assert_eq!(json, "\"EXEC\"");
    let restored: TransactionStatus = serde_json::from_str(&json).unwrap();
    assert_eq!(restored, status);
}

// ===========================================================================
// TransactionRecord tests
// ===========================================================================

#[test]
fn transaction_record_basics() {
    let mut tx = TransactionRecord::default();
    assert_eq!(tx.status, TransactionStatus::Init);
    assert!(tx.locks.is_empty());

    tx.add_lock("/a/.path.ovlock");
    tx.add_lock("/b/.path.ovlock");
    assert_eq!(tx.locks.len(), 2);

    // Duplicate add is no-op
    tx.add_lock("/a/.path.ovlock");
    assert_eq!(tx.locks.len(), 2);

    tx.remove_lock("/a/.path.ovlock");
    assert_eq!(tx.locks.len(), 1);
    assert_eq!(tx.locks[0], "/b/.path.ovlock");
}

#[test]
fn transaction_record_serde_roundtrip() {
    let mut tx = TransactionRecord::new(HashMap::new());
    tx.update_status(TransactionStatus::Exec);
    tx.add_lock("/resources/docs/.path.ovlock");

    let json = serde_json::to_string_pretty(&tx).unwrap();
    let restored: TransactionRecord = serde_json::from_str(&json).unwrap();

    assert_eq!(restored.id, tx.id);
    assert_eq!(restored.status, TransactionStatus::Exec);
    assert_eq!(restored.locks, vec!["/resources/docs/.path.ovlock"]);
}

// ===========================================================================
// PathLock tests
// ===========================================================================

#[tokio::test]
async fn path_lock_acquire_normal_success() {
    let fs = MockFs::with_dirs(&["/resources/docs"]);
    let lock = PathLock::new(fs.clone());
    let mut tx = TransactionRecord::default();

    let ok = lock.acquire_normal("/resources/docs", &mut tx).await;
    assert!(ok);
    assert_eq!(tx.locks.len(), 1);
    assert_eq!(tx.locks[0], "/resources/docs/.path.ovlock");

    // Verify lock file content
    let content = fs.read("/resources/docs/.path.ovlock").await.unwrap();
    assert_eq!(content, tx.id);
}

#[tokio::test]
async fn path_lock_acquire_normal_fails_if_locked_by_other() {
    let fs = MockFs::with_dirs(&["/resources/docs"]);
    let lock = PathLock::new(fs.clone());

    // Pre-create lock file from "other-tx"
    fs.write("/resources/docs/.path.ovlock", "other-tx-id")
        .await
        .unwrap();

    let mut tx = TransactionRecord::default();
    let ok = lock.acquire_normal("/resources/docs", &mut tx).await;
    assert!(!ok);
    assert!(tx.locks.is_empty());
}

#[tokio::test]
async fn path_lock_release_lifo() {
    let fs = MockFs::with_dirs(&["/a", "/b", "/c"]);
    let lock = PathLock::new(fs.clone());
    let mut tx = TransactionRecord::default();

    // Acquire three locks
    assert!(lock.acquire_normal("/a", &mut tx).await);
    assert!(lock.acquire_normal("/b", &mut tx).await);
    assert!(lock.acquire_normal("/c", &mut tx).await);
    assert_eq!(tx.locks.len(), 3);

    // Release all
    lock.release(&mut tx).await;
    assert!(tx.locks.is_empty());

    // Lock files should be removed
    assert!(fs.read("/a/.path.ovlock").await.is_err());
    assert!(fs.read("/b/.path.ovlock").await.is_err());
    assert!(fs.read("/c/.path.ovlock").await.is_err());
}

// ===========================================================================
// TransactionManager tests
// ===========================================================================

#[tokio::test]
async fn tx_manager_create_begin_commit() {
    let fs = MockFs::with_dirs(&["/resources/docs"]);
    let mgr = TransactionManager::new(
        fs.clone(),
        std::time::Duration::from_secs(3600),
        8,
    );

    let tx = mgr.create_transaction(HashMap::new()).await;
    assert_eq!(mgr.transaction_count().await, 1);

    assert!(mgr.begin(&tx.id).await);
    let snapshot = mgr.get_transaction(&tx.id).await.unwrap();
    assert_eq!(snapshot.status, TransactionStatus::Acquire);

    assert!(mgr.commit(&tx.id).await);
    assert_eq!(mgr.transaction_count().await, 0);
}

#[tokio::test]
async fn tx_manager_rollback() {
    let fs = MockFs::with_dirs(&["/docs"]);
    let mgr = TransactionManager::new(
        fs.clone(),
        std::time::Duration::from_secs(3600),
        8,
    );

    let tx = mgr.create_transaction(HashMap::new()).await;
    assert!(mgr.begin(&tx.id).await);
    assert!(mgr.rollback(&tx.id).await);
    assert_eq!(mgr.transaction_count().await, 0);
}

#[tokio::test]
async fn tx_manager_acquire_lock_normal() {
    let fs = MockFs::with_dirs(&["/resources/docs"]);
    let mgr = TransactionManager::new(
        fs.clone(),
        std::time::Duration::from_secs(3600),
        8,
    );

    let tx = mgr.create_transaction(HashMap::new()).await;
    let success = mgr.acquire_lock_normal(&tx.id, "/resources/docs").await;
    assert!(success);

    let snapshot = mgr.get_transaction(&tx.id).await.unwrap();
    assert_eq!(snapshot.status, TransactionStatus::Exec);
    assert_eq!(snapshot.locks.len(), 1);

    // Cleanup
    assert!(mgr.commit(&tx.id).await);
}
