//! Glob tool — file discovery via globset patterns.

use std::path::Path;

use anyhow::{Context, Result};
use serde::Deserialize;

use crate::server::{ContentItem, ToolCallResult, ToolDefinition};

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct GlobParams {
    /// Glob pattern (e.g. "**/*.rs", "src/**/*.ts").
    pub pattern: String,
    /// Directory to search in (relative to workspace).
    #[serde(default)]
    pub path: Option<String>,
    /// Maximum number of results.
    #[serde(default = "default_max_results")]
    pub max_results: usize,
}

const fn default_max_results() -> usize { 500 }

pub fn tool_definition() -> ToolDefinition {
    ToolDefinition {
        name: "glob".to_owned(),
        description: "Find files matching a glob pattern. Supports ** for recursive matching, \
            * for wildcards, {a,b} for alternation."
            .to_owned(),
        input_schema: serde_json::json!({
            "type": "object",
            "properties": {
                "pattern": {
                    "type": "string",
                    "description": "Glob pattern (e.g. \"**/*.rs\", \"src/**/*.ts\")"
                },
                "path": {
                    "type": "string",
                    "description": "Directory to search in (default: workspace root)"
                },
                "maxResults": {
                    "type": "integer",
                    "description": "Maximum number of results (default: 500)",
                    "default": 500
                }
            },
            "required": ["pattern"]
        }),
    }
}

pub fn execute(workspace: &Path, arguments: serde_json::Value) -> Result<ToolCallResult> {
    let params: GlobParams =
        serde_json::from_value(arguments).context("invalid glob parameters")?;

    let search_dir = match &params.path {
        Some(p) => match super::validate_dir_path(workspace, p) {
            Ok(path) => path,
            Err(e) => {
                return Ok(ToolCallResult {
                    content: vec![ContentItem {
                        content_type: "text".to_owned(),
                        text: format!("Error: {e}"),
                    }],
                    is_error: true,
                });
            }
        },
        None => workspace.to_path_buf(),
    };

    let glob = globset::GlobBuilder::new(&params.pattern)
        .literal_separator(false)
        .build()
        .with_context(|| format!("invalid glob pattern: {}", params.pattern))?
        .compile_matcher();

    let mut matches = Vec::new();
    collect_matches(&search_dir, &search_dir, &glob, &mut matches, params.max_results)?;

    // Sort by path for deterministic output.
    matches.sort();

    if matches.is_empty() {
        return Ok(ToolCallResult {
            content: vec![ContentItem {
                content_type: "text".to_owned(),
                text: format!("No files matching pattern: {}", params.pattern),
            }],
            is_error: false,
        });
    }

    let output = matches.join("\n");

    Ok(ToolCallResult {
        content: vec![ContentItem {
            content_type: "text".to_owned(),
            text: output,
        }],
        is_error: false,
    })
}

/// Maximum recursion depth for glob file walker.
const MAX_WALK_DEPTH: usize = 50;

/// Recursively collect files matching the glob pattern.
/// Uses `entry.file_type()` (no symlink following) and depth limit to prevent loops.
fn collect_matches(
    root: &Path,
    dir: &Path,
    glob: &globset::GlobMatcher,
    matches: &mut Vec<String>,
    max: usize,
) -> Result<()> {
    collect_matches_inner(root, dir, glob, matches, max, 0)
}

fn collect_matches_inner(
    root: &Path,
    dir: &Path,
    glob: &globset::GlobMatcher,
    matches: &mut Vec<String>,
    max: usize,
    depth: usize,
) -> Result<()> {
    if matches.len() >= max || depth > MAX_WALK_DEPTH {
        return Ok(());
    }

    let entries = std::fs::read_dir(dir)
        .with_context(|| format!("failed to read directory {}", dir.display()))?;

    for entry in entries {
        if matches.len() >= max {
            break;
        }

        let entry = entry.context("failed to read directory entry")?;
        let path = entry.path();

        // Skip hidden directories and common noise.
        if let Some(name) = path.file_name().and_then(|n| n.to_str()) {
            if name.starts_with('.') || name == "node_modules" || name == "target" {
                continue;
            }
        }

        // Use entry.file_type() which does NOT follow symlinks.
        let ft = match entry.file_type() {
            Ok(ft) => ft,
            Err(_) => continue,
        };

        if ft.is_dir() {
            collect_matches_inner(root, &path, glob, matches, max, depth + 1)?;
        } else if ft.is_file() {
            // Match against relative path from root.
            if let Ok(relative) = path.strip_prefix(root) {
                if glob.is_match(relative) {
                    matches.push(relative.display().to_string());
                }
            }
        }
        // Symlinks are skipped.
    }

    Ok(())
}
