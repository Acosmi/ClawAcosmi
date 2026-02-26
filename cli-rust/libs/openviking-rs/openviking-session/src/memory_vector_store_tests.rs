// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Unit tests for `InMemoryVectorStore`.

use std::collections::HashMap;

use crate::memory_vector_store::InMemoryVectorStore;
use crate::traits::{CollectionSchema, VectorStore};

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

fn default_schema() -> CollectionSchema {
    CollectionSchema {
        vector_dim: 3,
        ..Default::default()
    }
}

fn make_record(id: &str, uri: &str, vector: &[f32]) -> (String, Vec<f32>, HashMap<String, serde_json::Value>) {
    let mut fields = HashMap::new();
    fields.insert("id".to_owned(), serde_json::json!(id));
    fields.insert("uri".to_owned(), serde_json::json!(uri));
    (id.to_owned(), vector.to_vec(), fields)
}

// ---------------------------------------------------------------------------
// 1. Collection management
// ---------------------------------------------------------------------------

#[tokio::test]
async fn test_create_and_list_collections() {
    let vs = InMemoryVectorStore::new();
    let schema = default_schema();
    assert!(vs.create_collection("ctx", &schema).await.unwrap());
    assert!(vs.create_collection("res", &schema).await.unwrap());
    // Duplicate → false
    assert!(!vs.create_collection("ctx", &schema).await.unwrap());

    let mut names = vs.list_collections().await.unwrap();
    names.sort();
    assert_eq!(names, vec!["ctx", "res"]);
}

#[tokio::test]
async fn test_drop_collection() {
    let vs = InMemoryVectorStore::new();
    vs.create_collection("tmp", &default_schema()).await.unwrap();
    assert!(vs.drop_collection("tmp").await.unwrap());
    assert!(!vs.collection_exists("tmp").await.unwrap());
    // Double drop → false
    assert!(!vs.drop_collection("tmp").await.unwrap());
}

#[tokio::test]
async fn test_get_collection_info() {
    let vs = InMemoryVectorStore::new();
    assert!(vs.get_collection_info("nope").await.unwrap().is_none());

    let schema = CollectionSchema { vector_dim: 768, ..Default::default() };
    vs.create_collection("ctx", &schema).await.unwrap();
    let (id, vec, fields) = make_record("r1", "viking://test", &[1.0, 0.0, 0.0]);
    vs.upsert("ctx", &id, &vec, fields).await.unwrap();

    let info = vs.get_collection_info("ctx").await.unwrap().unwrap();
    assert_eq!(info.name, "ctx");
    assert_eq!(info.vector_dim, 768);
    assert_eq!(info.count, 1);
}

// ---------------------------------------------------------------------------
// 2. Basic CRUD
// ---------------------------------------------------------------------------

#[tokio::test]
async fn test_upsert_and_get() {
    let vs = InMemoryVectorStore::new();
    let (id, vec, fields) = make_record("r1", "viking://docs/a", &[1.0, 0.0, 0.0]);
    vs.upsert("ctx", &id, &vec, fields).await.unwrap();

    let results = vs.get("ctx", &["r1".to_owned()]).await.unwrap();
    assert_eq!(results.len(), 1);
    assert_eq!(results[0].get("uri").and_then(|v| v.as_str()), Some("viking://docs/a"));
}

#[tokio::test]
async fn test_update_fields() {
    let vs = InMemoryVectorStore::new();
    let (id, vec, fields) = make_record("r1", "viking://a", &[1.0, 0.0, 0.0]);
    vs.upsert("ctx", &id, &vec, fields).await.unwrap();

    let mut update = HashMap::new();
    update.insert("abstract".to_owned(), serde_json::json!("updated text"));
    vs.update("ctx", "r1", update).await.unwrap();

    let results = vs.get("ctx", &["r1".to_owned()]).await.unwrap();
    assert_eq!(
        results[0].get("abstract").and_then(|v| v.as_str()),
        Some("updated text")
    );
}

#[tokio::test]
async fn test_delete_and_record_exists() {
    let vs = InMemoryVectorStore::new();
    let (id, vec, fields) = make_record("r1", "viking://a", &[1.0, 0.0, 0.0]);
    vs.upsert("ctx", &id, &vec, fields).await.unwrap();

    assert!(vs.record_exists("ctx", "r1").await.unwrap());
    vs.delete("ctx", "r1").await.unwrap();
    assert!(!vs.record_exists("ctx", "r1").await.unwrap());
}

