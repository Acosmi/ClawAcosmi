package gateway

// server_methods_wizard.go — Setup Wizard RPC 处理器
// TS 对照: src/gateway/server-methods/wizard.ts (140L)
//
// 真实实现，替代 server_methods_stubs.go 中的 wizard.* stubs。
// 方法: wizard.start, wizard.next, wizard.cancel, wizard.status

import (
	"github.com/google/uuid"
	"github.com/openacosmi/claw-acismi/internal/agents/models"
	"github.com/openacosmi/claw-acismi/internal/config"
)

// WizardHandlerDeps wizard handler 依赖。
type WizardHandlerDeps struct {
	Tracker      *WizardSessionTracker
	ConfigLoader *config.ConfigLoader
	ModelCatalog *models.ModelCatalog
	State        *GatewayState
}

// WizardHandlers 返回 wizard.* 方法处理器映射。
func WizardHandlers(deps WizardHandlerDeps) map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		"wizard.start":  wizardStartHandler(deps),
		"wizard.next":   wizardNextHandler(deps),
		"wizard.cancel": wizardCancelHandler(deps),
		"wizard.status": wizardStatusHandler(deps),
	}
}

// wizardStartHandler wizard.start — 创建新会话并启动引导流程。
// 支持 params.mode = "advanced" 启用高级版向导。
func wizardStartHandler(deps WizardHandlerDeps) GatewayMethodHandler {
	return func(ctx *MethodHandlerContext) {
		// 检查是否已有运行中的会话
		if runningID := deps.Tracker.FindRunning(); runningID != "" {
			session := deps.Tracker.Get(runningID)
			if session != nil {
				result := session.Next()
				ctx.Respond(true, WizardStartResult{
					SessionID: runningID,
					Done:      result.Done,
					Step:      result.Step,
					Status:    result.Status,
					Error:     result.Error,
				}, nil)
				return
			}
		}

		// 创建新会话
		sessionID := uuid.New().String()
		onboardingDeps := WizardOnboardingDeps{
			ConfigLoader: deps.ConfigLoader,
			ModelCatalog: deps.ModelCatalog,
			State:        deps.State,
		}

		// 根据参数选择向导模式
		requestMode, _ := ctx.Params["mode"].(string)
		var runner WizardRunnerFunc
		switch requestMode {
		case "advanced":
			runner = RunOnboardingWizardAdvanced(onboardingDeps)
		case "open-coder":
			runner = RunOpenCoderWizard(onboardingDeps)
		default:
			runner = RunOnboardingWizard(onboardingDeps)
		}

		session := NewWizardSession(runner)
		deps.Tracker.Set(sessionID, session)

		// 获取第一步
		result := session.Next()
		ctx.Respond(true, WizardStartResult{
			SessionID: sessionID,
			Done:      result.Done,
			Step:      result.Step,
			Status:    result.Status,
			Error:     result.Error,
		}, nil)
	}
}

// wizardNextHandler wizard.next — 回答当前步骤并获取下一步。
func wizardNextHandler(deps WizardHandlerDeps) GatewayMethodHandler {
	return func(ctx *MethodHandlerContext) {
		sessionID, _ := ctx.Params["sessionId"].(string)
		if sessionID == "" {
			ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "missing sessionId"))
			return
		}

		session := deps.Tracker.Get(sessionID)
		if session == nil {
			ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "wizard session not found: "+sessionID))
			return
		}

		// 处理回答
		answer, hasAnswer := ctx.Params["answer"]
		if hasAnswer && answer != nil {
			answerMap, ok := answer.(map[string]interface{})
			if ok {
				stepID, _ := answerMap["stepId"].(string)
				value := answerMap["value"]
				if stepID != "" {
					if err := session.Answer(stepID, value); err != nil {
						ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "answer failed: "+err.Error()))
						return
					}
				}
			}
		}

		// 获取下一步
		result := session.Next()

		// 如果 done，清理会话
		if result.Done {
			deps.Tracker.Purge(sessionID)
		}

		ctx.Respond(true, WizardNextResult{
			Done:   result.Done,
			Step:   result.Step,
			Status: result.Status,
			Error:  result.Error,
		}, nil)
	}
}

// wizardCancelHandler wizard.cancel — 取消运行中的会话。
func wizardCancelHandler(deps WizardHandlerDeps) GatewayMethodHandler {
	return func(ctx *MethodHandlerContext) {
		sessionID, _ := ctx.Params["sessionId"].(string)
		if sessionID == "" {
			ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "missing sessionId"))
			return
		}

		session := deps.Tracker.Get(sessionID)
		if session == nil {
			ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "wizard session not found: "+sessionID))
			return
		}

		session.Cancel()
		deps.Tracker.Purge(sessionID)

		ctx.Respond(true, WizardStatusResult{
			Status: WizardStatusCancelled,
		}, nil)
	}
}

// wizardStatusHandler wizard.status — 查询会话状态。
func wizardStatusHandler(deps WizardHandlerDeps) GatewayMethodHandler {
	return func(ctx *MethodHandlerContext) {
		sessionID, _ := ctx.Params["sessionId"].(string)
		if sessionID == "" {
			ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "missing sessionId"))
			return
		}

		session := deps.Tracker.Get(sessionID)
		if session == nil {
			ctx.Respond(true, WizardStatusResult{
				Status: WizardStatusDone,
				Error:  "session not found",
			}, nil)
			return
		}

		ctx.Respond(true, WizardStatusResult{
			Status: session.GetStatus(),
			Error:  session.GetError(),
		}, nil)
	}
}
