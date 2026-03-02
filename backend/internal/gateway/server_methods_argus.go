package gateway

// server_methods_argus.go — argus.* RPC 方法
//
// 静态方法:
//   argus.status — Bridge 状态 + 工具清单 + PID + 健康信息
//   argus.approval.resolve — 审批决策中继（Phase 2 预留）
//
// 动态方法:
//   RegisterArgusDynamicMethods() 遍历工具列表，为每个工具注册 argus.<tool_name>

import (
	"context"
	"encoding/json"
	"time"

	"github.com/openacosmi/claw-acismi/internal/argus"
)

// ArgusHandlers 返回 argus.* 静态方法映射。
func ArgusHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		"argus.status":           handleArgusStatus,
		"argus.restart":          handleArgusRestart,
		"argus.approval.resolve": handleArgusApprovalResolve,
	}
}

// ---------- argus.status ----------

func handleArgusStatus(ctx *MethodHandlerContext) {
	bridge := ctx.Context.ArgusBridge
	if bridge == nil {
		ctx.Respond(true, map[string]interface{}{
			"available": false,
			"state":     "not_configured",
			"message":   "Argus binary not available or not configured",
		}, nil)
		return
	}

	tools := bridge.Tools()
	toolNames := make([]string, len(tools))
	for i, t := range tools {
		toolNames[i] = t.Name
	}

	lastPing, lastRTT := bridge.LastPing()

	ctx.Respond(true, map[string]interface{}{
		"available": true,
		"state":     string(bridge.State()),
		"pid":       bridge.PID(),
		"tools":     toolNames,
		"toolCount": len(tools),
		"lastPing":  lastPing.UnixMilli(),
		"lastRTTMs": lastRTT.Milliseconds(),
	}, nil)
}

// ---------- argus.restart ----------
// 在熔断或 stopped 状态后手动重启 Argus 子进程。

func handleArgusRestart(ctx *MethodHandlerContext) {
	bridge := ctx.Context.ArgusBridge
	if bridge == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeServiceUnavailable, "Argus binary not available or not configured"))
		return
	}

	state := bridge.State()
	if state == argus.BridgeStateReady {
		ctx.Respond(true, map[string]interface{}{
			"ok":      true,
			"message": "Argus is already running",
			"state":   string(state),
		}, nil)
		return
	}

	// 先停止残留进程
	if state != argus.BridgeStateStopped && state != argus.BridgeStateInit {
		bridge.Stop()
	}

	// 重新启动
	if err := bridge.Start(); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeServiceUnavailable, "argus restart failed: "+err.Error()))
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"ok":      true,
		"message": "Argus restarted successfully",
		"state":   string(bridge.State()),
		"pid":     bridge.PID(),
	}, nil)
}

// ---------- argus.approval.resolve ----------
// Phase 2 预留：审批决策中继。

func handleArgusApprovalResolve(ctx *MethodHandlerContext) {
	// Phase 2: 实现审批决策中继到 Argus ApprovalGateway
	ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "argus.approval.resolve: not yet implemented (Phase 2)"))
}

// ---------- 动态方法注册 ----------

// RegisterArgusDynamicMethods 遍历 Argus 工具列表，为每个工具注册 argus.<tool_name> 方法。
func RegisterArgusDynamicMethods(registry *MethodRegistry, bridge *argus.Bridge) {
	tools := bridge.Tools()
	for _, tool := range tools {
		toolName := tool.Name
		methodName := "argus." + toolName

		registry.Register(methodName, func(ctx *MethodHandlerContext) {
			b := ctx.Context.ArgusBridge
			if b == nil {
				ctx.Respond(false, nil, NewErrorShape(ErrCodeServiceUnavailable, "argus bridge not available"))
				return
			}

			// 提取参数并序列化为 JSON
			var args json.RawMessage
			if params := ctx.Params; len(params) > 0 {
				data, err := json.Marshal(params)
				if err != nil {
					ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "invalid params: "+err.Error()))
					return
				}
				args = data
			}

			// 解析超时
			timeout := 30 * time.Second
			if raw, ok := ctx.Params["_timeout"].(float64); ok && raw > 0 {
				timeout = time.Duration(raw) * time.Millisecond
			}

			// 调用 MCP 工具
			result, err := b.CallTool(context.Background(), toolName, args, timeout)
			if err != nil {
				ctx.Respond(false, nil, NewErrorShape(ErrCodeServiceUnavailable, "argus tool call failed: "+err.Error()))
				return
			}

			// 解析 MCP content → 网关响应
			if result.IsError {
				errMsg := "argus tool error"
				if len(result.Content) > 0 && result.Content[0].Text != "" {
					errMsg = result.Content[0].Text
				}
				ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, errMsg))
				return
			}

			// 构建响应
			response := map[string]interface{}{
				"tool":    toolName,
				"content": result.Content,
			}
			ctx.Respond(true, response, nil)
		})
	}
}
