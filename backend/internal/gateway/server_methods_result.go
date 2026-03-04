package gateway

// server_methods_result.go — 三级指挥体系 Phase 3: 结果签收 RPC
//
// WebSocket RPC: result.approve.resolve — 前端回调结果签收决策

import (
	"github.com/Acosmi/ClawAcosmi/internal/agents/runner"
)

// ResultApprovalHandlers 返回结果签收相关的 RPC 处理器。
func ResultApprovalHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		// result.approve.resolve — 前端回调结果签收决策
		// params: { id: string, action: "approve"|"reject", feedback?: string }
		"result.approve.resolve": func(ctx *MethodHandlerContext) {
			id, _ := ctx.Params["id"].(string)
			action, _ := ctx.Params["action"].(string)
			if id == "" || action == "" {
				ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "missing id or action"))
				return
			}

			if ctx.Context.ResultApprovalMgr == nil {
				ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "result approval not enabled"))
				return
			}

			decision := runner.ResultApprovalDecision{
				Action: action,
			}
			if feedback, ok := ctx.Params["feedback"].(string); ok {
				decision.Feedback = feedback
			}

			if err := ctx.Context.ResultApprovalMgr.ResolveResultApproval(id, decision); err != nil {
				ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, err.Error()))
				return
			}

			ctx.Respond(true, map[string]interface{}{"ok": true}, nil)
		},

		// result.approve.status — 查询当前 pending 结果签收数量（监控用）
		"result.approve.status": func(ctx *MethodHandlerContext) {
			if ctx.Context.ResultApprovalMgr == nil {
				ctx.Respond(true, map[string]interface{}{"enabled": false, "pending": 0}, nil)
				return
			}
			ctx.Respond(true, map[string]interface{}{
				"enabled": true,
				"pending": ctx.Context.ResultApprovalMgr.PendingCount(),
			}, nil)
		},
	}
}
