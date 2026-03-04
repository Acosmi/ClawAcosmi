// bridge/wizard_catalog.go — 前端 Wizard V2 用的 provider 目录构建
//
// 从 providerRegistry + defaultModelRefs 自动提取完整 provider 元数据，
// 消除前端硬编码。前端只保留 icon/color/bg 等视觉属性。
package bridge

import (
	"sort"

	gptypes "github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// WizardProviderEntry 前端 wizard 用的 provider 描述。
type WizardProviderEntry struct {
	ID                   string             `json:"id"`
	Name                 string             `json:"name"`
	Desc                 string             `json:"desc"`
	Category             string             `json:"category"`
	SortOrder            int                `json:"sortOrder"`
	AuthModes            []string           `json:"authModes"`
	DefaultModelRef      string             `json:"defaultModelRef"`
	Models               []WizardModelEntry `json:"models"`
	CustomBaseUrlAllowed bool               `json:"customBaseUrlAllowed"`
	RequiresBaseUrl      bool               `json:"requiresBaseUrl"`
}

// WizardModelEntry 前端模型下拉用的模型条目。
type WizardModelEntry struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Reasoning     bool     `json:"reasoning"`
	Input         []string `json:"input"`
	ContextWindow int      `json:"contextWindow"`
	MaxTokens     int      `json:"maxTokens"`
}

