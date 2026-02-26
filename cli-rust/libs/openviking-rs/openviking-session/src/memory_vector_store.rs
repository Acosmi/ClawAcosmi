// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! In-memory implementation of [`VectorStore`] for testing and lightweight use.
//!
//! Provides a fully functional vector store backed by `HashMap` collections.
//! Supports cosine-similarity search and a basic Filter DSL (`must` / `must_not`).

use std::collections::HashMap;
use std::sync::Mutex;

use async_trait::async_trait;

use crate::traits::{
    BoxError, CollectionInfo, CollectionSchema, ScrollResult, VectorHit, VectorStore,
};

// ---------------------------------------------------------------------------
// Data structures
// ---------------------------------------------------------------------------

/// A single stored record.
#[derive(Debug, Clone)]
struct InMemoryRecord {
    id: String,
    vector: Vec<f32>,
    fields: HashMap<String, serde_json::Value>,
}

/// A collection of records with its schema.
#[derive(Debug)]
struct InMemoryCollection {
    schema: CollectionSchema,
    records: HashMap<String, InMemoryRecord>,
}

/// In-memory [`VectorStore`] implementation.
///
/// Thread-safe via interior `Mutex`. Suitable for unit/integration tests
/// and lightweight single-process workloads.
pub struct InMemoryVectorStore {
    collections: Mutex<HashMap<String, InMemoryCollection>>,
}

impl InMemoryVectorStore {
    /// Create a new, empty store.
    #[must_use]
    pub fn new() -> Self {
        Self {
            collections: Mutex::new(HashMap::new()),
        }
    }
}

impl Default for InMemoryVectorStore {
    fn default() -> Self {
        Self::new()
    }
}

// ---------------------------------------------------------------------------
// Filter DSL engine
// ---------------------------------------------------------------------------

/// Evaluate the filter map against a record's fields.
///
/// Supported operators (top-level keys in the filter map):
///
/// - `"must"`:     `{ "field": "x", "conds": ["a","b"] }` — field value ∈ conds
/// - `"must_not"`: `{ "field": "x", "conds": ["a","b"] }` — field value ∉ conds
///
/// If `filter` is `None` or empty, all records pass.
fn matches_filter(
    fields: &HashMap<String, serde_json::Value>,
    filter: Option<&HashMap<String, serde_json::Value>>,
) -> bool {
    let filter = match filter {
        Some(f) if !f.is_empty() => f,
        _ => return true,
    };

    // Handle "must" clauses
    if let Some(must) = filter.get("must") {
        if !eval_clause(fields, must, true) {
            return false;
        }
    }

    // Handle "must_not" clauses
    if let Some(must_not) = filter.get("must_not") {
        if !eval_clause(fields, must_not, false) {
            return false;
        }
    }

    // Fallback: simple key-value equality (for backward-compat with mock pattern)
    for (key, val) in filter {
        if key == "must" || key == "must_not" {
            continue;
        }
        if fields.get(key) != Some(val) {
            return false;
        }
    }

    true
}

/// Evaluate a single `must` or `must_not` clause.
///
/// `inclusive = true` → must match (field value ∈ conds).
/// `inclusive = false` → must NOT match (field value ∉ conds).
fn eval_clause(
    fields: &HashMap<String, serde_json::Value>,
    clause: &serde_json::Value,
    inclusive: bool,
) -> bool {
    // Support array of clauses: [{ "field": ..., "conds": ... }, ...]
    if let Some(arr) = clause.as_array() {
        return arr.iter().all(|c| eval_single_clause(fields, c, inclusive));
    }
    // Single clause object
    eval_single_clause(fields, clause, inclusive)
}

fn eval_single_clause(
    fields: &HashMap<String, serde_json::Value>,
    clause: &serde_json::Value,
    inclusive: bool,
) -> bool {
    let field_name = match clause.get("field").and_then(|v| v.as_str()) {
        Some(f) => f,
        None => return true, // malformed clause — pass through
    };

    let conds = match clause.get("conds").and_then(|v| v.as_array()) {
        Some(c) => c,
        None => return true,
    };

    let field_val = fields.get(field_name);

    let found = match field_val {
        Some(val) => conds.iter().any(|c| c == val),
        None => false,
    };

    if inclusive {
        found
    } else {
        !found
    }
}

// ---------------------------------------------------------------------------
// VectorStore trait implementation — required + collection management
// ---------------------------------------------------------------------------

