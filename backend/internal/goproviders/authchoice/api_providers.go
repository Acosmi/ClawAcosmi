// authchoice/api_providers.go — API Provider 分发总线
// 对应 TS 文件: src/commands/auth-choice.apply.api-providers.ts
// 处理所有纯 API Key 类 Provider 的 auth-choice apply 逻辑。
package authchoice

import (
	"fmt"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// apiKeyTokenProviderAuthChoice Token Provider 到 AuthChoice 的映射。
// 当 authChoice=apiKey 且指定了 tokenProvider 时用作路由表。
var apiKeyTokenProviderAuthChoice = map[string]types.AuthChoice{
	"openrouter":            types.AuthChoiceOpenRouterAPIKey,
	"litellm":               types.AuthChoiceLiteLLMAPIKey,
	"vercel-ai-gateway":     types.AuthChoiceAIGatewayAPIKey,
	"cloudflare-ai-gateway": types.AuthChoiceCloudflareAIGatewayKey,
	"moonshot":              types.AuthChoiceMoonshotAPIKey,
	"kimi-code":             types.AuthChoiceKimiCodeAPIKey,
	"kimi-coding":           types.AuthChoiceKimiCodeAPIKey,
	"google":                types.AuthChoiceGeminiAPIKey,
	"zai":                   types.AuthChoiceZAIAPIKey,
	"xiaomi":                types.AuthChoiceXiaomiAPIKey,
	"synthetic":             types.AuthChoiceSyntheticAPIKey,
	"venice":                types.AuthChoiceVeniceAPIKey,
	"together":              types.AuthChoiceTogetherAPIKey,
	"huggingface":           types.AuthChoiceHuggingFaceAPIKey,
	"mistral":               types.AuthChoiceMistralAPIKey,
	"opencode":              types.AuthChoiceOpenCodeZen,
	"kilocode":              types.AuthChoiceKilocodeAPIKey,
	"qianfan":               types.AuthChoiceQianfanAPIKey,
}

// zaiAuthChoiceEndpoint AuthChoice 到 Z.AI 端点的映射。
var zaiAuthChoiceEndpoint = map[types.AuthChoice]string{
	types.AuthChoiceZAICodingGlobal: "coding-global",
	types.AuthChoiceZAICodingCN:     "coding-cn",
	types.AuthChoiceZAIGlobal:       "global",
	types.AuthChoiceZAICN:           "cn",
}

// SimpleApiKeyProviderFlow 简单 API Key Provider 流程配置。
type SimpleApiKeyProviderFlow struct {
	Provider            string
	ProfileID           string
	ExpectedProviders   []string
	EnvLabel            string
	PromptMessage       string
	SetCredential       func(types.SecretInput, string, *ApiKeyStorageOptions) error
	DefaultModel        string
	ApplyDefaultConfig  func(OpenClawConfig) OpenClawConfig
	ApplyProviderConfig func(OpenClawConfig) OpenClawConfig
	NoteDefault         string
	NoteMessage         string
	NoteTitle           string
	Normalize           func(string) string
	Validate            func(string) string
}

// simpleApiKeyProviderFlows 简单 API Key Provider 流程定义表。
var simpleApiKeyProviderFlows = map[types.AuthChoice]*SimpleApiKeyProviderFlow{
	types.AuthChoiceAIGatewayAPIKey: {
		Provider: "vercel-ai-gateway", ProfileID: "vercel-ai-gateway:default",
		ExpectedProviders: []string{"vercel-ai-gateway"}, EnvLabel: "AI_GATEWAY_API_KEY",
		PromptMessage: "Enter Vercel AI Gateway API key",
		SetCredential: func(k types.SecretInput, d string, o *ApiKeyStorageOptions) error {
			return SetVercelAiGatewayApiKey(k, d, o)
		},
		DefaultModel: VercelAiGatewayDefaultModelRef, ApplyDefaultConfig: ApplyVercelAiGatewayConfig,
		ApplyProviderConfig: ApplyVercelAiGatewayProviderConfig, NoteDefault: VercelAiGatewayDefaultModelRef,
	},
	types.AuthChoiceMoonshotAPIKey: {
		Provider: "moonshot", ProfileID: "moonshot:default",
		ExpectedProviders: []string{"moonshot"}, EnvLabel: "MOONSHOT_API_KEY",
		PromptMessage: "Enter Moonshot API key",
		SetCredential: func(k types.SecretInput, d string, o *ApiKeyStorageOptions) error { return SetMoonshotApiKey(k, d, o) },
		DefaultModel:  MoonshotDefaultModelRef, ApplyDefaultConfig: ApplyMoonshotConfig,
		ApplyProviderConfig: ApplyMoonshotProviderConfig,
	},
	types.AuthChoiceMoonshotAPIKeyCN: {
		Provider: "moonshot", ProfileID: "moonshot:default",
		ExpectedProviders: []string{"moonshot"}, EnvLabel: "MOONSHOT_API_KEY",
		PromptMessage: "Enter Moonshot API key (.cn)",
		SetCredential: func(k types.SecretInput, d string, o *ApiKeyStorageOptions) error { return SetMoonshotApiKey(k, d, o) },
		DefaultModel:  MoonshotDefaultModelRef, ApplyDefaultConfig: ApplyMoonshotConfigCn,
		ApplyProviderConfig: ApplyMoonshotProviderConfigCn,
	},
	types.AuthChoiceKimiCodeAPIKey: {
		Provider: "kimi-coding", ProfileID: "kimi-coding:default",
		ExpectedProviders: []string{"kimi-code", "kimi-coding"}, EnvLabel: "KIMI_API_KEY",
		PromptMessage: "Enter Kimi Coding API key",
		SetCredential: func(k types.SecretInput, d string, o *ApiKeyStorageOptions) error {
			return SetKimiCodingApiKey(k, d, o)
		},
		DefaultModel: KimiCodingModelRef, ApplyDefaultConfig: ApplyKimiCodeConfig,
		ApplyProviderConfig: ApplyKimiCodeProviderConfig, NoteDefault: KimiCodingModelRef,
		NoteMessage: "Kimi Coding uses a dedicated endpoint and API key.\nGet your API key at: https://www.kimi.com/code/en",
		NoteTitle:   "Kimi Coding",
	},
	types.AuthChoiceXiaomiAPIKey: {
		Provider: "xiaomi", ProfileID: "xiaomi:default",
		ExpectedProviders: []string{"xiaomi"}, EnvLabel: "XIAOMI_API_KEY",
		PromptMessage: "Enter Xiaomi API key",
		SetCredential: func(k types.SecretInput, d string, o *ApiKeyStorageOptions) error { return SetXiaomiApiKey(k, d, o) },
		DefaultModel:  XiaomiDefaultModelRef, ApplyDefaultConfig: ApplyXiaomiConfig,
		ApplyProviderConfig: ApplyXiaomiProviderConfig, NoteDefault: XiaomiDefaultModelRef,
	},
	types.AuthChoiceMistralAPIKey: {
		Provider: "mistral", ProfileID: "mistral:default",
		ExpectedProviders: []string{"mistral"}, EnvLabel: "MISTRAL_API_KEY",
		PromptMessage: "Enter Mistral API key",
		SetCredential: func(k types.SecretInput, d string, o *ApiKeyStorageOptions) error { return SetMistralApiKey(k, d, o) },
		DefaultModel:  MistralDefaultModelRef, ApplyDefaultConfig: ApplyMistralConfig,
		ApplyProviderConfig: ApplyMistralProviderConfig, NoteDefault: MistralDefaultModelRef,
	},
	types.AuthChoiceVeniceAPIKey: {
		Provider: "venice", ProfileID: "venice:default",
		ExpectedProviders: []string{"venice"}, EnvLabel: "VENICE_API_KEY",
		PromptMessage: "Enter Venice AI API key",
		SetCredential: func(k types.SecretInput, d string, o *ApiKeyStorageOptions) error { return SetVeniceApiKey(k, d, o) },
		DefaultModel:  VeniceDefaultModelRef, ApplyDefaultConfig: ApplyVeniceConfig,
		ApplyProviderConfig: ApplyVeniceProviderConfig, NoteDefault: VeniceDefaultModelRef,
		NoteMessage: "Venice AI provides privacy-focused inference with uncensored models.\nGet your API key at: https://venice.ai/settings/api\nSupports 'private' (fully private) and 'anonymized' (proxy) modes.",
		NoteTitle:   "Venice AI",
	},
	types.AuthChoiceOpenCodeZen: {
		Provider: "opencode", ProfileID: "opencode:default",
		ExpectedProviders: []string{"opencode"}, EnvLabel: "OPENCODE_API_KEY",
		PromptMessage: "Enter OpenCode Zen API key",
		SetCredential: func(k types.SecretInput, d string, o *ApiKeyStorageOptions) error {
			return SetOpencodeZenApiKey(k, d, o)
		},
		DefaultModel: OpencodeZenDefaultModel, ApplyDefaultConfig: ApplyOpencodeZenConfig,
		ApplyProviderConfig: ApplyOpencodeZenProviderConfig, NoteDefault: OpencodeZenDefaultModel,
		NoteMessage: "OpenCode Zen provides access to Claude, GPT, Gemini, and more models.\nGet your API key at: https://opencode.ai/auth\nOpenCode Zen bills per request. Check your OpenCode dashboard for details.",
		NoteTitle:   "OpenCode Zen",
	},
	types.AuthChoiceTogetherAPIKey: {
		Provider: "together", ProfileID: "together:default",
		ExpectedProviders: []string{"together"}, EnvLabel: "TOGETHER_API_KEY",
		PromptMessage: "Enter Together AI API key",
		SetCredential: func(k types.SecretInput, d string, o *ApiKeyStorageOptions) error { return SetTogetherApiKey(k, d, o) },
		DefaultModel:  TogetherDefaultModelRef, ApplyDefaultConfig: ApplyTogetherConfig,
		ApplyProviderConfig: ApplyTogetherProviderConfig, NoteDefault: TogetherDefaultModelRef,
		NoteMessage: "Together AI provides access to leading open-source models including Llama, DeepSeek, Qwen, and more.\nGet your API key at: https://api.together.xyz/settings/api-keys",
		NoteTitle:   "Together AI",
	},
	types.AuthChoiceQianfanAPIKey: {
		Provider: "qianfan", ProfileID: "qianfan:default",
		ExpectedProviders: []string{"qianfan"}, EnvLabel: "QIANFAN_API_KEY",
		PromptMessage: "Enter QIANFAN API key",
		SetCredential: func(k types.SecretInput, d string, o *ApiKeyStorageOptions) error { return SetQianfanApiKey(k, d, o) },
		DefaultModel:  QianfanDefaultModelRef, ApplyDefaultConfig: ApplyQianfanConfig,
		ApplyProviderConfig: ApplyQianfanProviderConfig, NoteDefault: QianfanDefaultModelRef,
		NoteMessage: "Get your API key at: https://console.bce.baidu.com/qianfan/ais/console/apiKey\nAPI key format: bce-v3/ALTAK-...",
		NoteTitle:   "QIANFAN",
	},
	types.AuthChoiceKilocodeAPIKey: {
		Provider: "kilocode", ProfileID: "kilocode:default",
		ExpectedProviders: []string{"kilocode"}, EnvLabel: "KILOCODE_API_KEY",
		PromptMessage: "Enter Kilo Gateway API key",
		SetCredential: func(k types.SecretInput, d string, o *ApiKeyStorageOptions) error { return SetKilocodeApiKey(k, d, o) },
		DefaultModel:  KilocodeDefaultModelRef, ApplyDefaultConfig: ApplyKilocodeConfig,
		ApplyProviderConfig: ApplyKilocodeProviderConfig, NoteDefault: KilocodeDefaultModelRef,
	},
	types.AuthChoiceSyntheticAPIKey: {
		Provider: "synthetic", ProfileID: "synthetic:default",
		ExpectedProviders: []string{"synthetic"}, EnvLabel: "SYNTHETIC_API_KEY",
		PromptMessage: "Enter Synthetic API key",
		SetCredential: func(k types.SecretInput, d string, o *ApiKeyStorageOptions) error { return SetSyntheticApiKey(k, d, o) },
		DefaultModel:  SyntheticDefaultModelRef, ApplyDefaultConfig: ApplySyntheticConfig,
		ApplyProviderConfig: ApplySyntheticProviderConfig,
		Normalize:           func(v string) string { return strings.TrimSpace(v) },
		Validate: func(v string) string {
			if strings.TrimSpace(v) != "" {
				return ""
			}
			return "Required"
		},
	},
}

// ApplyAuthChoiceApiProviders 处理所有 API Key Provider 的 auth-choice apply。
// 对应 TS: applyAuthChoiceApiProviders()
func ApplyAuthChoiceApiProviders(params ApplyAuthChoiceParams) (*ApplyAuthChoiceResult, error) {
	state := &ApplyAuthChoiceModelState{
		Config: params.Config,
	}
	applyProviderDefaultModel := CreateAuthChoiceDefaultModelApplier(params, state)

	authChoice := params.AuthChoice
	normalizedTokenProvider := ""
	if params.Opts != nil {
		normalizedTokenProvider = NormalizeTokenProviderInput(params.Opts.TokenProvider)
	}
	var requestedSecretInputMode types.SecretInputMode
	if params.Opts != nil {
		requestedSecretInputMode = NormalizeSecretInputModeInput(params.Opts.SecretInputMode)
	}

	// 如果 authChoice=apiKey 且有 tokenProvider，路由到具体 provider
	if authChoice == types.AuthChoiceAPIKey && params.Opts != nil && params.Opts.TokenProvider != nil {
		if normalizedTokenProvider != "anthropic" && normalizedTokenProvider != "openai" {
			if routed, ok := apiKeyTokenProviderAuthChoice[normalizedTokenProvider]; ok {
				authChoice = routed
			}
		}
	}

	// OpenRouter：委托给专门处理器
	if authChoice == types.AuthChoiceOpenRouterAPIKey {
		return ApplyAuthChoiceOpenRouter(params)
	}

	// LiteLLM：特殊处理（检查已有 profile）
	if authChoice == types.AuthChoiceLiteLLMAPIKey {
		return applyLitellmChoice(params, state, applyProviderDefaultModel, normalizedTokenProvider, requestedSecretInputMode)
	}

	// 简单 API Key Provider 流程
	if flow, ok := simpleApiKeyProviderFlows[authChoice]; ok {
		return applySimpleApiKeyProviderFlow(params, state, applyProviderDefaultModel, flow, normalizedTokenProvider, requestedSecretInputMode)
	}

	// Cloudflare AI Gateway：需要额外的 account/gateway ID
	if authChoice == types.AuthChoiceCloudflareAIGatewayKey {
		return applyCloudflareChoice(params, state, applyProviderDefaultModel, requestedSecretInputMode)
	}

	// Gemini API Key：特殊模型默认值处理
	if authChoice == types.AuthChoiceGeminiAPIKey {
		return applyGeminiChoice(params, state, normalizedTokenProvider, requestedSecretInputMode)
	}

	// Z.AI：端点选择 + 自动检测
	if authChoice == types.AuthChoiceZAIAPIKey ||
		authChoice == types.AuthChoiceZAICodingGlobal ||
		authChoice == types.AuthChoiceZAICodingCN ||
		authChoice == types.AuthChoiceZAIGlobal ||
		authChoice == types.AuthChoiceZAICN {
		return applyZaiChoice(params, state, applyProviderDefaultModel, normalizedTokenProvider, requestedSecretInputMode)
	}

	// HuggingFace：委托
	if authChoice == types.AuthChoiceHuggingFaceAPIKey {
		return ApplyAuthChoiceHuggingface(params)
	}

	return nil, nil
}

// applySimpleApiKeyProviderFlow 执行简单 API Key Provider 流程。
func applySimpleApiKeyProviderFlow(
	params ApplyAuthChoiceParams,
	state *ApplyAuthChoiceModelState,
	applyModel CreateDefaultModelApplierFunc,
	flow *SimpleApiKeyProviderFlow,
	normalizedTokenProvider string,
	requestedSecretInputMode types.SecretInputMode,
) (*ApplyAuthChoiceResult, error) {
	normalize := NormalizeApiKeyInput
	validate := ValidateApiKeyInput
	if flow.Normalize != nil {
		normalize = flow.Normalize
	}
	if flow.Validate != nil {
		validate = flow.Validate
	}

	var token *string
	if params.Opts != nil {
		token = params.Opts.Token
	}

	_, err := EnsureApiKeyFromOptionEnvOrPrompt(EnsureApiKeyFromOptionEnvOrPromptParams{
		Token:             token,
		TokenProvider:     &normalizedTokenProvider,
		SecretInputMode:   requestedSecretInputMode,
		Config:            state.Config,
		ExpectedProviders: flow.ExpectedProviders,
		Provider:          flow.Provider,
		EnvLabel:          flow.EnvLabel,
		PromptMessage:     flow.PromptMessage,
		Normalize:         normalize,
		Validate:          validate,
		Prompter:          params.Prompter,
		SetCredential: func(apiKey types.SecretInput, mode types.SecretInputMode) error {
			m := requestedSecretInputMode
			if mode != "" {
				m = mode
			}
			var opts *ApiKeyStorageOptions
			if m != "" {
				opts = &ApiKeyStorageOptions{SecretInputMode: m}
			}
			return flow.SetCredential(apiKey, params.AgentDir, opts)
		},
		NoteMessage: flow.NoteMessage,
		NoteTitle:   flow.NoteTitle,
	})
	if err != nil {
		return nil, err
	}

	state.Config = ApplyAuthProfileConfig(state.Config, ApplyProfileConfigParams{
		ProfileID: flow.ProfileID,
		Provider:  flow.Provider,
		Mode:      "api_key",
	})

	noteDefault := flow.NoteDefault
	if noteDefault == "" {
		noteDefault = flow.DefaultModel
	}
	if err := applyModel(DefaultModelApplyOptions{
		DefaultModel:        flow.DefaultModel,
		ApplyDefaultConfig:  flow.ApplyDefaultConfig,
		ApplyProviderConfig: flow.ApplyProviderConfig,
		NoteDefault:         noteDefault,
	}); err != nil {
		return nil, err
	}

	return &ApplyAuthChoiceResult{Config: state.Config, AgentModelOverride: state.AgentModelOverride}, nil
}

// applyLitellmChoice 处理 LiteLLM 特殊逻辑。
func applyLitellmChoice(
	params ApplyAuthChoiceParams,
	state *ApplyAuthChoiceModelState,
	applyModel CreateDefaultModelApplierFunc,
	normalizedTokenProvider string,
	requestedSecretInputMode types.SecretInputMode,
) (*ApplyAuthChoiceResult, error) {
	store := EnsureAuthProfileStore(params.AgentDir)
	profileOrder := ResolveAuthProfileOrder(state.Config, store, "litellm")
	existingProfileID := ""
	for _, pid := range profileOrder {
		if _, ok := store.Profiles[pid]; ok {
			existingProfileID = pid
			break
		}
	}
	profileID := "litellm:default"
	hasCredential := existingProfileID != ""
	if hasCredential {
		profileID = existingProfileID
	}

	if !hasCredential {
		var token *string
		if params.Opts != nil {
			token = params.Opts.Token
		}
		_, err := EnsureApiKeyFromOptionEnvOrPrompt(EnsureApiKeyFromOptionEnvOrPromptParams{
			Token:             token,
			TokenProvider:     &normalizedTokenProvider,
			SecretInputMode:   requestedSecretInputMode,
			Config:            state.Config,
			ExpectedProviders: []string{"litellm"},
			Provider:          "litellm",
			EnvLabel:          "LITELLM_API_KEY",
			PromptMessage:     "Enter LiteLLM API key",
			Normalize:         NormalizeApiKeyInput,
			Validate:          ValidateApiKeyInput,
			Prompter:          params.Prompter,
			SetCredential: func(apiKey types.SecretInput, mode types.SecretInputMode) error {
				return SetLitellmApiKey(apiKey, params.AgentDir, &ApiKeyStorageOptions{SecretInputMode: mode})
			},
			NoteMessage: "LiteLLM provides a unified API to 100+ LLM providers.\nGet your API key from your LiteLLM proxy or https://litellm.ai\nDefault proxy runs on http://localhost:4000",
			NoteTitle:   "LiteLLM",
		})
		if err != nil {
			return nil, err
		}
		hasCredential = true
	}

	if hasCredential {
		state.Config = ApplyAuthProfileConfig(state.Config, ApplyProfileConfigParams{
			ProfileID: profileID,
			Provider:  "litellm",
			Mode:      "api_key",
		})
	}

	if err := applyModel(DefaultModelApplyOptions{
		DefaultModel:        LitellmDefaultModelRef,
		ApplyDefaultConfig:  ApplyLitellmConfig,
		ApplyProviderConfig: ApplyLitellmProviderConfig,
		NoteDefault:         LitellmDefaultModelRef,
	}); err != nil {
		return nil, err
	}

	return &ApplyAuthChoiceResult{Config: state.Config, AgentModelOverride: state.AgentModelOverride}, nil
}

// applyCloudflareChoice 处理 Cloudflare AI Gateway 特殊逻辑。
func applyCloudflareChoice(
	params ApplyAuthChoiceParams,
	state *ApplyAuthChoiceModelState,
	applyModel CreateDefaultModelApplierFunc,
	requestedSecretInputMode types.SecretInputMode,
) (*ApplyAuthChoiceResult, error) {
	accountID := ""
	gatewayID := ""
	if params.Opts != nil {
		if params.Opts.CloudflareAIGatewayAccountID != nil {
			accountID = strings.TrimSpace(*params.Opts.CloudflareAIGatewayAccountID)
		}
		if params.Opts.CloudflareAIGatewayGatewayID != nil {
			gatewayID = strings.TrimSpace(*params.Opts.CloudflareAIGatewayGatewayID)
		}
	}

	if accountID == "" {
		val, err := params.Prompter.Text(TextPromptOptions{
			Message: "Enter Cloudflare Account ID",
			Validate: func(v string) string {
				if strings.TrimSpace(v) != "" {
					return ""
				}
				return "Account ID is required"
			},
		})
		if err != nil {
			return nil, err
		}
		accountID = strings.TrimSpace(val)
	}
	if gatewayID == "" {
		val, err := params.Prompter.Text(TextPromptOptions{
			Message: "Enter Cloudflare AI Gateway ID",
			Validate: func(v string) string {
				if strings.TrimSpace(v) != "" {
					return ""
				}
				return "Gateway ID is required"
			},
		})
		if err != nil {
			return nil, err
		}
		gatewayID = strings.TrimSpace(val)
	}

	var cfApiKey *string
	if params.Opts != nil {
		cfApiKey = params.Opts.CloudflareAIGatewayAPIKey
	}
	cfTokenProvider := "cloudflare-ai-gateway"

	_, err := EnsureApiKeyFromOptionEnvOrPrompt(EnsureApiKeyFromOptionEnvOrPromptParams{
		Token:             cfApiKey,
		TokenProvider:     &cfTokenProvider,
		SecretInputMode:   requestedSecretInputMode,
		Config:            state.Config,
		ExpectedProviders: []string{"cloudflare-ai-gateway"},
		Provider:          "cloudflare-ai-gateway",
		EnvLabel:          "CLOUDFLARE_AI_GATEWAY_API_KEY",
		PromptMessage:     "Enter Cloudflare AI Gateway API key",
		Normalize:         NormalizeApiKeyInput,
		Validate:          ValidateApiKeyInput,
		Prompter:          params.Prompter,
		SetCredential: func(apiKey types.SecretInput, mode types.SecretInputMode) error {
			return SetCloudflareAiGatewayConfig(accountID, gatewayID, apiKey, params.AgentDir, &ApiKeyStorageOptions{SecretInputMode: mode})
		},
	})
	if err != nil {
		return nil, err
	}

	state.Config = ApplyAuthProfileConfig(state.Config, ApplyProfileConfigParams{
		ProfileID: "cloudflare-ai-gateway:default",
		Provider:  "cloudflare-ai-gateway",
		Mode:      "api_key",
	})

	if err := applyModel(DefaultModelApplyOptions{
		DefaultModel: CloudflareAiGatewayDefaultModelRef,
		ApplyDefaultConfig: func(cfg OpenClawConfig) OpenClawConfig {
			return ApplyCloudflareAiGatewayConfigWithIDs(cfg, accountID, gatewayID)
		},
		ApplyProviderConfig: func(cfg OpenClawConfig) OpenClawConfig {
			return ApplyCloudflareAiGatewayProviderConfigWithIDs(cfg, accountID, gatewayID)
		},
		NoteDefault: CloudflareAiGatewayDefaultModelRef,
	}); err != nil {
		return nil, err
	}

	return &ApplyAuthChoiceResult{Config: state.Config, AgentModelOverride: state.AgentModelOverride}, nil
}

// applyGeminiChoice 处理 Gemini API Key 特殊逻辑。
func applyGeminiChoice(
	params ApplyAuthChoiceParams,
	state *ApplyAuthChoiceModelState,
	normalizedTokenProvider string,
	requestedSecretInputMode types.SecretInputMode,
) (*ApplyAuthChoiceResult, error) {
	var token *string
	if params.Opts != nil {
		token = params.Opts.Token
	}
	noteAgentModel := CreateAuthChoiceAgentModelNoter(params)

	_, err := EnsureApiKeyFromOptionEnvOrPrompt(EnsureApiKeyFromOptionEnvOrPromptParams{
		Token:             token,
		TokenProvider:     &normalizedTokenProvider,
		SecretInputMode:   requestedSecretInputMode,
		Config:            state.Config,
		ExpectedProviders: []string{"google"},
		Provider:          "google",
		EnvLabel:          "GEMINI_API_KEY",
		PromptMessage:     "Enter Gemini API key",
		Normalize:         NormalizeApiKeyInput,
		Validate:          ValidateApiKeyInput,
		Prompter:          params.Prompter,
		SetCredential: func(apiKey types.SecretInput, mode types.SecretInputMode) error {
			return SetGeminiApiKey(apiKey, params.AgentDir, &ApiKeyStorageOptions{SecretInputMode: mode})
		},
	})
	if err != nil {
		return nil, err
	}

	state.Config = ApplyAuthProfileConfig(state.Config, ApplyProfileConfigParams{
		ProfileID: "google:default",
		Provider:  "google",
		Mode:      "api_key",
	})

	if params.SetDefaultModel {
		result := ApplyGoogleGeminiModelDefault(state.Config)
		state.Config = result.Next
		if result.Changed {
			_ = params.Prompter.Note(
				fmt.Sprintf("Default model set to %s", GoogleGeminiDefaultModel),
				"Model configured",
			)
		}
	} else {
		state.AgentModelOverride = GoogleGeminiDefaultModel
		_ = noteAgentModel(GoogleGeminiDefaultModel)
	}

	return &ApplyAuthChoiceResult{Config: state.Config, AgentModelOverride: state.AgentModelOverride}, nil
}

// applyZaiChoice 处理 Z.AI 特殊逻辑。
func applyZaiChoice(
	params ApplyAuthChoiceParams,
	state *ApplyAuthChoiceModelState,
	applyModel CreateDefaultModelApplierFunc,
	normalizedTokenProvider string,
	requestedSecretInputMode types.SecretInputMode,
) (*ApplyAuthChoiceResult, error) {
	endpoint := zaiAuthChoiceEndpoint[params.AuthChoice]

	var token *string
	if params.Opts != nil {
		token = params.Opts.Token
	}

	apiKey, err := EnsureApiKeyFromOptionEnvOrPrompt(EnsureApiKeyFromOptionEnvOrPromptParams{
		Token:             token,
		TokenProvider:     &normalizedTokenProvider,
		SecretInputMode:   requestedSecretInputMode,
		Config:            state.Config,
		ExpectedProviders: []string{"zai"},
		Provider:          "zai",
		EnvLabel:          "ZAI_API_KEY",
		PromptMessage:     "Enter Z.AI API key",
		Normalize:         NormalizeApiKeyInput,
		Validate:          ValidateApiKeyInput,
		Prompter:          params.Prompter,
		SetCredential: func(apiKey types.SecretInput, mode types.SecretInputMode) error {
			return SetZaiApiKey(apiKey, params.AgentDir, &ApiKeyStorageOptions{SecretInputMode: mode})
		},
	})
	if err != nil {
		return nil, err
	}

	// 自动检测端点
	var modelIdOverride string
	if endpoint == "" {
		detected := DetectZaiEndpoint(apiKey)
		if detected != nil {
			endpoint = detected.Endpoint
			modelIdOverride = detected.ModelID
			_ = params.Prompter.Note(detected.Note, "Z.AI endpoint")
		} else {
			selected, sErr := params.Prompter.Select(SelectPromptOptions{
				Message:      "Select Z.AI endpoint",
				InitialValue: "global",
				Options: []SelectOption{
					{Value: "coding-global", Label: "Coding-Plan-Global", Hint: "GLM Coding Plan Global (api.z.ai)"},
					{Value: "coding-cn", Label: "Coding-Plan-CN", Hint: "GLM Coding Plan CN (open.bigmodel.cn)"},
					{Value: "global", Label: "Global", Hint: "Z.AI Global (api.z.ai)"},
					{Value: "cn", Label: "CN", Hint: "Z.AI CN (open.bigmodel.cn)"},
				},
			})
			if sErr != nil {
				return nil, sErr
			}
			endpoint = selected
		}
	}

	state.Config = ApplyAuthProfileConfig(state.Config, ApplyProfileConfigParams{
		ProfileID: "zai:default",
		Provider:  "zai",
		Mode:      "api_key",
	})

	defaultModel := ZaiDefaultModelRef
	if modelIdOverride != "" {
		defaultModel = "zai/" + modelIdOverride
	}

	zaiOpts := ZaiConfigOptions{Endpoint: endpoint}
	if modelIdOverride != "" {
		zaiOpts.ModelID = modelIdOverride
	}

	if err := applyModel(DefaultModelApplyOptions{
		DefaultModel: defaultModel,
		ApplyDefaultConfig: func(cfg OpenClawConfig) OpenClawConfig {
			return ApplyZaiConfig(cfg, zaiOpts)
		},
		ApplyProviderConfig: func(cfg OpenClawConfig) OpenClawConfig {
			return ApplyZaiProviderConfig(cfg, zaiOpts)
		},
		NoteDefault: defaultModel,
	}); err != nil {
		return nil, err
	}

	return &ApplyAuthChoiceResult{Config: state.Config, AgentModelOverride: state.AgentModelOverride}, nil
}