// wizardProviderMeta 所有 provider 的显示元数据。
// ID 使用前端 ID（如 "doubao" 不是 "volcengine"），与 wizard.v2.apply 的 payload 一致。
var wizardProviderMeta = []struct {
	ID                   string
	Name                 string
	Desc                 string
	Category             string
	SortOrder            int
	AuthModes            []string
	CustomBaseUrlAllowed bool
	RequiresBaseUrl      bool
}{
	// --- 优先推荐组 (OAuth 支持) ---
	{ID: "google", Name: "Google (Gemini)", Desc: "Gemini 系列模型，多模态能力出众。支持 OAuth 一键授权或 API Key。", Category: "oauth_priority", SortOrder: 1, AuthModes: []string{"oauth", "apiKey"}},
	{ID: "qwen", Name: "Qwen 通义千问 (阿里)", Desc: "通义千问大模型，中文开源天花板。支持 Portal 设备码授权或 API Key。", Category: "oauth_priority", SortOrder: 2, AuthModes: []string{"deviceCode", "apiKey"}},
	{ID: "github-copilot", Name: "GitHub Copilot", Desc: "基于 GitHub Copilot 订阅。通过 Device Flow 设备码授权登录。", Category: "oauth_priority", SortOrder: 3, AuthModes: []string{"deviceCode"}},
	{ID: "minimax", Name: "MiniMax (海螺)", Desc: "海螺大模型，长文本处理专家。支持 Portal 设备码授权或 API Key。", Category: "oauth_priority", SortOrder: 4, AuthModes: []string{"deviceCode", "apiKey"}},

	// --- 国内主力组 ---
	{ID: "deepseek", Name: "DeepSeek", Desc: "深度求索系列，国产之光。支持最新 V3/R1。", Category: "china_major", SortOrder: 1, AuthModes: []string{"apiKey"}},
	{ID: "doubao", Name: "Doubao (字节跳动 Ark)", Desc: "字节跳动火山引擎主力大模型。", Category: "china_major", SortOrder: 2, AuthModes: []string{"apiKey"}},
	{ID: "zhipu", Name: "智谱 (Zhipu / Z.AI)", Desc: "GLM 系列模型，国内老牌实力厂商。", Category: "china_major", SortOrder: 3, AuthModes: []string{"apiKey"}},
	{ID: "moonshot", Name: "Kimi (月之暗面)", Desc: "长文本处理专家，中文原生优化专家。", Category: "china_major", SortOrder: 4, AuthModes: []string{"apiKey"}},

	// --- 国际主力组 ---
	{ID: "openai", Name: "OpenAI", Desc: "行业标杆 GPT 与 o 系列推理模型。", Category: "international", SortOrder: 1, AuthModes: []string{"apiKey"}},
	{ID: "anthropic", Name: "Anthropic", Desc: "Claude 系列模型，顶级代码与复杂推理能力。", Category: "international", SortOrder: 2, AuthModes: []string{"apiKey"}},
	{ID: "xai", Name: "xAI Grok", Desc: "Grok 系列模型，无删减的直白输出。", Category: "international", SortOrder: 3, AuthModes: []string{"apiKey"}},

	// --- 新兴平台组 ---
	{ID: "qianfan", Name: "百度千帆 (Qianfan)", Desc: "百度千帆平台，ERNIE 系列大模型。", Category: "emerging", SortOrder: 1, AuthModes: []string{"apiKey"}},
	{ID: "xiaomi", Name: "小米 MiMo", Desc: "小米自研 MiMo 大模型，主打端侧+云侧协同。", Category: "emerging", SortOrder: 2, AuthModes: []string{"apiKey"}},
	{ID: "kimi-coding", Name: "Kimi Coding (月之暗面)", Desc: "Kimi 专注编程模型，代码补全与重构。", Category: "emerging", SortOrder: 3, AuthModes: []string{"apiKey"}},
	{ID: "mistral", Name: "Mistral AI", Desc: "欧洲顶尖开源模型厂商，高效小参数量领先。", Category: "emerging", SortOrder: 4, AuthModes: []string{"apiKey"}},

	// --- 聚合平台组 ---
	{ID: "openrouter", Name: "OpenRouter", Desc: "多模型统一网关路由，一个 Key 接入数百模型。", Category: "aggregator", SortOrder: 1, AuthModes: []string{"apiKey"}},
	{ID: "together", Name: "Together AI", Desc: "高性价比推理平台，主流开源模型全覆盖。", Category: "aggregator", SortOrder: 2, AuthModes: []string{"apiKey"}},
	{ID: "huggingface", Name: "Hugging Face", Desc: "全球最大开源模型社区，Inference API 即开即用。", Category: "aggregator", SortOrder: 3, AuthModes: []string{"apiKey"}},
	{ID: "litellm", Name: "LiteLLM Proxy", Desc: "轻量级多模型代理，统一 100+ LLM API 调用。", Category: "aggregator", SortOrder: 4, AuthModes: []string{"apiKey"}},
	{ID: "byteplus", Name: "BytePlus (海外火山)", Desc: "字节跳动海外版云服务，Doubao 全球版。", Category: "aggregator", SortOrder: 5, AuthModes: []string{"apiKey"}},

	// --- 本地推理与自定义组 ---
	{ID: "custom-openai", Name: "自定义端点 (OpenAI 兼容)", Desc: "支持接入 OpenRouter, Cloudflare AI Gateway, vLLM 等符合 OpenAI 协议的自定义服务。", Category: "local_custom", SortOrder: 1, AuthModes: []string{"apiKey"}, CustomBaseUrlAllowed: true, RequiresBaseUrl: true},
	{ID: "ollama", Name: "Ollama (本地私有化)", Desc: "零配置接入本地私有化模型体系。", Category: "local_custom", SortOrder: 2, AuthModes: []string{"none"}, CustomBaseUrlAllowed: true},
}

// frontendToBackendID 前端 wizard ID → 后端注册表 ID 映射。
// 注意：不能使用 NormalizeProviderID()，因为它走 common.NormalizeProviderId
// 会将 "qwen" 映射为 "qwen-portal"，而 defaultModelRefs 的 key 是 "qwen"。
var frontendToBackendID = map[string]string{
	"doubao": "volcengine",
	"zhipu":  "zai",
}

