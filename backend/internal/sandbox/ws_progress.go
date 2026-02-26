// =============================================================================
// 文件: backend/internal/sandbox/ws_progress.go | 模块: sandbox | 职责: WebSocket 实时任务执行进度推送 (Hub 模式)
// 审计: V12 2026-02-21 | 适配: 2026-02-23 — 移除 gin-gonic/gin 依赖，使用原生 net/http
// =============================================================================

// Package sandbox — WebSocket 实时进度推送
//
// [C-2.8] 通过 gorilla/websocket 实现沙箱任务执行进度的实时推送。
//
// 设计: Hub 模式 (类似 chat room)
// - 每个 WebSocket 连接订阅指定 taskID
// - Worker 执行时通过 Hub.Broadcast 推送事件
// - 客户端断开自动清理
package sandbox

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"sync"

	"github.com/gorilla/websocket"
)

// ProgressEvent 进度事件 (广播给 WebSocket 客户端)
type ProgressEvent struct {
	TaskID   string `json:"taskId"`
	Progress int    `json:"progress"`
	Message  string `json:"message"`
	Type     string `json:"type"`             // "progress" | "stdout" | "stderr" | "done" | "error"
	Output   string `json:"output,omitempty"` // stdout/stderr 增量输出文本
}

// ProgressHub 管理 WebSocket 连接和广播
type ProgressHub struct {
	mu             sync.RWMutex
	clients        map[string]map[*websocket.Conn]bool // taskID -> connections
	allowedOrigins []string                            // [SANDBOX-02] 允许的 Origin 列表 (空=放行所有, 开发模式)
}

// NewProgressHub 创建进度推送 Hub
// [SANDBOX-02] allowedOrigins: 允许的 WebSocket Origin 列表, 空则放行所有 (开发环境)
func NewProgressHub(allowedOrigins []string) *ProgressHub {
	return &ProgressHub{
		clients:        make(map[string]map[*websocket.Conn]bool),
		allowedOrigins: allowedOrigins,
	}
}

// HandleWebSocket 处理 WebSocket 连接
//
// 前端连接: ws://host/api/v4/sandbox/tasks/:id/ws
// taskID 由调用方从 URL 路径中提取并传入
func (h *ProgressHub) HandleWebSocket(w http.ResponseWriter, r *http.Request, taskID string) {
	if taskID == "" {
		http.Error(w, `{"error":"task ID required"}`, http.StatusBadRequest)
		return
	}

	// [SANDBOX-02] CheckOrigin 校验: 生产环境限制 Origin, 开发环境放行
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			if len(h.allowedOrigins) == 0 {
				return true // 开发环境无限制
			}
			origin := r.Header.Get("Origin")
			if origin == "" {
				return false
			}
			parsed, err := url.Parse(origin)
			if err != nil {
				return false
			}
			for _, allowed := range h.allowedOrigins {
				allowedParsed, err := url.Parse(allowed)
				if err != nil {
					continue
				}
				if parsed.Host == allowedParsed.Host {
					return true
				}
			}
			slog.Warn("websocket origin rejected",
				"origin", origin,
				"allowed", h.allowedOrigins,
			)
			return false
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "error", err)
		return
	}

	// 注册连接
	h.subscribe(taskID, conn)

	slog.Debug("websocket client connected",
		"task_id", taskID,
		"remote", conn.RemoteAddr().String(),
	)

	// 读取循环 (仅用于检测断开)
	go func() {
		defer func() {
			h.unsubscribe(taskID, conn)
			conn.Close()
			slog.Debug("websocket client disconnected",
				"task_id", taskID,
			)
		}()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()
}

// Broadcast 广播进度事件给所有订阅该 taskID 的客户端
func (h *ProgressHub) Broadcast(event ProgressEvent) {
	h.mu.RLock()
	clients, ok := h.clients[event.TaskID]
	h.mu.RUnlock()

	if !ok || len(clients) == 0 {
		return
	}

	data, _ := json.Marshal(event)

	h.mu.RLock()
	defer h.mu.RUnlock()

	for conn := range clients {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			slog.Debug("websocket write failed, will clean up",
				"task_id", event.TaskID,
				"error", err,
			)
			// 标记待清理 (不在 RLock 中删除, 靠读取循环退出时清理)
		}
	}
}

// subscribe 注册连接到 taskID
func (h *ProgressHub) subscribe(taskID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[taskID] == nil {
		h.clients[taskID] = make(map[*websocket.Conn]bool)
	}
	h.clients[taskID][conn] = true
}

// unsubscribe 取消注册
func (h *ProgressHub) unsubscribe(taskID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.clients[taskID]; ok {
		delete(clients, conn)
		if len(clients) == 0 {
			delete(h.clients, taskID)
		}
	}
}

// ClientCount 返回当前连接数 (主要用于测试/监控)
func (h *ProgressHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	count := 0
	for _, clients := range h.clients {
		count += len(clients)
	}
	return count
}
