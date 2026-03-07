//! Stub builder — returns UnsupportedProjectType for JS/Python/Unknown.
//!
//! Placeholder until Phase 2 (JS/TS) and Phase 3 (Python) support is added.

use std::path::Path;

use crate::error::{McpInstallError, Result};
use crate::types::{BuildResult, McpServerManifest, ProjectType};

use super::McpBuilder;

/// Stub builder for unsupported project types.
pub struct StubBuilder;

impl McpBuilder for StubBuilder {
    fn can_build(&self, _project_type: &ProjectType) -> bool {
        false
    }

    fn build(&self, _clone_dir: &Path, manifest: &McpServerManifest) -> Result<BuildResult> {
        Err(McpInstallError::UnsupportedProjectType(format!(
            "cannot build {:?} — only Rust and Go are supported in Phase 1",
            manifest.name
        )))
    }
}
