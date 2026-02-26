package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// ──────────────────────────────────────────────────────────────
// Tests: MCP Server
// ──────────────────────────────────────────────────────────────

// helper: send a JSON-RPC message and return the response
func serverExchange(t *testing.T, srv *Server, input string) jsonRPCResponse {
	t.Helper()
	in := strings.NewReader(input + "\n")
	out := &bytes.Buffer{}
	srv.reader = in
	srv.writer = out

	if err := srv.Run(); err != nil {
		t.Fatalf("Server.Run error: %v", err)
	}

	var resp jsonRPCResponse
	if out.Len() > 0 {
		if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v (raw: %s)", err, out.String())
		}
	}
	return resp
}

// helper: run multiple messages through the server
func serverMultiExchange(t *testing.T, srv *Server, inputs []string) []jsonRPCResponse {
	t.Helper()
	in := strings.NewReader(strings.Join(inputs, "\n") + "\n")
	out := &bytes.Buffer{}
	srv.reader = in
	srv.writer = out

	if err := srv.Run(); err != nil {
		t.Fatalf("Server.Run error: %v", err)
	}

	var responses []jsonRPCResponse
	decoder := json.NewDecoder(bytes.NewReader(out.Bytes()))
	for decoder.More() {
		var resp jsonRPCResponse
		if err := decoder.Decode(&resp); err != nil {
			break
		}
		responses = append(responses, resp)
	}
	return responses
}

// buildRegistry creates a registry with a simple echo tool for testing.
func buildTestRegistry() *Registry {
	r := NewRegistry()
	r.Register(&echoTool{})
	return r
}

type echoTool struct{}

func (t *echoTool) Name() string           { return "echo" }
func (t *echoTool) Description() string    { return "Echoes input" }
func (t *echoTool) Category() ToolCategory { return CategoryPerception }
func (t *echoTool) Risk() RiskLevel        { return RiskLow }
func (t *echoTool) InputSchema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]SchemaField{
			"message": {Type: "string", Description: "Message to echo"},
		},
		Required: []string{"message"},
	}
}
func (t *echoTool) Execute(_ context.Context, params json.RawMessage) (*ToolResult, error) {
	var p struct {
		Message string `json:"message"`
	}
	json.Unmarshal(params, &p)
	return &ToolResult{Content: p.Message}, nil
}

// ── Initialize ──

func TestServer_Initialize(t *testing.T) {
	srv := NewServer(buildTestRegistry())
	resp := serverExchange(t, srv, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test-client","version":"1.0"}}}`)

	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %q, want 2.0", resp.JSONRPC)
	}
	// Check result has serverInfo
	data, _ := json.Marshal(resp.Result)
	var result mcpInitializeResult
	json.Unmarshal(data, &result)
	if result.ServerInfo.Name != mcpServerName {
		t.Errorf("ServerInfo.Name = %q, want %q", result.ServerInfo.Name, mcpServerName)
	}
}

// ── Tools List ──

func TestServer_ToolsList(t *testing.T) {
	reg := buildTestRegistry()
	srv := NewServer(reg)
	srv.initialized = true

	resp := serverExchange(t, srv, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var result mcpToolsListResult
	json.Unmarshal(data, &result)
	if len(result.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(result.Tools))
	}
	if result.Tools[0].Name != "echo" {
		t.Errorf("Tool name = %q, want echo", result.Tools[0].Name)
	}
}

func TestServer_ToolsList_NotInitialized(t *testing.T) {
	srv := NewServer(buildTestRegistry())
	// Don't set initialized = true
	resp := serverExchange(t, srv, `{"jsonrpc":"2.0","id":3,"method":"tools/list"}`)
	if resp.Error == nil {
		t.Error("Expected error for uninitialized server")
	}
}

// ── Tools Call ──

