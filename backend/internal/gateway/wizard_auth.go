package gateway

// wizard_auth.go — 高级版向导认证阶段
// 将认证选择逻辑从 cmd 层下沉到 gateway 包，
// 通过 WizardPrompter 接口实现 Web UI 交互。

import (
	"fmt"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/agents/models"
	"github.com/Acosmi/ClawAcosmi/pkg/i18n"
)

// ---------- 认证选择 ----------

// WizardAuthGroup 认证提供商分组。
type WizardAuthGroup struct {
	Value   string
	Label   string
	Hint    string
	Methods []WizardAuthMethod
}

// WizardAuthMethod 认证方式。
type WizardAuthMethod struct {
	Value string
	Label string
	Hint  string
}

// getWizardAuthGroups 获取所有认证提供商分组。
// 对应 cmd 层 setup_auth_options.go 的 authChoiceGroupDefs
func getWizardAuthGroups() []WizardAuthGroup {
	return []WizardAuthGroup{
		{"openai", "OpenAI", "API key", []WizardAuthMethod{
			{"openai-api-key", "OpenAI API key", ""},
		}},
		{"anthropic", "Anthropic", "setup-token + API key", []WizardAuthMethod{
			{"token", "Anthropic token (paste setup-token)", "run `openacosmi setup-token` elsewhere, then paste the token here"},
			{"apiKey", "Anthropic API key", ""},
		}},
		{"minimax", "MiniMax", "M2.1 (recommended)", []WizardAuthMethod{
			{"minimax-portal", "MiniMax OAuth", "OAuth plugin for MiniMax"},
			{"minimax-api", "MiniMax M2.1", ""},
			{"minimax-api-lightning", "MiniMax M2.1 Lightning", "Faster, higher output cost"},
		}},
		{"moonshot", "Moonshot AI (Kimi K2.5)", "Kimi K2.5 + Kimi Coding", []WizardAuthMethod{
			{"moonshot-api-key", "Kimi API key (.ai)", ""},
			{"moonshot-api-key-cn", "Kimi API key (.cn)", ""},
			{"kimi-code-api-key", "Kimi Code API key (subscription)", ""},
		}},
		{"google", "Google", "Gemini API key + OAuth", []WizardAuthMethod{
			{"gemini-api-key", "Google Gemini API key", ""},
			{"google-antigravity", "Google Antigravity OAuth", "Uses the bundled Antigravity auth plugin"},
			{"google-gemini-cli", "Google Gemini CLI OAuth", "Uses the bundled Gemini CLI auth plugin"},
		}},
		{"xai", "xAI (Grok)", "API key", []WizardAuthMethod{
			{"xai-api-key", "xAI (Grok) API key", ""},
		}},
		{"openrouter", "OpenRouter", "API key", []WizardAuthMethod{
			{"openrouter-api-key", "OpenRouter API key", ""},
		}},
		{"qwen", "Qwen", "OAuth", []WizardAuthMethod{
			{"qwen-portal", "Qwen OAuth", ""},
		}},
		{"zai", "Z.AI (GLM 4.7)", "API key", []WizardAuthMethod{
			{"zai-api-key", "Z.AI (GLM 4.7) API key", ""},
		}},
		{"qianfan", "Qianfan", "API key", []WizardAuthMethod{
			{"qianfan-api-key", "Qianfan API key", ""},
		}},
		{"copilot", "Copilot", "GitHub + local proxy", []WizardAuthMethod{
			{"github-copilot", "GitHub Copilot (GitHub device login)", "Uses GitHub device flow"},
			{"copilot-proxy", "Copilot Proxy (local)", "Local proxy for VS Code Copilot models"},
		}},
		{"ai-gateway", "Vercel AI Gateway", "API key", []WizardAuthMethod{
			{"ai-gateway-api-key", "Vercel AI Gateway API key", ""},
		}},
		{"openacosmi-zen", "OpenAcosmi Zen", "API key", []WizardAuthMethod{
			{"openacosmi-zen", "OpenAcosmi Zen (multi-model proxy)", "Claude, GPT, Gemini via openacosmi.com/zen"},
		}},
		{"xiaomi", "Xiaomi", "API key", []WizardAuthMethod{
			{"xiaomi-api-key", "Xiaomi API key", ""},
		}},
		{"synthetic", "Synthetic", "Anthropic-compatible (multi-model)", []WizardAuthMethod{
			{"synthetic-api-key", "Synthetic API key", ""},
		}},
		{"venice", "Venice AI", "Privacy-focused (uncensored models)", []WizardAuthMethod{
			{"venice-api-key", "Venice AI API key", "Privacy-focused inference (uncensored models)"},
		}},
		{"cloudflare-ai-gateway", "Cloudflare AI Gateway", "Account ID + Gateway ID + API key", []WizardAuthMethod{
			{"cloudflare-ai-gateway-api-key", "Cloudflare AI Gateway", "Account ID + Gateway ID + API key"},
		}},
	}
}

// WizardAuthResult 认证阶段结果。
type WizardAuthResult struct {
	AuthChoice string // 选中的认证方式 value
	ProviderID string // 推断的 provider ID
	APIKey     string // 用户输入的 API Key（如果是 API key 类型的认证方式）
	Skipped    bool   // 是否跳过
}

