// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Path-based locking for transactional file operations.
//!
//! Ported from `openviking/storage/transaction/path_lock.py`.
//!
//! Lock protocol: a lock file `{path}/.path.ovlock` whose content equals
//! the owning transaction ID indicates that `path` is locked.

use openviking_session::traits::FileSystem;

use crate::transaction_record::TransactionRecord;

/// Name of the lock sentinel file placed inside a locked directory.
pub const LOCK_FILE_NAME: &str = ".path.ovlock";

/// Path-level lock manager backed by an abstract [`FileSystem`].
///
/// All file-system calls are async and delegated to the injected `FS`.
pub struct PathLock<FS: FileSystem> {
    fs: FS,
}

impl<FS: FileSystem> PathLock<FS> {
    /// Create a new `PathLock` with the given file-system backend.
    pub fn new(fs: FS) -> Self {
        Self { fs }
    }

    // -----------------------------------------------------------------------
    // Internal helpers
    // -----------------------------------------------------------------------

    fn lock_path(path: &str) -> String {
        let path = path.trim_end_matches('/');
        format!("{path}/{LOCK_FILE_NAME}")
    }

    fn parent_path(path: &str) -> Option<String> {
        let path = path.trim_end_matches('/');
        let idx = path.rfind('/')?;
        if idx == 0 {
            return None;
        }
        Some(path[..idx].to_owned())
    }

    /// Check if `lock_path` is locked by a *different* transaction.
    async fn is_locked_by_other(&self, lock_path: &str, tx_id: &str) -> bool {
        match self.fs.read(lock_path).await {
            Ok(content) => {
                let owner = content.trim();
                !owner.is_empty() && owner != tx_id
            }
            Err(_) => false, // file doesn't exist → not locked
        }
    }

    async fn create_lock_file(&self, lock_path: &str, tx_id: &str) -> Result<(), String> {
        self.fs
            .write(lock_path, tx_id)
            .await
            .map_err(|e| format!("Failed to create lock file {lock_path}: {e}"))
    }

    async fn verify_ownership(&self, lock_path: &str, tx_id: &str) -> bool {
        match self.fs.read(lock_path).await {
            Ok(content) => content.trim() == tx_id,
            Err(_) => false,
        }
    }

    async fn remove_lock_file(&self, lock_path: &str) {
        let _ = self.fs.rm(lock_path).await;
    }

    // -----------------------------------------------------------------------
    // Public API
    // -----------------------------------------------------------------------

    /// Acquire a lock for normal (read/write) operations.
    ///
    /// Steps:
    /// 1. Verify directory existence via `stat`.
    /// 2. Check target is not locked by another transaction.
    /// 3. Check parent is not locked by another transaction.
    /// 4. Write lock file.
    /// 5. Re-check parent (race-condition guard).
    /// 6. Verify lock ownership (content matches our tx ID).
    pub async fn acquire_normal(
        &self,
        path: &str,
        tx: &mut TransactionRecord,
    ) -> bool {
        let tx_id = tx.id.clone();
        let lp = Self::lock_path(path);
        let parent = Self::parent_path(path);

        // Step 1: directory must exist
        if self.fs.stat(path).await.is_err() {
            log::warn!("Directory does not exist: {path}");
            return false;
        }

        // Step 2: target not locked by another tx
        if self.is_locked_by_other(&lp, &tx_id).await {
            log::warn!("Path already locked by another transaction: {path}");
            return false;
        }

        // Step 3: parent not locked
        if let Some(ref pp) = parent {
            let parent_lp = Self::lock_path(pp);
            if self.is_locked_by_other(&parent_lp, &tx_id).await {
                log::warn!("Parent path locked by another transaction: {pp}");
                return false;
            }
        }

        // Step 4: create lock
        if let Err(e) = self.create_lock_file(&lp, &tx_id).await {
            log::error!("{e}");
            return false;
        }

        // Step 5: re-check parent (guard against race)
        if let Some(ref pp) = parent {
            let parent_lp = Self::lock_path(pp);
            if self.is_locked_by_other(&parent_lp, &tx_id).await {
                log::warn!("Parent path locked after lock creation: {pp}");
                self.remove_lock_file(&lp).await;
                return false;
            }
        }

        // Step 6: verify ownership
        if !self.verify_ownership(&lp, &tx_id).await {
            log::error!("Lock ownership verification failed: {path}");
            return false;
        }

        tx.add_lock(&lp);
        log::debug!("Lock acquired: {lp}");
        true
    }

