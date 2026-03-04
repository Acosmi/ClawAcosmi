package media

// docconv_mcp.go — MCP 文档转换实现（Phase D 新增）
// 通过标准 MCP 协议调用文档转换服务器。
// 支持 mcp-pandoc / doc-ops-mcp / mcp-document-converter 等。
//
// 关键改进：
// 1) 复用 stdio MCP 会话，避免每次转换冷启动子进程。
// 2) 会话失效后自动重建并重试 1 次（仅连接类错误）。
// 3) 使用并发安全临时文件创建，规避 os.Getpid() 文件名冲突。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/mcpclient"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

const (
	mcpDocConvToolName            = "convert_document"
	mcpDocConvInitTimeout         = 8 * time.Second
	mcpDocConvToolCallTimeout     = 60 * time.Second
	mcpDocConvToolDiscoverTimeout = 5 * time.Second
)

// mcpDocConvSession 复用的 MCP stdio 会话。
type mcpDocConvSession struct {
	cmd    *exec.Cmd
	client *mcpclient.Client
}

// MCPDocConverter MCP 协议文档转换器
type MCPDocConverter struct {
	serverName string
	transport  string // "stdio" | "sse"
	command    string // stdio 模式启动命令
	url        string // sse 模式端点 URL

	mu      sync.Mutex
	session *mcpDocConvSession
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

// Convert 通过 MCP 调用文档转换。
func (c *MCPDocConverter) Convert(ctx context.Context, data []byte, mimeType, fileName string) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("docconv/mcp: empty document data")
	}
	if err := c.validateStdioConfig(); err != nil {
		return "", err
	}

	ext := filepath.Ext(fileName)
	if ext == "" {
		ext = mimeTypeToDocExt(mimeType)
	}
	inputPath, err := writeTempInputFile("docconv-input", ext, data)
	if err != nil {
		return "", fmt.Errorf("docconv/mcp: create input file: %w", err)
	}
	defer os.Remove(inputPath)

	outputPath, err := createTempOutputPath("docconv-output", ".md")
	if err != nil {
		return "", fmt.Errorf("docconv/mcp: create output file: %w", err)
	}
	defer os.Remove(outputPath)

	argumentsJSON, err := json.Marshal(map[string]interface{}{
		"source_path":   inputPath,
		"target_format": "markdown",
		"output_path":   outputPath,
	})
	if err != nil {
		return "", fmt.Errorf("docconv/mcp: marshal tool arguments: %w", err)
	}

	callCtx, cancel := context.WithTimeout(ctx, mcpDocConvToolCallTimeout)
	defer cancel()

	toolResult, err := c.callConvertTool(callCtx, argumentsJSON)
	if err != nil {
		return "", err
	}

	// 优先读取 output_path
	if mdData, readErr := os.ReadFile(outputPath); readErr == nil {
		outputText := strings.TrimSpace(string(mdData))
		if outputText != "" {
			slog.Info("docconv/mcp: conversion complete",
				"input", fileName,
				"output_size", len(mdData),
			)
			return outputText, nil
		}
	}

	// fallback: 读取 MCP text content
	text := extractToolText(toolResult)
	if text != "" {
		return text, nil
	}

	if toolResult != nil && toolResult.IsError {
		return "", fmt.Errorf("docconv/mcp: convert_document returned error without output")
	}

	return "", fmt.Errorf("docconv/mcp: no output produced")
}

// TestConnection 测试 MCP server 可用性
func (c *MCPDocConverter) TestConnection(ctx context.Context) error {
	if c.transport == "sse" {
		if c.url == "" {
			return fmt.Errorf("docconv/mcp: URL not set")
		}
		return fmt.Errorf("docconv/mcp: sse transport is configured but convert currently supports stdio only")
	}
	if err := c.validateStdioConfig(); err != nil {
		return err
	}
	testCtx, cancel := context.WithTimeout(ctx, mcpDocConvInitTimeout)
	defer cancel()
	_, err := c.getOrCreateSession(testCtx)
	return err
}

func (c *MCPDocConverter) validateStdioConfig() error {
	if c.transport != "stdio" {
		return fmt.Errorf("docconv/mcp: only stdio transport is currently supported, got: %s", c.transport)
	}
	parts := strings.Fields(c.command)
	if len(parts) == 0 {
		return fmt.Errorf("docconv/mcp: empty command")
	}
	if _, err := exec.LookPath(parts[0]); err != nil {
		return fmt.Errorf("docconv/mcp: command not found: %s", parts[0])
	}
	return nil
}

func (c *MCPDocConverter) callConvertTool(ctx context.Context, arguments json.RawMessage) (*mcpclient.MCPToolsCallResult, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		session, err := c.getOrCreateSession(ctx)
		if err != nil {
			return nil, err
		}

		result, callErr := session.client.CallTool(ctx, mcpDocConvToolName, arguments, mcpDocConvToolCallTimeout)
		if callErr == nil {
			return result, nil
		}
		lastErr = callErr

		if !isRecoverableMCPCallError(callErr) || attempt == 1 {
			return nil, fmt.Errorf("docconv/mcp: call %s failed: %w", mcpDocConvToolName, callErr)
		}

		slog.Warn("docconv/mcp: call failed, recreating MCP session",
			"attempt", attempt+1,
			"error", callErr,
		)
		c.resetSession()
	}
	return nil, fmt.Errorf("docconv/mcp: call %s failed: %w", mcpDocConvToolName, lastErr)
}

