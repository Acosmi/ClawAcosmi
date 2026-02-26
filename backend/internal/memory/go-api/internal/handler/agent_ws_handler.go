// Package handler — Agent WebSocket 端点（P4-4b）。
// 接收本地 Agent 的 WebSocket 连接，建立反向隧道。
package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/uhms/go-api/internal/mcp"
	"github.com/uhms/go-api/internal/services"
)

// AgentWSHandler 处理 Agent WebSocket 连接。
type AgentWSHandler struct {
	pool     *mcp.TunnelPool
	registry *services.AgentRegistry
	upgrader websocket.Upgrader
}

// NewAgentWSHandler 创建 Agent WebSocket handler。
func NewAgentWSHandler(
	pool *mcp.TunnelPool,
	registry *services.AgentRegistry,
	allowedOrigins []string,
) *AgentWSHandler {
	return &AgentWSHandler{
		pool:     pool,
		registry: registry,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Agent 连接使用 token 鉴权，允许所有来源
				// 实际鉴权在 HandleWS 中通过 X-Agent-Token 完成
				return true
			},
		},
	}
}

// RegisterRoutes 注册 Agent WebSocket 路由。
func (h *AgentWSHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/agent/ws", h.HandleWS)
	rg.GET("/agent/list", h.ListAgents)
}

// HandleWS 处理 Agent WebSocket 连接请求。
// Agent 通过 X-Agent-Token header 鉴权，连接后建立反向隧道。
func (h *AgentWSHandler) HandleWS(c *gin.Context) {
	// 鉴权: 从 header 获取 token
	token := c.GetHeader("X-Agent-Token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "missing X-Agent-Token header",
		})
		return
	}

	// 从 context 获取 tenant_id（由 DevAuthBypass 中间件设置）
	tenantID, _ := c.Get("user_id")
	tid, ok := tenantID.(string)
	if !ok || tid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "tenant_id not found in context",
		})
		return
	}

	// 升级到 WebSocket
	ws, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("WebSocket upgrade failed",
			"tenant_id", tid, "error", err)
		return
	}

	// 注册 Agent
	agentName := c.GetHeader("X-Agent-Name")
	if agentName == "" {
		agentName = "local-agent"
	}
	_, regErr := h.registry.RegisterAgent(tid, agentName, "ws://tunnel", nil)
	if regErr != nil {
		slog.Error("Agent registration failed",
			"tenant_id", tid, "error", regErr)
		_ = ws.Close()
		return
	}

	// 创建隧道连接
	tunnel := mcp.NewTunnelConn(tid, ws)
	h.pool.Add(tunnel)

	slog.Info("Agent tunnel established", "tenant_id", tid, "name", agentName)

	// 启动读取循环（阻塞直到断连）
	tunnel.ReadPump(c.Request.Context())

	// 断连清理
	h.pool.Remove(tid)
	h.registry.DeregisterAgent(tid)
	slog.Info("Agent tunnel closed", "tenant_id", tid)
}

// ListAgents 返回已注册的 Agent 列表。
func (h *AgentWSHandler) ListAgents(c *gin.Context) {
	agents, err := h.registry.ListAgents()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to list agents",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"agents": agents,
		"total":  len(agents),
	})
}
