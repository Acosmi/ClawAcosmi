package mcpremote

// client.go — MCP Streamable HTTP Client
//
// 使用 HTTP POST + JSON-RPC 2.0 与远程 MCP Server 通信。
// 认证: OAuth Bearer token 注入 via authTransport (custom http.RoundTripper)。
// 方法: Connect(), ListTools(), CallTool(), Close()

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// ---------- 配置 ----------

const (
	defaultCallTimeout = 30 * time.Second
	maxResponseSize    = 10 * 1024 * 1024 // 10MB
)

// ---------- Auth Transport ----------

// authTransport 注入 Bearer token 到每个 HTTP 请求。
type authTransport struct {
	base     http.RoundTripper
	tokenMgr *OAuthTokenManager
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.tokenMgr.GetAccessToken()
	if err != nil {
		return nil, fmt.Errorf("mcpremote: get access token: %w", err)
	}

	// Clone 请求以避免修改原始请求
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+token)
	clone.Header.Set("Content-Type", "application/json")
	clone.Header.Set("Accept", "application/json")

	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(clone)
}

// ---------- Remote Client ----------

// RemoteClient MCP Streamable HTTP 客户端。
type RemoteClient struct {
	endpoint   string
	httpClient *http.Client
	nextID     atomic.Int64
	sessionID  string // MCP session ID (from Mcp-Session header)

	mu     sync.Mutex
	closed bool
}

// NewRemoteClient 创建远程 MCP 客户端。
func NewRemoteClient(endpoint string, tokenMgr *OAuthTokenManager) *RemoteClient {
	transport := &authTransport{
		base:     http.DefaultTransport,
		tokenMgr: tokenMgr,
	}

	return &RemoteClient{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout:   60 * time.Second,
			Transport: transport,
		},
	}
}

// Connect 执行 MCP 握手 (initialize + notifications/initialized)。
func (c *RemoteClient) Connect(ctx context.Context) (*InitializeResult, error) {
	params := InitializeParams{
		ProtocolVersion: MCPProtocolVersion,
		Capabilities:    ClientCapabilities{},
		ClientInfo: Implementation{
			Name:    "openacosmi-mcpremote",
			Version: "1.0.0",
		},
	}

	resp, err := c.send(ctx, "initialize", params)
	if err != nil {
		return nil, fmt.Errorf("mcpremote: initialize: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("mcpremote: initialize error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	var result InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("mcpremote: unmarshal initialize result: %w", err)
	}

	slog.Info("mcpremote: connected",
		"server", result.ServerInfo.Name,
		"version", result.ServerInfo.Version,
		"protocol", result.ProtocolVersion,
	)

	// 发送 initialized 通知
	if err := c.sendNotification(ctx, "notifications/initialized", nil); err != nil {
		slog.Warn("mcpremote: send initialized notification failed", "error", err)
	}

	return &result, nil
}

// ListTools 获取远程工具列表。
func (c *RemoteClient) ListTools(ctx context.Context) ([]Tool, error) {
	resp, err := c.send(ctx, "tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("mcpremote: tools/list: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("mcpremote: tools/list error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	var result ToolsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("mcpremote: unmarshal tools/list: %w", err)
	}

	return result.Tools, nil
}

// CallTool 调用远程工具。
func (c *RemoteClient) CallTool(ctx context.Context, name string, arguments json.RawMessage, timeout time.Duration) (*ToolCallResult, error) {
	if timeout <= 0 {
		timeout = defaultCallTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	params := ToolCallParams{
		Name:      name,
		Arguments: arguments,
	}

	resp, err := c.send(ctx, "tools/call", params)
	if err != nil {
		return nil, fmt.Errorf("mcpremote: tools/call %s: %w", name, err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("mcpremote: tools/call %s error %d: %s", name, resp.Error.Code, resp.Error.Message)
	}

	var result ToolCallResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("mcpremote: unmarshal tools/call %s: %w", name, err)
	}

	return &result, nil
}

// Ping 健康检查 (ping JSON-RPC method)，返回 RTT。
func (c *RemoteClient) Ping(ctx context.Context) (time.Duration, error) {
	start := time.Now()
	resp, err := c.send(ctx, "ping", nil)
	if err != nil {
		return 0, fmt.Errorf("mcpremote: ping: %w", err)
	}
	rtt := time.Since(start)

	if resp.Error != nil {
		return 0, fmt.Errorf("mcpremote: ping error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	return rtt, nil
}

// Close 关闭客户端。
func (c *RemoteClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

// SessionID 返回当前 MCP session ID。
func (c *RemoteClient) SessionID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessionID
}

// ---------- 内部方法 ----------

// send 发送 JSON-RPC 2.0 请求到 MCP endpoint。
func (c *RemoteClient) send(ctx context.Context, method string, params interface{}) (*JSONRPCResponse, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, fmt.Errorf("mcpremote: client closed")
	}
	c.mu.Unlock()

	id := c.nextID.Add(1)

	req := JSONRPCRequest{
		JSONRPC: JSONRPC2,
		ID:      id,
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("mcpremote: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("mcpremote: create http request: %w", err)
	}

	// 附加 session ID（如果有）
	c.mu.Lock()
	sid := c.sessionID
	c.mu.Unlock()
	if sid != "" {
		httpReq.Header.Set("Mcp-Session", sid)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("mcpremote: http request: %w", err)
	}
	defer httpResp.Body.Close()

	// 保存 session ID
	if newSid := httpResp.Header.Get("Mcp-Session"); newSid != "" {
		c.mu.Lock()
		c.sessionID = newSid
		c.mu.Unlock()
	}

	if httpResp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(httpResp.Body, 1024))
		return nil, fmt.Errorf("mcpremote: HTTP %d: %s", httpResp.StatusCode, string(errBody))
	}

	respBody, err := io.ReadAll(io.LimitReader(httpResp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("mcpremote: read response: %w", err)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("mcpremote: unmarshal response: %w", err)
	}

	return &resp, nil
}

// sendNotification 发送 JSON-RPC 2.0 通知（无 ID，不期望响应）。
func (c *RemoteClient) sendNotification(ctx context.Context, method string, params interface{}) error {
	type notification struct {
		JSONRPC string      `json:"jsonrpc"`
		Method  string      `json:"method"`
		Params  interface{} `json:"params,omitempty"`
	}

	notif := notification{
		JSONRPC: JSONRPC2,
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("mcpremote: marshal notification: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("mcpremote: create notification request: %w", err)
	}

	c.mu.Lock()
	sid := c.sessionID
	c.mu.Unlock()
	if sid != "" {
		httpReq.Header.Set("Mcp-Session", sid)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("mcpremote: notification request: %w", err)
	}
	defer httpResp.Body.Close()
	io.Copy(io.Discard, httpResp.Body)

	return nil
}
