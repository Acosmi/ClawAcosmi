package mcpinstall

// types.go — MCP Local Bridge types.
// Mirrors Rust oa-mcp-install types for Go Gateway runtime management.

import (
	"encoding/json"
	"time"
)

// TransportMode MCP server transport mode.
type TransportMode string

const (
	TransportStdio TransportMode = "stdio"
	TransportSSE   TransportMode = "sse"
	TransportHTTP  TransportMode = "http"
)

// BridgeState lifecycle state of a local MCP bridge.
type BridgeState string

const (
	BridgeStateInit     BridgeState = "init"
	BridgeStateStarting BridgeState = "starting"
	BridgeStateReady    BridgeState = "ready"
	BridgeStateDegraded BridgeState = "degraded"
	BridgeStateStopped  BridgeState = "stopped"
)

// InstalledMcpServer mirrors the Rust registry.json entry.
type InstalledMcpServer struct {
	Name         string            `json:"name"`
	SourceURL    string            `json:"source_url"`
	SourceKind   string            `json:"source_kind"`
	ProjectType  string            `json:"project_type"`
	Transport    TransportMode     `json:"transport"`
	BinaryPath   string            `json:"binary_path"`
	Command      *string           `json:"command,omitempty"`
	Args         []string          `json:"args"`
	CloneDir     *string           `json:"clone_dir,omitempty"`
	Env          map[string]string `json:"env"`
	PinnedRef    *string           `json:"pinned_ref,omitempty"`
	SourceCommit *string           `json:"source_commit,omitempty"`
	BinarySHA256 *string           `json:"binary_sha256,omitempty"`
	InstalledAt  string            `json:"installed_at"`
	UpdatedAt    *string           `json:"updated_at,omitempty"`
}

// McpServerRegistry top-level registry structure from registry.json.
type McpServerRegistry struct {
	SchemaVersion int                           `json:"schema_version"`
	Servers       map[string]InstalledMcpServer `json:"servers"`
}

// McpLocalBridgeConfig configuration for a local MCP stdio bridge.
type McpLocalBridgeConfig struct {
	// Server registration info.
	Server InstalledMcpServer
	// HealthInterval ping interval (0 = default 30s).
	HealthInterval time.Duration
}

// McpTool a tool exposed by an MCP server (from tools/list response).
type McpTool struct {
	Name        string          `json:"name"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// McpToolCallResult result of calling an MCP tool.
type McpToolCallResult struct {
	Content []McpToolCallContent `json:"content"`
	IsError bool                 `json:"isError"`
}

// McpToolCallContent a single content block in a tool call result.
type McpToolCallContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	MIME string `json:"mimeType,omitempty"`
	Data string `json:"data,omitempty"`
}

// McpToolCallResultToText converts a tool call result to plain text.
func McpToolCallResultToText(result *McpToolCallResult) string {
	if result == nil {
		return ""
	}
	var sb string
	for _, c := range result.Content {
		switch c.Type {
		case "text":
			if sb != "" {
				sb += "\n"
			}
			sb += c.Text
		case "image":
			if sb != "" {
				sb += "\n"
			}
			sb += "[image: " + c.MIME + "]"
		}
	}
	if result.IsError {
		return "[MCP tool error] " + sb
	}
	return sb
}
