// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Transaction lifecycle manager.
//!
//! Ported from `openviking/storage/transaction/transaction_manager.py`.

use std::collections::HashMap;
use std::sync::Arc;
use std::time::Duration;

use tokio::sync::Mutex;
use tokio::task::JoinHandle;

use openviking_session::traits::FileSystem;

use crate::path_lock::PathLock;
use crate::transaction_record::{TransactionRecord, TransactionStatus};

/// Manages transaction lifecycles — creation, locking, commit, rollback,
/// and background timeout cleanup.
///
/// Unlike the Python original's global singleton, this struct is created
/// by the caller and its lifetime is explicitly managed.
pub struct TransactionManager<FS: FileSystem + Send + Sync + 'static> {
    path_lock: Arc<PathLock<FS>>,
    transactions: Arc<Mutex<HashMap<String, TransactionRecord>>>,
    timeout: Duration,
    max_parallel_locks: usize,
    cleanup_handle: Mutex<Option<JoinHandle<()>>>,
    running: Arc<tokio::sync::watch::Sender<bool>>,
}

impl<FS: FileSystem + Send + Sync + 'static> TransactionManager<FS> {
    /// Create a new `TransactionManager`.
    ///
    /// * `fs` — file-system backend for lock file operations.
    /// * `timeout` — maximum transaction age before forced rollback.
    /// * `max_parallel_locks` — max concurrent lock operations for RM/MV.
    pub fn new(fs: FS, timeout: Duration, max_parallel_locks: usize) -> Self {
        let (tx, _rx) = tokio::sync::watch::channel(false);
        Self {
            path_lock: Arc::new(PathLock::new(fs)),
            transactions: Arc::new(Mutex::new(HashMap::new())),
            timeout,
            max_parallel_locks,
            cleanup_handle: Mutex::new(None),
            running: Arc::new(tx),
        }
    }

    /// Start the background cleanup task for timed-out transactions.
    pub async fn start(&self) {
        let mut handle = self.cleanup_handle.lock().await;
        if handle.is_some() {
            return;
        }

        let _ = self.running.send(true);
        let txs = Arc::clone(&self.transactions);
        let timeout = self.timeout;
        let mut rx = self.running.subscribe();

        let task = tokio::spawn(async move {
            let mut interval = tokio::time::interval(Duration::from_secs(60));
            loop {
                tokio::select! {
                    _ = interval.tick() => {
                        Self::cleanup_timed_out(&txs, timeout).await;
                    }
                    _ = rx.changed() => {
                        if !*rx.borrow() {
                            break;
                        }
                    }
                }
            }
        });

        *handle = Some(task);
        log::info!("TransactionManager started");
    }

    /// Stop the background cleanup task.
    pub async fn stop(&self) {
        let _ = self.running.send(false);
        let mut handle = self.cleanup_handle.lock().await;
        if let Some(h) = handle.take() {
            h.abort();
        }
        self.transactions.lock().await.clear();
        log::info!("TransactionManager stopped");
    }

    async fn cleanup_timed_out(
        txs: &Mutex<HashMap<String, TransactionRecord>>,
        timeout: Duration,
    ) {
        let now = chrono::Utc::now();
        let mut guard = txs.lock().await;
        let timed_out: Vec<String> = guard
            .iter()
            .filter(|(_, tx)| {
                let age = now.signed_duration_since(tx.updated_at);
                age.to_std().unwrap_or(Duration::ZERO) > timeout
            })
            .map(|(id, _)| id.clone())
            .collect();

        for id in timed_out {
            log::warn!("Transaction timed out: {id}");
            if let Some(tx) = guard.get_mut(&id) {
                tx.update_status(TransactionStatus::Fail);
                tx.update_status(TransactionStatus::Released);
                tx.locks.clear();
            }
            guard.remove(&id);
        }
    }

    // -----------------------------------------------------------------------
    // Transaction lifecycle
    // -----------------------------------------------------------------------

    /// Create a new transaction, returning its record.
    pub async fn create_transaction(
        &self,
        init_info: HashMap<String, serde_json::Value>,
    ) -> TransactionRecord {
        let tx = TransactionRecord::new(init_info);
        self.transactions
            .lock()
            .await
            .insert(tx.id.clone(), tx.clone());
        log::debug!("Transaction created: {}", tx.id);
        tx
    }

    /// Get a clone of a transaction record by ID.
    pub async fn get_transaction(&self, id: &str) -> Option<TransactionRecord> {
        self.transactions.lock().await.get(id).cloned()
    }

    /// Transition a transaction to the `Acquire` state.
    pub async fn begin(&self, id: &str) -> bool {
        let mut guard = self.transactions.lock().await;
        match guard.get_mut(id) {
            Some(tx) => {
                tx.update_status(TransactionStatus::Acquire);
                true
            }
            None => {
                log::error!("Transaction not found: {id}");
                false
            }
        }
    }

    /// Commit a transaction — release all locks and remove from active set.
    pub async fn commit(&self, id: &str) -> bool {
        let mut guard = self.transactions.lock().await;
        let tx = match guard.get_mut(id) {
            Some(t) => t,
            None => {
                log::error!("Transaction not found: {id}");
                return false;
            }
        };

        tx.update_status(TransactionStatus::Commit);
        tx.update_status(TransactionStatus::Releasing);
        self.path_lock.release(tx).await;
        tx.update_status(TransactionStatus::Released);
        guard.remove(id);
        log::debug!("Transaction committed: {id}");
        true
    }

    /// Rollback a transaction — release all locks and remove from active set.
    pub async fn rollback(&self, id: &str) -> bool {
        let mut guard = self.transactions.lock().await;
        let tx = match guard.get_mut(id) {
            Some(t) => t,
            None => {
                log::error!("Transaction not found: {id}");
                return false;
            }
        };

        tx.update_status(TransactionStatus::Fail);
        tx.update_status(TransactionStatus::Releasing);
        self.path_lock.release(tx).await;
        tx.update_status(TransactionStatus::Released);
        guard.remove(id);
        log::debug!("Transaction rolled back: {id}");
        true
    }

    // -----------------------------------------------------------------------
    // Lock acquisition
    // -----------------------------------------------------------------------

    /// Acquire a path lock for normal (non-rm/mv) operations.
    pub async fn acquire_lock_normal(&self, id: &str, path: &str) -> bool {
        let mut guard = self.transactions.lock().await;
        let tx = match guard.get_mut(id) {
            Some(t) => t,
            None => {
                log::error!("Transaction not found: {id}");
                return false;
            }
        };

        tx.update_status(TransactionStatus::Acquire);
        let success = self.path_lock.acquire_normal(path, tx).await;
        tx.update_status(if success {
            TransactionStatus::Exec
        } else {
            TransactionStatus::Fail
        });
        success
    }

    /// Acquire path locks for a recursive-delete (rm) operation.
    pub async fn acquire_lock_rm(
        &self,
        id: &str,
        path: &str,
        subdirs: &[String],
    ) -> bool {
        let mut guard = self.transactions.lock().await;
        let tx = match guard.get_mut(id) {
            Some(t) => t,
            None => {
                log::error!("Transaction not found: {id}");
                return false;
            }
        };

        tx.update_status(TransactionStatus::Acquire);
        let success = self.path_lock.acquire_rm(path, tx, subdirs).await;
        tx.update_status(if success {
            TransactionStatus::Exec
        } else {
            TransactionStatus::Fail
        });
        success
    }

    /// Acquire path locks for a move (mv) operation.
    pub async fn acquire_lock_mv(
        &self,
        id: &str,
        src: &str,
        dst: &str,
        src_subdirs: &[String],
    ) -> bool {
        let mut guard = self.transactions.lock().await;
        let tx = match guard.get_mut(id) {
            Some(t) => t,
            None => {
                log::error!("Transaction not found: {id}");
                return false;
            }
        };

        tx.update_status(TransactionStatus::Acquire);
        let success = self
            .path_lock
            .acquire_mv(src, dst, tx, src_subdirs)
            .await;
        tx.update_status(if success {
            TransactionStatus::Exec
        } else {
            TransactionStatus::Fail
        });
        success
    }

    // -----------------------------------------------------------------------
    // Introspection
    // -----------------------------------------------------------------------

    /// Get all active transactions (clone).
    pub async fn get_active_transactions(&self) -> HashMap<String, TransactionRecord> {
        self.transactions.lock().await.clone()
    }

    /// Get the number of active transactions.
    pub async fn transaction_count(&self) -> usize {
        self.transactions.lock().await.len()
    }

    /// Maximum parallel lock operations for RM/MV (exposed for callers).
    pub fn max_parallel_locks(&self) -> usize {
        self.max_parallel_locks
    }
}
