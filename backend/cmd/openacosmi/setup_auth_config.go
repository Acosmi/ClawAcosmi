package main

// setup_auth_config.go — Provider 配置应用函数
// TS 对照: src/commands/onboard-auth.config-core.ts (793L)
//
// 26 个 provider config 变换函数，统一使用 helper 模式减少重复。

import (
	"github.com/anthropic/open-acosmi/pkg/types"
)

// ---------- 内部 helper ----------

// setModelAlias 在 agents.defaults.models 中注册模型别名。
func setModelAlias(cfg *types.OpenAcosmiConfig, ref, alias string) {
	ensureAgentDefaults(cfg)
	if cfg.Agents.Defaults.Models == nil {
		cfg.Agents.Defaults.Models = make(map[string]*types.AgentModelEntryConfig)
	}
	entry := cfg.Agents.Defaults.Models[ref]
	if entry == nil {
		entry = &types.AgentModelEntryConfig{}
		cfg.Agents.Defaults.Models[ref] = entry
	}
	if entry.Alias == "" {
		entry.Alias = alias
	}
}

// setDefaultModel 设置 agents.defaults.model.primary（保留 fallbacks）。
func setDefaultModel(cfg *types.OpenAcosmiConfig, primary string) {
	ensureAgentDefaults(cfg)
	existing := cfg.Agents.Defaults.Model
	if existing == nil {
		cfg.Agents.Defaults.Model = &types.AgentModelListConfig{Primary: primary}
		return
	}
	existing.Primary = primary
}

// ensureProvider 确保 models.providers 中有指定的 provider。
func ensureProvider(cfg *types.OpenAcosmiConfig, name string) *types.ModelProviderConfig {
	if cfg.Models == nil {
		cfg.Models = &types.ModelsConfig{Mode: "merge"}
	}
	if cfg.Models.Providers == nil {
		cfg.Models.Providers = make(map[string]*types.ModelProviderConfig)
	}
	p := cfg.Models.Providers[name]
	if p == nil {
		p = &types.ModelProviderConfig{}
		cfg.Models.Providers[name] = p
	}
	return p
}

// mergeModelIfMissing 向 provider 添加模型（如不存在）。
func mergeModelIfMissing(p *types.ModelProviderConfig, model types.ModelDefinitionConfig) {
	for _, m := range p.Models {
		if m.ID == model.ID {
			return
		}
	}
	p.Models = append(p.Models, model)
}

// ensureAgentDefaults 确保 agents.defaults 存在。
func ensureAgentDefaults(cfg *types.OpenAcosmiConfig) {
	if cfg.Agents == nil {
		cfg.Agents = &types.AgentsConfig{}
	}
	if cfg.Agents.Defaults == nil {
		cfg.Agents.Defaults = &types.AgentDefaultsConfig{}
	}
}

// ---------- Google (Gemini) ----------

const (
	geminiDefaultModelRef = "google/gemini-3-flash-preview"
	geminiDefaultModelID  = "gemini-3-flash-preview"
)

// ApplyGoogleProviderConfig 注册 Google Gemini provider 及模型列表。
func ApplyGoogleProviderConfig(cfg *types.OpenAcosmiConfig) {
	setModelAlias(cfg, geminiDefaultModelRef, "Gemini Flash")
	setModelAlias(cfg, "google/gemini-3-pro-preview", "Gemini Pro")

	p := ensureProvider(cfg, "google")
	p.API = "google-gemini"

	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "gemini-3-flash-preview",
		Name:          "Gemini 3 Flash",
		ContextWindow: 1_000_000,
		MaxTokens:     8192,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "gemini-3-pro-preview",
		Name:          "Gemini 3 Pro",
		ContextWindow: 1_000_000,
		MaxTokens:     8192,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "gemini-3.1-pro-preview",
		Name:          "Gemini 3.1 Pro",
		ContextWindow: 2_000_000,
		MaxTokens:     16384,
	})
}

// ApplyGoogleConfig 注册 Google Gemini 并设为默认模型。
func ApplyGoogleConfig(cfg *types.OpenAcosmiConfig) {
	ApplyGoogleProviderConfig(cfg)
	setDefaultModel(cfg, geminiDefaultModelRef)
}

