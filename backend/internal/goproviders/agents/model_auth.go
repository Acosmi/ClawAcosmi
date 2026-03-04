// agents/model_auth.go — 模型级认证
// 对应 TS 文件: src/agents/model-auth.ts
package agents

import (
	"fmt"
	"os"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/authprofile"
	"github.com/Acosmi/ClawAcosmi/internal/goproviders/common"
	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// AWS 环境变量常量
const (
	awsBearerEnv    = "AWS_BEARER_TOKEN_BEDROCK"
	awsAccessKeyEnv = "AWS_ACCESS_KEY_ID"
	awsSecretKeyEnv = "AWS_SECRET_ACCESS_KEY"
	awsProfileEnv   = "AWS_PROFILE"
)

// ResolvedProviderAuth 解析后的 Provider 认证信息。
type ResolvedProviderAuth struct {
	ApiKey    string `json:"apiKey,omitempty"`
	ProfileId string `json:"profileId,omitempty"`
	Source    string `json:"source"`
	Mode      string `json:"mode"` // "api-key" | "oauth" | "token" | "aws-sdk"
}

// EnvApiKeyResult 环境变量 API 密钥结果。
type EnvApiKeyResult struct {
	ApiKey string
	Source string
}

// ModelAuthMode 模型认证模式。
type ModelAuthMode string

const (
	ModelAuthModeApiKey  ModelAuthMode = "api-key"
	ModelAuthModeOAuth   ModelAuthMode = "oauth"
	ModelAuthModeToken   ModelAuthMode = "token"
	ModelAuthModeMixed   ModelAuthMode = "mixed"
	ModelAuthModeAwsSdk  ModelAuthMode = "aws-sdk"
	ModelAuthModeUnknown ModelAuthMode = "unknown"
)

// resolveProviderConfig 解析 Provider 配置。
func resolveProviderConfig(cfg *authprofile.OpenClawConfig, provider string) *authprofile.ModelProviderConfig {
	if cfg == nil || cfg.Models == nil || cfg.Models.Providers == nil {
		return nil
	}
	if direct, ok := cfg.Models.Providers[provider]; ok {
		return direct
	}
	normalized := common.NormalizeProviderId(provider)
	if normalized == provider {
		for key, config := range cfg.Models.Providers {
			if common.NormalizeProviderId(key) == normalized {
				return config
			}
		}
		return nil
	}
	if config, ok := cfg.Models.Providers[normalized]; ok {
		return config
	}
	for key, config := range cfg.Models.Providers {
		if common.NormalizeProviderId(key) == normalized {
			return config
		}
	}
	return nil
}

// GetCustomProviderApiKey 获取自定义 Provider 的 API 密钥。
// 对应 TS: getCustomProviderApiKey()
func GetCustomProviderApiKey(cfg *authprofile.OpenClawConfig, provider string) string {
	entry := resolveProviderConfig(cfg, provider)
	if entry == nil {
		return ""
	}
	return strings.TrimSpace(entry.ApiKey)
}

// resolveProviderAuthOverride 获取 Provider 认证模式覆盖。
func resolveProviderAuthOverride(cfg *authprofile.OpenClawConfig, provider string) string {
	entry := resolveProviderConfig(cfg, provider)
	if entry == nil {
		return ""
	}
	switch entry.Auth {
	case "api-key", "aws-sdk", "oauth", "token":
		return entry.Auth
	default:
		return ""
	}
}

// resolveAwsSdkAuthInfo 解析 AWS SDK 认证信息。
func resolveAwsSdkAuthInfo() ResolvedProviderAuth {
	if strings.TrimSpace(os.Getenv(awsBearerEnv)) != "" {
		return ResolvedProviderAuth{Mode: "aws-sdk", Source: "env: " + awsBearerEnv}
	}
	if strings.TrimSpace(os.Getenv(awsAccessKeyEnv)) != "" && strings.TrimSpace(os.Getenv(awsSecretKeyEnv)) != "" {
		return ResolvedProviderAuth{Mode: "aws-sdk", Source: fmt.Sprintf("env: %s + %s", awsAccessKeyEnv, awsSecretKeyEnv)}
	}
	if strings.TrimSpace(os.Getenv(awsProfileEnv)) != "" {
		return ResolvedProviderAuth{Mode: "aws-sdk", Source: "env: " + awsProfileEnv}
	}
	return ResolvedProviderAuth{Mode: "aws-sdk", Source: "aws-sdk default chain"}
}

// ResolveAwsSdkEnvVarName 解析 AWS SDK 使用的环境变量名。
// 对应 TS: resolveAwsSdkEnvVarName()
func ResolveAwsSdkEnvVarName() string {
	if strings.TrimSpace(os.Getenv(awsBearerEnv)) != "" {
		return awsBearerEnv
	}
	if strings.TrimSpace(os.Getenv(awsAccessKeyEnv)) != "" && strings.TrimSpace(os.Getenv(awsSecretKeyEnv)) != "" {
		return awsAccessKeyEnv
	}
	if strings.TrimSpace(os.Getenv(awsProfileEnv)) != "" {
		return awsProfileEnv
	}
	return ""
}

// ResolveEnvApiKey 通过环境变量解析 API 密钥。
// 对应 TS: resolveEnvApiKey()
func ResolveEnvApiKey(provider string) *EnvApiKeyResult {
	normalized := common.NormalizeProviderId(provider)

	pick := func(envVar string) *EnvApiKeyResult {
		value := strings.TrimSpace(os.Getenv(envVar))
		if value == "" {
			return nil
		}
		return &EnvApiKeyResult{ApiKey: value, Source: "env: " + envVar}
	}

	switch normalized {
	case "github-copilot":
		r := pick("COPILOT_GITHUB_TOKEN")
		if r != nil {
			return r
		}
		r = pick("GH_TOKEN")
		if r != nil {
			return r
		}
		return pick("GITHUB_TOKEN")
	case "anthropic":
		r := pick("ANTHROPIC_OAUTH_TOKEN")
		if r != nil {
			return r
		}
		return pick("ANTHROPIC_API_KEY")
	case "chutes":
		r := pick("CHUTES_OAUTH_TOKEN")
		if r != nil {
			return r
		}
		return pick("CHUTES_API_KEY")
	case "zai":
		r := pick("ZAI_API_KEY")
		if r != nil {
			return r
		}
		return pick("Z_AI_API_KEY")
	case "opencode":
		r := pick("OPENCODE_API_KEY")
		if r != nil {
			return r
		}
		return pick("OPENCODE_ZEN_API_KEY")
	case "qwen-portal":
		r := pick("QWEN_OAUTH_TOKEN")
		if r != nil {
			return r
		}
		return pick("QWEN_PORTAL_API_KEY")
	case "volcengine", "volcengine-plan":
		return pick("VOLCANO_ENGINE_API_KEY")
	case "byteplus", "byteplus-plan":
		return pick("BYTEPLUS_API_KEY")
	case "minimax-portal":
		r := pick("MINIMAX_OAUTH_TOKEN")
		if r != nil {
			return r
		}
		return pick("MINIMAX_API_KEY")
	case "kimi-coding":
		r := pick("KIMI_API_KEY")
		if r != nil {
			return r
		}
		return pick("KIMICODE_API_KEY")
	case "huggingface":
		r := pick("HUGGINGFACE_HUB_TOKEN")
		if r != nil {
			return r
		}
		return pick("HF_TOKEN")
	}

	envMap := map[string]string{
		"openai": "OPENAI_API_KEY", "google": "GEMINI_API_KEY",
		"voyage": "VOYAGE_API_KEY", "groq": "GROQ_API_KEY",
		"deepgram": "DEEPGRAM_API_KEY", "cerebras": "CEREBRAS_API_KEY",
		"xai": "XAI_API_KEY", "openrouter": "OPENROUTER_API_KEY",
		"litellm": "LITELLM_API_KEY", "vercel-ai-gateway": "AI_GATEWAY_API_KEY",
		"cloudflare-ai-gateway": "CLOUDFLARE_AI_GATEWAY_API_KEY",
		"moonshot":              "MOONSHOT_API_KEY", "minimax": "MINIMAX_API_KEY",
		"nvidia": "NVIDIA_API_KEY", "xiaomi": "XIAOMI_API_KEY",
		"synthetic": "SYNTHETIC_API_KEY", "venice": "VENICE_API_KEY",
		"mistral": "MISTRAL_API_KEY", "together": "TOGETHER_API_KEY",
		"qianfan": "QIANFAN_API_KEY", "ollama": "OLLAMA_API_KEY",
		"vllm": "VLLM_API_KEY", "kilocode": "KILOCODE_API_KEY",
	}
	if envVar, ok := envMap[normalized]; ok {
		return pick(envVar)
	}
	return nil
}

// ResolveApiKeyForProvider 解析指定 Provider 的 API 密钥。
// 对应 TS: resolveApiKeyForProvider()
func ResolveApiKeyForProvider(
	provider string,
	cfg *authprofile.OpenClawConfig,
	profileId string,
	preferredProfile string,
	store *types.AuthProfileStore,
	agentDir string,
	cliReaders map[string]authprofile.ExternalCliCredentialReader,
	refresher authprofile.OAuthTokenRefresher,
) (*ResolvedProviderAuth, error) {
	if store == nil {
		store = authprofile.EnsureAuthProfileStore(agentDir, cliReaders)
	}

	// 指定 Profile
	if profileId != "" {
		resolved, err := authprofile.ResolveApiKeyForProfile(cfg, store, profileId, agentDir, cliReaders, refresher)
		if err != nil {
			return nil, err
		}
		if resolved == nil {
			return nil, fmt.Errorf("No credentials found for profile \"%s\".", profileId)
		}
		cred := store.Profiles[profileId]
		credType, _ := cred["type"].(string)
		mode := "api-key"
		if credType == "oauth" {
			mode = "oauth"
		} else if credType == "token" {
			mode = "token"
		}
		return &ResolvedProviderAuth{
			ApiKey: resolved.ApiKey, ProfileId: profileId,
			Source: "profile:" + profileId, Mode: mode,
		}, nil
	}

	// AWS SDK 覆盖
	authOverride := resolveProviderAuthOverride(cfg, provider)
	if authOverride == "aws-sdk" {
		result := resolveAwsSdkAuthInfo()
		return &result, nil
	}

	// 按顺序尝试所有 Profile
	order := authprofile.ResolveAuthProfileOrder(cfg, store, provider, preferredProfile)
	for _, candidate := range order {
		resolved, err := authprofile.ResolveApiKeyForProfile(cfg, store, candidate, agentDir, cliReaders, refresher)
		if err != nil {
			continue
		}
		if resolved != nil {
			cred := store.Profiles[candidate]
			credType, _ := cred["type"].(string)
			mode := "api-key"
			if credType == "oauth" {
				mode = "oauth"
			} else if credType == "token" {
				mode = "token"
			}
			return &ResolvedProviderAuth{
				ApiKey: resolved.ApiKey, ProfileId: candidate,
				Source: "profile:" + candidate, Mode: mode,
			}, nil
		}
	}

	// 尝试环境变量
	envResolved := ResolveEnvApiKey(provider)
	if envResolved != nil {
		mode := "api-key"
		if strings.Contains(envResolved.Source, "OAUTH_TOKEN") {
			mode = "oauth"
		}
		return &ResolvedProviderAuth{
			ApiKey: envResolved.ApiKey, Source: envResolved.Source, Mode: mode,
		}, nil
	}

	// 尝试自定义 Provider 密钥
	customKey := GetCustomProviderApiKey(cfg, provider)
	if customKey != "" {
		return &ResolvedProviderAuth{ApiKey: customKey, Source: "models.json", Mode: "api-key"}, nil
	}

	// AWS SDK 回退
	normalized := common.NormalizeProviderId(provider)
	if authOverride == "" && normalized == "amazon-bedrock" {
		result := resolveAwsSdkAuthInfo()
		return &result, nil
	}

	return nil, fmt.Errorf("No API key found for provider \"%s\".", provider)
}

// ResolveModelAuthMode 解析模型认证模式。
// 对应 TS: resolveModelAuthMode()
func ResolveModelAuthMode(
	provider string,
	cfg *authprofile.OpenClawConfig,
	store *types.AuthProfileStore,
	cliReaders map[string]authprofile.ExternalCliCredentialReader,
) ModelAuthMode {
	resolved := strings.TrimSpace(provider)
	if resolved == "" {
		return ""
	}

	authOverride := resolveProviderAuthOverride(cfg, resolved)
	if authOverride == "aws-sdk" {
		return ModelAuthModeAwsSdk
	}

	if store == nil {
		store = authprofile.EnsureAuthProfileStore("", cliReaders)
	}
	profiles := authprofile.ListProfilesForProvider(store, resolved)
	if len(profiles) > 0 {
		modes := make(map[string]bool)
		for _, id := range profiles {
			cred := store.Profiles[id]
			credType, _ := cred["type"].(string)
			if credType != "" {
				modes[credType] = true
			}
		}
		modeCount := 0
		for _, m := range []string{"oauth", "token", "api_key"} {
			if modes[m] {
				modeCount++
			}
		}
		if modeCount >= 2 {
			return ModelAuthModeMixed
		}
		if modes["oauth"] {
			return ModelAuthModeOAuth
		}
		if modes["token"] {
			return ModelAuthModeToken
		}
		if modes["api_key"] {
			return ModelAuthModeApiKey
		}
	}

	if authOverride == "" && common.NormalizeProviderId(resolved) == "amazon-bedrock" {
		return ModelAuthModeAwsSdk
	}

	envKey := ResolveEnvApiKey(resolved)
	if envKey != nil {
		if strings.Contains(envKey.Source, "OAUTH_TOKEN") {
			return ModelAuthModeOAuth
		}
		return ModelAuthModeApiKey
	}

	if GetCustomProviderApiKey(cfg, resolved) != "" {
		return ModelAuthModeApiKey
	}

	return ModelAuthModeUnknown
}

// RequireApiKey 确保 API 密钥存在。
// 对应 TS: requireApiKey()
func RequireApiKey(auth *ResolvedProviderAuth, provider string) (string, error) {
	key := strings.TrimSpace(auth.ApiKey)
	if key != "" {
		return key, nil
	}
	return "", fmt.Errorf("No API key resolved for provider \"%s\" (auth mode: %s).", provider, auth.Mode)
}