// ---------------------------------------------------------------------------
// 3. Cosine search ordering
// ---------------------------------------------------------------------------

#[tokio::test]
async fn test_search_cosine_ordering() {
    let vs = InMemoryVectorStore::new();
    // Insert 3 records with known vectors
    let (id1, v1, f1) = make_record("r1", "v://a", &[1.0, 0.0, 0.0]);
    let (id2, v2, f2) = make_record("r2", "v://b", &[0.7, 0.7, 0.0]);
    let (id3, v3, f3) = make_record("r3", "v://c", &[0.0, 0.0, 1.0]);
    vs.upsert("ctx", &id1, &v1, f1).await.unwrap();
    vs.upsert("ctx", &id2, &v2, f2).await.unwrap();
    vs.upsert("ctx", &id3, &v3, f3).await.unwrap();

    // Query with [1, 0, 0] → r1 should be first (cos=1.0), r2 second, r3 last (cos=0.0)
    let query = [1.0_f32, 0.0, 0.0];
    let hits = vs.search("ctx", &query, None, 3, None).await.unwrap();
    assert_eq!(hits.len(), 3);
    assert_eq!(hits[0].id, "r1");
    assert!((hits[0].score - 1.0).abs() < 1e-4);
    assert_eq!(hits[1].id, "r2");
    assert_eq!(hits[2].id, "r3");
    assert!(hits[2].score.abs() < 1e-4);
}

#[tokio::test]
async fn test_search_with_limit() {
    let vs = InMemoryVectorStore::new();
    for i in 0..5 {
        let (id, v, f) = make_record(&format!("r{i}"), &format!("v://{i}"), &[1.0, 0.0, 0.0]);
        vs.upsert("ctx", &id, &v, f).await.unwrap();
    }
    let hits = vs.search("ctx", &[1.0, 0.0, 0.0], None, 2, None).await.unwrap();
    assert_eq!(hits.len(), 2);
}

// ---------------------------------------------------------------------------
// 4. Filter DSL
// ---------------------------------------------------------------------------

#[tokio::test]
async fn test_filter_must() {
    let vs = InMemoryVectorStore::new();
    let mut f1 = HashMap::new();
    f1.insert("id".to_owned(), serde_json::json!("r1"));
    f1.insert("type".to_owned(), serde_json::json!("memory"));
    vs.upsert("ctx", "r1", &[1.0, 0.0, 0.0], f1).await.unwrap();

    let mut f2 = HashMap::new();
    f2.insert("id".to_owned(), serde_json::json!("r2"));
    f2.insert("type".to_owned(), serde_json::json!("resource"));
    vs.upsert("ctx", "r2", &[0.0, 1.0, 0.0], f2).await.unwrap();

    // must: type == "memory"
    let mut filter = HashMap::new();
    filter.insert(
        "must".to_owned(),
        serde_json::json!({"field": "type", "conds": ["memory"]}),
    );
    let hits = vs.search("ctx", &[1.0, 0.0, 0.0], None, 10, Some(&filter)).await.unwrap();
    assert_eq!(hits.len(), 1);
    assert_eq!(hits[0].id, "r1");
}

#[tokio::test]
async fn test_filter_must_not() {
    let vs = InMemoryVectorStore::new();
    let mut f1 = HashMap::new();
    f1.insert("id".to_owned(), serde_json::json!("r1"));
    f1.insert("type".to_owned(), serde_json::json!("memory"));
    vs.upsert("ctx", "r1", &[1.0, 0.0, 0.0], f1).await.unwrap();

    let mut f2 = HashMap::new();
    f2.insert("id".to_owned(), serde_json::json!("r2"));
    f2.insert("type".to_owned(), serde_json::json!("resource"));
    vs.upsert("ctx", "r2", &[0.0, 1.0, 0.0], f2).await.unwrap();

    // must_not: type == "memory" → only r2 remains
    let mut filter = HashMap::new();
    filter.insert(
        "must_not".to_owned(),
        serde_json::json!({"field": "type", "conds": ["memory"]}),
    );
    let hits = vs.search("ctx", &[1.0, 0.0, 0.0], None, 10, Some(&filter)).await.unwrap();
    assert_eq!(hits.len(), 1);
    assert_eq!(hits[0].id, "r2");
}

