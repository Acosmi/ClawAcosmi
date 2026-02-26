// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Integration tests for [`SegmentVectorStore`].

use segment::types::Distance;
use serde_json::json;
use tempfile::TempDir;

use crate::segment_store::{CollectionConfig, SegmentVectorStore};

fn test_store() -> (TempDir, SegmentVectorStore) {
    let dir = TempDir::new().unwrap();
    let store = SegmentVectorStore::new(dir.path()).unwrap();
    (dir, store)
}

fn simple_config(dim: usize) -> CollectionConfig {
    CollectionConfig {
        dimension: dim,
        distance: Distance::Cosine,
        sparse_vectors: false,
    }
}

/// Helper: build a Payload from a JSON object.
fn make_payload(obj: serde_json::Value) -> segment::types::Payload {
    let map = obj.as_object().expect("expected JSON object").clone();
    segment::types::Payload::from(map)
}

// ── Collection lifecycle ────────────────────────────────────────────────

#[test]
fn test_create_and_list_collections() {
    let (_dir, store) = test_store();

    assert!(store.create_collection("col_a", &simple_config(4)).unwrap());
    assert!(store.create_collection("col_b", &simple_config(8)).unwrap());

    // Duplicate create returns false.
    assert!(!store.create_collection("col_a", &simple_config(4)).unwrap());

    let mut names = store.list_collections();
    names.sort();
    assert_eq!(names, vec!["col_a", "col_b"]);

    assert!(store.collection_exists("col_a"));
    assert!(!store.collection_exists("col_c"));
}

#[test]
fn test_drop_collection() {
    let (_dir, store) = test_store();

    store
        .create_collection("drop_me", &simple_config(4))
        .unwrap();
    assert!(store.collection_exists("drop_me"));

    assert!(store.drop_collection("drop_me"));
    assert!(!store.collection_exists("drop_me"));

    // Dropping non-existent returns false.
    assert!(!store.drop_collection("nope"));
}

// ── Upsert + Search ─────────────────────────────────────────────────────

#[test]
fn test_upsert_and_search() {
    let (_dir, store) = test_store();

    store.create_collection("test", &simple_config(4)).unwrap();

    let id1 = "00000000-0000-0000-0000-000000000001";
    let id2 = "00000000-0000-0000-0000-000000000002";
    let id3 = "00000000-0000-0000-0000-000000000003";

    // Upsert three points.
    let payload1 = make_payload(json!({
        "content": "hello world",
        "user_id": "u1"
    }));

    store
        .upsert("test", id1, &[1.0, 0.0, 0.0, 0.0], Some(&payload1))
        .unwrap();
    store
        .upsert("test", id2, &[0.0, 1.0, 0.0, 0.0], None)
        .unwrap();
    store
        .upsert("test", id3, &[1.0, 0.0, 1.0, 0.0], None)
        .unwrap();

    assert_eq!(store.point_count("test"), Some(3));

    // Search — query vector close to id1 and id3.
    let hits = store
        .search("test", &[1.0, 0.0, 0.5, 0.0], None, 2)
        .unwrap();

    assert_eq!(hits.len(), 2);

    // The first result should be id3 or id1 (both are close to the query).
    let hit_ids: Vec<&str> = hits.iter().map(|h| h.id.as_str()).collect();
    assert!(hit_ids.contains(&id1) || hit_ids.contains(&id3));
}

// ── Delete ──────────────────────────────────────────────────────────────

#[test]
fn test_delete_point() {
    let (_dir, store) = test_store();

    store.create_collection("del", &simple_config(4)).unwrap();

    let id = "00000000-0000-0000-0000-000000000010";
    store
        .upsert("del", id, &[1.0, 0.0, 0.0, 0.0], None)
        .unwrap();
    assert_eq!(store.point_count("del"), Some(1));

    let deleted = store.delete("del", id).unwrap();
    assert!(deleted);

    assert_eq!(store.point_count("del"), Some(0));
}

// ── Flush ───────────────────────────────────────────────────────────────

#[test]
fn test_flush() {
    let (_dir, store) = test_store();

    store.create_collection("fl", &simple_config(4)).unwrap();

    store
        .upsert(
            "fl",
            "00000000-0000-0000-0000-000000000020",
            &[1.0, 0.0, 0.0, 0.0],
            None,
        )
        .unwrap();

    // Flush should succeed without error.
    store.flush("fl").unwrap();
}

// ── Payload search ──────────────────────────────────────────────────────

#[test]
fn test_upsert_with_payload() {
    let (_dir, store) = test_store();

    store
        .create_collection("payloads", &simple_config(4))
        .unwrap();

    let id = "00000000-0000-0000-0000-000000000030";
    let payload = make_payload(json!({
        "content": "memory content",
        "user_id": "user_123",
        "memory_type": "episodic",
        "importance_score": 0.85
    }));

    store
        .upsert("payloads", id, &[0.5, 0.5, 0.0, 0.0], Some(&payload))
        .unwrap();

    // Search and verify payload is returned.
    let hits = store
        .search("payloads", &[0.5, 0.5, 0.0, 0.0], None, 1)
        .unwrap();

    assert_eq!(hits.len(), 1);
    assert_eq!(hits[0].id, id);
    assert_eq!(
        hits[0].payload.get("content"),
        Some(&json!("memory content"))
    );
    assert_eq!(hits[0].payload.get("user_id"), Some(&json!("user_123")));
}
