// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Async trait abstractions for IO injection.
//!
//! These traits decouple business logic from concrete storage, LLM, and
//! embedding backends. Implementors provide the actual IO; consumers (Session,
//! Compressor, Retriever) operate purely against these interfaces.

use std::collections::HashMap;

use async_trait::async_trait;

/// Error type for trait operations.
pub type BoxError = Box<dyn std::error::Error + Send + Sync>;

// ---------------------------------------------------------------------------
// LLM Provider
// ---------------------------------------------------------------------------

/// Abstraction over a text-based LLM completion API.
#[async_trait]
pub trait LlmProvider: Send + Sync {
    /// Generate a completion for the given prompt.
    async fn completion(&self, prompt: &str) -> Result<String, BoxError>;
}

// ---------------------------------------------------------------------------
// Vector Store
// ---------------------------------------------------------------------------

/// A single vector search hit.
#[derive(Debug, Clone)]
pub struct VectorHit {
    /// URI or ID of the matched record.
    pub id: String,
    /// Relevance score.
    pub score: f64,
    /// Field values returned with the hit.
    pub fields: HashMap<String, serde_json::Value>,
}

/// Distance metric for vector similarity.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, serde::Serialize, serde::Deserialize)]
pub enum DistanceMetric {
    /// Cosine similarity.
    #[default]
    Cosine,
    /// Euclidean (L2) distance.
    Euclid,
    /// Dot product.
    DotProduct,
}

/// A field definition within a collection schema.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct FieldDef {
    /// Field name.
    pub name: String,
    /// Field type (e.g. "string", "path", "vector", "sparse_vector",
    /// "date_time", "int64", "bool").
    pub field_type: String,
    /// Whether this field is indexed.
    #[serde(default)]
    pub indexed: bool,
    /// Whether this field is the primary key.
    #[serde(default)]
    pub is_primary: bool,
    /// Vector dimension (only for `field_type = "vector"`).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub dim: Option<usize>,
}

/// Schema for creating a collection.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct CollectionSchema {
    /// Vector dimension (default: 2048).
    pub vector_dim: usize,
    /// Distance metric.
    pub distance: DistanceMetric,
    /// Field definitions.
    pub fields: Vec<FieldDef>,
}

impl Default for CollectionSchema {
    fn default() -> Self {
        Self {
            vector_dim: 2048,
            distance: DistanceMetric::default(),
            fields: Vec::new(),
        }
    }
}

/// Collection metadata and statistics.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct CollectionInfo {
    /// Collection name.
    pub name: String,
    /// Vector dimension.
    pub vector_dim: usize,
    /// Record count.
    pub count: u64,
    /// Status string (e.g. "ready", "loading").
    pub status: String,
}

/// Result of a scroll operation.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct ScrollResult {
    /// Records in this batch.
    pub records: Vec<HashMap<String, serde_json::Value>>,
    /// Cursor for next batch; `None` when exhausted.
    pub next_cursor: Option<String>,
}

/// Typed errors for vector store operations.
#[derive(Debug)]
pub enum VectorStoreError {
    /// Collection does not exist.
    CollectionNotFound(String),
    /// Record does not exist.
    RecordNotFound(String),
    /// Duplicate key on insert.
    DuplicateKey(String),
    /// Backend connection failure.
    ConnectionError(String),
    /// Schema validation failure.
    SchemaError(String),
    /// Catch-all.
    Other(BoxError),
}

impl std::fmt::Display for VectorStoreError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::CollectionNotFound(n) => write!(f, "collection not found: {n}"),
            Self::RecordNotFound(id) => write!(f, "record not found: {id}"),
            Self::DuplicateKey(id) => write!(f, "duplicate key: {id}"),
            Self::ConnectionError(msg) => write!(f, "connection error: {msg}"),
            Self::SchemaError(msg) => write!(f, "schema error: {msg}"),
            Self::Other(e) => write!(f, "{e}"),
        }
    }
}

