package gateway

// server_methods_plan.go — 三级指挥体系 Phase 1: 方案确认 RPC
//
// WebSocket RPC: plan.confirm.resolve — 前端回调方案确认决策

import (
	"github.com/openacosmi/claw-acismi/internal/agents/runner"
)

// PlanConfirmHandlers 返回方案确认相关的 RPC 处理器。
func PlanConfirmHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		// plan.confirm.resolve — 前端回调方案确认决策
		// params: { id: string, action: "approve"|"reject"|"edit", editedPlan?: string, feedback?: string }
		"plan.confirm.resolve": func(ctx *MethodHandlerContext) {
			id, _ := ctx.Params["id"].(string)
			action, _ := ctx.Params["action"].(string)
			if id == "" || action == "" {
				ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "missing id or action"))
				return
			}

			if ctx.Context.PlanConfirmMgr == nil {
				ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "plan confirmation not enabled"))
				return
			}

			decision := runner.PlanDecision{
				Action: action,
			}
			if editedPlan, ok := ctx.Params["editedPlan"].(string); ok {
				decision.EditedPlan = editedPlan
			}
			if feedback, ok := ctx.Params["feedback"].(string); ok {
				decision.Feedback = feedback
			}

			if err := ctx.Context.PlanConfirmMgr.ResolvePlanConfirmation(id, decision); err != nil {
				ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, err.Error()))
				return
			}

			ctx.Respond(true, map[string]interface{}{"ok": true}, nil)
		},

		// plan.confirm.status — 查询当前 pending 方案确认数量（监控用）
		"plan.confirm.status": func(ctx *MethodHandlerContext) {
			if ctx.Context.PlanConfirmMgr == nil {
				ctx.Respond(true, map[string]interface{}{"enabled": false, "pending": 0}, nil)
				return
			}
			ctx.Respond(true, map[string]interface{}{
				"enabled":  true,
				"pending":  ctx.Context.PlanConfirmMgr.PendingCount(),
				"gateMode": ctx.Context.PlanConfirmMgr.GateMode(),
			}, nil)
		},
	}
}
