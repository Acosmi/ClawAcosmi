// onboard/config_opencode.go — OpenCode Zen 配置
// 对应 TS 文件: src/commands/onboard-auth.config-opencode.ts
package onboard

// OpencodeZenDefaultModelRef OpenCode Zen 默认模型引用。
const OpencodeZenDefaultModelRef = "opencode/claude-opus-4-6"

// ApplyOpencodeZenProviderConfig 应用 OpenCode Zen Provider 配置。
func ApplyOpencodeZenProviderConfig(cfg OpenClawConfig) OpenClawConfig {
	models := getAgentModels(cfg)
	setModelAlias(models, OpencodeZenDefaultModelRef, "Opus")

	result := copyMap(cfg)
	agents := copyMap(getNestedMap(cfg, "agents"))
	defaults := copyMap(getNestedMap(cfg, "agents", "defaults"))
	defaults["models"] = models
	agents["defaults"] = defaults
	result["agents"] = agents
	return result
}

// ApplyOpencodeZenConfig 应用 OpenCode Zen 配置并设置为默认模型。
func ApplyOpencodeZenConfig(cfg OpenClawConfig) OpenClawConfig {
	next := ApplyOpencodeZenProviderConfig(cfg)
	return ApplyAgentDefaultModelPrimary(next, OpencodeZenDefaultModelRef)
}