impl std::error::Error for VectorStoreError {}

/// Abstraction over a vector database (e.g. VikingDB, Qdrant, Weaviate).
///
/// Core methods (`search`, `upsert`, `update`, `delete`) must be implemented.
/// All other methods provide default implementations returning
/// `Err("not implemented")` so existing implementors remain backward-compatible.
#[async_trait]
pub trait VectorStore: Send + Sync {
    // ===================================================================
    // Core methods (required — backward-compatible with Phase 1)
    // ===================================================================

    /// Search by dense and/or sparse vector.
    async fn search(
        &self,
        collection: &str,
        vector: &[f32],
        sparse_vector: Option<&HashMap<String, f64>>,
        limit: usize,
        filter: Option<&HashMap<String, serde_json::Value>>,
    ) -> Result<Vec<VectorHit>, BoxError>;

    /// Upsert a record with vector and fields.
    async fn upsert(
        &self,
        collection: &str,
        id: &str,
        vector: &[f32],
        fields: HashMap<String, serde_json::Value>,
    ) -> Result<(), BoxError>;

    /// Update specific fields of a record (no vector change).
    async fn update(
        &self,
        collection: &str,
        id: &str,
        fields: HashMap<String, serde_json::Value>,
    ) -> Result<(), BoxError>;

    /// Delete a record by ID.
    async fn delete(&self, collection: &str, id: &str) -> Result<(), BoxError>;

    // ===================================================================
    // Collection management
    // ===================================================================

    /// Create a new collection.
    async fn create_collection(
        &self,
        _name: &str,
        _schema: &CollectionSchema,
    ) -> Result<bool, BoxError> {
        Err("create_collection not implemented".into())
    }

    /// Drop a collection.
    async fn drop_collection(&self, _name: &str) -> Result<bool, BoxError> {
        Err("drop_collection not implemented".into())
    }

    /// Check if a collection exists.
    async fn collection_exists(&self, _name: &str) -> Result<bool, BoxError> {
        Err("collection_exists not implemented".into())
    }

    /// List all collection names.
    async fn list_collections(&self) -> Result<Vec<String>, BoxError> {
        Err("list_collections not implemented".into())
    }

    /// Get collection metadata and statistics.
    async fn get_collection_info(
        &self,
        _name: &str,
    ) -> Result<Option<CollectionInfo>, BoxError> {
        Err("get_collection_info not implemented".into())
    }

    // ===================================================================
    // CRUD — single record extensions
    // ===================================================================

    /// Insert a single record, returning its ID.
    async fn insert_record(
        &self,
        _collection: &str,
        _data: HashMap<String, serde_json::Value>,
    ) -> Result<String, BoxError> {
        Err("insert_record not implemented".into())
    }

    /// Get records by IDs.
    async fn get(
        &self,
        _collection: &str,
        _ids: &[String],
    ) -> Result<Vec<HashMap<String, serde_json::Value>>, BoxError> {
        Err("get not implemented".into())
    }

    /// Check if a record exists.
    async fn record_exists(
        &self,
        _collection: &str,
        _id: &str,
    ) -> Result<bool, BoxError> {
        Err("record_exists not implemented".into())
    }

    // ===================================================================
    // CRUD — batch operations
    // ===================================================================

    /// Batch insert multiple records, returning their IDs.
    async fn batch_insert(
        &self,
        _collection: &str,
        _data: Vec<HashMap<String, serde_json::Value>>,
    ) -> Result<Vec<String>, BoxError> {
        Err("batch_insert not implemented".into())
    }

    /// Batch upsert multiple records, returning their IDs.
    async fn batch_upsert(
        &self,
        _collection: &str,
        _data: Vec<HashMap<String, serde_json::Value>>,
    ) -> Result<Vec<String>, BoxError> {
        Err("batch_upsert not implemented".into())
    }

