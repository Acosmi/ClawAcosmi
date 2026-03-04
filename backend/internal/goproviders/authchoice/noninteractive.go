// authchoice/noninteractive.go — 非交互式 auth-choice apply
// 对应 TS 文件: src/commands/onboard-non-interactive/local/auth-choice.ts
// 处理所有非交互模式下的认证选择应用逻辑。
package authchoice

import (
	"fmt"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// ApplyNonInteractiveAuthChoiceParams 非交互式认证选择参数。
type ApplyNonInteractiveAuthChoiceParams struct {
	NextConfig OpenClawConfig
	AuthChoice types.AuthChoice
	Opts       types.OnboardOptions
	Runtime    RuntimeEnv
	BaseConfig OpenClawConfig
}

// toStoredSecretInput 将解析结果转为存储用的 SecretInput。
func toStoredSecretInput(
	resolved *ResolvedNonInteractiveApiKey,
	requestedMode types.SecretInputMode,
	baseConfig OpenClawConfig,
	authChoice types.AuthChoice,
	runtime RuntimeEnv,
) types.SecretInput {
	if requestedMode != types.SecretInputModeRef {
		return resolved.Key
	}
	if resolved.Source != "env" {
		return resolved.Key
	}
	if resolved.EnvVarName == "" {
		runtime.Error(strings.Join([]string{
			fmt.Sprintf("Unable to determine which environment variable to store as a ref for provider %q.", string(authChoice)),
			"Set an explicit provider env var and retry, or use --secret-input-mode plaintext.",
		}, "\n"))
		runtime.Exit(1)
		return nil
	}
	return types.SecretRef{
		Source:   types.SecretRefSourceEnv,
		Provider: ResolveDefaultSecretProviderAlias(baseConfig, "env", nil),
		ID:       resolved.EnvVarName,
	}
}

// resolveApiKeyNI 非交互式 API Key 解析包装。
func resolveApiKeyNI(
	provider string, cfg OpenClawConfig, flagValue *string,
	flagName, envVar string, runtime RuntimeEnv,
	requestedMode types.SecretInputMode,
) *ResolvedNonInteractiveApiKey {
	return ResolveNonInteractiveApiKey(ResolveNonInteractiveApiKeyParams{
		Provider:        provider,
		Cfg:             cfg,
		FlagValue:       flagValue,
		FlagName:        flagName,
		EnvVar:          envVar,
		Runtime:         runtime,
		Required:        true,
		SecretInputMode: requestedMode,
	})
}

// maybeSetResolvedApiKeyNI 设置解析的 API Key（非交互式）。
func maybeSetResolvedApiKeyNI(
	resolved *ResolvedNonInteractiveApiKey,
	setter func(types.SecretInput) error,
	requestedMode types.SecretInputMode,
	baseConfig OpenClawConfig,
	authChoice types.AuthChoice,
	runtime RuntimeEnv,
) bool {
	if resolved.Source == "profile" {
		return true
	}
	stored := toStoredSecretInput(resolved, requestedMode, baseConfig, authChoice, runtime)
	if stored == nil {
		return false
	}
	if err := setter(stored); err != nil {
		runtime.Error(fmt.Sprint(err))
		return false
	}
	return true
}

// ──────────────────────────────────────────────
// 主入口（前半段：deprecated + token + anthropic + gemini）
// ──────────────────────────────────────────────

// ApplyNonInteractiveAuthChoice 非交互式认证选择的主入口。
// 对应 TS: applyNonInteractiveAuthChoice()
func ApplyNonInteractiveAuthChoice(params ApplyNonInteractiveAuthChoiceParams) OpenClawConfig {
	authChoice := params.AuthChoice
	opts := params.Opts
	runtime := params.Runtime
	baseConfig := params.BaseConfig
	nextConfig := params.NextConfig

	requestedSecretInputMode := NormalizeSecretInputModeInput(opts.SecretInputMode)
	if opts.SecretInputMode != nil && requestedSecretInputMode == "" {
		runtime.Error(`Invalid --secret-input-mode. Use "plaintext" or "ref".`)
		runtime.Exit(1)
		return nil
	}
	var apiKeyStorageOpts *ApiKeyStorageOptions
	if requestedSecretInputMode != "" {
		apiKeyStorageOpts = &ApiKeyStorageOptions{SecretInputMode: requestedSecretInputMode}
	}
	_ = apiKeyStorageOpts // 用于后续 setter

	// deprecated 选择检查
	if authChoice == types.AuthChoiceClaudeCLI || authChoice == types.AuthChoiceCodexCLI {
		runtime.Error(strings.Join([]string{
			fmt.Sprintf("Auth choice %q is deprecated.", string(authChoice)),
			`Use "--auth-choice token" (Anthropic setup-token) or "--auth-choice openai-codex".`,
		}, "\n"))
		runtime.Exit(1)
		return nil
	}
	if authChoice == types.AuthChoiceSetupToken {
		runtime.Error(strings.Join([]string{
			`Auth choice "setup-token" requires interactive mode.`,
			`Use "--auth-choice token" with --token and --token-provider anthropic.`,
		}, "\n"))
		runtime.Exit(1)
		return nil
	}
	if authChoice == types.AuthChoiceVLLM {
		runtime.Error(strings.Join([]string{
			`Auth choice "vllm" requires interactive mode.`,
			"Use interactive onboard/configure to enter base URL, API key, and model ID.",
		}, "\n"))
		runtime.Exit(1)
		return nil
	}

	// token 处理
	if authChoice == types.AuthChoiceToken {
		return applyNIToken(opts, runtime, nextConfig, requestedSecretInputMode)
	}

	// 各 provider 处理
	result := applyNIProviders(authChoice, opts, runtime, baseConfig, nextConfig, requestedSecretInputMode)
	return result
}