#[async_trait]
impl VectorStore for InMemoryVectorStore {
    // === Core required methods ===

    async fn search(
        &self,
        collection: &str,
        vector: &[f32],
        _sparse_vector: Option<&HashMap<String, f64>>,
        limit: usize,
        filter: Option<&HashMap<String, serde_json::Value>>,
    ) -> Result<Vec<VectorHit>, BoxError> {
        let cols = self.collections.lock().unwrap();
        let col = match cols.get(collection) {
            Some(c) => c,
            None => return Ok(Vec::new()),
        };

        let mut scored: Vec<(String, f64, HashMap<String, serde_json::Value>)> = col
            .records
            .values()
            .filter(|r| matches_filter(&r.fields, filter))
            .map(|r| {
                let sim = openviking_core::cosine_similarity(vector, &r.vector);
                (r.id.clone(), f64::from(sim), r.fields.clone())
            })
            .collect();

        // Sort descending by score
        scored.sort_by(|a, b| b.1.partial_cmp(&a.1).unwrap_or(std::cmp::Ordering::Equal));
        scored.truncate(limit);

        let hits = scored
            .into_iter()
            .map(|(id, score, fields)| VectorHit { id, score, fields })
            .collect();

        Ok(hits)
    }

    async fn upsert(
        &self,
        collection: &str,
        id: &str,
        vector: &[f32],
        fields: HashMap<String, serde_json::Value>,
    ) -> Result<(), BoxError> {
        let mut cols = self.collections.lock().unwrap();
        let col = cols.entry(collection.to_owned()).or_insert_with(|| {
            InMemoryCollection {
                schema: CollectionSchema::default(),
                records: HashMap::new(),
            }
        });
        col.records.insert(
            id.to_owned(),
            InMemoryRecord {
                id: id.to_owned(),
                vector: vector.to_vec(),
                fields,
            },
        );
        Ok(())
    }

    async fn update(
        &self,
        collection: &str,
        id: &str,
        fields: HashMap<String, serde_json::Value>,
    ) -> Result<(), BoxError> {
        let mut cols = self.collections.lock().unwrap();
        let col = cols
            .get_mut(collection)
            .ok_or_else(|| format!("collection not found: {collection}"))?;
        let rec = col
            .records
            .get_mut(id)
            .ok_or_else(|| format!("record not found: {id}"))?;
        for (k, v) in fields {
            rec.fields.insert(k, v);
        }
        Ok(())
    }

    async fn delete(&self, collection: &str, id: &str) -> Result<(), BoxError> {
        let mut cols = self.collections.lock().unwrap();
        if let Some(col) = cols.get_mut(collection) {
            col.records.remove(id);
        }
        Ok(())
    }

    // === Collection management ===

    async fn create_collection(
        &self,
        name: &str,
        schema: &CollectionSchema,
    ) -> Result<bool, BoxError> {
        let mut cols = self.collections.lock().unwrap();
        if cols.contains_key(name) {
            return Ok(false);
        }
        cols.insert(
            name.to_owned(),
            InMemoryCollection {
                schema: schema.clone(),
                records: HashMap::new(),
            },
        );
        Ok(true)
    }

    async fn drop_collection(&self, name: &str) -> Result<bool, BoxError> {
        let mut cols = self.collections.lock().unwrap();
        Ok(cols.remove(name).is_some())
    }

    async fn collection_exists(&self, name: &str) -> Result<bool, BoxError> {
        Ok(self.collections.lock().unwrap().contains_key(name))
    }

    async fn list_collections(&self) -> Result<Vec<String>, BoxError> {
        let cols = self.collections.lock().unwrap();
        Ok(cols.keys().cloned().collect())
    }

    async fn get_collection_info(
        &self,
        name: &str,
    ) -> Result<Option<CollectionInfo>, BoxError> {
        let cols = self.collections.lock().unwrap();
        match cols.get(name) {
            Some(col) => Ok(Some(CollectionInfo {
                name: name.to_owned(),
                vector_dim: col.schema.vector_dim,
                count: col.records.len() as u64,
                status: "ready".to_owned(),
            })),
            None => Ok(None),
        }
    }

    // === CRUD — single record extensions ===