// ---------- Zai (GLM) ----------

// ApplyZaiConfig 注册 ZAI/GLM provider 并设为默认模型。
func ApplyZaiConfig(cfg *types.OpenAcosmiConfig) {
	ref := "zhipu/glm-4-plus"
	setModelAlias(cfg, ref, "GLM")
	setDefaultModel(cfg, ref)
}

// ---------- OpenRouter ----------

// ApplyOpenrouterProviderConfig 注册 OpenRouter provider（不改默认模型）。
func ApplyOpenrouterProviderConfig(cfg *types.OpenAcosmiConfig) {
	ref := "openrouter/auto"
	setModelAlias(cfg, ref, "OpenRouter")
}

// ApplyOpenrouterConfig 注册 OpenRouter 并设为默认模型。
func ApplyOpenrouterConfig(cfg *types.OpenAcosmiConfig) {
	ApplyOpenrouterProviderConfig(cfg)
	setDefaultModel(cfg, "openrouter/auto")
}

// ---------- Vercel AI Gateway ----------

// ApplyVercelAiGatewayProviderConfig 注册 Vercel AI Gateway（不改默认模型）。
func ApplyVercelAiGatewayProviderConfig(cfg *types.OpenAcosmiConfig) {
	ref := "vercel-ai-gateway/claude-4-sonnet"
	setModelAlias(cfg, ref, "Vercel AI Gateway")
}

// ApplyVercelAiGatewayConfig 注册并设为默认模型。
func ApplyVercelAiGatewayConfig(cfg *types.OpenAcosmiConfig) {
	ApplyVercelAiGatewayProviderConfig(cfg)
	setDefaultModel(cfg, "vercel-ai-gateway/claude-4-sonnet")
}

// ---------- Cloudflare AI Gateway ----------

// CloudflareAiGatewayParams Cloudflare 参数。
type CloudflareAiGatewayParams struct {
	AccountID string
	GatewayID string
}

// ApplyCloudflareAiGatewayProviderConfig 注册 Cloudflare AI Gateway provider。
func ApplyCloudflareAiGatewayProviderConfig(cfg *types.OpenAcosmiConfig, params *CloudflareAiGatewayParams) {
	ref := "cloudflare-ai-gateway/claude-4-sonnet"
	setModelAlias(cfg, ref, "Cloudflare AI Gateway")

	p := ensureProvider(cfg, "cloudflare-ai-gateway")
	p.API = "anthropic-messages"
	if params != nil && params.AccountID != "" && params.GatewayID != "" {
		p.BaseURL = "https://gateway.ai.cloudflare.com/v1/" + params.AccountID + "/" + params.GatewayID + "/anthropic"
	}
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:   "claude-4-sonnet-20260514",
		Name: "Claude 4 Sonnet (Cloudflare)",
	})
}

// ApplyCloudflareAiGatewayConfig 注册并设为默认。
func ApplyCloudflareAiGatewayConfig(cfg *types.OpenAcosmiConfig, params *CloudflareAiGatewayParams) {
	ApplyCloudflareAiGatewayProviderConfig(cfg, params)
	setDefaultModel(cfg, "cloudflare-ai-gateway/claude-4-sonnet")
}

// ---------- Moonshot / Kimi ----------

const (
	moonshotBaseURL   = "https://api.moonshot.cn/v1"
	moonshotCnBaseURL = "https://api.moonshot.cn/v1"
	moonshotModelID   = "moonshot-v1-128k"
	moonshotModelRef  = "moonshot/moonshot-v1-128k"
	kimiCodingRef     = "moonshot/kimi-k2.5-coding"
)

// ApplyMoonshotProviderConfig 注册 Moonshot（国际）。
func ApplyMoonshotProviderConfig(cfg *types.OpenAcosmiConfig) {
	applyMoonshotWithBaseURL(cfg, moonshotBaseURL)
}

// ApplyMoonshotProviderConfigCn 注册 Moonshot（中国区）。
func ApplyMoonshotProviderConfigCn(cfg *types.OpenAcosmiConfig) {
	applyMoonshotWithBaseURL(cfg, moonshotCnBaseURL)
}

