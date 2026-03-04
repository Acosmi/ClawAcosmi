package gateway

// wizard_open_coder.go — Open Coder 子智能体配置向导
//
// 3 步向导: Provider → API Key (+baseURL) → Model
// 保存到 subAgents.openCoder.*，独立于主 agent 配置。

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/agents/models"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// RunOpenCoderWizard 执行 open-coder 配置向导。
// Provider → API Key (+baseURL) → Model → Confirm → Save
func RunOpenCoderWizard(deps WizardOnboardingDeps) WizardRunnerFunc {
	return func(prompter WizardPrompter) error {
		log := slog.Default().With("subsystem", "wizard-open-coder")

		// 加载现有配置
		var cfg *types.OpenAcosmiConfig
		if deps.ConfigLoader != nil {
			if loaded, err := deps.ConfigLoader.LoadConfig(); err == nil {
				cfg = loaded
			}
		}
		if cfg == nil {
			cfg = &types.OpenAcosmiConfig{}
		}

		// 读取现有 open-coder 设置（用于预填充）
		var existing *types.OpenCoderSettings
		if cfg.SubAgents != nil && cfg.SubAgents.OpenCoder != nil {
			existing = cfg.SubAgents.OpenCoder
		}

		// ---------- Intro ----------
		if err := prompter.Intro("Open Coder Configuration"); err != nil {
			return err
		}

		if err := prompter.Note(
			"Configure the AI provider and model for the Open Coder sub-agent.\n\n"+
				"Open Coder can use a different provider/model from the main agent.\n"+
				"This is useful for cost optimization (e.g. DeepSeek for coding tasks).",
			"Open Coder Setup",
		); err != nil {
			return err
		}

		// ---------- Step 1: Select Provider ----------
		providers := getWizardProviders()

		// 追加 "OpenAI Compatible" 选项
		providers = append(providers, wizardProviderInfo{
			ID:           "openai-compatible",
			Label:        "OpenAI Compatible",
			Hint:         "Custom endpoint (vLLM, Ollama, LiteLLM, etc.)",
			DefaultModel: "gpt-3.5-turbo",
		})

		var providerOptions []WizardStepOption
		for _, p := range providers {
			providerOptions = append(providerOptions, WizardStepOption{
				Value: p.ID,
				Label: p.Label,
				Hint:  p.Hint,
			})
		}

		// 默认预选: 已有配置 → deepseek → anthropic
		defaultProvider := "deepseek"
		if existing != nil && existing.Provider != "" {
			defaultProvider = existing.Provider
		}

		selectedProvider, err := prompter.Select(
			"Select AI provider for Open Coder",
			providerOptions,
			defaultProvider,
		)
		if err != nil {
			return err
		}
		providerID, _ := selectedProvider.(string)
		if providerID == "" {
			providerID = "deepseek"
		}
		log.Info("open-coder wizard: provider selected", "provider", providerID)

		// 查找 provider 信息
		var selectedInfo wizardProviderInfo
		for _, p := range providers {
			if p.ID == providerID {
				selectedInfo = p
				break
			}
		}

		// ---------- Step 2: API Key + BaseURL ----------
		var apiKey, baseURL string

		// OpenAI Compatible 模式：先输入 baseURL
		if providerID == "openai-compatible" {
			defaultBaseURL := ""
			if existing != nil && existing.BaseURL != "" {
				defaultBaseURL = existing.BaseURL
			}
			inputURL, urlErr := prompter.Text(
				"Enter the API base URL",
				"http://localhost:11434/v1",
				defaultBaseURL,
				false,
			)
			if urlErr != nil {
				return urlErr
			}
			baseURL = strings.TrimSpace(inputURL)
		}

		// API Key 输入
		if providerID != "ollama" {
			// 检查环境变量中是否已有 key
			existingEnvKey := ""
			if providerID != "openai-compatible" {
				existingEnvKey = models.ResolveEnvApiKeyWithFallback(providerID)
			}

			if existingEnvKey != "" {
				envVar := models.ResolveEnvApiKeyVarName(providerID)
				useExisting, confirmErr := prompter.Confirm(
					fmt.Sprintf("Detected %s in environment. Use it?", envVar),
					true,
				)
				if confirmErr != nil {
					return confirmErr
				}
				if useExisting {
					apiKey = existingEnvKey
				}
			}

			if apiKey == "" {
				placeholder := "sk-..."
				if providerID == "deepseek" {
					placeholder = "sk-..."
				}
				inputKey, keyErr := prompter.Text(
					"Enter API Key for Open Coder",
					placeholder,
					"",
					true, // sensitive
				)
				if keyErr != nil {
					return keyErr
				}
				apiKey = strings.TrimSpace(inputKey)
				if apiKey == "" {
					return fmt.Errorf("API key is required for %s", selectedInfo.Label)
				}
			}
		}

		log.Info("open-coder wizard: API key configured",
			"provider", providerID,
			"hasKey", apiKey != "",
			"hasBaseURL", baseURL != "",
		)

		// ---------- Step 3: Select Model ----------
		// 对 openai-compatible 使用 openai 的模型列表作为参考
		catalogProvider := providerID
		if providerID == "openai-compatible" {
			catalogProvider = "openai"
		}

		modelOptions := resolveModelOptions(catalogProvider, selectedInfo.DefaultModel, deps.ModelCatalog)

		// 对 openai-compatible 追加自定义模型输入提示
		if providerID == "openai-compatible" {
			modelOptions = append(modelOptions, WizardStepOption{
				Value: "__custom__",
				Label: "Custom model ID",
				Hint:  "Enter your own model identifier",
			})
		}

		defaultModel := selectedInfo.DefaultModel
		if existing != nil && existing.Model != "" {
			defaultModel = existing.Model
		}

		var selectedModel string
		if len(modelOptions) > 0 {
			selected, selErr := prompter.Select(
				"Select model for Open Coder",
				modelOptions,
				defaultModel,
			)
			if selErr != nil {
				return selErr
			}
			if s, ok := selected.(string); ok {
				selectedModel = s
			}
		}

		// 自定义模型输入
		if selectedModel == "__custom__" || selectedModel == "" {
			customModel, cmErr := prompter.Text(
				"Enter custom model ID",
				"model-name",
				defaultModel,
				false,
			)
			if cmErr != nil {
				return cmErr
			}
			selectedModel = strings.TrimSpace(customModel)
		}
		if selectedModel == "" {
			selectedModel = defaultModel
		}
		if selectedModel == "" {
			selectedModel = "deepseek-chat"
		}
		log.Info("open-coder wizard: model selected", "model", selectedModel)

		// ---------- Confirm ----------
		// 对 openai-compatible 使用 "openai" 作为实际 provider（RunEmbeddedPiAgent 使用 OpenAI 兼容 API）
		actualProvider := providerID
		if providerID == "openai-compatible" {
			actualProvider = "openai"
		}

		summary := fmt.Sprintf(
			"Provider: %s\nModel: %s\nAPI Key: %s",
			selectedInfo.Label,
			selectedModel,
			maskAPIKey(apiKey),
		)
		if baseURL != "" {
			summary += fmt.Sprintf("\nBase URL: %s", baseURL)
		}

		confirmed, confirmErr := prompter.Confirm(
			"Save Open Coder configuration?\n\n"+summary,
			true,
		)
		if confirmErr != nil {
			return confirmErr
		}
		if !confirmed {
			return &WizardCancelledError{}
		}

		// ---------- 保存配置 ----------
		if cfg.SubAgents == nil {
			cfg.SubAgents = &types.SubAgentConfig{}
		}
		cfg.SubAgents.OpenCoder = &types.OpenCoderSettings{
			Provider: actualProvider,
			Model:    selectedModel,
			BaseURL:  baseURL,
		}
		// 仅当 apiKey 非环境变量来源时存储
		if apiKey != "" && models.ResolveEnvApiKeyWithFallback(actualProvider) != apiKey {
			cfg.SubAgents.OpenCoder.APIKey = apiKey
		}

		if deps.ConfigLoader != nil {
			if writeErr := deps.ConfigLoader.WriteConfigFile(cfg); writeErr != nil {
				return fmt.Errorf("save open-coder config: %w", writeErr)
			}
		}

		log.Info("open-coder wizard: configuration saved",
			"provider", actualProvider,
			"model", selectedModel,
		)

		if err := prompter.Outro(
			fmt.Sprintf("Open Coder configured: %s / %s", selectedInfo.Label, selectedModel),
		); err != nil {
			return err
		}

		return nil
	}
}
