package gateway

// server_methods_mcp.go — MCP 远程工具 RPC 方法
//
// 3 个新 RPC 方法:
//   mcp.remote.status  — 连接状态 + 工具数量
//   mcp.remote.tools   — 远程工具列表
//   mcp.remote.connect — 手动连接/重连/刷新

import (
	"context"
	"time"

	"github.com/anthropic/open-acosmi/pkg/mcpremote"
)

// MCPRemoteHandlers 返回 MCP 远程工具 RPC 方法处理器。
func MCPRemoteHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		"mcp.remote.status":  handleMCPRemoteStatus,
		"mcp.remote.tools":   handleMCPRemoteTools,
		"mcp.remote.connect": handleMCPRemoteConnect,
	}
}

// ---------- mcp.remote.status ----------

// MCPRemoteStatusResult mcp.remote.status 响应。
type MCPRemoteStatusResult struct {
	Available  bool   `json:"available"`            // Bridge 是否已初始化
	State      string `json:"state"`                // BridgeState 字符串
	ToolCount  int    `json:"toolCount"`            // 已缓存的远程工具数量
	Endpoint   string `json:"endpoint,omitempty"`   // MCP 端点 URL
	LastPingMs int64  `json:"lastPingMs,omitempty"` // 最近 ping RTT (ms)
	LastPingAt string `json:"lastPingAt,omitempty"` // 最近 ping 时间 (RFC3339)
}

func handleMCPRemoteStatus(ctx *MethodHandlerContext) {
	bridge := ctx.Context.RemoteMCPBridge
	if bridge == nil {
		ctx.Respond(true, MCPRemoteStatusResult{
			Available: false,
			State:     "disabled",
		}, nil)
		return
	}

	state := bridge.State()
	tools := bridge.Tools()
	lastPing, lastRTT := bridge.LastPing()

	result := MCPRemoteStatusResult{
		Available: true,
		State:     string(state),
		ToolCount: len(tools),
		Endpoint:  bridge.Endpoint(),
	}

	if !lastPing.IsZero() {
		result.LastPingAt = lastPing.Format(time.RFC3339)
		result.LastPingMs = lastRTT.Milliseconds()
	}

	ctx.Respond(true, result, nil)
}

// ---------- mcp.remote.tools ----------

// MCPRemoteToolItem 远程工具摘要。
type MCPRemoteToolItem struct {
	Name         string `json:"name"`
	Title        string `json:"title,omitempty"`
	Description  string `json:"description"`
	PrefixedName string `json:"prefixedName"` // "remote_" + Name
}

func handleMCPRemoteTools(ctx *MethodHandlerContext) {
	bridge := ctx.Context.RemoteMCPBridge
	if bridge == nil {
		ctx.Respond(true, map[string]interface{}{
			"tools": []interface{}{},
			"state": "disabled",
		}, nil)
		return
	}

	state := bridge.State()
	if state != mcpremote.BridgeStateReady && state != mcpremote.BridgeStateDegraded {
		ctx.Respond(true, map[string]interface{}{
			"tools": []interface{}{},
			"state": string(state),
		}, nil)
		return
	}

	tools := bridge.Tools()
	items := make([]MCPRemoteToolItem, len(tools))
	for i, t := range tools {
		items[i] = MCPRemoteToolItem{
			Name:         t.Name,
			Title:        t.Title,
			Description:  t.Description,
			PrefixedName: "remote_" + t.Name,
		}
	}

	ctx.Respond(true, map[string]interface{}{
		"tools": items,
		"state": string(state),
		"count": len(items),
	}, nil)
}

// ---------- mcp.remote.connect ----------

func handleMCPRemoteConnect(ctx *MethodHandlerContext) {
	bridge := ctx.Context.RemoteMCPBridge
	if bridge == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "MCP remote not configured"))
		return
	}

	// 解析动作: "connect" (默认) | "reconnect" | "refresh"
	action, _ := ctx.Params["action"].(string)
	if action == "" {
		action = "connect"
	}

	switch action {
	case "refresh":
		// 仅刷新工具列表（不断开连接）
		bgCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := bridge.Refresh(bgCtx); err != nil {
			ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "refresh failed: "+err.Error()))
			return
		}

		tools := bridge.Tools()
		ctx.Respond(true, map[string]interface{}{
			"action":    "refresh",
			"state":     string(bridge.State()),
			"toolCount": len(tools),
		}, nil)

	case "reconnect":
		// 停止 → 重新连接
		bridge.Stop()

		bgCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := bridge.Start(bgCtx); err != nil {
			ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "reconnect failed: "+err.Error()))
			return
		}

		tools := bridge.Tools()
		ctx.Respond(true, map[string]interface{}{
			"action":    "reconnect",
			"state":     string(bridge.State()),
			"toolCount": len(tools),
		}, nil)

	default: // "connect"
		state := bridge.State()
		if state == mcpremote.BridgeStateReady || state == mcpremote.BridgeStateDegraded {
			// 已连接
			tools := bridge.Tools()
			ctx.Respond(true, map[string]interface{}{
				"action":    "connect",
				"state":     string(state),
				"toolCount": len(tools),
				"message":   "already connected",
			}, nil)
			return
		}

		bgCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := bridge.Start(bgCtx); err != nil {
			ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "connect failed: "+err.Error()))
			return
		}

		tools := bridge.Tools()
		ctx.Respond(true, map[string]interface{}{
			"action":    "connect",
			"state":     string(bridge.State()),
			"toolCount": len(tools),
		}, nil)
	}
}
