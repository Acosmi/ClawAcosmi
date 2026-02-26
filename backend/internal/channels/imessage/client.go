//go:build darwin

package imessage

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// iMessage JSON-RPC 客户端 — 继承自 src/imessage/client.ts (245L)
// 通过 stdio 管道与 `imsg rpc` 子进程通信

// resolveUserPath 展开 ~ 前缀的用户路径（G2 修复：与 TS resolveUserPath 对齐）
func resolveUserPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// IMessageRpcError JSON-RPC 错误
type IMessageRpcError struct {
	Code    int             `json:"code,omitempty"`
	Message string          `json:"message,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// IMessageRpcResponse JSON-RPC 响应
type IMessageRpcResponse struct {
	JSONRPC string            `json:"jsonrpc,omitempty"`
	ID      json.RawMessage   `json:"id,omitempty"`
	Result  json.RawMessage   `json:"result,omitempty"`
	Error   *IMessageRpcError `json:"error,omitempty"`
	Method  string            `json:"method,omitempty"`
	Params  json.RawMessage   `json:"params,omitempty"`
}

// IMessageRpcNotification JSON-RPC 通知（无 id 的消息）
type IMessageRpcNotification struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// NotificationHandler 通知回调
type NotificationHandler func(IMessageRpcNotification)

// IMessageRpcClientOptions 客户端配置
type IMessageRpcClientOptions struct {
	CliPath        string
	DbPath         string
	LogInfo        func(string)
	LogError       func(string)
	OnNotification NotificationHandler
}

// pendingRequest 待处理请求
type pendingRequest struct {
	ch    chan pendingResult
	timer *time.Timer
}

type pendingResult struct {
	result json.RawMessage
	err    error
}

// IMessageRpcClient JSON-RPC 客户端（管理 imsg rpc 子进程）
type IMessageRpcClient struct {
	cliPath        string
	dbPath         string
	logInfo        func(string)
	logError       func(string)
	onNotification NotificationHandler

	mu          sync.Mutex
	pending     map[string]*pendingRequest
	nextID      atomic.Int64
	cmd         *exec.Cmd
	stdin       *json.Encoder
	stdinCloser io.Closer // H11: 保存 stdin 管道用于优雅关闭
	closed      chan struct{}
}

// NewIMessageRpcClient 创建 iMessage RPC 客户端（不启动进程）
func NewIMessageRpcClient(opts IMessageRpcClientOptions) *IMessageRpcClient {
	cliPath := strings.TrimSpace(opts.CliPath)
	if cliPath == "" {
		cliPath = "imsg"
	}
	logInfo := opts.LogInfo
	if logInfo == nil {
		logInfo = func(string) {}
	}
	logError := opts.LogError
	if logError == nil {
		logError = func(string) {}
	}
	return &IMessageRpcClient{
		cliPath:        cliPath,
		dbPath:         resolveUserPath(strings.TrimSpace(opts.DbPath)),
		logInfo:        logInfo,
		logError:       logError,
		onNotification: opts.OnNotification,
		pending:        make(map[string]*pendingRequest),
		closed:         make(chan struct{}),
	}
}

// Start 启动 imsg rpc 子进程
func (c *IMessageRpcClient) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.cmd != nil {
		c.mu.Unlock()
		return nil // 已启动
	}

	args := []string{"rpc"}
	if c.dbPath != "" {
		args = append(args, "--db", c.dbPath)
	}

	cmd := exec.CommandContext(ctx, c.cliPath, args...)
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		c.mu.Unlock()
		return fmt.Errorf("imsg rpc stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		c.mu.Unlock()
		return fmt.Errorf("imsg rpc stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		c.mu.Unlock()
		return fmt.Errorf("imsg rpc stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		c.mu.Unlock()
		return fmt.Errorf("imsg rpc start: %w", err)
	}

	c.cmd = cmd
	c.stdin = json.NewEncoder(stdinPipe)
	c.stdinCloser = stdinPipe // H11: 保存用于优雅关闭
	c.mu.Unlock()

	// stderr 日志 goroutine
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				c.logError(fmt.Sprintf("imsg rpc stderr: %s", line))
			}
		}
	}()

	// stdout 行读取 goroutine
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 最大 1MB 行
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				c.handleLine(line)
			}
		}
	}()

	// 进程退出等待 goroutine
	go func() {
		waitErr := cmd.Wait()
		if waitErr != nil {
			c.failAll(fmt.Errorf("imsg rpc exited: %w", waitErr))
		} else {
			c.failAll(fmt.Errorf("imsg rpc closed"))
		}
		close(c.closed)
	}()

	return nil
}

// Stop 停止 imsg rpc 子进程
// H11: 先关闭 stdin 让进程自然退出，等 500ms 后再 Kill（与 TS child.stdin.end() 一致）
func (c *IMessageRpcClient) Stop() {
	c.mu.Lock()
	cmd := c.cmd
	stdinCloser := c.stdinCloser
	c.cmd = nil
	c.stdinCloser = nil
	c.mu.Unlock()

	if cmd == nil {
		return
	}

	// 先关闭 stdin，让子进程收到 EOF 后自然退出
	if stdinCloser != nil {
		_ = stdinCloser.Close()
	}

	// 等待最多 500ms 让进程优雅退出
	select {
	case <-c.closed:
		return // 进程已自然退出
	case <-time.After(500 * time.Millisecond):
	}

	// 超时后强制 Kill
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}

	// 再等一下确保退出
	select {
	case <-c.closed:
	case <-time.After(200 * time.Millisecond):
	}
}

// WaitForClose 等待子进程退出
func (c *IMessageRpcClient) WaitForClose() {
	<-c.closed
}

// Request 发送 JSON-RPC 请求并等待响应
func (c *IMessageRpcClient) Request(ctx context.Context, method string, params map[string]interface{}, timeoutMs int) (json.RawMessage, error) {
	c.mu.Lock()
	if c.cmd == nil || c.stdin == nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("imsg rpc not running")
	}

	id := c.nextID.Add(1)
	idStr := fmt.Sprintf("%d", id)

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	if params == nil {
		payload["params"] = map[string]interface{}{}
	}

	resultCh := make(chan pendingResult, 1)
	req := &pendingRequest{ch: resultCh}

	if timeoutMs <= 0 {
		timeoutMs = DefaultProbeTimeoutMs
	}
	req.timer = time.AfterFunc(time.Duration(timeoutMs)*time.Millisecond, func() {
		c.mu.Lock()
		if _, ok := c.pending[idStr]; ok {
			delete(c.pending, idStr)
			c.mu.Unlock()
			resultCh <- pendingResult{err: fmt.Errorf("imsg rpc timeout (%s)", method)}
		} else {
			c.mu.Unlock()
		}
	})
	c.pending[idStr] = req

	err := c.stdin.Encode(payload)
	c.mu.Unlock()

	if err != nil {
		c.mu.Lock()
		delete(c.pending, idStr)
		c.mu.Unlock()
		req.timer.Stop()
		return nil, fmt.Errorf("imsg rpc write: %w", err)
	}

	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, idStr)
		c.mu.Unlock()
		req.timer.Stop()
		return nil, ctx.Err()
	case res := <-resultCh:
		return res.result, res.err
	}
}

// handleLine 处理 stdout 中的一行 JSON-RPC 响应
func (c *IMessageRpcClient) handleLine(line string) {
	var resp IMessageRpcResponse
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		c.logError(fmt.Sprintf("imsg rpc: parse failed: %s (line=%s)", err, line))
		return
	}

	// 有 id → 请求响应
	if len(resp.ID) > 0 && string(resp.ID) != "null" {
		idStr := strings.Trim(string(resp.ID), "\"")
		c.mu.Lock()
		req, ok := c.pending[idStr]
		if !ok {
			c.mu.Unlock()
			return
		}
		if req.timer != nil {
			req.timer.Stop()
		}
		delete(c.pending, idStr)
		c.mu.Unlock()

		if resp.Error != nil {
			msg := resp.Error.Message
			if msg == "" {
				msg = "imsg rpc error"
			}
			var suffixes []string
			if resp.Error.Code != 0 {
				suffixes = append(suffixes, fmt.Sprintf("code=%d", resp.Error.Code))
			}
			if len(resp.Error.Data) > 0 && string(resp.Error.Data) != "null" {
				suffixes = append(suffixes, string(resp.Error.Data))
			}
			if len(suffixes) > 0 {
				msg = msg + ": " + strings.Join(suffixes, " ")
			}
			req.ch <- pendingResult{err: fmt.Errorf("%s", msg)}
			return
		}
		req.ch <- pendingResult{result: resp.Result}
		return
	}

	// 无 id + 有 method → 通知
	if resp.Method != "" && c.onNotification != nil {
		c.onNotification(IMessageRpcNotification{
			Method: resp.Method,
			Params: resp.Params,
		})
	}
}

// failAll 关闭所有未完成的 pending 请求
func (c *IMessageRpcClient) failAll(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, req := range c.pending {
		if req.timer != nil {
			req.timer.Stop()
		}
		req.ch <- pendingResult{err: err}
		delete(c.pending, id)
	}
}

// CreateIMessageRpcClient 创建并启动 iMessage RPC 客户端
func CreateIMessageRpcClient(ctx context.Context, opts IMessageRpcClientOptions) (*IMessageRpcClient, error) {
	client := NewIMessageRpcClient(opts)
	if err := client.Start(ctx); err != nil {
		return nil, err
	}
	return client, nil
}
