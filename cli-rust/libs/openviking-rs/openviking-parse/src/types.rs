// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Core parse types: enums, `ResourceNode`, `ParseResult`, and parser traits.
//!
//! Ported from `openviking/parse/base.py` (439L) and `parse/custom.py` (245L).
//! All structs are `Serialize` / `Deserialize` for seamless JSON round-tripping.

use std::collections::HashMap;

use async_trait::async_trait;
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

/// Convenience alias matching the rest of the workspace.
pub type BoxError = Box<dyn std::error::Error + Send + Sync>;

// ---------------------------------------------------------------------------
// Enums
// ---------------------------------------------------------------------------

/// Document node types (v2.0 simplified).
///
/// Only `Root` and `Section` are used. All content (paragraphs, code blocks,
/// tables, lists, etc.) remains in the content string as Markdown.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum NodeType {
    /// Document root node.
    Root,
    /// Section node representing a chapter / heading.
    Section,
}

/// High-level resource category.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ResourceCategory {
    /// Text-based document types.
    Document,
    /// Media types (image, audio, video).
    Media,
}

/// Document format types.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum DocumentType {
    /// PDF document.
    Pdf,
    /// Markdown document.
    Markdown,
    /// Plain text.
    PlainText,
    /// HTML page.
    Html,
}

/// Media format types (future expansion).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum MediaType {
    /// Image (PNG, JPEG, etc.).
    Image,
    /// Audio (MP3, WAV, etc.).
    Audio,
    /// Video (MP4, WebM, etc.).
    Video,
}

/// Media processing strategy.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum MediaStrategy {
    /// Document is image-heavy → full-page VLM processing.
    FullPageVlm,
    /// Document has some images → extract individually.
    Extract,
    /// Document is text-only → no media processing.
    TextOnly,
}

impl MediaStrategy {
    /// Returns the strategy value as a static string slice.
    pub fn as_str(&self) -> &'static str {
        match self {
            Self::FullPageVlm => "full_page_vlm",
            Self::Extract => "extract",
            Self::TextOnly => "text_only",
        }
    }
}

// ---------------------------------------------------------------------------
// Utility functions
// ---------------------------------------------------------------------------

/// Unified media processing strategy calculation.
///
/// # Arguments
/// * `image_count` — Number of images in the document.
/// * `line_count` — Number of text lines in the document.
///
/// # Returns
/// A [`MediaStrategy`] enum variant.
pub fn calculate_media_strategy(image_count: usize, line_count: usize) -> MediaStrategy {
    if line_count > 0 {
        let ratio = image_count as f64 / line_count as f64;
        if ratio > 0.3 || image_count >= 5 {
            return MediaStrategy::FullPageVlm;
        }
    }
    if image_count > 0 {
        MediaStrategy::Extract
    } else {
        MediaStrategy::TextOnly
    }
}

/// Format table data as a Markdown table.
///
/// # Arguments
/// * `rows` — Table data where each inner `Vec` is a row of cell strings.
/// * `has_header` — Whether the first row should be treated as a header.
///
/// # Returns
/// A Markdown-formatted table string. Returns an empty string if `rows` is empty.
pub fn format_table_to_markdown(rows: &[Vec<String>], has_header: bool) -> String {
    if rows.is_empty() {
        return String::new();
    }

    // Determine the maximum number of columns.
    let col_count = rows.iter().map(|r| r.len()).max().unwrap_or(0);
    if col_count == 0 {
        return String::new();
    }

    // Calculate maximum width for each column.
    let mut col_widths = vec![0usize; col_count];
    for row in rows {
        for (i, cell) in row.iter().enumerate() {
            col_widths[i] = col_widths[i].max(cell.len());
        }
    }

    let mut lines = Vec::with_capacity(rows.len() + 1);
    for (row_idx, row) in rows.iter().enumerate() {
        // Pad missing columns with empty strings.
        let cells: Vec<String> = (0..col_count)
            .map(|i| {
                let cell = row.get(i).map_or("", String::as_str);
                format!("{:<width$}", cell, width = col_widths[i])
            })
            .collect();
        lines.push(format!("| {} |", cells.join(" | ")));

        // Separator row after header.
        if row_idx == 0 && has_header && rows.len() > 1 {
            let sep: Vec<String> = col_widths.iter().map(|&w| "-".repeat(w)).collect();
            lines.push(format!("| {} |", sep.join(" | ")));
        }
    }

    lines.join("\n")
}

// ---------------------------------------------------------------------------
// ResourceNode
// ---------------------------------------------------------------------------

