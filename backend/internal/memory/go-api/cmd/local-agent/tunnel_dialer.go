// Package main — WebSocket 反向隧道拨号器（DEF-AGENT-01）。
// Local Agent 主动向云端发起 WebSocket 连接，建立反向隧道。
// 云端通过隧道下发 MCP 请求，Agent 转发到本地 MCP Server 并回传响应。
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/uhms/go-api/internal/mcp"
)

// TunnelDialer 负责从 Local Agent 端主动拨号连接到云端 WebSocket，
// 建立反向隧道并处理消息转发。
type TunnelDialer struct {
	cloudURL     string // 云端 WebSocket 地址 (ws://或wss://)
	token        string // 鉴权 Token
	agentName    string // Agent 名称标识
	localMCPAddr string // 本地 MCP HTTP 地址

	conn   *websocket.Conn
	mu     sync.Mutex // 保护 conn 的写操作
	client *http.Client
}

// NewTunnelDialer 创建隧道拨号器。
func NewTunnelDialer(cloudURL, token, agentName, localMCPAddr string) *TunnelDialer {
	return &TunnelDialer{
		cloudURL:     cloudURL,
		token:        token,
		agentName:    agentName,
		localMCPAddr: localMCPAddr,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// reconnect 参数常量。
const (
	initialBackoff = 1 * time.Second
	maxBackoff     = 60 * time.Second
	backoffFactor  = 2.0
	dialTimeout    = 10 * time.Second
)

// Run 启动隧道拨号器主循环。
// 包含自动重连和指数退避逻辑，直到 ctx 被取消。
func (d *TunnelDialer) Run(ctx context.Context) {
	backoff := initialBackoff

	for {
		select {
		case <-ctx.Done():
			slog.Info("Tunnel dialer stopped")
			d.closeConn()
			return
		default:
		}

		slog.Info("Connecting to cloud tunnel...",
			"url", d.cloudURL, "agent", d.agentName)

		err := d.dial(ctx)
		if err != nil {
			slog.Error("Tunnel dial failed",
				"url", d.cloudURL, "error", err)
			d.waitBackoff(ctx, backoff)
			backoff = nextBackoff(backoff)
			continue
		}

		slog.Info("Tunnel established", "url", d.cloudURL)
		backoff = initialBackoff // 成功连接后重置退避

		// 阻塞读取，直到连接断开
		d.readPump(ctx)

		slog.Warn("Tunnel connection lost, will reconnect...",
			"url", d.cloudURL)
		d.closeConn()
	}
}

// dial 执行单次 WebSocket 拨号。
func (d *TunnelDialer) dial(ctx context.Context) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: dialTimeout,
	}

	header := http.Header{}
	header.Set("X-Agent-Token", d.token)
	header.Set("X-Agent-Name", d.agentName)

	dialCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	conn, resp, err := dialer.DialContext(dialCtx, d.cloudURL, header)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("dial %s: status=%d, err=%w",
				d.cloudURL, resp.StatusCode, err)
		}
		return fmt.Errorf("dial %s: %w", d.cloudURL, err)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	d.mu.Lock()
	d.conn = conn
	d.mu.Unlock()

	return nil
}

// readPump 从 WebSocket 读取消息并分发处理。
func (d *TunnelDialer) readPump(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		var msg mcp.TunnelMessage
		if err := d.conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(
				err,
				websocket.CloseGoingAway,
				websocket.CloseNormalClosure,
			) {
				slog.Warn("Tunnel read error", "error", err)
			}
			return
		}

		switch msg.Type {
		case "heartbeat":
			// 回写心跳应答
			resp := mcp.TunnelMessage{Type: "heartbeat"}
			if sendErr := d.sendMessage(resp); sendErr != nil {
				slog.Warn("Heartbeat response failed", "error", sendErr)
				return
			}
			slog.Debug("Heartbeat responded")

		case "request":
			// 异步转发到本地 MCP Server
			go d.handleRequest(ctx, msg)

		default:
			slog.Warn("Unknown tunnel message type",
				"type", msg.Type, "id", msg.ID)
		}
	}
}

// handleRequest 将云端下发的 MCP 请求转发到本地 MCP HTTP 端点。
func (d *TunnelDialer) handleRequest(ctx context.Context, req mcp.TunnelMessage) {
	payload, err := d.forwardToLocalMCP(ctx, req.Payload)

	resp := mcp.TunnelMessage{
		ID:   req.ID,
		Type: "response",
	}

	if err != nil {
		slog.Error("Forward to local MCP failed",
			"request_id", req.ID, "error", err)
		// 构造错误响应
		errPayload, _ := json.Marshal(map[string]string{
			"error": err.Error(),
		})
		resp.Payload = errPayload
	} else {
		resp.Payload = payload
	}

	if sendErr := d.sendMessage(resp); sendErr != nil {
		slog.Error("Send response failed",
			"request_id", req.ID, "error", sendErr)
	}
}

// forwardToLocalMCP 将请求体转发到本地 MCP HTTP Server。
func (d *TunnelDialer) forwardToLocalMCP(
	ctx context.Context, payload json.RawMessage,
) (json.RawMessage, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(
		reqCtx, http.MethodPost, d.localMCPAddr, bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("local MCP request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("local MCP returned %d: %s",
			resp.StatusCode, string(body))
	}

	return json.RawMessage(body), nil
}

// sendMessage 线程安全地向 WebSocket 发送消息。
func (d *TunnelDialer) sendMessage(msg mcp.TunnelMessage) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.conn == nil {
		return fmt.Errorf("tunnel connection is nil")
	}
	return d.conn.WriteJSON(msg)
}

// closeConn 安全关闭当前 WebSocket 连接。
func (d *TunnelDialer) closeConn() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.conn != nil {
		_ = d.conn.Close()
		d.conn = nil
	}
}

// waitBackoff 等待指定退避时间，可被 ctx 取消中断。
func (d *TunnelDialer) waitBackoff(ctx context.Context, duration time.Duration) {
	slog.Info("Reconnecting after backoff",
		"delay", duration.String())
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

// nextBackoff 计算下一次退避时间（指数增长，上限 maxBackoff）。
func nextBackoff(current time.Duration) time.Duration {
	next := time.Duration(float64(current) * backoffFactor)
	maxDur := time.Duration(math.Min(float64(next), float64(maxBackoff)))
	return maxDur
}
