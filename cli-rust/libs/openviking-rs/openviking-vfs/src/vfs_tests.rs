// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Tests for VikingFs.

use std::collections::HashMap;
use std::sync::{Arc, Mutex};

use async_trait::async_trait;

use openviking_session::traits::{
    BoxError, EmbedResult, Embedder, FileSystem, FsEntry, FsStat, GrepMatch, VectorHit,
    VectorStore,
};

use crate::viking_fs::VikingFs;

// ===========================================================================
// Mock implementations
// ===========================================================================

/// In-memory mock file system.
#[derive(Clone)]
struct MockFs {
    store: Arc<Mutex<HashMap<String, MockEntry>>>,
}

#[derive(Clone)]
enum MockEntry {
    File(String),
    Dir,
}

impl MockFs {
    fn new() -> Self {
        Self {
            store: Arc::new(Mutex::new(HashMap::new())),
        }
    }

    fn with_files(files: &[(&str, &str)]) -> Self {
        let fs = Self::new();
        let mut s = fs.store.lock().unwrap();
        for (path, content) in files {
            s.insert((*path).to_owned(), MockEntry::File((*content).to_owned()));
        }
        drop(s);
        fs
    }

    fn with_entries(entries: &[(&str, MockEntry)]) -> Self {
        let fs = Self::new();
        let mut s = fs.store.lock().unwrap();
        for (path, entry) in entries {
            s.insert((*path).to_owned(), entry.clone());
        }
        drop(s);
        fs
    }
}

#[async_trait]
impl FileSystem for MockFs {
    async fn read(&self, uri: &str) -> Result<String, BoxError> {
        let s = self.store.lock().unwrap();
        match s.get(uri) {
            Some(MockEntry::File(content)) => Ok(content.clone()),
            _ => Err(format!("not found: {uri}").into()),
        }
    }

    async fn read_bytes(&self, uri: &str) -> Result<Vec<u8>, BoxError> {
        self.read(uri).await.map(|s| s.into_bytes())
    }

    async fn write(&self, uri: &str, content: &str) -> Result<(), BoxError> {
        self.store
            .lock()
            .unwrap()
            .insert(uri.to_owned(), MockEntry::File(content.to_owned()));
        Ok(())
    }

    async fn write_bytes(&self, uri: &str, content: &[u8]) -> Result<(), BoxError> {
        let text = String::from_utf8_lossy(content).to_string();
        self.write(uri, &text).await
    }

    async fn mkdir(&self, uri: &str) -> Result<(), BoxError> {
        self.store
            .lock()
            .unwrap()
            .insert(uri.to_owned(), MockEntry::Dir);
        Ok(())
    }

    async fn ls(&self, uri: &str) -> Result<Vec<FsEntry>, BoxError> {
        let s = self.store.lock().unwrap();
        let prefix = format!("{}/", uri.trim_end_matches('/'));
        let mut entries = Vec::new();
        let mut seen = std::collections::HashSet::new();

        for (key, val) in s.iter() {
            if let Some(rest) = key.strip_prefix(&prefix) {
                // Only direct children
                let name = rest.split('/').next().unwrap_or("");
                if !name.is_empty() && seen.insert(name.to_owned()) {
                    let is_dir = matches!(val, MockEntry::Dir)
                        || rest.contains('/');
                    entries.push(FsEntry {
                        name: name.to_owned(),
                        is_dir,
                        size: if is_dir {
                            0
                        } else {
                            match val {
                                MockEntry::File(c) => c.len() as u64,
                                _ => 0,
                            }
                        },
                    });
                }
            }
        }
        Ok(entries)
    }

    async fn rm(&self, uri: &str) -> Result<(), BoxError> {
        let mut s = self.store.lock().unwrap();
        let prefix = format!("{}/", uri.trim_end_matches('/'));
        s.retain(|k, _| k != uri && !k.starts_with(&prefix));
        Ok(())
    }

