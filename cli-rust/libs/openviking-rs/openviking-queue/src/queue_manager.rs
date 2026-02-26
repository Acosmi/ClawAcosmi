// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Multi-queue lifecycle manager.
//!
//! Ported from `openviking/storage/queuefs/queue_manager.py`.
//!
//! Unlike the Python original (global singleton with threading), this Rust
//! version:
//! - Is **not** a singleton — callers own and manage its lifetime.
//! - Uses `tokio::spawn` for background workers instead of OS threads.

use std::collections::HashMap;
use std::sync::Arc;
use std::time::Duration;

use tokio::sync::Mutex;
use tokio::task::JoinHandle;

use openviking_session::traits::FileSystem;

use crate::named_queue::NamedQueue;
use crate::queue_types::{DequeueHandler, EnqueueHook, QueueStatus};

/// Standard queue name for embedding operations.
pub const QUEUE_EMBEDDING: &str = "Embedding";
/// Standard queue name for semantic extraction operations.
pub const QUEUE_SEMANTIC: &str = "Semantic";

/// Manages multiple named queues, each with its own background worker.
#[allow(clippy::type_complexity)]
pub struct QueueManager<FS: FileSystem + 'static> {
    fs: Arc<FS>,
    mount_point: String,
    queues: Arc<Mutex<HashMap<String, Arc<Mutex<NamedQueue<FS>>>>>>,
    workers: Mutex<HashMap<String, JoinHandle<()>>>,
    poll_interval: Duration,
    started: bool,
}