/// A node in the document tree structure.
///
/// Three-phase architecture:
/// - Phase 1: `detail_file` stores flat UUID.md filename.
/// - Phase 2: `meta` stores semantic_title, abstract, overview.
/// - Phase 3: `content_path` points to content.md in final directory.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResourceNode {
    /// Node type (root or section).
    #[serde(rename = "type")]
    pub node_type: NodeType,

    /// Phase 1: UUID.md filename (e.g., "a1b2c3d4.md").
    #[serde(skip_serializing_if = "Option::is_none")]
    pub detail_file: Option<String>,

    /// Phase 3: Final content file path (as string URI).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub content_path: Option<String>,

    /// Original title (from heading). `None` means split plain text.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub title: Option<String>,

    /// Hierarchy level (0 = root, 1 = top section, etc.).
    pub level: u32,

    /// Child nodes.
    #[serde(default)]
    pub children: Vec<ResourceNode>,

    /// Metadata bag (semantic_title, abstract, overview, etc.).
    #[serde(default)]
    pub meta: HashMap<String, serde_json::Value>,

    /// Content type: "text" / "image" / "video" / "audio".
    #[serde(default = "default_content_type")]
    pub content_type: String,

    /// Auxiliary file mapping {filename: uuid.ext}.
    #[serde(default)]
    pub auxiliary_files: HashMap<String, String>,
}

fn default_content_type() -> String {
    "text".to_owned()
}

/// Text file extensions recognised for content reading.
const TEXT_EXTENSIONS: &[&str] = &[
    ".md", ".txt", ".text", ".markdown", ".json", ".yaml", ".yml",
];

/// Truncate a string at a safe UTF-8 character boundary.
///
/// If `max_bytes` falls in the middle of a multi-byte character (e.g. CJK),
/// the boundary is moved backward to the nearest valid position.
/// Uses `str::floor_char_boundary` (stable since Rust 1.80).
fn truncate_str(s: &str, max_bytes: usize) -> &str {
    if s.len() <= max_bytes {
        return s;
    }
    let idx = s.floor_char_boundary(max_bytes);
    &s[..idx]
}

impl ResourceNode {
    /// Create a new root node.
    pub fn root() -> Self {
        Self {
            node_type: NodeType::Root,
            detail_file: None,
            content_path: None,
            title: None,
            level: 0,
            children: Vec::new(),
            meta: HashMap::new(),
            content_type: "text".to_owned(),
            auxiliary_files: HashMap::new(),
        }
    }

    /// Create a new section node.
    pub fn section(title: impl Into<String>, level: u32) -> Self {
        Self {
            node_type: NodeType::Section,
            detail_file: None,
            content_path: None,
            title: Some(title.into()),
            level,
            children: Vec::new(),
            meta: HashMap::new(),
            content_type: "text".to_owned(),
            auxiliary_files: HashMap::new(),
        }
    }

    /// Add a child node.
    pub fn add_child(&mut self, child: ResourceNode) {
        self.children.push(child);
    }

    /// Whether the content path points to a text file.
    pub fn is_text_file(&self) -> bool {
        match &self.content_path {
            Some(p) => {
                let lower = p.to_lowercase();
                TEXT_EXTENSIONS.iter().any(|ext| lower.ends_with(ext))
            }
            None => false,
        }
    }

    /// Recursively collect all text from this node and children.
    ///
    /// Note: This only gathers from the `meta["content"]` field or the title,
    /// since direct file IO is not performed in a pure-library context.
    pub fn get_text(&self, include_children: bool) -> String {
        let mut texts = Vec::new();

        // Gather inline content from meta if present.
        if let Some(serde_json::Value::String(c)) = self.meta.get("content") {
            if !c.is_empty() {
                texts.push(c.clone());
            }
        }

        if include_children {
            for child in &self.children {
                let child_text = child.get_text(true);
                if !child_text.is_empty() {
                    texts.push(child_text);
                }
            }
        }

        texts.join("\n")
    }

    /// Generate L0 abstract for this node.
    ///
    /// Prioritises `meta["abstract"]`, then `title`, then truncated content.
    pub fn get_abstract(&self, max_length: usize) -> String {
        if let Some(serde_json::Value::String(a)) = self.meta.get("abstract") {
            return a.clone();
        }

        let raw = if let Some(t) = &self.title {
            t.clone()
        } else if let Some(serde_json::Value::String(c)) = self.meta.get("content") {
            if c.len() > max_length {
                truncate_str(c, max_length).to_owned()
            } else {
                c.clone()
            }
        } else {
            return String::new();
        };

        if raw.len() > max_length {
            format!("{}...", truncate_str(&raw, max_length.saturating_sub(3)))
        } else {
            raw
        }
    }