func isRecoverableMCPCallError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection closed") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "write stdin") ||
		strings.Contains(msg, "client closed") ||
		strings.Contains(msg, "eof")
}

func (c *MCPDocConverter) getOrCreateSession(ctx context.Context) (*mcpDocConvSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.session != nil {
		return c.session, nil
	}

	session, err := c.startSession(ctx)
	if err != nil {
		return nil, err
	}
	c.session = session
	return c.session, nil
}

func (c *MCPDocConverter) startSession(ctx context.Context) (*mcpDocConvSession, error) {
	parts := strings.Fields(c.command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("docconv/mcp: empty command")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stderr = &mcpLogWriter{
		logger: slog.Default(),
		level:  slog.LevelDebug,
		prefix: "docconv/mcp",
	}

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("docconv/mcp: create stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("docconv/mcp: create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("docconv/mcp: start process: %w", err)
	}

	session := &mcpDocConvSession{
		cmd:    cmd,
		client: mcpclient.NewClient(stdinPipe, stdoutPipe),
	}

	initCtx, initCancel := context.WithTimeout(ctx, mcpDocConvInitTimeout)
	defer initCancel()

	initResult, err := session.client.Initialize(initCtx)
	if err != nil {
		closeMCPDocConvSession(session)
		return nil, fmt.Errorf("docconv/mcp: initialize: %w", err)
	}

	toolCount := -1
	listCtx, listCancel := context.WithTimeout(ctx, mcpDocConvToolDiscoverTimeout)
	defer listCancel()
	tools, err := session.client.ListTools(listCtx)
	if err != nil {
		slog.Warn("docconv/mcp: tools/list failed, continue and rely on call-time validation", "error", err)
	} else {
		toolCount = len(tools)
		if !hasMCPTool(tools, mcpDocConvToolName) {
			closeMCPDocConvSession(session)
			return nil, fmt.Errorf("docconv/mcp: tool %q not found", mcpDocConvToolName)
		}
	}

	slog.Info("docconv/mcp: session ready",
		"server_name", initResult.ServerInfo.Name,
		"server_version", initResult.ServerInfo.Version,
		"tool_count", toolCount,
		"pid", cmd.Process.Pid,
	)
	return session, nil
}

func (c *MCPDocConverter) resetSession() {
	c.mu.Lock()
	session := c.session
	c.session = nil
	c.mu.Unlock()
	closeMCPDocConvSession(session)
}

func closeMCPDocConvSession(session *mcpDocConvSession) {
	if session == nil {
		return
	}
	if session.client != nil {
		_ = session.client.Close()
	}
	if session.cmd != nil {
		if session.cmd.Process != nil {
			_ = session.cmd.Process.Kill()
		}
		_ = session.cmd.Wait()
	}
}

func hasMCPTool(tools []mcpclient.MCPToolDef, name string) bool {
	for _, tool := range tools {
		if strings.TrimSpace(tool.Name) == name {
			return true
		}
	}
	return false
}

func extractToolText(result *mcpclient.MCPToolsCallResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	parts := make([]string, 0, len(result.Content))
	for _, block := range result.Content {
		if block.Type != "text" {
			continue
		}
		text := strings.TrimSpace(block.Text)
		if text == "" {
			continue
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n")
}

func writeTempInputFile(prefix, ext string, data []byte) (string, error) {
	pattern := fmt.Sprintf("%s-*%s", sanitizeTempPrefix(prefix), normalizeTempExtension(ext))
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}

	path := f.Name()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func createTempOutputPath(prefix, ext string) (string, error) {
	pattern := fmt.Sprintf("%s-*%s", sanitizeTempPrefix(prefix), normalizeTempExtension(ext))
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	path := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func sanitizeTempPrefix(prefix string) string {
	trimmed := strings.TrimSpace(prefix)
	if trimmed == "" {
		return "tmp"
	}
	return trimmed
}

func normalizeTempExtension(ext string) string {
	trimmed := strings.TrimSpace(ext)
	if trimmed == "" {
		return ".bin"
	}
	if !strings.HasPrefix(trimmed, ".") {
		return "." + trimmed
	}
	return trimmed
}

// mcpLogWriter 将子进程 stderr 接入 slog，避免静默丢失诊断信息。
type mcpLogWriter struct {
	logger *slog.Logger
	level  slog.Level
	prefix string
}

func (w *mcpLogWriter) Write(p []byte) (int, error) {
	if w == nil || w.logger == nil {
		return len(p), nil
	}
	msg := strings.TrimSpace(string(p))
	if msg != "" {
		w.logger.Log(context.Background(), w.level, msg, "component", w.prefix)
	}
	return len(p), nil
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
