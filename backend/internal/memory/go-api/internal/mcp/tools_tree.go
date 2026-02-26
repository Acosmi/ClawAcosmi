// Package mcp — MemTree 相关 MCP 工具实现。
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ============================================================================
// MemTree Tool Implementations
// ============================================================================

func (s *Server) toolRecallTreeMemory(ctx context.Context, req *sdkmcp.CallToolRequest, input RecallTreeMemoryInput) (*sdkmcp.CallToolResult, any, error) {
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

	results, err := s.treeManager.CollapsedTreeRetrieve(ctx, s.db, input.Query, input.UserID, input.Limit, input.Category)
	if err != nil {
		slog.Error("MCP recall_tree_memory failed", "error", err)
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "Error: failed to recall tree memories"}},
			IsError: true,
		}, nil, nil
	}

	if len(results) == 0 {
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "No memories found in tree for query: " + input.Query}},
		}, nil, nil
	}

	var parts []string
	for _, r := range results {
		depthIndicator := strings.Repeat("  ", r.Depth)
		leafTag := ""
		if !r.IsLeaf {
			leafTag = " [SUMMARY]"
		}
		parts = append(parts, fmt.Sprintf("%s[%.2f] [%s/%s]%s %s",
			depthIndicator, r.Score, r.Category, r.NodeType, leafTag, r.Content))
	}

	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{
			Text: fmt.Sprintf("Found %d tree memories (multi-granularity):\n%s",
				len(results), strings.Join(parts, "\n")),
		}},
	}, nil, nil
}

func (s *Server) toolBrowseMemoryTree(ctx context.Context, req *sdkmcp.CallToolRequest, input BrowseMemoryTreeInput) (*sdkmcp.CallToolResult, any, error) {
	if input.UserID == "" {
		input.UserID = "default"
	}

	var nodeID *uuid.UUID
	if input.NodeID != "" {
		parsed, err := uuid.Parse(input.NodeID)
		if err != nil {
			return &sdkmcp.CallToolResult{
				Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "Error: invalid node_id"}},
				IsError: true,
			}, nil, nil
		}
		nodeID = &parsed
	}

	nodes, err := s.treeManager.GetSubtree(s.db, input.UserID, nodeID, input.Category)
	if err != nil {
		slog.Error("MCP browse_memory_tree failed", "error", err)
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "Error: failed to browse memory tree"}},
			IsError: true,
		}, nil, nil
	}

	if len(nodes) == 0 {
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "No tree nodes found."}},
		}, nil, nil
	}

	// Also get stats
	stats := s.treeManager.GetTreeStats(s.db, input.UserID)

	var parts []string
	for _, n := range nodes {
		childInfo := ""
		if n.ChildrenCount > 0 {
			childInfo = fmt.Sprintf(" (%d children)", n.ChildrenCount)
		}
		parts = append(parts, fmt.Sprintf("[%s] %s: %s%s",
			n.NodeType, n.Category, truncate(n.Content, 100), childInfo))
	}

	statsJSON, _ := json.Marshal(stats)

	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{
			Text: fmt.Sprintf("Memory Tree (%d nodes):\n%s\n\nTree Stats: %s",
				len(nodes), strings.Join(parts, "\n"), string(statsJSON)),
		}},
	}, nil, nil
}

// truncate shortens a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
