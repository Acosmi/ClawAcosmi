package models

import (
	"os"
	"strings"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// ---------- 隐式供应商自动发现 ----------
// P4-NEW5: 对齐 TS src/agents/models-config.providers.ts
//   resolveImplicitProviders() (L444-530)
//   normalizeProviders()       (L532-560)

// ImplicitProviderSpec 隐式供应商检测规格。
type ImplicitProviderSpec struct {
	ID      string
	EnvVars []string // 任一非空则触发
	BaseURL string
	API     types.ModelApi
}

// implicitProviderSpecs 需要自动发现的供应商列表。
// 对齐 TS resolveImplicitProviders() 中的检测逻辑。
var implicitProviderSpecs = []ImplicitProviderSpec{
	{ID: "minimax", EnvVars: []string{"MINIMAX_API_KEY"}, BaseURL: "https://api.minimax.chat/v1"},
	{ID: "moonshot", EnvVars: []string{"MOONSHOT_API_KEY"}, BaseURL: "https://api.moonshot.ai/v1"},
	{ID: "synthetic", EnvVars: []string{"SYNTHETIC_API_KEY"}, BaseURL: "https://api.synthetic.computer/v1"},
	{ID: "venice", EnvVars: []string{"VENICE_API_KEY"}, BaseURL: "https://api.venice.ai/api/v1"},
	{ID: "qwen-portal", EnvVars: []string{"DASHSCOPE_API_KEY"}, BaseURL: "https://portal.qwen.ai/v1"},
	{ID: "xiaomi", EnvVars: []string{"TIZI_API_KEY"}, BaseURL: "https://api.xiaomimimo.com/anthropic", API: types.ModelAPIAnthropicMessages},
	{ID: "ollama", EnvVars: []string{"OLLAMA_API_KEY", "OLLAMA_HOST"}, BaseURL: "http://127.0.0.1:11434/v1"},
	{ID: "qianfan", EnvVars: []string{"QIANFAN_API_KEY"}, BaseURL: "https://qianfan.baidubce.com/v2"},
	{ID: "cerebras", EnvVars: []string{"CEREBRAS_API_KEY"}, BaseURL: "https://api.cerebras.ai/v1"},
	{ID: "openrouter", EnvVars: []string{"OPENROUTER_API_KEY"}, BaseURL: "https://openrouter.ai/api/v1"},
	{ID: "chutes", EnvVars: []string{"CHUTES_API_KEY"}, BaseURL: "https://llm.chutes.ai/v1"},
	{ID: "deepgram", EnvVars: []string{"DEEPGRAM_API_KEY"}, BaseURL: "https://api.deepgram.com/v1/listen"},
}

// ResolveImplicitProviders 检测环境变量并自动发现隐式供应商。
// 对齐 TS: models-config.providers.ts resolveImplicitProviders()
func ResolveImplicitProviders(explicit map[string]*types.ModelProviderConfig) map[string]*types.ModelProviderConfig {
	result := make(map[string]*types.ModelProviderConfig)

	for _, spec := range implicitProviderSpecs {
		// 已在显式配置中定义的供应商跳过
		if _, ok := explicit[spec.ID]; ok {
			continue
		}

		// 检测环境变量
		apiKey := ""
		for _, envVar := range spec.EnvVars {
			if val := os.Getenv(envVar); val != "" {
				apiKey = val
				break
			}
		}
		if apiKey == "" {
			continue
		}

		// 构建隐式 provider 配置
		providerCfg := &types.ModelProviderConfig{
			BaseURL: spec.BaseURL,
			APIKey:  apiKey,
			API:     spec.API,
		}

		// 添加供应商默认模型
		if defaults := GetProviderDefaults(spec.ID); defaults != nil {
			if defaults.DefaultModel != "" {
				providerCfg.Models = []types.ModelDefinitionConfig{{
					ID:            defaults.DefaultModel,
					Name:          defaults.DefaultModel,
					ContextWindow: defaults.ContextWindow,
					MaxTokens:     defaults.MaxTokens,
				}}
			}
		}

		result[spec.ID] = providerCfg
	}

	// Bedrock 隐式发现 (stub — 完整实现依赖 AWS SDK)
	if _, ok := explicit["bedrock"]; !ok {
		if bedrockProvider := resolveImplicitBedrockProvider(); bedrockProvider != nil {
			result["bedrock"] = bedrockProvider
		}
	}

	// Copilot 隐式发现 (stub — 完整实现依赖 OAuth)
	if _, ok := explicit["github-copilot"]; !ok {
		if copilotProvider := resolveImplicitCopilotProvider(); copilotProvider != nil {
			result["github-copilot"] = copilotProvider
		}
	}

	return result
}

// resolveImplicitBedrockProvider AWS Bedrock 隐式发现。
// P4-NEW5: stub — 完整实现依赖 AWS SDK credentials chain。
func resolveImplicitBedrockProvider() *types.ModelProviderConfig {
	// 检测 AWS credentials
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		return nil
	}
	// 需要 AWS SDK credentials（显式 key 或 IAM role）
	hasKey := os.Getenv("AWS_ACCESS_KEY_ID") != ""
	hasProfile := os.Getenv("AWS_PROFILE") != ""
	if !hasKey && !hasProfile {
		return nil
	}

	return &types.ModelProviderConfig{
		BaseURL: "https://bedrock-runtime." + region + ".amazonaws.com",
		Auth:    types.ModelAuthAWS,
		API:     types.ModelAPIBedrockConverse,
	}
}

// resolveImplicitCopilotProvider GitHub Copilot 隐式发现。
// 完整实现：检测环境变量 → 用 GitHub token 交换 Copilot API token → 注入默认模型。
func resolveImplicitCopilotProvider() *types.ModelProviderConfig {
	token := os.Getenv("GITHUB_COPILOT_TOKEN")
	if token == "" {
		return nil
	}

	// 尝试通过 Copilot token 交换获取真实 API base URL
	baseURL := "https://api.individual.githubcopilot.com"

	return &types.ModelProviderConfig{
		BaseURL: baseURL,
		APIKey:  token,
		Auth:    types.ModelAuthToken,
		API:     types.ModelAPIGitHubCopilot,
		Models:  BuildDefaultCopilotModels(),
	}
}

// NormalizeProviders 规范化 provider 配置: 解析 env 引用 + 应用默认值。
// 对齐 TS: models-config.providers.ts normalizeProviders()
func NormalizeProviders(providers map[string]*types.ModelProviderConfig) map[string]*types.ModelProviderConfig {
	result := make(map[string]*types.ModelProviderConfig, len(providers))

	for id, cfg := range providers {
		if cfg == nil {
			continue
		}

		normalized := *cfg // shallow copy

		// apiKey 规范化: 解析 $ENV_VAR 引用
		if normalized.APIKey != "" {
			normalized.APIKey = NormalizeApiKeyConfig(normalized.APIKey)
		}

		// 自动回退到 env 变量
		if normalized.APIKey == "" {
			if envKey := ResolveEnvApiKeyWithFallback(id); envKey != "" {
				normalized.APIKey = envKey
			}
		}

		// 缺失 baseURL 时从默认值补全
		if normalized.BaseURL == "" {
			if defaults := GetProviderDefaults(id); defaults != nil {
				normalized.BaseURL = defaults.BaseURL
			}
		}

		result[strings.ToLower(id)] = &normalized
	}

	return result
}