    async fn insert_record(
        &self,
        collection: &str,
        mut data: HashMap<String, serde_json::Value>,
    ) -> Result<String, BoxError> {
        let id = data
            .get("id")
            .and_then(|v| v.as_str())
            .map(|s| s.to_owned())
            .unwrap_or_else(|| uuid::Uuid::new_v4().to_string());
        data.insert("id".to_owned(), serde_json::json!(&id));

        let vector: Vec<f32> = data
            .remove("vector")
            .and_then(|v| {
                v.as_array()
                    .map(|a| a.iter().filter_map(|x| x.as_f64().map(|f| f as f32)).collect::<Vec<f32>>())
            })
            .unwrap_or_default();

        self.upsert(collection, &id, &vector, data).await?;
        Ok(id)
    }

    async fn get(
        &self,
        collection: &str,
        ids: &[String],
    ) -> Result<Vec<HashMap<String, serde_json::Value>>, BoxError> {
        let cols = self.collections.lock().unwrap();
        let col = match cols.get(collection) {
            Some(c) => c,
            None => return Ok(Vec::new()),
        };
        let mut results = Vec::new();
        for id in ids {
            if let Some(rec) = col.records.get(id.as_str()) {
                let mut fields = rec.fields.clone();
                fields.insert("id".to_owned(), serde_json::json!(&rec.id));
                results.push(fields);
            }
        }
        Ok(results)
    }

    async fn record_exists(
        &self,
        collection: &str,
        id: &str,
    ) -> Result<bool, BoxError> {
        let cols = self.collections.lock().unwrap();
        Ok(cols
            .get(collection)
            .map(|c| c.records.contains_key(id))
            .unwrap_or(false))
    }

    // === Batch operations ===

    async fn batch_insert(
        &self,
        collection: &str,
        data: Vec<HashMap<String, serde_json::Value>>,
    ) -> Result<Vec<String>, BoxError> {
        let mut ids = Vec::with_capacity(data.len());
        for item in data {
            ids.push(self.insert_record(collection, item).await?);
        }
        Ok(ids)
    }

    async fn batch_upsert(
        &self,
        collection: &str,
        data: Vec<HashMap<String, serde_json::Value>>,
    ) -> Result<Vec<String>, BoxError> {
        let mut ids = Vec::with_capacity(data.len());
        for mut item in data {
            let id = item
                .get("id")
                .and_then(|v| v.as_str())
                .unwrap_or("auto")
                .to_owned();
            let vector: Vec<f32> = item
                .remove("vector")
                .and_then(|v| {
                    v.as_array()
                        .map(|a| a.iter().filter_map(|x| x.as_f64().map(|f| f as f32)).collect::<Vec<f32>>())
                })
                .unwrap_or_default();
            self.upsert(collection, &id, &vector, item).await?;
            ids.push(id);
        }
        Ok(ids)
    }

    async fn batch_delete(
        &self,
        collection: &str,
        filter: &HashMap<String, serde_json::Value>,
    ) -> Result<u64, BoxError> {
        let mut cols = self.collections.lock().unwrap();
        if let Some(col) = cols.get_mut(collection) {
            let before = col.records.len();
            col.records.retain(|_, r| {
                !filter.iter().all(|(k, v)| r.fields.get(k) == Some(v))
            });
            Ok((before - col.records.len()) as u64)
        } else {
            Ok(0)
        }
    }

    async fn remove_by_uri(
        &self,
        collection: &str,
        uri: &str,
    ) -> Result<u64, BoxError> {
        let mut cols = self.collections.lock().unwrap();
        if let Some(col) = cols.get_mut(collection) {
            let before = col.records.len();
            let prefix = format!("{uri}/");
            col.records.retain(|_, r| {
                let rec_uri = r
                    .fields
                    .get("uri")
                    .and_then(|v| v.as_str())
                    .unwrap_or("");
                rec_uri != uri && !rec_uri.starts_with(&prefix)
            });
            Ok((before - col.records.len()) as u64)
        } else {
            Ok(0)
        }
    }

    // === Advanced search ===

    #[allow(clippy::too_many_arguments)]
    async fn search_full(
        &self,
        collection: &str,
        query_vector: Option<&[f32]>,
        sparse_query_vector: Option<&HashMap<String, f64>>,
        filter: Option<&HashMap<String, serde_json::Value>>,
        limit: usize,
        offset: usize,
        _output_fields: Option<&[String]>,
        _with_vector: bool,
    ) -> Result<Vec<HashMap<String, serde_json::Value>>, BoxError> {
        let hits = self
            .search(
                collection,
                query_vector.unwrap_or(&[]),
                sparse_query_vector,
                limit + offset,
                filter,
            )
            .await?;
        Ok(hits.into_iter().skip(offset).take(limit).map(|h| h.fields).collect())
    }

