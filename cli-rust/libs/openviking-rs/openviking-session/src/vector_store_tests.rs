// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Comprehensive tests for the expanded `VectorStore` trait.

use std::collections::HashMap;
use std::sync::{Arc, Mutex};

use async_trait::async_trait;

use crate::traits::{
    BoxError, CollectionInfo, CollectionSchema, ScrollResult, VectorHit, VectorStore,
};

// ===========================================================================
// Full Mock — implements ALL methods for testing
// ===========================================================================

#[derive(Clone, Default)]
#[allow(clippy::type_complexity)]
struct FullMockVs {
    /// collection_name -> Vec<record>
    collections: Arc<Mutex<HashMap<String, Vec<HashMap<String, serde_json::Value>>>>>,
    /// collection_name -> schema exists
    schemas: Arc<Mutex<HashMap<String, CollectionSchema>>>,
}

impl FullMockVs {
    fn new() -> Self {
        Self::default()
    }

    /// Helper: get next ID for a collection.
    fn next_id(records: &[HashMap<String, serde_json::Value>]) -> String {
        format!("rec_{}", records.len() + 1)
    }
}

#[async_trait]
impl VectorStore for FullMockVs {
    // === Core (required) ===

    async fn search(
        &self,
        collection: &str,
        _vector: &[f32],
        _sparse_vector: Option<&HashMap<String, f64>>,
        limit: usize,
        _filter: Option<&HashMap<String, serde_json::Value>>,
    ) -> Result<Vec<VectorHit>, BoxError> {
        let cols = self.collections.lock().unwrap();
        let records = cols.get(collection).cloned().unwrap_or_default();
        let hits: Vec<VectorHit> = records
            .iter()
            .take(limit)
            .enumerate()
            .map(|(i, rec)| VectorHit {
                id: rec
                    .get("id")
                    .and_then(|v| v.as_str())
                    .unwrap_or("unknown")
                    .to_owned(),
                score: 1.0 - (i as f64 * 0.1),
                fields: rec.clone(),
            })
            .collect();
        Ok(hits)
    }

    async fn upsert(
        &self,
        collection: &str,
        id: &str,
        _vector: &[f32],
        fields: HashMap<String, serde_json::Value>,
    ) -> Result<(), BoxError> {
        let mut cols = self.collections.lock().unwrap();
        let records = cols.entry(collection.to_owned()).or_default();
        // Remove existing with same ID
        records.retain(|r| {
            r.get("id").and_then(|v| v.as_str()) != Some(id)
        });
        let mut data = fields;
        data.insert("id".to_owned(), serde_json::json!(id));
        records.push(data);
        Ok(())
    }

    async fn update(
        &self,
        collection: &str,
        id: &str,
        fields: HashMap<String, serde_json::Value>,
    ) -> Result<(), BoxError> {
        let mut cols = self.collections.lock().unwrap();
        if let Some(records) = cols.get_mut(collection) {
            for rec in records.iter_mut() {
                if rec.get("id").and_then(|v| v.as_str()) == Some(id) {
                    for (k, v) in &fields {
                        rec.insert(k.clone(), v.clone());
                    }
                    return Ok(());
                }
            }
        }
        Err(format!("record not found: {id}").into())
    }

    async fn delete(&self, collection: &str, id: &str) -> Result<(), BoxError> {
        let mut cols = self.collections.lock().unwrap();
        if let Some(records) = cols.get_mut(collection) {
            records.retain(|r| {
                r.get("id").and_then(|v| v.as_str()) != Some(id)
            });
        }
        Ok(())
    }

    // === Collection management ===

    async fn create_collection(
        &self,
        name: &str,
        schema: &CollectionSchema,
    ) -> Result<bool, BoxError> {
        let mut schemas = self.schemas.lock().unwrap();
        if schemas.contains_key(name) {
            return Ok(false);
        }
        schemas.insert(name.to_owned(), schema.clone());
        self.collections
            .lock()
            .unwrap()
            .entry(name.to_owned())
            .or_default();
        Ok(true)
    }

