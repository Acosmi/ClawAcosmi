// Package mcp — WebSocket 安全隧道（P4-4）。
// Agent 端主动通过 WebSocket 连接到云端，建立反向隧道。
// 云端通过该隧道转发 MCP 请求到 Agent 的本地 MCP Server。
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// TunnelMessage 隧道消息格式。
type TunnelMessage struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"` // "request" | "response" | "heartbeat"
	Payload json.RawMessage `json:"payload,omitempty"`
}

// TunnelConn 表示一条 Agent WebSocket 隧道连接。
type TunnelConn struct {
	TenantID string
	conn     *websocket.Conn
	mu       sync.Mutex
	pending  map[string]chan TunnelMessage // requestID -> response chan
}

// NewTunnelConn 创建隧道连接封装。
func NewTunnelConn(tenantID string, conn *websocket.Conn) *TunnelConn {
	return &TunnelConn{
		TenantID: tenantID,
		conn:     conn,
		pending:  make(map[string]chan TunnelMessage),
	}
}

// Send 向 Agent 发送消息。
func (t *TunnelConn) Send(msg TunnelMessage) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.conn.WriteJSON(msg)
}

// SendRequest 发送请求并等待响应（同步阻塞）。
func (t *TunnelConn) SendRequest(
	ctx context.Context, id string, payload json.RawMessage,
) (json.RawMessage, error) {
	ch := make(chan TunnelMessage, 1)
	t.mu.Lock()
	t.pending[id] = ch
	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		delete(t.pending, id)
		t.mu.Unlock()
	}()

	msg := TunnelMessage{ID: id, Type: "request", Payload: payload}
	if err := t.Send(msg); err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	select {
	case resp := <-ch:
		return resp.Payload, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ReadPump 从 WebSocket 读取消息并分发。
func (t *TunnelConn) ReadPump(ctx context.Context) {
	defer t.Close()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		var msg TunnelMessage
		if err := t.conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(
				err, websocket.CloseGoingAway, websocket.CloseNormalClosure,
			) {
				slog.Warn("Tunnel read error",
					"tenant_id", t.TenantID, "error", err)
			}
			return
		}

		switch msg.Type {
		case "response":
			t.mu.Lock()
			if ch, ok := t.pending[msg.ID]; ok {
				ch <- msg
			}
			t.mu.Unlock()
		case "heartbeat":
			// 心跳响应，更新 last_seen（由调用方处理）
			slog.Debug("Tunnel heartbeat", "tenant_id", t.TenantID)
		default:
			slog.Warn("Unknown tunnel message type",
				"type", msg.Type, "tenant_id", t.TenantID)
		}
	}
}

// Close 关闭隧道连接。
func (t *TunnelConn) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	// 关闭所有 pending 请求
	for id, ch := range t.pending {
		close(ch)
		delete(t.pending, id)
	}
	return t.conn.Close()
}

// TunnelPool 管理多个 Agent 的隧道连接池。
type TunnelPool struct {
	mu    sync.RWMutex
	conns map[string]*TunnelConn // tenantID -> tunnel
}

// NewTunnelPool 创建隧道连接池。
func NewTunnelPool() *TunnelPool {
	return &TunnelPool{
		conns: make(map[string]*TunnelConn),
	}
}

// Add 添加隧道连接。
func (p *TunnelPool) Add(tc *TunnelConn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// 关闭旧连接（如果存在）
	if old, ok := p.conns[tc.TenantID]; ok {
		_ = old.Close()
	}
	p.conns[tc.TenantID] = tc
}

// Get 获取指定租户的隧道连接。
func (p *TunnelPool) Get(tenantID string) (*TunnelConn, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	tc, ok := p.conns[tenantID]
	return tc, ok
}

// Remove 移除并关闭隧道连接。
func (p *TunnelPool) Remove(tenantID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if tc, ok := p.conns[tenantID]; ok {
		_ = tc.Close()
		delete(p.conns, tenantID)
	}
}

// HeartbeatLoop 定期发送心跳探测。
func (p *TunnelPool) HeartbeatLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.mu.RLock()
			for tid, tc := range p.conns {
				msg := TunnelMessage{Type: "heartbeat"}
				if err := tc.Send(msg); err != nil {
					slog.Warn("Heartbeat failed, removing tunnel",
						"tenant_id", tid, "error", err)
					go p.Remove(tid)
				}
			}
			p.mu.RUnlock()
		}
	}
}