func applyMoonshotWithBaseURL(cfg *types.OpenAcosmiConfig, baseURL string) {
	setModelAlias(cfg, moonshotModelRef, "Kimi")
	p := ensureProvider(cfg, "moonshot")
	p.BaseURL = baseURL
	p.API = "openai-completions"
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:   moonshotModelID,
		Name: "Moonshot V1 128K",
	})
}

// ApplyMoonshotConfig 注册 Moonshot 并设为默认。
func ApplyMoonshotConfig(cfg *types.OpenAcosmiConfig) {
	ApplyMoonshotProviderConfig(cfg)
	setDefaultModel(cfg, moonshotModelRef)
}

// ApplyMoonshotConfigCn 注册中国区 Moonshot 并设为默认。
func ApplyMoonshotConfigCn(cfg *types.OpenAcosmiConfig) {
	ApplyMoonshotProviderConfigCn(cfg)
	setDefaultModel(cfg, moonshotModelRef)
}

// ApplyKimiCodeProviderConfig 注册 Kimi K2.5 Coding。
func ApplyKimiCodeProviderConfig(cfg *types.OpenAcosmiConfig) {
	setModelAlias(cfg, kimiCodingRef, "Kimi K2.5")
}

// ApplyKimiCodeConfig 设为默认。
func ApplyKimiCodeConfig(cfg *types.OpenAcosmiConfig) {
	ApplyKimiCodeProviderConfig(cfg)
	setDefaultModel(cfg, kimiCodingRef)
}

// ---------- Synthetic (MiniMax) ----------

const (
	syntheticBaseURL  = "https://api.minimax.chat/v1"
	syntheticModelRef = "synthetic/minimax-m2.1"
	syntheticModelID  = "minimax-m2.1"
)

// ApplySyntheticProviderConfig 注册 Synthetic/MiniMax provider。
func ApplySyntheticProviderConfig(cfg *types.OpenAcosmiConfig) {
	setModelAlias(cfg, syntheticModelRef, "MiniMax M2.1")
	p := ensureProvider(cfg, "synthetic")
	p.BaseURL = syntheticBaseURL
	p.API = "anthropic-messages"
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:   syntheticModelID,
		Name: "MiniMax M2.1",
	})
}

// ApplySyntheticConfig 注册并设为默认。
func ApplySyntheticConfig(cfg *types.OpenAcosmiConfig) {
	ApplySyntheticProviderConfig(cfg)
	setDefaultModel(cfg, syntheticModelRef)
}

// ---------- Xiaomi ----------

const xiaomiModelRef = "xiaomi/mimo-vl-7b"

// ApplyXiaomiProviderConfig 注册 Xiaomi provider。
func ApplyXiaomiProviderConfig(cfg *types.OpenAcosmiConfig) {
	setModelAlias(cfg, xiaomiModelRef, "Xiaomi")
	p := ensureProvider(cfg, "xiaomi")
	if p.BaseURL == "" {
		p.BaseURL = "https://api.xiaomi.com/v1"
	}
	if p.API == "" {
		p.API = "openai-completions"
	}
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:   "mimo-vl-7b",
		Name: "MiMo-VL 7B",
	})
}

// ApplyXiaomiConfig 注册并设为默认。
func ApplyXiaomiConfig(cfg *types.OpenAcosmiConfig) {
	ApplyXiaomiProviderConfig(cfg)
	setDefaultModel(cfg, xiaomiModelRef)
}

// ---------- Venice ----------

const (
	veniceBaseURL  = "https://api.venice.ai/api/v1"
	veniceModelRef = "venice/llama-3.3-70b"
)

// ApplyVeniceProviderConfig 注册 Venice provider。
func ApplyVeniceProviderConfig(cfg *types.OpenAcosmiConfig) {
	setModelAlias(cfg, veniceModelRef, "Llama 3.3 70B")
	p := ensureProvider(cfg, "venice")
	p.BaseURL = veniceBaseURL
	p.API = "openai-completions"
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:   "llama-3.3-70b",
		Name: "Llama 3.3 70B",
	})
}

