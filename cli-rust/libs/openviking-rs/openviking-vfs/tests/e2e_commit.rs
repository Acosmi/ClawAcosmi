// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! End-to-end integration tests for the OpenViking session pipeline.
//!
//! Validates the full `Session::commit()` flow using real `LocalFs` +
//! `InMemoryVectorStore` + mock LLM/Embedder — proving that all Phase 3
//! components integrate correctly.

use async_trait::async_trait;
use tempfile::TempDir;

use openviking_core::message::Message;
use openviking_core::user::UserIdentifier;

use openviking_session::traits::{BoxError, EmbedResult, Embedder, LlmProvider};
use openviking_session::{init_context_collection, InMemoryVectorStore};

use openviking_vfs::{DirectoryInitializer, LocalFs, VikingFs};

// ---------------------------------------------------------------------------
// Mock types
// ---------------------------------------------------------------------------

/// Mock LLM that returns a predictable structured summary.
struct MockLlm;

#[async_trait]
impl LlmProvider for MockLlm {
    async fn completion(&self, _prompt: &str) -> Result<String, BoxError> {
        Ok("**Overview**: Mock summary of the conversation\n\n\
            The user discussed Rust porting and memory systems."
            .to_owned())
    }
}

/// Mock embedder that returns a fixed 768-dimensional dense vector.
struct MockEmbedder;

#[async_trait]
impl Embedder for MockEmbedder {
    async fn embed(&self, _text: &str) -> Result<EmbedResult, BoxError> {
        Ok(EmbedResult {
            dense_vector: vec![0.1_f32; 768],
            sparse_vector: None,
        })
    }
}

// ---------------------------------------------------------------------------
// Helper: build the full pipeline
// ---------------------------------------------------------------------------

const COLLECTION_NAME: &str = "context";
const VECTOR_DIM: usize = 768;

/// Assemble `LocalFs(tempdir)` + `InMemoryVectorStore` + `MockEmbedder` into
/// a fully initialized `VikingFs`, with the context collection and preset
/// directories already created.
///
/// Returns `(tempdir, vfs, vector_store)`.
///
/// `vector_store` is a **second** `InMemoryVectorStore` that shares no state
/// with the one inside `vfs`.  For assertions on vector data, use the helper
/// `assert_vector_store_*` functions which construct their own store.
///
/// The `TempDir` handle is returned to keep the directory alive for the
/// duration of the test.
async fn build_pipeline() -> (
    TempDir,
    VikingFs<LocalFs, InMemoryVectorStore, MockEmbedder>,
    InMemoryVectorStore,
) {
    let tmp = TempDir::new().expect("failed to create tempdir");
    let local_fs = LocalFs::new(tmp.path());
    let vs = InMemoryVectorStore::new();

    // Initialize context collection
    init_context_collection(&vs, COLLECTION_NAME, VECTOR_DIM)
        .await
        .expect("init_context_collection failed");

    let vfs = VikingFs::with_backends(local_fs, vs, MockEmbedder);

    // Initialize preset directory tree
    let di = DirectoryInitializer::new(&vfs, COLLECTION_NAME);
    di.initialize_all()
        .await
        .expect("DirectoryInitializer::initialize_all failed");

    // Build a fresh VS for independent assertions (the original was moved
    // into VikingFs).
    let assert_vs = InMemoryVectorStore::new();
    init_context_collection(&assert_vs, COLLECTION_NAME, VECTOR_DIM)
        .await
        .expect("init assert_vs collection");

    (tmp, vfs, assert_vs)
}

// ---------------------------------------------------------------------------
// Test 1: DirectoryInitializer creates preset directories on disk
// ---------------------------------------------------------------------------

#[tokio::test]
async fn e2e_directory_init_creates_preset_dirs() {
    let (tmp, _vfs, _vs) = build_pipeline().await;
    let root = tmp.path();

    // The session scope should exist as directories on disk
    assert!(
        root.join("session").exists(),
        "session directory should exist"
    );

    // Agent scope
    assert!(root.join("agent").exists(), "agent directory should exist");

    // Resources scope
    assert!(
        root.join("resources").exists(),
        "resources directory should exist"
    );
}

// ---------------------------------------------------------------------------
// Test 2: init_context_collection creates the collection
// ---------------------------------------------------------------------------

