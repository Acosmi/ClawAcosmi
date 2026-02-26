//! Read tool — file reading with line numbers, offset/limit, binary detection.

use std::io::Read as _;
use std::path::Path;

use anyhow::{Context, Result};
use serde::Deserialize;

use crate::server::{ContentItem, ToolCallResult, ToolDefinition};

/// Parameters for the read tool.
#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ReadParams {
    /// Path to the file to read.
    pub file_path: String,
    /// Starting line number (1-based). Default: 1.
    #[serde(default = "default_offset")]
    pub offset: usize,
    /// Maximum number of lines to return. Default: 2000.
    #[serde(default = "default_limit")]
    pub limit: usize,
}

const fn default_offset() -> usize { 1 }
const fn default_limit() -> usize { 2000 }

/// Max bytes to check for binary content detection.
const BINARY_CHECK_BYTES: usize = 8192;

/// Max line length before truncation.
const MAX_LINE_LENGTH: usize = 2000;

pub fn tool_definition() -> ToolDefinition {
    ToolDefinition {
        name: "read".to_owned(),
        description: "Read a file with line numbers. Returns content in `cat -n` format. \
            Supports offset and limit for large files. Detects binary files."
            .to_owned(),
        input_schema: serde_json::json!({
            "type": "object",
            "properties": {
                "filePath": {
                    "type": "string",
                    "description": "Path to the file to read"
                },
                "offset": {
                    "type": "integer",
                    "description": "Starting line number (1-based, default: 1)",
                    "default": 1,
                    "minimum": 1
                },
                "limit": {
                    "type": "integer",
                    "description": "Maximum number of lines to return (default: 2000)",
                    "default": 2000,
                    "minimum": 1
                }
            },
            "required": ["filePath"]
        }),
    }
}

/// Execute the read tool.
pub fn execute(workspace: &Path, arguments: serde_json::Value) -> Result<ToolCallResult> {
    let params: ReadParams =
        serde_json::from_value(arguments).context("invalid read parameters")?;

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

    if !file_path.exists() {
        return Ok(ToolCallResult {
            content: vec![ContentItem {
                content_type: "text".to_owned(),
                text: format!("Error: file not found: {}", file_path.display()),
            }],
            is_error: true,
        });
    }

    // Binary detection: only read first 8KB to check for null bytes,
    // avoiding loading entire large binary files into memory.
    {
        let mut file = std::fs::File::open(&file_path)
            .with_context(|| format!("failed to open {}", file_path.display()))?;
        let mut check_buf = vec![0u8; BINARY_CHECK_BYTES];
        let n = file
            .by_ref()
            .take(BINARY_CHECK_BYTES as u64)
            .read(&mut check_buf)
            .with_context(|| format!("failed to read {}", file_path.display()))?;
        if check_buf[..n].contains(&0) {
            let file_size = file.metadata().map(|m| m.len()).unwrap_or(0);
            return Ok(ToolCallResult {
                content: vec![ContentItem {
                    content_type: "text".to_owned(),
                    text: format!(
                        "Binary file detected: {} ({} bytes)",
                        file_path.display(),
                        file_size,
                    ),
                }],
                is_error: true,
            });
        }
    }

    // Not binary — read full file as text.
    let content_raw = std::fs::read_to_string(&file_path)
        .with_context(|| format!("failed to read {}", file_path.display()))?;
    let content = std::borrow::Cow::Borrowed(content_raw.as_str());
    let lines: Vec<&str> = content.lines().collect();
    let total_lines = lines.len();

    // Apply offset (1-based) and limit.
    let start = params.offset.saturating_sub(1).min(total_lines);
    let end = (start + params.limit).min(total_lines);

    let mut output = String::new();
    let line_num_width = format!("{}", end).len();

    for (i, line) in lines[start..end].iter().enumerate() {
        let line_num = start + i + 1;
        let truncated = if line.len() > MAX_LINE_LENGTH {
            // Find last valid UTF-8 char boundary at or before MAX_LINE_LENGTH
            // to avoid panicking on multi-byte characters (CJK, emoji, etc).
            let mut end = MAX_LINE_LENGTH;
            while end > 0 && !line.is_char_boundary(end) {
                end -= 1;
            }
            &line[..end]
        } else {
            line
        };
        output.push_str(&format!("{line_num:>line_num_width$}\t{truncated}\n"));
    }

    if end < total_lines {
        output.push_str(&format!(
            "\n... ({} more lines, {} total)\n",
            total_lines - end,
            total_lines
        ));
    }

    Ok(ToolCallResult {
        content: vec![ContentItem {
            content_type: "text".to_owned(),
            text: output,
        }],
        is_error: false,
    })
}
