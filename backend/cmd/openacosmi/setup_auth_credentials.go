package main

// setup_auth_credentials.go — 凭证存储 + 模型定义
// TS 对照: onboard-auth.credentials.ts (231L) + onboard-auth.models.ts (123L)
//
// 提供 set*ApiKey 系列函数（统一 helper 模式写入 auth profile）
// 以及 buildModel* 系列函数（模型定义工厂）。

import (
	"fmt"
	"strings"

	"github.com/openacosmi/claw-acismi/internal/agents/auth"
	"github.com/openacosmi/claw-acismi/internal/tui"
	"github.com/openacosmi/claw-acismi/pkg/i18n"
	"github.com/openacosmi/claw-acismi/pkg/types"
)

// ---------- 内部 helper ----------

// upsertApiKeyProfile 通用的 API key profile 写入。
// 对应 TS: upsertAuthProfile({profileId, credential:{type:"api_key", provider, key}})
func upsertApiKeyProfile(store *auth.AuthStore, provider, key string) error {
	profileID := provider + ":default"
	if store == nil {
		return nil
	}
	_, err := store.Update(func(s *auth.AuthProfileStore) bool {
		s.Profiles[profileID] = &auth.AuthProfileCredential{
			Type:     auth.CredentialAPIKey,
			Provider: provider,
			Key:      key,
		}
		return true
	})
	return err
}

// upsertOAuthProfile 写入 OAuth 凭证。
// 对应 TS: writeOAuthCredentials(provider, creds, agentDir)
func upsertOAuthProfile(store *auth.AuthStore, provider, email, token, refreshToken string) error {
	emailNormalized := strings.TrimSpace(email)
	if emailNormalized == "" {
		emailNormalized = "default"
	}
	profileID := provider + ":" + emailNormalized
	if store == nil {
		return nil
	}
	_, err := store.Update(func(s *auth.AuthProfileStore) bool {
		s.Profiles[profileID] = &auth.AuthProfileCredential{
			Type:     auth.CredentialOAuth,
			Provider: provider,
			Key:      token,
			Email:    emailNormalized,
			Metadata: map[string]string{
				"refreshToken": refreshToken,
			},
		}
		return true
	})
	return err
}

// ---------- set*ApiKey 系列（17 个 provider） ----------

// SetAnthropicApiKey 写入 Anthropic API key。
func SetAnthropicApiKey(store *auth.AuthStore, key string) error {
	return upsertApiKeyProfile(store, "anthropic", key)
}

// SetGeminiApiKey 写入 Google/Gemini API key。
func SetGeminiApiKey(store *auth.AuthStore, key string) error {
	return upsertApiKeyProfile(store, "google", key)
}

// SetMinimaxApiKey 写入 Minimax API key。
func SetMinimaxApiKey(store *auth.AuthStore, key string) error {
	return upsertApiKeyProfile(store, "minimax", key)
}

// SetMoonshotApiKey 写入 Moonshot API key。
func SetMoonshotApiKey(store *auth.AuthStore, key string) error {
	return upsertApiKeyProfile(store, "moonshot", key)
}

// SetKimiCodingApiKey 写入 Kimi Coding API key。
func SetKimiCodingApiKey(store *auth.AuthStore, key string) error {
	return upsertApiKeyProfile(store, "kimi-coding", key)
}

// SetSyntheticApiKey 写入 Synthetic/MiniMax hosted API key。
func SetSyntheticApiKey(store *auth.AuthStore, key string) error {
	return upsertApiKeyProfile(store, "synthetic", key)
}

// SetVeniceApiKey 写入 Venice API key。
func SetVeniceApiKey(store *auth.AuthStore, key string) error {
	return upsertApiKeyProfile(store, "venice", key)
}

// SetZaiApiKey 写入 ZAI/GLM API key。
func SetZaiApiKey(store *auth.AuthStore, key string) error {
	return upsertApiKeyProfile(store, "zai", key)
}

// SetXiaomiApiKey 写入 Xiaomi API key。
func SetXiaomiApiKey(store *auth.AuthStore, key string) error {
	return upsertApiKeyProfile(store, "xiaomi", key)
}

// SetOpenrouterApiKey 写入 OpenRouter API key。
func SetOpenrouterApiKey(store *auth.AuthStore, key string) error {
	return upsertApiKeyProfile(store, "openrouter", key)
}

// SetVercelAiGatewayApiKey 写入 Vercel AI Gateway API key。
func SetVercelAiGatewayApiKey(store *auth.AuthStore, key string) error {
	return upsertApiKeyProfile(store, "vercel-ai-gateway", key)
}

// SetAcosmiZenApiKey 写入 OpenAcosmi Zen API key。
func SetAcosmiZenApiKey(store *auth.AuthStore, key string) error {
	return upsertApiKeyProfile(store, "openacosmi", key)
}

// SetQianfanApiKey 写入百度千帆 API key。
func SetQianfanApiKey(store *auth.AuthStore, key string) error {
	return upsertApiKeyProfile(store, "qianfan", key)
}

