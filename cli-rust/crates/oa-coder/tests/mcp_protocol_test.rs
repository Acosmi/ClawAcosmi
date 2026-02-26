//! MCP protocol integration tests.
//!
//! Tests the JSON-RPC 2.0 MCP server by simulating client requests
//! via stdin/stdout pipes.

use serde_json::json;

/// Find the compiled test binary for oa-coder MCP server.
/// We use a helper binary that starts the MCP server.
/// For now, test the protocol types and dispatch logic directly.

#[test]
fn test_json_rpc_request_parsing() {
    let req_json = json!({
        "jsonrpc": "2.0",
        "id": 1,
        "method": "initialize",
        "params": {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {
                "name": "test-client",
                "version": "0.1.0"
            }
        }
    });

    let req: oa_coder::server::JsonRpcRequest =
        serde_json::from_value(req_json).expect("should parse initialize request");

    assert_eq!(req.method, "initialize");
    assert_eq!(req.id, Some(json!(1)));
}

#[test]
fn test_json_rpc_response_serialization() {
    let resp = oa_coder::server::JsonRpcResponse {
        jsonrpc: "2.0".to_owned(),
        id: Some(json!(1)),
        result: Some(json!({"protocolVersion": "2024-11-05"})),
        error: None,
    };

    let json_str = serde_json::to_string(&resp).expect("should serialize");
    assert!(json_str.contains("2024-11-05"));
    assert!(!json_str.contains("error")); // error is None, should be skipped
}

#[test]
fn test_json_rpc_error_response() {
    let resp = oa_coder::server::JsonRpcResponse {
        jsonrpc: "2.0".to_owned(),
        id: Some(json!(2)),
        result: None,
        error: Some(oa_coder::server::JsonRpcError {
            code: -32601,
            message: "method not found".to_owned(),
            data: None,
        }),
    };

    let json_str = serde_json::to_string(&resp).expect("should serialize");
    assert!(json_str.contains("-32601"));
    assert!(json_str.contains("method not found"));
    assert!(!json_str.contains("result")); // result is None, should be skipped
}

#[test]
fn test_tool_definitions_complete() {
    let router = oa_coder::tools::ToolRouter::new(
        std::path::PathBuf::from("/tmp"),
        false,
    );

    let tools = router.list_tools();
    assert_eq!(tools.len(), 6);

    let names: Vec<&str> = tools.iter().map(|t| t.name.as_str()).collect();
    assert!(names.contains(&"edit"));
    assert!(names.contains(&"read"));
    assert!(names.contains(&"write"));
    assert!(names.contains(&"grep"));
    assert!(names.contains(&"glob"));
    assert!(names.contains(&"bash"));

    // Verify each tool has a description and input_schema.
    for tool in &tools {
        assert!(!tool.description.is_empty(), "tool {} missing description", tool.name);
        assert!(tool.input_schema.is_object(), "tool {} missing input_schema", tool.name);
    }
}

#[test]
fn test_tool_call_read_nonexistent() {
    let router = oa_coder::tools::ToolRouter::new(
        std::path::PathBuf::from("/tmp"),
        false,
    );

    let result = router.call_tool("read", json!({
        "filePath": "/tmp/nonexistent_oa_coder_test_file_12345.txt"
    })).expect("should not error");

    assert!(result.is_error);
    assert!(result.content[0].text.contains("not found"));
}

#[test]
fn test_tool_call_unknown() {
    let router = oa_coder::tools::ToolRouter::new(
        std::path::PathBuf::from("/tmp"),
        false,
    );

    let result = router.call_tool("nonexistent_tool", json!({}))
        .expect("should not error");

    assert!(result.is_error);
    assert!(result.content[0].text.contains("Unknown tool"));
}

