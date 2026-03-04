package gateway

// wizard_onboarding.go — Setup Wizard 引导流程
// TS 对照: src/wizard/onboarding.ts (471L) 裁剪为核心三步
//
// Gateway 模式的 Setup Wizard 仅需:
// 1. 选择 Provider（anthropic/deepseek/openai/google 等）
// 2. 输入 API Key（写入配置）
// 3. 选择默认模型
// 4. 确认并保存
//
// 完整的 channel/skills/hooks/daemon 配置通过 config 页面独立处理。

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/agents/auth"
	"github.com/Acosmi/ClawAcosmi/internal/agents/models"
	"github.com/Acosmi/ClawAcosmi/internal/config"
	"github.com/Acosmi/ClawAcosmi/pkg/i18n"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ---------- Provider 定义 ----------

// wizardProviderInfo 向导中的 provider 选项信息。
type wizardProviderInfo struct {
	ID           string
	Label        string
	Hint         string
	EnvVar       string
	DefaultModel string
}

// getWizardProviders 获取向导支持的 provider 列表。
// 检测环境变量中已有 API Key 的 provider 标注 ✓。
func getWizardProviders() []wizardProviderInfo {
	// 主要 provider 列表（按推荐顺序）
	providerOrder := []struct {
		id    string
		label string
	}{
		{"anthropic", "Anthropic (Claude)"},
		{"deepseek", "DeepSeek"},
		{"openai", "OpenAI (GPT)"},
		{"google", "Google (Gemini)"},
		{"groq", "Groq"},
		{"mistral", "Mistral"},
		{"xai", "xAI (Grok)"},
		{"moonshot", "Moonshot (Kimi)"},
		{"minimax", "MiniMax"},
		{"qwen", "Alibaba (Qwen)"},
		{"cerebras", "Cerebras"},
		{"openrouter", "OpenRouter"},
		{"ollama", "Ollama (Local)"},
	}

	var result []wizardProviderInfo
	for _, p := range providerOrder {
		envVar := models.ResolveEnvApiKeyVarName(p.id)
		hasKey := false
		if envVar != "" {
			hasKey = os.Getenv(envVar) != ""
		}
		// 也检查 fallback
		if !hasKey {
			hasKey = models.ResolveEnvApiKeyWithFallback(p.id) != ""
		}

		hint := envVar
		if hasKey {
			hint = fmt.Sprintf("✓ %s detected", envVar)
		}

		defaultModel := ""
		if defaults := models.GetProviderDefaults(p.id); defaults != nil {
			defaultModel = defaults.DefaultModel
		}
		if defaultModel == "" {
			if p.id == "anthropic" {
				defaultModel = models.DefaultModel
			} else if p.id == "openai" {
				defaultModel = "gpt-5.2"
			} else if p.id == "google" {
				defaultModel = "gemini-3-flash"
			}
		}

		result = append(result, wizardProviderInfo{
			ID:           p.id,
			Label:        p.label,
			Hint:         hint,
			EnvVar:       envVar,
			DefaultModel: defaultModel,
		})
	}
	return result
}

// ---------- 引导流程 ----------

// WizardFlow 向导流程模式。
type WizardFlow string

const (
	WizardFlowQuickstart WizardFlow = "quickstart"
	WizardFlowAdvanced   WizardFlow = "advanced"
)

// WizardSetupFn 配置阶段回调函数签名。
// 接收当前配置和 prompter，返回更新后的配置。
type WizardSetupFn func(cfg *types.OpenAcosmiConfig, prompter WizardPrompter) (*types.OpenAcosmiConfig, error)

// WizardOnboardingDeps 引导流程依赖。
type WizardOnboardingDeps struct {
	ConfigLoader *config.ConfigLoader
	ModelCatalog *models.ModelCatalog
	State        *GatewayState

	// 以下为高级版向导新增依赖
	WorkspaceDir   string        // workspace 路径
	ChannelSetupFn WizardSetupFn // 频道配置回调（可选）
	SkillsSetupFn  WizardSetupFn // 技能配置回调（可选）
	HooksSetupFn   WizardSetupFn // hooks 配置回调（可选）
}

