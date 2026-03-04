// oauth/minimax/plugin.go — MiniMax Portal 插件定义
// 对应 TS 文件: extensions/minimax-portal-auth/index.ts (173 行)
// 本文件定义 MiniMax Portal OAuth 插件的元数据、模型定义和注册信息。
package minimax

// ──────────────────── 插件常量 ────────────────────

const (
	// ProviderID MiniMax Portal 提供者标识。
	ProviderID = "minimax-portal"
	// ProviderLabel 提供者显示名称。
	ProviderLabel = "MiniMax"
	// DefaultModelID 默认模型 ID。
	DefaultModelID = "MiniMax-M2.5"
	// DefaultBaseURLCN 中国大陆默认 API 基础 URL。
	DefaultBaseURLCN = "https://api.minimaxi.com/anthropic"
	// DefaultBaseURLGlobal 全球默认 API 基础 URL。
	DefaultBaseURLGlobal = "https://api.minimax.io/anthropic"
	// DefaultContextWindow 默认上下文窗口大小。
	DefaultContextWindow = 200000
	// DefaultMaxTokens 默认最大输出 token 数。
	DefaultMaxTokens = 8192
	// OAuthPlaceholder OAuth 占位符 API Key。
	OAuthPlaceholder = "minimax-oauth"
)

// GetDefaultBaseURL 根据区域返回默认 API 基础 URL。
// 对应 TS: getDefaultBaseUrl()
func GetDefaultBaseURL(region MiniMaxRegion) string {
	if region == RegionCN {
		return DefaultBaseURLCN
	}
	return DefaultBaseURLGlobal
}

// ModelRef 生成模型全限定引用标识。
// 对应 TS: modelRef()
func ModelRef(modelID string) string {
	return ProviderID + "/" + modelID
}

// ModelDefinitionParams 模型定义参数。
type ModelDefinitionParams struct {
	// ID 模型 ID
	ID string
	// Name 模型显示名称
	Name string
	// Input 支持的输入类型
	Input []string
	// Reasoning 是否支持推理
	Reasoning bool
}

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

// BuildModelDefinition 构建模型定义。
// 对应 TS: buildModelDefinition()
func BuildModelDefinition(params ModelDefinitionParams) ModelDefinition {
	return ModelDefinition{
		ID:            params.ID,
		Name:          params.Name,
		Reasoning:     params.Reasoning,
		Input:         params.Input,
		Cost:          ModelCost{},
		ContextWindow: DefaultContextWindow,
		MaxTokens:     DefaultMaxTokens,
	}
}

// DefaultModels 默认模型列表。
// 对应 TS: index.ts 中的 models 数组
var DefaultModels = []ModelDefinition{
	BuildModelDefinition(ModelDefinitionParams{
		ID:    "MiniMax-M2.5",
		Name:  "MiniMax M2.5",
		Input: []string{"text"},
	}),
	BuildModelDefinition(ModelDefinitionParams{
		ID:        "MiniMax-M2.5-highspeed",
		Name:      "MiniMax M2.5 Highspeed",
		Input:     []string{"text"},
		Reasoning: true,
	}),
	BuildModelDefinition(ModelDefinitionParams{
		ID:        "MiniMax-M2.5-Lightning",
		Name:      "MiniMax M2.5 Lightning",
		Input:     []string{"text"},
		Reasoning: true,
	}),
}

// ──────────────────── 插件注册信息 ────────────────────

// PluginInfo MiniMax Portal OAuth 插件信息。
type PluginInfo struct {
	ID          string
	Name        string
	Description string
}

// GetPluginInfo 返回插件元信息。
func GetPluginInfo() PluginInfo {
	return PluginInfo{
		ID:          "minimax-portal-auth",
		Name:        "MiniMax OAuth",
		Description: "OAuth flow for MiniMax models",
	}
}

// AuthRegistration 认证方式注册信息。
type AuthRegistration struct {
	ID    string
	Label string
	Hint  string
	Kind  string
}

// GetAuthRegistrations 返回认证方式注册列表。
// 对应 TS: auth 数组（包含 global 和 cn 两个区域）
func GetAuthRegistrations() []AuthRegistration {
	return []AuthRegistration{
		{
			ID:    "oauth",
			Label: "MiniMax OAuth (Global)",
			Hint:  "Global endpoint - api.minimax.io",
			Kind:  "device_code",
		},
		{
			ID:    "oauth-cn",
			Label: "MiniMax OAuth (CN)",
			Hint:  "CN endpoint - api.minimaxi.com",
			Kind:  "device_code",
		},
	}
}
