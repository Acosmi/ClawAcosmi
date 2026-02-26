// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! VikingDB Manager — façade combining VectorStore + QueueManager.
//!
//! Ported from `openviking/storage/vikingdb_manager.py` (160L).
//! Unlike the Python original (which inherits from `VikingVectorIndexBackend`),
//! this Rust version uses **composition**: it holds `Arc<VS>` and
//! `Option<Arc<Mutex<QueueManager<FS>>>>`, delegating calls accordingly.

use std::sync::Arc;

use log::{debug, warn};
use tokio::sync::Mutex;

use openviking_queue::QueueManager;
use openviking_session::traits::{BoxError, FileSystem, VectorStore};

// ---------------------------------------------------------------------------
// VikingDBManager
// ---------------------------------------------------------------------------

/// Façade combining a `VectorStore` with an optional `QueueManager`.
///
/// Provides convenience methods for embedding message enqueueing and
/// graceful shutdown.
///
/// # Type Parameters
/// * `VS` — concrete vector store implementation.
/// * `FS` — concrete file system implementation (used by `QueueManager`).
pub struct VikingDBManager<VS: VectorStore, FS: FileSystem + 'static> {
    vector_store: Arc<VS>,
    queue_manager: Option<Arc<Mutex<QueueManager<FS>>>>,
    closing: bool,
}

impl<VS: VectorStore + 'static, FS: FileSystem + 'static> VikingDBManager<VS, FS> {
    /// Create a new `VikingDBManager`.
    ///
    /// # Arguments
    /// * `vector_store` — shared vector store backend.
    /// * `queue_manager` — optional shared queue manager.
    pub fn new(
        vector_store: Arc<VS>,
        queue_manager: Option<Arc<Mutex<QueueManager<FS>>>>,
    ) -> Self {
        Self {
            vector_store,
            queue_manager,
            closing: false,
        }
    }

    /// Get a reference to the inner vector store.
    pub fn vector_store(&self) -> &VS {
        &self.vector_store
    }

    /// Get a clone of the inner vector store Arc.
    pub fn vector_store_arc(&self) -> Arc<VS> {
        Arc::clone(&self.vector_store)
    }

    /// Get a reference to the queue manager (if present).
    pub fn queue_manager(&self) -> Option<&Arc<Mutex<QueueManager<FS>>>> {
        self.queue_manager.as_ref()
    }

    /// Whether a queue manager has been configured.
    pub fn has_queue_manager(&self) -> bool {
        self.queue_manager.is_some()
    }

    /// Whether the manager is in shutdown flow.
    pub fn is_closing(&self) -> bool {
        self.closing
    }

    // -----------------------------------------------------------------------
    // Queue convenience methods
    // -----------------------------------------------------------------------

    /// Enqueue an embedding message for processing.
    ///
    /// Serialises `msg` to JSON and pushes it into the `"Embedding"` queue.
    ///
    /// # Errors
    /// Returns `Err` if no queue manager is configured or enqueueing fails.
    pub async fn enqueue_embedding_msg(
        &self,
        msg: serde_json::Value,
    ) -> Result<bool, BoxError> {
        let qm = self
            .queue_manager
            .as_ref()
            .ok_or("Queue manager not initialized, cannot enqueue embedding")?;

        let manager = qm.lock().await;
        manager.enqueue(openviking_queue::queue_manager::QUEUE_EMBEDDING, msg).await?;
        debug!("Enqueued embedding message");
        Ok(true)
    }

    /// Get the current size of the embedding queue.
    pub async fn get_embedding_queue_size(&self) -> usize {
        match &self.queue_manager {
            Some(qm) => {
                let manager = qm.lock().await;
                manager.size(openviking_queue::queue_manager::QUEUE_EMBEDDING).await
            }
            None => 0,
        }
    }

    /// Gracefully close the manager.
    ///
    /// Sets the `is_closing` flag and delegates to the vector store's `close()`.
    /// The queue manager is **not** stopped here — it is an injected dependency
    /// and should be managed by its creator.
    pub async fn close(&mut self) -> Result<(), BoxError> {
        self.closing = true;
        if let Err(e) = self.vector_store.close().await {
            warn!("Error closing VikingDB manager: {e}");
        }
        Ok(())
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use openviking_session::InMemoryVectorStore;

    // A minimal dummy FileSystem (needed for QueueManager type parameter).
    // We don't actually use the FS operations in these unit tests.
    struct DummyFs;

    #[async_trait::async_trait]
    impl openviking_session::traits::FileSystem for DummyFs {
        async fn read(&self, _: &str) -> Result<String, BoxError> {
            Err("not implemented".into())
        }
        async fn read_bytes(&self, _: &str) -> Result<Vec<u8>, BoxError> {
            Err("not implemented".into())
        }
        async fn write(&self, _: &str, _: &str) -> Result<(), BoxError> {
            Err("not implemented".into())
        }
        async fn write_bytes(&self, _: &str, _: &[u8]) -> Result<(), BoxError> {
            Err("not implemented".into())
        }
        async fn mkdir(&self, _: &str) -> Result<(), BoxError> {
            Ok(())
        }
        async fn ls(
            &self,
            _: &str,
        ) -> Result<Vec<openviking_session::traits::FsEntry>, BoxError> {
            Ok(vec![])
        }
        async fn rm(&self, _: &str) -> Result<(), BoxError> {
            Ok(())
        }
        async fn mv(&self, _: &str, _: &str) -> Result<(), BoxError> {
            Ok(())
        }
        async fn stat(
            &self,
            _: &str,
        ) -> Result<openviking_session::traits::FsStat, BoxError> {
            Err("not implemented".into())
        }
        async fn grep(
            &self,
            _: &str,
            _: &str,
            _: bool,
            _: bool,
        ) -> Result<Vec<openviking_session::traits::GrepMatch>, BoxError> {
            Ok(vec![])
        }
        async fn exists(&self, _: &str) -> Result<bool, BoxError> {
            Ok(false)
        }
        async fn append(&self, _: &str, _: &str) -> Result<(), BoxError> {
            Ok(())
        }
        async fn link(&self, _: &str, _: &str) -> Result<(), BoxError> {
            Ok(())
        }
    }

    #[test]
    fn test_manager_new_without_queue() {
        let vs = Arc::new(InMemoryVectorStore::new());
        let mgr: VikingDBManager<_, DummyFs> =
            VikingDBManager::new(vs, None);
        assert!(!mgr.has_queue_manager());
        assert!(!mgr.is_closing());
    }

    #[test]
    fn test_manager_new_with_queue() {
        let vs = Arc::new(InMemoryVectorStore::new());
        let fs = Arc::new(DummyFs);
        let qm = QueueManager::new(fs, "viking://queues");
        let qm_arc = Arc::new(tokio::sync::Mutex::new(qm));
        let mgr = VikingDBManager::new(vs, Some(qm_arc));
        assert!(mgr.has_queue_manager());
    }

    #[tokio::test]
    async fn test_manager_enqueue_no_queue() {
        let vs = Arc::new(InMemoryVectorStore::new());
        let mgr: VikingDBManager<_, DummyFs> =
            VikingDBManager::new(vs, None);
        let result = mgr.enqueue_embedding_msg(serde_json::json!({"id": "test"})).await;
        assert!(result.is_err());
    }

    #[tokio::test]
    async fn test_manager_close() {
        let vs = Arc::new(InMemoryVectorStore::new());
        let mut mgr: VikingDBManager<_, DummyFs> =
            VikingDBManager::new(vs, None);
        assert!(!mgr.is_closing());
        mgr.close().await.unwrap();
        assert!(mgr.is_closing());
    }
}