// RunOnboardingWizard 执行简化版引导流程。
// 通过 WizardPrompter 接口与前端交互。
func RunOnboardingWizard(deps WizardOnboardingDeps) WizardRunnerFunc {
	return func(prompter WizardPrompter) error {
		log := slog.Default().With("subsystem", "wizard-onboarding")

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

		// ---------- Intro ----------
		if err := prompter.Intro(i18n.Tp("onboard.title")); err != nil {
			return err
		}

		if err := prompter.Note(
			i18n.Tp("onboard.welcome"),
			i18n.Tp("onboard.provider.title"),
		); err != nil {
			return err
		}

		// ---------- Step 1: Select Provider ----------
		providers := getWizardProviders()
		var providerOptions []WizardStepOption
		for _, p := range providers {
			providerOptions = append(providerOptions, WizardStepOption{
				Value: p.ID,
				Label: p.Label,
				Hint:  p.Hint,
			})
		}

		selectedProvider, err := prompter.Select(
			i18n.Tp("onboard.provider.select"),
			providerOptions,
			"anthropic",
		)
		if err != nil {
			return err
		}
		providerID, _ := selectedProvider.(string)
		if providerID == "" {
			providerID = "anthropic"
		}
		log.Info("wizard: provider selected", "provider", providerID)

		// 查找 provider 信息
		var selectedInfo wizardProviderInfo
		for _, p := range providers {
			if p.ID == providerID {
				selectedInfo = p
				break
			}
		}

		// ---------- S1-3: OAuth 模式选择（支持 Google/Qwen/MiniMax/OpenAI Codex） ----------
		var apiKey string
		oauthSuccess := false

		oauthCfg := auth.GetOAuthProviderConfig(providerID)
		if oauthCfg != nil {
			// 此 provider 支持 OAuth，提供选择
			authMode, authErr := prompter.Select(
				fmt.Sprintf("%s Authentication Mode", selectedInfo.Label),
				[]WizardStepOption{
					{Value: "api_key", Label: "API Key", Hint: fmt.Sprintf("Use a %s API key", selectedInfo.Label)},
					{Value: "oauth", Label: "OAuth (Browser Login)", Hint: "Authenticate via browser"},
				},
				"api_key",
			)
			if authErr != nil {
				return authErr
			}

			if fmt.Sprint(authMode) == "oauth" {
				if oauthCfg.ClientID == "" {
					// 缺少 Client ID，引导用户设置
					envVar := strings.ToUpper(strings.ReplaceAll(providerID, "-", "_")) + "_CLIENT_ID"
					if noteErr := prompter.Note(
						fmt.Sprintf("OAuth requires a Client ID.\n\n"+
							"Please set the %s environment variable, then restart the wizard.\n\n"+
							"For Google:\n"+
							"1. Go to https://console.cloud.google.com/apis/credentials\n"+
							"2. Create an OAuth 2.0 Client ID (Desktop app)\n"+
							"3. Set %s=<your-client-id>\n\n"+
							"Falling back to API Key mode.", envVar, envVar),
						"OAuth Setup",
					); noteErr != nil {
						return noteErr
					}
					log.Info("wizard: oauth client_id missing, falling back to api key", "provider", providerID)
				} else {
					// 有 Client ID，执行真正的 OAuth 流程
					if noteErr := prompter.Note(
						"Opening browser for authorization...\n"+
							"Complete the login in your browser, then return here.",
						"OAuth",
					); noteErr != nil {
						return noteErr
					}

					tokenResp, oauthErr := auth.RunOAuthWebFlow(context.Background(), oauthCfg, nil)
					if oauthErr != nil {
						log.Warn("wizard: oauth flow failed", "provider", providerID, "error", oauthErr)
						_ = prompter.Note(
							fmt.Sprintf("OAuth failed: %s\n\nFalling back to API Key mode.", oauthErr),
							"OAuth Error",
						)
					} else {
						apiKey = tokenResp.AccessToken
						oauthSuccess = true
						log.Info("wizard: oauth succeeded", "provider", providerID)
						_ = prompter.Note(
							"✅ OAuth authorization successful!",
							"OAuth",
						)
					}
				}
			}
		}

		// ---------- Step 2: API Key（OAuth 成功时跳过） ----------
		if !oauthSuccess {
			// 检查是否已有环境变量中的 key
			existingKey := models.ResolveEnvApiKeyWithFallback(providerID)

			if apiKey == "" && existingKey != "" {
				// 已有 key，询问是否使用
				useExisting, err := prompter.Confirm(
					i18n.Tf("onboard.provider.env_detected", selectedInfo.EnvVar),
					true,
				)
				if err != nil {
					return err
				}
				if useExisting {
					apiKey = existingKey
				}
			}

			if apiKey == "" {
				// 需要输入新 key
				inputKey, err := prompter.Text(
					i18n.Tp("onboard.apikey.prompt"),
					fmt.Sprintf("sk-... or %s", selectedInfo.EnvVar),
					"",
					true, // sensitive
				)
				if err != nil {
					return err
				}
				apiKey = strings.TrimSpace(inputKey)
				if apiKey == "" && providerID != "ollama" {
					return fmt.Errorf("API key is required for %s", selectedInfo.Label)
				}
			}
		}
		log.Info("wizard: API key configured", "provider", providerID, "hasKey", apiKey != "", "oauth", oauthSuccess)

		// ---------- Step 3: Select Model ----------
		modelOptions := resolveModelOptions(providerID, selectedInfo.DefaultModel, deps.ModelCatalog)

		var selectedModel string
		if len(modelOptions) > 0 {
			selected, err := prompter.Select(
				i18n.Tp("onboard.model.select"),
				modelOptions,
				selectedInfo.DefaultModel,
			)
			if err != nil {
				return err
			}
			if s, ok := selected.(string); ok {
				selectedModel = s
			}
		}
		if selectedModel == "" {
			selectedModel = selectedInfo.DefaultModel
		}
		if selectedModel == "" {
			selectedModel = models.DefaultModel
		}
		log.Info("wizard: model selected", "model", selectedModel)

		// ---------- Step 4: Confirm ----------
		summary := fmt.Sprintf(
			"Provider: %s\nModel: %s\nAPI Key: %s",
			selectedInfo.Label,
			selectedModel,
			maskAPIKey(apiKey),
		)
		confirmed, err := prompter.Confirm(
			i18n.Tp("onboard.model.confirm")+"\n\n"+summary,
			true,
		)
		if err != nil {
			return err
		}
		if !confirmed {
			return &WizardCancelledError{}
		}

		// ---------- 保存配置 ----------
		if err := applyWizardConfig(cfg, providerID, apiKey, selectedModel, deps); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}

		log.Info("wizard: configuration saved",
			"provider", providerID,
			"model", selectedModel,
		)

		// ---------- S1-2: 完成后提供高级配置入口 ----------
		nextAction, err := prompter.Select(
			fmt.Sprintf("✅ Setup complete! Using %s with model %s.",
				selectedInfo.Label, selectedModel),
			[]WizardStepOption{
				{Value: "done", Label: "Start Using", Hint: "Begin using OpenAcosmi now"},
				{Value: "advanced", Label: "Continue to Advanced Configuration", Hint: "Configure network, channels, skills, hooks"},
			},
			"done",
		)
		if err != nil {
			return err
		}

		if fmt.Sprint(nextAction) == "advanced" {
			log.Info("wizard: user chose advanced configuration, skipping to phase 8")
			advancedRunner := RunOnboardingWizardAdvanced(deps, 8)
			return advancedRunner(prompter)
		}

		if err := prompter.Outro(
			"You can access advanced settings anytime from the Config tab or Security tab.",
		); err != nil {
			return err
		}

		return nil
	}
}

