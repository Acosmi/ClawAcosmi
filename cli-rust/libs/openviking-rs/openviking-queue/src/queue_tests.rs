// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Unit tests for the openviking-queue crate.

use std::collections::HashMap;
use std::sync::{Arc, Mutex as StdMutex};

use async_trait::async_trait;
use openviking_session::traits::{
    BoxError, FileSystem, FsEntry, FsStat, GrepMatch,
};

use crate::converter::EmbeddingMsgConverter;
use crate::embedding_msg::{EmbeddingContent, EmbeddingMsg};
use crate::named_queue::NamedQueue;
use crate::queue_manager::QueueManager;
use crate::queue_types::QueueStatus;
use crate::semantic_msg::{SemanticMsg, SemanticMsgStatus};

// ---------------------------------------------------------------------------
// Mock FileSystem
// ---------------------------------------------------------------------------

#[derive(Debug, Clone)]
struct MockFs {
    files: Arc<StdMutex<HashMap<String, String>>>,
}

impl MockFs {
    fn new() -> Self {
        Self {
            files: Arc::new(StdMutex::new(HashMap::new())),
        }
    }
}

#[async_trait]
impl FileSystem for MockFs {
    async fn read(&self, uri: &str) -> Result<String, BoxError> {
        self.files
            .lock()
            .unwrap()
            .get(uri)
            .cloned()
            .ok_or_else(|| format!("not found: {uri}").into())
    }
    async fn read_bytes(&self, uri: &str) -> Result<Vec<u8>, BoxError> {
        Ok(self.read(uri).await?.into_bytes())
    }
    async fn write(&self, uri: &str, content: &str) -> Result<(), BoxError> {
        self.files
            .lock()
            .unwrap()
            .insert(uri.to_owned(), content.to_owned());
        Ok(())
    }
    async fn write_bytes(&self, uri: &str, content: &[u8]) -> Result<(), BoxError> {
        self.write(uri, &String::from_utf8_lossy(content)).await
    }
    async fn mkdir(&self, _uri: &str) -> Result<(), BoxError> {
        Ok(())
    }
    async fn ls(&self, _uri: &str) -> Result<Vec<FsEntry>, BoxError> {
        Ok(vec![])
    }
    async fn rm(&self, uri: &str) -> Result<(), BoxError> {
        self.files.lock().unwrap().remove(uri);
        Ok(())
    }
    async fn mv(&self, _from: &str, _to: &str) -> Result<(), BoxError> {
        Ok(())
    }
    async fn stat(&self, _uri: &str) -> Result<FsStat, BoxError> {
        Ok(FsStat {
            name: String::new(),
            size: 0,
            is_dir: false,
            mod_time: String::new(),
        })
    }
    async fn grep(
        &self,
        _uri: &str,
        _pattern: &str,
        _recursive: bool,
        _case_insensitive: bool,
    ) -> Result<Vec<GrepMatch>, BoxError> {
        Ok(vec![])
    }
    async fn exists(&self, uri: &str) -> Result<bool, BoxError> {
        Ok(self.files.lock().unwrap().contains_key(uri))
    }
    async fn append(&self, _uri: &str, _content: &str) -> Result<(), BoxError> {
        Ok(())
    }
    async fn link(&self, _source: &str, _target: &str) -> Result<(), BoxError> {
        Ok(())
    }
}

// ===========================================================================
// EmbeddingMsg tests
// ===========================================================================

#[test]
fn embedding_msg_serde_roundtrip() {
    let msg = EmbeddingMsg::new(
        EmbeddingContent::Text("hello world".to_owned()),
        serde_json::json!({"uri": "viking://resources/readme"}),
    );
    let json = msg.to_json().unwrap();
    let restored = EmbeddingMsg::from_json(&json).unwrap();
    assert_eq!(restored.id, msg.id);
    if let EmbeddingContent::Text(ref t) = restored.message {
        assert_eq!(t, "hello world");
    } else {
        panic!("expected Text variant");
    }
}

