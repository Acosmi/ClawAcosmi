package gateway

// server_methods_tts.go — tts.* 方法处理器
// 对应 TS: src/gateway/server-methods/tts.ts (158L)
//
// 方法列表 (6):
//   tts.status, tts.providers, tts.enable, tts.disable,
//   tts.convert, tts.setProvider

import (
	"strings"

	"github.com/openacosmi/claw-acismi/internal/tts"
)

// TtsHandlers 返回 tts.* 方法映射。
func TtsHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		"tts.status":      handleTtsStatus,
		"tts.providers":   handleTtsProviders,
		"tts.enable":      handleTtsEnable,
		"tts.disable":     handleTtsDisable,
		"tts.convert":     handleTtsConvert,
		"tts.setProvider": handleTtsSetProvider,
	}
}

// ---------- tts.status ----------

func handleTtsStatus(ctx *MethodHandlerContext) {
	provider := ctx.Context.TtsConfig
	if provider == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "tts config not available"))
		return
	}

	config := provider.ResolveConfig()
	prefsPath := provider.PrefsPath()

	enabled := tts.IsTtsEnabled(config, prefsPath, "")
	currentProvider := tts.GetTtsProvider(config, prefsPath)
	autoMode := tts.ResolveTtsAutoMode(config, prefsPath, "")
	lastAttempt := tts.GetLastTtsAttempt()

	// API key 状态
	openaiConfigured := tts.IsTtsProviderConfigured(config, tts.ProviderOpenAI)
	elevenLabsConfigured := tts.IsTtsProviderConfigured(config, tts.ProviderElevenLabs)

	result := map[string]interface{}{
		"enabled":  enabled,
		"provider": string(currentProvider),
		"auto":     string(autoMode),
		"keys": map[string]interface{}{
			"openai":     openaiConfigured,
			"elevenlabs": elevenLabsConfigured,
		},
	}

	if lastAttempt != nil {
		result["lastAttempt"] = lastAttempt
	}

	ctx.Respond(true, result, nil)
}

// ---------- tts.providers ----------

func handleTtsProviders(ctx *MethodHandlerContext) {
	provider := ctx.Context.TtsConfig
	if provider == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "tts config not available"))
		return
	}

	config := provider.ResolveConfig()

	providers := make([]map[string]interface{}, 0, len(tts.TtsProviders))
	for _, p := range tts.TtsProviders {
		providers = append(providers, map[string]interface{}{
			"id":         string(p),
			"configured": tts.IsTtsProviderConfigured(config, p),
		})
	}

	ctx.Respond(true, map[string]interface{}{
		"providers": providers,
	}, nil)
}

// ---------- tts.enable ----------

func handleTtsEnable(ctx *MethodHandlerContext) {
	provider := ctx.Context.TtsConfig
	if provider == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "tts config not available"))
		return
	}

	prefsPath := provider.PrefsPath()
	if err := tts.SetTtsEnabled(prefsPath, true); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "failed to enable tts: "+err.Error()))
		return
	}

	ctx.Respond(true, map[string]interface{}{"enabled": true}, nil)
}

// ---------- tts.disable ----------

func handleTtsDisable(ctx *MethodHandlerContext) {
	provider := ctx.Context.TtsConfig
	if provider == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "tts config not available"))
		return
	}

	prefsPath := provider.PrefsPath()
	if err := tts.SetTtsEnabled(prefsPath, false); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "failed to disable tts: "+err.Error()))
		return
	}

	ctx.Respond(true, map[string]interface{}{"enabled": false}, nil)
}

// ---------- tts.convert ----------

func handleTtsConvert(ctx *MethodHandlerContext) {
	provider := ctx.Context.TtsConfig
	if provider == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "tts config not available"))
		return
	}

	text, _ := ctx.Params["text"].(string)
	text = strings.TrimSpace(text)
	if text == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "tts.convert requires non-empty text"))
		return
	}

	config := provider.ResolveConfig()
	prefsPath := provider.PrefsPath()
	channel, _ := ctx.Params["channel"].(string)

	result := tts.SynthesizeTts(tts.SynthesizeTtsParams{
		Text:      text,
		Config:    config,
		PrefsPath: prefsPath,
		ChannelID: channel,
	})

	if result == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "tts synthesis returned nil"))
		return
	}

	if !result.Success {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "tts synthesis failed: "+result.Error))
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"audioPath":       result.AudioPath,
		"provider":        result.Provider,
		"voiceCompatible": result.VoiceCompatible,
		"latencyMs":       result.LatencyMs,
	}, nil)
}

// ---------- tts.setProvider ----------

func handleTtsSetProvider(ctx *MethodHandlerContext) {
	provider := ctx.Context.TtsConfig
	if provider == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "tts config not available"))
		return
	}

	providerName, _ := ctx.Params["provider"].(string)
	providerName = strings.TrimSpace(providerName)
	if providerName == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "tts.setProvider requires provider"))
		return
	}

	// 验证 provider 名称
	valid := false
	for _, p := range tts.TtsProviders {
		if string(p) == providerName {
			valid = true
			break
		}
	}
	if !valid {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "unknown tts provider: "+providerName))
		return
	}

	prefsPath := provider.PrefsPath()
	if err := tts.SetTtsProvider(prefsPath, tts.TtsProvider(providerName)); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "failed to set tts provider: "+err.Error()))
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"provider": providerName,
	}, nil)
}
