// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Unit tests for `tree_builder` module.

#[cfg(test)]
mod tests {
    use std::collections::HashMap;
    use std::sync::{Arc, Mutex};

    use async_trait::async_trait;

    use openviking_session::traits::{BoxError, FileSystem, FsEntry, FsStat, GrepMatch};

    use crate::tree_builder::{SemanticEnqueuer, TreeBuilder};

    // -----------------------------------------------------------------------
    // Mock FileSystem
    // -----------------------------------------------------------------------

    /// A minimal in-memory file system for testing TreeBuilder.
    struct MockFs {
        /// URI → content (for files) or URI → "" (for directories).
        entries: Mutex<HashMap<String, MockEntry>>,
    }

    #[derive(Clone)]
    enum MockEntry {
        File(String),
        Dir,
    }

    impl MockFs {
        fn new() -> Self {
            Self {
                entries: Mutex::new(HashMap::new()),
            }
        }

        fn add_dir(&self, uri: &str) {
            self.entries
                .lock()
                .unwrap()
                .insert(uri.to_owned(), MockEntry::Dir);
        }

        fn add_file(&self, uri: &str, content: &str) {
            self.entries
                .lock()
                .unwrap()
                .insert(uri.to_owned(), MockEntry::File(content.to_owned()));
        }
    }

    #[async_trait]
    impl FileSystem for MockFs {
        async fn read(&self, uri: &str) -> Result<String, BoxError> {
            match self.entries.lock().unwrap().get(uri) {
                Some(MockEntry::File(c)) => Ok(c.clone()),
                _ => Err(format!("not found: {uri}").into()),
            }
        }

        async fn write(&self, uri: &str, content: &str) -> Result<(), BoxError> {
            self.entries
                .lock()
                .unwrap()
                .insert(uri.to_owned(), MockEntry::File(content.to_owned()));
            Ok(())
        }

        async fn read_bytes(&self, uri: &str) -> Result<Vec<u8>, BoxError> {
            match self.entries.lock().unwrap().get(uri) {
                Some(MockEntry::File(c)) => Ok(c.as_bytes().to_vec()),
                _ => Err(format!("not found: {uri}").into()),
            }
        }

        async fn write_bytes(&self, uri: &str, _data: &[u8]) -> Result<(), BoxError> {
            self.entries
                .lock()
                .unwrap()
                .insert(uri.to_owned(), MockEntry::File(String::new()));
            Ok(())
        }

        async fn mkdir(&self, uri: &str) -> Result<(), BoxError> {
            self.entries
                .lock()
                .unwrap()
                .insert(uri.to_owned(), MockEntry::Dir);
            Ok(())
        }

        async fn rm(&self, uri: &str) -> Result<(), BoxError> {
            self.entries.lock().unwrap().remove(uri);
            Ok(())
        }

        async fn ls(&self, uri: &str) -> Result<Vec<FsEntry>, BoxError> {
            let entries = self.entries.lock().unwrap();
            let prefix = format!("{uri}/");
            let mut result = Vec::new();

            for (key, val) in entries.iter() {
                if let Some(rest) = key.strip_prefix(&prefix) {
                    // Only direct children (no further slashes).
                    if !rest.contains('/') && !rest.is_empty() {
                        result.push(FsEntry {
                            name: rest.to_owned(),
                            is_dir: matches!(val, MockEntry::Dir),
                            size: 0,
                        });
                    }
                }
            }

            Ok(result)
        }

        async fn mv(&self, from: &str, to: &str) -> Result<(), BoxError> {
            let mut entries = self.entries.lock().unwrap();
            if let Some(entry) = entries.remove(from) {
                entries.insert(to.to_owned(), entry);
                Ok(())
            } else {
                Err(format!("not found: {from}").into())
            }
        }

        async fn stat(&self, uri: &str) -> Result<FsStat, BoxError> {
            match self.entries.lock().unwrap().get(uri) {
                Some(_) => Ok(FsStat {
                    name: uri.to_owned(),
                    size: 0,
                    is_dir: true,
                    mod_time: String::new(),
                }),
                None => Err(format!("not found: {uri}").into()),
            }
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
            Ok(self.entries.lock().unwrap().contains_key(uri))
        }