    /// Batch delete by filter, returning count of deleted records.
    async fn batch_delete(
        &self,
        _collection: &str,
        _filter: &HashMap<String, serde_json::Value>,
    ) -> Result<u64, BoxError> {
        Err("batch_delete not implemented".into())
    }

    /// Remove records by URI (including directory descendants).
    async fn remove_by_uri(
        &self,
        _collection: &str,
        _uri: &str,
    ) -> Result<u64, BoxError> {
        Err("remove_by_uri not implemented".into())
    }

    // ===================================================================
    // Advanced search
    // ===================================================================

    /// Full hybrid search with pagination and output field selection.
    #[allow(clippy::too_many_arguments)]
    async fn search_full(
        &self,
        _collection: &str,
        _query_vector: Option<&[f32]>,
        _sparse_query_vector: Option<&HashMap<String, f64>>,
        _filter: Option<&HashMap<String, serde_json::Value>>,
        _limit: usize,
        _offset: usize,
        _output_fields: Option<&[String]>,
        _with_vector: bool,
    ) -> Result<Vec<HashMap<String, serde_json::Value>>, BoxError> {
        Err("search_full not implemented".into())
    }

    /// Pure scalar filtering without vector search.
    #[allow(clippy::too_many_arguments)]
    async fn filter_query(
        &self,
        _collection: &str,
        _filter: &HashMap<String, serde_json::Value>,
        _limit: usize,
        _offset: usize,
        _output_fields: Option<&[String]>,
        _order_by: Option<&str>,
        _order_desc: bool,
    ) -> Result<Vec<HashMap<String, serde_json::Value>>, BoxError> {
        Err("filter_query not implemented".into())
    }

    /// Scroll through large result sets.
    async fn scroll(
        &self,
        _collection: &str,
        _filter: Option<&HashMap<String, serde_json::Value>>,
        _limit: usize,
        _cursor: Option<&str>,
        _output_fields: Option<&[String]>,
    ) -> Result<ScrollResult, BoxError> {
        Err("scroll not implemented".into())
    }

    // ===================================================================
    // Aggregation
    // ===================================================================

    /// Count records matching an optional filter.
    async fn count(
        &self,
        _collection: &str,
        _filter: Option<&HashMap<String, serde_json::Value>>,
    ) -> Result<u64, BoxError> {
        Err("count not implemented".into())
    }

    // ===================================================================
    // Index management
    // ===================================================================

    /// Create an index on a field.
    async fn create_index(
        &self,
        _collection: &str,
        _field: &str,
        _index_type: &str,
    ) -> Result<bool, BoxError> {
        Err("create_index not implemented".into())
    }

    /// Drop an index on a field.
    async fn drop_index(
        &self,
        _collection: &str,
        _field: &str,
    ) -> Result<bool, BoxError> {
        Err("drop_index not implemented".into())
    }

    // ===================================================================
    // Lifecycle
    // ===================================================================

    /// Clear all data in a collection (keep schema).
    async fn clear(&self, _collection: &str) -> Result<bool, BoxError> {
        Err("clear not implemented".into())
    }

    /// Optimize collection for better performance.
    async fn optimize(&self, _collection: &str) -> Result<bool, BoxError> {
        Err("optimize not implemented".into())
    }

    /// Close storage connection and release resources.
    async fn close(&self) -> Result<(), BoxError> {
        Err("close not implemented".into())
    }

    // ===================================================================
    // Health & Status
    // ===================================================================

    /// Check if backend is healthy.
    async fn health_check(&self) -> Result<bool, BoxError> {
        Err("health_check not implemented".into())
    }

    /// Get storage statistics.
    async fn get_stats(&self) -> Result<HashMap<String, serde_json::Value>, BoxError> {
        Err("get_stats not implemented".into())
    }
}

// ---------------------------------------------------------------------------
// Embedder
// ---------------------------------------------------------------------------

/// Result of an embedding operation.
#[derive(Debug, Clone)]
pub struct EmbedResult {
    /// Dense vector.
    pub dense_vector: Vec<f32>,
    /// Sparse vector (term → weight).
    pub sparse_vector: Option<HashMap<String, f64>>,
}