    #[allow(clippy::too_many_arguments)]
    async fn filter_query(
        &self,
        collection: &str,
        filter: &HashMap<String, serde_json::Value>,
        limit: usize,
        offset: usize,
        _output_fields: Option<&[String]>,
        _order_by: Option<&str>,
        _order_desc: bool,
    ) -> Result<Vec<HashMap<String, serde_json::Value>>, BoxError> {
        let cols = self.collections.lock().unwrap();
        let col = match cols.get(collection) {
            Some(c) => c,
            None => return Ok(Vec::new()),
        };
        let filtered: Vec<_> = col
            .records
            .values()
            .filter(|r| matches_filter(&r.fields, Some(filter)))
            .skip(offset)
            .take(limit)
            .map(|r| {
                let mut f = r.fields.clone();
                f.insert("id".to_owned(), serde_json::json!(&r.id));
                f
            })
            .collect();
        Ok(filtered)
    }

    async fn scroll(
        &self,
        collection: &str,
        filter: Option<&HashMap<String, serde_json::Value>>,
        limit: usize,
        cursor: Option<&str>,
        _output_fields: Option<&[String]>,
    ) -> Result<ScrollResult, BoxError> {
        let cols = self.collections.lock().unwrap();
        let col = match cols.get(collection) {
            Some(c) => c,
            None => {
                return Ok(ScrollResult {
                    records: Vec::new(),
                    next_cursor: None,
                })
            }
        };
        let start: usize = cursor.and_then(|c| c.parse().ok()).unwrap_or(0);
        let all: Vec<_> = col
            .records
            .values()
            .filter(|r| matches_filter(&r.fields, filter))
            .collect();
        let batch: Vec<_> = all
            .iter()
            .skip(start)
            .take(limit)
            .map(|r| {
                let mut f = r.fields.clone();
                f.insert("id".to_owned(), serde_json::json!(&r.id));
                f
            })
            .collect();
        let next = if batch.len() == limit && start + limit < all.len() {
            Some((start + limit).to_string())
        } else {
            None
        };
        Ok(ScrollResult {
            records: batch,
            next_cursor: next,
        })
    }

    // === Aggregation ===

    async fn count(
        &self,
        collection: &str,
        filter: Option<&HashMap<String, serde_json::Value>>,
    ) -> Result<u64, BoxError> {
        let cols = self.collections.lock().unwrap();
        match cols.get(collection) {
            Some(col) => {
                let n = col
                    .records
                    .values()
                    .filter(|r| matches_filter(&r.fields, filter))
                    .count();
                Ok(n as u64)
            }
            None => Ok(0),
        }
    }

    // === Index (no-op in memory) ===

    async fn create_index(
        &self,
        _collection: &str,
        _field: &str,
        _index_type: &str,
    ) -> Result<bool, BoxError> {
        Ok(true)
    }

    async fn drop_index(
        &self,
        _collection: &str,
        _field: &str,
    ) -> Result<bool, BoxError> {
        Ok(true)
    }

    // === Lifecycle ===

    async fn clear(&self, collection: &str) -> Result<bool, BoxError> {
        let mut cols = self.collections.lock().unwrap();
        if let Some(col) = cols.get_mut(collection) {
            col.records.clear();
            Ok(true)
        } else {
            Ok(false)
        }
    }

    async fn optimize(&self, _collection: &str) -> Result<bool, BoxError> {
        Ok(true) // no-op
    }

    async fn close(&self) -> Result<(), BoxError> {
        Ok(()) // nothing to release
    }

    // === Health & Status ===

    async fn health_check(&self) -> Result<bool, BoxError> {
        Ok(true)
    }

    async fn get_stats(&self) -> Result<HashMap<String, serde_json::Value>, BoxError> {
        let cols = self.collections.lock().unwrap();
        let total: usize = cols.values().map(|c| c.records.len()).sum();
        let mut stats = HashMap::new();
        stats.insert("collections".to_owned(), serde_json::json!(cols.len()));
        stats.insert("total_records".to_owned(), serde_json::json!(total));
        stats.insert("backend".to_owned(), serde_json::json!("in_memory"));
        Ok(stats)
    }
}
