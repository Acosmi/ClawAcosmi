//! MCP server auto-install for OpenAcosmi.
//!
//! Given a git URL or Release URL, automatically clone/download, detect project
//! type, build, register and manage MCP servers.
//!
//! Rust CLI handles acquisition/build; Go Gateway handles runtime lifecycle.

pub mod builder;
pub mod detect;
pub mod downloader;
pub mod error;
pub mod import;
pub mod manifest;
pub mod registry;
pub mod service;
pub mod types;
pub mod url_parser;

pub use error::{McpInstallError, Result};
pub use types::*;
