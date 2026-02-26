package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

// ---------- ACP Server (Agent Side) ----------

// AcpServerHandler ACP 服务端 Agent 接口，由 translator 实现。
// 对应 TS: @agentclientprotocol/sdk Agent 接口。
type AcpServerHandler interface {
	// Initialize 处理初始化请求。
	Initialize(req InitializeRequest) (*InitializeResponse, error)
	// NewSession 创建新会话。
	NewSession(req NewSessionRequest) (*NewSessionResponse, error)
	// LoadSession 加载已有会话。
	LoadSession(req LoadSessionRequest) (*LoadSessionResponse, error)
	// ListSessions 列出会话。
	ListSessions(req ListSessionsRequest) (*ListSessionsResponse, error)
	// Prompt 发送提示词。
	Prompt(ctx context.Context, req PromptRequest) (*PromptResponse, error)
	// Cancel 取消运行。
	Cancel(notif CancelNotification)
	// SetSessionMode 设置会话模式。
	SetSessionMode(req SetSessionModeRequest) (*SetSessionModeResponse, error)
	// Start 启动 Agent（初始化后调用）。
	Start()
}

// AgentSideConnection 表示从 Agent 侧向 Client 发送通知的能力。
// 对应 TS: @agentclientprotocol/sdk AgentSideConnection
type AgentSideConnection struct {
	mu      sync.Mutex
	writer  io.Writer
	verbose bool
}

// NewAgentSideConnection 创建 Agent 侧连接。
func NewAgentSideConnection(writer io.Writer, verbose bool) *AgentSideConnection {
	return &AgentSideConnection{
		writer:  writer,
		verbose: verbose,
	}
}

// SendSessionUpdate 向客户端发送会话更新通知。
func (c *AgentSideConnection) SendSessionUpdate(notification SessionNotification) {
	c.sendNotification("notifications/session_update", notification)
}

// SendRequestPermission 向客户端发送权限请求通知。
func (c *AgentSideConnection) SendRequestPermission(req RequestPermissionRequest) {
	c.sendNotification("notifications/permission_request", req)
}

func (c *AgentSideConnection) sendNotification(method string, params interface{}) {
	msg := NDJSONMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		if c.verbose {
			log.Printf("[acp-server] marshal notification %s failed: %v", method, err)
		}
		return
	}
	data = append(data, '\n')
	if _, err := c.writer.Write(data); err != nil {
		if c.verbose {
			log.Printf("[acp-server] write notification %s failed: %v", method, err)
		}
	}
}

// ---------- ServeAcpGateway ----------

// ServeAcpGatewayOptions 服务端选项。
type ServeAcpGatewayOptions struct {
	Verbose bool
}

// ServeAcpGateway 启动 ACP 网关服务端。
// 从 stdin 读取 ndJSON 请求，通过 handler 处理后将响应写入 stdout。
// 对应 TS: acp/server.ts serveAcpGateway()
func ServeAcpGateway(ctx context.Context, handler AcpServerHandler, opts *ServeAcpGatewayOptions) error {
	verbose := opts != nil && opts.Verbose
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	writer := os.Stdout

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var msg NDJSONMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			if verbose {
				log.Printf("[acp-server] invalid ndJSON request: %v", err)
			}
			continue
		}

		// 通知（无 ID）
		if msg.ID == nil {
			handleNotification(ctx, &msg, handler, verbose)
			continue
		}

		// 请求（有 ID）
		go handleRequest(ctx, &msg, handler, writer, verbose)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stdin read error: %w", err)
	}
	return nil
}

