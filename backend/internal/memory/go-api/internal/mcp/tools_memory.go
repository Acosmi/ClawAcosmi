// Package mcp — 记忆类 MCP 工具实现。
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/uhms/go-api/internal/services"
)

// ============================================================================
// Tool Implementations — Memory
// ============================================================================

func (s *Server) toolSaveObservation(ctx context.Context, req *sdkmcp.CallToolRequest, input SaveObservationInput) (*sdkmcp.CallToolResult, any, error) {
	if input.UserID == "" {
		input.UserID = "default"
	}
	if input.Content == "" {
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "Error: content is required"}},
			IsError: true,
		}, nil, nil
	}

	var metadata map[string]any
	if input.Category != "" {
		metadata = map[string]any{"category": input.Category}
	}

	memory, err := s.manager.AddObservation(ctx, s.db, input.Content, input.UserID, metadata)
	if err != nil {
		slog.Error("MCP save_observation failed", "error", err)
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "Error: failed to save observation"}},
			IsError: true,
		}, nil, nil
	}

	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{
			Text: fmt.Sprintf("Saved observation with ID %s. Category: %s, Importance: %.2f", memory.ID, memory.Category, memory.ImportanceScore),
		}},
	}, nil, nil
}

func (s *Server) toolRecallMemory(ctx context.Context, req *sdkmcp.CallToolRequest, input RecallMemoryInput) (*sdkmcp.CallToolResult, any, error) {
	if input.UserID == "" {
		input.UserID = "default"
	}
	if input.Limit <= 0 {
		input.Limit = 5
	}
	if input.Query == "" {
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "Error: query is required"}},
			IsError: true,
		}, nil, nil
	}

	results, err := s.manager.SearchMemories(ctx, s.db, input.Query, input.UserID, input.Limit, nil, 0, input.Category, nil, nil)
	if err != nil {
		slog.Error("MCP recall_memory failed", "error", err)
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "Error: failed to recall memories"}},
			IsError: true,
		}, nil, nil
	}

	if len(results) == 0 {
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "No relevant memories found."}},
		}, nil, nil
	}

	var parts []string
	for _, r := range results {
		parts = append(parts, fmt.Sprintf("[Score: %.2f] [%s/%s] %s", r.Score, r.MemoryType, r.Category, r.Content))
	}

	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{
			Text: fmt.Sprintf("Found %d relevant memories:\n\n%s", len(results), strings.Join(parts, "\n\n")),
		}},
	}, nil, nil
}

func (s *Server) toolReflect(ctx context.Context, req *sdkmcp.CallToolRequest, input ReflectInput) (*sdkmcp.CallToolResult, any, error) {
	if input.UserID == "" {
		input.UserID = "default"
	}

	reflection, err := s.manager.TriggerReflection(ctx, s.db, input.UserID)
	if err != nil {
		slog.Error("MCP reflect failed", "error", err)
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "Error: failed to generate reflection"}},
			IsError: true,
		}, nil, nil
	}

	if reflection == nil {
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "Not enough recent observations for reflection."}},
		}, nil, nil
	}

	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{
			Text: "Reflection generated:\n\n" + reflection.Content,
		}},
	}, nil, nil
}

func (s *Server) toolGetKnowledgeGraph(ctx context.Context, req *sdkmcp.CallToolRequest, input GetKnowledgeGraphInput) (*sdkmcp.CallToolResult, any, error) {
	if input.UserID == "" {
		input.UserID = "default"
	}

	entities, relations, err := s.graphStore.GetGraph(s.db, input.UserID)
	if err != nil {
		slog.Error("MCP get_knowledge_graph failed", "error", err)
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "Error: failed to retrieve knowledge graph"}},
			IsError: true,
		}, nil, nil
	}

	if len(entities) == 0 {
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "No knowledge graph data found."}},
		}, nil, nil
	}

	// Group entities by type
	byType := make(map[string][]string)
	for _, e := range entities {
		byType[e.EntityType] = append(byType[e.EntityType], e.Name)
	}

	var summary []string
	for eType, names := range byType {
		summary = append(summary, fmt.Sprintf("**%s**: %s", eType, strings.Join(names, ", ")))
	}

	// Sample relations (up to 10)
	var relSummary []string
	maxRels := 10
	if len(relations) < maxRels {
		maxRels = len(relations)
	}
	for _, r := range relations[:maxRels] {
		relSummary = append(relSummary, fmt.Sprintf("  %s --[%s]--> %s", r.SourceID, r.RelationType, r.TargetID))
	}

	text := fmt.Sprintf("Knowledge Graph (%d entities, %d relations):\n\n%s",
		len(entities), len(relations), strings.Join(summary, "\n"))
	if len(relSummary) > 0 {
		text += "\n\nSample relations:\n" + strings.Join(relSummary, "\n")
	}

	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: text}},
	}, nil, nil
}

func (s *Server) toolSavePlan(ctx context.Context, req *sdkmcp.CallToolRequest, input SavePlanInput) (*sdkmcp.CallToolResult, any, error) {
	if input.UserID == "" {
		input.UserID = "default"
	}
	if input.Priority <= 0 {
		input.Priority = 2
	}
	if input.Content == "" {
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "Error: content is required"}},
			IsError: true,
		}, nil, nil
	}

	metadata := map[string]any{
		"priority": input.Priority,
		"status":   "todo",
	}

	memory, err := s.manager.AddMemory(ctx, s.db, input.Content, input.UserID, "plan", nil, metadata)
	if err != nil {
		slog.Error("MCP save_plan failed", "error", err)
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "Error: failed to save plan"}},
			IsError: true,
		}, nil, nil
	}

	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{
			Text: fmt.Sprintf("Saved plan with ID %s. Priority: %d", memory.ID, input.Priority),
		}},
	}, nil, nil
}

func (s *Server) toolGetSystemMetrics(ctx context.Context, req *sdkmcp.CallToolRequest, input GetSystemMetricsInput) (*sdkmcp.CallToolResult, any, error) {
	metrics := services.GetMetrics()
	summary := metrics.GetSummary()

	data, _ := json.MarshalIndent(summary, "", "  ")
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{
			Text: "System Metrics:\n\n" + string(data),
		}},
	}, nil, nil
}
