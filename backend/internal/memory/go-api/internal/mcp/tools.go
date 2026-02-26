// Package mcp — Tool/Resource 注册。
//
// 使用 MCP Go SDK 的 mcp.AddTool() 注册所有 UHMS 工具。
// 每个工具的 handler 定义在各自的文件中:
//   - tools_memory.go      — 记忆类工具实现
//   - tools_tree.go        — MemTree 相关工具实现
//   - tools_core_memory.go — Core Memory 工具实现
package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ============================================================================
// Input Types — strongly typed inputs for each tool (SDK uses jsonschema tags)
// ============================================================================

// SaveObservationInput is the input for the save_observation tool.
type SaveObservationInput struct {
	Content  string `json:"content"  jsonschema:"required,description=The observation to save"`
	UserID   string `json:"user_id"  jsonschema:"description=The user ID"`
	Category string `json:"category" jsonschema:"description=Category: preference\\, habit\\, profile\\, skill\\, relationship\\, event\\, opinion\\, fact\\, goal\\, task\\, reminder (auto-detected if omitted)"`
}

// RecallMemoryInput is the input for the recall_memory tool.
type RecallMemoryInput struct {
	Query    string `json:"query"    jsonschema:"required,description=What to search for"`
	UserID   string `json:"user_id"  jsonschema:"description=The user ID"`
	Limit    int    `json:"limit"    jsonschema:"description=Maximum number of memories to return"`
	Category string `json:"category" jsonschema:"description=Filter by category"`
}

// ReflectInput is the input for the reflect_on_memory tool.
type ReflectInput struct {
	UserID string `json:"user_id" jsonschema:"description=The user ID"`
}

// GetKnowledgeGraphInput is the input for the get_knowledge_graph tool.
type GetKnowledgeGraphInput struct {
	UserID string `json:"user_id" jsonschema:"description=The user ID"`
}

// SavePlanInput is the input for the save_plan tool.
type SavePlanInput struct {
	Content  string `json:"content"  jsonschema:"required,description=The plan or action item content"`
	UserID   string `json:"user_id"  jsonschema:"description=The user ID"`
	Priority int    `json:"priority" jsonschema:"description=Priority level (1=high\\, 2=medium\\, 3=low)"`
}

// GetSystemMetricsInput is the input for the get_system_metrics tool (no params).
type GetSystemMetricsInput struct{}

// RecallTreeMemoryInput is the input for the recall_tree_memory tool.
type RecallTreeMemoryInput struct {
	Query    string `json:"query"    jsonschema:"required,description=What to search for in the memory tree"`
	UserID   string `json:"user_id"  jsonschema:"description=The user ID"`
	Limit    int    `json:"limit"    jsonschema:"description=Maximum results to return"`
	Category string `json:"category" jsonschema:"description=Filter by category subtree"`
}

// BrowseMemoryTreeInput is the input for the browse_memory_tree tool.
type BrowseMemoryTreeInput struct {
	UserID   string `json:"user_id"  jsonschema:"description=The user ID"`
	Category string `json:"category" jsonschema:"description=Filter by category"`
	NodeID   string `json:"node_id"  jsonschema:"description=Parent node ID to browse children of"`
}

// CoreMemoryReadInput is the input for the memory_core_read tool.
type CoreMemoryReadInput struct {
	UserID string `json:"user_id" jsonschema:"description=The user ID"`
}

// CoreMemoryEditInput is the input for the memory_core_edit tool.
type CoreMemoryEditInput struct {
	UserID  string `json:"user_id" jsonschema:"description=The user ID"`
	Section string `json:"section" jsonschema:"required,description=Section to edit: persona\\, preferences\\, or instructions"`
	Content string `json:"content" jsonschema:"required,description=New content for the section"`
	Mode    string `json:"mode"    jsonschema:"description=Edit mode: replace (default) or append"`
}

// ============================================================================
// Tool & Resource Registration
// ============================================================================

func (s *Server) registerTools() {
	sdkmcp.AddTool(s.inner, &sdkmcp.Tool{
		Name:        "save_observation",
		Description: "Save a new observation to memory. Use this when you learn something new about the user or their context.",
	}, s.toolSaveObservation)

	sdkmcp.AddTool(s.inner, &sdkmcp.Tool{
		Name:        "recall_memory",
		Description: "Search and retrieve relevant memories. Use this when you need to remember something about the user or their context.",
	}, s.toolRecallMemory)

	sdkmcp.AddTool(s.inner, &sdkmcp.Tool{
		Name:        "reflect_on_memory",
		Description: "Trigger reflection to synthesize recent observations into higher-level insights.",
	}, s.toolReflect)

	sdkmcp.AddTool(s.inner, &sdkmcp.Tool{
		Name:        "get_knowledge_graph",
		Description: "Get the user's knowledge graph containing entities and their relationships.",
	}, s.toolGetKnowledgeGraph)

	sdkmcp.AddTool(s.inner, &sdkmcp.Tool{
		Name:        "save_plan",
		Description: "Save a plan/action item to memory. Use this for tasks, goals, and action items.",
	}, s.toolSavePlan)

	sdkmcp.AddTool(s.inner, &sdkmcp.Tool{
		Name:        "get_system_metrics",
		Description: "Get system metrics and health information for the memory system.",
	}, s.toolGetSystemMetrics)

	sdkmcp.AddTool(s.inner, &sdkmcp.Tool{
		Name:        "recall_tree_memory",
		Description: "Search memories using hierarchical tree retrieval. Returns multi-granularity results: both high-level summaries and specific details.",
	}, s.toolRecallTreeMemory)

	sdkmcp.AddTool(s.inner, &sdkmcp.Tool{
		Name:        "browse_memory_tree",
		Description: "Browse the user's memory tree structure. Shows category subtrees or children of a specific node.",
	}, s.toolBrowseMemoryTree)

	sdkmcp.AddTool(s.inner, &sdkmcp.Tool{
		Name:        "memory_core_read",
		Description: "Read the user's core memory (persona, preferences, instructions). Core memory persists across conversations.",
	}, s.toolCoreMemoryRead)

	sdkmcp.AddTool(s.inner, &sdkmcp.Tool{
		Name:        "memory_core_edit",
		Description: "Edit the user's core memory. Use to update persona, preferences, or instructions that persist across conversations.",
	}, s.toolCoreMemoryEdit)
}

func (s *Server) registerResources() {
	s.inner.AddResource(&sdkmcp.Resource{
		URI:         "memory://recent",
		Name:        "Recent Memories",
		Description: "Recent memories across all types",
		MIMEType:    "application/json",
	}, s.resourceRecentMemories)
}

// ============================================================================
// Resource Handler
// ============================================================================

func (s *Server) resourceRecentMemories(ctx context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
	results, err := s.manager.SearchMemories(ctx, s.db, "", "default", 10, nil, 0, "", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to read recent memories: %w", err)
	}

	type MemorySummary struct {
		ID         string  `json:"id"`
		Content    string  `json:"content"`
		Type       string  `json:"type"`
		Importance float64 `json:"importance"`
	}
	memories := make([]MemorySummary, len(results))
	for i, r := range results {
		memories[i] = MemorySummary{
			ID:         r.MemoryID.String(),
			Content:    r.Content,
			Type:       r.MemoryType,
			Importance: r.Score,
		}
	}

	jsonData, _ := json.MarshalIndent(memories, "", "  ")
	return &sdkmcp.ReadResourceResult{
		Contents: []*sdkmcp.ResourceContents{
			{
				URI:      "memory://recent",
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}
