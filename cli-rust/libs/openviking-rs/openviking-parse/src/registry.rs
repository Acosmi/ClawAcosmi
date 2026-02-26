// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Parser registry — extension-based lookup and management.
//!
//! Ported from `openviking/parse/registry.py` (291L).
//! Concrete parsers (PDF, Word, HTML, etc.) are **not** registered by default;
//! this is a skeleton that consumers populate at runtime.

use std::collections::HashMap;
use std::sync::Arc;

use log::info;

use crate::types::{BoxError, CustomParser, ParseResult, Parser};

// ---------------------------------------------------------------------------
// ParserRegistry
// ---------------------------------------------------------------------------

/// Registry for document parsers.
///
/// Automatically selects the appropriate parser based on file extension.
/// Unlike the Python global-singleton version, this is an owned value that
/// callers construct and pass around explicitly.
pub struct ParserRegistry {
    /// parser_name → parser instance
    parsers: HashMap<String, Arc<dyn Parser>>,
    /// file_extension (lower) → parser_name
    extension_map: HashMap<String, String>,
}

impl ParserRegistry {
    /// Create an empty registry (no default parsers registered).
    pub fn new() -> Self {
        Self {
            parsers: HashMap::new(),
            extension_map: HashMap::new(),
        }
    }

    /// Register a parser by name.
    ///
    /// Any file extensions reported by `parser.supported_extensions()` are
    /// automatically mapped to this parser name.
    pub fn register(&mut self, name: impl Into<String>, parser: Arc<dyn Parser>) {
        let name = name.into();
        for ext in parser.supported_extensions() {
            self.extension_map.insert(ext.to_lowercase(), name.clone());
        }
        self.parsers.insert(name, parser);
    }

    /// Register a custom parser (wraps [`CustomParser`] into a regular
    /// [`Parser`] via `Arc`).
    ///
    /// If `name` is `None`, an auto-generated name like `custom_0` is used.
    pub fn register_custom(
        &mut self,
        handler: Arc<dyn CustomParser>,
        extensions: Option<Vec<String>>,
        name: Option<String>,
    ) {
        let name = name.unwrap_or_else(|| {
            let custom_count = self.parsers.keys().filter(|n| n.starts_with("custom_")).count();
            format!("custom_{custom_count}")
        });

        // Override extensions if provided.
        if let Some(exts) = extensions {
            for ext in &exts {
                self.extension_map
                    .insert(ext.to_lowercase(), name.clone());
            }
        } else {
            for ext in handler.supported_extensions() {
                self.extension_map
                    .insert(ext.to_lowercase(), name.clone());
            }
        }

        info!(
            "Registered custom parser '{}' for {:?}",
            name,
            handler.supported_extensions()
        );
        self.parsers.insert(name, handler);
    }

    /// Remove a parser from the registry.
    pub fn unregister(&mut self, name: &str) {
        if let Some(parser) = self.parsers.remove(name) {
            for ext in parser.supported_extensions() {
                let ext_lower = ext.to_lowercase();
                if self.extension_map.get(&ext_lower).map(String::as_str) == Some(name) {
                    self.extension_map.remove(&ext_lower);
                }
            }
        }
    }

    /// Get a parser by its registered name.
    pub fn get_parser(&self, name: &str) -> Option<Arc<dyn Parser>> {
        self.parsers.get(name).cloned()
    }

    /// Get the appropriate parser for a file path based on its extension.
    pub fn get_parser_for_file(&self, path: &str) -> Option<Arc<dyn Parser>> {
        let ext = path
            .rsplit_once('.')
            .map(|(_, e)| format!(".{}", e.to_lowercase()))?;
        let parser_name = self.extension_map.get(&ext)?;
        self.parsers.get(parser_name).cloned()
    }

    /// Parse a source (file path) by looking up a parser.
    ///
    /// Falls back to the `"text"` parser if no matching parser is found.
    pub async fn parse(&self, source: &str) -> Result<ParseResult, BoxError> {
        // Try extension-based lookup first.
        if let Some(parser) = self.get_parser_for_file(source) {
            return parser.parse(source).await;
        }

        // Fallback to text parser.
        if let Some(text_parser) = self.parsers.get("text") {
            return text_parser.parse(source).await;
        }

        Err(format!("No parser found for source: {source}").into())
    }

    /// List all registered parser names.
    pub fn list_parsers(&self) -> Vec<String> {
        self.parsers.keys().cloned().collect()
    }

    /// List all supported file extensions.
    pub fn list_supported_extensions(&self) -> Vec<String> {
        self.extension_map.keys().cloned().collect()
    }

    /// Number of registered parsers.
    pub fn len(&self) -> usize {
        self.parsers.len()
    }

    /// Whether the registry is empty.
    pub fn is_empty(&self) -> bool {
        self.parsers.is_empty()
    }
}

impl Default for ParserRegistry {
    fn default() -> Self {
        Self::new()
    }
}