    /// Generate L1 overview for this node.
    ///
    /// Includes title, content preview, and children summary.
    pub fn get_overview(&self, max_length: usize) -> String {
        if let Some(serde_json::Value::String(o)) = self.meta.get("overview") {
            return o.clone();
        }

        let mut parts = Vec::new();

        if let Some(t) = &self.title {
            parts.push(format!("**{t}**"));
        }

        // Content preview.
        if let Some(serde_json::Value::String(c)) = self.meta.get("content") {
            let preview = if c.len() > 1000 {
                format!("{}...", truncate_str(c, 1000))
            } else {
                c.clone()
            };
            parts.push(preview);
        }

        // Children summary.
        if !self.children.is_empty() {
            parts.push(format!("\n[Contains {} sub-sections]", self.children.len()));
            for child in self.children.iter().take(5) {
                let child_abstract = child.get_abstract(100);
                parts.push(format!("  - {child_abstract}"));
            }
            if self.children.len() > 5 {
                parts.push(format!("  ... and {} more", self.children.len() - 5));
            }
        }

        let overview = parts.join("\n");
        if overview.len() > max_length {
            format!("{}...", truncate_str(&overview, max_length.saturating_sub(3)))
        } else {
            overview
        }
    }

    /// Total number of descendant nodes (recursive).
    pub fn descendant_count(&self) -> usize {
        self.children
            .iter()
            .map(|c| 1 + c.descendant_count())
            .sum()
    }
}

// ---------------------------------------------------------------------------
// ParseResult
// ---------------------------------------------------------------------------

/// Result of parsing a document.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ParseResult {
    /// Document tree root node.
    pub root: ResourceNode,

    /// Source file path.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub source_path: Option<String>,

    /// Temporary directory path (for v4.0 architecture).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub temp_dir_path: Option<String>,

    /// File format (e.g., "pdf", "markdown").
    #[serde(skip_serializing_if = "Option::is_none")]
    pub source_format: Option<String>,

    /// Parser name (e.g., "PDFParser").
    #[serde(skip_serializing_if = "Option::is_none")]
    pub parser_name: Option<String>,

    /// Parser version.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub parser_version: Option<String>,

    /// Parse duration in seconds.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub parse_time: Option<f64>,

    /// Parse timestamp.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub parse_timestamp: Option<DateTime<Utc>>,

    /// Metadata bag.
    #[serde(default)]
    pub meta: HashMap<String, serde_json::Value>,

    /// Warning messages accumulated during parsing.
    #[serde(default)]
    pub warnings: Vec<String>,
}

impl ParseResult {
    /// Whether the parse completed without warnings.
    pub fn success(&self) -> bool {
        self.warnings.is_empty()
    }

    /// Flatten the tree into a `Vec` of all nodes (depth-first).
    pub fn get_all_nodes(&self) -> Vec<&ResourceNode> {
        let mut nodes = Vec::new();
        Self::collect_nodes(&self.root, &mut nodes);
        nodes
    }

    /// Get section nodes within a level range.
    pub fn get_sections(&self, min_level: u32, max_level: u32) -> Vec<&ResourceNode> {
        self.get_all_nodes()
            .into_iter()
            .filter(|n| {
                n.node_type == NodeType::Section && n.level >= min_level && n.level <= max_level
            })
            .collect()
    }

    fn collect_nodes<'a>(node: &'a ResourceNode, out: &mut Vec<&'a ResourceNode>) {
        out.push(node);
        for child in &node.children {
            Self::collect_nodes(child, out);
        }
    }
}

/// Helper function to create a [`ParseResult`] with populated fields.
#[allow(clippy::too_many_arguments)]
pub fn create_parse_result(
    root: ResourceNode,
    source_path: Option<String>,
    source_format: Option<String>,
    parser_name: Option<String>,
    parser_version: Option<String>,
    parse_time: Option<f64>,
    meta: Option<HashMap<String, serde_json::Value>>,
    warnings: Option<Vec<String>>,
) -> ParseResult {
    ParseResult {
        root,
        source_path,
        temp_dir_path: None,
        source_format,
        parser_name,
        parser_version: parser_version.or_else(|| Some("2.0".to_owned())),
        parse_time,
        parse_timestamp: parse_time.map(|_| Utc::now()),
        meta: meta.unwrap_or_default(),
        warnings: warnings.unwrap_or_default(),
    }
}

// ---------------------------------------------------------------------------
// Parser Traits
// ---------------------------------------------------------------------------

/// Async trait for document parsers.
///
/// Implement this trait to add support for parsing a new document format.
#[async_trait]
pub trait Parser: Send + Sync {
    /// Parse a source (file path or URI) and return a [`ParseResult`].
    async fn parse(&self, source: &str) -> Result<ParseResult, BoxError>;

    /// Parse raw content string.
    ///
    /// Default implementation returns an error; override for parsers that
    /// support in-memory content parsing.
    async fn parse_content(
        &self,
        _content: &str,
        _source_path: Option<&str>,
    ) -> Result<ParseResult, BoxError> {
        Err("content parsing not supported by this parser".into())
    }

    /// List of file extensions this parser handles (e.g., `[".md", ".markdown"]`).
    fn supported_extensions(&self) -> Vec<String>;
}

/// Extended parser trait for custom / third-party parsers.
///
/// Adds a `can_handle` check so the registry can probe parsers dynamically.
#[async_trait]
pub trait CustomParser: Parser {
    /// Check if this parser can handle the given source.
    fn can_handle(&self, source: &str) -> bool;
}
