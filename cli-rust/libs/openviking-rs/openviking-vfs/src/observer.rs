// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Storage system observers — status reporting and health checking.
//!
//! Ported from `openviking/storage/observers/` (4 files, 535L total).
//! Python's `tabulate` dependency is replaced with manual fixed-width
//! table formatting using `format!()`.
//!
//! **Key difference from Python**: All observer methods are `async` because
//! the underlying `QueueManager`, `TransactionManager`, and `VectorStore`
//! APIs are asynchronous in Rust.

use std::collections::HashMap;
use std::sync::Arc;

use async_trait::async_trait;
use log::warn;
use openviking_session::traits::{FileSystem, VectorStore};

// ---------------------------------------------------------------------------
// Observer trait
// ---------------------------------------------------------------------------

/// Base trait for storage system observers.
///
/// All methods are async because the underlying subsystems expose async APIs.
#[async_trait]
pub trait Observer: Send + Sync {
    /// Format status information as a fixed-width text table.
    async fn get_status_table(&self) -> String;

    /// Check if the observed system is healthy.
    async fn is_healthy(&self) -> bool;

    /// Check if the observed system has any errors.
    async fn has_errors(&self) -> bool;
}

// ---------------------------------------------------------------------------
// Helper: fixed-width table formatter (replaces Python tabulate)
// ---------------------------------------------------------------------------

/// Format rows into a simple fixed-width table.
fn format_fixed_table(headers: &[&str], rows: &[Vec<String>]) -> String {
    if rows.is_empty() {
        return "No data available.".to_owned();
    }

    let col_count = headers.len();
    let mut widths: Vec<usize> = headers.iter().map(|h| h.len()).collect();
    for row in rows {
        for (i, cell) in row.iter().enumerate().take(col_count) {
            widths[i] = widths[i].max(cell.len());
        }
    }

    let mut lines = Vec::new();

    // Header
    let header_cells: Vec<String> = headers
        .iter()
        .enumerate()
        .map(|(i, h)| format!("{:<width$}", h, width = widths[i]))
        .collect();
    lines.push(format!("| {} |", header_cells.join(" | ")));

    // Separator
    let sep: Vec<String> = widths.iter().map(|&w| "-".repeat(w)).collect();
    lines.push(format!("| {} |", sep.join(" | ")));

    // Data rows
    for row in rows {
        let cells: Vec<String> = (0..col_count)
            .map(|i| {
                let cell = row.get(i).map_or("", String::as_str);
                format!("{:<width$}", cell, width = widths[i])
            })
            .collect();
        lines.push(format!("| {} |", cells.join(" | ")));
    }

    lines.join("\n")
}

// ---------------------------------------------------------------------------
// QueueObserver
// ---------------------------------------------------------------------------

/// Observer for queue management status.
pub struct QueueObserver<FS: FileSystem + 'static> {
    queue_manager: Arc<openviking_queue::QueueManager<FS>>,
}

impl<FS: FileSystem + 'static> QueueObserver<FS> {
    /// Create a new `QueueObserver`.
    pub fn new(queue_manager: Arc<openviking_queue::QueueManager<FS>>) -> Self {
        Self { queue_manager }
    }
}

