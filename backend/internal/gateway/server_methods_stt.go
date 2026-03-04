package gateway

// server_methods_stt.go — STT（语音转文本）RPC 方法（Phase C 新增）
// 提供 stt.config.get / stt.config.set / stt.test / stt.models 方法
// 纯新增文件，不修改任何已有方法

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/media"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// STTHandlers 返回 STT RPC 方法处理器。
func STTHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		"stt.config.get": handleSTTConfigGet,
		"stt.config.set": handleSTTConfigSet,
		"stt.test":       handleSTTTest,
		"stt.models":     handleSTTModels,
		"stt.transcribe": handleSTTTranscribe,
	}
}

// ---------- stt.config.get ----------

// STTConfigGetResult stt.config.get 响应
type STTConfigGetResult struct {
	Configured bool              `json:"configured"`
	Provider   string            `json:"provider,omitempty"`
	Model      string            `json:"model,omitempty"`
	BaseURL    string            `json:"baseUrl,omitempty"`
	Language   string            `json:"language,omitempty"`
	HasAPIKey  bool              `json:"hasApiKey"`
	Providers  []STTProviderInfo `json:"providers"`
}

// STTProviderInfo 可选 STT Provider 描述
type STTProviderInfo struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Hint  string `json:"hint,omitempty"`
}

func handleSTTConfigGet(ctx *MethodHandlerContext) {
	// 探测本地 Ollama 是否运行
	ollamaHint := "本地 Ollama 语音模型"
	if ollamaRunning := probeOllama(); ollamaRunning {
		ollamaHint = "已检测到本地 Ollama 运行中 ✓"
	} else {
		ollamaHint = "未检测到 Ollama（localhost:11434）"
	}

	result := STTConfigGetResult{
		Providers: []STTProviderInfo{
			{ID: "qwen", Label: "通义千问 Qwen", Hint: "DashScope API，中文优化，sensevoice-v1"},
			{ID: "openai", Label: "OpenAI Whisper", Hint: "whisper-1 / gpt-4o-transcribe"},
			{ID: "groq", Label: "Groq Whisper", Hint: "极速推理，Whisper Large V3"},
			{ID: "ollama", Label: "本地 Ollama", Hint: ollamaHint},
			{ID: "azure", Label: "Azure Speech", Hint: "企业级私有部署"},
			{ID: "local-whisper", Label: "本地 Whisper", Hint: "离线，需安装 whisper.cpp"},
			{ID: "", Label: "禁用", Hint: "不使用语音转文本"},
		},
	}

	cfg := loadSTTConfigFromCtx(ctx)
	if cfg != nil && cfg.Provider != "" {
		result.Configured = true
		result.Provider = cfg.Provider
		result.Model = cfg.Model
		result.BaseURL = cfg.BaseURL
		result.Language = cfg.Language
		result.HasAPIKey = cfg.APIKey != ""
	}

	ctx.Respond(true, result, nil)
}

// ---------- stt.config.set ----------

func handleSTTConfigSet(ctx *MethodHandlerContext) {
	// 从 map[string]interface{} 提取参数
	paramsJSON, err := json.Marshal(ctx.Params)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "invalid params"))
		return
	}
	var params types.STTConfig
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "parse params: "+err.Error()))
		return
	}

	cfgLoader := ctx.Context.ConfigLoader
	if cfgLoader == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "config loader not available"))
		return
	}

	currentCfg, err := cfgLoader.LoadConfig()
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "load config: "+err.Error()))
		return
	}
	if currentCfg == nil {
		currentCfg = &types.OpenAcosmiConfig{}
	}

	currentCfg.STT = &params

	if err := cfgLoader.WriteConfigFile(currentCfg); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "save config: "+err.Error()))
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"saved":    true,
		"provider": params.Provider,
	}, nil)
}

// ---------- stt.test ----------

func handleSTTTest(ctx *MethodHandlerContext) {
	cfg := loadSTTConfigFromCtx(ctx)
	if cfg == nil || cfg.Provider == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "STT not configured"))
		return
	}

	provider, err := media.NewSTTProvider(cfg)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "create provider: "+err.Error()))
		return
	}

	testCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := provider.TestConnection(testCtx); err != nil {
		ctx.Respond(true, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}, nil)
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"success":  true,
		"provider": provider.Name(),
	}, nil)
}

// ---------- stt.models ----------

func handleSTTModels(ctx *MethodHandlerContext) {
	provider, _ := ctx.Params["provider"].(string)

	models := media.DefaultSTTModels(provider)
	ctx.Respond(true, map[string]interface{}{
		"provider": provider,
		"models":   models,
	}, nil)
}

// ---------- stt.transcribe ----------

// handleSTTTranscribe 接收前端录音的 base64 音频并返回转录文本。
// Params: { audio: string (base64), mimeType: string }
// Response: { text: string }
func handleSTTTranscribe(ctx *MethodHandlerContext) {
	audioBase64, _ := ctx.Params["audio"].(string)
	if audioBase64 == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "audio is required (base64)"))
		return
	}
	mimeType, _ := ctx.Params["mimeType"].(string)
	if mimeType == "" {
		mimeType = "audio/webm"
	}

	// M-04: 先检查 base64 字符串长度，避免解码大数据后再拒绝
	const maxAudioSize = 25 * 1024 * 1024
	const maxBase64Len = maxAudioSize*4/3 + 4
	if len(audioBase64) > maxBase64Len {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "audio too large (max 25 MB)"))
		return
	}

	audioData, err := base64.StdEncoding.DecodeString(audioBase64)
	if err != nil {
		// Fallback: 尝试不带 padding 的解码（某些浏览器生成无 padding 的 base64）
		audioData, err = base64.RawStdEncoding.DecodeString(audioBase64)
		if err != nil {
			ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "invalid base64 audio"))
			return
		}
	}
	if len(audioData) > maxAudioSize {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "audio too large (max 25 MB)"))
		return
	}

	cfg := loadSTTConfigFromCtx(ctx)
	if cfg == nil || cfg.Provider == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "STT not configured"))
		return
	}

	provider, err := media.NewSTTProvider(cfg)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "create STT provider: "+err.Error()))
		return
	}

	transcribeCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	text, err := provider.Transcribe(transcribeCtx, audioData, mimeType)
	if err != nil {
		slog.Error("stt.transcribe failed", "provider", provider.Name(), "error", err)
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "transcription failed"))
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"text": text,
	}, nil)
}

// ---------- helpers ----------

func loadSTTConfigFromCtx(ctx *MethodHandlerContext) *types.STTConfig {
	cfgLoader := ctx.Context.ConfigLoader
	if cfgLoader == nil {
		return nil
	}
	cfg, err := cfgLoader.LoadConfig()
	if err != nil || cfg == nil {
		return nil
	}
	return cfg.STT
}

// probeOllama 探测本地 Ollama 是否运行（GET http://localhost:11434/api/tags，2s 超时）
func probeOllama() bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:11434/api/tags")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
