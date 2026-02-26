//! Write tool — file creation and overwrite with directory auto-creation.

use std::path::Path;

use anyhow::{Context, Result};
use serde::Deserialize;

use crate::server::{ContentItem, ToolCallResult, ToolDefinition};

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct WriteParams {
    /// Path to the file to write.
    pub file_path: String,
    /// Content to write.
    pub content: String,
}

pub fn tool_definition() -> ToolDefinition {
    ToolDefinition {
        name: "write".to_owned(),
        description: "Write content to a file. Creates the file and parent directories if they don't exist. \
            Overwrites existing content."
            .to_owned(),
        input_schema: serde_json::json!({
            "type": "object",
            "properties": {
                "filePath": {
                    "type": "string",
                    "description": "Path to the file to write"
                },
                "content": {
                    "type": "string",
                    "description": "Content to write to the file"
                }
            },
            "required": ["filePath", "content"]
        }),
    }
}

pub fn execute(workspace: &Path, arguments: serde_json::Value) -> Result<ToolCallResult> {
    let params: WriteParams =
        serde_json::from_value(arguments).context("invalid write parameters")?;

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

    // Create parent directories.
    if let Some(parent) = file_path.parent() {
        std::fs::create_dir_all(parent)
            .with_context(|| format!("failed to create directories for {}", file_path.display()))?;
    }

    let existed = file_path.exists();
    crate::util::atomic::atomic_write(&file_path, &params.content)?;

    let action = if existed { "Updated" } else { "Created" };
    let line_count = params.content.lines().count();

    Ok(ToolCallResult {
        content: vec![ContentItem {
            content_type: "text".to_owned(),
            text: format!("{action} {}: {line_count} lines written", file_path.display()),
        }],
        is_error: false,
    })
}
