// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Unit tests for `LocalFs` — the local-disk `FileSystem` implementation.
//!
//! All tests use `tempfile::tempdir()` to create isolated temporary directories
//! that are automatically cleaned up when dropped.

use openviking_session::traits::FileSystem;

use crate::local_fs::LocalFs;

/// Helper: create a `LocalFs` rooted in a fresh temp dir.
fn make_fs() -> (tempfile::TempDir, LocalFs) {
    let dir = tempfile::tempdir().expect("failed to create tempdir");
    let fs = LocalFs::new(dir.path());
    (dir, fs)
}

// =========================================================================
// 1. read/write roundtrip
// =========================================================================

#[tokio::test]
async fn read_write_roundtrip() {
    let (_dir, fs) = make_fs();
    fs.write("hello.txt", "world").await.unwrap();
    let content = fs.read("hello.txt").await.unwrap();
    assert_eq!(content, "world");
}

// =========================================================================
// 2. read/write bytes roundtrip
// =========================================================================

#[tokio::test]
async fn read_write_bytes_roundtrip() {
    let (_dir, fs) = make_fs();
    let data: Vec<u8> = vec![0xDE, 0xAD, 0xBE, 0xEF];
    fs.write_bytes("bin.dat", &data).await.unwrap();
    let read_back = fs.read_bytes("bin.dat").await.unwrap();
    assert_eq!(read_back, data);
}

// =========================================================================
// 3. write creates parent directories
// =========================================================================

#[tokio::test]
async fn write_creates_parent_dirs() {
    let (_dir, fs) = make_fs();
    fs.write("a/b/c/deep.txt", "nested").await.unwrap();
    let content = fs.read("a/b/c/deep.txt").await.unwrap();
    assert_eq!(content, "nested");
}

// =========================================================================
// 4. mkdir idempotent
// =========================================================================

#[tokio::test]
async fn mkdir_idempotent() {
    let (_dir, fs) = make_fs();
    fs.mkdir("some/dir").await.unwrap();
    // Second call should not error.
    fs.mkdir("some/dir").await.unwrap();
    assert!(fs.exists("some/dir").await.unwrap());
}

// =========================================================================
// 5. ls lists entries
// =========================================================================

#[tokio::test]
async fn ls_lists_entries() {
    let (_dir, fs) = make_fs();
    fs.write("dir1/a.txt", "a").await.unwrap();
    fs.write("dir1/b.txt", "b").await.unwrap();
    fs.mkdir("dir1/sub").await.unwrap();

    let entries = fs.ls("dir1").await.unwrap();
    assert_eq!(entries.len(), 3);

    let names: Vec<&str> = entries.iter().map(|e| e.name.as_str()).collect();
    assert!(names.contains(&"a.txt"));
    assert!(names.contains(&"b.txt"));
    assert!(names.contains(&"sub"));

    let sub = entries.iter().find(|e| e.name == "sub").unwrap();
    assert!(sub.is_dir);
}

// =========================================================================
// 6. rm file
// =========================================================================

#[tokio::test]
async fn rm_file() {
    let (_dir, fs) = make_fs();
    fs.write("to_delete.txt", "bye").await.unwrap();
    assert!(fs.exists("to_delete.txt").await.unwrap());

    fs.rm("to_delete.txt").await.unwrap();
    assert!(!fs.exists("to_delete.txt").await.unwrap());
}

// =========================================================================
// 7. rm directory (recursive)
// =========================================================================

#[tokio::test]
async fn rm_directory() {
    let (_dir, fs) = make_fs();
    fs.write("rmdir/child/file.txt", "data").await.unwrap();
    assert!(fs.exists("rmdir/child/file.txt").await.unwrap());

    fs.rm("rmdir").await.unwrap();
    assert!(!fs.exists("rmdir").await.unwrap());
}

// =========================================================================
// 8. mv rename
// =========================================================================

#[tokio::test]
async fn mv_rename() {
    let (_dir, fs) = make_fs();
    fs.write("old.txt", "content").await.unwrap();
    fs.mv("old.txt", "new.txt").await.unwrap();

    assert!(!fs.exists("old.txt").await.unwrap());
    let content = fs.read("new.txt").await.unwrap();
    assert_eq!(content, "content");
}

