// oauth/qwen/plugin.go — Qwen Portal 插件定义
// 对应 TS 文件: extensions/qwen-portal-auth/index.ts (135 行)
// 本文件定义 Qwen Portal OAuth 插件的元数据、模型定义和注册信息。
package qwen

import "strings"

// ──────────────────── 插件常量 ────────────────────

const (
	// ProviderID Qwen Portal 提供者标识。
	ProviderID = "qwen-portal"
	// ProviderLabel 提供者显示名称。
	ProviderLabel = "Qwen"
	// DefaultModelID 默认模型标识。
	DefaultModelID = "qwen-portal/coder-model"
	// DefaultBaseURL 默认 API 基础 URL。
	DefaultBaseURL = "https://portal.qwen.ai/v1"
	// DefaultContextWindow 默认上下文窗口大小。
	DefaultContextWindow = 128000
	// DefaultMaxTokens 默认最大输出 token 数。
	DefaultMaxTokens = 8192
	// OAuthPlaceholder OAuth 占位符 API Key。
	OAuthPlaceholder = "qwen-oauth"
)

// NormalizeBaseURL 规范化 API 基础 URL。
// 对应 TS: normalizeBaseUrl()
func NormalizeBaseURL(value string) string {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return DefaultBaseURL
	}
	withProtocol := raw
	if !strings.HasPrefix(raw, "http") {
		withProtocol = "https://" + raw
	}
	if strings.HasSuffix(withProtocol, "/v1") {
		return withProtocol
	}
	return strings.TrimRight(withProtocol, "/") + "/v1"
}

// ──────────────────── 模型定义 ────────────────────

// ModelDefinition 模型定义。
type ModelDefinition struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Reasoning     bool      `json:"reasoning"`
	Input         []string  `json:"input"`
	Cost          ModelCost `json:"cost"`
	ContextWindow int       `json:"contextWindow"`
	MaxTokens     int       `json:"maxTokens"`
}

// ModelCost 模型费用。
type ModelCost struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
}

// DefaultModels 默认模型列表。
// 对应 TS: index.ts 中 models 数组
var DefaultModels = []ModelDefinition{
	{
		ID:            "coder-model",
		Name:          "Qwen Coder",
		Reasoning:     false,
		Input:         []string{"text"},
		Cost:          ModelCost{},
		ContextWindow: DefaultContextWindow,
		MaxTokens:     DefaultMaxTokens,
	},
	{
		ID:            "vision-model",
		Name:          "Qwen Vision",
		Reasoning:     false,
		Input:         []string{"text", "image"},
		Cost:          ModelCost{},
		ContextWindow: DefaultContextWindow,
		MaxTokens:     DefaultMaxTokens,
	},
}

// ──────────────────── 插件注册信息 ────────────────────

// PluginInfo Qwen Portal OAuth 插件信息。
type PluginInfo struct {
	ID          string
	Name        string
	Description string
}

// GetPluginInfo 返回插件元信息。
func GetPluginInfo() PluginInfo {
	return PluginInfo{
		ID:          "qwen-portal-auth",
		Name:        "Qwen OAuth",
		Description: "OAuth flow for Qwen (free-tier) models",
	}
}

// AuthRegistration 认证方式注册信息。
type AuthRegistration struct {
	ID    string
	Label string
	Hint  string
	Kind  string
}

// GetAuthRegistration 返回认证方式注册配置。
func GetAuthRegistration() AuthRegistration {
	return AuthRegistration{
		ID:    "device",
		Label: "Qwen OAuth",
		Hint:  "Device code login",
		Kind:  "device_code",
	}
}