#[test]
fn test_tool_call_write_and_read() {
    let dir = tempfile::tempdir().expect("tempdir");
    let file_path = dir.path().join("test_write.txt");

    let router = oa_coder::tools::ToolRouter::new(
        dir.path().to_path_buf(),
        false,
    );

    // Write a file.
    let write_result = router.call_tool("write", json!({
        "filePath": file_path.to_str().expect("path"),
        "content": "line1\nline2\nline3\n"
    })).expect("write should succeed");
    assert!(!write_result.is_error);
    assert!(write_result.content[0].text.contains("Created"));

    // Read it back.
    let read_result = router.call_tool("read", json!({
        "filePath": file_path.to_str().expect("path")
    })).expect("read should succeed");
    assert!(!read_result.is_error);
    assert!(read_result.content[0].text.contains("line1"));
    assert!(read_result.content[0].text.contains("line2"));
    assert!(read_result.content[0].text.contains("line3"));
}

#[test]
fn test_tool_call_edit() {
    let dir = tempfile::tempdir().expect("tempdir");
    let file_path = dir.path().join("test_edit.txt");
    std::fs::write(&file_path, "hello world\nfoo bar\n").expect("write");

    let router = oa_coder::tools::ToolRouter::new(
        dir.path().to_path_buf(),
        false,
    );

    let result = router.call_tool("edit", json!({
        "filePath": file_path.to_str().expect("path"),
        "oldString": "foo bar",
        "newString": "baz qux"
    })).expect("edit should succeed");

    assert!(!result.is_error);
    // Verify the diff output.
    assert!(result.content[0].text.contains("-foo bar") || result.content[0].text.contains("+baz qux"));

    // Verify file was actually changed.
    let content = std::fs::read_to_string(&file_path).expect("read");
    assert!(content.contains("baz qux"));
    assert!(!content.contains("foo bar"));
}

#[test]
fn test_tool_call_edit_create_new_file() {
    let dir = tempfile::tempdir().expect("tempdir");
    let file_path = dir.path().join("subdir/new_file.txt");

    let router = oa_coder::tools::ToolRouter::new(
        dir.path().to_path_buf(),
        false,
    );

    let result = router.call_tool("edit", json!({
        "filePath": file_path.to_str().expect("path"),
        "oldString": "",
        "newString": "new content here"
    })).expect("edit should succeed");

    assert!(!result.is_error);
    assert!(result.content[0].text.contains("Created"));

    let content = std::fs::read_to_string(&file_path).expect("read");
    assert_eq!(content, "new content here");
}

#[test]
fn test_tool_call_glob() {
    let dir = tempfile::tempdir().expect("tempdir");
    std::fs::write(dir.path().join("test.rs"), "fn main() {}").expect("write");
    std::fs::write(dir.path().join("test.txt"), "hello").expect("write");
    std::fs::create_dir_all(dir.path().join("sub")).expect("mkdir");
    std::fs::write(dir.path().join("sub/nested.rs"), "mod sub;").expect("write");

    let router = oa_coder::tools::ToolRouter::new(
        dir.path().to_path_buf(),
        false,
    );

    let result = router.call_tool("glob", json!({
        "pattern": "**/*.rs"
    })).expect("glob should succeed");

    assert!(!result.is_error);
    assert!(result.content[0].text.contains("test.rs"));
    assert!(result.content[0].text.contains("nested.rs"));
    assert!(!result.content[0].text.contains("test.txt"));
}

#[test]
fn test_tool_call_grep() {
    let dir = tempfile::tempdir().expect("tempdir");
    std::fs::write(dir.path().join("test.txt"), "hello world\nfoo bar\nhello again\n")
        .expect("write");

    let router = oa_coder::tools::ToolRouter::new(
        dir.path().to_path_buf(),
        false,
    );

    let result = router.call_tool("grep", json!({
        "pattern": "hello",
        "path": dir.path().to_str().expect("path")
    })).expect("grep should succeed");

    assert!(!result.is_error);
    assert!(result.content[0].text.contains("hello"));
}

#[test]
fn test_tool_call_bash() {
    let dir = tempfile::tempdir().expect("tempdir");

    let router = oa_coder::tools::ToolRouter::new(
        dir.path().to_path_buf(),
        false, // not sandboxed
    );

    let result = router.call_tool("bash", json!({
        "command": "echo 'oa-coder-test-output'"
    })).expect("bash should succeed");

    assert!(!result.is_error);
    assert!(result.content[0].text.contains("oa-coder-test-output"));
}
