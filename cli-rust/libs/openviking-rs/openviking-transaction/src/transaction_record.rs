// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Transaction record and status definitions.
//!
//! Ported from `openviking/storage/transaction/transaction_record.py`.

use std::collections::HashMap;
use std::fmt;

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

// ---------------------------------------------------------------------------
// TransactionStatus
// ---------------------------------------------------------------------------

/// Transaction lifecycle status.
///
/// State machine: `Init → Acquire → Exec → Commit/Fail → Releasing → Released`
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default, Serialize, Deserialize)]
#[serde(rename_all = "UPPERCASE")]
pub enum TransactionStatus {
    /// Transaction initialized, waiting for lock acquisition.
    #[default]
    Init,
    /// Acquiring lock resources.
    Acquire,
    /// Transaction operation in progress.
    Exec,
    /// Transaction completed successfully.
    Commit,
    /// Transaction failed.
    Fail,
    /// Releasing lock resources.
    Releasing,
    /// Lock resources fully released, transaction ended.
    Released,
}

impl fmt::Display for TransactionStatus {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let s = match self {
            Self::Init => "INIT",
            Self::Acquire => "ACQUIRE",
            Self::Exec => "EXEC",
            Self::Commit => "COMMIT",
            Self::Fail => "FAIL",
            Self::Releasing => "RELEASING",
            Self::Released => "RELEASED",
        };
        f.write_str(s)
    }
}

// ---------------------------------------------------------------------------
// TransactionRecord
// ---------------------------------------------------------------------------

/// Record tracking a single transaction's lifecycle.
///
/// Each transaction holds a list of acquired lock paths and transitions
/// through the [`TransactionStatus`] state machine.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TransactionRecord {
    /// Unique transaction ID (UUID v4).
    pub id: String,
    /// List of lock file paths held by this transaction.
    pub locks: Vec<String>,
    /// Current status in the state machine.
    pub status: TransactionStatus,
    /// Initialization metadata (operation type, etc.).
    #[serde(default)]
    pub init_info: HashMap<String, serde_json::Value>,
    /// Information needed for rollback operations.
    #[serde(default)]
    pub rollback_info: HashMap<String, serde_json::Value>,
    /// Creation timestamp (UTC).
    pub created_at: DateTime<Utc>,
    /// Last update timestamp (UTC).
    pub updated_at: DateTime<Utc>,
}

impl TransactionRecord {
    /// Create a new transaction record with a fresh UUID.
    #[must_use]
    pub fn new(init_info: HashMap<String, serde_json::Value>) -> Self {
        let now = Utc::now();
        Self {
            id: Uuid::new_v4().to_string(),
            locks: Vec::new(),
            status: TransactionStatus::Init,
            init_info,
            rollback_info: HashMap::new(),
            created_at: now,
            updated_at: now,
        }
    }

    /// Update transaction status and refresh `updated_at`.
    pub fn update_status(&mut self, status: TransactionStatus) {
        self.status = status;
        self.updated_at = Utc::now();
    }

    /// Add a lock path to the transaction (de-duplicated).
    pub fn add_lock(&mut self, lock_path: impl Into<String>) {
        let path = lock_path.into();
        if !self.locks.contains(&path) {
            self.locks.push(path);
            self.updated_at = Utc::now();
        }
    }

    /// Remove a lock path from the transaction.
    pub fn remove_lock(&mut self, lock_path: &str) {
        if let Some(pos) = self.locks.iter().position(|p| p == lock_path) {
            self.locks.remove(pos);
            self.updated_at = Utc::now();
        }
    }
}

impl Default for TransactionRecord {
    fn default() -> Self {
        Self::new(HashMap::new())
    }
}