    async fn mv(&self, from: &str, to: &str) -> Result<(), BoxError> {
        let mut s = self.store.lock().unwrap();
        let prefix = format!("{}/", from.trim_end_matches('/'));
        let mut moves = Vec::new();

        // Direct entry
        if let Some(val) = s.remove(from) {
            moves.push((to.to_owned(), val));
        }

        // Children
        let keys: Vec<String> = s.keys().cloned().collect();
        for key in keys {
            if key.starts_with(&prefix) {
                if let Some(val) = s.remove(&key) {
                    let new_key = key.replacen(from, to, 1);
                    moves.push((new_key, val));
                }
            }
        }

        for (k, v) in moves {
            s.insert(k, v);
        }
        Ok(())
    }

    async fn stat(&self, uri: &str) -> Result<FsStat, BoxError> {
        let s = self.store.lock().unwrap();
        match s.get(uri) {
            Some(MockEntry::File(c)) => Ok(FsStat {
                name: uri.rsplit('/').next().unwrap_or(uri).to_owned(),
                size: c.len() as u64,
                is_dir: false,
                mod_time: "2026-01-01T00:00:00Z".to_owned(),
            }),
            Some(MockEntry::Dir) => Ok(FsStat {
                name: uri.rsplit('/').next().unwrap_or(uri).to_owned(),
                size: 0,
                is_dir: true,
                mod_time: "2026-01-01T00:00:00Z".to_owned(),
            }),
            None => Err(format!("not found: {uri}").into()),
        }
    }

    async fn grep(
        &self,
        uri: &str,
        pattern: &str,
        _recursive: bool,
        case_insensitive: bool,
    ) -> Result<Vec<GrepMatch>, BoxError> {
        let s = self.store.lock().unwrap();
        let prefix = format!("{}/", uri.trim_end_matches('/'));
        let mut matches = Vec::new();

        for (key, val) in s.iter() {
            if key.starts_with(&prefix) || key == uri {
                if let MockEntry::File(content) = val {
                    for (i, line) in content.lines().enumerate() {
                        let m = if case_insensitive {
                            line.to_lowercase()
                                .contains(&pattern.to_lowercase())
                        } else {
                            line.contains(pattern)
                        };
                        if m {
                            matches.push(GrepMatch {
                                uri: key.clone(),
                                line: (i + 1) as u64,
                                content: line.to_owned(),
                            });
                        }
                    }
                }
            }
        }
        Ok(matches)
    }

    async fn exists(&self, uri: &str) -> Result<bool, BoxError> {
        Ok(self.store.lock().unwrap().contains_key(uri))
    }

    async fn append(&self, uri: &str, content: &str) -> Result<(), BoxError> {
        let mut s = self.store.lock().unwrap();
        let entry = s
            .entry(uri.to_owned())
            .or_insert(MockEntry::File(String::new()));
        if let MockEntry::File(ref mut c) = entry {
            c.push_str(content);
        }
        Ok(())
    }

    async fn link(&self, _: &str, _: &str) -> Result<(), BoxError> {
        Ok(())
    }
}

/// Mock vector store that tracks operations.
#[derive(Clone)]
#[allow(clippy::type_complexity)]
struct MockVs {
    deleted: Arc<Mutex<Vec<String>>>,
    updated: Arc<Mutex<Vec<(String, HashMap<String, serde_json::Value>)>>>,
}

impl MockVs {
    fn new() -> Self {
        Self {
            deleted: Arc::new(Mutex::new(Vec::new())),
            updated: Arc::new(Mutex::new(Vec::new())),
        }
    }
}

#[async_trait]
impl VectorStore for MockVs {
    async fn search(
        &self,
        _: &str,
        _: &[f32],
        _: Option<&HashMap<String, f64>>,
        _: usize,
        _: Option<&HashMap<String, serde_json::Value>>,
    ) -> Result<Vec<VectorHit>, BoxError> {
        Ok(Vec::new())
    }