// ---------- 辅助函数 ----------

// wizardModelFallbacks 向导模型选择的静态回退列表。
// 当 ModelCatalog 中无 provider 模型时使用（首次配置场景）。
var wizardModelFallbacks = map[string][]WizardStepOption{
	"google": {
		{Value: "gemini-3-flash-preview", Label: "Gemini 3 Flash", Hint: "1000k context, fast"},
		{Value: "gemini-3-pro-preview", Label: "Gemini 3 Pro", Hint: "1000k context"},
		{Value: "gemini-3.1-pro-preview", Label: "Gemini 3.1 Pro", Hint: "2000k context"},
	},
	"openai": {
		{Value: "gpt-5.2", Label: "GPT-5.2", Hint: "latest"},
		{Value: "o3", Label: "o3", Hint: "reasoning"},
		{Value: "o4-mini", Label: "o4-mini", Hint: "reasoning, fast"},
	},
	"anthropic": {
		{Value: "claude-sonnet-4-20250514", Label: "Claude Sonnet 4", Hint: "balanced"},
		{Value: "claude-opus-4-20250514", Label: "Claude Opus 4", Hint: "most capable"},
	},
	"deepseek": {
		{Value: "deepseek-chat", Label: "DeepSeek V3.2", Hint: "128k context"},
		{Value: "deepseek-reasoner", Label: "DeepSeek V3.2 Reasoning", Hint: "128k context, thinking"},
	},
}

