package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
)

// ---------- ACP Client ----------

// AcpClientOptions ACP 客户端选项。
type AcpClientOptions struct {
	// ServerCmd 启动 ACP 服务端的命令。
	ServerCmd string
	// ServerArgs 命令参数。
	ServerArgs []string
	// Env 附加环境变量。
	Env []string
	// OnSessionUpdate 会话更新回调。
	OnSessionUpdate func(notification SessionNotification)
	// OnPermissionRequest 权限请求回调（返回 outcome）。
	OnPermissionRequest func(req RequestPermissionRequest) *PermissionOutcome
	// Verbose 详细日志。
	Verbose bool
}

// AcpClientHandle ACP 客户端句柄。
type AcpClientHandle struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	cancel  context.CancelFunc
	done    chan struct{}
	nextID  int
	verbose bool

	// pendingRequests 等待响应的 RPC 请求。
	pendingMu sync.Mutex
	pending   map[interface{}]chan *NDJSONMessage
}

// CreateAcpClient 启动 ACP 服务端子进程并建立 ndJSON 双向通信。
// 对应 TS: acp/client.ts createAcpClient()
func CreateAcpClient(ctx context.Context, opts AcpClientOptions) (*AcpClientHandle, error) {
	ctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(ctx, opts.ServerCmd, opts.ServerArgs...)
	cmd.Env = append(os.Environ(), opts.Env...)
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start ACP server process: %w", err)
	}

	handle := &AcpClientHandle{
		cmd:     cmd,
		stdin:   stdin,
		cancel:  cancel,
		done:    make(chan struct{}),
		pending: make(map[interface{}]chan *NDJSONMessage),
		verbose: opts.Verbose,
	}

	// 后台读取 stdout（ndJSON 流）
	go handle.readLoop(stdout, opts)

	// 后台等待进程退出
	go func() {
		_ = cmd.Wait()
		close(handle.done)
		// 进程退出时关闭所有 pending 请求
		handle.pendingMu.Lock()
		for id, ch := range handle.pending {
			close(ch)
			delete(handle.pending, id)
		}
		handle.pendingMu.Unlock()
	}()

	return handle, nil
}

// Send 发送 JSON-RPC 请求并等待响应。
func (h *AcpClientHandle) Send(ctx context.Context, method string, params interface{}) (*NDJSONMessage, error) {
	h.mu.Lock()
	h.nextID++
	id := h.nextID
	h.mu.Unlock()

	respCh := make(chan *NDJSONMessage, 1)
	h.pendingMu.Lock()
	h.pending[id] = respCh
	h.pendingMu.Unlock()

	msg := NDJSONMessage{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
	if err := h.writeMessage(&msg); err != nil {
		h.pendingMu.Lock()
		delete(h.pending, id)
		h.pendingMu.Unlock()
		return nil, fmt.Errorf("send RPC %s: %w", method, err)
	}

	select {
	case resp, ok := <-respCh:
		if !ok {
			return nil, fmt.Errorf("RPC %s: connection closed", method)
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("RPC %s error %d: %s", method, resp.Error.Code, resp.Error.Message)
		}
		return resp, nil
	case <-ctx.Done():
		h.pendingMu.Lock()
		delete(h.pending, id)
		h.pendingMu.Unlock()
		return nil, ctx.Err()
	}
}

// SendNotification 发送 JSON-RPC 通知（无 ID，无响应）。
func (h *AcpClientHandle) SendNotification(method string, params interface{}) error {
	msg := NDJSONMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	return h.writeMessage(&msg)
}

// Close 关闭客户端并终止子进程。
func (h *AcpClientHandle) Close() {
	h.cancel()
	_ = h.stdin.Close()
}

// Done 返回进程退出信号。
func (h *AcpClientHandle) Done() <-chan struct{} {
	return h.done
}

// writeMessage 写入一行 ndJSON 消息。
func (h *AcpClientHandle) writeMessage(msg *NDJSONMessage) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal ndJSON: %w", err)
	}
	data = append(data, '\n')
	_, err = h.stdin.Write(data)
	return err
}

// readLoop 从 stdout 读取 ndJSON 消息。
func (h *AcpClientHandle) readLoop(r io.Reader, opts AcpClientOptions) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB max line

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var msg NDJSONMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			if h.verbose {
				log.Printf("[acp-client] invalid ndJSON line: %v", err)
			}
			continue
		}

		// 响应消息（有 ID 且无 method）
		if msg.ID != nil && msg.Method == "" {
			h.pendingMu.Lock()
			// JSON 解码时数字 ID 会解为 float64
			var lookupKey interface{} = msg.ID
			if fid, ok := msg.ID.(float64); ok {
				lookupKey = int(fid)
			}
			ch, ok := h.pending[lookupKey]
			if ok {
				delete(h.pending, lookupKey)
			}
			h.pendingMu.Unlock()
			if ok {
				ch <- &msg
			}
			continue
		}

		// 通知/请求消息
		h.handleIncoming(&msg, opts)
	}
}

// handleIncoming 处理服务端发来的通知/请求。
func (h *AcpClientHandle) handleIncoming(msg *NDJSONMessage, opts AcpClientOptions) {
	switch msg.Method {
	case "notifications/session_update":
		if opts.OnSessionUpdate != nil {
			data, _ := json.Marshal(msg.Params)
			var notification SessionNotification
			if json.Unmarshal(data, &notification) == nil {
				opts.OnSessionUpdate(notification)
			}
		}
	case "requests/permission":
		if opts.OnPermissionRequest != nil && msg.ID != nil {
			data, _ := json.Marshal(msg.Params)
			var req RequestPermissionRequest
			if json.Unmarshal(data, &req) == nil {
				outcome := opts.OnPermissionRequest(req)
				resp := NDJSONMessage{
					JSONRPC: "2.0",
					ID:      msg.ID,
					Result: RequestPermissionResponse{
						Outcome: outcome,
					},
				}
				_ = h.writeMessage(&resp)
			}
		}
	default:
		if h.verbose {
			log.Printf("[acp-client] unhandled incoming method: %s", msg.Method)
		}
	}
}
