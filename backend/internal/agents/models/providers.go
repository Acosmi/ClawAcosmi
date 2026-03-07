package models

import (
	"os"
	"strings"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
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
		BaseURL:       "https://api.minimax.io/v1",
		DefaultModel:  "MiniMax-Text-01",
		ContextWindow: 1_000_000,
		MaxTokens:     8192,
	},
	"minimax-portal": {
		BaseURL:       "https://api.minimax.io/anthropic",
		DefaultModel:  "MiniMax-M2.5",
		ContextWindow: 1_000_000,
		MaxTokens:     8192,
	},
	"moonshot": {
		BaseURL:       "https://api.moonshot.ai/v1",
		DefaultModel:  "moonshot-v1-kimi-k2", // K2.5 稳定别名（2026-01-27）
		ContextWindow: 131_072,
		MaxTokens:     16384,
	},
	"qwen-portal": {
		BaseURL:       "https://portal.qwen.ai/v1",
		DefaultModel:  "qwen3.5-plus",
		ContextWindow: 1_000_000,
		MaxTokens:     81920,
	},
	"ollama": {
		BaseURL:       "http://127.0.0.1:11434/v1",
		APIURL:        "http://127.0.0.1:11434",
		ContextWindow: 128_000,
		MaxTokens:     8192,
	},
	"deepseek": {
		BaseURL:       "https://api.deepseek.com/v1",
		DefaultModel:  "deepseek-chat",
		ContextWindow: 131_072, // DeepSeek-V3.2 实际 128K
		MaxTokens:     8192,
	},
	"google": {
		BaseURL:       "https://generativelanguage.googleapis.com/v1beta",
		DefaultModel:  "gemini-2.5-flash",
		ContextWindow: 1_000_000,
		MaxTokens:     8192,
	},
	"doubao": {
		BaseURL:       "https://ark.cn-beijing.volces.com/api/v3",
		DefaultModel:  "doubao-pro-32k",
		ContextWindow: 32_768,
		MaxTokens:     4096,
	},
	"qwen": {
		BaseURL:       "https://dashscope.aliyuncs.com/compatible-mode/v1",
		DefaultModel:  "qwen3.5-plus",
		ContextWindow: 1_000_000,
		MaxTokens:     81920,
	},
	"zai": {
		BaseURL:       "https://open.bigmodel.cn/api/paas/v4",
		DefaultModel:  "glm-4-plus",
		ContextWindow: 128_000,
		MaxTokens:     4096,
	},
	"openai": {
		BaseURL:       "https://api.openai.com/v1",
		DefaultModel:  "gpt-4o",
		ContextWindow: 128_000,
		MaxTokens:     16384,
	},
	"anthropic": {
		BaseURL:       "https://api.anthropic.com/v1",
		DefaultModel:  "claude-sonnet-4-20250514",
		ContextWindow: 200_000,
		MaxTokens:     8192,
	},
	"xai": {
		BaseURL:       "https://api.x.ai/v1",
		DefaultModel:  "grok-3",
		ContextWindow: 131_072,
		MaxTokens:     8192,
	},
}

// EnvApiKeyVarNames 供应商 API Key 环境变量名映射。
// P4-DRIFT4: 对齐 TS model-auth.ts L287-307 完整映射。
var EnvApiKeyVarNames = map[string]string{
	"anthropic":      "ANTHROPIC_API_KEY",
	"openai":         "OPENAI_API_KEY",
	"google":         "GEMINI_API_KEY",
	"deepseek":       "DEEPSEEK_API_KEY",
	"qwen":           "DASHSCOPE_API_KEY",
	"qwen-portal":    "DASHSCOPE_API_KEY",
	"moonshot":       "MOONSHOT_API_KEY",
	"minimax":        "MINIMAX_API_KEY",
	"minimax-portal": "MINIMAX_API_KEY",
	"xai":            "XAI_API_KEY",
	"zai":            "ZAI_API_KEY",
	"doubao":         "ARK_API_KEY",
	"openacosmi":     "OPENACOSMI_API_KEY",
	"ollama":         "OLLAMA_API_KEY",
}

// EnvApiKeyFallbacks OAuth/令牌回退链。
// TS 参考: model-auth.ts resolveEnvApiKey 中的 fallback 链。
var EnvApiKeyFallbacks = map[string][]string{
	"anthropic":      {"ANTHROPIC_OAUTH_TOKEN"},
	"qwen-portal":    {"QWEN_PORTAL_API_KEY"},
	"minimax-portal": {"MINIMAX_PORTAL_API_KEY"},
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