// runWizardAuthPhase 执行两级分组认证选择。
// 对应 TS promptAuthChoiceGrouped (auth-choice-prompt.ts)
func runWizardAuthPhase(prompter WizardPrompter, includeSkip bool) (*WizardAuthResult, error) {
	groups := getWizardAuthGroups()
	backValue := "__back"

	for {
		// 第一级：选择提供商组
		providerOptions := make([]WizardStepOption, 0, len(groups)+1)
		for _, g := range groups {
			providerOptions = append(providerOptions, WizardStepOption{
				Value: g.Value,
				Label: g.Label,
				Hint:  g.Hint,
			})
		}
		if includeSkip {
			providerOptions = append(providerOptions, WizardStepOption{
				Value: "skip",
				Label: "Skip for now",
			})
		}

		providerSelection, err := prompter.Select(
			i18n.Tp("onboard.auth.provider_select"),
			providerOptions,
			"",
		)
		if err != nil {
			return nil, fmt.Errorf("provider selection: %w", err)
		}

		selStr := fmt.Sprint(providerSelection)
		if selStr == "skip" {
			return &WizardAuthResult{Skipped: true}, nil
		}

		// 查找选中的组
		var selectedGroup *WizardAuthGroup
		for i := range groups {
			if groups[i].Value == selStr {
				selectedGroup = &groups[i]
				break
			}
		}
		if selectedGroup == nil || len(selectedGroup.Methods) == 0 {
			_ = prompter.Note(i18n.Tp("onboard.auth.no_methods"), i18n.Tp("onboard.auth.title"))
			continue
		}

		// 第二级：选择认证方式
		methodOptions := make([]WizardStepOption, 0, len(selectedGroup.Methods)+1)
		for _, m := range selectedGroup.Methods {
			methodOptions = append(methodOptions, WizardStepOption{
				Value: m.Value,
				Label: m.Label,
				Hint:  m.Hint,
			})
		}
		methodOptions = append(methodOptions, WizardStepOption{Value: backValue, Label: "Back"})

		methodSelection, err := prompter.Select(
			i18n.Tp("onboard.auth.method_select"),
			methodOptions,
			"",
		)
		if err != nil {
			return nil, fmt.Errorf("method selection: %w", err)
		}

		methodStr := fmt.Sprint(methodSelection)
		if methodStr == backValue {
			continue
		}

		// 推断 provider ID
		providerID := resolveProviderForAuthChoice(methodStr)

		return &WizardAuthResult{
			AuthChoice: methodStr,
			ProviderID: providerID,
		}, nil
	}
}

// runWizardAPIKeyInput 在选定 provider 后输入 API Key。
func runWizardAPIKeyInput(prompter WizardPrompter, providerID string) (string, error) {
	envVar := models.ResolveEnvApiKeyVarName(providerID)
	existingKey := models.ResolveEnvApiKeyWithFallback(providerID)

	if existingKey != "" {
		useExisting, err := prompter.Confirm(
			i18n.Tf("onboard.provider.env_detected", envVar),
			true,
		)
		if err != nil {
			return "", err
		}
		if useExisting {
			return existingKey, nil
		}
	}

	inputKey, err := prompter.Text(
		i18n.Tp("onboard.apikey.prompt"),
		fmt.Sprintf("sk-... or %s", envVar),
		"",
		true,
	)
	if err != nil {
		return "", err
	}
	apiKey := strings.TrimSpace(inputKey)
	if apiKey == "" && providerID != "ollama" {
		return "", fmt.Errorf("API key is required for %s", providerID)
	}
	return apiKey, nil
}

// resolveProviderForAuthChoice 从认证方式推断 provider ID。
func resolveProviderForAuthChoice(authChoice string) string {
	mapping := map[string]string{
		"apiKey":                        "anthropic",
		"token":                         "anthropic",
		"openai-api-key":                "openai",
		"gemini-api-key":                "google",
		"google-antigravity":            "google",
		"google-gemini-cli":             "google",
		"xai-api-key":                   "xai",
		"openrouter-api-key":            "openrouter",
		"ai-gateway-api-key":            "ai-gateway",
		"cloudflare-ai-gateway-api-key": "cloudflare",
		"moonshot-api-key":              "moonshot",
		"moonshot-api-key-cn":           "moonshot",
		"kimi-code-api-key":             "moonshot",
		"synthetic-api-key":             "synthetic",
		"venice-api-key":                "venice",
		"github-copilot":                "copilot",
		"copilot-proxy":                 "copilot",
		"zai-api-key":                   "zai",
		"xiaomi-api-key":                "xiaomi",
		"minimax-portal":                "minimax",
		"minimax-api":                   "minimax",
		"minimax-api-lightning":         "minimax",
		"qwen-portal":                   "qwen",
		"openacosmi-zen":                "openacosmi-zen",
		"qianfan-api-key":               "qianfan",
	}
	if p, ok := mapping[authChoice]; ok {
		return p
	}
	return "anthropic"
}