impl<FS: FileSystem + 'static> QueueManager<FS> {
    /// Create a new `QueueManager` (does not start workers yet).
    pub fn new(fs: Arc<FS>, mount_point: impl Into<String>) -> Self {
        Self {
            fs,
            mount_point: mount_point.into(),
            queues: Arc::new(Mutex::new(HashMap::new())),
            workers: Mutex::new(HashMap::new()),
            poll_interval: Duration::from_millis(200),
            started: false,
        }
    }

    /// Set the poll interval for background workers.
    pub fn with_poll_interval(mut self, interval: Duration) -> Self {
        self.poll_interval = interval;
        self
    }

    /// Get or create a named queue.
    pub async fn get_or_create_queue(
        &mut self,
        name: &str,
        enqueue_hook: Option<Arc<dyn EnqueueHook>>,
        dequeue_handler: Option<Arc<dyn DequeueHandler>>,
    ) -> Arc<Mutex<NamedQueue<FS>>> {
        let mut queues = self.queues.lock().await;

        if let Some(q) = queues.get(name) {
            return Arc::clone(q);
        }

        let queue = NamedQueue::new(
            Arc::clone(&self.fs),
            &self.mount_point,
            name,
            enqueue_hook,
            dequeue_handler,
        );
        let queue = Arc::new(Mutex::new(queue));
        queues.insert(name.to_owned(), Arc::clone(&queue));

        // Start a worker if the manager is running
        if self.started {
            self.spawn_worker(name, Arc::clone(&queue)).await;
        }

        queue
    }

    /// Start the queue manager — launch workers for all existing queues.
    pub async fn start(&mut self) {
        if self.started {
            return;
        }
        self.started = true;

        let queues = self.queues.lock().await;
        for (name, queue) in queues.iter() {
            self.spawn_worker(name, Arc::clone(queue)).await;
        }

        log::info!("[QueueManager] Started");
    }

    /// Stop the queue manager — abort all workers and clear state.
    pub async fn stop(&mut self) {
        if !self.started {
            return;
        }
        self.started = false;

        let mut workers = self.workers.lock().await;
        for (_, handle) in workers.drain() {
            handle.abort();
        }
        self.queues.lock().await.clear();

        log::info!("[QueueManager] Stopped");
    }

    /// Whether the manager has been started.
    pub fn is_running(&self) -> bool {
        self.started
    }

    /// Check status of one or all queues.
    pub async fn check_status(
        &self,
        queue_name: Option<&str>,
    ) -> HashMap<String, QueueStatus> {
        let queues = self.queues.lock().await;
        let mut result = HashMap::new();

        if let Some(name) = queue_name {
            if let Some(q) = queues.get(name) {
                let mut q = q.lock().await;
                result.insert(name.to_owned(), q.get_status().await);
            }
        } else {
            for (name, q) in queues.iter() {
                let mut q = q.lock().await;
                result.insert(name.clone(), q.get_status().await);
            }
        }

        result
    }

    /// Check if all queues (or a specific one) have completed processing.
    pub async fn is_all_complete(&self, queue_name: Option<&str>) -> bool {
        let statuses = self.check_status(queue_name).await;
        statuses.values().all(|s| s.is_complete())
    }

    /// Wait for completion and return final status.
    ///
    /// Polls `is_all_complete` at `poll_interval` until all queues finish or
    /// `timeout` elapses. Returns the final status map.
    ///
    /// # Errors
    /// Returns `Err("timeout")` if the timeout expires before completion.
    pub async fn wait_complete(
        &self,
        queue_name: Option<&str>,
        timeout: Option<Duration>,
    ) -> Result<HashMap<String, QueueStatus>, String> {
        let start = tokio::time::Instant::now();
        let poll = self.poll_interval;
        loop {
            if self.is_all_complete(queue_name).await {
                return Ok(self.check_status(queue_name).await);
            }
            if let Some(t) = timeout {
                if start.elapsed() > t {
                    return Err(format!(
                        "Queue processing not complete after {:.1}s",
                        t.as_secs_f64()
                    ));
                }
            }
            tokio::time::sleep(poll).await;
        }
    }

    /// Check if any queue (or a specific one) has errors.
    pub async fn has_errors(&self, queue_name: Option<&str>) -> bool {
        let statuses = self.check_status(queue_name).await;
        statuses.values().any(|s| s.has_errors())
    }

    // -----------------------------------------------------------------------
    // Convenience methods — delegate to the named queue
    // -----------------------------------------------------------------------

    /// Enqueue data into a named queue.
    pub async fn enqueue(
        &self,
        queue_name: &str,
        data: serde_json::Value,
    ) -> Result<(), openviking_session::traits::BoxError> {
        let q = {
            let queues = self.queues.lock().await;
            Arc::clone(
                queues
                    .get(queue_name)
                    .ok_or_else(|| format!("Queue not found: {queue_name}"))?,
            )
        };
        let mut guard = q.lock().await;
        guard.enqueue(data).await
    }

    /// Dequeue the next message from a named queue.
    pub async fn dequeue(
        &self,
        queue_name: &str,
    ) -> Result<Option<serde_json::Value>, openviking_session::traits::BoxError> {
        let q = {
            let queues = self.queues.lock().await;
            Arc::clone(
                queues
                    .get(queue_name)
                    .ok_or_else(|| format!("Queue not found: {queue_name}"))?,
            )
        };
        let mut guard = q.lock().await;
        guard.dequeue().await
    }

    /// Peek at the head message of a named queue.
    pub async fn peek(
        &self,
        queue_name: &str,
    ) -> Result<Option<serde_json::Value>, openviking_session::traits::BoxError> {
        let q = {
            let queues = self.queues.lock().await;
            Arc::clone(
                queues
                    .get(queue_name)
                    .ok_or_else(|| format!("Queue not found: {queue_name}"))?,
            )
        };
        let mut guard = q.lock().await;
        guard.peek().await
    }

    /// Get the size of a named queue.
    pub async fn size(&self, queue_name: &str) -> usize {
        let q = {
            let queues = self.queues.lock().await;
            queues.get(queue_name).cloned()
        };
        match q {
            Some(q) => q.lock().await.size().await,
            None => 0,
        }
    }

    /// Clear a named queue.
    pub async fn clear(&self, queue_name: &str) -> bool {
        let q = {
            let queues = self.queues.lock().await;
            queues.get(queue_name).cloned()
        };
        match q {
            Some(q) => q.lock().await.clear().await,
            None => false,
        }
    }

    // -----------------------------------------------------------------------
    // Internal
    // -----------------------------------------------------------------------

    async fn spawn_worker(
        &self,
        name: &str,
        queue: Arc<Mutex<NamedQueue<FS>>>,
    ) {
        let mut workers = self.workers.lock().await;
        if workers.contains_key(name) {
            return;
        }

        let poll = self.poll_interval;
        let queue_name = name.to_owned();

        let handle = tokio::spawn(async move {
            loop {
                {
                    let mut q = queue.lock().await;
                    let has_handler = q.has_dequeue_handler();
                    let sz = q.size().await;
                    if has_handler && sz > 0 {
                        if let Err(e) = q.dequeue().await {
                            log::error!("[QueueManager] Worker error for {queue_name}: {e}");
                        }
                    }
                }
                tokio::time::sleep(poll).await;
            }
        });

        workers.insert(name.to_owned(), handle);
    }
}