// BuildWizardProviderCatalog 构建完整的 provider 目录。
// 从 wizardProviderMeta 获取显示信息，从 defaultModelRefs/providerRegistry 提取模型列表。
func BuildWizardProviderCatalog() []WizardProviderEntry {
	result := make([]WizardProviderEntry, 0, len(wizardProviderMeta))

	for _, meta := range wizardProviderMeta {
		entry := WizardProviderEntry{
			ID:                   meta.ID,
			Name:                 meta.Name,
			Desc:                 meta.Desc,
			Category:             meta.Category,
			SortOrder:            meta.SortOrder,
			AuthModes:            meta.AuthModes,
			CustomBaseUrlAllowed: meta.CustomBaseUrlAllowed,
			RequiresBaseUrl:      meta.RequiresBaseUrl,
		}

		// 解析后端 provider ID（使用 frontendToBackendID，不用 NormalizeProviderID
		// 因为后者会将 qwen→qwen-portal 等，与 defaultModelRefs 的 key 不匹配）
		backendID := meta.ID
		if mapped, ok := frontendToBackendID[meta.ID]; ok {
			backendID = mapped
		}

		// 获取默认模型 ref
		entry.DefaultModelRef = GetDefaultModelRef(backendID)

		// 提取模型列表
		entry.Models = extractModelsForProvider(backendID)

		result = append(result, entry)
	}

	// 按 category 分组排序（保持原有顺序，category 内按 sortOrder）
	categoryOrder := map[string]int{
		"oauth_priority": 0,
		"china_major":    1,
		"international":  2,
		"emerging":       3,
		"aggregator":     4,
		"local_custom":   5,
	}
	sort.SliceStable(result, func(i, j int) bool {
		ci := categoryOrder[result[i].Category]
		cj := categoryOrder[result[j].Category]
		if ci != cj {
			return ci < cj
		}
		return result[i].SortOrder < result[j].SortOrder
	})

	return result
}

// extractModelsForProvider 从后端数据源提取 provider 的模型列表。
func extractModelsForProvider(backendID string) []WizardModelEntry {
	// 1. defaultModelRefs（19 provider，完整模型列表 — 唯一权威来源）
	if info, ok := defaultModelRefs[backendID]; ok {
		return convertGPModels(info.DefaultModels)
	}

	// 2. providerRegistry Apply 提取（剩余 providerRegistry-only provider:
	//    zai/moonshot/xai/mistral/kilocode/xiaomi/qianfan/kimi-coding）
	if applyFn, ok := providerRegistry[backendID]; ok {
		models := extractModelsFromApply(applyFn, backendID)
		if len(models) > 0 {
			return models
		}
	}

	// 返回空切片而非 nil，确保 JSON 序列化为 [] 而非 null
	return []WizardModelEntry{}
}

// extractModelsFromApply 调用 Apply 函数在空配置上，从结果 map 中提取模型列表。
func extractModelsFromApply(applyFn providerApplyFunc, providerID string) []WizardModelEntry {
	emptyMap := make(map[string]interface{})
	resultMap := applyFn(emptyMap)

	// 提取 models.providers[providerID].models
	modelsMap, _ := resultMap["models"].(map[string]interface{})
	if modelsMap == nil {
		return nil
	}
	providersMap, _ := modelsMap["providers"].(map[string]interface{})
	if providersMap == nil {
		return nil
	}
	provMap, _ := providersMap[providerID].(map[string]interface{})
	if provMap == nil {
		return nil
	}

	modelsRaw, ok := provMap["models"]
	if !ok {
		return nil
	}
	gpModels, ok := modelsRaw.([]gptypes.ModelDefinitionConfig)
	if !ok {
		return nil
	}

	return convertGPModels(gpModels)
}

// convertGPModels 将 go-providers ModelDefinitionConfig 列表转换为 WizardModelEntry 列表。
func convertGPModels(models []gptypes.ModelDefinitionConfig) []WizardModelEntry {
	result := make([]WizardModelEntry, 0, len(models))
	for _, m := range models {
		entry := WizardModelEntry{
			ID:            m.ID,
			Name:          m.Name,
			Reasoning:     m.Reasoning,
			Input:         m.Input,
			ContextWindow: m.ContextWindow,
			MaxTokens:     m.MaxTokens,
		}
		if entry.Name == "" {
			entry.Name = entry.ID
		}
		result = append(result, entry)
	}
	return result
}
