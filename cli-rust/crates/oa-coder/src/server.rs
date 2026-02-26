//! MCP Server — stdio transport, JSON-RPC 2.0, newline-delimited.
//!
//! Implements the Model Context Protocol (spec 2025-06-18) server over
//! stdin/stdout. Reads JSON-RPC requests from stdin (one per line),
//! dispatches to the tool router, and writes responses to stdout.
//!
//! Protocol flow:
//! 1. Client sends `initialize` → server responds with capabilities
//! 2. Client sends `notifications/initialized`
//! 3. Client sends `tools/list` → server returns tool definitions
//! 4. Client sends `tools/call` → server executes tool and returns result
//! 5. Client closes stdin → server exits

use std::io::{BufRead, Write};
use std::path::PathBuf;

use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};
use tracing::{debug, error, info, warn};

/// Maximum size of a single JSON-RPC line (10 MiB), matching oa-sandbox worker protocol.
const MAX_LINE_BYTES: usize = 10 * 1024 * 1024;

use crate::tools::ToolRouter;

// ---------------------------------------------------------------------------
// JSON-RPC 2.0 types
// ---------------------------------------------------------------------------

/// JSON-RPC 2.0 request.
#[derive(Debug, Deserialize)]
pub struct JsonRpcRequest {
    pub jsonrpc: String,
    pub id: Option<serde_json::Value>,
    pub method: String,
    #[serde(default)]
    pub params: serde_json::Value,
}

/// JSON-RPC 2.0 response.
#[derive(Debug, Serialize)]
pub struct JsonRpcResponse {
    pub jsonrpc: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub id: Option<serde_json::Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub result: Option<serde_json::Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<JsonRpcError>,
}

/// JSON-RPC 2.0 error object.
#[derive(Debug, Serialize)]
pub struct JsonRpcError {
    pub code: i64,
    pub message: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub data: Option<serde_json::Value>,
}

// ---------------------------------------------------------------------------
// MCP protocol types
// ---------------------------------------------------------------------------

/// MCP server info returned in initialize response.
#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct ServerInfo {
    name: String,
    version: String,
}

/// MCP server capabilities.
#[derive(Debug, Serialize)]
struct ServerCapabilities {
    tools: ToolsCapability,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct ToolsCapability {
    list_changed: bool,
}

/// MCP initialize result.
#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct InitializeResult {
    protocol_version: String,
    capabilities: ServerCapabilities,
    server_info: ServerInfo,
}

/// MCP tool definition for tools/list.
#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ToolDefinition {
    pub name: String,
    pub description: String,
    pub input_schema: serde_json::Value,
}

/// MCP tools/list result.
#[derive(Debug, Serialize)]
struct ToolsListResult {
    tools: Vec<ToolDefinition>,
}

/// MCP tools/call params.
#[derive(Debug, Deserialize)]
struct ToolCallParams {
    name: String,
    #[serde(default)]
    arguments: serde_json::Value,
}

/// MCP content item in tools/call response.
#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ContentItem {
    #[serde(rename = "type")]
    pub content_type: String,
    pub text: String,
}

/// MCP tools/call result.
#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ToolCallResult {
    pub content: Vec<ContentItem>,
    #[serde(skip_serializing_if = "std::ops::Not::not")]
    pub is_error: bool,
}

// ---------------------------------------------------------------------------
// MCP Server configuration
// ---------------------------------------------------------------------------

/// Configuration for the MCP server.
#[derive(Debug, Clone)]
pub struct McpServerConfig {
    /// Working directory for file operations.
    pub workspace: PathBuf,
    /// Whether to enable sandboxed execution for bash tool.
    pub sandboxed: bool,
}

impl Default for McpServerConfig {
    fn default() -> Self {
        Self {
            workspace: PathBuf::from("."),
            sandboxed: false,
        }
    }
}

// ---------------------------------------------------------------------------
// Server main loop
// ---------------------------------------------------------------------------

