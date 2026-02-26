//! nexus-unified — 统一 FFI 入口。
//!
//! 将所有 nexus-* crate 的 FFI 函数合并到一个 staticlib，
//! 避免多个 staticlib 链接时 Rust runtime 符号重复。

// 强制链接器包含所有 crate 的符号
extern crate nexus_crypto;
extern crate nexus_decay;
extern crate nexus_graph;
extern crate nexus_memfs;
extern crate nexus_tokenizer;
extern crate nexus_vector;
