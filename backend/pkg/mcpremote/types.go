package mcpremote

// types.go — MCP 协议类型定义 (Streamable HTTP transport)
//
// 精简版：仅保留 OpenAcosmi client 需要的类型。
// 协议参考: MCP 2025-11-25 + JSON-RPC 2.0

import "encoding/json"

// ---------- JSON-RPC 2.0 基础 ----------

// JSONRPCRequest JSON-RPC 2.0 请求。
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse JSON-RPC 2.0 响应。
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError JSON-RPC 2.0 错误。
type JSONRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// ---------- MCP 协议常量 ----------

const (
	MCPProtocolVersion = "2025-11-25"
	JSONRPC2           = "2.0"
)

// ---------- MCP initialize ----------

// InitializeParams initialize 请求参数。
type InitializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      Implementation     `json:"clientInfo"`
}

// ClientCapabilities 客户端能力声明。
type ClientCapabilities struct{}

// Implementation 实现信息。
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult initialize 响应。
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      Implementation     `json:"serverInfo"`
}

// ServerCapabilities 服务端能力声明。
type ServerCapabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

// ToolsCapability tools 能力。
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ---------- MCP tools/list ----------

// ToolsListResult tools/list 响应。
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// Tool MCP 工具定义。
type Tool struct {
	Name        string          `json:"name"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ---------- MCP tools/call ----------

// ToolCallParams tools/call 请求参数。
type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// ToolCallResult tools/call 响应。
type ToolCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock MCP 内容块。
type ContentBlock struct {
	Type string `json:"type"`           // "text" | "image" | "resource"
	Text string `json:"text,omitempty"` // type=text 时
	Data string `json:"data,omitempty"` // base64 (type=image)
	MIME string `json:"mimeType,omitempty"`
}
