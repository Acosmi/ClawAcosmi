package main

// setup_auth_config_openacosmi.go — OpenAcosmi Zen Provider 配置
// TS 对照: src/commands/onboard-auth.config-opencode.ts (45L)
//
// OpenAcosmi Zen 使用内建 provider（不需要配置 baseUrl/apiKey），
// 只需在 config 中 seed allowlist alias 和设置 default model。

import "github.com/openacosmi/claw-acismi/pkg/types"

// ApplyAcosmiZenProviderConfig 注册 OpenAcosmi Zen provider（仅 alias，无 API 配置）。
// 对应 TS: applyOpencodeZenProviderConfig (config-opencode.ts L4-22)。
func ApplyAcosmiZenProviderConfig(cfg *types.OpenAcosmiConfig) {
	setModelAlias(cfg, acosmiZenDefaultModelRef, "Opus")
}

// ApplyAcosmiZenConfig 注册 OpenAcosmi Zen 并设为默认模型。
// 对应 TS: applyOpencodeZenConfig (config-opencode.ts L24-44)。
func ApplyAcosmiZenConfig(cfg *types.OpenAcosmiConfig) {
	ApplyAcosmiZenProviderConfig(cfg)
	setDefaultModel(cfg, acosmiZenDefaultModelRef)
}