#[async_trait]
impl<FS: FileSystem + 'static> Observer for QueueObserver<FS> {
    async fn get_status_table(&self) -> String {
        let statuses = self.queue_manager.check_status(None).await;
        if statuses.is_empty() {
            return "No queue status data available.".to_owned();
        }

        let headers = &["Queue", "Pending", "In Progress", "Processed", "Errors", "Total"];
        let mut rows = Vec::new();
        let mut total_pending: usize = 0;
        let mut total_in_progress: usize = 0;
        let mut total_processed: usize = 0;
        let mut total_errors: usize = 0;

        for (name, status) in &statuses {
            let total = status.pending + status.in_progress + status.processed;
            rows.push(vec![
                name.clone(),
                status.pending.to_string(),
                status.in_progress.to_string(),
                status.processed.to_string(),
                status.error_count.to_string(),
                total.to_string(),
            ]);
            total_pending += status.pending;
            total_in_progress += status.in_progress;
            total_processed += status.processed;
            total_errors += status.error_count;
        }

        let grand_total = total_pending + total_in_progress + total_processed;
        rows.push(vec![
            "TOTAL".to_owned(),
            total_pending.to_string(),
            total_in_progress.to_string(),
            total_processed.to_string(),
            total_errors.to_string(),
            grand_total.to_string(),
        ]);

        format_fixed_table(headers, &rows)
    }

    async fn is_healthy(&self) -> bool {
        !self.has_errors().await
    }

    async fn has_errors(&self) -> bool {
        self.queue_manager.has_errors(None).await
    }
}

// ---------------------------------------------------------------------------
// TransactionObserver
// ---------------------------------------------------------------------------

/// Observer for transaction management status.
pub struct TransactionObserver<FS: FileSystem + 'static> {
    transaction_manager: Arc<openviking_transaction::TransactionManager<FS>>,
}

impl<FS: FileSystem + 'static> TransactionObserver<FS> {
    /// Create a new `TransactionObserver`.
    pub fn new(
        transaction_manager: Arc<openviking_transaction::TransactionManager<FS>>,
    ) -> Self {
        Self {
            transaction_manager,
        }
    }

    /// Get failed transactions.
    pub async fn get_failed_transactions(
        &self,
    ) -> HashMap<String, openviking_transaction::TransactionRecord> {
        let txs = self.transaction_manager.get_active_transactions().await;
        txs.into_iter()
            .filter(|(_, tx)| tx.status == openviking_transaction::TransactionStatus::Fail)
            .collect()
    }

    /// Get transactions running longer than `timeout_seconds`.
    pub async fn get_hanging_transactions(
        &self,
        timeout_seconds: u64,
    ) -> HashMap<String, openviking_transaction::TransactionRecord> {
        let now = chrono::Utc::now();
        let txs = self.transaction_manager.get_active_transactions().await;
        txs.into_iter()
            .filter(|(_, tx)| {
                let duration = now.signed_duration_since(tx.created_at);
                duration.num_seconds() > timeout_seconds as i64
            })
            .collect()
    }

    /// Get status summary by transaction status.
    pub async fn get_status_summary(&self) -> HashMap<String, usize> {
        let txs = self.transaction_manager.get_active_transactions().await;
        let mut summary = HashMap::new();
        for tx in txs.values() {
            *summary.entry(format!("{:?}", tx.status)).or_insert(0) += 1;
        }
        summary.insert("TOTAL".to_owned(), txs.len());
        summary
    }
}

#[async_trait]
impl<FS: FileSystem + 'static> Observer for TransactionObserver<FS> {
    async fn get_status_table(&self) -> String {
        let txs = self.transaction_manager.get_active_transactions().await;
        if txs.is_empty() {
            return "No active transactions.".to_owned();
        }

        let now = chrono::Utc::now();

        let headers = &["Transaction ID", "Status", "Locks", "Duration"];
        let mut rows: Vec<Vec<String>> = txs
            .iter()
            .map(|(id, tx)| {
                let short_id = if id.len() > 8 {
                    format!("{}...", &id[..8])
                } else {
                    id.clone()
                };
                let duration = now.signed_duration_since(tx.created_at).num_seconds();
                vec![
                    short_id,
                    format!("{:?}", tx.status),
                    tx.locks.len().to_string(),
                    format!("{duration}s"),
                ]
            })
            .collect();

        let total_locks: usize = txs.values().map(|tx| tx.locks.len()).sum();
        rows.push(vec![
            format!("TOTAL ({})", txs.len()),
            String::new(),
            total_locks.to_string(),
            String::new(),
        ]);

        format_fixed_table(headers, &rows)
    }

    async fn is_healthy(&self) -> bool {
        !self.has_errors().await
    }

    async fn has_errors(&self) -> bool {
        let txs = self.transaction_manager.get_active_transactions().await;
        txs.values()
            .any(|tx| tx.status == openviking_transaction::TransactionStatus::Fail)
    }
}

