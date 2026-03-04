// authchoice/noninteractive_extra.go — 非交互式额外 Provider 处理
// 对应 TS: auth-choice.ts 后半段（volcengine/byteplus/moonshot/kimi/zai/minimax 等）
package authchoice

import (
	"fmt"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// niSimpleParams 简单非交互式 provider 处理参数。
type niSimpleParams struct {
	provider    string
	flagValue   *string
	flagName    string
	envVar      string
	profileID   string
	setter      func(types.SecretInput) error
	applyConfig func(OpenClawConfig) OpenClawConfig
}

// applyNISimple 通用简单非交互式 Provider 处理。
func applyNISimple(
	p niSimpleParams,
	runtime RuntimeEnv,
	baseConfig, nextConfig OpenClawConfig,
	requestedMode types.SecretInputMode,
) OpenClawConfig {
	resolved := resolveApiKeyNI(p.provider, baseConfig, p.flagValue, p.flagName, p.envVar, runtime, requestedMode)
	if resolved == nil {
		return nil
	}
	authChoice := types.AuthChoice(p.profileID) // 近似
	if !maybeSetResolvedApiKeyNI(resolved, p.setter, requestedMode, baseConfig, authChoice, runtime) {
		return nil
	}
	nextConfig = ApplyAuthProfileConfig(nextConfig, ApplyProfileConfigParams{
		ProfileID: p.profileID, Provider: p.provider, Mode: "api_key",
	})
	if p.applyConfig != nil {
		return p.applyConfig(nextConfig)
	}
	return nextConfig
}

// applyNIProvidersExtra 非交互式剩余 Provider 处理。
func applyNIProvidersExtra(
	authChoice types.AuthChoice,
	opts types.OnboardOptions,
	runtime RuntimeEnv,
	baseConfig, nextConfig OpenClawConfig,
	requestedMode types.SecretInputMode,
	storageOpts *ApiKeyStorageOptions,
) OpenClawConfig {
	// volcengine-api-key
	if authChoice == types.AuthChoiceVolcengineAPIKey {
		resolved := resolveApiKeyNI("volcengine", baseConfig, opts.VolcengineAPIKey, "--volcengine-api-key", "VOLCANO_ENGINE_API_KEY", runtime, requestedMode)
		if resolved == nil {
			return nil
		}
		if !maybeSetResolvedApiKeyNI(resolved, func(v types.SecretInput) error { return SetVolcengineApiKey(v, "", storageOpts) }, requestedMode, baseConfig, authChoice, runtime) {
			return nil
		}
		nextConfig = ApplyAuthProfileConfig(nextConfig, ApplyProfileConfigParams{ProfileID: "volcengine:default", Provider: "volcengine", Mode: "api_key"})
		return ApplyPrimaryModel(nextConfig, "volcengine-plan/ark-code-latest")
	}

	// byteplus-api-key
	if authChoice == types.AuthChoiceBytePlusAPIKey {
		resolved := resolveApiKeyNI("byteplus", baseConfig, opts.BytePlusAPIKey, "--byteplus-api-key", "BYTEPLUS_API_KEY", runtime, requestedMode)
		if resolved == nil {
			return nil
		}
		if !maybeSetResolvedApiKeyNI(resolved, func(v types.SecretInput) error { return SetByteplusApiKey(v, "", storageOpts) }, requestedMode, baseConfig, authChoice, runtime) {
			return nil
		}
		nextConfig = ApplyAuthProfileConfig(nextConfig, ApplyProfileConfigParams{ProfileID: "byteplus:default", Provider: "byteplus", Mode: "api_key"})
		return ApplyPrimaryModel(nextConfig, "byteplus-plan/ark-code-latest")
	}

	// qianfan-api-key
	if authChoice == types.AuthChoiceQianfanAPIKey {
		return applyNISimple(niSimpleParams{
			provider: "qianfan", flagValue: opts.QianfanAPIKey,
			flagName: "--qianfan-api-key", envVar: "QIANFAN_API_KEY",
			profileID:   "qianfan:default",
			setter:      func(v types.SecretInput) error { return SetQianfanApiKey(v, "", storageOpts) },
			applyConfig: func(cfg OpenClawConfig) OpenClawConfig { return ApplyQianfanConfig(cfg) },
		}, runtime, baseConfig, nextConfig, requestedMode)
	}

	// moonshot-api-key / moonshot-api-key-cn
	if authChoice == types.AuthChoiceMoonshotAPIKey {
		return applyNIMoonshot(opts, runtime, baseConfig, nextConfig, requestedMode, storageOpts, ApplyMoonshotConfig)
	}
	if authChoice == types.AuthChoiceMoonshotAPIKeyCN {
		return applyNIMoonshot(opts, runtime, baseConfig, nextConfig, requestedMode, storageOpts, ApplyMoonshotConfigCn)
	}

	// kimi-code-api-key
	if authChoice == types.AuthChoiceKimiCodeAPIKey {
		return applyNISimple(niSimpleParams{
			provider: "kimi-coding", flagValue: opts.KimiCodeAPIKey,
			flagName: "--kimi-code-api-key", envVar: "KIMI_API_KEY",
			profileID:   "kimi-coding:default",
			setter:      func(v types.SecretInput) error { return SetKimiCodingApiKey(v, "", storageOpts) },
			applyConfig: func(cfg OpenClawConfig) OpenClawConfig { return ApplyKimiCodeConfig(cfg) },
		}, runtime, baseConfig, nextConfig, requestedMode)
	}

	// synthetic-api-key
	if authChoice == types.AuthChoiceSyntheticAPIKey {
		return applyNISimple(niSimpleParams{
			provider: "synthetic", flagValue: opts.SyntheticAPIKey,
			flagName: "--synthetic-api-key", envVar: "SYNTHETIC_API_KEY",
			profileID:   "synthetic:default",
			setter:      func(v types.SecretInput) error { return SetSyntheticApiKey(v, "", storageOpts) },
			applyConfig: func(cfg OpenClawConfig) OpenClawConfig { return ApplySyntheticConfig(cfg) },
		}, runtime, baseConfig, nextConfig, requestedMode)
	}

	// venice-api-key
	if authChoice == types.AuthChoiceVeniceAPIKey {
		return applyNISimple(niSimpleParams{
			provider: "venice", flagValue: opts.VeniceAPIKey,
			flagName: "--venice-api-key", envVar: "VENICE_API_KEY",
			profileID:   "venice:default",
			setter:      func(v types.SecretInput) error { return SetVeniceApiKey(v, "", storageOpts) },
			applyConfig: func(cfg OpenClawConfig) OpenClawConfig { return ApplyVeniceConfig(cfg) },
		}, runtime, baseConfig, nextConfig, requestedMode)
	}

	// opencode-zen
	if authChoice == types.AuthChoiceOpenCodeZen {
		return applyNISimple(niSimpleParams{
			provider: "opencode", flagValue: opts.OpenCodeZenAPIKey,
			flagName: "--opencode-zen-api-key", envVar: "OPENCODE_API_KEY (or OPENCODE_ZEN_API_KEY)",
			profileID:   "opencode:default",
			setter:      func(v types.SecretInput) error { return SetOpencodeZenApiKey(v, "", storageOpts) },
			applyConfig: func(cfg OpenClawConfig) OpenClawConfig { return ApplyOpencodeZenConfig(cfg) },
		}, runtime, baseConfig, nextConfig, requestedMode)
	}

	// together-api-key
	if authChoice == types.AuthChoiceTogetherAPIKey {
		return applyNISimple(niSimpleParams{
			provider: "together", flagValue: opts.TogetherAPIKey,
			flagName: "--together-api-key", envVar: "TOGETHER_API_KEY",
			profileID:   "together:default",
			setter:      func(v types.SecretInput) error { return SetTogetherApiKey(v, "", storageOpts) },
			applyConfig: func(cfg OpenClawConfig) OpenClawConfig { return ApplyTogetherConfig(cfg) },
		}, runtime, baseConfig, nextConfig, requestedMode)
	}

	// huggingface-api-key
	if authChoice == types.AuthChoiceHuggingFaceAPIKey {
		return applyNISimple(niSimpleParams{
			provider: "huggingface", flagValue: opts.HuggingFaceAPIKey,
			flagName: "--huggingface-api-key", envVar: "HF_TOKEN",
			profileID:   "huggingface:default",
			setter:      func(v types.SecretInput) error { return SetHuggingfaceApiKey(v, "", storageOpts) },
			applyConfig: func(cfg OpenClawConfig) OpenClawConfig { return ApplyHuggingfaceConfig(cfg) },
		}, runtime, baseConfig, nextConfig, requestedMode)
	}

	// Z.AI 系列
	if authChoice == types.AuthChoiceZAIAPIKey ||
		authChoice == types.AuthChoiceZAICodingGlobal ||
		authChoice == types.AuthChoiceZAICodingCN ||
		authChoice == types.AuthChoiceZAIGlobal ||
		authChoice == types.AuthChoiceZAICN {
		return applyNIZai(authChoice, opts, runtime, baseConfig, nextConfig, requestedMode, storageOpts)
	}

	// cloudflare-ai-gateway-api-key
	if authChoice == types.AuthChoiceCloudflareAIGatewayKey {
		return applyNICloudflare(opts, runtime, baseConfig, nextConfig, requestedMode, storageOpts)
	}

	// minimax 系列
	if authChoice == types.AuthChoiceMinimaxCloud ||
		authChoice == types.AuthChoiceMinimaxAPI ||
		authChoice == types.AuthChoiceMinimaxAPIKeyCN ||
		authChoice == types.AuthChoiceMinimaxAPILightning {
		return applyNIMinimax(authChoice, opts, runtime, baseConfig, nextConfig, requestedMode, storageOpts)
	}
	if authChoice == types.AuthChoiceMinimax {
		return ApplyMinimaxConfig(nextConfig)
	}

	// OAuth 类选择需要交互模式
	if authChoice == types.AuthChoiceOAuth ||
		authChoice == types.AuthChoiceChutes ||
		authChoice == types.AuthChoiceOpenAICodex ||
		authChoice == types.AuthChoiceQwenPortal ||
		authChoice == types.AuthChoiceMinimaxPortal {
		runtime.Error("OAuth requires interactive mode.")
		runtime.Exit(1)
		return nil
	}

	// custom-api-key: 占位（窗口 7/8 补全完整逻辑）
	if authChoice == types.AuthChoiceCustomAPIKey {
		runtime.Error("Custom provider configuration is not yet implemented in Go.")
		runtime.Exit(1)
		return nil
	}

	return nextConfig
}

// applyNIMoonshot Moonshot 非交互式处理。
func applyNIMoonshot(
	opts types.OnboardOptions, runtime RuntimeEnv,
	baseConfig, nextConfig OpenClawConfig,
	requestedMode types.SecretInputMode, storageOpts *ApiKeyStorageOptions,
	applyConfig func(OpenClawConfig) OpenClawConfig,
) OpenClawConfig {
	resolved := resolveApiKeyNI("moonshot", baseConfig, opts.MoonshotAPIKey, "--moonshot-api-key", "MOONSHOT_API_KEY", runtime, requestedMode)
	if resolved == nil {
		return nil
	}
	ac := types.AuthChoiceMoonshotAPIKey
	if !maybeSetResolvedApiKeyNI(resolved, func(v types.SecretInput) error { return SetMoonshotApiKey(v, "", storageOpts) }, requestedMode, baseConfig, ac, runtime) {
		return nil
	}
	nextConfig = ApplyAuthProfileConfig(nextConfig, ApplyProfileConfigParams{ProfileID: "moonshot:default", Provider: "moonshot", Mode: "api_key"})
	return applyConfig(nextConfig)
}

// applyNIZai Z.AI 非交互式处理。
func applyNIZai(
	authChoice types.AuthChoice, opts types.OnboardOptions, runtime RuntimeEnv,
	baseConfig, nextConfig OpenClawConfig,
	requestedMode types.SecretInputMode, storageOpts *ApiKeyStorageOptions,
) OpenClawConfig {
	resolved := resolveApiKeyNI("zai", baseConfig, opts.ZAIAPIKey, "--zai-api-key", "ZAI_API_KEY", runtime, requestedMode)
	if resolved == nil {
		return nil
	}
	if !maybeSetResolvedApiKeyNI(resolved, func(v types.SecretInput) error { return SetZaiApiKey(v, "", storageOpts) }, requestedMode, baseConfig, authChoice, runtime) {
		return nil
	}
	nextConfig = ApplyAuthProfileConfig(nextConfig, ApplyProfileConfigParams{ProfileID: "zai:default", Provider: "zai", Mode: "api_key"})

	var endpoint, modelIdOverride string
	switch authChoice {
	case types.AuthChoiceZAICodingGlobal:
		endpoint = "coding-global"
	case types.AuthChoiceZAICodingCN:
		endpoint = "coding-cn"
	case types.AuthChoiceZAIGlobal:
		endpoint = "global"
	case types.AuthChoiceZAICN:
		endpoint = "cn"
	default:
		detected := DetectZaiEndpoint(resolved.Key)
		if detected != nil {
			endpoint = detected.Endpoint
			modelIdOverride = detected.ModelID
		} else {
			endpoint = "global"
		}
	}

	zaiOpts := ZaiConfigOptions{Endpoint: endpoint}
	if modelIdOverride != "" {
		zaiOpts.ModelID = modelIdOverride
	}
	return ApplyZaiConfig(nextConfig, zaiOpts)
}

// applyNICloudflare Cloudflare AI Gateway 非交互式处理。
func applyNICloudflare(
	opts types.OnboardOptions, runtime RuntimeEnv,
	baseConfig, nextConfig OpenClawConfig,
	requestedMode types.SecretInputMode, storageOpts *ApiKeyStorageOptions,
) OpenClawConfig {
	accountID := ""
	gatewayID := ""
	if opts.CloudflareAIGatewayAccountID != nil {
		accountID = strings.TrimSpace(*opts.CloudflareAIGatewayAccountID)
	}
	if opts.CloudflareAIGatewayGatewayID != nil {
		gatewayID = strings.TrimSpace(*opts.CloudflareAIGatewayGatewayID)
	}
	if accountID == "" || gatewayID == "" {
		runtime.Error(strings.Join([]string{
			`Auth choice "cloudflare-ai-gateway-api-key" requires Account ID and Gateway ID.`,
			"Use --cloudflare-ai-gateway-account-id and --cloudflare-ai-gateway-gateway-id.",
		}, "\n"))
		runtime.Exit(1)
		return nil
	}
	resolved := resolveApiKeyNI("cloudflare-ai-gateway", baseConfig, opts.CloudflareAIGatewayAPIKey, "--cloudflare-ai-gateway-api-key", "CLOUDFLARE_AI_GATEWAY_API_KEY", runtime, requestedMode)
	if resolved == nil {
		return nil
	}
	if resolved.Source != "profile" {
		stored := toStoredSecretInput(resolved, requestedMode, baseConfig, types.AuthChoiceCloudflareAIGatewayKey, runtime)
		if stored == nil {
			return nil
		}
		_ = SetCloudflareAiGatewayConfig(accountID, gatewayID, stored, "", storageOpts)
	}
	nextConfig = ApplyAuthProfileConfig(nextConfig, ApplyProfileConfigParams{ProfileID: "cloudflare-ai-gateway:default", Provider: "cloudflare-ai-gateway", Mode: "api_key"})
	return ApplyCloudflareAiGatewayConfigWithIDs(nextConfig, accountID, gatewayID)
}

// applyNIMinimax MiniMax 非交互式处理。
func applyNIMinimax(
	authChoice types.AuthChoice, opts types.OnboardOptions, runtime RuntimeEnv,
	baseConfig, nextConfig OpenClawConfig,
	requestedMode types.SecretInputMode, storageOpts *ApiKeyStorageOptions,
) OpenClawConfig {
	isCn := authChoice == types.AuthChoiceMinimaxAPIKeyCN
	providerID := "minimax"
	if isCn {
		providerID = "minimax-cn"
	}
	profileID := fmt.Sprintf("%s:default", providerID)

	resolved := resolveApiKeyNI(providerID, baseConfig, opts.MinimaxAPIKey, "--minimax-api-key", "MINIMAX_API_KEY", runtime, requestedMode)
	if resolved == nil {
		return nil
	}
	if !maybeSetResolvedApiKeyNI(resolved, func(v types.SecretInput) error { return SetMinimaxApiKey(v, "", profileID, storageOpts) }, requestedMode, baseConfig, authChoice, runtime) {
		return nil
	}
	nextConfig = ApplyAuthProfileConfig(nextConfig, ApplyProfileConfigParams{ProfileID: profileID, Provider: providerID, Mode: "api_key"})

	modelID := "MiniMax-M2.5"
	if authChoice == types.AuthChoiceMinimaxAPILightning {
		modelID = "MiniMax-M2.5-highspeed"
	}
	if isCn {
		return ApplyMinimaxApiConfigCn(nextConfig, modelID)
	}
	return ApplyMinimaxApiConfig(nextConfig, modelID)
}