// ---------------------------------------------------------------------------
// 5. remove_by_uri
// ---------------------------------------------------------------------------

#[tokio::test]
async fn test_remove_by_uri() {
    let vs = InMemoryVectorStore::new();
    vs.upsert("ctx", "r1", &[1.0, 0.0, 0.0], {
        let mut m = HashMap::new();
        m.insert("uri".to_owned(), serde_json::json!("viking://docs/readme"));
        m
    }).await.unwrap();
    vs.upsert("ctx", "r2", &[1.0, 0.0, 0.0], {
        let mut m = HashMap::new();
        m.insert("uri".to_owned(), serde_json::json!("viking://docs/readme/s1"));
        m
    }).await.unwrap();
    vs.upsert("ctx", "r3", &[1.0, 0.0, 0.0], {
        let mut m = HashMap::new();
        m.insert("uri".to_owned(), serde_json::json!("viking://other"));
        m
    }).await.unwrap();

    let removed = vs.remove_by_uri("ctx", "viking://docs/readme").await.unwrap();
    assert_eq!(removed, 2); // parent + child
    assert_eq!(vs.count("ctx", None).await.unwrap(), 1);
}

// ---------------------------------------------------------------------------
// 6. Count with filter
// ---------------------------------------------------------------------------

#[tokio::test]
async fn test_count_with_filter() {
    let vs = InMemoryVectorStore::new();
    for i in 0..5 {
        let mut f = HashMap::new();
        f.insert("type".to_owned(), serde_json::json!(if i < 3 { "a" } else { "b" }));
        vs.upsert("ctx", &format!("r{i}"), &[1.0, 0.0, 0.0], f).await.unwrap();
    }
    assert_eq!(vs.count("ctx", None).await.unwrap(), 5);

    let mut filter = HashMap::new();
    filter.insert("type".to_owned(), serde_json::json!("a"));
    assert_eq!(vs.count("ctx", Some(&filter)).await.unwrap(), 3);
}

// ---------------------------------------------------------------------------
// 7. Clear & Optimize
// ---------------------------------------------------------------------------

#[tokio::test]
async fn test_clear_and_optimize() {
    let vs = InMemoryVectorStore::new();
    vs.create_collection("ctx", &default_schema()).await.unwrap();
    let (id, vec, fields) = make_record("r1", "v://a", &[1.0, 0.0, 0.0]);
    vs.upsert("ctx", &id, &vec, fields).await.unwrap();

    assert!(vs.clear("ctx").await.unwrap());
    assert_eq!(vs.count("ctx", None).await.unwrap(), 0);
    assert!(vs.collection_exists("ctx").await.unwrap()); // schema preserved

    assert!(vs.optimize("ctx").await.unwrap());
}

// ---------------------------------------------------------------------------
// 8. Health & Stats
// ---------------------------------------------------------------------------

#[tokio::test]
async fn test_health_and_stats() {
    let vs = InMemoryVectorStore::new();
    assert!(vs.health_check().await.unwrap());

    let (id, vec, fields) = make_record("r1", "v://a", &[1.0, 0.0, 0.0]);
    vs.upsert("ctx", &id, &vec, fields).await.unwrap();

    let stats = vs.get_stats().await.unwrap();
    assert_eq!(stats.get("backend").and_then(|v| v.as_str()), Some("in_memory"));
    assert_eq!(stats.get("total_records").and_then(|v| v.as_u64()), Some(1));
}

// ---------------------------------------------------------------------------
// 9. Batch delete by filter
// ---------------------------------------------------------------------------

#[tokio::test]
async fn test_batch_delete() {
    let vs = InMemoryVectorStore::new();
    let mut d1 = HashMap::new();
    d1.insert("type".to_owned(), serde_json::json!("mem"));
    vs.upsert("ctx", "a", &[1.0, 0.0, 0.0], d1).await.unwrap();

    let mut d2 = HashMap::new();
    d2.insert("type".to_owned(), serde_json::json!("res"));
    vs.upsert("ctx", "b", &[0.0, 1.0, 0.0], d2).await.unwrap();

    let mut filter = HashMap::new();
    filter.insert("type".to_owned(), serde_json::json!("mem"));
    let deleted = vs.batch_delete("ctx", &filter).await.unwrap();
    assert_eq!(deleted, 1);
    assert_eq!(vs.count("ctx", None).await.unwrap(), 1);
}
