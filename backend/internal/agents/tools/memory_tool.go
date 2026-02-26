// tools/memory_tool.go — 记忆搜索/获取工具。
// TS 参考：src/agents/tools/memory-tool.ts (218L)
package tools

import (
	"context"
	"fmt"
)

// MemoryStore 记忆存储接口。
type MemoryStore interface {
	Search(ctx context.Context, query string, limit int) ([]MemoryEntry, error)
	Get(ctx context.Context, id string) (*MemoryEntry, error)
}

// MemoryEntry 记忆条目。
type MemoryEntry struct {
	ID        string  `json:"id"`
	Content   string  `json:"content"`
	Metadata  any     `json:"metadata,omitempty"`
	Score     float64 `json:"score,omitempty"`
	CreatedAt string  `json:"createdAt,omitempty"`
}

// CreateMemorySearchTool 创建记忆搜索工具。
// TS 参考: memory-tool.ts
func CreateMemorySearchTool(store MemoryStore) *AgentTool {
	return &AgentTool{
		Name:        "memory_search",
		Label:       "Memory Search",
		Description: "Search through saved memories and notes using semantic search.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "The search query to find relevant memories",
				},
				"limit": map[string]any{
					"type":        "number",
					"description": "Maximum number of results to return (default: 5)",
				},
			},
			"required": []any{"query"},
		},
		Execute: func(ctx context.Context, toolCallID string, args map[string]any) (*AgentToolResult, error) {
			query, err := ReadStringParam(args, "query", &StringParamOptions{Required: true})
			if err != nil {
				return nil, err
			}

			limit := 5
			if l, ok, _ := ReadNumberParam(args, "limit", &NumberParamOptions{Integer: true}); ok && l > 0 {
				limit = int(l)
			}

			if store == nil {
				return JsonResult(map[string]any{
					"results": []any{},
					"message": "memory store not configured",
				}), nil
			}

			results, err := store.Search(ctx, query, limit)
			if err != nil {
				return nil, fmt.Errorf("memory search failed: %w", err)
			}

			return JsonResult(map[string]any{
				"query":   query,
				"results": results,
				"count":   len(results),
			}), nil
		},
	}
}

// CreateMemoryGetTool 创建记忆获取工具。
func CreateMemoryGetTool(store MemoryStore) *AgentTool {
	return &AgentTool{
		Name:        "memory_get",
		Label:       "Memory Get",
		Description: "Retrieve a specific memory entry by its ID.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "The memory entry ID",
				},
			},
			"required": []any{"id"},
		},
		Execute: func(ctx context.Context, toolCallID string, args map[string]any) (*AgentToolResult, error) {
			id, err := ReadStringParam(args, "id", &StringParamOptions{Required: true})
			if err != nil {
				return nil, err
			}

			if store == nil {
				return nil, fmt.Errorf("memory store not configured")
			}

			entry, err := store.Get(ctx, id)
			if err != nil {
				return nil, fmt.Errorf("memory get failed: %w", err)
			}
			if entry == nil {
				return nil, fmt.Errorf("memory entry not found: %s", id)
			}

			return JsonResult(entry), nil
		},
	}
}
