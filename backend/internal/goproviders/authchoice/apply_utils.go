// authchoice/apply_utils.go — 各 Provider apply 文件的共享工具函数
// 本文件提供参数提取和辅助函数，避免重复代码。
package authchoice

import "github.com/Acosmi/ClawAcosmi/internal/goproviders/types"

// strPtr 返回字符串指针。
func strPtr(s string) *string {
	return &s
}

// getSecretInputMode 安全获取 OnboardOptions 中的 SecretInputMode。
func getSecretInputMode(opts *types.OnboardOptions) *types.SecretInputMode {
	if opts == nil || opts.SecretInputMode == nil {
		return nil
	}
	return opts.SecretInputMode
}

// getOptToken 安全获取 OnboardOptions 中的 Token 字段。
// fieldName 参数标识从哪个字段提取（"token" 为通用 Token，其他为 Provider 特定）。
func getOptToken(opts *types.OnboardOptions, fieldName string) *string {
	if opts == nil {
		return nil
	}
	switch fieldName {
	case "token":
		return opts.Token
	case "volcengineApiKey":
		return opts.VolcengineAPIKey
	case "byteplusApiKey":
		return opts.BytePlusAPIKey
	case "xaiApiKey":
		return opts.XAIAPIKey
	default:
		return opts.Token
	}
}

// getOptTokenProvider 安全获取 OnboardOptions 中的 TokenProvider。
func getOptTokenProvider(opts *types.OnboardOptions) *string {
	if opts == nil {
		return nil
	}
	return opts.TokenProvider
}

// ──────────────────────────────────────────────
// 窗口 6 新增 stub 函数
// 以下函数在窗口 6 新文件中使用，但实际实现在窗口 7/8。
// ──────────────────────────────────────────────

// EnablePluginInConfig 在配置中启用插件。
// stub：假设所有插件都已启用。供窗口 7/8 补全。
func EnablePluginInConfig(config OpenClawConfig, pluginID string) (bool, string) {
	return true, ""
}

// PluginProvider 插件 Provider 定义。
type PluginProvider struct {
	ID   string
	Auth []PluginAuthMethod
}

// PluginAuthMethod 插件认证方法。
type PluginAuthMethod struct {
	ID  string
	Run func(params PluginAuthRunParams) (PluginProviderAuthRunResult, error)
}

// PluginAuthRunParams 插件认证运行参数。
type PluginAuthRunParams struct {
	Config       OpenClawConfig
	AgentDir     string
	WorkspaceDir string
	Prompter     WizardPrompter
	Runtime      RuntimeEnv
	IsRemote     bool
}

// ResolvePluginProvider 解析插件 Provider。
// stub：供窗口 7/8 补全。
func ResolvePluginProvider(config OpenClawConfig, providerID string) *PluginProvider {
	// stub：返回一个最小可用的 provider
	return &PluginProvider{
		ID: providerID,
		Auth: []PluginAuthMethod{
			{ID: "default"},
		},
	}
}

// RunPluginAuthMethodParams 运行插件认证方法的参数。
type RunPluginAuthMethodParams struct {
	Config     OpenClawConfig
	AgentDir   string
	Prompter   WizardPrompter
	Runtime    RuntimeEnv
	IsRemote   bool
	ProviderID string
	MethodID   string
	Label      string
}

// RunPluginAuthMethod 运行插件认证方法。
// stub：供窗口 7/8 补全。
func RunPluginAuthMethod(params RunPluginAuthMethodParams) (PluginProviderAuthRunResult, error) {
	return PluginProviderAuthRunResult{}, nil
}

// MergeConfigPatch 合并配置补丁。
// stub：供窗口 7/8 补全。
func MergeConfigPatch(base OpenClawConfig, patch OpenClawConfig) OpenClawConfig {
	if patch == nil {
		return base
	}
	result := make(OpenClawConfig)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range patch {
		result[k] = v
	}
	return result
}

// ApplyDefaultModel 应用默认模型到配置。
// stub：供窗口 7/8 补全。
func ApplyDefaultModel(config OpenClawConfig, modelRef string) OpenClawConfig {
	return ApplyPrimaryModel(config, modelRef)
}

// GitHubCopilotLoginCommand 执行 GitHub Copilot 登录命令。
// stub：供窗口 7/8 补全。
func GitHubCopilotLoginCommand(runtime RuntimeEnv) error {
	return nil
}

// PromptAndConfigureVllmParams vLLM 配置参数。
type PromptAndConfigureVllmParams struct {
	Cfg      OpenClawConfig
	Prompter WizardPrompter
	AgentDir string
}

// PromptAndConfigureVllm 交互式配置 vLLM。
// stub：供窗口 7/8 补全。
func PromptAndConfigureVllm(params PromptAndConfigureVllmParams) (OpenClawConfig, string, error) {
	return params.Cfg, "vllm/default-model", nil
}

// LoginOpenAICodexOAuthParams OpenAI Codex OAuth 登录参数。
type LoginOpenAICodexOAuthParams struct {
	Prompter            WizardPrompter
	Runtime             RuntimeEnv
	IsRemote            bool
	OpenURL             func(string) error
	LocalBrowserMessage string
}

// LoginOpenAICodexOAuth 执行 OpenAI Codex OAuth 登录。
// stub：供窗口 7/8 补全。
func LoginOpenAICodexOAuth(params LoginOpenAICodexOAuthParams) (interface{}, error) {
	return nil, nil
}

// HuggingfaceModel HuggingFace 模型信息。
type HuggingfaceModel struct {
	ID   string
	Name string
}

// DiscoverHuggingfaceModels 发现 HuggingFace 可用模型。
// stub：供窗口 7/8 补全。
func DiscoverHuggingfaceModels(apiKey string) []HuggingfaceModel {
	return nil
}

// IsHuggingfacePolicyLocked 检查 HuggingFace 模型是否策略锁定。
// stub：供窗口 7/8 补全。
func IsHuggingfacePolicyLocked(modelRef string) bool {
	return false
}
