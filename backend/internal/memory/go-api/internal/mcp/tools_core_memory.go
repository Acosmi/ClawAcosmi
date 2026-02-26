// Package mcp — Core Memory MCP 工具实现。
// 允许 Agent 读取和编辑用户的核心记忆分区（persona/preferences/instructions）。
package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/uhms/go-api/internal/services"
)

// toolCoreMemoryRead 处理 memory_core_read 工具调用。
func (s *Server) toolCoreMemoryRead(ctx context.Context, req *sdkmcp.CallToolRequest, input CoreMemoryReadInput) (*sdkmcp.CallToolResult, any, error) {
	if input.UserID == "" {
		input.UserID = "default"
	}

	cm, err := services.GetCoreMemory(s.db, input.UserID)
	if err != nil {
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: fmt.Sprintf("Error: %v", err)}},
			IsError: true,
		}, nil, nil
	}

	data, _ := json.MarshalIndent(cm, "", "  ")
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: string(data)}},
	}, nil, nil
}

// toolCoreMemoryEdit 处理 memory_core_edit 工具调用。
func (s *Server) toolCoreMemoryEdit(ctx context.Context, req *sdkmcp.CallToolRequest, input CoreMemoryEditInput) (*sdkmcp.CallToolResult, any, error) {
	if input.UserID == "" {
		input.UserID = "default"
	}
	if input.Section == "" {
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "Error: section is required"}},
			IsError: true,
		}, nil, nil
	}
	if input.Content == "" {
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "Error: content is required"}},
			IsError: true,
		}, nil, nil
	}
	if input.Mode == "" {
		input.Mode = "replace"
	}

	var err error
	switch input.Mode {
	case "append":
		err = services.AppendCoreMemory(s.db, input.UserID, input.Section, input.Content, "agent")
	default:
		err = services.UpdateCoreMemory(s.db, input.UserID, input.Section, input.Content, "agent")
	}

	if err != nil {
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: fmt.Sprintf("Error: %v", err)}},
			IsError: true,
		}, nil, nil
	}

	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: fmt.Sprintf("Core memory '%s' updated", input.Section)}},
	}, nil, nil
}