/// Abstraction over a text embedding model.
#[async_trait]
pub trait Embedder: Send + Sync {
    /// Embed text into dense and optionally sparse vectors.
    async fn embed(&self, text: &str) -> Result<EmbedResult, BoxError>;
}

// ---------------------------------------------------------------------------
// File System
// ---------------------------------------------------------------------------

/// A directory listing entry.
#[derive(Debug, Clone)]
pub struct FsEntry {
    /// Entry name.
    pub name: String,
    /// Whether this is a directory.
    pub is_dir: bool,
    /// Size in bytes (0 for directories).
    pub size: u64,
}

/// File/directory metadata (stat result).
#[derive(Debug, Clone)]
pub struct FsStat {
    /// Entry name.
    pub name: String,
    /// Size in bytes.
    pub size: u64,
    /// Whether this is a directory.
    pub is_dir: bool,
    /// Last modification time (ISO 8601 string).
    pub mod_time: String,
}

/// A single grep match.
#[derive(Debug, Clone)]
pub struct GrepMatch {
    /// URI of the matched file.
    pub uri: String,
    /// Line number (1-indexed).
    pub line: u64,
    /// Content of the matching line.
    pub content: String,
}

/// Abstraction over a Viking-URI-aware file system (e.g. AGFS).
#[async_trait]
pub trait FileSystem: Send + Sync {
    /// Read file content as string.
    async fn read(&self, uri: &str) -> Result<String, BoxError>;

    /// Read file content as bytes.
    async fn read_bytes(&self, uri: &str) -> Result<Vec<u8>, BoxError>;

    /// Write string content.
    async fn write(&self, uri: &str, content: &str) -> Result<(), BoxError>;

    /// Write binary content.
    async fn write_bytes(&self, uri: &str, content: &[u8]) -> Result<(), BoxError>;

    /// Create a directory.
    async fn mkdir(&self, uri: &str) -> Result<(), BoxError>;

    /// List directory contents.
    async fn ls(&self, uri: &str) -> Result<Vec<FsEntry>, BoxError>;

    /// Remove a file or directory.
    async fn rm(&self, uri: &str) -> Result<(), BoxError>;

    /// Move/rename a file or directory.
    async fn mv(&self, from_uri: &str, to_uri: &str) -> Result<(), BoxError>;

    /// Get file/directory metadata.
    async fn stat(&self, uri: &str) -> Result<FsStat, BoxError>;

    /// Content search by pattern (grep).
    async fn grep(
        &self,
        uri: &str,
        pattern: &str,
        recursive: bool,
        case_insensitive: bool,
    ) -> Result<Vec<GrepMatch>, BoxError>;

    /// Check if a file or directory exists.
    async fn exists(&self, uri: &str) -> Result<bool, BoxError>;

    /// Append string content to a file.
    async fn append(&self, uri: &str, content: &str) -> Result<(), BoxError>;

    /// Create a symbolic link between two URIs.
    async fn link(&self, source_uri: &str, target_uri: &str) -> Result<(), BoxError>;
}

// ---------------------------------------------------------------------------
// Reranker
// ---------------------------------------------------------------------------

/// A single reranking result.
#[derive(Debug, Clone)]
pub struct RerankResult {
    /// Original index in the input list.
    pub index: usize,
    /// Reranked relevance score (higher = more relevant).
    pub score: f64,
}

/// Abstraction over a reranking model (e.g. Cohere, BGE-reranker).
///
/// Rerankers improve precision by re-scoring candidate results against
/// the original query using cross-encoder attention.
#[async_trait]
pub trait Reranker: Send + Sync {
    /// Rerank a list of documents against a query.
    ///
    /// Returns scores for each document, sorted by relevance (highest first).
    async fn rerank(
        &self,
        query: &str,
        documents: &[String],
        top_k: usize,
    ) -> Result<Vec<RerankResult>, BoxError>;
}