// SetXaiApiKey 写入 xAI/Grok API key。
func SetXaiApiKey(store *auth.AuthStore, key string) error {
	return upsertApiKeyProfile(store, "xai", key)
}

// SetOpenAIApiKey 写入 OpenAI API key。
func SetOpenAIApiKey(store *auth.AuthStore, key string) error {
	return upsertApiKeyProfile(store, "openai", key)
}

// SetCloudflareAiGatewayCredential 写入 Cloudflare AI Gateway 凭证（含 account/gateway metadata）。
// 对应 TS: setCloudflareAiGatewayConfig(accountId, gatewayId, apiKey, agentDir)
func SetCloudflareAiGatewayCredential(store *auth.AuthStore, accountID, gatewayID, apiKey string) error {
	normalizedAccount := strings.TrimSpace(accountID)
	normalizedGateway := strings.TrimSpace(gatewayID)
	normalizedKey := strings.TrimSpace(apiKey)
	if store == nil {
		return nil
	}
	_, err := store.Update(func(s *auth.AuthProfileStore) bool {
		s.Profiles["cloudflare-ai-gateway:default"] = &auth.AuthProfileCredential{
			Type:     auth.CredentialAPIKey,
			Provider: "cloudflare-ai-gateway",
			Key:      normalizedKey,
			Metadata: map[string]string{
				"accountId": normalizedAccount,
				"gatewayId": normalizedGateway,
			},
		}
		return true
	})
	return err
}

// WriteOAuthCredentials 写入 OAuth 凭证到 profile store。
// 对应 TS: writeOAuthCredentials(provider, creds, agentDir)
func WriteOAuthCredentials(store *auth.AuthStore, provider, email, token, refreshToken string) error {
	return upsertOAuthProfile(store, provider, email, token, refreshToken)
}

// ---------- 模型定义常量 ----------

// Minimax 常量
const (
	DefaultMinimaxBaseURL   = "https://api.minimax.io/v1"
	MinimaxAPIBaseURL       = "https://api.minimax.io/anthropic"
	MinimaxHostedModelID    = "MiniMax-M2.1"
	MinimaxHostedModelRef   = "minimax/MiniMax-M2.1"
	DefaultMinimaxCtxWindow = 200000
	DefaultMinimaxMaxTokens = 8192
)

// Moonshot 常量
const (
	MoonshotBaseURL         = "https://api.moonshot.ai/v1"
	MoonshotCnBaseURL       = "https://api.moonshot.cn/v1"
	MoonshotDefaultModelID  = "kimi-k2.5"
	MoonshotDefaultModelRef = "moonshot/kimi-k2.5"
	MoonshotDefaultCtxWin   = 256000
	MoonshotDefaultMaxTok   = 8192
	KimiCodingModelID       = "k2p5"
	KimiCodingModelRef      = "kimi-coding/k2p5"
)

// xAI 常量
const (
	XaiBaseURL         = "https://api.x.ai/v1"
	XaiDefaultModelID  = "grok-4"
	XaiDefaultModelRef = "xai/grok-4"
	XaiDefaultCtxWin   = 131072
	XaiDefaultMaxTok   = 8192
)

// 其他 provider 默认模型引用
const (
	ZaiDefaultModelRef              = "zai/glm-4.7"
	XiaomiDefaultModelRef           = "xiaomi/mimo-v2-flash"
	OpenrouterDefaultModelRef       = "openrouter/auto"
	VercelAiGatewayDefaultModelRef  = "vercel-ai-gateway/anthropic/claude-opus-4.6"
	QianfanDefaultModelRef          = "qianfan/ernie-4.5-8k"
	CloudflareAiGatewayDefaultModel = "cloudflare-ai-gateway/claude-4-sonnet"
)

// ---------- buildModel* 工厂函数 ----------

// BuildMinimaxModelDefinition 构建 Minimax 模型定义。
func BuildMinimaxModelDefinition(id, name string, reasoning bool, cost types.ModelCostConfig, ctxWindow, maxTokens int) types.ModelDefinitionConfig {
	displayName := name
	if displayName == "" {
		displayName = "MiniMax " + id
	}
	return types.ModelDefinitionConfig{
		ID:            id,
		Name:          displayName,
		Reasoning:     reasoning,
		ContextWindow: ctxWindow,
		MaxTokens:     maxTokens,
		Cost:          cost,
	}
}

// BuildMinimaxApiModelDefinition 从模型 ID 构建标准 Minimax API 模型定义。
func BuildMinimaxApiModelDefinition(modelID string) types.ModelDefinitionConfig {
	return BuildMinimaxModelDefinition(modelID, "", false, types.ModelCostConfig{
		Input:      15,
		Output:     60,
		CacheRead:  2,
		CacheWrite: 10,
	}, DefaultMinimaxCtxWindow, DefaultMinimaxMaxTokens)
}