    async fn drop_collection(&self, name: &str) -> Result<bool, BoxError> {
        let removed_schema = self.schemas.lock().unwrap().remove(name).is_some();
        let removed_data = self.collections.lock().unwrap().remove(name).is_some();
        Ok(removed_schema || removed_data)
    }

    async fn collection_exists(&self, name: &str) -> Result<bool, BoxError> {
        Ok(self.schemas.lock().unwrap().contains_key(name))
    }

    async fn list_collections(&self) -> Result<Vec<String>, BoxError> {
        let schemas = self.schemas.lock().unwrap();
        Ok(schemas.keys().cloned().collect())
    }

    async fn get_collection_info(
        &self,
        name: &str,
    ) -> Result<Option<CollectionInfo>, BoxError> {
        let schemas = self.schemas.lock().unwrap();
        let schema = match schemas.get(name) {
            Some(s) => s,
            None => return Ok(None),
        };
        let cols = self.collections.lock().unwrap();
        let count = cols.get(name).map(|r| r.len() as u64).unwrap_or(0);
        Ok(Some(CollectionInfo {
            name: name.to_owned(),
            vector_dim: schema.vector_dim,
            count,
            status: "ready".to_owned(),
        }))
    }

    // === Single record extensions ===

    async fn insert_record(
        &self,
        collection: &str,
        mut data: HashMap<String, serde_json::Value>,
    ) -> Result<String, BoxError> {
        let mut cols = self.collections.lock().unwrap();
        let records = cols.entry(collection.to_owned()).or_default();
        let id = data
            .get("id")
            .and_then(|v| v.as_str())
            .map(|s| s.to_owned())
            .unwrap_or_else(|| Self::next_id(records));
        data.insert("id".to_owned(), serde_json::json!(&id));
        records.push(data);
        Ok(id)
    }

    async fn get(
        &self,
        collection: &str,
        ids: &[String],
    ) -> Result<Vec<HashMap<String, serde_json::Value>>, BoxError> {
        let cols = self.collections.lock().unwrap();
        let records = cols.get(collection).cloned().unwrap_or_default();
        let found: Vec<_> = records
            .into_iter()
            .filter(|r| {
                r.get("id")
                    .and_then(|v| v.as_str())
                    .map(|id| ids.iter().any(|want| want == id))
                    .unwrap_or(false)
            })
            .collect();
        Ok(found)
    }

    async fn record_exists(
        &self,
        collection: &str,
        id: &str,
    ) -> Result<bool, BoxError> {
        let cols = self.collections.lock().unwrap();
        let exists = cols
            .get(collection)
            .map(|records| {
                records
                    .iter()
                    .any(|r| r.get("id").and_then(|v| v.as_str()) == Some(id))
            })
            .unwrap_or(false);
        Ok(exists)
    }

    // === Batch operations ===

    async fn batch_insert(
        &self,
        collection: &str,
        data: Vec<HashMap<String, serde_json::Value>>,
    ) -> Result<Vec<String>, BoxError> {
        let mut ids = Vec::new();
        for item in data {
            let id = self.insert_record(collection, item).await?;
            ids.push(id);
        }
        Ok(ids)
    }

