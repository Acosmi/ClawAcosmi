// Package mcp — 云端 MCP Client（P4-3）。
// 用于连接到租户本地 Agent 的 MCP Server。
package mcp

import (
	"context"
	"fmt"
	"log/slog"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// AgentClient 封装 MCP SDK Client，用于连接远端 Agent。
type AgentClient struct {
	inner    *sdkmcp.Client
	session  *sdkmcp.ClientSession
	tenantID string
}

// NewAgentClient 创建面向指定租户的 MCP Client。
func NewAgentClient(tenantID string) *AgentClient {
	client := sdkmcp.NewClient(&sdkmcp.Implementation{
		Name:    "uhms-cloud",
		Version: "2.0.0-go",
	}, nil)

	return &AgentClient{
		inner:    client,
		tenantID: tenantID,
	}
}

// Connect 通过指定 transport 连接到远端 Agent MCP Server。
func (c *AgentClient) Connect(ctx context.Context, transport sdkmcp.Transport) error {
	session, err := c.inner.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("connect to agent (tenant=%s): %w", c.tenantID, err)
	}
	c.session = session
	slog.Info("Connected to agent MCP server", "tenant_id", c.tenantID)
	return nil
}

// CallTool 调用远端 Agent 上的 MCP tool。
func (c *AgentClient) CallTool(
	ctx context.Context,
	toolName string,
	args map[string]any,
) (*sdkmcp.CallToolResult, error) {
	if c.session == nil {
		return nil, fmt.Errorf("not connected (tenant=%s)", c.tenantID)
	}
	result, err := c.session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		return nil, fmt.Errorf("call tool %q: %w", toolName, err)
	}
	return result, nil
}

// ListTools 列出远端 Agent 上可用的 MCP tools。
func (c *AgentClient) ListTools(ctx context.Context) (*sdkmcp.ListToolsResult, error) {
	if c.session == nil {
		return nil, fmt.Errorf("not connected (tenant=%s)", c.tenantID)
	}
	return c.session.ListTools(ctx, nil)
}

// Close 关闭与 Agent 的连接。
func (c *AgentClient) Close() error {
	if c.session != nil {
		return c.session.Close()
	}
	return nil
}

// TenantID 返回关联的租户 ID。
func (c *AgentClient) TenantID() string {
	return c.tenantID
}
