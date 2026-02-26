//! Grep tool — ripgrep (`rg --json`) subprocess wrapper.
//!
//! Shells out to `rg` for full ripgrep functionality (type filtering,
//! .gitignore support, parallel search, SIMD acceleration).
//! Falls back to a basic Rust regex search if `rg` is not installed.

use std::path::Path;
use std::process::Command;

use anyhow::{Context, Result};
use serde::Deserialize;

use crate::server::{ContentItem, ToolCallResult, ToolDefinition};

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct GrepParams {
    /// Regex pattern to search for.
    pub pattern: String,
    /// Directory or file to search in (relative to workspace).
    #[serde(default)]
    pub path: Option<String>,
    /// Glob pattern to filter files (e.g. "*.rs", "*.{ts,tsx}").
    #[serde(default)]
    pub include: Option<String>,
    /// Maximum number of results.
    #[serde(default = "default_max_results")]
    pub max_results: usize,
    /// Include N lines of context around matches.
    #[serde(default)]
    pub context_lines: usize,
}

const fn default_max_results() -> usize { 100 }

pub fn tool_definition() -> ToolDefinition {
    ToolDefinition {
        name: "grep".to_owned(),
        description: "Search file contents using regex patterns. Uses ripgrep (rg) for fast, \
            gitignore-aware searching. Supports file type filtering and context lines."
            .to_owned(),
        input_schema: serde_json::json!({
            "type": "object",
            "properties": {
                "pattern": {
                    "type": "string",
                    "description": "Regex pattern to search for"
                },
                "path": {
                    "type": "string",
                    "description": "Directory or file to search in (default: workspace root)"
                },
                "include": {
                    "type": "string",
                    "description": "Glob pattern to filter files (e.g. \"*.rs\")"
                },
                "maxResults": {
                    "type": "integer",
                    "description": "Maximum number of results (default: 100)",
                    "default": 100
                },
                "contextLines": {
                    "type": "integer",
                    "description": "Lines of context around matches (default: 0)",
                    "default": 0
                }
            },
            "required": ["pattern"]
        }),
    }
}

pub fn execute(workspace: &Path, arguments: serde_json::Value) -> Result<ToolCallResult> {
    let params: GrepParams =
        serde_json::from_value(arguments).context("invalid grep parameters")?;

    let search_path = match &params.path {
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

    // Try to find rg binary.
    let rg_path = which::which("rg");

    match rg_path {
        Ok(rg) => execute_rg(&rg, &search_path, &params),
        Err(_) => {
            // Fallback: basic regex search (no gitignore, no parallel).
            execute_fallback(&search_path, &params)
        }
    }
}

/// Execute search using ripgrep subprocess.
fn execute_rg(
    rg: &Path,
    search_path: &Path,
    params: &GrepParams,
) -> Result<ToolCallResult> {
    // Note: rg --max-count is per-file, not total. Use a higher limit
    // to avoid missing results spread across many files, then truncate
    // client-side to params.max_results.
    let rg_max = params.max_results.saturating_mul(10).max(100);

    let mut cmd = Command::new(rg);
    cmd.arg("--color").arg("never")
        .arg("--line-number")
        .arg("--no-heading")
        .arg("--max-count").arg(rg_max.to_string());

    if params.context_lines > 0 {
        cmd.arg("-C").arg(params.context_lines.to_string());
    }

    if let Some(include) = &params.include {
        cmd.arg("--glob").arg(include);
    }

    cmd.arg(&params.pattern).arg(search_path);

    let output = cmd.output().context("failed to execute rg")?;

    let stdout = String::from_utf8_lossy(&output.stdout);
    let stderr = String::from_utf8_lossy(&output.stderr);

    // rg exit code: 0 = matches found, 1 = no matches, 2 = error.
    if output.status.code() == Some(2) {
        return Ok(ToolCallResult {
            content: vec![ContentItem {
                content_type: "text".to_owned(),
                text: format!("grep error: {stderr}"),
            }],
            is_error: true,
        });
    }

    if stdout.is_empty() {
        return Ok(ToolCallResult {
            content: vec![ContentItem {
                content_type: "text".to_owned(),
                text: "No matches found.".to_owned(),
            }],
            is_error: false,
        });
    }

    // Client-side truncation to respect max_results (rg --max-count is per-file).
    let lines: Vec<&str> = stdout.lines().collect();
    let truncated = if lines.len() > params.max_results {
        let mut out = lines[..params.max_results].join("\n");
        out.push_str(&format!(
            "\n\n... truncated ({} results shown out of {}+)",
            params.max_results,
            lines.len()
        ));
        out
    } else {
        stdout.into_owned()
    };

    Ok(ToolCallResult {
        content: vec![ContentItem {
            content_type: "text".to_owned(),
            text: truncated,
        }],
        is_error: false,
    })
}

/// Fallback: basic regex file search without ripgrep.
fn execute_fallback(
    search_path: &Path,
    params: &GrepParams,
) -> Result<ToolCallResult> {
    let re = regex::Regex::new(&params.pattern)
        .with_context(|| format!("invalid regex pattern: {}", params.pattern))?;

    let mut results = Vec::new();
    let mut count = 0;

    walk_files(search_path, &mut |path| {
        if count >= params.max_results {
            return;
        }

        let Ok(content) = std::fs::read_to_string(path) else {
            return;
        };

        for (i, line) in content.lines().enumerate() {
            if count >= params.max_results {
                return;
            }
            if re.is_match(line) {
                results.push(format!("{}:{}:{}", path.display(), i + 1, line));
                count += 1;
            }
        }
    })?;

    if results.is_empty() {
        return Ok(ToolCallResult {
            content: vec![ContentItem {
                content_type: "text".to_owned(),
                text: "No matches found. (Note: rg not installed, using basic fallback)".to_owned(),
            }],
            is_error: false,
        });
    }

    Ok(ToolCallResult {
        content: vec![ContentItem {
            content_type: "text".to_owned(),
            text: results.join("\n"),
        }],
        is_error: false,
    })
}

/// Maximum recursion depth for fallback file walker.
const MAX_WALK_DEPTH: usize = 50;

/// Simple recursive file walker (fallback only, no gitignore support).
/// Uses `entry.file_type()` (no symlink following) and depth limit to prevent loops.
fn walk_files(dir: &Path, cb: &mut impl FnMut(&Path)) -> Result<()> {
    walk_files_inner(dir, cb, 0)
}

fn walk_files_inner(dir: &Path, cb: &mut impl FnMut(&Path), depth: usize) -> Result<()> {
    if depth > MAX_WALK_DEPTH {
        return Ok(()); // Silently stop at max depth.
    }

    // Check the top-level entry without following symlinks.
    let meta = std::fs::symlink_metadata(dir);
    match meta {
        Ok(m) if m.is_file() => {
            cb(dir);
            return Ok(());
        }
        Ok(m) if m.is_symlink() => return Ok(()), // Skip symlinks.
        Ok(m) if !m.is_dir() => return Ok(()),
        Err(_) => return Ok(()),
        _ => {}
    }

    let entries = std::fs::read_dir(dir)
        .with_context(|| format!("failed to read directory {}", dir.display()))?;

    for entry in entries {
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
            walk_files_inner(&path, cb, depth + 1)?;
        } else if ft.is_file() {
            cb(&path);
        }
        // Symlinks are skipped (ft.is_symlink() falls through).
    }

    Ok(())
}