    async fn upsert(
        &self,
        _: &str,
        _: &str,
        _: &[f32],
        _: HashMap<String, serde_json::Value>,
    ) -> Result<(), BoxError> {
        Ok(())
    }

    async fn update(
        &self,
        _: &str,
        id: &str,
        fields: HashMap<String, serde_json::Value>,
    ) -> Result<(), BoxError> {
        self.updated
            .lock()
            .unwrap()
            .push((id.to_owned(), fields));
        Ok(())
    }

    async fn delete(&self, _: &str, id: &str) -> Result<(), BoxError> {
        self.deleted.lock().unwrap().push(id.to_owned());
        Ok(())
    }
}

/// Mock embedder.
#[derive(Clone)]
struct MockEmb;

#[async_trait]
impl Embedder for MockEmb {
    async fn embed(&self, _: &str) -> Result<EmbedResult, BoxError> {
        Ok(EmbedResult {
            dense_vector: vec![0.1, 0.2, 0.3],
            sparse_vector: None,
        })
    }
}

// ===========================================================================
// Tests
// ===========================================================================

#[tokio::test]
async fn test_read_write() {
    let fs = MockFs::new();
    let vfs = VikingFs::<MockFs, MockVs, MockEmb>::new(fs);

    vfs.write("viking://resources/test.md", "hello world")
        .await
        .unwrap();
    let content = vfs.read("viking://resources/test.md").await.unwrap();
    assert_eq!(content, "hello world");
}

#[tokio::test]
async fn test_write_read_bytes() {
    let fs = MockFs::new();
    let vfs = VikingFs::<MockFs, MockVs, MockEmb>::new(fs);

    vfs.write_bytes("viking://resources/bin.dat", b"\x00\x01\x02")
        .await
        .unwrap();
    let data = vfs.read_bytes("viking://resources/bin.dat").await.unwrap();
    assert_eq!(data.len(), 3);
}

#[tokio::test]
async fn test_mkdir_and_exists() {
    let fs = MockFs::new();
    let vfs = VikingFs::<MockFs, MockVs, MockEmb>::new(fs);

    vfs.mkdir("viking://resources/newdir", false).await.unwrap();
    let exists = vfs.exists("viking://resources/newdir").await.unwrap();
    assert!(exists);
}

#[tokio::test]
async fn test_rm_basic() {
    let fs = MockFs::with_files(&[("viking://resources/a.md", "content")]);
    let vfs = VikingFs::<MockFs, MockVs, MockEmb>::new(fs);

    assert!(vfs.exists("viking://resources/a.md").await.unwrap());
    vfs.rm("viking://resources/a.md", false).await.unwrap();
    assert!(!vfs.exists("viking://resources/a.md").await.unwrap());
}

#[tokio::test]
async fn test_rm_with_vector_sync() {
    let fs = MockFs::with_files(&[("viking://resources/doc.md", "text")]);
    let vs = MockVs::new();
    let vfs = VikingFs::with_backends(fs, vs.clone(), MockEmb);

    vfs.rm("viking://resources/doc.md", false).await.unwrap();
    let deleted = vs.deleted.lock().unwrap();
    assert!(deleted.contains(&"viking://resources/doc.md".to_owned()));
}

#[tokio::test]
async fn test_mv_with_vector_sync() {
    let fs = MockFs::with_entries(&[
        ("viking://resources/old", MockEntry::Dir),
        ("viking://resources/old/a.md", MockEntry::File("data".into())),
    ]);
    let vs = MockVs::new();
    let vfs = VikingFs::with_backends(fs, vs.clone(), MockEmb);

    vfs.mv("viking://resources/old", "viking://resources/new")
        .await
        .unwrap();

    // Old should be gone
    assert!(!vfs.exists("viking://resources/old").await.unwrap());

    // Vector store should have been updated
    let updated = vs.updated.lock().unwrap();
    assert!(!updated.is_empty());
}