    /// Recursively collect all subdirectory paths under `path`.
    ///
    /// Uses [`FileSystem::ls`] to list entries and recursively descends
    /// into directories, mirroring Python `PathLock._collect_subdirectories`.
    pub async fn collect_subdirectories(&self, path: &str) -> Vec<String> {
        let mut subdirs = Vec::new();
        match self.fs.ls(path).await {
            Ok(entries) => {
                for entry in entries {
                    if entry.is_dir {
                        let entry_path = if entry.name.starts_with('/') || entry.name.contains("://") {
                            entry.name.clone()
                        } else {
                            format!("{}/{}", path.trim_end_matches('/'), entry.name)
                        };
                        subdirs.push(entry_path.clone());
                        // Recurse into subdirectory
                        let mut children = Box::pin(self.collect_subdirectories(&entry_path)).await;
                        subdirs.append(&mut children);
                    }
                }
            }
            Err(e) => {
                log::warn!("Failed to list directory {path}: {e}");
            }
        }
        subdirs
    }

    /// Acquire locks for a recursive-delete (rm) operation.
    ///
    /// Locks all subdirectories bottom-up, then the target directory.
    /// On failure, all acquired locks are released in reverse order.
    pub async fn acquire_rm(
        &self,
        path: &str,
        tx: &mut TransactionRecord,
        subdirs: &[String],
    ) -> bool {
        let tx_id = tx.id.clone();
        let lp = Self::lock_path(path);
        let mut acquired: Vec<String> = Vec::new();

        // Lock subdirectories (assumed pre-sorted deepest-first by caller)
        for subdir in subdirs {
            let sub_lp = Self::lock_path(subdir);
            if let Err(e) = self.create_lock_file(&sub_lp, &tx_id).await {
                log::error!("Failed to acquire RM sub-lock: {e}");
                // Rollback
                for acq in acquired.iter().rev() {
                    self.remove_lock_file(acq).await;
                }
                return false;
            }
            acquired.push(sub_lp);
        }

        // Lock target directory
        if let Err(e) = self.create_lock_file(&lp, &tx_id).await {
            log::error!("Failed to acquire RM target lock: {e}");
            for acq in acquired.iter().rev() {
                self.remove_lock_file(acq).await;
            }
            return false;
        }
        acquired.push(lp);

        // Register all locks with the transaction
        for lock in &acquired {
            tx.add_lock(lock);
        }

        log::debug!("RM locks acquired for {} paths", acquired.len());
        true
    }

    /// Acquire locks for a move (mv) operation.
    ///
    /// Source is locked with rm-style locking; destination with normal locking.
    pub async fn acquire_mv(
        &self,
        src_path: &str,
        dst_path: &str,
        tx: &mut TransactionRecord,
        src_subdirs: &[String],
    ) -> bool {
        // Lock source (rm-style)
        if !self.acquire_rm(src_path, tx, src_subdirs).await {
            log::warn!("Failed to lock source path: {src_path}");
            return false;
        }

        // Lock destination (normal)
        if !self.acquire_normal(dst_path, tx).await {
            log::warn!("Failed to lock destination path: {dst_path}");
            // Release all source locks
            self.release(tx).await;
            return false;
        }

        log::debug!("MV locks acquired: {src_path} -> {dst_path}");
        true
    }

    /// Release all locks held by the transaction (LIFO order).
    pub async fn release(&self, tx: &mut TransactionRecord) {
        let locks: Vec<String> = tx.locks.iter().rev().cloned().collect();
        for lock_path in &locks {
            self.remove_lock_file(lock_path).await;
        }
        tx.locks.clear();
        log::debug!("Released locks for transaction {}", tx.id);
    }
}
