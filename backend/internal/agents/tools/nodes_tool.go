// tools/nodes_tool.go — 节点操作工具。
// TS 参考：src/agents/tools/nodes-tool.ts (491L) + nodes-utils.ts (177L)
package tools

import (
	"context"
	"fmt"
)

// NodeStore 节点存储接口。
type NodeStore interface {
	ListNodes(ctx context.Context, parentID string) ([]NodeInfo, error)
	GetNode(ctx context.Context, nodeID string) (*NodeInfo, error)
	CreateNode(ctx context.Context, node NodeCreateInput) (string, error)
	UpdateNode(ctx context.Context, nodeID string, updates map[string]any) error
	DeleteNode(ctx context.Context, nodeID string) error
	SearchNodes(ctx context.Context, query string, limit int) ([]NodeInfo, error)
}

// NodeInfo 节点信息。
type NodeInfo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content,omitempty"`
	ParentID  string `json:"parentId,omitempty"`
	Type      string `json:"type,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// NodeCreateInput 节点创建输入。
type NodeCreateInput struct {
	Title    string `json:"title"`
	Content  string `json:"content,omitempty"`
	ParentID string `json:"parentId,omitempty"`
	Type     string `json:"type,omitempty"`
}

// CreateNodesTool 创建节点操作工具。
// TS 参考: nodes-tool.ts
func CreateNodesTool(store NodeStore) *AgentTool {
	return &AgentTool{
		Name:        "nodes",
		Label:       "Nodes",
		Description: "Create, read, update, delete, list, and search notes/nodes.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"enum":        []any{"list", "get", "create", "update", "delete", "search"},
					"description": "The node action to perform",
				},
				"node_id":   map[string]any{"type": "string", "description": "Node ID (for get/update/delete)"},
				"parent_id": map[string]any{"type": "string", "description": "Parent node ID (for list/create)"},
				"title":     map[string]any{"type": "string", "description": "Node title (for create/update)"},
				"content":   map[string]any{"type": "string", "description": "Node content (for create/update)"},
				"query":     map[string]any{"type": "string", "description": "Search query (for search)"},
				"limit":     map[string]any{"type": "number", "description": "Max results (for list/search)"},
			},
			"required": []any{"action"},
		},
		Execute: func(ctx context.Context, toolCallID string, args map[string]any) (*AgentToolResult, error) {
			action, err := ReadStringParam(args, "action", &StringParamOptions{Required: true})
			if err != nil {
				return nil, err
			}
			if store == nil {
				return nil, fmt.Errorf("node store not configured")
			}

			switch action {
			case "list":
				parentID, _ := ReadStringParam(args, "parent_id", nil)
				nodes, err := store.ListNodes(ctx, parentID)
				if err != nil {
					return nil, err
				}
				return JsonResult(map[string]any{"nodes": nodes, "count": len(nodes)}), nil
			case "get":
				nodeID, err := ReadStringParam(args, "node_id", &StringParamOptions{Required: true})
				if err != nil {
					return nil, err
				}
				node, err := store.GetNode(ctx, nodeID)
				if err != nil {
					return nil, err
				}
				return JsonResult(node), nil
			case "create":
				title, err := ReadStringParam(args, "title", &StringParamOptions{Required: true})
				if err != nil {
					return nil, err
				}
				content, _ := ReadStringParam(args, "content", nil)
				parentID, _ := ReadStringParam(args, "parent_id", nil)
				id, err := store.CreateNode(ctx, NodeCreateInput{Title: title, Content: content, ParentID: parentID})
				if err != nil {
					return nil, err
				}
				return JsonResult(map[string]any{"status": "created", "id": id}), nil
			case "update":
				nodeID, err := ReadStringParam(args, "node_id", &StringParamOptions{Required: true})
				if err != nil {
					return nil, err
				}
				updates := map[string]any{}
				if t, _ := ReadStringParam(args, "title", nil); t != "" {
					updates["title"] = t
				}
				if c, _ := ReadStringParam(args, "content", nil); c != "" {
					updates["content"] = c
				}
				if err := store.UpdateNode(ctx, nodeID, updates); err != nil {
					return nil, err
				}
				return JsonResult(map[string]any{"status": "updated", "id": nodeID}), nil
			case "delete":
				nodeID, err := ReadStringParam(args, "node_id", &StringParamOptions{Required: true})
				if err != nil {
					return nil, err
				}
				if err := store.DeleteNode(ctx, nodeID); err != nil {
					return nil, err
				}
				return JsonResult(map[string]any{"status": "deleted", "id": nodeID}), nil
			case "search":
				query, err := ReadStringParam(args, "query", &StringParamOptions{Required: true})
				if err != nil {
					return nil, err
				}
				limit := 10
				if l, ok, _ := ReadNumberParam(args, "limit", &NumberParamOptions{Integer: true}); ok {
					limit = int(l)
				}
				nodes, err := store.SearchNodes(ctx, query, limit)
				if err != nil {
					return nil, err
				}
				return JsonResult(map[string]any{"results": nodes, "count": len(nodes)}), nil
			default:
				return nil, fmt.Errorf("unknown node action: %s", action)
			}
		},
	}
}
