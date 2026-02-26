//! `oa-coder` — MCP-based coding sub-agent for OpenAcosmi.
//!
//! Provides a set of coding tools exposed via the Model Context Protocol (MCP)
//! over stdio (JSON-RPC 2.0, newline-delimited). Can run standalone as an
//! independent MCP server or be managed by the OpenAcosmi Gateway via
//! `CoderBridge`.
//!
//! # Tools
//!
//! - `edit` — 9-layer fuzzy matching file editor
//! - `read` — File reading with line numbers, offset/limit, binary detection
//! - `write` — File creation/overwrite with directory auto-creation
//! - `grep` — ripgrep (`rg --json`) subprocess wrapper
//! - `glob` — File discovery via globset patterns
//! - `bash` — Sandboxed command execution via oa-sandbox
//!
//! # Architecture
//!
//! ```text
//! stdin (JSON-RPC) → McpServer → ToolRouter → Tool implementations
//!                                                   ↓ (optional)
//!                                              oa-sandbox
//! stdout (JSON-RPC) ←────────────────────────────────┘
//! ```

pub mod edit;
pub mod error;
pub mod server;
pub mod tools;
pub mod util;

pub use error::{CoderError, CoderResult};
pub use server::run_mcp_server;
