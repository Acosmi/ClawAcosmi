//! Edit tool — 9-layer fuzzy matching file editor.
//!
//! Finds `old_string` in a file using progressively fuzzier matching
//! strategies and replaces it with `new_string`. Returns a unified diff
//! of the changes.
//!
//! Port of OpenAcosmi's `edit.ts` 9-layer replacer chain.

use std::path::Path;

use anyhow::{Context, Result};
use serde::Deserialize;

use crate::server::{ContentItem, ToolCallResult, ToolDefinition};

/// Parameters for the edit tool.
#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct EditParams {
    /// Path to the file to edit (relative to workspace or absolute).
    pub file_path: String,
    /// The text to find and replace.
    pub old_string: String,
    /// The replacement text.
    pub new_string: String,
    /// Replace all occurrences (default: false, replace first match only).
    #[serde(default)]
    pub replace_all: bool,
}

/// Return the MCP tool definition for `edit`.
pub fn tool_definition() -> ToolDefinition {
    ToolDefinition {
        name: "edit".to_owned(),
        description: "Edit a file by replacing old_string with new_string using 9-layer fuzzy matching. \
            If old_string is empty and the file doesn't exist, creates a new file with new_string as content."
            .to_owned(),
        input_schema: serde_json::json!({
            "type": "object",
            "properties": {
                "filePath": {
                    "type": "string",
                    "description": "Path to the file to edit"
                },
                "oldString": {
                    "type": "string",
                    "description": "The text to find (empty string = create new file)"
                },
                "newString": {
                    "type": "string",
                    "description": "The replacement text"
                },
                "replaceAll": {
                    "type": "boolean",
                    "description": "Replace all occurrences (default: false)",
                    "default": false
                }
            },
            "required": ["filePath", "oldString", "newString"]
        }),
    }
}

/// Execute the edit tool.
///
/// # Errors
///
/// Returns an error if the file cannot be read/written or no match is found.
pub fn execute(workspace: &Path, arguments: serde_json::Value) -> Result<ToolCallResult> {
    let params: EditParams =
        serde_json::from_value(arguments).context("invalid edit parameters")?;

    let file_path = match super::validate_path(workspace, &params.file_path) {
        Ok(p) => p,
        Err(e) => {
            return Ok(ToolCallResult {
                content: vec![ContentItem {
                    content_type: "text".to_owned(),
                    text: format!("Error: {e}"),
                }],
                is_error: true,
            });
        }
    };

    // Empty old_string: create new file or reject if file already exists.
    if params.old_string.is_empty() {
        if file_path.exists() {
            return Ok(ToolCallResult {
                content: vec![ContentItem {
                    content_type: "text".to_owned(),
                    text: format!(
                        "Error: old_string cannot be empty for existing file {}. \
                         Provide the text to find and replace, or delete the file first to recreate it.",
                        file_path.display()
                    ),
                }],
                is_error: true,
            });
        }

        if let Some(parent) = file_path.parent() {
            std::fs::create_dir_all(parent)
                .with_context(|| format!("failed to create directories for {}", file_path.display()))?;
        }
        crate::util::atomic::atomic_write(&file_path, &params.new_string)?;

        return Ok(ToolCallResult {
            content: vec![ContentItem {
                content_type: "text".to_owned(),
                text: format!("Created new file: {}", file_path.display()),
            }],
            is_error: false,
        });
    }

    let original = std::fs::read_to_string(&file_path)
        .with_context(|| format!("failed to read {}", file_path.display()))?;

    // Delegate to the edit engine (Phase 2 will implement full 9-layer chain).
    let result = crate::edit::replace(&original, &params.old_string, &params.new_string, params.replace_all);

    match result {
        Some(new_content) => {
            // Generate diff before writing.
            let diff = crate::edit::diff::unified_diff(
                &file_path.display().to_string(),
                &original,
                &new_content,
            );

            crate::util::atomic::atomic_write(&file_path, &new_content)?;

            Ok(ToolCallResult {
                content: vec![ContentItem {
                    content_type: "text".to_owned(),
                    text: diff,
                }],
                is_error: false,
            })
        }
        None => Ok(ToolCallResult {
            content: vec![ContentItem {
                content_type: "text".to_owned(),
                text: format!(
                    "Error: no match found for the provided old_string in {}",
                    file_path.display()
                ),
            }],
            is_error: true,
        }),
    }
}