#[tokio::test]
async fn test_read_abstract() {
    let fs = MockFs::with_entries(&[
        ("viking://resources/proj", MockEntry::Dir),
        (
            "viking://resources/proj/.abstract.md",
            MockEntry::File("Project summary".into()),
        ),
    ]);
    let vfs = VikingFs::<MockFs, MockVs, MockEmb>::new(fs);

    let abstract_text = vfs.read_abstract("viking://resources/proj").await.unwrap();
    assert_eq!(abstract_text, "Project summary");
}

#[tokio::test]
async fn test_read_abstract_not_dir() {
    let fs = MockFs::with_files(&[("viking://resources/file.md", "content")]);
    let vfs = VikingFs::<MockFs, MockVs, MockEmb>::new(fs);

    let result = vfs.read_abstract("viking://resources/file.md").await;
    assert!(result.is_err());
}

#[tokio::test]
async fn test_read_overview() {
    let fs = MockFs::with_entries(&[
        ("viking://resources/proj", MockEntry::Dir),
        (
            "viking://resources/proj/.overview.md",
            MockEntry::File("Detailed overview".into()),
        ),
    ]);
    let vfs = VikingFs::<MockFs, MockVs, MockEmb>::new(fs);

    let overview = vfs.read_overview("viking://resources/proj").await.unwrap();
    assert_eq!(overview, "Detailed overview");
}

#[tokio::test]
async fn test_add_and_get_relations() {
    let fs = MockFs::with_entries(&[
        ("viking://resources/a", MockEntry::Dir),
    ]);
    let vfs = VikingFs::<MockFs, MockVs, MockEmb>::new(fs);

    vfs.add_relation(
        "viking://resources/a",
        &["viking://resources/b".into()],
        "related topic",
    )
    .await
    .unwrap();

    let relations = vfs.get_relations("viking://resources/a").await;
    assert_eq!(relations, vec!["viking://resources/b"]);
}

#[tokio::test]
async fn test_remove_relation() {
    let fs = MockFs::with_entries(&[
        ("viking://resources/a", MockEntry::Dir),
    ]);
    let vfs = VikingFs::<MockFs, MockVs, MockEmb>::new(fs);

    vfs.add_relation(
        "viking://resources/a",
        &[
            "viking://resources/b".into(),
            "viking://resources/c".into(),
        ],
        "test",
    )
    .await
    .unwrap();

    vfs.remove_relation("viking://resources/a", "viking://resources/b")
        .await
        .unwrap();

    let relations = vfs.get_relations("viking://resources/a").await;
    assert_eq!(relations, vec!["viking://resources/c"]);
}

#[tokio::test]
async fn test_relation_roundtrip() {
    let fs = MockFs::with_entries(&[
        ("viking://user/mem", MockEntry::Dir),
    ]);
    let vfs = VikingFs::<MockFs, MockVs, MockEmb>::new(fs);

    // Add two relations
    vfs.add_relation(
        "viking://user/mem",
        &["viking://resources/x".into()],
        "reason1",
    )
    .await
    .unwrap();
    vfs.add_relation(
        "viking://user/mem",
        &["viking://resources/y".into()],
        "reason2",
    )
    .await
    .unwrap();

    let table = vfs.get_relation_table("viking://user/mem").await;
    assert_eq!(table.len(), 2);
    assert_eq!(table[0].id, "link_1");
    assert_eq!(table[1].id, "link_2");
}