#[tokio::test]
async fn e2e_vector_store_has_collection() {
    let vs = InMemoryVectorStore::new();
    let created = init_context_collection(&vs, COLLECTION_NAME, VECTOR_DIM)
        .await
        .expect("init_context_collection failed");
    assert!(created, "collection should be newly created");

    let exists = vs
        .collection_exists(COLLECTION_NAME)
        .await
        .expect("collection_exists failed");
    assert!(exists, "context collection should exist");

    // Idempotent: second call returns false
    let second = init_context_collection(&vs, COLLECTION_NAME, VECTOR_DIM)
        .await
        .expect("init_context_collection idempotent");
    assert!(!second, "collection already exists — should return false");
}

// need VectorStore trait in scope for .collection_exists()
use openviking_session::traits::VectorStore;

// ---------------------------------------------------------------------------
// Test 3: Session commit writes archive files to disk
// ---------------------------------------------------------------------------

#[tokio::test]
async fn e2e_session_commit_writes_archive() {
    use openviking_session::session::Session;

    let (tmp, vfs, _vs) = build_pipeline().await;

    let mut session = Session::new(
        vfs,
        MockLlm,
        UserIdentifier::default_user(),
        "e2e-test-session".to_owned(),
    );

    // Add two messages
    session
        .add_message(Message::create_user("用户偏好: 喜欢 Rust"))
        .await
        .expect("add_message 1");
    session
        .add_message(Message::create_user(
            "实体: OpenViking 是火山引擎开源的记忆系统",
        ))
        .await
        .expect("add_message 2");

    // Commit
    let result = session.commit().await.expect("commit failed");

    assert!(result.archived, "messages should be archived");
    assert_eq!(result.status, "committed");

    // Verify archive files exist on disk
    let archive_dir = tmp
        .path()
        .join("session/e2e-test-session/history/archive_001");
    assert!(
        archive_dir.exists(),
        "archive_001 directory should exist at {:?}",
        archive_dir
    );
    assert!(
        archive_dir.join("messages.jsonl").exists(),
        "messages.jsonl should exist in archive"
    );
}

// ---------------------------------------------------------------------------
// Test 4: Session commit writes abstract file
// ---------------------------------------------------------------------------

#[tokio::test]
async fn e2e_session_commit_writes_abstract() {
    use openviking_session::session::Session;

    let (tmp, vfs, _vs) = build_pipeline().await;

    let mut session = Session::new(
        vfs,
        MockLlm,
        UserIdentifier::default_user(),
        "abstract-test".to_owned(),
    );

    session
        .add_message(Message::create_user("Testing abstract generation"))
        .await
        .unwrap();

    session.commit().await.expect("commit failed");

    let abstract_path = tmp
        .path()
        .join("session/abstract-test/history/archive_001/.abstract.md");
    assert!(
        abstract_path.exists(),
        ".abstract.md should exist at {:?}",
        abstract_path
    );

    let content = std::fs::read_to_string(&abstract_path).expect("read abstract");
    assert!(
        !content.is_empty(),
        "abstract file should have content"
    );
}

// ---------------------------------------------------------------------------
// Test 5: Session commit writes overview file
// ---------------------------------------------------------------------------

#[tokio::test]
async fn e2e_session_commit_writes_overview() {
    use openviking_session::session::Session;

    let (tmp, vfs, _vs) = build_pipeline().await;

    let mut session = Session::new(
        vfs,
        MockLlm,
        UserIdentifier::default_user(),
        "overview-test".to_owned(),
    );

    session
        .add_message(Message::create_user("Testing overview generation"))
        .await
        .unwrap();

    session.commit().await.expect("commit failed");

    let overview_path = tmp
        .path()
        .join("session/overview-test/history/archive_001/.overview.md");
    assert!(
        overview_path.exists(),
        ".overview.md should exist at {:?}",
        overview_path
    );

    let content = std::fs::read_to_string(&overview_path).expect("read overview");
    assert!(
        content.contains("Mock summary"),
        "overview should contain LLM-generated summary"
    );
}

// ---------------------------------------------------------------------------
// Test 6: Session commit updates stats correctly
// ---------------------------------------------------------------------------