// BuildMoonshotModelDefinition 构建 Moonshot 模型定义。
func BuildMoonshotModelDefinition() types.ModelDefinitionConfig {
	return types.ModelDefinitionConfig{
		ID:            MoonshotDefaultModelID,
		Name:          "Kimi K2.5",
		Reasoning:     false,
		ContextWindow: MoonshotDefaultCtxWin,
		MaxTokens:     MoonshotDefaultMaxTok,
		Cost: types.ModelCostConfig{
			Input:      0,
			Output:     0,
			CacheRead:  0,
			CacheWrite: 0,
		},
	}
}

// BuildXaiModelDefinition 构建 xAI/Grok 模型定义。
func BuildXaiModelDefinition() types.ModelDefinitionConfig {
	return types.ModelDefinitionConfig{
		ID:            XaiDefaultModelID,
		Name:          "Grok 4",
		Reasoning:     false,
		ContextWindow: XaiDefaultCtxWin,
		MaxTokens:     XaiDefaultMaxTok,
		Cost: types.ModelCostConfig{
			Input:      0,
			Output:     0,
			CacheRead:  0,
			CacheWrite: 0,
		},
	}
}

// ---------- 模型目录 ----------

// ModelCatalogEntry 模型目录条目。
type ModelCatalogEntry struct {
	Ref      string // provider/model-id 格式
	Label    string // 显示名
	Default  bool   // 是否推荐默认
	Provider string
}

// BuildProviderModelCatalog 按 provider 返回可用模型目录。
// 对应 TS: onboard-auth.models.ts 的模型常量 + config-core 的模型选择逻辑。
func BuildProviderModelCatalog(provider string) []ModelCatalogEntry {
	switch provider {
	case "anthropic":
		return []ModelCatalogEntry{
			{Ref: "anthropic/claude-opus-4-6", Label: "Claude Opus 4.6", Default: true, Provider: "anthropic"},
			{Ref: "anthropic/claude-sonnet-4-6", Label: "Claude Sonnet 4.6", Provider: "anthropic"},
			{Ref: "anthropic/claude-haiku-3-5", Label: "Claude Haiku 3.5", Provider: "anthropic"},
		}
	case "openai":
		return []ModelCatalogEntry{
			{Ref: "openai/o3", Label: "o3", Default: true, Provider: "openai"},
			{Ref: "openai/gpt-4.1", Label: "GPT-4.1", Provider: "openai"},
			{Ref: "openai/o4-mini", Label: "o4-mini", Provider: "openai"},
		}
	case "google":
		return []ModelCatalogEntry{
			{Ref: "google/gemini-2.5-pro", Label: "Gemini 2.5 Pro", Default: true, Provider: "google"},
			{Ref: "google/gemini-2.5-flash", Label: "Gemini 2.5 Flash", Provider: "google"},
		}
	case "moonshot":
		return []ModelCatalogEntry{
			{Ref: MoonshotDefaultModelRef, Label: "Kimi K2.5", Default: true, Provider: "moonshot"},
		}
	case "minimax":
		return []ModelCatalogEntry{
			{Ref: MinimaxHostedModelRef, Label: "MiniMax M2.1", Default: true, Provider: "minimax"},
			{Ref: "minimax/MiniMax-M2.1-lightning", Label: "MiniMax M2.1 Lightning", Provider: "minimax"},
		}
	case "xai":
		return []ModelCatalogEntry{
			{Ref: XaiDefaultModelRef, Label: "Grok 4", Default: true, Provider: "xai"},
		}
	case "zai":
		return []ModelCatalogEntry{
			{Ref: ZaiDefaultModelRef, Label: "GLM 4.7", Default: true, Provider: "zai"},
		}
	case "openrouter":
		return []ModelCatalogEntry{
			{Ref: OpenrouterDefaultModelRef, Label: "Auto", Default: true, Provider: "openrouter"},
		}
	case "qianfan":
		return []ModelCatalogEntry{
			{Ref: QianfanDefaultModelRef, Label: "ERNIE 4.5 8K", Default: true, Provider: "qianfan"},
		}
	default:
		return nil
	}
}

// PickDefaultModel 交互式模型选择。
// 对应 TS: onboard-auth.models.ts 的 pickDefaultModel 逻辑。
// 如果 provider 只有一个模型则自动选择，否则通过 prompter 交互。
func PickDefaultModel(prompter tui.WizardPrompter, provider string) (string, error) {
	catalog := BuildProviderModelCatalog(provider)
	if len(catalog) == 0 {
		return "", nil
	}

	// 单模型 → 自动选择
	if len(catalog) == 1 {
		return catalog[0].Ref, nil
	}

	// 构建选项
	options := make([]tui.PromptOption, 0, len(catalog))
	var initialValue string
	for _, entry := range catalog {
		hint := ""
		if entry.Default {
			hint = "recommended"
			if initialValue == "" {
				initialValue = entry.Ref
			}
		}
		options = append(options, tui.PromptOption{
			Value: entry.Ref,
			Label: entry.Label,
			Hint:  hint,
		})
	}

	selected, err := prompter.Select(
		i18n.Tp("onboard.auth.cred_select"),
		options,
		initialValue,
	)
	if err != nil {
		return "", fmt.Errorf("model selection: %w", err)
	}
	return selected, nil
}
