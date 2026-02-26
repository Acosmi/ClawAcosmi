package config

// 配置合并工具 — 对应 src/config/merge-config.ts (39 行)
//
// 提供 map 级别的配置合并，支持 unsetOnUndefined 语义。
//
// 依赖: pkg/types

import "github.com/anthropic/open-acosmi/pkg/types"

// MergeConfigSection 合并配置段。
// patch 中非 nil 值会覆盖 base 中的对应键。
// unsetOnUndefined 中列出的键如果在 patch 中值为零值则从结果中删除。
//
// 注意: 由于 Go 泛型对 struct 字段访问的限制，这里使用 map[string]interface{} 形式。
// 对应 TS: mergeConfigSection<T>(base, patch, options)
func MergeConfigSection(
	base map[string]interface{},
	patch map[string]interface{},
	unsetOnUndefined []string,
) map[string]interface{} {
	result := make(map[string]interface{})
	// 复制 base
	for k, v := range base {
		result[k] = v
	}
	// 应用 patch
	unsetKeys := make(map[string]bool, len(unsetOnUndefined))
	for _, key := range unsetOnUndefined {
		unsetKeys[key] = true
	}
	for k, v := range patch {
		if v == nil {
			if unsetKeys[k] {
				delete(result, k)
			}
			continue
		}
		result[k] = v
	}
	return result
}

// MergeWhatsAppConfig 合并 WhatsApp 配置到 OpenAcosmiConfig。
// 对应 TS: mergeWhatsAppConfig(cfg, patch, options)
func MergeWhatsAppConfig(cfg *types.OpenAcosmiConfig, patch *types.WhatsAppConfig) *types.OpenAcosmiConfig {
	if cfg == nil {
		cfg = &types.OpenAcosmiConfig{}
	}
	result := *cfg
	if result.Channels == nil {
		result.Channels = &types.ChannelsConfig{}
	}
	channels := *result.Channels

	if channels.WhatsApp == nil {
		channels.WhatsApp = patch
	} else if patch != nil {
		// 浅合并：非零值覆盖
		merged := *channels.WhatsApp
		if patch.Capabilities != nil {
			merged.Capabilities = patch.Capabilities
		}
		channels.WhatsApp = &merged
	}

	result.Channels = &channels
	return &result
}