#[tokio::test]
async fn test_tree_listing() {
    let fs = MockFs::with_entries(&[
        ("viking://resources", MockEntry::Dir),
        ("viking://resources/docs", MockEntry::Dir),
        (
            "viking://resources/docs/readme.md",
            MockEntry::File("readme".into()),
        ),
        (
            "viking://resources/code.rs",
            MockEntry::File("fn main() {}".into()),
        ),
    ]);
    let vfs = VikingFs::<MockFs, MockVs, MockEmb>::new(fs);

    let entries = vfs.tree("viking://resources", false, 100).await.unwrap();
    assert!(!entries.is_empty());
    // Should contain docs/ directory and code.rs
    let names: Vec<&str> = entries.iter().map(|e| e.rel_path.as_str()).collect();
    assert!(names.contains(&"docs") || names.contains(&"code.rs"));
}

#[tokio::test]
async fn test_glob_md_files() {
    let fs = MockFs::with_entries(&[
        ("viking://resources", MockEntry::Dir),
        (
            "viking://resources/readme.md",
            MockEntry::File("r".into()),
        ),
        (
            "viking://resources/code.rs",
            MockEntry::File("c".into()),
        ),
    ]);
    let vfs = VikingFs::<MockFs, MockVs, MockEmb>::new(fs);

    let matches = vfs
        .glob("*.md", "viking://resources", 100)
        .await
        .unwrap();
    assert_eq!(matches.len(), 1);
    assert!(matches[0].ends_with("readme.md"));
}

#[tokio::test]
async fn test_write_context() {
    let fs = MockFs::new();
    let vfs = VikingFs::<MockFs, MockVs, MockEmb>::new(fs.clone());

    vfs.write_context(
        "viking://user/memories/topic",
        "Full content here",
        "A brief summary",
        "Detailed overview",
        "content.md",
        false,
    )
    .await
    .unwrap();

    // Verify all three files were written
    let content = fs
        .read("viking://user/memories/topic/content.md")
        .await
        .unwrap();
    assert_eq!(content, "Full content here");

    let abs = fs
        .read("viking://user/memories/topic/.abstract.md")
        .await
        .unwrap();
    assert_eq!(abs, "A brief summary");

    let ov = fs
        .read("viking://user/memories/topic/.overview.md")
        .await
        .unwrap();
    assert_eq!(ov, "Detailed overview");
}

#[tokio::test]
async fn test_append_file() {
    let fs = MockFs::with_files(&[("viking://resources/log.txt", "line1\n")]);
    let vfs = VikingFs::<MockFs, MockVs, MockEmb>::new(fs);

    vfs.append("viking://resources/log.txt", "line2\n")
        .await
        .unwrap();
    let content = vfs.read("viking://resources/log.txt").await.unwrap();
    assert_eq!(content, "line1\nline2\n");
}

#[tokio::test]
async fn test_uri_conversion() {
    assert_eq!(
        VikingFs::<MockFs, MockVs, MockEmb>::uri_to_path("viking://user/memories/code"),
        "/local/user/memories/code"
    );
    assert_eq!(
        VikingFs::<MockFs, MockVs, MockEmb>::path_to_uri("/local/resources/docs"),
        "viking://resources/docs"
    );
}

#[tokio::test]
async fn test_temp_uri_creation() {
    let uri = VikingFs::<MockFs, MockVs, MockEmb>::create_temp_uri();
    assert!(uri.starts_with("viking://temp/"));
}

#[tokio::test]
async fn test_batch_read() {
    let fs = MockFs::with_entries(&[
        ("viking://resources/a", MockEntry::Dir),
        (
            "viking://resources/a/.abstract.md",
            MockEntry::File("Summary A".into()),
        ),
        ("viking://resources/b", MockEntry::Dir),
        (
            "viking://resources/b/.abstract.md",
            MockEntry::File("Summary B".into()),
        ),
    ]);
    let vfs = VikingFs::<MockFs, MockVs, MockEmb>::new(fs);

    let results = vfs
        .batch_read(
            &[
                "viking://resources/a".into(),
                "viking://resources/b".into(),
            ],
            "l0",
        )
        .await;
    assert_eq!(results.len(), 2);
    assert_eq!(results["viking://resources/a"], "Summary A");
}