#[tokio::test]
async fn e2e_session_commit_updates_stats() {
    use openviking_session::session::Session;

    let (_tmp, vfs, _vs) = build_pipeline().await;

    let mut session = Session::new(
        vfs,
        MockLlm,
        UserIdentifier::default_user(),
        "stats-test".to_owned(),
    );

    session
        .add_message(Message::create_user("Message 1"))
        .await
        .unwrap();
    session
        .add_message(Message::create_user("Message 2"))
        .await
        .unwrap();

    assert_eq!(session.stats().total_turns, 2);
    assert_eq!(session.messages().len(), 2);

    let result = session.commit().await.expect("commit failed");

    // After commit, messages are cleared
    assert!(session.messages().is_empty(), "messages should be cleared");

    // Compression index advanced
    assert_eq!(session.compression().compression_index, 1);

    // Stats in result
    let stats = result.stats.expect("stats should be present");
    assert_eq!(stats.compression_count, 1);
}

// ---------------------------------------------------------------------------
// Test 7: Full pipeline — init → add × 2 → commit → file + stats assertions
// ---------------------------------------------------------------------------

#[tokio::test]
async fn e2e_full_pipeline() {
    use openviking_session::session::Session;

    let (tmp, vfs, _vs) = build_pipeline().await;
    let root = tmp.path();

    // --- Phase 1: Verify init created directories ---
    assert!(root.join("session").is_dir());
    assert!(root.join("resources").is_dir());

    // --- Phase 2: Session lifecycle ---
    let mut session = Session::new(
        vfs,
        MockLlm,
        UserIdentifier::default_user(),
        "full-e2e".to_owned(),
    );

    // Add messages
    let should_commit_1 = session
        .add_message(Message::create_user("用户偏好: 喜欢 Rust"))
        .await
        .expect("add_message 1");
    assert!(
        !should_commit_1,
        "should not auto-commit with only 1 message"
    );

    let should_commit_2 = session
        .add_message(Message::create_user(
            "实体: OpenViking 是火山引擎开源的记忆系统",
        ))
        .await
        .expect("add_message 2");
    assert!(
        !should_commit_2,
        "should not auto-commit with only 2 messages"
    );

    // Pre-commit assertions
    assert_eq!(session.messages().len(), 2);
    assert_eq!(session.stats().total_turns, 2);

    // --- Phase 3: Commit ---
    let result = session.commit().await.expect("commit failed");

    assert!(result.archived);
    assert_eq!(result.status, "committed");
    assert!(result.stats.is_some());

    // Post-commit: messages cleared
    assert!(session.messages().is_empty());
    assert_eq!(session.compression().compression_index, 1);

    // --- Phase 4: File system assertions ---
    let session_dir = root.join("session/full-e2e");
    assert!(session_dir.exists(), "session directory should exist");

    // Archive
    let archive_dir = session_dir.join("history/archive_001");
    assert!(archive_dir.exists(), "archive directory should exist");

    // messages.jsonl in archive
    let archive_messages = archive_dir.join("messages.jsonl");
    assert!(archive_messages.exists(), "archive messages.jsonl");
    let archive_content =
        std::fs::read_to_string(&archive_messages).expect("read archive messages");
    assert!(
        archive_content.contains("喜欢 Rust"),
        "archive should contain first message"
    );
    assert!(
        archive_content.contains("OpenViking"),
        "archive should contain second message"
    );

    // L0 abstract
    let abstract_file = archive_dir.join(".abstract.md");
    assert!(abstract_file.exists(), "L0 abstract should exist");
    let abstract_content = std::fs::read_to_string(&abstract_file).expect("read abstract");
    assert!(
        !abstract_content.is_empty(),
        "abstract should have content"
    );

    // L1 overview
    let overview_file = archive_dir.join(".overview.md");
    assert!(overview_file.exists(), "L1 overview should exist");
    let overview_content = std::fs::read_to_string(&overview_file).expect("read overview");
    assert!(
        overview_content.contains("Mock summary"),
        "overview should contain LLM summary"
    );

    // --- Phase 5: Current session state files ---
    let current_messages = session_dir.join("messages.jsonl");
    assert!(
        current_messages.exists(),
        "current messages.jsonl should exist"
    );

    // Current messages should be empty (cleared after commit)
    let current_content =
        std::fs::read_to_string(&current_messages).expect("read current messages");
    // After commit, write_to_agfs writes the cleared messages list
    assert!(
        current_content.trim().is_empty(),
        "current messages should be empty after commit"
    );

    // Session-level L0/L1
    let session_abstract = session_dir.join(".abstract.md");
    assert!(
        session_abstract.exists(),
        "session .abstract.md should exist"
    );

    let session_overview = session_dir.join(".overview.md");
    assert!(
        session_overview.exists(),
        "session .overview.md should exist"
    );
}