/// Run the MCP server on stdin/stdout.
///
/// Reads JSON-RPC 2.0 requests line-by-line from stdin, dispatches to
/// the tool router, and writes responses to stdout. Exits when stdin
/// is closed.
///
/// # Errors
///
/// Returns an error if stdin/stdout I/O fails fatally.
pub fn run_mcp_server(config: McpServerConfig) -> Result<()> {
    info!(
        workspace = %config.workspace.display(),
        sandboxed = config.sandboxed,
        "oa-coder MCP server starting"
    );

    let router = ToolRouter::new(config.workspace.clone(), config.sandboxed);
    let stdin = std::io::stdin();
    let mut reader = std::io::BufReader::new(stdin.lock());
    let mut stdout = std::io::stdout().lock();
    let mut line_buf = String::new();

    loop {
        line_buf.clear();
        let bytes_read = read_line_limited(&mut reader, &mut line_buf, MAX_LINE_BYTES)
            .context("failed to read from stdin")?;

        // EOF — client closed stdin, clean exit.
        if bytes_read == 0 {
            info!("stdin closed, shutting down");
            break;
        }

        let trimmed = line_buf.trim();
        if trimmed.is_empty() {
            continue;
        }

        debug!(raw = trimmed, "received request");

        let request: JsonRpcRequest = match serde_json::from_str(trimmed) {
            Ok(r) => r,
            Err(e) => {
                warn!(error = %e, "invalid JSON-RPC request");
                let resp = error_response(None, -32700, &format!("parse error: {e}"));
                write_response(&mut stdout, &resp)?;
                continue;
            }
        };

        // JSON-RPC 2.0 spec: "jsonrpc" MUST be exactly "2.0".
        if request.jsonrpc != "2.0" {
            warn!(
                version = request.jsonrpc,
                "invalid JSON-RPC version (expected \"2.0\")"
            );
            let resp = error_response(
                request.id.clone(),
                -32600,
                &format!("invalid request: jsonrpc version must be \"2.0\", got \"{}\"", request.jsonrpc),
            );
            write_response(&mut stdout, &resp)?;
            continue;
        }

        // Notifications (no id) don't require a response.
        let is_notification = request.id.is_none();
        let response = dispatch(&router, &request);

        if is_notification {
            // Per JSON-RPC 2.0 spec, notifications MUST NOT receive a response.
            debug!(method = request.method, "notification handled (no response)");
            continue;
        }

        if let Some(resp) = response {
            write_response(&mut stdout, &resp)?;
        }
    }

    info!("oa-coder MCP server stopped");
    Ok(())
}

