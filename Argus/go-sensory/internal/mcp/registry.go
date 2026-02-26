// Package mcp implements the Model Context Protocol (MCP) tool layer.
//
// Architecture priorities (Senior Architect Review):
//   - Thin orchestration: tools compose existing services, never duplicate logic
//   - Consumer-side interfaces: dependencies defined as narrow interfaces (§4.4)
//   - Registry pattern: new tools register via Tool interface for O(1) extension
//   - Privacy-first: every tool declares its RiskLevel for ApprovalGateway routing
//
// Tool lifecycle:
//  1. MCP client calls a tool by name
//  2. Registry looks up the Tool and validates input schema
//  3. Tool.Execute() runs the domain logic
//  4. Result is serialized back to MCP protocol
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// ──────────────────────────────────────────────────────────────
// Tool interface — the contract every MCP tool must satisfy
// ──────────────────────────────────────────────────────────────

// ToolCategory groups tools for prioritized scheduling and
// capability negotiation.
type ToolCategory string

const (
	CategoryPerception ToolCategory = "perception" // read-only screen understanding
	CategoryAction     ToolCategory = "action"     // write operations (click, type)
	CategoryTUI        ToolCategory = "tui"        // terminal interaction
	CategoryMacOS      ToolCategory = "macos"      // macOS-specific shortcuts
)

// ToolSchema describes the JSON Schema for a tool's input parameters.
// This is used for MCP capability advertisement and input validation.
type ToolSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]SchemaField `json:"properties,omitempty"`
	Required   []string               `json:"required,omitempty"`
}

// SchemaField describes a single field in a tool's schema.
type SchemaField struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
	Default     any      `json:"default,omitempty"`
}

// RiskLevel mirrors input.ActionRiskLevel but avoids a hard import
// dependency at the interface level.
type RiskLevel int

const (
	RiskLow    RiskLevel = 0
	RiskMedium RiskLevel = 1
	RiskHigh   RiskLevel = 2
)

// ToolResult is the unified return type from Tool.Execute().
type ToolResult struct {
	Content  any    `json:"content"`            // primary result payload
	IsError  bool   `json:"isError,omitempty"`  // true if the tool errored
	Error    string `json:"error,omitempty"`    // error message if IsError
	Metadata any    `json:"metadata,omitempty"` // optional extra data
}

// Tool is the contract every MCP tool must implement.
//
// Design note: we use a single Execute(ctx, params) signature rather
// than per-tool methods so the registry can dispatch generically.
// Individual tool implementations validate and parse params internally.
type Tool interface {
	// Name returns the MCP tool name (e.g. "capture_screen").
	Name() string

	// Description returns a human-readable summary for capability ads.
	Description() string

	// Category returns the tool's category for grouping.
	Category() ToolCategory

	// Risk returns the default risk level for approval routing.
	Risk() RiskLevel

	// InputSchema returns the JSON Schema describing expected parameters.
	InputSchema() ToolSchema

	// Execute runs the tool with the given parameters.
	Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error)
}

// ──────────────────────────────────────────────────────────────
// Registry — tool lookup and capability advertisement
// ──────────────────────────────────────────────────────────────

// Registry holds all registered MCP tools and provides lookup.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry. Panics on duplicate name
// to catch configuration errors at startup, not at runtime.
func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[t.Name()]; exists {
		panic(fmt.Sprintf("mcp: duplicate tool registration: %q", t.Name()))
	}
	r.tools[t.Name()] = t
}

// Get returns a tool by name, or nil if not found.
func (r *Registry) Get(name string) Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// List returns all registered tools, optionally filtered by category.
func (r *Registry) List(category ...ToolCategory) []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	filter := make(map[ToolCategory]bool, len(category))
	for _, c := range category {
		filter[c] = true
	}

	result := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		if len(filter) == 0 || filter[t.Category()] {
			result = append(result, t)
		}
	}
	return result
}

// Capabilities returns the MCP tools/list response payload.
func (r *Registry) Capabilities() []ToolCapability {
	tools := r.List()
	caps := make([]ToolCapability, len(tools))
	for i, t := range tools {
		caps[i] = ToolCapability{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		}
	}
	return caps
}

// ToolCapability is the MCP protocol representation of a tool
// for the tools/list response.
type ToolCapability struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	InputSchema ToolSchema `json:"inputSchema"`
}
