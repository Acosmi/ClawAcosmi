package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

// ──────────────────────────────────────────────────────────────
// JSON-RPC 2.0 protocol types
//
// Implementing the MCP (Model Context Protocol) wire format.
// MCP uses JSON-RPC 2.0 over stdio with Content-Length framing.
// ──────────────────────────────────────────────────────────────

// jsonRPCRequest is an incoming JSON-RPC 2.0 request.
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"` // null for notifications
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// jsonRPCResponse is an outgoing JSON-RPC 2.0 response.
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError is the JSON-RPC 2.0 error object.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// JSON-RPC 2.0 error codes
const (
	rpcParseError     = -32700
	rpcInvalidRequest = -32600
	rpcMethodNotFound = -32601
	rpcInvalidParams  = -32602
	rpcInternalError  = -32603
)

// ──────────────────────────────────────────────────────────────
// MCP protocol types
// ──────────────────────────────────────────────────────────────

// mcpInitializeParams is the MCP initialize request params.
type mcpInitializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities,omitempty"`
	ClientInfo      mcpClientInfo  `json:"clientInfo"`
}

// mcpClientInfo describes the connected MCP client.
type mcpClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// mcpInitializeResult is the MCP initialize response.
type mcpInitializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ServerInfo      mcpServerInfo  `json:"serverInfo"`
}

// mcpServerInfo describes this MCP server.
type mcpServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// mcpToolsListResult is the tools/list response.
type mcpToolsListResult struct {
	Tools []ToolCapability `json:"tools"`
}

// mcpToolsCallParams is the tools/call request.
type mcpToolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// mcpToolsCallResult is the tools/call response.
type mcpToolsCallResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// mcpContent is an MCP content block (text or image).
type mcpContent struct {
	Type string `json:"type"` // "text" or "image"
	Text string `json:"text,omitempty"`
}

// ──────────────────────────────────────────────────────────────
// MCP Server
// ──────────────────────────────────────────────────────────────

const (
	mcpProtocolVersion = "2024-11-05"
	mcpServerName      = "argus-sensory"
	mcpServerVersion   = "1.0.0"
)

// Server is the MCP JSON-RPC 2.0 server.
//
// Architecture: reads JSON-RPC messages from stdin, dispatches
// to method handlers, writes responses to stdout. All logging
// goes to stderr to keep stdout clean for protocol messages.
type Server struct {
	registry    *Registry
	reader      io.Reader
	writer      io.Writer
	mu          sync.Mutex // protects writer
	initialized bool
	clientInfo  mcpClientInfo
	ctx         context.Context
	cancel      context.CancelFunc
}

// ServerOption configures a Server.
type ServerOption func(*Server)

// WithIO sets custom reader/writer (for testing).
func WithIO(r io.Reader, w io.Writer) ServerOption {
	return func(s *Server) {
		s.reader = r
		s.writer = w
	}
}

// NewServer creates an MCP server with the given registry.
func NewServer(registry *Registry, opts ...ServerOption) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{
		registry: registry,
		reader:   os.Stdin,
		writer:   os.Stdout,
		ctx:      ctx,
		cancel:   cancel,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Run starts the MCP server, reading from stdin and writing to stdout.
// Blocks until EOF or context cancellation.
func (s *Server) Run() error {
	// Redirect log output to stderr — stdout is reserved for protocol messages.
	log.SetOutput(os.Stderr)
	log.Printf("[MCP] Server starting (protocol=%s)", mcpProtocolVersion)

	scanner := bufio.NewScanner(s.reader)
	// MCP messages can be large (e.g., base64 images); allow up to 10MB.
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue // skip blank lines
		}

		var req jsonRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(nil, rpcParseError, "Parse error", err.Error())
			continue
		}

		if req.JSONRPC != "2.0" {
			s.sendError(req.ID, rpcInvalidRequest, "Invalid Request", "jsonrpc must be \"2.0\"")
			continue
		}

		// Notifications (no id) — just handle and don't respond
		if req.ID == nil || string(req.ID) == "null" {
			s.handleNotification(req)
			continue
		}

		s.handleRequest(req)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stdin read error: %w", err)
	}

	log.Printf("[MCP] Server shutting down (stdin closed)")
	return nil
}

// Stop cancels the server context and closes the reader to unblock Run().
func (s *Server) Stop() {
	s.cancel()
	// Close the reader (typically os.Stdin) so scanner.Scan() returns false
	// immediately instead of blocking on the next read.
	if c, ok := s.reader.(io.Closer); ok {
		c.Close()
	}
}