// handleRequest 处理带 ID 的 JSON-RPC 请求。
func handleRequest(ctx context.Context, msg *NDJSONMessage, handler AcpServerHandler, writer io.Writer, verbose bool) {
	var result interface{}
	var rpcErr *RPCError

	switch msg.Method {
	case "initialize":
		var req InitializeRequest
		if err := unmarshalParams(msg.Params, &req); err != nil {
			rpcErr = &RPCError{Code: -32602, Message: "invalid params: " + err.Error()}
		} else if resp, err := handler.Initialize(req); err != nil {
			rpcErr = &RPCError{Code: -32603, Message: err.Error()}
		} else {
			result = resp
			handler.Start()
		}

	case "sessions/new":
		var req NewSessionRequest
		if err := unmarshalParams(msg.Params, &req); err != nil {
			rpcErr = &RPCError{Code: -32602, Message: "invalid params: " + err.Error()}
		} else if resp, err := handler.NewSession(req); err != nil {
			rpcErr = &RPCError{Code: -32603, Message: err.Error()}
		} else {
			result = resp
		}

	case "sessions/load":
		var req LoadSessionRequest
		if err := unmarshalParams(msg.Params, &req); err != nil {
			rpcErr = &RPCError{Code: -32602, Message: "invalid params: " + err.Error()}
		} else if resp, err := handler.LoadSession(req); err != nil {
			rpcErr = &RPCError{Code: -32603, Message: err.Error()}
		} else {
			result = resp
		}

	case "sessions/list":
		var req ListSessionsRequest
		if err := unmarshalParams(msg.Params, &req); err != nil {
			rpcErr = &RPCError{Code: -32602, Message: "invalid params: " + err.Error()}
		} else if resp, err := handler.ListSessions(req); err != nil {
			rpcErr = &RPCError{Code: -32603, Message: err.Error()}
		} else {
			result = resp
		}

	case "prompt":
		var req PromptRequest
		if err := unmarshalParams(msg.Params, &req); err != nil {
			rpcErr = &RPCError{Code: -32602, Message: "invalid params: " + err.Error()}
		} else if resp, err := handler.Prompt(ctx, req); err != nil {
			rpcErr = &RPCError{Code: -32603, Message: err.Error()}
		} else {
			result = resp
		}

	case "sessions/setMode":
		var req SetSessionModeRequest
		if err := unmarshalParams(msg.Params, &req); err != nil {
			rpcErr = &RPCError{Code: -32602, Message: "invalid params: " + err.Error()}
		} else if resp, err := handler.SetSessionMode(req); err != nil {
			rpcErr = &RPCError{Code: -32603, Message: err.Error()}
		} else {
			result = resp
		}

	default:
		rpcErr = &RPCError{Code: -32601, Message: "method not found: " + msg.Method}
	}

	resp := NDJSONMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
	}
	if rpcErr != nil {
		resp.Error = rpcErr
	} else {
		resp.Result = result
	}

	writeNDJSON(writer, &resp, verbose)
}

// handleNotification 处理无 ID 的 JSON-RPC 通知。
func handleNotification(_ context.Context, msg *NDJSONMessage, handler AcpServerHandler, verbose bool) {
	switch msg.Method {
	case "notifications/cancel":
		var notif CancelNotification
		if err := unmarshalParams(msg.Params, &notif); err != nil {
			if verbose {
				log.Printf("[acp-server] invalid cancel notification: %v", err)
			}
			return
		}
		handler.Cancel(notif)
	default:
		if verbose {
			log.Printf("[acp-server] unhandled notification: %s", msg.Method)
		}
	}
}

// unmarshalParams 将 interface{} params 解码为目标结构体。
func unmarshalParams(params interface{}, target interface{}) error {
	if params == nil {
		return nil
	}
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// writeNDJSON 向 writer 写入一行 ndJSON。
var writeMu sync.Mutex

func writeNDJSON(w io.Writer, msg *NDJSONMessage, verbose bool) {
	data, err := json.Marshal(msg)
	if err != nil {
		if verbose {
			log.Printf("[acp-server] marshal response failed: %v", err)
		}
		return
	}
	data = append(data, '\n')
	writeMu.Lock()
	defer writeMu.Unlock()
	if _, err := w.Write(data); err != nil {
		if verbose {
			log.Printf("[acp-server] write response failed: %v", err)
		}
	}
}
