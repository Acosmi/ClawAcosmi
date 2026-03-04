// authchoice/apply_helpers.go — Apply 公共帮助函数
// 对应 TS 文件: src/commands/auth-choice.apply-helpers.ts
// 包含密钥引用交互、模型状态桥接、API Key 从选项/环境/提示获取等核心逻辑。
package authchoice

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// envSourceLabelRE 从 source 标签中提取环境变量名。
var envSourceLabelRE = regexp.MustCompile(`(?:^|:\s)([A-Z][A-Z0-9_]*)$`)

// envSecretRefIDRE 合法环境变量名模式。
var envSecretRefIDRE = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,127}$`)

// ──────────────────────────────────────────────
// 提示文案类型
// ──────────────────────────────────────────────

// SecretInputModePromptCopy 密钥输入模式提示文案。
type SecretInputModePromptCopy struct {
	ModeMessage    string
	PlaintextLabel string
	PlaintextHint  string
	RefLabel       string
	RefHint        string
}

// SecretRefOnboardingPromptCopy 密钥引用引导提示文案。
type SecretRefOnboardingPromptCopy struct {
	SourceMessage            string
	EnvVarMessage            string
	EnvVarPlaceholder        string
	EnvVarFormatError        string
	EnvVarMissingError       func(envVar string) string
	NoProvidersMessage       string
	EnvValidatedMessage      func(envVar string) string
	ProviderValidatedMessage func(provider, id, source string) string
}

// ──────────────────────────────────────────────
// 内部工具函数
// ──────────────────────────────────────────────

// formatErrorMessage 格式化错误信息。
func formatErrorMessage(err error) string {
	if err != nil && err.Error() != "" {
		return err.Error()
	}
	return fmt.Sprint(err)
}

// extractEnvVarFromSourceLabel 从 source 标签中提取环境变量名。
func extractEnvVarFromSourceLabel(source string) string {
	matches := envSourceLabelRE.FindStringSubmatch(strings.TrimSpace(source))
	if matches == nil {
		return ""
	}
	return matches[1]
}

// resolveDefaultProviderEnvVar 解析提供者默认环境变量。
func resolveDefaultProviderEnvVar(provider string) string {
	envVars, ok := ProviderEnvVars[provider]
	if !ok || len(envVars) == 0 {
		return ""
	}
	for _, v := range envVars {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// resolveDefaultFilePointerId 解析默认文件指针 ID。
func resolveDefaultFilePointerId(provider string) string {
	return "/providers/" + EncodeJsonPointerToken(provider) + "/apiKey"
}

// ──────────────────────────────────────────────
// 规范化工具
// ──────────────────────────────────────────────

// NormalizeTokenProviderInput 规范化 token provider 输入。
// 对应 TS: normalizeTokenProviderInput()
func NormalizeTokenProviderInput(tokenProvider *string) string {
	if tokenProvider == nil {
		return ""
	}
	normalized := strings.ToLower(strings.TrimSpace(*tokenProvider))
	return normalized
}

// NormalizeSecretInputModeInput 规范化密钥输入模式输入。
// 对应 TS: normalizeSecretInputModeInput()
func NormalizeSecretInputModeInput(mode *types.SecretInputMode) types.SecretInputMode {
	if mode == nil {
		return ""
	}
	normalized := types.SecretInputMode(strings.ToLower(strings.TrimSpace(string(*mode))))
	if normalized == types.SecretInputModePlaintext || normalized == types.SecretInputModeRef {
		return normalized
	}
	return ""
}

// ──────────────────────────────────────────────
// Agent Model 记录器
// ──────────────────────────────────────────────

// CreateAuthChoiceAgentModelNoter 创建 Agent 模型记录回调。
// 对应 TS: createAuthChoiceAgentModelNoter()
func CreateAuthChoiceAgentModelNoter(params ApplyAuthChoiceParams) func(model string) error {
	return func(model string) error {
		if params.AgentID == "" {
			return nil
		}
		return params.Prompter.Note(
			fmt.Sprintf("Default model set to %s for agent \"%s\".", model, params.AgentID),
			"Model configured",
		)
	}
}

// ──────────────────────────────────────────────
// 模型状态桥接
// ──────────────────────────────────────────────

// ApplyAuthChoiceModelState 模型状态结构。
// 对应 TS: ApplyAuthChoiceModelState
type ApplyAuthChoiceModelState struct {
	Config             OpenClawConfig
	AgentModelOverride string
}

// CreateDefaultModelApplierFunc 创建默认模型应用函数类型。
type CreateDefaultModelApplierFunc func(opts DefaultModelApplyOptions) error

// DefaultModelApplyOptions 默认模型应用选项。
type DefaultModelApplyOptions struct {
	DefaultModel        string
	ApplyDefaultConfig  func(OpenClawConfig) OpenClawConfig
	ApplyProviderConfig func(OpenClawConfig) OpenClawConfig
	NoteDefault         string
}

// CreateAuthChoiceDefaultModelApplier 创建认证选择默认模型应用器。
// 对应 TS: createAuthChoiceDefaultModelApplier()
func CreateAuthChoiceDefaultModelApplier(
	params ApplyAuthChoiceParams,
	state *ApplyAuthChoiceModelState,
) CreateDefaultModelApplierFunc {
	noteAgentModel := CreateAuthChoiceAgentModelNoter(params)

	return func(opts DefaultModelApplyOptions) error {
		applied, err := ApplyDefaultModelChoice(DefaultModelChoiceParams{
			Config:              state.Config,
			SetDefaultModel:     params.SetDefaultModel,
			DefaultModel:        opts.DefaultModel,
			ApplyDefaultConfig:  opts.ApplyDefaultConfig,
			ApplyProviderConfig: opts.ApplyProviderConfig,
			NoteDefault:         opts.NoteDefault,
			NoteAgentModel:      noteAgentModel,
			Prompter:            params.Prompter,
		})
		if err != nil {
			return err
		}
		state.Config = applied.Config
		if applied.AgentModelOverride != "" {
			state.AgentModelOverride = applied.AgentModelOverride
		}
		return nil
	}
}

// ──────────────────────────────────────────────
// 密钥输入模式选择
// ──────────────────────────────────────────────

// ResolveSecretInputModeForEnvSelection 解析密钥输入模式。
// 对应 TS: resolveSecretInputModeForEnvSelection()
func ResolveSecretInputModeForEnvSelection(
	prompter WizardPrompter,
	explicitMode types.SecretInputMode,
	copy *SecretInputModePromptCopy,
) (types.SecretInputMode, error) {
	if explicitMode != "" {
		return explicitMode, nil
	}
	if prompter == nil {
		return types.SecretInputModePlaintext, nil
	}

	modeMsg := "How do you want to provide this API key?"
	ptLabel := "Paste API key now"
	ptHint := "Stores the key directly in OpenClaw config"
	refLabel := "Use external secret provider"
	refHint := "Stores a reference to env or configured external secret providers"
	if copy != nil {
		if copy.ModeMessage != "" {
			modeMsg = copy.ModeMessage
		}
		if copy.PlaintextLabel != "" {
			ptLabel = copy.PlaintextLabel
		}
		if copy.PlaintextHint != "" {
			ptHint = copy.PlaintextHint
		}
		if copy.RefLabel != "" {
			refLabel = copy.RefLabel
		}
		if copy.RefHint != "" {
			refHint = copy.RefHint
		}
	}

	selected, err := prompter.Select(SelectPromptOptions{
		Message:      modeMsg,
		InitialValue: string(types.SecretInputModePlaintext),
		Options: []SelectOption{
			{Value: string(types.SecretInputModePlaintext), Label: ptLabel, Hint: ptHint},
			{Value: string(types.SecretInputModeRef), Label: refLabel, Hint: refHint},
		},
	})
	if err != nil {
		return types.SecretInputModePlaintext, err
	}
	if selected == string(types.SecretInputModeRef) {
		return types.SecretInputModeRef, nil
	}
	return types.SecretInputModePlaintext, nil
}

// ──────────────────────────────────────────────
// API Key 从选项获取
// ──────────────────────────────────────────────

// MaybeApplyApiKeyFromOptionParams 从选项应用 API Key 的参数。
type MaybeApplyApiKeyFromOptionParams struct {
	Token             *string
	TokenProvider     *string
	SecretInputMode   types.SecretInputMode
	ExpectedProviders []string
	Normalize         func(string) string
	SetCredential     func(apiKey types.SecretInput, mode types.SecretInputMode) error
}

// MaybeApplyApiKeyFromOption 尝试从命令行选项获取并应用 API Key。
// 对应 TS: maybeApplyApiKeyFromOption()
func MaybeApplyApiKeyFromOption(params MaybeApplyApiKeyFromOptionParams) (string, error) {
	tokenProvider := NormalizeTokenProviderInput(params.TokenProvider)
	expectedProviders := make([]string, 0, len(params.ExpectedProviders))
	for _, p := range params.ExpectedProviders {
		np := strings.ToLower(strings.TrimSpace(p))
		if np != "" {
			expectedProviders = append(expectedProviders, np)
		}
	}

	if params.Token == nil || *params.Token == "" || tokenProvider == "" {
		return "", nil
	}
	found := false
	for _, ep := range expectedProviders {
		if ep == tokenProvider {
			found = true
			break
		}
	}
	if !found {
		return "", nil
	}

	apiKey := params.Normalize(*params.Token)
	if err := params.SetCredential(apiKey, params.SecretInputMode); err != nil {
		return "", err
	}
	return apiKey, nil
}

// ──────────────────────────────────────────────
// API Key 从选项/环境/提示获取
// ──────────────────────────────────────────────

// EnsureApiKeyFromOptionEnvOrPromptParams 确保 API Key 的完整参数。
type EnsureApiKeyFromOptionEnvOrPromptParams struct {
	Token             *string
	TokenProvider     *string
	SecretInputMode   types.SecretInputMode
	Config            OpenClawConfig
	ExpectedProviders []string
	Provider          string
	EnvLabel          string
	PromptMessage     string
	Normalize         func(string) string
	Validate          func(string) string
	Prompter          WizardPrompter
	SetCredential     func(apiKey types.SecretInput, mode types.SecretInputMode) error
	NoteMessage       string
	NoteTitle         string
}

// EnsureApiKeyFromOptionEnvOrPrompt 确保 API Key（从选项、环境或交互提示获取）。
// 对应 TS: ensureApiKeyFromOptionEnvOrPrompt()
func EnsureApiKeyFromOptionEnvOrPrompt(params EnsureApiKeyFromOptionEnvOrPromptParams) (string, error) {
	// 先尝试从选项获取
	optionKey, err := MaybeApplyApiKeyFromOption(MaybeApplyApiKeyFromOptionParams{
		Token:             params.Token,
		TokenProvider:     params.TokenProvider,
		SecretInputMode:   params.SecretInputMode,
		ExpectedProviders: params.ExpectedProviders,
		Normalize:         params.Normalize,
		SetCredential:     params.SetCredential,
	})
	if err != nil {
		return "", err
	}
	if optionKey != "" {
		return optionKey, nil
	}

	// 显示提示信息
	if params.NoteMessage != "" && params.Prompter != nil {
		_ = params.Prompter.Note(params.NoteMessage, params.NoteTitle)
	}

	// 从环境或提示获取
	return EnsureApiKeyFromEnvOrPrompt(EnsureApiKeyFromEnvOrPromptParams{
		Config:          params.Config,
		Provider:        params.Provider,
		EnvLabel:        params.EnvLabel,
		PromptMessage:   params.PromptMessage,
		Normalize:       params.Normalize,
		Validate:        params.Validate,
		Prompter:        params.Prompter,
		SecretInputMode: params.SecretInputMode,
		SetCredential:   params.SetCredential,
	})
}

// EnsureApiKeyFromEnvOrPromptParams 从环境或提示获取 API Key 的参数。
type EnsureApiKeyFromEnvOrPromptParams struct {
	Config          OpenClawConfig
	Provider        string
	EnvLabel        string
	PromptMessage   string
	Normalize       func(string) string
	Validate        func(string) string
	Prompter        WizardPrompter
	SecretInputMode types.SecretInputMode
	SetCredential   func(apiKey types.SecretInput, mode types.SecretInputMode) error
}

// EnsureApiKeyFromEnvOrPrompt 从环境变量或交互提示获取 API Key。
// 对应 TS: ensureApiKeyFromEnvOrPrompt()
func EnsureApiKeyFromEnvOrPrompt(params EnsureApiKeyFromEnvOrPromptParams) (string, error) {
	selectedMode, err := ResolveSecretInputModeForEnvSelection(
		params.Prompter, params.SecretInputMode, nil,
	)
	if err != nil {
		return "", err
	}

	envKey := ResolveEnvApiKey(params.Provider)

	// ref 模式：使用密钥引用
	if selectedMode == types.SecretInputModeRef {
		if params.Prompter == nil {
			// 非交互：回退到 ref fallback
			preferredEnvVar := ""
			if envKey != nil {
				preferredEnvVar = extractEnvVarFromSourceLabel(envKey.Source)
			}
			fallbackRef, fallbackValue, fErr := resolveRefFallbackInput(params.Config, params.Provider, preferredEnvVar)
			if fErr != nil {
				return "", fErr
			}
			if err := params.SetCredential(fallbackRef, selectedMode); err != nil {
				return "", err
			}
			return fallbackValue, nil
		}
		preferredEnvVar := ""
		if envKey != nil {
			preferredEnvVar = extractEnvVarFromSourceLabel(envKey.Source)
		}
		ref, resolvedValue, pErr := PromptSecretRefForOnboarding(PromptSecretRefParams{
			Provider:        params.Provider,
			Config:          params.Config,
			Prompter:        params.Prompter,
			PreferredEnvVar: preferredEnvVar,
		})
		if pErr != nil {
			return "", pErr
		}
		if err := params.SetCredential(ref, selectedMode); err != nil {
			return "", err
		}
		return resolvedValue, nil
	}

	// plaintext 模式：从环境或提示获取
	if envKey != nil && selectedMode == types.SecretInputModePlaintext {
		useExisting, cErr := params.Prompter.Confirm(ConfirmPromptOptions{
			Message:      fmt.Sprintf("Use existing %s (%s, %s)?", params.EnvLabel, envKey.Source, FormatApiKeyPreview(envKey.ApiKey, 0, 0)),
			InitialValue: true,
		})
		if cErr != nil {
			return "", cErr
		}
		if useExisting {
			if err := params.SetCredential(envKey.ApiKey, selectedMode); err != nil {
				return "", err
			}
			return envKey.ApiKey, nil
		}
	}

	// 交互输入
	key, tErr := params.Prompter.Text(TextPromptOptions{
		Message:  params.PromptMessage,
		Validate: params.Validate,
	})
	if tErr != nil {
		return "", tErr
	}
	apiKey := params.Normalize(key)
	if err := params.SetCredential(apiKey, selectedMode); err != nil {
		return "", err
	}
	return apiKey, nil
}

// ──────────────────────────────────────────────
// 密钥引用交互提示
// ──────────────────────────────────────────────

// PromptSecretRefParams 密钥引用提示参数。
type PromptSecretRefParams struct {
	Provider        string
	Config          OpenClawConfig
	Prompter        WizardPrompter
	PreferredEnvVar string
	Copy            *SecretRefOnboardingPromptCopy
}

// PromptSecretRefForOnboarding 交互式引导密钥引用配置。
// 对应 TS: promptSecretRefForOnboarding()
func PromptSecretRefForOnboarding(params PromptSecretRefParams) (types.SecretRef, string, error) {
	defaultEnvVar := params.PreferredEnvVar
	if defaultEnvVar == "" {
		defaultEnvVar = resolveDefaultProviderEnvVar(params.Provider)
	}
	defaultFilePointer := resolveDefaultFilePointerId(params.Provider)
	sourceChoice := "env"

	for {
		sourceMsg := "Where is this API key stored?"
		if params.Copy != nil && params.Copy.SourceMessage != "" {
			sourceMsg = params.Copy.SourceMessage
		}

		sourceRaw, err := params.Prompter.Select(SelectPromptOptions{
			Message:      sourceMsg,
			InitialValue: sourceChoice,
			Options: []SelectOption{
				{Value: "env", Label: "Environment variable", Hint: "Reference a variable from your runtime environment"},
				{Value: "provider", Label: "Configured secret provider", Hint: "Use a configured file or exec secret provider"},
			},
		})
		if err != nil {
			return types.SecretRef{}, "", err
		}
		source := "env"
		if sourceRaw == "provider" {
			source = "provider"
		}
		sourceChoice = source

		if source == "env" {
			envVarMsg := "Environment variable name"
			envVarPlaceholder := "OPENAI_API_KEY"
			if params.Copy != nil {
				if params.Copy.EnvVarMessage != "" {
					envVarMsg = params.Copy.EnvVarMessage
				}
				if params.Copy.EnvVarPlaceholder != "" {
					envVarPlaceholder = params.Copy.EnvVarPlaceholder
				}
			}

			envVarRaw, tErr := params.Prompter.Text(TextPromptOptions{
				Message:      envVarMsg,
				InitialValue: defaultEnvVar,
				Placeholder:  envVarPlaceholder,
				Validate: func(value string) string {
					candidate := strings.TrimSpace(value)
					if !envSecretRefIDRE.MatchString(candidate) {
						if params.Copy != nil && params.Copy.EnvVarFormatError != "" {
							return params.Copy.EnvVarFormatError
						}
						return `Use an env var name like "OPENAI_API_KEY" (uppercase letters, numbers, underscores).`
					}
					if strings.TrimSpace(os.Getenv(candidate)) == "" {
						if params.Copy != nil && params.Copy.EnvVarMissingError != nil {
							return params.Copy.EnvVarMissingError(candidate)
						}
						return fmt.Sprintf(`Environment variable "%s" is missing or empty in this session.`, candidate)
					}
					return ""
				},
			})
			if tErr != nil {
				return types.SecretRef{}, "", tErr
			}

			envCandidate := strings.TrimSpace(envVarRaw)
			envVar := defaultEnvVar
			if envCandidate != "" && envSecretRefIDRE.MatchString(envCandidate) {
				envVar = envCandidate
			}
			if envVar == "" {
				return types.SecretRef{}, "", fmt.Errorf("no valid environment variable name provided for provider %q", params.Provider)
			}

			ref := types.SecretRef{
				Source:   types.SecretRefSourceEnv,
				Provider: ResolveDefaultSecretProviderAlias(params.Config, "env", nil),
				ID:       envVar,
			}
			resolvedValue, rErr := ResolveSecretRefString(ref, nil)
			if rErr != nil {
				return types.SecretRef{}, "", rErr
			}

			noteMsg := fmt.Sprintf("Validated environment variable %s. OpenClaw will store a reference, not the key value.", envVar)
			if params.Copy != nil && params.Copy.EnvValidatedMessage != nil {
				noteMsg = params.Copy.EnvValidatedMessage(envVar)
			}
			_ = params.Prompter.Note(noteMsg, "Reference validated")
			return ref, resolvedValue, nil
		}

		// provider 模式：使用已配置的 secret provider
		// 简化处理：如果没有配置外部 provider，提示用户选环境变量
		_ = params.Prompter.Note(
			"No file/exec secret providers are configured yet. Add one under secrets.providers, or select Environment variable.",
			"No providers configured",
		)
		_ = defaultFilePointer // 保留引用
		continue
	}
}

// resolveRefFallbackInput 在非交互模式下回退到环境变量引用。
func resolveRefFallbackInput(config OpenClawConfig, provider, preferredEnvVar string) (types.SecretRef, string, error) {
	fallbackEnvVar := preferredEnvVar
	if fallbackEnvVar == "" {
		fallbackEnvVar = resolveDefaultProviderEnvVar(provider)
	}
	if fallbackEnvVar == "" {
		return types.SecretRef{}, "", fmt.Errorf(
			"no default environment variable mapping found for provider %q. Set a provider-specific env var, or re-run onboarding in an interactive terminal to configure a ref",
			provider,
		)
	}
	value := strings.TrimSpace(os.Getenv(fallbackEnvVar))
	if value == "" {
		return types.SecretRef{}, "", fmt.Errorf(
			"environment variable %q is required for --secret-input-mode ref in non-interactive onboarding",
			fallbackEnvVar,
		)
	}
	ref := types.SecretRef{
		Source:   types.SecretRefSourceEnv,
		Provider: ResolveDefaultSecretProviderAlias(config, "env", nil),
		ID:       fallbackEnvVar,
	}
	return ref, value, nil
}
