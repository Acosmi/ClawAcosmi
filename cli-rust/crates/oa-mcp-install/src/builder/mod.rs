//! Build pipeline for MCP servers.
//!
//! Provides the [`McpBuilder`] trait and concrete implementations for
//! Rust (`cargo build`) and Go (`go build`).

pub mod go_builder;
pub mod rust_builder;
pub mod stub_builder;

use std::path::Path;

use crate::error::Result;
use crate::types::{BuildResult, McpServerManifest, ProjectType};

/// Trait for MCP server build backends.
pub trait McpBuilder: Send + Sync {
    /// Check whether this builder can handle the given project type.
    fn can_build(&self, project_type: &ProjectType) -> bool;

    /// Build the MCP server in `clone_dir` according to `manifest`.
    ///
    /// Returns the path to the built binary, its SHA-256 hash, and the
    /// source commit if determinable.
    fn build(&self, clone_dir: &Path, manifest: &McpServerManifest) -> Result<BuildResult>;
}

/// Select the appropriate builder for a project type.
pub fn select_builder(project_type: &ProjectType) -> Box<dyn McpBuilder> {
    match project_type {
        ProjectType::Rust => Box::new(rust_builder::RustBuilder),
        ProjectType::Go => Box::new(go_builder::GoBuilder),
        _ => Box::new(stub_builder::StubBuilder),
    }
}
