// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Collection schema definitions for OpenViking.
//!
//! Provides centralized schema definitions and factory functions for creating
//! collections. Ported from `openviking/storage/collection_schemas.py`.
//!
//! **Not ported**: `TextEmbeddingHandler` — depends on runtime Embedder
//! injection, belongs to Stage D / Go integration layer.

use crate::traits::{BoxError, CollectionSchema, FieldDef, VectorStore};

// ---------------------------------------------------------------------------
// Schema helpers
// ---------------------------------------------------------------------------

/// Shorthand for a scalar field definition.
fn scalar(name: &str, field_type: &str) -> FieldDef {
    FieldDef {
        name: name.to_owned(),
        field_type: field_type.to_owned(),
        indexed: false,
        is_primary: false,
        dim: None,
    }
}

/// Shorthand for the primary key field.
fn primary(name: &str) -> FieldDef {
    FieldDef {
        name: name.to_owned(),
        field_type: "string".to_owned(),
        indexed: false,
        is_primary: true,
        dim: None,
    }
}

/// Shorthand for the dense vector field.
fn vector_field(name: &str, dim: usize) -> FieldDef {
    FieldDef {
        name: name.to_owned(),
        field_type: "vector".to_owned(),
        indexed: false,
        is_primary: false,
        dim: Some(dim),
    }
}

// ---------------------------------------------------------------------------
// CollectionSchemas
// ---------------------------------------------------------------------------

/// Centralized collection schema definitions.
///
/// Mirrors the Python `CollectionSchemas` class, providing static factory
/// methods that return fully populated [`CollectionSchema`] instances.
pub struct CollectionSchemas;

impl CollectionSchemas {
    /// Schema for the unified context collection.
    ///
    /// Contains 15 fields and 10 scalar indexes, matching the Python
    /// `CollectionSchemas.context_collection()` output.
    ///
    /// # Fields
    ///
    /// | Name | Type | Notes |
    /// |------|------|-------|
    /// | `id` | string | Primary key |
    /// | `uri` | path | Viking URI |
    /// | `type` | string | Context type label |
    /// | `context_type` | string | Memory / Resource / Skill |
    /// | `vector` | vector | Dense embedding (dim = `vector_dim`) |
    /// | `sparse_vector` | sparse_vector | Sparse embedding |
    /// | `created_at` | date_time | Creation timestamp |
    /// | `updated_at` | date_time | Last update timestamp |
    /// | `active_count` | int64 | Reference counter |
    /// | `parent_uri` | path | Parent node URI |
    /// | `is_leaf` | bool | Leaf vs directory flag |
    /// | `name` | string | Human-readable name |
    /// | `description` | string | Detailed description |
    /// | `tags` | string | Comma-separated tags |
    /// | `abstract` | string | L0 summary text |
    #[must_use]
    pub fn context_collection(vector_dim: usize) -> CollectionSchema {
        CollectionSchema {
            vector_dim,
            distance: crate::traits::DistanceMetric::Cosine,
            fields: vec![
                primary("id"),
                scalar("uri", "path"),
                scalar("type", "string"),
                scalar("context_type", "string"),
                vector_field("vector", vector_dim),
                scalar("sparse_vector", "sparse_vector"),
                scalar("created_at", "date_time"),
                scalar("updated_at", "date_time"),
                scalar("active_count", "int64"),
                scalar("parent_uri", "path"),
                scalar("is_leaf", "bool"),
                scalar("name", "string"),
                scalar("description", "string"),
                scalar("tags", "string"),
                scalar("abstract", "string"),
            ],
        }
    }

    /// Names of fields that should have scalar indexes on the context
    /// collection, matching the Python `ScalarIndex` list.
    #[must_use]
    pub fn context_scalar_indexes() -> &'static [&'static str] {
        &[
            "uri",
            "type",
            "context_type",
            "created_at",
            "updated_at",
            "active_count",
            "parent_uri",
            "is_leaf",
            "name",
            "tags",
        ]
    }
}

// ---------------------------------------------------------------------------
// init_context_collection
// ---------------------------------------------------------------------------

/// Initialize the context collection with proper schema, creating it if it
/// does not already exist.
///
/// This is a convenience function that:
/// 1. Checks whether the collection already exists.
/// 2. If not, creates it with the schema from
///    [`CollectionSchemas::context_collection`].
/// 3. Creates scalar indexes on the standard fields.
///
/// Returns `true` if the collection was newly created, `false` if it already
/// existed.
///
/// # Errors
///
/// Returns an error if the underlying [`VectorStore`] operations fail.
pub async fn init_context_collection<VS: VectorStore>(
    vs: &VS,
    collection_name: &str,
    vector_dim: usize,
) -> Result<bool, BoxError> {
    // Check if collection already exists (idempotent)
    if vs.collection_exists(collection_name).await? {
        return Ok(false);
    }

    // Create with full schema
    let schema = CollectionSchemas::context_collection(vector_dim);
    vs.create_collection(collection_name, &schema).await?;

    // Create scalar indexes (best-effort — implementations may no-op)
    for field in CollectionSchemas::context_scalar_indexes() {
        let _ = vs.create_index(collection_name, field, "scalar").await;
    }

    Ok(true)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use crate::memory_vector_store::InMemoryVectorStore;

    #[test]
    fn context_schema_has_15_fields() {
        let schema = CollectionSchemas::context_collection(768);
        assert_eq!(schema.fields.len(), 15);
        assert_eq!(schema.vector_dim, 768);
    }

    #[test]
    fn context_schema_primary_key_is_id() {
        let schema = CollectionSchemas::context_collection(768);
        let pk = schema.fields.iter().find(|f| f.is_primary);
        assert!(pk.is_some(), "must have a primary key");
        assert_eq!(pk.unwrap().name, "id");
    }

    #[test]
    fn context_schema_vector_field_has_correct_dim() {
        let schema = CollectionSchemas::context_collection(1536);
        let vec_field = schema.fields.iter().find(|f| f.field_type == "vector");
        assert!(vec_field.is_some());
        assert_eq!(vec_field.unwrap().dim, Some(1536));
    }

    #[test]
    fn context_schema_has_10_scalar_indexes() {
        assert_eq!(CollectionSchemas::context_scalar_indexes().len(), 10);
    }

    #[test]
    fn context_schema_field_names_match_python() {
        let schema = CollectionSchemas::context_collection(768);
        let names: Vec<&str> = schema.fields.iter().map(|f| f.name.as_str()).collect();
        assert_eq!(
            names,
            vec![
                "id", "uri", "type", "context_type", "vector", "sparse_vector",
                "created_at", "updated_at", "active_count", "parent_uri",
                "is_leaf", "name", "description", "tags", "abstract",
            ]
        );
    }

    #[tokio::test]
    async fn init_context_collection_creates_new() {
        let vs = InMemoryVectorStore::new();
        let created = init_context_collection(&vs, "context", 768).await.unwrap();
        assert!(created);
        assert!(vs.collection_exists("context").await.unwrap());

        // Verify schema was applied
        let info = vs.get_collection_info("context").await.unwrap().unwrap();
        assert_eq!(info.vector_dim, 768);
    }

    #[tokio::test]
    async fn init_context_collection_idempotent() {
        let vs = InMemoryVectorStore::new();
        let first = init_context_collection(&vs, "context", 768).await.unwrap();
        assert!(first);
        let second = init_context_collection(&vs, "context", 768).await.unwrap();
        assert!(!second, "second call should return false (already exists)");
    }
}
