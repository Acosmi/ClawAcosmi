package gateway

// agent.* 方法处理器 — 对应 src/gateway/server-methods/agent.ts
//
// 提供 Agent 身份查询和等待功能。
// 依赖: scope.ResolveAgentIdentity

import (
	"context"
	"math"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/agents/scope"
)

// AgentCommandSnapshot — agent.wait 返回的快照。
// 对应 TS waitForAgentJob 返回的 snapshot 对象。
type AgentCommandSnapshot struct {
	Status    string `json:"status"`
	StartedAt int64  `json:"startedAt,omitempty"` // UnixMilli
	EndedAt   int64  `json:"endedAt,omitempty"`   // UnixMilli
	Error     string `json:"error,omitempty"`
}

// AgentCommandWaiter DI 接口 — 等待 agent 命令完成。
type AgentCommandWaiter interface {
	WaitForCompletion(ctx context.Context, runID string) (*AgentCommandSnapshot, error)
}

// AgentHandlers 返回 agent.* 方法处理器映射。
func AgentHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		"agent.identity.get": handleAgentIdentityGet,
		"agent.wait":         handleAgentWait,
	}
}

// ---------- agent.identity.get ----------
// 对应 TS agent.ts:L384-L430

func handleAgentIdentityGet(ctx *MethodHandlerContext) {
	cfg := resolveConfigFromContext(ctx)
	if cfg == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "config not available"))
		return
	}

	agentId, _ := ctx.Params["agentId"].(string)
	if agentId == "" {
		agentId = scope.ResolveDefaultAgentId(cfg)
	}
	agentId = scope.NormalizeAgentId(agentId)

	identity := scope.ResolveAgentIdentity(cfg, agentId)

	result := map[string]interface{}{
		"agentId": agentId,
	}

	if identity != nil {
		identityMap := map[string]interface{}{}
		if identity.Name != "" {
			identityMap["name"] = identity.Name
		}
		if identity.Theme != "" {
			identityMap["theme"] = identity.Theme
		}
		if identity.Emoji != "" {
			identityMap["emoji"] = identity.Emoji
		}
		if identity.Avatar != "" {
			identityMap["avatar"] = identity.Avatar
		}
		if len(identityMap) > 0 {
			result["identity"] = identityMap
		}
	}

	ctx.Respond(true, result, nil)
}

// ---------- agent.wait ----------
// 对应 TS agent.ts:L477-L514
// 等待 Agent 命令运行完成。

const defaultAgentWaitTimeoutMs = 30_000 // 30 秒，与 TS 一致

func handleAgentWait(ctx *MethodHandlerContext) {
	runID, _ := ctx.Params["runId"].(string)

	// 解析超时（毫秒）— 对应 TS timeoutMs 参数
	timeoutMs := float64(defaultAgentWaitTimeoutMs)
	if v, ok := ctx.Params["timeoutMs"].(float64); ok && !math.IsNaN(v) && !math.IsInf(v, 0) && v > 0 {
		timeoutMs = math.Max(0, math.Floor(v))
	}
	timeout := time.Duration(timeoutMs) * time.Millisecond

	// 若 DI waiter 已注入，执行真正的等待
	if ctx.Context.AgentWaiter != nil && runID != "" {
		waitCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		snapshot, err := ctx.Context.AgentWaiter.WaitForCompletion(waitCtx, runID)
		if err != nil {
			if waitCtx.Err() == context.DeadlineExceeded {
				ctx.Respond(true, map[string]interface{}{
					"runId":  runID,
					"status": "timeout",
				}, nil)
				return
			}
			ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "wait failed: "+err.Error()))
			return
		}

		result := map[string]interface{}{
			"runId":  runID,
			"status": snapshot.Status,
		}
		if snapshot.StartedAt > 0 {
			result["startedAt"] = snapshot.StartedAt
		}
		if snapshot.EndedAt > 0 {
			result["endedAt"] = snapshot.EndedAt
		}
		if snapshot.Error != "" {
			result["error"] = snapshot.Error
		}
		ctx.Respond(true, result, nil)
		return
	}

	// Fallback: DI 未注入 — 立即返回 completed
	ctx.Respond(true, map[string]interface{}{
		"status": "completed",
	}, nil)
}
