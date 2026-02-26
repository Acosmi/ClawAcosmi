// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Persistent named queue backed by `trait FileSystem`.
//!
//! Ported from `openviking/storage/queuefs/named_queue.py` (lines 92–287).
//!
//! The Python original uses AGFS "virtual files" (`enqueue`, `dequeue`, etc.)
//! to interact with a server-side queue. This Rust port keeps the same
//! path-convention interface so it works with any FileSystem implementation.

use std::sync::Arc;

use chrono::Utc;
use tokio::sync::Mutex;

use openviking_session::traits::{BoxError, FileSystem};

use crate::queue_types::{DequeueHandler, EnqueueHook, QueueError, QueueStatus};

/// Maximum number of error records to keep.
const MAX_ERRORS: usize = 100;

/// A named, persistent queue backed by a [`FileSystem`].
///
/// Queue operations map to virtual file paths:
/// - `{mount}/{name}/enqueue` — write data to enqueue
/// - `{mount}/{name}/dequeue` — read+consume from queue
/// - `{mount}/{name}/peek`    — read head without consuming
/// - `{mount}/{name}/size`    — read queue depth
/// - `{mount}/{name}/clear`   — write empty to clear
pub struct NamedQueue<FS: FileSystem> {
    /// Queue name.
    pub name: String,
    /// Full path prefix for this queue.
    path: String,
    /// File-system backend.
    fs: Arc<FS>,
    /// Optional pre-enqueue hook.
    enqueue_hook: Option<Arc<dyn EnqueueHook>>,
    /// Optional post-dequeue handler.
    dequeue_handler: Option<Arc<dyn DequeueHandler>>,
    /// Whether the queue directory has been ensured.
    initialized: bool,

    // Status tracking (protected by async mutex).
    in_progress: Arc<Mutex<usize>>,
    processed: Arc<Mutex<usize>>,
    error_count: Arc<Mutex<usize>>,
    errors: Arc<Mutex<Vec<QueueError>>>,
}

impl<FS: FileSystem> NamedQueue<FS> {
    /// Create a new named queue.
    pub fn new(
        fs: Arc<FS>,
        mount_point: &str,
        name: impl Into<String>,
        enqueue_hook: Option<Arc<dyn EnqueueHook>>,
        dequeue_handler: Option<Arc<dyn DequeueHandler>>,
    ) -> Self {
        let name = name.into();
        let path = format!("{mount_point}/{name}");
        Self {
            name,
            path,
            fs,
            enqueue_hook,
            dequeue_handler,
            initialized: false,
            in_progress: Arc::new(Mutex::new(0)),
            processed: Arc::new(Mutex::new(0)),
            error_count: Arc::new(Mutex::new(0)),
            errors: Arc::new(Mutex::new(Vec::new())),
        }
    }

    /// Ensure the queue directory exists on the file system.
    async fn ensure_init(&mut self) {
        if !self.initialized {
            let _ = self.fs.mkdir(&self.path).await;
            self.initialized = true;
        }
    }

    // -----------------------------------------------------------------------
    // Status tracking helpers
    // -----------------------------------------------------------------------

    async fn on_dequeue_start(&self) {
        let mut g = self.in_progress.lock().await;
        *g += 1;
    }

    /// Record a processing success.
    pub async fn report_success(&self) {
        {
            let mut g = self.in_progress.lock().await;
            *g = g.saturating_sub(1);
        }
        {
            let mut g = self.processed.lock().await;
            *g += 1;
        }
    }

    /// Record a processing error.
    pub async fn report_error(&self, message: impl Into<String>, data: Option<serde_json::Value>) {
        {
            let mut g = self.in_progress.lock().await;
            *g = g.saturating_sub(1);
        }
        {
            let mut g = self.error_count.lock().await;
            *g += 1;
        }
        {
            let mut g = self.errors.lock().await;
            g.push(QueueError {
                timestamp: Utc::now(),
                message: message.into(),
                data,
            });
            if g.len() > MAX_ERRORS {
                let start = g.len() - MAX_ERRORS;
                *g = g[start..].to_vec();
            }
        }
    }

    /// Get aggregate queue status.
    pub async fn get_status(&mut self) -> QueueStatus {
        let pending = self.size().await;
        QueueStatus {
            pending,
            in_progress: *self.in_progress.lock().await,
            processed: *self.processed.lock().await,
            error_count: *self.error_count.lock().await,
            errors: self.errors.lock().await.clone(),
        }
    }

    /// Reset all status counters.
    pub async fn reset_status(&self) {
        *self.in_progress.lock().await = 0;
        *self.processed.lock().await = 0;
        *self.error_count.lock().await = 0;
        self.errors.lock().await.clear();
    }

    /// Check if this queue has an attached dequeue handler.
    pub fn has_dequeue_handler(&self) -> bool {
        self.dequeue_handler.is_some()
    }

    // -----------------------------------------------------------------------
    // Core queue operations
    // -----------------------------------------------------------------------

    /// Enqueue data. Returns the result of the FS write (may be a message ID).
    pub async fn enqueue(&mut self, data: serde_json::Value) -> Result<(), BoxError> {
        self.ensure_init().await;
        let enqueue_file = format!("{}/enqueue", self.path);

        let data = if let Some(ref hook) = self.enqueue_hook {
            hook.on_enqueue(data).await?
        } else {
            data
        };

        let payload = serde_json::to_string(&data)?;
        self.fs.write(&enqueue_file, &payload).await
    }

    /// Dequeue the next message, optionally invoking the dequeue handler.
    pub async fn dequeue(&mut self) -> Result<Option<serde_json::Value>, BoxError> {
        self.ensure_init().await;
        let dequeue_file = format!("{}/dequeue", self.path);

        let content = match self.fs.read(&dequeue_file).await {
            Ok(c) if c.is_empty() || c == "{}" => return Ok(None),
            Ok(c) => c,
            Err(_) => return Ok(None),
        };

        let data: serde_json::Value = serde_json::from_str(&content)?;

        if let Some(ref handler) = self.dequeue_handler {
            self.on_dequeue_start().await;
            let result = handler.on_dequeue(Some(data)).await?;
            Ok(result)
        } else {
            Ok(Some(data))
        }
    }

    /// Peek at the head message without consuming it.
    pub async fn peek(&mut self) -> Result<Option<serde_json::Value>, BoxError> {
        self.ensure_init().await;
        let peek_file = format!("{}/peek", self.path);

        let content = match self.fs.read(&peek_file).await {
            Ok(c) if c.is_empty() || c == "{}" => return Ok(None),
            Ok(c) => c,
            Err(_) => return Ok(None),
        };

        let data: serde_json::Value = serde_json::from_str(&content)?;
        Ok(Some(data))
    }

    /// Get the number of pending messages.
    pub async fn size(&mut self) -> usize {
        self.ensure_init().await;
        let size_file = format!("{}/size", self.path);

        match self.fs.read(&size_file).await {
            Ok(s) => s.trim().parse().unwrap_or(0),
            Err(_) => 0,
        }
    }

    /// Clear all messages in the queue.
    pub async fn clear(&mut self) -> bool {
        self.ensure_init().await;
        let clear_file = format!("{}/clear", self.path);

        self.fs.write(&clear_file, "").await.is_ok()
    }
}