#[test]
fn embedding_msg_items_variant() {
    let items = vec![
        serde_json::json!({"type": "text", "content": "hello"}),
        serde_json::json!({"type": "image", "url": "http://img.png"}),
    ];
    let msg = EmbeddingMsg::new(
        EmbeddingContent::Items(items),
        serde_json::json!({}),
    );
    let json = msg.to_json().unwrap();
    let restored = EmbeddingMsg::from_json(&json).unwrap();
    if let EmbeddingContent::Items(ref v) = restored.message {
        assert_eq!(v.len(), 2);
    } else {
        panic!("expected Items variant");
    }
}

// ===========================================================================
// SemanticMsg tests
// ===========================================================================

#[test]
fn semantic_msg_serde_roundtrip() {
    let msg = SemanticMsg::new("viking://resources/docs", "resource", true);
    let json = msg.to_json().unwrap();
    let restored = SemanticMsg::from_json(&json).unwrap();
    assert_eq!(restored.uri, "viking://resources/docs");
    assert_eq!(restored.context_type, "resource");
    assert!(restored.recursive);
    assert_eq!(restored.status, SemanticMsgStatus::Pending);
}

#[test]
fn semantic_msg_missing_fields_uses_defaults() {
    // Minimal JSON with only required fields
    let json = r#"{"id":"abc","uri":"viking://x","context_type":"memory","timestamp":0}"#;
    let msg = SemanticMsg::from_json(json).unwrap();
    assert!(msg.recursive); // default true
    assert_eq!(msg.status, SemanticMsgStatus::Pending); // default
}

// ===========================================================================
// QueueStatus tests
// ===========================================================================

#[test]
fn queue_status_is_complete() {
    let status = QueueStatus {
        pending: 0,
        in_progress: 0,
        processed: 5,
        ..Default::default()
    };
    assert!(status.is_complete());
    assert!(!status.has_errors());
}

#[test]
fn queue_status_not_complete_if_pending() {
    let status = QueueStatus {
        pending: 1,
        ..Default::default()
    };
    assert!(!status.is_complete());
}

#[test]
fn queue_status_has_errors() {
    let status = QueueStatus {
        error_count: 3,
        ..Default::default()
    };
    assert!(status.has_errors());
}

// ===========================================================================
// EmbeddingMsgConverter tests
// ===========================================================================

#[test]
fn converter_from_context_with_text() {
    let ctx = openviking_core::Context::new(
        "viking://resources/readme",
        "A README file describing the project",
    );
    let msg = EmbeddingMsgConverter::from_context(&ctx);
    assert!(msg.is_some());
    let msg = msg.unwrap();
    if let EmbeddingContent::Text(ref t) = msg.message {
        assert!(t.contains("README"));
    } else {
        panic!("expected Text variant");
    }
}

#[test]
fn converter_from_context_empty_text() {
    let mut ctx = openviking_core::Context::new("viking://resources/empty", "");
    ctx.set_vectorize(openviking_core::Vectorize::new(""));
    let msg = EmbeddingMsgConverter::from_context(&ctx);
    assert!(msg.is_none());
}

// ===========================================================================
// NamedQueue tests (with MockFs)
// ===========================================================================

#[tokio::test]
async fn named_queue_enqueue_writes_to_fs() {
    let fs = Arc::new(MockFs::new());
    let mut q = NamedQueue::new(Arc::clone(&fs), "/queue", "test", None, None);

    q.enqueue(serde_json::json!({"hello": "world"}))
        .await
        .unwrap();

    // Verify the enqueue virtual file was written
    let content = fs.read("/queue/test/enqueue").await.unwrap();
    assert!(content.contains("hello"));
}

#[tokio::test]
async fn named_queue_clear_returns_true() {
    let fs = Arc::new(MockFs::new());
    let mut q = NamedQueue::new(Arc::clone(&fs), "/queue", "test", None, None);
    assert!(q.clear().await);
}

// ===========================================================================
// QueueManager tests
// ===========================================================================

#[tokio::test]
async fn queue_manager_create_and_get_queue() {
    let fs = Arc::new(MockFs::new());
    let mut mgr = QueueManager::new(Arc::clone(&fs), "/queue");

    let q1 = mgr.get_or_create_queue("Embedding", None, None).await;
    let q2 = mgr.get_or_create_queue("Embedding", None, None).await;

    // Should return the same queue
    let name1 = q1.lock().await.name.clone();
    let name2 = q2.lock().await.name.clone();
    assert_eq!(name1, "Embedding");
    assert_eq!(name2, "Embedding");
}