// ---------------------------------------------------------------------------
// VectorStoreObserver
// ---------------------------------------------------------------------------

/// Observer for vector store (VikingDB) status.
pub struct VectorStoreObserver<VS: VectorStore> {
    vector_store: Arc<VS>,
}

impl<VS: VectorStore + 'static> VectorStoreObserver<VS> {
    /// Create a new `VectorStoreObserver`.
    pub fn new(vector_store: Arc<VS>) -> Self {
        Self { vector_store }
    }
}

#[async_trait]
impl<VS: VectorStore + 'static> Observer for VectorStoreObserver<VS> {
    async fn get_status_table(&self) -> String {
        let collections = match self.vector_store.list_collections().await {
            Ok(c) => c,
            Err(e) => {
                warn!("Failed to list collections: {e}");
                return "VectorStore: failed to list collections.".to_owned();
            }
        };

        if collections.is_empty() {
            return "No collections found.".to_owned();
        }

        let headers = &["Collection", "Record Count", "Status"];
        let mut rows = Vec::new();
        let mut total_count: u64 = 0;

        for name in &collections {
            let (count, status) = match self.vector_store.count(name, None).await {
                Ok(c) => {
                    total_count += c;
                    (c.to_string(), "OK".to_owned())
                }
                Err(e) => ("?".to_owned(), format!("ERROR: {e}")),
            };
            rows.push(vec![name.clone(), count, status]);
        }

        rows.push(vec![
            "TOTAL".to_owned(),
            total_count.to_string(),
            String::new(),
        ]);

        format_fixed_table(headers, &rows)
    }

    async fn is_healthy(&self) -> bool {
        self.vector_store.health_check().await.unwrap_or(false)
    }

    async fn has_errors(&self) -> bool {
        !self.is_healthy().await
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_format_fixed_table() {
        let headers = &["Name", "Value"];
        let rows = vec![
            vec!["foo".to_owned(), "123".to_owned()],
            vec!["bar".to_owned(), "456789".to_owned()],
        ];
        let table = format_fixed_table(headers, &rows);
        assert!(table.contains("Name"));
        assert!(table.contains("Value"));
        assert!(table.contains("foo"));
        assert!(table.contains("456789"));
        assert!(table.contains("---"));
    }

    #[test]
    fn test_format_fixed_table_empty() {
        let headers = &["Name", "Value"];
        let rows: Vec<Vec<String>> = vec![];
        let table = format_fixed_table(headers, &rows);
        assert_eq!(table, "No data available.");
    }

    #[test]
    fn test_format_fixed_table_alignment() {
        let headers = &["A", "B", "C"];
        let rows = vec![
            vec!["short".to_owned(), "x".to_owned(), "y".to_owned()],
            vec!["a".to_owned(), "longer".to_owned(), "z".to_owned()],
        ];
        let table = format_fixed_table(headers, &rows);
        assert!(table.contains("| short"));
    }

    #[tokio::test]
    async fn test_vector_store_observer_empty() {
        let vs = Arc::new(openviking_session::InMemoryVectorStore::new());
        let obs = VectorStoreObserver::new(vs);
        let table = obs.get_status_table().await;
        assert_eq!(table, "No collections found.");
        assert!(obs.is_healthy().await);
        assert!(!obs.has_errors().await);
    }

    #[tokio::test]
    async fn test_vector_store_observer_with_collection() {
        let vs = Arc::new(openviking_session::InMemoryVectorStore::new());
        let schema = openviking_session::CollectionSchema::default();
        vs.create_collection("test_col", &schema).await.unwrap();

        let obs = VectorStoreObserver::new(vs);
        let table = obs.get_status_table().await;
        assert!(table.contains("test_col"));
        assert!(table.contains("OK"));
    }
}