/// Dispatch a JSON-RPC request to the appropriate handler.
fn dispatch(router: &ToolRouter, req: &JsonRpcRequest) -> Option<JsonRpcResponse> {
    match req.method.as_str() {
        "initialize" => Some(handle_initialize(req)),
        "notifications/initialized" => {
            info!("client initialized");
            None // notification, no response
        }
        "tools/list" => Some(handle_tools_list(router, req)),
        "tools/call" => Some(handle_tools_call(router, req)),
        "ping" => Some(handle_ping(req)),
        _ => {
            warn!(method = req.method, "unknown method");
            Some(error_response(
                req.id.clone(),
                -32601,
                &format!("method not found: {}", req.method),
            ))
        }
    }
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

fn handle_initialize(req: &JsonRpcRequest) -> JsonRpcResponse {
    let result = InitializeResult {
        protocol_version: "2024-11-05".to_owned(),
        capabilities: ServerCapabilities {
            tools: ToolsCapability {
                list_changed: false,
            },
        },
        server_info: ServerInfo {
            name: "oa-coder".to_owned(),
            version: env!("CARGO_PKG_VERSION").to_owned(),
        },
    };

    success_response(req.id.clone(), &result)
}

fn handle_tools_list(router: &ToolRouter, req: &JsonRpcRequest) -> JsonRpcResponse {
    let tools = router.list_tools();
    let result = ToolsListResult { tools };
    success_response(req.id.clone(), &result)
}

fn handle_tools_call(router: &ToolRouter, req: &JsonRpcRequest) -> JsonRpcResponse {
    let params: ToolCallParams = match serde_json::from_value(req.params.clone()) {
        Ok(p) => p,
        Err(e) => {
            return error_response(
                req.id.clone(),
                -32602,
                &format!("invalid tools/call params: {e}"),
            );
        }
    };

    match router.call_tool(&params.name, params.arguments) {
        Ok(result) => success_response(req.id.clone(), &result),
        Err(e) => {
            error!(tool = params.name, error = %e, "tool call failed");
            let result = ToolCallResult {
                content: vec![ContentItem {
                    content_type: "text".to_owned(),
                    text: format!("Error: {e}"),
                }],
                is_error: true,
            };
            success_response(req.id.clone(), &result)
        }
    }
}

fn handle_ping(req: &JsonRpcRequest) -> JsonRpcResponse {
    success_response(req.id.clone(), &serde_json::json!({}))
}

// ---------------------------------------------------------------------------
// Response helpers
// ---------------------------------------------------------------------------

fn success_response(id: Option<serde_json::Value>, result: &impl Serialize) -> JsonRpcResponse {
    match serde_json::to_value(result) {
        Ok(v) => JsonRpcResponse {
            jsonrpc: "2.0".to_owned(),
            id,
            result: Some(v),
            error: None,
        },
        Err(e) => {
            error!(error = %e, "failed to serialize success response");
            JsonRpcResponse {
                jsonrpc: "2.0".to_owned(),
                id,
                result: None,
                error: Some(JsonRpcError {
                    code: -32603,
                    message: format!("internal error: failed to serialize result: {e}"),
                    data: None,
                }),
            }
        }
    }
}

fn error_response(id: Option<serde_json::Value>, code: i64, message: &str) -> JsonRpcResponse {
    JsonRpcResponse {
        jsonrpc: "2.0".to_owned(),
        id,
        result: None,
        error: Some(JsonRpcError {
            code,
            message: message.to_owned(),
            data: None,
        }),
    }
}

/// Write a JSON-RPC response as a single line to stdout.
fn write_response(out: &mut impl Write, resp: &JsonRpcResponse) -> Result<()> {
    let json = serde_json::to_string(resp).context("failed to serialize response")?;
    debug!(response = json, "sending response");
    out.write_all(json.as_bytes())
        .context("failed to write to stdout")?;
    out.write_all(b"\n")
        .context("failed to write newline to stdout")?;
    out.flush().context("failed to flush stdout")?;
    Ok(())
}

/// Read a line from `reader` into `buf`, stopping at newline or `max_bytes`.
///
/// Returns the number of bytes read (0 = EOF). If the line exceeds `max_bytes`,
/// the excess is consumed and discarded, and an error is returned.
fn read_line_limited(reader: &mut impl BufRead, buf: &mut String, max_bytes: usize) -> Result<usize> {
    let mut total = 0usize;
    loop {
        let available = reader.fill_buf().context("stdin fill_buf failed")?;
        if available.is_empty() {
            return Ok(total); // EOF
        }
        // Find newline position in available data.
        let (consumed, found_newline) = match available.iter().position(|&b| b == b'\n') {
            Some(pos) => (pos + 1, true),
            None => (available.len(), false),
        };
        if total + consumed > max_bytes {
            // Consume everything up to the newline (or buffer end) and error out.
            reader.consume(consumed);
            // Keep consuming until we find a newline or EOF.
            if !found_newline {
                loop {
                    let rest = reader.fill_buf().context("stdin fill_buf failed")?;
                    if rest.is_empty() {
                        break;
                    }
                    let eat = match rest.iter().position(|&b| b == b'\n') {
                        Some(pos) => { let n = pos + 1; reader.consume(n); break; }
                        None => rest.len(),
                    };
                    reader.consume(eat);
                }
            }
            anyhow::bail!("line exceeds maximum size ({max_bytes} bytes)");
        }
        // Safe: we're reading from stdin which should be valid UTF-8 JSON.
        let chunk = std::str::from_utf8(&available[..consumed])
            .context("non-UTF-8 data on stdin")?;
        buf.push_str(chunk);
        total += consumed;
        reader.consume(consumed);
        if found_newline {
            return Ok(total);
        }
    }
}