    async fn batch_upsert(
        &self,
        collection: &str,
        data: Vec<HashMap<String, serde_json::Value>>,
    ) -> Result<Vec<String>, BoxError> {
        let mut ids = Vec::new();
        for item in data {
            let id = item
                .get("id")
                .and_then(|v| v.as_str())
                .unwrap_or("auto")
                .to_owned();
            self.upsert(collection, &id, &[], item.clone()).await?;
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
        if let Some(records) = cols.get_mut(collection) {
            let before = records.len();
            // Simple: delete by matching any filter key-value
            records.retain(|r| {
                !filter.iter().all(|(k, v)| r.get(k) == Some(v))
            });
            Ok((before - records.len()) as u64)
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
        if let Some(records) = cols.get_mut(collection) {
            let before = records.len();
            records.retain(|r| {
                let rec_uri = r.get("uri").and_then(|v| v.as_str()).unwrap_or("");
                rec_uri != uri && !rec_uri.starts_with(&format!("{uri}/"))
            });
            Ok((before - records.len()) as u64)
        } else {
            Ok(0)
        }
    }

    // === Advanced search ===

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
        // Delegate to core search for vector part
        let hits = self
            .search(
                collection,
                query_vector.unwrap_or(&[]),
                sparse_query_vector,
                limit + offset,
                filter,
            )
            .await?;
        let results: Vec<_> = hits
            .into_iter()
            .skip(offset)
            .take(limit)
            .map(|h| h.fields)
            .collect();
        Ok(results)
    }

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
        let records = cols.get(collection).cloned().unwrap_or_default();
        let filtered: Vec<_> = records
            .into_iter()
            .filter(|r| filter.iter().all(|(k, v)| r.get(k) == Some(v)))
            .skip(offset)
            .take(limit)
            .collect();
        Ok(filtered)
    }

    async fn scroll(
        &self,
        collection: &str,
        _filter: Option<&HashMap<String, serde_json::Value>>,
        limit: usize,
        cursor: Option<&str>,
        _output_fields: Option<&[String]>,
    ) -> Result<ScrollResult, BoxError> {
        let cols = self.collections.lock().unwrap();
        let records = cols.get(collection).cloned().unwrap_or_default();
        let start: usize = cursor
            .and_then(|c| c.parse().ok())
            .unwrap_or(0);
        let batch: Vec<_> = records
            .into_iter()
            .skip(start)
            .take(limit)
            .collect();
        let next = if batch.len() == limit {
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
        _filter: Option<&HashMap<String, serde_json::Value>>,
    ) -> Result<u64, BoxError> {
        let cols = self.collections.lock().unwrap();
        Ok(cols.get(collection).map(|r| r.len() as u64).unwrap_or(0))
    }

    // === Index (no-op in mock) ===

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
        if let Some(records) = cols.get_mut(collection) {
            records.clear();
            Ok(true)
        } else {
            Ok(false)
        }
    }

    async fn optimize(&self, _collection: &str) -> Result<bool, BoxError> {
        Ok(true)
    }

    async fn close(&self) -> Result<(), BoxError> {
        Ok(())
    }

    // === Health ===

    async fn health_check(&self) -> Result<bool, BoxError> {
        Ok(true)
    }

    async fn get_stats(&self) -> Result<HashMap<String, serde_json::Value>, BoxError> {
        let cols = self.collections.lock().unwrap();
        let total: usize = cols.values().map(|r| r.len()).sum();
        let mut stats = HashMap::new();
        stats.insert(
            "collections".to_owned(),
            serde_json::json!(cols.len()),
        );
        stats.insert(
            "total_records".to_owned(),
            serde_json::json!(total),
        );
        stats.insert(
            "backend".to_owned(),
            serde_json::json!("full_mock"),
        );
        Ok(stats)
    }
}

// ===========================================================================
// Tests
// ===========================================================================

#[tokio::test]
async fn test_create_and_list_collections() {
    let vs = FullMockVs::new();
    let schema = CollectionSchema::default();
    assert!(vs.create_collection("memory", &schema).await.unwrap());
    assert!(vs.create_collection("resource", &schema).await.unwrap());
    // Duplicate → false
    assert!(!vs.create_collection("memory", &schema).await.unwrap());

    let mut names = vs.list_collections().await.unwrap();
    names.sort();
    assert_eq!(names, vec!["memory", "resource"]);
}

