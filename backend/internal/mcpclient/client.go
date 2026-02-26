package mcpclient

// client.go — MCP stdio 客户端
// 行分隔 JSON-RPC 2.0，10MB scanner buffer（匹配 Argus 服务端）。
// 单调递增请求 ID + pending map 做请求-响应关联。

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

const (
	maxScannerBuffer = 10 * 1024 * 1024 // 10MB — 匹配 Argus 服务端
	defaultTimeout   = 30 * time.Second
)

// Client MCP stdio 客户端。
type Client struct {
	stdin  io.WriteCloser // 写入子进程 stdin
	stdout io.ReadCloser  // 读取子进程 stdout

	nextID  atomic.Int64
	pending sync.Map // map[int64]chan *JSONRPCResponse

	mu     sync.Mutex
	closed bool

	// 读循环退出信号
	done chan struct{}
}

// NewClient 创建 MCP 客户端。
// stdin: 写入子进程的 stdin 管道
// stdout: 读取子进程的 stdout 管道
func NewClient(stdin io.WriteCloser, stdout io.ReadCloser) *Client {
	c := &Client{
		stdin:  stdin,
		stdout: stdout,
		done:   make(chan struct{}),
	}
	go c.readLoop()
	return c
}

// readLoop 持续读取 stdout，按行解析 JSON-RPC 响应，分发到 pending map。
func (c *Client) readLoop() {
	defer close(c.done)
	scanner := bufio.NewScanner(c.stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), maxScannerBuffer)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var resp JSONRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			continue // 跳过非法行（可能是 stderr 混入或日志）
		}

		// 分发到等待的请求
		if ch, ok := c.pending.LoadAndDelete(resp.ID); ok {
			ch.(chan *JSONRPCResponse) <- &resp
		}
	}
}

// send 发送 JSON-RPC 请求并等待响应。
func (c *Client) send(ctx context.Context, method string, params interface{}) (*JSONRPCResponse, error) {
	id := c.nextID.Add(1)

	req := JSONRPCRequest{
		JSONRPC: JSONRPC2,
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("mcpclient: marshal request: %w", err)
	}
	data = append(data, '\n')

	// 注册 pending
	ch := make(chan *JSONRPCResponse, 1)
	c.pending.Store(id, ch)
	defer c.pending.Delete(id)

	// 写入 stdin
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, fmt.Errorf("mcpclient: client closed")
	}
	_, err = c.stdin.Write(data)
	c.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("mcpclient: write stdin: %w", err)
	}

	// 等待响应
	select {
	case resp := <-ch:
		return resp, nil
	case <-c.done:
		return nil, fmt.Errorf("mcpclient: connection closed")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// notify 发送 JSON-RPC 通知（无需响应）。
func (c *Client) notify(method string, params interface{}) error {
	notif := JSONRPCNotification{
		JSONRPC: JSONRPC2,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("mcpclient: marshal notification: %w", err)
	}
	data = append(data, '\n')

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("mcpclient: client closed")
	}
	_, err = c.stdin.Write(data)
	return err
}

// Initialize 执行 MCP 握手：initialize → notifications/initialized。
func (c *Client) Initialize(ctx context.Context) (*MCPInitializeResult, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	params := MCPInitializeParams{
		ProtocolVersion: MCPProtocolVersion,
		Capabilities:    MCPCapabilities{},
		ClientInfo: MCPImplementation{
			Name:    "openacosmi-gateway",
			Version: "1.0.0",
		},
	}

	resp, err := c.send(ctx, "initialize", params)
	if err != nil {
		return nil, fmt.Errorf("mcpclient: initialize: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("mcpclient: initialize error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	var result MCPInitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("mcpclient: unmarshal initialize result: %w", err)
	}

	// 发送 initialized 通知
	if err := c.notify("notifications/initialized", nil); err != nil {
		return nil, fmt.Errorf("mcpclient: send initialized notification: %w", err)
	}

	return &result, nil
}

// ListTools 发现 Argus 全部工具。
func (c *Client) ListTools(ctx context.Context) ([]MCPToolDef, error) {
	resp, err := c.send(ctx, "tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("mcpclient: tools/list: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("mcpclient: tools/list error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	var result MCPToolsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("mcpclient: unmarshal tools/list: %w", err)
	}

	return result.Tools, nil
}

// CallTool 调用单个 MCP 工具。
func (c *Client) CallTool(ctx context.Context, name string, arguments json.RawMessage, timeout time.Duration) (*MCPToolsCallResult, error) {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	params := MCPToolsCallParams{
		Name:      name,
		Arguments: arguments,
	}

	resp, err := c.send(ctx, "tools/call", params)
	if err != nil {
		return nil, fmt.Errorf("mcpclient: tools/call %s: %w", name, err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("mcpclient: tools/call %s error %d: %s", name, resp.Error.Code, resp.Error.Message)
	}

	var result MCPToolsCallResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("mcpclient: unmarshal tools/call %s: %w", name, err)
	}

	return &result, nil
}

// Ping 健康检查，返回 RTT。
func (c *Client) Ping(ctx context.Context) (time.Duration, error) {
	start := time.Now()
	resp, err := c.send(ctx, "ping", nil)
	if err != nil {
		return 0, fmt.Errorf("mcpclient: ping: %w", err)
	}
	rtt := time.Since(start)

	if resp.Error != nil {
		return 0, fmt.Errorf("mcpclient: ping error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	return rtt, nil
}

// Close 关闭客户端，释放资源。
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return c.stdin.Close()
}

// Done 返回客户端关闭信号通道。
func (c *Client) Done() <-chan struct{} {
	return c.done
}