        async fn append(&self, uri: &str, content: &str) -> Result<(), BoxError> {
            let mut entries = self.entries.lock().unwrap();
            match entries.get_mut(uri) {
                Some(MockEntry::File(c)) => {
                    c.push_str(content);
                    Ok(())
                }
                None => {
                    entries.insert(uri.to_owned(), MockEntry::File(content.to_owned()));
                    Ok(())
                }
                _ => Err("cannot append to directory".into()),
            }
        }

        async fn link(&self, _src: &str, _dst: &str) -> Result<(), BoxError> {
            Ok(())
        }
    }

    // -----------------------------------------------------------------------
    // Mock Enqueuer
    // -----------------------------------------------------------------------

    struct MockEnqueuer {
        enqueued: Mutex<Vec<(String, String)>>,
    }

    impl MockEnqueuer {
        fn new() -> Self {
            Self {
                enqueued: Mutex::new(Vec::new()),
            }
        }

        fn enqueued(&self) -> Vec<(String, String)> {
            self.enqueued.lock().unwrap().clone()
        }
    }

    #[async_trait]
    impl SemanticEnqueuer for MockEnqueuer {
        async fn enqueue(&self, uri: &str, context_type: &str) -> Result<(), BoxError> {
            self.enqueued
                .lock()
                .unwrap()
                .push((uri.to_owned(), context_type.to_owned()));
            Ok(())
        }
    }

    // -----------------------------------------------------------------------
    // Tests
    // -----------------------------------------------------------------------

    #[tokio::test]
    async fn test_resolve_unique_uri_free() {
        let fs = Arc::new(MockFs::new());
        let builder = TreeBuilder::new(fs, None);

        let uri = builder
            .resolve_unique_uri("viking://resources/doc", 10)
            .await
            .unwrap();
        assert_eq!(uri, "viking://resources/doc");
    }

    #[tokio::test]
    async fn test_resolve_unique_uri_conflict() {
        let fs = Arc::new(MockFs::new());
        fs.add_dir("viking://resources/doc");

        let builder = TreeBuilder::new(fs, None);
        let uri = builder
            .resolve_unique_uri("viking://resources/doc", 10)
            .await
            .unwrap();
        assert_eq!(uri, "viking://resources/doc_1");
    }

    #[tokio::test]
    async fn test_resolve_unique_uri_multiple_conflicts() {
        let fs = Arc::new(MockFs::new());
        fs.add_dir("viking://resources/doc");
        fs.add_dir("viking://resources/doc_1");
        fs.add_dir("viking://resources/doc_2");

        let builder = TreeBuilder::new(fs, None);
        let uri = builder
            .resolve_unique_uri("viking://resources/doc", 10)
            .await
            .unwrap();
        assert_eq!(uri, "viking://resources/doc_3");
    }

    #[tokio::test]
    async fn test_move_directory() {
        let fs = Arc::new(MockFs::new());
        fs.add_dir("viking://temp/doc");
        fs.add_file("viking://temp/doc/content.md", "hello");

        let builder = TreeBuilder::new(Arc::clone(&fs), None);
        builder
            .move_directory_in_agfs("viking://temp/doc", "viking://resources/doc")
            .await
            .unwrap();

        // Source should be cleaned up.
        assert!(!fs.entries.lock().unwrap().contains_key("viking://temp/doc"));
        // Destination should exist.
        assert!(fs
            .entries
            .lock()
            .unwrap()
            .contains_key("viking://resources/doc/content.md"));
    }

    #[tokio::test]
    async fn test_finalize_from_temp() {
        let fs = Arc::new(MockFs::new());
        // Setup temp structure: viking://temp/root/my_doc/content.md
        fs.add_dir("viking://temp/root");
        fs.add_dir("viking://temp/root/my_doc");
        fs.add_file("viking://temp/root/my_doc/content.md", "hello world");

        let enqueuer = Arc::new(MockEnqueuer::new());

        let builder = TreeBuilder::new(Arc::clone(&fs), Some(Arc::clone(&enqueuer) as _));
        let tree = builder
            .finalize_from_temp(
                "viking://temp/root",
                "resources",
                "viking://resources",
                Some("/tmp/test.pdf"),
                Some("pdf"),
            )
            .await
            .unwrap();

        // Verify root URI was set.
        assert!(tree.root_uri().is_some());
        let root_uri = tree.root_uri().unwrap();
        assert!(root_uri.starts_with("viking://resources/my_doc"));

        // Verify semantic enqueue was called.
        let enqueued = enqueuer.enqueued();
        assert_eq!(enqueued.len(), 1);
        assert_eq!(enqueued[0].1, "resource");
    }
}
