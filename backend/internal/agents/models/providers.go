package models

import (
	"os"
	"strings"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

// ---------- 供应商配置 ----------

// TS 参考: src/agents/models-config.providers.ts (637 行)

// ProviderDefaults 供应商默认配置。
type ProviderDefaults struct {
	BaseURL        string  `json:"baseUrl,omitempty"`
	APIURL         string  `json:"apiUrl,omitempty"`
	DefaultModel   string  `json:"defaultModel,omitempty"`
	ContextWindow  int     `json:"contextWindow,omitempty"`
	MaxTokens      int     `json:"maxTokens,omitempty"`
	CostInput      float64 `json:"costInput,omitempty"`
	CostOutput     float64 `json:"costOutput,omitempty"`
	CostCacheRead  float64 `json:"costCacheRead,omitempty"`
	CostCacheWrite float64 `json:"costCacheWrite,omitempty"`
}

// 供应商默认值
// TS 参考: models-config.providers.ts 常量定义
var providerDefaults = map[string]ProviderDefaults{
	"minimax": {
		BaseURL:       "https://api.minimax.chat/v1",
		DefaultModel:  "MiniMax-M2.5",
		ContextWindow: 200_000,
		MaxTokens:     8192,
	},
	"minimax-portal": {
		BaseURL:       "https://api.minimax.io/anthropic",
		DefaultModel:  "MiniMax-M2.5",
		ContextWindow: 200_000,
		MaxTokens:     8192,
	},
	"moonshot": {
		BaseURL:       "https://api.moonshot.ai/v1",
		DefaultModel:  "kimi-k2.5",
		ContextWindow: 256_000,
		MaxTokens:     8192,
	},
	"qwen-portal": {
		BaseURL:       "https://portal.qwen.ai/v1",
		DefaultModel:  "coder-model",
		ContextWindow: 128_000,
		MaxTokens:     8192,
	},
	"ollama": {
		BaseURL:       "http://127.0.0.1:11434/v1",
		APIURL:        "http://127.0.0.1:11434",
		ContextWindow: 128_000,
		MaxTokens:     8192,
	},
	"qianfan": {
		BaseURL:       "https://qianfan.baidubce.com/v2",
		DefaultModel:  "deepseek-v3.2",
		ContextWindow: 98304,
		MaxTokens:     32768,
	},
	"xiaomi": {
		BaseURL:       "https://api.xiaomimimo.com/anthropic",
		DefaultModel:  "mimo-v2-flash",
		ContextWindow: 262_144,
		MaxTokens:     8192,
	},
	"deepseek": {
		BaseURL:       "https://api.deepseek.com/v1",
		DefaultModel:  "deepseek-chat",
		ContextWindow: 128_000,
		MaxTokens:     8192,
	},
	"google": {
		BaseURL:       "https://generativelanguage.googleapis.com/v1beta",
		DefaultModel:  "gemini-3-flash-preview",
		ContextWindow: 1_000_000,
		MaxTokens:     8192,
	},
}

// EnvApiKeyVarNames 供应商 API Key 环境变量名映射。
// P4-DRIFT4: 对齐 TS model-auth.ts L287-307 完整映射。
var EnvApiKeyVarNames = map[string]string{
	"anthropic":             "ANTHROPIC_API_KEY",
	"openai":                "OPENAI_API_KEY",
	"google":                "GEMINI_API_KEY",
	"google-vertex":         "GOOGLE_API_KEY",
	"mistral":               "MISTRAL_API_KEY",
	"groq":                  "GROQ_API_KEY",
	"deepseek":              "DEEPSEEK_API_KEY",
	"qwen":                  "DASHSCOPE_API_KEY",
	"qwen-portal":           "DASHSCOPE_API_KEY",
	"moonshot":              "MOONSHOT_API_KEY",
	"minimax":               "MINIMAX_API_KEY",
	"minimax-portal":        "MINIMAX_API_KEY",
	"xai":                   "XAI_API_KEY",
	"zai":                   "XAI_API_KEY",
	"venice":                "VENICE_API_KEY",
	"qianfan":               "QIANFAN_API_KEY",
	"xiaomi":                "TIZI_API_KEY",
	"voyage":                "VOYAGE_API_KEY",
	"deepgram":              "DEEPGRAM_API_KEY",
	"cerebras":              "CEREBRAS_API_KEY",
	"openrouter":            "OPENROUTER_API_KEY",
	"vercel-ai-gateway":     "AI_GATEWAY_API_KEY",
	"cloudflare-ai-gateway": "CLOUDFLARE_AI_GATEWAY_API_KEY",
	"synthetic":             "SYNTHETIC_API_KEY",
	"openacosmi":            "OPENACOSMI_API_KEY",
	"ollama":                "OLLAMA_API_KEY",
	"chutes":                "CHUTES_API_KEY",
	"kimi-coding":           "MOONSHOT_API_KEY",
}

// EnvApiKeyFallbacks OAuth/令牌回退链。
// TS 参考: model-auth.ts resolveEnvApiKey 中的 fallback 链。
var EnvApiKeyFallbacks = map[string][]string{
	"anthropic":      {"ANTHROPIC_OAUTH_TOKEN"},
	"zai":            {"ZAI_API_KEY"},
	"qwen-portal":    {"QWEN_PORTAL_API_KEY"},
	"minimax-portal": {"MINIMAX_PORTAL_API_KEY"},
	"kimi-coding":    {"KIMI_CODING_API_KEY"},
}

// ResolveEnvApiKeyVarName 解析供应商 API Key 环境变量名。
func ResolveEnvApiKeyVarName(provider string) string {
	normalized := strings.ToLower(strings.TrimSpace(provider))
	if varName, ok := EnvApiKeyVarNames[normalized]; ok {
		return varName
	}
	return ""
}

// ResolveEnvApiKeyWithFallback P4-DRIFT4: 解析 API Key，支持 OAuth 回退链。
// 对齐 TS model-auth.ts resolveEnvApiKey() 中的 fallback 逻辑。
func ResolveEnvApiKeyWithFallback(provider string) string {
	normalized := strings.ToLower(strings.TrimSpace(provider))

	// 主环境变量
	if varName, ok := EnvApiKeyVarNames[normalized]; ok {
		if val := os.Getenv(varName); val != "" {
			return val
		}
	}

	// 回退链
	if fallbacks, ok := EnvApiKeyFallbacks[normalized]; ok {
		for _, fb := range fallbacks {
			if val := os.Getenv(fb); val != "" {
				return val
			}
		}
	}

	return ""
}

// NormalizeApiKeyConfig 规范化 API Key 配置字符串。
func NormalizeApiKeyConfig(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	// 检查是否为环境变量引用 (以 $ 开头)
	if strings.HasPrefix(trimmed, "$") {
		envVar := strings.TrimPrefix(trimmed, "$")
		envVar = strings.Trim(envVar, "{}")
		return os.Getenv(envVar)
	}
	return trimmed
}

// NormalizeGoogleModelId 规范化 Google 模型 ID。
// TS 参考: models-config.providers.ts L191-198
func NormalizeGoogleModelId(id string) string {
	if id == "" {
		return id
	}
	// gemini-3 系列名称映射
	if id == "gemini-3.1-pro" {
		return "gemini-3.1-pro-preview"
	}
	if id == "gemini-3-pro" {
		return "gemini-3-pro-preview"
	}
	if id == "gemini-3-flash" {
		return "gemini-3-flash-preview"
	}
	// 移除 models/ 前缀
	if strings.HasPrefix(id, "models/") {
		return strings.TrimPrefix(id, "models/")
	}
	return id
}

// GetProviderDefaults 获取供应商默认配置。
func GetProviderDefaults(provider string) *ProviderDefaults {
	normalized := strings.ToLower(strings.TrimSpace(provider))
	if defaults, ok := providerDefaults[normalized]; ok {
		return &defaults
	}
	return nil
}

// ResolveProviderBaseURL 解析供应商 base URL。
func ResolveProviderBaseURL(provider string, cfg *types.OpenAcosmiConfig) string {
	// 使用默认值
	defaults := GetProviderDefaults(provider)
	if defaults != nil {
		return defaults.BaseURL
	}
	return ""
}

// ---------- 工具级 API Key 环境变量（非 LLM provider） ----------
// 对应 TS: tools/web-search.ts 和 tools/web-fetch.ts 中的 env fallback 逻辑。

// ToolEnvApiKeyVarNames 工具名称到环境变量名的映射。
// configPath 对应 schema_hints_data.go 中的 "fallback: XXX_API_KEY env var" 描述。
var ToolEnvApiKeyVarNames = map[string]string{
	"brave_search":  "BRAVE_API_KEY",
	"firecrawl":     "FIRECRAWL_API_KEY",
	"perplexity":    "PERPLEXITY_API_KEY",
	"openrouter":    "OPENROUTER_API_KEY",
	"elevenlabs":    "ELEVENLABS_API_KEY",
	"elevenlabs_xi": "XI_API_KEY",
}

// ResolveToolApiKey 解析工具级 API Key。
// configValue 为配置文件中的显式值（优先）；tool 为工具名（用于 env fallback）。
// TS 参考: web-search.ts resolveSearchApiKey(), web-fetch.ts resolveFirecrawlApiKey()
func ResolveToolApiKey(tool, configValue string) string {
	if v := NormalizeApiKeyConfig(configValue); v != "" {
		return v
	}
	if envVar, ok := ToolEnvApiKeyVarNames[strings.ToLower(tool)]; ok {
		return os.Getenv(envVar)
	}
	return ""
}