// resolveModelOptions 解析可用模型选项列表。
func resolveModelOptions(providerID, defaultModel string, catalog *models.ModelCatalog) []WizardStepOption {
	var options []WizardStepOption
	seen := make(map[string]bool)

	// 1. 从 ModelCatalog 获取
	if catalog != nil {
		entries := catalog.All()
		for _, e := range entries {
			if strings.EqualFold(e.Provider, providerID) {
				if seen[e.ID] {
					continue
				}
				seen[e.ID] = true
				hint := ""
				if e.ContextWindow != nil && *e.ContextWindow > 0 {
					hint = fmt.Sprintf("%dk context", *e.ContextWindow/1000)
				}
				if e.Reasoning != nil && *e.Reasoning {
					if hint != "" {
						hint += ", "
					}
					hint += "reasoning"
				}
				options = append(options, WizardStepOption{
					Value: e.ID,
					Label: e.Name,
					Hint:  hint,
				})
			}
		}
	}

	// 2. 如果 catalog 为空，使用静态回退列表
	if len(options) == 0 {
		if fallbacks, ok := wizardModelFallbacks[strings.ToLower(providerID)]; ok {
			options = append(options, fallbacks...)
			for _, opt := range fallbacks {
				seen[fmt.Sprintf("%v", opt.Value)] = true
			}
		}
	}

	// 3. 如果仍为空，使用 provider 默认值
	if len(options) == 0 && defaultModel != "" {
		if !seen[defaultModel] {
			options = append(options, WizardStepOption{
				Value: defaultModel,
				Label: defaultModel,
				Hint:  "provider default",
			})
		}
	}

	// 3. 排序: 默认模型优先
	sort.Slice(options, func(i, j int) bool {
		iDefault := options[i].Value == defaultModel
		jDefault := options[j].Value == defaultModel
		if iDefault != jDefault {
			return iDefault
		}
		li := options[i].Label
		lj := options[j].Label
		if li == "" {
			li = fmt.Sprintf("%v", options[i].Value)
		}
		if lj == "" {
			lj = fmt.Sprintf("%v", options[j].Value)
		}
		return li < lj
	})

	return options
}

// maskAPIKey 对 API key 进行掩码处理。
func maskAPIKey(key string) string {
	if key == "" {
		return "(from environment)"
	}
	if len(key) <= 8 {
		return strings.Repeat("•", len(key))
	}
	return key[:4] + strings.Repeat("•", len(key)-8) + key[len(key)-4:]
}

