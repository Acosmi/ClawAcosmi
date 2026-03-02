package media

// docconv_mcp.go — MCP 文档转换实现（Phase D 新增）
// 通过标准 MCP 协议调用文档转换服务器
// 支持 mcp-pandoc / doc-ops-mcp / mcp-document-converter 等
// MCP tool: convert_document(source_path, target_format, output_path)

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

// MCPDocConverter MCP 协议文档转换器
type MCPDocConverter struct {
	serverName string
	transport  string // "stdio" | "sse"
	command    string // stdio 模式启动命令
	url        string // sse 模式端点 URL
}

// NewMCPDocConverter 创建 MCP 文档转换器
func NewMCPDocConverter(cfg *types.DocConvConfig) *MCPDocConverter {
	return &MCPDocConverter{
		serverName: cfg.MCPServerName,
		transport:  cfg.MCPTransport,
		command:    cfg.MCPCommand,
		url:        cfg.MCPURL,
	}
}

// Name 返回 Provider 名称
func (c *MCPDocConverter) Name() string {
	return "mcp"
}

// SupportedFormats 返回 MCP 模式支持的格式
func (c *MCPDocConverter) SupportedFormats() []string {
	return []string{".pdf", ".docx", ".xlsx", ".pptx", ".html", ".htm", ".txt", ".md"}
}

// Convert 通过 MCP 调用文档转换
// 当前实现：通过 stdio 子进程调用 MCP server 的 convert_document 工具
func (c *MCPDocConverter) Convert(ctx context.Context, data []byte, mimeType, fileName string) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("docconv/mcp: empty document data")
	}

	// 写入临时文件
	tmpDir := os.TempDir()
	ext := filepath.Ext(fileName)
	if ext == "" {
		ext = mimeTypeToDocExt(mimeType)
	}
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("docconv_input_%d%s", os.Getpid(), ext))
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return "", fmt.Errorf("docconv/mcp: write temp file: %w", err)
	}
	defer os.Remove(tmpFile)

	// 输出文件
	outFile := filepath.Join(tmpDir, fmt.Sprintf("docconv_output_%d.md", os.Getpid()))
	defer os.Remove(outFile)

	// 构建 MCP tool call JSON
	toolCall := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "convert_document",
			"arguments": map[string]interface{}{
				"source_path":   tmpFile,
				"target_format": "markdown",
				"output_path":   outFile,
			},
		},
	}
	toolCallJSON, err := json.Marshal(toolCall)
	if err != nil {
		return "", fmt.Errorf("docconv/mcp: marshal tool call: %w", err)
	}

	// 通过 stdio 调用 MCP server
	if c.transport != "stdio" || c.command == "" {
		return "", fmt.Errorf("docconv/mcp: only stdio transport is currently supported, got: %s", c.transport)
	}

	parts := strings.Fields(c.command)
	if len(parts) == 0 {
		return "", fmt.Errorf("docconv/mcp: empty command")
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Stdin = strings.NewReader(string(toolCallJSON) + "\n")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docconv/mcp: server error: %w, output: %s",
			err, truncateString(string(output), 500))
	}

	// 尝试读取输出文件
	if mdData, err := os.ReadFile(outFile); err == nil && len(mdData) > 0 {
		slog.Info("docconv/mcp: conversion complete",
			"input", fileName,
			"output_size", len(mdData),
		)
		return string(mdData), nil
	}

	// fallback: 解析 MCP 响应中的 content
	var resp struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output, &resp); err == nil {
		for _, c := range resp.Result.Content {
			if c.Type == "text" && c.Text != "" {
				return c.Text, nil
			}
		}
	}

	return "", fmt.Errorf("docconv/mcp: no output produced")
}

// TestConnection 测试 MCP server 可用性
func (c *MCPDocConverter) TestConnection(ctx context.Context) error {
	if c.transport == "stdio" {
		if c.command == "" {
			return fmt.Errorf("docconv/mcp: command not set")
		}
		parts := strings.Fields(c.command)
		if _, err := exec.LookPath(parts[0]); err != nil {
			return fmt.Errorf("docconv/mcp: command not found: %s", parts[0])
		}
		return nil
	}
	if c.transport == "sse" {
		if c.url == "" {
			return fmt.Errorf("docconv/mcp: URL not set")
		}
		return nil // SSE 连接在实际调用时验证
	}
	return fmt.Errorf("docconv/mcp: unknown transport: %s", c.transport)
}

// mimeTypeToDocExt MIME 类型转文档扩展名
func mimeTypeToDocExt(mimeType string) string {
	switch mimeType {
	case "application/pdf":
		return ".pdf"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return ".xlsx"
	case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return ".pptx"
	case "text/html":
		return ".html"
	case "text/plain":
		return ".txt"
	case "text/markdown":
		return ".md"
	default:
		return ".bin"
	}
}