#[tokio::test]
async fn test_drop_collection() {
    let vs = FullMockVs::new();
    let schema = CollectionSchema::default();
    vs.create_collection("tmp", &schema).await.unwrap();
    assert!(vs.drop_collection("tmp").await.unwrap());
    assert!(!vs.collection_exists("tmp").await.unwrap());
}

#[tokio::test]
async fn test_collection_exists() {
    let vs = FullMockVs::new();
    assert!(!vs.collection_exists("nope").await.unwrap());
    let schema = CollectionSchema::default();
    vs.create_collection("ctx", &schema).await.unwrap();
    assert!(vs.collection_exists("ctx").await.unwrap());
}

#[tokio::test]
async fn test_insert_and_get() {
    let vs = FullMockVs::new();
    let mut data = HashMap::new();
    data.insert("uri".to_owned(), serde_json::json!("viking://test"));
    data.insert("abstract".to_owned(), serde_json::json!("hello"));
    let id = vs.insert_record("ctx", data).await.unwrap();
    assert!(!id.is_empty());

    let results = vs.get("ctx", std::slice::from_ref(&id)).await.unwrap();
    assert_eq!(results.len(), 1);
    assert_eq!(
        results[0].get("abstract").and_then(|v| v.as_str()),
        Some("hello")
    );
}

#[tokio::test]
async fn test_record_exists() {
    let vs = FullMockVs::new();
    let mut data = HashMap::new();
    data.insert("id".to_owned(), serde_json::json!("r1"));
    vs.insert_record("ctx", data).await.unwrap();
    assert!(vs.record_exists("ctx", "r1").await.unwrap());
    assert!(!vs.record_exists("ctx", "r999").await.unwrap());
}

#[tokio::test]
async fn test_batch_insert() {
    let vs = FullMockVs::new();
    let items: Vec<HashMap<String, serde_json::Value>> = (0..5)
        .map(|i| {
            let mut m = HashMap::new();
            m.insert("uri".to_owned(), serde_json::json!(format!("v://r/{i}")));
            m
        })
        .collect();
    let ids = vs.batch_insert("ctx", items).await.unwrap();
    assert_eq!(ids.len(), 5);
    assert_eq!(vs.count("ctx", None).await.unwrap(), 5);
}

#[tokio::test]
async fn test_batch_delete() {
    let vs = FullMockVs::new();
    let mut d1 = HashMap::new();
    d1.insert("id".to_owned(), serde_json::json!("a"));
    d1.insert("type".to_owned(), serde_json::json!("mem"));
    let mut d2 = HashMap::new();
    d2.insert("id".to_owned(), serde_json::json!("b"));
    d2.insert("type".to_owned(), serde_json::json!("res"));
    vs.batch_insert("ctx", vec![d1, d2]).await.unwrap();

    let mut filter = HashMap::new();
    filter.insert("type".to_owned(), serde_json::json!("mem"));
    let deleted = vs.batch_delete("ctx", &filter).await.unwrap();
    assert_eq!(deleted, 1);
    assert_eq!(vs.count("ctx", None).await.unwrap(), 1);
}

#[tokio::test]
async fn test_remove_by_uri() {
    let vs = FullMockVs::new();
    let items = vec![
        {
            let mut m = HashMap::new();
            m.insert("id".to_owned(), serde_json::json!("1"));
            m.insert(
                "uri".to_owned(),
                serde_json::json!("viking://docs/readme"),
            );
            m
        },
        {
            let mut m = HashMap::new();
            m.insert("id".to_owned(), serde_json::json!("2"));
            m.insert(
                "uri".to_owned(),
                serde_json::json!("viking://docs/readme/s1"),
            );
            m
        },
        {
            let mut m = HashMap::new();
            m.insert("id".to_owned(), serde_json::json!("3"));
            m.insert("uri".to_owned(), serde_json::json!("viking://other"));
            m
        },
    ];
    vs.batch_insert("ctx", items).await.unwrap();

    let removed = vs
        .remove_by_uri("ctx", "viking://docs/readme")
        .await
        .unwrap();
    assert_eq!(removed, 2); // parent + child
    assert_eq!(vs.count("ctx", None).await.unwrap(), 1);
}