// ApplyVeniceConfig 注册并设为默认。
func ApplyVeniceConfig(cfg *types.OpenAcosmiConfig) {
	ApplyVeniceProviderConfig(cfg)
	setDefaultModel(cfg, veniceModelRef)
}

// ---------- xAI (Grok) ----------

const (
	xaiBaseURL  = "https://api.x.ai/v1"
	xaiModelRef = "xai/grok-3"
	xaiModelID  = "grok-3"
)

// ApplyXaiProviderConfig 注册 xAI/Grok provider。
func ApplyXaiProviderConfig(cfg *types.OpenAcosmiConfig) {
	setModelAlias(cfg, xaiModelRef, "Grok")
	p := ensureProvider(cfg, "xai")
	p.BaseURL = xaiBaseURL
	p.API = "openai-completions"
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:   xaiModelID,
		Name: "Grok 3",
	})
}

// ApplyXaiConfig 注册并设为默认。
func ApplyXaiConfig(cfg *types.OpenAcosmiConfig) {
	ApplyXaiProviderConfig(cfg)
	setDefaultModel(cfg, xaiModelRef)
}

// ---------- Qianfan (百度千帆) ----------

const (
	qianfanBaseURL  = "https://qianfan.baidubce.com/v2"
	qianfanModelRef = "qianfan/ernie-4.5-8k"
	qianfanModelID  = "ernie-4.5-8k"
)

// ApplyQianfanProviderConfig 注册千帆 provider。
func ApplyQianfanProviderConfig(cfg *types.OpenAcosmiConfig) {
	setModelAlias(cfg, qianfanModelRef, "QIANFAN")
	p := ensureProvider(cfg, "qianfan")
	if p.BaseURL == "" {
		p.BaseURL = qianfanBaseURL
	}
	if p.API == "" {
		p.API = "openai-completions"
	}
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:   qianfanModelID,
		Name: "ERNIE 4.5 8K",
	})
}

// ApplyQianfanConfig 注册并设为默认。
func ApplyQianfanConfig(cfg *types.OpenAcosmiConfig) {
	ApplyQianfanProviderConfig(cfg)
	setDefaultModel(cfg, qianfanModelRef)
}

// ---------- Auth Profile ----------

// AuthProfileParams 多 profile 凭证参数。
type AuthProfileParams struct {
	ProfileID          string
	Provider           string
	Mode               string // "api_key" | "oauth" | "token"
	Email              string
	PreferProfileFirst bool
}

// ApplyAuthProfileConfig 在 auth.profiles 中注册/更新 profile + 维护 auth.order。
func ApplyAuthProfileConfig(cfg *types.OpenAcosmiConfig, params AuthProfileParams) {
	if cfg.Auth == nil {
		cfg.Auth = &types.AuthConfig{}
	}
	if cfg.Auth.Profiles == nil {
		cfg.Auth.Profiles = make(map[string]*types.AuthProfileConfig)
	}

	profile := &types.AuthProfileConfig{
		Provider: params.Provider,
		Mode:     types.AuthProfileMode(params.Mode),
		Email:    params.Email,
	}
	cfg.Auth.Profiles[params.ProfileID] = profile

	// 维护 auth.order
	if cfg.Auth.Order == nil {
		return // 无显式 order 时不创建
	}
	existingOrder, exists := cfg.Auth.Order[params.Provider]
	if !exists {
		return
	}
	preferFirst := params.PreferProfileFirst
	// 确保 profileID 在 order 中
	found := false
	for _, id := range existingOrder {
		if id == params.ProfileID {
			found = true
			break
		}
	}
	if !found {
		existingOrder = append(existingOrder, params.ProfileID)
	}
	if preferFirst {
		// 移到最前
		reordered := []string{params.ProfileID}
		for _, id := range existingOrder {
			if id != params.ProfileID {
				reordered = append(reordered, id)
			}
		}
		existingOrder = reordered
	}
	cfg.Auth.Order[params.Provider] = existingOrder
}

// ---------- 辅助 ----------

func boolPtr(v bool) *bool {
	return &v
}