// ──────────────────────────────────────────────────────────────
// Request routing
// ──────────────────────────────────────────────────────────────

func (s *Server) handleRequest(req jsonRPCRequest) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "tools/list":
		s.handleToolsList(req)
	case "tools/call":
		s.handleToolsCall(req)
	case "ping":
		s.sendResult(req.ID, map[string]string{})
	default:
		s.sendError(req.ID, rpcMethodNotFound, "Method not found",
			fmt.Sprintf("unknown method: %q", req.Method))
	}
}

func (s *Server) handleNotification(req jsonRPCRequest) {
	switch req.Method {
	case "notifications/initialized":
		log.Printf("[MCP] Client confirmed initialization")
	case "notifications/cancelled":
		log.Printf("[MCP] Client cancelled request")
	default:
		log.Printf("[MCP] Ignoring notification: %s", req.Method)
	}
}

// ──────────────────────────────────────────────────────────────
// MCP method handlers
// ──────────────────────────────────────────────────────────────

func (s *Server) handleInitialize(req jsonRPCRequest) {
	var params mcpInitializeParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendError(req.ID, rpcInvalidParams, "Invalid params", err.Error())
			return
		}
	}

	s.initialized = true
	s.clientInfo = params.ClientInfo
	log.Printf("[MCP] Client connected: %s %s (protocol=%s)",
		params.ClientInfo.Name, params.ClientInfo.Version, params.ProtocolVersion)

	s.sendResult(req.ID, mcpInitializeResult{
		ProtocolVersion: mcpProtocolVersion,
		Capabilities: map[string]any{
			"tools": map[string]any{},
		},
		ServerInfo: mcpServerInfo{
			Name:    mcpServerName,
			Version: mcpServerVersion,
		},
	})
}

func (s *Server) handleToolsList(req jsonRPCRequest) {
	if !s.initialized {
		s.sendError(req.ID, rpcInternalError, "Not initialized",
			"client must send 'initialize' before other requests")
		return
	}

	caps := s.registry.Capabilities()
	s.sendResult(req.ID, mcpToolsListResult{Tools: caps})
}

func (s *Server) handleToolsCall(req jsonRPCRequest) {
	if !s.initialized {
		s.sendError(req.ID, rpcInternalError, "Not initialized",
			"client must send 'initialize' before other requests")
		return
	}

	var params mcpToolsCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, rpcInvalidParams, "Invalid params", err.Error())
		return
	}

	tool := s.registry.Get(params.Name)
	if tool == nil {
		s.sendError(req.ID, rpcInvalidParams, "Unknown tool",
			fmt.Sprintf("tool %q not found", params.Name))
		return
	}

	log.Printf("[MCP] Executing tool: %s (risk=%d)", params.Name, tool.Risk())

	result, err := tool.Execute(s.ctx, params.Arguments)
	if err != nil {
		s.sendError(req.ID, rpcInternalError, "Tool execution failed", err.Error())
		return
	}

	// Convert ToolResult to MCP content format
	content := resultToMCPContent(result)
	s.sendResult(req.ID, mcpToolsCallResult{
		Content: content,
		IsError: result.IsError,
	})
}

// ──────────────────────────────────────────────────────────────
// Response helpers
// ──────────────────────────────────────────────────────────────

func (s *Server) sendResult(id json.RawMessage, result any) {
	s.writeResponse(jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

func (s *Server) sendError(id json.RawMessage, code int, message, data string) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &rpcError{
			Code:    code,
			Message: message,
		},
	}
	if data != "" {
		resp.Error.Data = data
	}
	s.writeResponse(resp)
}

func (s *Server) writeResponse(resp jsonRPCResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("[MCP] Failed to marshal response: %v", err)
		return
	}

	// Write line-delimited JSON
	data = append(data, '\n')
	if _, err := s.writer.Write(data); err != nil {
		log.Printf("[MCP] Failed to write response: %v", err)
	}
}

// resultToMCPContent converts a ToolResult to MCP content blocks.
func resultToMCPContent(result *ToolResult) []mcpContent {
	if result.IsError {
		return []mcpContent{{Type: "text", Text: result.Error}}
	}

	switch v := result.Content.(type) {
	case string:
		return []mcpContent{{Type: "text", Text: v}}
	case map[string]any:
		data, _ := json.MarshalIndent(v, "", "  ")
		return []mcpContent{{Type: "text", Text: string(data)}}
	default:
		data, _ := json.Marshal(v)
		return []mcpContent{{Type: "text", Text: string(data)}}
	}
}