// =========================================================================
// 9. stat returns metadata
// =========================================================================

#[tokio::test]
async fn stat_returns_metadata() {
    let (_dir, fs) = make_fs();
    fs.write("stat_me.txt", "12345").await.unwrap();

    let st = fs.stat("stat_me.txt").await.unwrap();
    assert_eq!(st.name, "stat_me.txt");
    assert_eq!(st.size, 5);
    assert!(!st.is_dir);
    assert!(!st.mod_time.is_empty());
}

// =========================================================================
// 10. grep finds pattern
// =========================================================================

#[tokio::test]
async fn grep_finds_pattern() {
    let (_dir, fs) = make_fs();
    fs.write("search.txt", "hello world\nfoo bar\nhello rust")
        .await
        .unwrap();

    let matches = fs.grep("search.txt", "hello", false, false).await.unwrap();
    assert_eq!(matches.len(), 2);
    assert_eq!(matches[0].line, 1);
    assert_eq!(matches[1].line, 3);
}

// =========================================================================
// 11. grep case insensitive
// =========================================================================

#[tokio::test]
async fn grep_case_insensitive() {
    let (_dir, fs) = make_fs();
    fs.write("ci.txt", "Hello World\nhello rust\nHELLO ALL")
        .await
        .unwrap();

    let matches = fs.grep("ci.txt", "hello", false, true).await.unwrap();
    assert_eq!(matches.len(), 3);
}

// =========================================================================
// 12. exists returns correct result
// =========================================================================

#[tokio::test]
async fn exists_returns_correct() {
    let (_dir, fs) = make_fs();
    assert!(!fs.exists("nope.txt").await.unwrap());

    fs.write("yes.txt", "hi").await.unwrap();
    assert!(fs.exists("yes.txt").await.unwrap());
}

// =========================================================================
// 13. append content
// =========================================================================

#[tokio::test]
async fn append_content() {
    let (_dir, fs) = make_fs();
    fs.write("log.txt", "line1\n").await.unwrap();
    fs.append("log.txt", "line2\n").await.unwrap();

    let content = fs.read("log.txt").await.unwrap();
    assert_eq!(content, "line1\nline2\n");
}

// =========================================================================
// 14. read nonexistent returns error
// =========================================================================

#[tokio::test]
async fn read_nonexistent_returns_error() {
    let (_dir, fs) = make_fs();
    let result = fs.read("does_not_exist.txt").await;
    assert!(result.is_err());
}

// =========================================================================
// 15. viking:// URI prefix is handled
// =========================================================================

#[tokio::test]
async fn viking_uri_prefix() {
    let (_dir, fs) = make_fs();
    fs.write("viking://resources/note.md", "# Note").await.unwrap();
    let content = fs.read("viking://resources/note.md").await.unwrap();
    assert_eq!(content, "# Note");

    // Also accessible without prefix.
    let content2 = fs.read("resources/note.md").await.unwrap();
    assert_eq!(content2, "# Note");
}

// =========================================================================
// 16. grep recursive in directory
// =========================================================================

#[tokio::test]
async fn grep_recursive_directory() {
    let (_dir, fs) = make_fs();
    fs.write("grepdir/a.txt", "match here\nno match")
        .await
        .unwrap();
    fs.write("grepdir/sub/b.txt", "another match here")
        .await
        .unwrap();

    let matches = fs.grep("grepdir", "match", true, false).await.unwrap();
    // Should find 3 matches: line 1 of a.txt, line 2 of a.txt has "no match", and line 1 of b.txt
    assert!(matches.len() >= 3);
}

// =========================================================================
// 17. link creates symlink
// =========================================================================

#[cfg(unix)]
#[tokio::test]
async fn link_creates_symlink() {
    let (_dir, fs) = make_fs();
    fs.write("original.txt", "source content").await.unwrap();
    fs.link("original.txt", "linked.txt").await.unwrap();

    // Read through symlink.
    let content = fs.read("linked.txt").await.unwrap();
    assert_eq!(content, "source content");
}