// applyWizardConfig 将向导结果写入配置文件。
func applyWizardConfig(
	cfg *types.OpenAcosmiConfig,
	providerID, apiKey, model string,
	deps WizardOnboardingDeps,
) error {
	// 设置 agents.defaults.model.primary
	if cfg.Agents == nil {
		cfg.Agents = &types.AgentsConfig{}
	}
	if cfg.Agents.Defaults == nil {
		cfg.Agents.Defaults = &types.AgentDefaultsConfig{}
	}
	if cfg.Agents.Defaults.Model == nil {
		cfg.Agents.Defaults.Model = &types.AgentModelListConfig{}
	}
	// 保存完整的 provider/model 格式，确保 ResolveConfiguredModelRef 能正确解析 provider
	if providerID != "" && !strings.Contains(model, "/") {
		cfg.Agents.Defaults.Model.Primary = providerID + "/" + model
	} else {
		cfg.Agents.Defaults.Model.Primary = model
	}

	// 设置 models.providers[providerID].apiKey（仅当非环境变量来源时）
	if apiKey != "" && models.ResolveEnvApiKeyWithFallback(providerID) != apiKey {
		if cfg.Models == nil {
			cfg.Models = &types.ModelsConfig{}
		}
		if cfg.Models.Providers == nil {
			cfg.Models.Providers = make(map[string]*types.ModelProviderConfig)
		}
		providerCfg := cfg.Models.Providers[providerID]
		if providerCfg == nil {
			providerCfg = &types.ModelProviderConfig{}
		}
		providerCfg.APIKey = apiKey
		// 填充 baseUrl（如果缺失）
		if providerCfg.BaseURL == "" {
			if defaults := models.GetProviderDefaults(providerID); defaults != nil && defaults.BaseURL != "" {
				providerCfg.BaseURL = defaults.BaseURL
			}
		}
		cfg.Models.Providers[providerID] = providerCfg
	}

	// 记录 wizard 元数据
	if cfg.Wizard == nil {
		cfg.Wizard = &types.OpenAcosmiWizardConfig{}
	}
	cfg.Wizard.LastRunCommand = "setup"
	cfg.Wizard.LastRunMode = "local"

	// 保存配置
	if deps.ConfigLoader != nil {
		if err := deps.ConfigLoader.WriteConfigFile(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
	}

	// 标记 setup 完成
	if deps.State != nil {
		deps.State.SetPhase(BootPhaseReady)
	}

	return nil
}

// ---------- 高级版引导流程 ----------

// RunOnboardingWizardAdvanced 执行完整 12 阶段引导流程。
// 对应 TS onboarding.ts runOnboardingWizard (471L)。
// startFromPhase (可选): 指定从第几阶段开始执行（跳过之前的阶段）。
// 用于简化向导完成后直接跳转到高级配置的后续阶段。
func RunOnboardingWizardAdvanced(deps WizardOnboardingDeps, startFromPhase ...int) WizardRunnerFunc {
	skipTo := 1
	if len(startFromPhase) > 0 && startFromPhase[0] > 1 {
		skipTo = startFromPhase[0]
	}

	return func(prompter WizardPrompter) error {
		log := slog.Default().With("subsystem", "wizard-onboarding-advanced")

		// 加载现有配置
		var baseConfig *types.OpenAcosmiConfig
		if deps.ConfigLoader != nil {
			if loaded, err := deps.ConfigLoader.LoadConfig(); err == nil {
				baseConfig = loaded
			}
		}
		if baseConfig == nil {
			baseConfig = &types.OpenAcosmiConfig{}
		}

		flow := WizardFlowAdvanced
		var providerID, apiKey string

		// ========== Phase 1: Intro + 安全确认 ==========
		if skipTo <= 1 {
			if err := prompter.Intro(i18n.Tp("onboard.title")); err != nil {
				return err
			}

			if err := requireRiskAcknowledgement(prompter); err != nil {
				return err
			}
		}

		// ========== Phase 2: 流程选择 ==========
		if skipTo <= 2 {
			var err error
			flow, err = selectWizardFlow(prompter)
			if err != nil {
				return err
			}
			log.Info("wizard: flow selected", "flow", flow)
		}

		// ========== Phase 3: 已有配置处理 ==========
		if skipTo <= 3 {
			hasExisting := deps.ConfigLoader != nil && baseConfig.Agents != nil
			if hasExisting {
				action, err := prompter.Select(
					i18n.Tp("onboard.config.action"),
					[]WizardStepOption{
						{Value: "keep", Label: "Use existing values"},
						{Value: "modify", Label: "Update values"},
						{Value: "reset", Label: "Reset"},
					},
					"keep",
				)
				if err != nil {
					return err
				}
				if fmt.Sprint(action) == "reset" {
					// Reset 仅清除 models/agents/auth 配置，保留 channels 配置，
					// 避免已配置的飞书/钉钉/企微等频道在重置 API Key 后意外丢失。
					savedChannels := baseConfig.Channels
					baseConfig = &types.OpenAcosmiConfig{}
					baseConfig.Channels = savedChannels
				}
			}
		}

		// ========== Phase 4: 模式选择 (local/remote) ==========
		mode := "local"
		if skipTo <= 4 {
			if flow == WizardFlowAdvanced {
				modeVal, err := prompter.Select(
					"What do you want to set up?",
					[]WizardStepOption{
						{Value: "local", Label: "Local gateway (this machine)"},
						{Value: "remote", Label: "Remote gateway (info-only)"},
					},
					"local",
				)
				if err != nil {
					return err
				}
				mode = fmt.Sprint(modeVal)
			}

			if mode == "remote" {
				// Remote 模式：仅配置远程 URL + token
				urlInput, err := prompter.Text(
					"Remote gateway URL",
					"ws://192.168.1.100:19001",
					"",
					false,
				)
				if err != nil {
					return err
				}
				tokenInput, err := prompter.Text(
					"Remote gateway token",
					"",
					"",
					true,
				)
				if err != nil {
					return err
				}
				baseConfig.Gateway = &types.GatewayConfig{
					Mode: "remote",
					Remote: &types.GatewayRemoteConfig{
						URL:   strings.TrimSpace(urlInput),
						Token: strings.TrimSpace(tokenInput),
					},
				}
				if err := saveWizardConfig(baseConfig, deps); err != nil {
					return err
				}
				return prompter.Outro("Remote gateway configured.")
			}
		}

		// ========== Phase 5: Workspace ==========
		nextConfig := &types.OpenAcosmiConfig{}
		*nextConfig = *baseConfig
		if nextConfig.Agents == nil {
			nextConfig.Agents = &types.AgentsConfig{}
		}
		if nextConfig.Agents.Defaults == nil {
			nextConfig.Agents.Defaults = &types.AgentDefaultsConfig{}
		}
		if nextConfig.Gateway == nil {
			nextConfig.Gateway = &types.GatewayConfig{}
		}
		nextConfig.Gateway.Mode = "local"

		if skipTo <= 5 {
			workspace := deps.WorkspaceDir
			if workspace == "" {
				workspace = "agents"
			}
			if flow == WizardFlowAdvanced {
				wsInput, err := prompter.Text(
					"Workspace directory",
					"agents",
					workspace,
					false,
				)
				if err != nil {
					return err
				}
				if t := strings.TrimSpace(wsInput); t != "" {
					workspace = t
				}
			}
			nextConfig.Agents.Defaults.Workspace = workspace
		}

		// ========== Phase 6: 认证选择 ==========
		if skipTo <= 6 {
			authResult, err := runWizardAuthPhase(prompter, true)
			if err != nil {
				return err
			}
			if authResult.Skipped {
				_ = prompter.Note("Auth setup skipped. You can configure it later.", "Auth")
			} else {
				providerID = authResult.ProviderID
				apiKey, err = runWizardAPIKeyInput(prompter, providerID)
				if err != nil {
					return err
				}
				log.Info("wizard: auth configured", "provider", providerID)
			}
		}

		// ========== Phase 7: 模型选择 ==========
		if skipTo <= 7 {
			if providerID != "" {
				var defaultModel string
				for _, p := range getWizardProviders() {
					if p.ID == providerID {
						defaultModel = p.DefaultModel
						break
					}
				}
				modelOptions := resolveModelOptions(providerID, defaultModel, deps.ModelCatalog)
				var selectedModel string
				if len(modelOptions) > 0 {
					sel, err := prompter.Select(
						i18n.Tp("onboard.model.select"),
						modelOptions,
						defaultModel,
					)
					if err != nil {
						return err
					}
					if s, ok := sel.(string); ok {
						selectedModel = s
					}
				}
				if selectedModel == "" {
					selectedModel = defaultModel
				}
				if selectedModel == "" {
					selectedModel = models.DefaultModel
				}
				// 写入认证+模型配置
				if err := applyWizardConfig(nextConfig, providerID, apiKey, selectedModel, deps); err != nil {
					return fmt.Errorf("apply auth config: %w", err)
				}
				log.Info("wizard: model selected", "model", selectedModel)
			}
		}

		// ========== Phase 8: Gateway 网络配置 ==========
		if skipTo == 8 {
			_ = prompter.Note(
				"Continuing with advanced configuration: network, channels, skills, hooks.",
				"Advanced Setup",
			)
		}
		gwResult, err := ConfigureGatewayForOnboarding(ConfigureGatewayOptions{
			Flow:       string(flow),
			BaseConfig: baseConfig,
			NextConfig: nextConfig,
			LocalPort:  DefaultGatewayPort,
			Prompter:   prompter,
		})
		if err != nil {
			return fmt.Errorf("gateway config: %w", err)
		}
		nextConfig = gwResult.NextConfig

		// ========== Phase 9: 频道配置（可选） ==========
		if deps.ChannelSetupFn != nil {
			updated, err := deps.ChannelSetupFn(nextConfig, prompter)
			if err != nil {
				log.Warn("channel setup", "error", err)
			} else if updated != nil {
				nextConfig = updated
			}
		}

		// ========== Phase 10: 技能配置（可选） ==========
		if deps.SkillsSetupFn != nil {
			updated, err := deps.SkillsSetupFn(nextConfig, prompter)
			if err != nil {
				log.Warn("skills setup", "error", err)
			} else if updated != nil {
				nextConfig = updated
			}
		}

		// ========== Phase 11: Hooks 配置（可选） ==========
		if deps.HooksSetupFn != nil {
			updated, err := deps.HooksSetupFn(nextConfig, prompter)
			if err != nil {
				log.Warn("hooks setup", "error", err)
			} else if updated != nil {
				nextConfig = updated
			}
		}

		// 保存配置
		if err := saveWizardConfig(nextConfig, deps); err != nil {
			return err
		}

		// ========== Phase 12: 完成阶段 ==========
		_, err = FinalizeOnboardingWizard(FinalizeOnboardingOptions{
			Flow:       string(flow),
			BaseConfig: baseConfig,
			NextConfig: nextConfig,
			Settings:   gwResult.Settings,
			Prompter:   prompter,
		})
		if err != nil {
			log.Warn("finalize", "error", err)
		}

		return nil
	}
}

// ---------- 高级版辅助函数 ----------

// requireRiskAcknowledgement 安全风险确认。
// 对应 TS requireRiskAcknowledgement (onboarding.ts L46-87)。
func requireRiskAcknowledgement(prompter WizardPrompter) error {
	if err := prompter.Note(
		"Security warning — please read.\n\n"+
			"OpenAcosmi can read files and run actions if tools are enabled.\n"+
			"A bad prompt can trick it into doing unsafe things.\n\n"+
			"Recommended baseline:\n"+
			"- Pairing/allowlists + mention gating.\n"+
			"- Sandbox + least-privilege tools.\n"+
			"- Keep secrets out of the agent's reachable filesystem.\n\n"+
			"Must read: docs/skills/general/security/SKILL.md",
		"Security",
	); err != nil {
		return err
	}

	ok, err := prompter.Confirm(
		"I understand this is powerful and inherently risky. Continue?",
		false,
	)
	if err != nil {
		return err
	}
	if !ok {
		return &WizardCancelledError{}
	}
	return nil
}

// selectWizardFlow 选择向导流程模式。
func selectWizardFlow(prompter WizardPrompter) (WizardFlow, error) {
	selected, err := prompter.Select(
		"Onboarding mode",
		[]WizardStepOption{
			{Value: "quickstart", Label: "QuickStart", Hint: "Configure details later via openacosmi configure."},
			{Value: "advanced", Label: "Manual", Hint: "Configure port, network, Tailscale, and auth options."},
		},
		"quickstart",
	)
	if err != nil {
		return "", err
	}
	if fmt.Sprint(selected) == "advanced" {
		return WizardFlowAdvanced, nil
	}
	return WizardFlowQuickstart, nil
}

// saveWizardConfig 保存配置到文件并标记完成。
func saveWizardConfig(cfg *types.OpenAcosmiConfig, deps WizardOnboardingDeps) error {
	// 记录 wizard 元数据
	if cfg.Wizard == nil {
		cfg.Wizard = &types.OpenAcosmiWizardConfig{}
	}
	cfg.Wizard.LastRunCommand = "setup"
	cfg.Wizard.LastRunMode = "local"

	if deps.ConfigLoader != nil {
		if err := deps.ConfigLoader.WriteConfigFile(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
	}
	if deps.State != nil {
		deps.State.SetPhase(BootPhaseReady)
	}
	return nil
}
