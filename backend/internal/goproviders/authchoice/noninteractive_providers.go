// authchoice/noninteractive_providers.go — 非交互式各 Provider 处理
// 对应 TS 文件: src/commands/onboard-non-interactive/local/auth-choice.ts（后半段）
// 从 noninteractive.go 拆分出来避免单文件过大。
package authchoice

import (
	"fmt"
	"strings"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// applyNIToken 处理非交互式 token 认证。
func applyNIToken(
	opts types.OnboardOptions,
	runtime RuntimeEnv,
	nextConfig OpenClawConfig,
	requestedMode types.SecretInputMode,
) OpenClawConfig {
	providerRaw := ""
	if opts.TokenProvider != nil {
		providerRaw = strings.TrimSpace(*opts.TokenProvider)
	}
	if providerRaw == "" {
		runtime.Error("Missing --token-provider for --auth-choice token.")
		runtime.Exit(1)
		return nil
	}
	provider := NormalizeProviderId(providerRaw)
	if provider != "anthropic" {
		runtime.Error("Only --token-provider anthropic is supported for --auth-choice token.")
		runtime.Exit(1)
		return nil
	}
	tokenRaw := NormalizeSecretInput(opts.Token)
	if tokenRaw == "" {
		runtime.Error("Missing --token for --auth-choice token.")
		runtime.Exit(1)
		return nil
	}
	tokenError := ValidateAnthropicSetupToken(tokenRaw)
	if tokenError != "" {
		runtime.Error(tokenError)
		runtime.Exit(1)
		return nil
	}

	var expires int64
	if opts.TokenExpiresIn != nil {
		raw := strings.TrimSpace(*opts.TokenExpiresIn)
		if raw != "" {
			dur, err := ParseDurationMs(raw, "d")
			if err != nil {
				runtime.Error(fmt.Sprintf("Invalid --token-expires-in: %v", err))
				runtime.Exit(1)
				return nil
			}
			_ = requestedMode // 保留引用
			expires = currentTimeMs() + dur
		}
	}

	profileID := BuildTokenProfileId(provider, "")
	if opts.TokenProfileID != nil && strings.TrimSpace(*opts.TokenProfileID) != "" {
		profileID = strings.TrimSpace(*opts.TokenProfileID)
	}

	cred := map[string]interface{}{
		"type":     "token",
		"provider": provider,
		"token":    strings.TrimSpace(tokenRaw),
	}
	if expires > 0 {
		cred["expires"] = expires
	}
	UpsertAuthProfile(profileID, cred)

	return ApplyAuthProfileConfig(nextConfig, ApplyProfileConfigParams{
		ProfileID: profileID,
		Provider:  provider,
		Mode:      "token",
	})
}

// currentTimeMs 获取当前时间毫秒值。
func currentTimeMs() int64 {
	return time.Now().UnixMilli()
}

// applyNIProviders 分发所有 Provider 的非交互式处理。
func applyNIProviders(
	authChoice types.AuthChoice,
	opts types.OnboardOptions,
	runtime RuntimeEnv,
	baseConfig, nextConfig OpenClawConfig,
	requestedMode types.SecretInputMode,
) OpenClawConfig {
	storageOpts := &ApiKeyStorageOptions{SecretInputMode: requestedMode}

	// apiKey (Anthropic)
	if authChoice == types.AuthChoiceAPIKey {
		return applyNISimple(niSimpleParams{
			provider: "anthropic", flagValue: opts.AnthropicAPIKey,
			flagName: "--anthropic-api-key", envVar: "ANTHROPIC_API_KEY",
			profileID:   "anthropic:default",
			setter:      func(v types.SecretInput) error { return SetAnthropicApiKey(v, "", storageOpts) },
			applyConfig: nil,
		}, runtime, baseConfig, nextConfig, requestedMode)
	}

	// gemini-api-key
	if authChoice == types.AuthChoiceGeminiAPIKey {
		resolved := resolveApiKeyNI("google", baseConfig, opts.GeminiAPIKey, "--gemini-api-key", "GEMINI_API_KEY", runtime, requestedMode)
		if resolved == nil {
			return nil
		}
		if !maybeSetResolvedApiKeyNI(resolved, func(v types.SecretInput) error { return SetGeminiApiKey(v, "", storageOpts) }, requestedMode, baseConfig, authChoice, runtime) {
			return nil
		}
		nextConfig = ApplyAuthProfileConfig(nextConfig, ApplyProfileConfigParams{ProfileID: "google:default", Provider: "google", Mode: "api_key"})
		return ApplyGoogleGeminiModelDefault(nextConfig).Next
	}

	// openai-api-key
	if authChoice == types.AuthChoiceOpenAIAPIKey {
		return applyNISimple(niSimpleParams{
			provider: "openai", flagValue: opts.OpenAIAPIKey,
			flagName: "--openai-api-key", envVar: "OPENAI_API_KEY",
			profileID:   "openai:default",
			setter:      func(v types.SecretInput) error { return SetOpenaiApiKey(v, "", storageOpts) },
			applyConfig: func(cfg OpenClawConfig) OpenClawConfig { return ApplyOpenAIConfig(cfg) },
		}, runtime, baseConfig, nextConfig, requestedMode)
	}

	// openrouter-api-key
	if authChoice == types.AuthChoiceOpenRouterAPIKey {
		return applyNISimple(niSimpleParams{
			provider: "openrouter", flagValue: opts.OpenRouterAPIKey,
			flagName: "--openrouter-api-key", envVar: "OPENROUTER_API_KEY",
			profileID:   "openrouter:default",
			setter:      func(v types.SecretInput) error { return SetOpenrouterApiKey(v, "", storageOpts) },
			applyConfig: func(cfg OpenClawConfig) OpenClawConfig { return ApplyOpenrouterConfig(cfg) },
		}, runtime, baseConfig, nextConfig, requestedMode)
	}

	// kilocode-api-key
	if authChoice == types.AuthChoiceKilocodeAPIKey {
		return applyNISimple(niSimpleParams{
			provider: "kilocode", flagValue: opts.KilocodeAPIKey,
			flagName: "--kilocode-api-key", envVar: "KILOCODE_API_KEY",
			profileID:   "kilocode:default",
			setter:      func(v types.SecretInput) error { return SetKilocodeApiKey(v, "", storageOpts) },
			applyConfig: func(cfg OpenClawConfig) OpenClawConfig { return ApplyKilocodeConfig(cfg) },
		}, runtime, baseConfig, nextConfig, requestedMode)
	}

	// litellm-api-key
	if authChoice == types.AuthChoiceLiteLLMAPIKey {
		return applyNISimple(niSimpleParams{
			provider: "litellm", flagValue: opts.LiteLLMAPIKey,
			flagName: "--litellm-api-key", envVar: "LITELLM_API_KEY",
			profileID:   "litellm:default",
			setter:      func(v types.SecretInput) error { return SetLitellmApiKey(v, "", storageOpts) },
			applyConfig: func(cfg OpenClawConfig) OpenClawConfig { return ApplyLitellmConfig(cfg) },
		}, runtime, baseConfig, nextConfig, requestedMode)
	}

	// ai-gateway-api-key
	if authChoice == types.AuthChoiceAIGatewayAPIKey {
		return applyNISimple(niSimpleParams{
			provider: "vercel-ai-gateway", flagValue: opts.AIGatewayAPIKey,
			flagName: "--ai-gateway-api-key", envVar: "AI_GATEWAY_API_KEY",
			profileID:   "vercel-ai-gateway:default",
			setter:      func(v types.SecretInput) error { return SetVercelAiGatewayApiKey(v, "", storageOpts) },
			applyConfig: func(cfg OpenClawConfig) OpenClawConfig { return ApplyVercelAiGatewayConfig(cfg) },
		}, runtime, baseConfig, nextConfig, requestedMode)
	}

	// mistral-api-key
	if authChoice == types.AuthChoiceMistralAPIKey {
		return applyNISimple(niSimpleParams{
			provider: "mistral", flagValue: opts.MistralAPIKey,
			flagName: "--mistral-api-key", envVar: "MISTRAL_API_KEY",
			profileID:   "mistral:default",
			setter:      func(v types.SecretInput) error { return SetMistralApiKey(v, "", storageOpts) },
			applyConfig: func(cfg OpenClawConfig) OpenClawConfig { return ApplyMistralConfig(cfg) },
		}, runtime, baseConfig, nextConfig, requestedMode)
	}

	// xai-api-key
	if authChoice == types.AuthChoiceXAIAPIKey {
		return applyNISimple(niSimpleParams{
			provider: "xai", flagValue: opts.XAIAPIKey,
			flagName: "--xai-api-key", envVar: "XAI_API_KEY",
			profileID:   "xai:default",
			setter:      func(v types.SecretInput) error { return SetXaiApiKey(v, "", storageOpts) },
			applyConfig: func(cfg OpenClawConfig) OpenClawConfig { return ApplyXaiConfig(cfg) },
		}, runtime, baseConfig, nextConfig, requestedMode)
	}

	// xiaomi-api-key
	if authChoice == types.AuthChoiceXiaomiAPIKey {
		return applyNISimple(niSimpleParams{
			provider: "xiaomi", flagValue: opts.XiaomiAPIKey,
			flagName: "--xiaomi-api-key", envVar: "XIAOMI_API_KEY",
			profileID:   "xiaomi:default",
			setter:      func(v types.SecretInput) error { return SetXiaomiApiKey(v, "", storageOpts) },
			applyConfig: func(cfg OpenClawConfig) OpenClawConfig { return ApplyXiaomiConfig(cfg) },
		}, runtime, baseConfig, nextConfig, requestedMode)
	}

	// 委托给第二批处理
	return applyNIProvidersExtra(authChoice, opts, runtime, baseConfig, nextConfig, requestedMode, storageOpts)
}
