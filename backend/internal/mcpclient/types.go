package mcpclient

// types.go — MCP (Model Context Protocol) JSON-RPC 2.0 类型定义
// 匹配 Argus server.go 线协议（协议版本 2024-11-05）。

import "encoding/json"

// ---------- JSON-RPC 2.0 基础 ----------

// JSONRPCRequest JSON-RPC 2.0 请求。
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCNotification JSON-RPC 2.0 通知（无 ID）。
type JSONRPCNotification struct {
	JSONRPC string      `json:"jsonrpc"`
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
	MCPProtocolVersion = "2024-11-05"
	JSONRPC2           = "2.0"
)

// ---------- MCP initialize ----------

// MCPInitializeParams initialize 请求参数。
type MCPInitializeParams struct {
	ProtocolVersion string            `json:"protocolVersion"`
	Capabilities    MCPCapabilities   `json:"capabilities"`
	ClientInfo      MCPImplementation `json:"clientInfo"`
}

// MCPCapabilities 客户端能力声明。
type MCPCapabilities struct{}

// MCPImplementation 实现信息。
type MCPImplementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCPInitializeResult initialize 响应。
type MCPInitializeResult struct {
	ProtocolVersion string            `json:"protocolVersion"`
	Capabilities    json.RawMessage   `json:"capabilities"`
	ServerInfo      MCPImplementation `json:"serverInfo"`
}

// ---------- MCP tools/list ----------

// MCPToolsListResult tools/list 响应。
type MCPToolsListResult struct {
	Tools []MCPToolDef `json:"tools"`
}

// MCPToolDef 单个工具定义。
type MCPToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ---------- MCP tools/call ----------

// MCPToolsCallParams tools/call 请求参数。
type MCPToolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// MCPToolsCallResult tools/call 响应。
type MCPToolsCallResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// MCPContent MCP 内容块。
type MCPContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Data string `json:"data,omitempty"` // base64 编码的二进制数据
	MIME string `json:"mimeType,omitempty"`
}
