package reply

import (
	"sort"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/agents/models"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// TS 对照: auto-reply/reply/directive-handling.model-picker.ts (98L)
// 模型选择器目录条目、provider 排序、endpoint 标签。

// ---------- 类型 ----------

// ModelPickerCatalogEntry 模型选择器目录条目。
// TS 对照: directive-handling.model-picker.ts ModelPickerCatalogEntry
type ModelPickerCatalogEntry struct {
	Provider string
	ID       string
	Name     string
}

// ProviderEndpointLabel provider 端点标签。
type ProviderEndpointLabel struct {
	Endpoint string
	API      string
}

// ---------- Provider 优先级排序 ----------

// modelPickProviderPreference Provider 展示优先级。
// TS 对照: directive-handling.model-picker.ts MODEL_PICK_PROVIDER_PREFERENCE
var modelPickProviderPreference = []string{
	"anthropic",
	"openai",
	"openai-codex",
	"minimax",
	"synthetic",
	"google",
	"zai",
	"openrouter",
	"openacosmi",
	"github-copilot",
	"groq",
	"cerebras",
	"mistral",
	"xai",
	"lmstudio",
}

// providerRank 预计算的 rank 映射。
var providerRank map[string]int

func init() {
	providerRank = make(map[string]int, len(modelPickProviderPreference))
	for i, p := range modelPickProviderPreference {
		providerRank[p] = i
	}
}

// compareProvidersForPicker 按优先级比较两个 provider。
// TS 对照: directive-handling.model-picker.ts compareProvidersForPicker
func compareProvidersForPicker(a, b string) int {
	pa, aOk := providerRank[a]
	pb, bOk := providerRank[b]
	if aOk && bOk {
		return pa - pb
	}
	if aOk {
		return -1
	}
	if bOk {
		return 1
	}
	return strings.Compare(a, b)
}

// BuildModelPickerItems 从目录条目构建去重、排序后的模型选择项列表。
// TS 对照: directive-handling.model-picker.ts buildModelPickerItems
func BuildModelPickerItems(catalog []ModelPickerCatalogEntry) []models.ModelRef {
	seen := make(map[string]bool)
	var out []models.ModelRef

	for _, entry := range catalog {
		provider := models.NormalizeProviderId(entry.Provider)
		model := strings.TrimSpace(entry.ID)
		if provider == "" || model == "" {
			continue
		}
		key := provider + "/" + model
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, models.ModelRef{Provider: provider, Model: model})
	}

	sort.Slice(out, func(i, j int) bool {
		providerOrder := compareProvidersForPicker(out[i].Provider, out[j].Provider)
		if providerOrder != 0 {
			return providerOrder < 0
		}
		return strings.ToLower(out[i].Model) < strings.ToLower(out[j].Model)
	})

	return out
}

// ResolveProviderEndpointLabel 解析 provider 的自定义 endpoint 标签。
// TS 对照: directive-handling.model-picker.ts resolveProviderEndpointLabel
func ResolveProviderEndpointLabel(provider string, cfg *types.OpenAcosmiConfig) ProviderEndpointLabel {
	normalized := models.NormalizeProviderId(provider)
	if cfg == nil || cfg.Models == nil || cfg.Models.Providers == nil {
		return ProviderEndpointLabel{}
	}
	entry := cfg.Models.Providers[normalized]
	if entry == nil {
		return ProviderEndpointLabel{}
	}
	result := ProviderEndpointLabel{}
	if ep := strings.TrimSpace(entry.BaseURL); ep != "" {
		result.Endpoint = ep
	}
	if api := strings.TrimSpace(string(entry.API)); api != "" {
		result.API = api
	}
	return result
}