func TestServer_ToolsCall_Success(t *testing.T) {
	srv := NewServer(buildTestRegistry())
	srv.initialized = true

	resp := serverExchange(t, srv, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"echo","arguments":{"message":"hello world"}}}`)
	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var result mcpToolsCallResult
	json.Unmarshal(data, &result)
	if len(result.Content) != 1 {
		t.Fatalf("Expected 1 content block, got %d", len(result.Content))
	}
	if result.Content[0].Text != "hello world" {
		t.Errorf("Content text = %q, want 'hello world'", result.Content[0].Text)
	}
}

func TestServer_ToolsCall_UnknownTool(t *testing.T) {
	srv := NewServer(buildTestRegistry())
	srv.initialized = true

	resp := serverExchange(t, srv, `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"nonexistent"}}`)
	if resp.Error == nil {
		t.Error("Expected error for unknown tool")
	}
}

// ── Error handling ──

func TestServer_InvalidJSON(t *testing.T) {
	srv := NewServer(buildTestRegistry())
	resp := serverExchange(t, srv, `{invalid json}`)
	if resp.Error == nil {
		t.Error("Expected parse error")
	}
	if resp.Error.Code != rpcParseError {
		t.Errorf("Error code = %d, want %d", resp.Error.Code, rpcParseError)
	}
}

func TestServer_InvalidJSONRPCVersion(t *testing.T) {
	srv := NewServer(buildTestRegistry())
	resp := serverExchange(t, srv, `{"jsonrpc":"1.0","id":6,"method":"ping"}`)
	if resp.Error == nil {
		t.Error("Expected invalid request error")
	}
}

func TestServer_MethodNotFound(t *testing.T) {
	srv := NewServer(buildTestRegistry())
	srv.initialized = true
	resp := serverExchange(t, srv, `{"jsonrpc":"2.0","id":7,"method":"unknown/method"}`)
	if resp.Error == nil {
		t.Error("Expected method not found error")
	}
	if resp.Error.Code != rpcMethodNotFound {
		t.Errorf("Error code = %d, want %d", resp.Error.Code, rpcMethodNotFound)
	}
}

// ── Ping ──

func TestServer_Ping(t *testing.T) {
	srv := NewServer(buildTestRegistry())
	resp := serverExchange(t, srv, `{"jsonrpc":"2.0","id":8,"method":"ping"}`)
	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}
}

// ── Multi-message flow ──

func TestServer_FullFlow(t *testing.T) {
	srv := NewServer(buildTestRegistry())

	messages := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo","arguments":{"message":"test flow"}}}`,
	}

	responses := serverMultiExchange(t, srv, messages)
	// Should get 3 responses (no response for notification)
	if len(responses) != 3 {
		t.Fatalf("Expected 3 responses, got %d", len(responses))
	}

	// Response 1: initialize
	if responses[0].Error != nil {
		t.Errorf("Initialize error: %v", responses[0].Error)
	}

	// Response 2: tools/list
	if responses[1].Error != nil {
		t.Errorf("Tools list error: %v", responses[1].Error)
	}

	// Response 3: tools/call
	if responses[2].Error != nil {
		t.Errorf("Tools call error: %v", responses[2].Error)
	}
}

// ── Blank lines ──

func TestServer_SkipsBlankLines(t *testing.T) {
	srv := NewServer(buildTestRegistry())
	messages := []string{
		"",
		`{"jsonrpc":"2.0","id":1,"method":"ping"}`,
		"",
	}
	responses := serverMultiExchange(t, srv, messages)
	if len(responses) != 1 {
		t.Errorf("Expected 1 response, got %d", len(responses))
	}
}

// ── Content conversion ──

func TestResultToMCPContent_String(t *testing.T) {
	content := resultToMCPContent(&ToolResult{Content: "hello"})
	if len(content) != 1 || content[0].Text != "hello" {
		t.Errorf("Unexpected content: %+v", content)
	}
}

func TestResultToMCPContent_Map(t *testing.T) {
	content := resultToMCPContent(&ToolResult{Content: map[string]any{"key": "val"}})
	if len(content) != 1 {
		t.Fatalf("Expected 1 content block, got %d", len(content))
	}
	if !strings.Contains(content[0].Text, "key") {
		t.Errorf("Content should contain 'key': %s", content[0].Text)
	}
}

func TestResultToMCPContent_Error(t *testing.T) {
	content := resultToMCPContent(&ToolResult{IsError: true, Error: "something failed"})
	if len(content) != 1 || content[0].Text != "something failed" {
		t.Errorf("Unexpected error content: %+v", content)
	}
}

// ── Stop ──

func TestServer_Stop(t *testing.T) {
	srv := NewServer(buildTestRegistry())
	srv.Stop() // should not panic
}