#[tokio::test]
async fn test_search_full_with_filter() {
    let vs = FullMockVs::new();
    for i in 0..10 {
        let mut d = HashMap::new();
        d.insert("id".to_owned(), serde_json::json!(format!("r{i}")));
        d.insert("uri".to_owned(), serde_json::json!(format!("v://r/{i}")));
        vs.insert_record("ctx", d).await.unwrap();
    }

    let results = vs
        .search_full("ctx", Some(&[0.1]), None, None, 3, 2, None, false)
        .await
        .unwrap();
    assert_eq!(results.len(), 3);
}

#[tokio::test]
async fn test_scroll_pagination() {
    let vs = FullMockVs::new();
    for i in 0..7 {
        let mut d = HashMap::new();
        d.insert("id".to_owned(), serde_json::json!(format!("s{i}")));
        vs.insert_record("ctx", d).await.unwrap();
    }

    // Page 1
    let page1 = vs.scroll("ctx", None, 3, None, None).await.unwrap();
    assert_eq!(page1.records.len(), 3);
    assert!(page1.next_cursor.is_some());

    // Page 2
    let page2 = vs
        .scroll("ctx", None, 3, page1.next_cursor.as_deref(), None)
        .await
        .unwrap();
    assert_eq!(page2.records.len(), 3);

    // Page 3 (last)
    let page3 = vs
        .scroll("ctx", None, 3, page2.next_cursor.as_deref(), None)
        .await
        .unwrap();
    assert_eq!(page3.records.len(), 1);
    assert!(page3.next_cursor.is_none());
}

#[tokio::test]
async fn test_count() {
    let vs = FullMockVs::new();
    assert_eq!(vs.count("empty", None).await.unwrap(), 0);
    let mut d = HashMap::new();
    d.insert("id".to_owned(), serde_json::json!("x"));
    vs.insert_record("ctx", d).await.unwrap();
    assert_eq!(vs.count("ctx", None).await.unwrap(), 1);
}

#[tokio::test]
async fn test_health_check() {
    let vs = FullMockVs::new();
    assert!(vs.health_check().await.unwrap());

    let stats = vs.get_stats().await.unwrap();
    assert_eq!(stats.get("backend").and_then(|v| v.as_str()), Some("full_mock"));
}

#[tokio::test]
async fn test_clear_and_optimize() {
    let vs = FullMockVs::new();
    let schema = CollectionSchema::default();
    vs.create_collection("ctx", &schema).await.unwrap();
    let mut d = HashMap::new();
    d.insert("id".to_owned(), serde_json::json!("z"));
    vs.insert_record("ctx", d).await.unwrap();
    assert_eq!(vs.count("ctx", None).await.unwrap(), 1);

    assert!(vs.clear("ctx").await.unwrap());
    assert_eq!(vs.count("ctx", None).await.unwrap(), 0);
    // Schema should still exist
    assert!(vs.collection_exists("ctx").await.unwrap());

    assert!(vs.optimize("ctx").await.unwrap());
}

#[tokio::test]
async fn test_get_collection_info() {
    let vs = FullMockVs::new();
    assert!(vs.get_collection_info("nope").await.unwrap().is_none());

    let schema = CollectionSchema { vector_dim: 1536, ..Default::default() };
    vs.create_collection("ctx", &schema).await.unwrap();
    let mut d = HashMap::new();
    d.insert("id".to_owned(), serde_json::json!("i1"));
    vs.insert_record("ctx", d).await.unwrap();

    let info = vs.get_collection_info("ctx").await.unwrap().unwrap();
    assert_eq!(info.name, "ctx");
    assert_eq!(info.vector_dim, 1536);
    assert_eq!(info.count, 1);
    assert_eq!(info.status, "ready");
}
