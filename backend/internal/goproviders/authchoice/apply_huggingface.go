// authchoice/apply_huggingface.go — HuggingFace 认证 apply
// 对应 TS 文件: src/commands/auth-choice.apply.huggingface.ts
// HuggingFace API Key + 模型发现 + 模型选择。
package authchoice

import (
	"sort"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// HuggingfaceDefaultModelRef HuggingFace 默认模型引用。
const HuggingfaceDefaultModelRef = "huggingface/Qwen/Qwen2.5-Coder-32B-Instruct"

// ApplyAuthChoiceHuggingface HuggingFace 认证 apply。
// API Key 获取 + 模型发现 + 交互式模型选择 + 默认模型应用。
// 对应 TS: applyAuthChoiceHuggingface()
func ApplyAuthChoiceHuggingface(params ApplyAuthChoiceParams) (*ApplyAuthChoiceResult, error) {
	if params.AuthChoice != types.AuthChoiceHuggingFaceAPIKey {
		return nil, nil
	}

	nextConfig := params.Config
	var agentModelOverride string
	noteAgentModel := CreateAuthChoiceAgentModelNoter(params)
	requestedSecretInputMode := NormalizeSecretInputModeInput(getSecretInputMode(params.Opts))

	// 获取 API Key
	hfKey, err := EnsureApiKeyFromOptionEnvOrPrompt(EnsureApiKeyFromOptionEnvOrPromptParams{
		Token:             getOptToken(params.Opts, "token"),
		TokenProvider:     getOptTokenProvider(params.Opts),
		SecretInputMode:   requestedSecretInputMode,
		Config:            nextConfig,
		ExpectedProviders: []string{"huggingface"},
		Provider:          "huggingface",
		EnvLabel:          "Hugging Face token",
		PromptMessage:     "Enter Hugging Face API key (HF token)",
		Normalize:         NormalizeApiKeyInput,
		Validate:          ValidateApiKeyInput,
		Prompter:          params.Prompter,
		SetCredential: func(apiKey types.SecretInput, mode types.SecretInputMode) error {
			return SetHuggingfaceApiKey(apiKey, params.AgentDir, &ApiKeyStorageOptions{SecretInputMode: mode})
		},
		NoteMessage: strings.Join([]string{
			"Hugging Face Inference Providers offer OpenAI-compatible chat completions.",
			"Create a token at: https://huggingface.co/settings/tokens (fine-grained, 'Make calls to Inference Providers').",
		}, "\n"),
		NoteTitle: "Hugging Face",
	})
	if err != nil {
		return nil, err
	}

	nextConfig = ApplyAuthProfileConfig(nextConfig, ApplyProfileConfigParams{
		ProfileID: "huggingface:default",
		Provider:  "huggingface",
		Mode:      "api_key",
	})

	// 发现模型（stub）
	models := DiscoverHuggingfaceModels(hfKey)

	// 构建选项列表
	modelRefPrefix := "huggingface/"
	var options []SelectOption
	for _, m := range models {
		baseRef := modelRefPrefix + m.ID
		label := m.Name
		if label == "" {
			label = m.ID
		}
		options = append(options,
			SelectOption{Value: baseRef, Label: label},
			SelectOption{Value: baseRef + ":cheapest", Label: label + " (cheapest)"},
			SelectOption{Value: baseRef + ":fastest", Label: label + " (fastest)"},
		)
	}

	defaultRef := HuggingfaceDefaultModelRef

	// 排序：默认模型优先
	sort.SliceStable(options, func(i, j int) bool {
		if options[i].Value == defaultRef {
			return true
		}
		if options[j].Value == defaultRef {
			return false
		}
		return strings.Compare(
			strings.ToLower(options[i].Label),
			strings.ToLower(options[j].Label),
		) < 0
	})

	// 选择模型
	selectedModelRef := defaultRef
	if len(options) == 1 {
		selectedModelRef = options[0].Value
	} else if len(options) > 1 {
		initialValue := defaultRef
		hasDefault := false
		for _, o := range options {
			if o.Value == defaultRef {
				hasDefault = true
				break
			}
		}
		if !hasDefault && len(options) > 0 {
			initialValue = options[0].Value
		}

		selected, sErr := params.Prompter.Select(SelectPromptOptions{
			Message:      "Default Hugging Face model",
			Options:      options,
			InitialValue: initialValue,
		})
		if sErr != nil {
			return nil, sErr
		}
		selectedModelRef = selected
	}

	// 检查策略锁定
	if IsHuggingfacePolicyLocked(selectedModelRef) {
		_ = params.Prompter.Note(
			"Provider locked — router will choose backend by cost or speed.",
			"Hugging Face",
		)
	}

	// 应用默认模型
	applied, err := ApplyDefaultModelChoice(DefaultModelChoiceParams{
		Config:          nextConfig,
		SetDefaultModel: params.SetDefaultModel,
		DefaultModel:    selectedModelRef,
		ApplyDefaultConfig: func(config OpenClawConfig) OpenClawConfig {
			withProvider := ApplyHuggingfaceConfig(config)
			withPrimary := ApplyPrimaryModel(withProvider, selectedModelRef)
			return EnsureModelAllowlistEntry(withPrimary, selectedModelRef)
		},
		ApplyProviderConfig: ApplyHuggingfaceConfig,
		NoteDefault:         selectedModelRef,
		NoteAgentModel:      noteAgentModel,
		Prompter:            params.Prompter,
	})
	if err != nil {
		return nil, err
	}

	nextConfig = applied.Config
	if applied.AgentModelOverride != "" {
		agentModelOverride = applied.AgentModelOverride
	}

	return &ApplyAuthChoiceResult{Config: nextConfig, AgentModelOverride: agentModelOverride}, nil
}
