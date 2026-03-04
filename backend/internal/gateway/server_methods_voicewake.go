package gateway

// server_methods_voicewake.go — voicewake.* 方法处理器
// 对应 TS: src/gateway/server-methods/voicewake.ts (35L)
//
// 方法列表 (2): voicewake.get, voicewake.set

import (
	"github.com/Acosmi/ClawAcosmi/internal/infra"
)

// VoiceWakeHandlers 返回 voicewake.* 方法映射。
func VoiceWakeHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		"voicewake.get": handleVoiceWakeGet,
		"voicewake.set": handleVoiceWakeSet,
	}
}

// ---------- voicewake.get ----------

func handleVoiceWakeGet(ctx *MethodHandlerContext) {
	baseDir := ctx.Context.PairingBaseDir // 复用 settings 基目录
	cfg, err := infra.LoadVoiceWakeConfig(baseDir)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "failed to load voicewake config: "+err.Error()))
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"triggers": cfg.Triggers,
	}, nil)
}

// ---------- voicewake.set ----------

func handleVoiceWakeSet(ctx *MethodHandlerContext) {
	triggersRaw, ok := ctx.Params["triggers"].([]interface{})
	if !ok {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "voicewake.set requires triggers: string[]"))
		return
	}

	triggers := make([]string, 0, len(triggersRaw))
	for _, t := range triggersRaw {
		if s, ok := t.(string); ok {
			triggers = append(triggers, s)
		}
	}

	baseDir := ctx.Context.PairingBaseDir
	cfg, err := infra.SetVoiceWakeTriggers(triggers, baseDir)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "failed to set voicewake triggers: "+err.Error()))
		return
	}

	// 广播变更
	if ctx.Context.BroadcastFn != nil {
		ctx.Context.BroadcastFn("voicewake.changed", map[string]interface{}{
			"triggers": cfg.Triggers,
		}, nil)
	}

	ctx.Respond(true, map[string]interface{}{
		"triggers": cfg.Triggers,
	}, nil)
}
