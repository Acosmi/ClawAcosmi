// authchoice/stubs.go — 占位接口和 stub 函数
// 本文件定义本窗口所需但尚未在其他窗口实现的外部依赖。
// 标记为「供窗口 7/8 补全」的项目将在后续窗口中替换为真实实现。
package authchoice

import (
	"github.com/Acosmi/ClawAcosmi/internal/goproviders/onboard"
	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// ──────────────────────────────────────────────
// 接口定义
// ──────────────────────────────────────────────

// WizardPrompter 向导交互提示器接口。
// 供窗口 7/8 补全真实实现。
type WizardPrompter interface {
	// Text 文本输入提示
	Text(opts TextPromptOptions) (string, error)
	// Select 单选提示
	Select(opts SelectPromptOptions) (string, error)
	// Confirm 确认提示
	Confirm(opts ConfirmPromptOptions) (bool, error)
	// Note 显示提示信息
	Note(message string, title string) error
	// Progress 显示进度
	Progress(message string) ProgressSpinner
}

// TextPromptOptions Text 提示选项。
type TextPromptOptions struct {
	Message      string
	InitialValue string
	Placeholder  string
	Validate     func(string) string
}

// SelectPromptOptions Select 提示选项。
type SelectPromptOptions struct {
	Message      string
	InitialValue string
	Options      []SelectOption
}

// SelectOption Select 选项项。
type SelectOption struct {
	Value string
	Label string
	Hint  string
}

// ConfirmPromptOptions Confirm 提示选项。
type ConfirmPromptOptions struct {
	Message      string
	InitialValue bool
}

// ProgressSpinner 进度旋转器接口。
type ProgressSpinner interface {
	Update(message string)
	Stop(message string)
}

// RuntimeEnv 运行时环境接口。
// 供窗口 7/8 补全真实实现。
type RuntimeEnv interface {
	Error(msg string)
	Log(msg string)
	Exit(code int)
}

// ──────────────────────────────────────────────
// OpenClawConfig 占位类型
// ──────────────────────────────────────────────

// OpenClawConfig 配置类型占位。
// 供窗口 7/8 替换为真实结构。
type OpenClawConfig = map[string]interface{}

// ──────────────────────────────────────────────
// ApiKeyStorageOptions 密钥存储选项
// ──────────────────────────────────────────────

// ApiKeyStorageOptions API 密钥存储选项。
type ApiKeyStorageOptions struct {
	SecretInputMode types.SecretInputMode
}

// ──────────────────────────────────────────────
// 窗口 7 的 onboard-auth.ts stub 函数
// 以下函数签名对应 TS 中 onboard-auth.ts 的导出。
// 供窗口 7 补全真实实现。
// ──────────────────────────────────────────────

// ApplyAuthProfileConfig 应用认证 Profile 配置。
func ApplyAuthProfileConfig(config OpenClawConfig, params ApplyProfileConfigParams) OpenClawConfig {
	// stub: 窗口 7 补全
	return config
}

// ApplyProfileConfigParams ApplyAuthProfileConfig 的参数。
type ApplyProfileConfigParams struct {
	ProfileID string
	Provider  string
	Mode      string
}

// WriteOAuthCredentials 写入 OAuth 凭据。
func WriteOAuthCredentials(provider string, creds interface{}, agentDir string) (string, error) {
	// stub: 窗口 7 补全
	return provider + ":oauth", nil
}

// ──────────────────────────────────────────────
// set*ApiKey stub 函数
// ──────────────────────────────────────────────

func SetAnthropicApiKey(apiKey types.SecretInput, agentDir string, opts *ApiKeyStorageOptions) error {
	return nil
}
func SetGeminiApiKey(apiKey types.SecretInput, agentDir string, opts *ApiKeyStorageOptions) error {
	return nil
}
func SetMoonshotApiKey(apiKey types.SecretInput, agentDir string, opts *ApiKeyStorageOptions) error {
	return nil
}
func SetKimiCodingApiKey(apiKey types.SecretInput, agentDir string, opts *ApiKeyStorageOptions) error {
	return nil
}
func SetMistralApiKey(apiKey types.SecretInput, agentDir string, opts *ApiKeyStorageOptions) error {
	return nil
}
func SetOpenaiApiKey(apiKey types.SecretInput, agentDir string, opts *ApiKeyStorageOptions) error {
	return nil
}
func SetOpenrouterApiKey(apiKey types.SecretInput, agentDir string, opts *ApiKeyStorageOptions) error {
	return nil
}
func SetKilocodeApiKey(apiKey types.SecretInput, agentDir string, opts *ApiKeyStorageOptions) error {
	return nil
}
func SetLitellmApiKey(apiKey types.SecretInput, agentDir string, opts *ApiKeyStorageOptions) error {
	return nil
}
func SetVercelAiGatewayApiKey(apiKey types.SecretInput, agentDir string, opts *ApiKeyStorageOptions) error {
	return nil
}
func SetCloudflareAiGatewayConfig(accountID, gatewayID string, apiKey types.SecretInput, agentDir string, opts *ApiKeyStorageOptions) error {
	return nil
}
func SetXiaomiApiKey(apiKey types.SecretInput, agentDir string, opts *ApiKeyStorageOptions) error {
	return nil
}
func SetOpencodeZenApiKey(apiKey types.SecretInput, agentDir string, opts *ApiKeyStorageOptions) error {
	return nil
}
func SetSyntheticApiKey(apiKey types.SecretInput, agentDir string, opts *ApiKeyStorageOptions) error {
	return nil
}
func SetVeniceApiKey(apiKey types.SecretInput, agentDir string, opts *ApiKeyStorageOptions) error {
	return nil
}
func SetTogetherApiKey(apiKey types.SecretInput, agentDir string, opts *ApiKeyStorageOptions) error {
	return nil
}
func SetHuggingfaceApiKey(apiKey types.SecretInput, agentDir string, opts *ApiKeyStorageOptions) error {
	return nil
}
func SetZaiApiKey(apiKey types.SecretInput, agentDir string, opts *ApiKeyStorageOptions) error {
	return nil
}
func SetXaiApiKey(apiKey types.SecretInput, agentDir string, opts *ApiKeyStorageOptions) error {
	return nil
}
func SetVolcengineApiKey(apiKey types.SecretInput, agentDir string, opts *ApiKeyStorageOptions) error {
	return nil
}
func SetByteplusApiKey(apiKey types.SecretInput, agentDir string, opts *ApiKeyStorageOptions) error {
	return nil
}
func SetQianfanApiKey(apiKey types.SecretInput, agentDir string, opts *ApiKeyStorageOptions) error {
	return nil
}
func SetMinimaxApiKey(apiKey types.SecretInput, agentDir string, profileID string, opts *ApiKeyStorageOptions) error {
	return nil
}

// ──────────────────────────────────────────────
// apply*Config / apply*ProviderConfig stub 函数
// ──────────────────────────────────────────────

func ApplyMoonshotConfig(config OpenClawConfig) OpenClawConfig                { return config }
func ApplyMoonshotProviderConfig(config OpenClawConfig) OpenClawConfig        { return config }
func ApplyMoonshotConfigCn(config OpenClawConfig) OpenClawConfig              { return config }
func ApplyMoonshotProviderConfigCn(config OpenClawConfig) OpenClawConfig      { return config }
func ApplyKimiCodeConfig(config OpenClawConfig) OpenClawConfig                { return config }
func ApplyKimiCodeProviderConfig(config OpenClawConfig) OpenClawConfig        { return config }
func ApplyMistralConfig(config OpenClawConfig) OpenClawConfig                 { return config }
func ApplyMistralProviderConfig(config OpenClawConfig) OpenClawConfig         { return config }
func ApplyXiaomiConfig(config OpenClawConfig) OpenClawConfig                  { return config }
func ApplyXiaomiProviderConfig(config OpenClawConfig) OpenClawConfig          { return config }
func ApplyVeniceConfig(config OpenClawConfig) OpenClawConfig                  { return config }
func ApplyVeniceProviderConfig(config OpenClawConfig) OpenClawConfig          { return config }
func ApplyOpencodeZenConfig(config OpenClawConfig) OpenClawConfig             { return config }
func ApplyOpencodeZenProviderConfig(config OpenClawConfig) OpenClawConfig     { return config }
func ApplyTogetherConfig(config OpenClawConfig) OpenClawConfig                { return config }
func ApplyTogetherProviderConfig(config OpenClawConfig) OpenClawConfig        { return config }
func ApplyQianfanConfig(config OpenClawConfig) OpenClawConfig                 { return config }
func ApplyQianfanProviderConfig(config OpenClawConfig) OpenClawConfig         { return config }
func ApplyKilocodeConfig(config OpenClawConfig) OpenClawConfig                { return config }
func ApplyKilocodeProviderConfig(config OpenClawConfig) OpenClawConfig        { return config }
func ApplySyntheticConfig(config OpenClawConfig) OpenClawConfig               { return config }
func ApplySyntheticProviderConfig(config OpenClawConfig) OpenClawConfig       { return config }
func ApplyVercelAiGatewayConfig(config OpenClawConfig) OpenClawConfig         { return config }
func ApplyVercelAiGatewayProviderConfig(config OpenClawConfig) OpenClawConfig { return config }
func ApplyLitellmConfig(config OpenClawConfig) OpenClawConfig                 { return config }
func ApplyLitellmProviderConfig(config OpenClawConfig) OpenClawConfig         { return config }
func ApplyOpenrouterConfig(config OpenClawConfig) OpenClawConfig              { return config }
func ApplyHuggingfaceConfig(config OpenClawConfig) OpenClawConfig             { return config }
func ApplyXaiConfig(config OpenClawConfig) OpenClawConfig                     { return config }

// ApplyCloudflareAiGatewayConfigWithIDs 应用 Cloudflare AI Gateway 配置（带 account/gateway ID）。
func ApplyCloudflareAiGatewayConfigWithIDs(config OpenClawConfig, accountID, gatewayID string) OpenClawConfig {
	return config
}

// ApplyCloudflareAiGatewayProviderConfigWithIDs 应用 Cloudflare AI Gateway Provider 配置。
func ApplyCloudflareAiGatewayProviderConfigWithIDs(config OpenClawConfig, accountID, gatewayID string) OpenClawConfig {
	return config
}

// ZaiConfigOptions Z.AI 配置选项。
type ZaiConfigOptions struct {
	Endpoint string
	ModelID  string
}

func ApplyZaiConfig(config OpenClawConfig, opts ZaiConfigOptions) OpenClawConfig { return config }
func ApplyZaiProviderConfig(config OpenClawConfig, opts ZaiConfigOptions) OpenClawConfig {
	return config
}

func ApplyMinimaxConfig(config OpenClawConfig) OpenClawConfig                      { return config }
func ApplyMinimaxApiConfig(config OpenClawConfig, modelID string) OpenClawConfig   { return config }
func ApplyMinimaxApiConfigCn(config OpenClawConfig, modelID string) OpenClawConfig { return config }

func ApplyOpenAIConfig(config OpenClawConfig) OpenClawConfig { return config }

// ──────────────────────────────────────────────
// 模型默认值常量 stub
// ──────────────────────────────────────────────

const (
	MoonshotDefaultModelRef            = "moonshot/kimi-k2.5"
	KimiCodingModelRef                 = "kimi-coding/k2p5"
	MistralDefaultModelRef             = "mistral/mistral-large-latest"
	XiaomiDefaultModelRef              = "xiaomi/mimo-v2-flash"
	VeniceDefaultModelRef              = "venice/llama-3.3-70b"
	OpencodeZenDefaultModel            = "opencode/claude-opus-4-6"
	TogetherDefaultModelRef            = "together/moonshotai/Kimi-K2.5"
	QianfanDefaultModelRef             = "qianfan/ernie-x1-turbo-32k"
	KilocodeDefaultModelRef            = "kilocode/anthropic/claude-opus-4.6"
	SyntheticDefaultModelRef           = "synthetic/MiniMax-M2.5"
	VercelAiGatewayDefaultModelRef     = "vercel-ai-gateway/anthropic/claude-opus-4.6"
	LitellmDefaultModelRef             = "litellm/claude-opus-4-6"
	CloudflareAiGatewayDefaultModelRef = "cloudflare-ai-gateway/anthropic/claude-sonnet-4-20250514"
	ZaiDefaultModelRef                 = "zai/glm-5"
	GoogleGeminiDefaultModel           = "google/gemini-3.1-pro-preview"
	OpenaiCodexDefaultModel            = "openai-codex/codex-mini-latest"
)

// ──────────────────────────────────────────────
// Google Gemini 模型默认值 stub
// ──────────────────────────────────────────────

// ApplyGoogleGeminiModelDefaultResult Google Gemini 模型默认值应用结果。
type ApplyGoogleGeminiModelDefaultResult struct {
	Next    OpenClawConfig
	Changed bool
}

// ApplyGoogleGeminiModelDefault 应用 Google Gemini 默认模型。
func ApplyGoogleGeminiModelDefault(config OpenClawConfig) ApplyGoogleGeminiModelDefaultResult {
	return ApplyGoogleGeminiModelDefaultResult{Next: config, Changed: true}
}

// ApplyPrimaryModel 应用主模型。
// 委托到 onboard.ApplyAgentDefaultModelPrimary 设置 agents.defaults.model.primary。
func ApplyPrimaryModel(config OpenClawConfig, modelRef string) OpenClawConfig {
	return onboard.ApplyAgentDefaultModelPrimary(config, modelRef)
}

// ──────────────────────────────────────────────
// 其他外部依赖 stub
// ──────────────────────────────────────────────

// DetectZaiEndpointResult Z.AI 端点检测结果。
type DetectZaiEndpointResult struct {
	Endpoint string
	ModelID  string
	Note     string
}

// DetectZaiEndpoint 检测 Z.AI 端点。
func DetectZaiEndpoint(apiKey string) *DetectZaiEndpointResult {
	return nil
}

// EnsureModelAllowlistEntry 确保模型白名单条目。
func EnsureModelAllowlistEntry(cfg OpenClawConfig, modelRef string) OpenClawConfig {
	return cfg
}

// LoginChutesParams Chutes OAuth 登录参数。
type LoginChutesParams struct {
	App        ChutesApp
	Manual     bool
	OnAuth     func(url string) error
	OnPrompt   func() (string, error)
	OnProgress func(msg string)
}

// ChutesApp Chutes 应用配置。
type ChutesApp struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Scopes       []string
}

// LoginChutes 执行 Chutes OAuth 登录。
func LoginChutes(params LoginChutesParams) (interface{}, error) {
	return nil, nil
}

// IsRemoteEnvironment 检查是否在远程环境中运行。
func IsRemoteEnvironment() bool {
	return false
}

// VpsAwareOAuthHandlersParams VPS 感知 OAuth 处理器参数。
type VpsAwareOAuthHandlersParams struct {
	IsRemote            bool
	Prompter            WizardPrompter
	Runtime             RuntimeEnv
	Spin                ProgressSpinner
	OpenURL             func(string) error
	LocalBrowserMessage string
}

// VpsAwareOAuthHandlers VPS 感知 OAuth 处理器结果。
type VpsAwareOAuthHandlers struct {
	OnAuth   func(url string) error
	OnPrompt func() (string, error)
}

// CreateVpsAwareOAuthHandlers 创建 VPS 感知 OAuth 处理器。
func CreateVpsAwareOAuthHandlers(params VpsAwareOAuthHandlersParams) VpsAwareOAuthHandlers {
	return VpsAwareOAuthHandlers{
		OnAuth:   func(_ string) error { return nil },
		OnPrompt: func() (string, error) { return "", nil },
	}
}

// OpenURL 打开 URL。
func OpenURL(url string) error {
	return nil
}

// ──────────────────────────────────────────────
// ResolveEnvApiKey stub（窗口 4 agents/model_auth.go 应有）
// ──────────────────────────────────────────────

// EnvApiKeyResult 环境变量 API Key 解析结果。
type EnvApiKeyResult struct {
	ApiKey string
	Source string
}

// ResolveEnvApiKey 从环境变量解析 API Key。
func ResolveEnvApiKey(provider string) *EnvApiKeyResult {
	return nil
}

// ──────────────────────────────────────────────
// 密钥引用相关 stub
// ──────────────────────────────────────────────

// ProviderEnvVars 提供者对应的环境变量名称映射。
// 委托到 onboard 包的完整映射（25+ provider）。
var ProviderEnvVars = onboard.ProviderEnvVars

// ResolveDefaultSecretProviderAlias 解析默认密钥提供者别名。
func ResolveDefaultSecretProviderAlias(config OpenClawConfig, source string, opts interface{}) string {
	return "default"
}

// IsValidFileSecretRefId 检查文件密钥引用 ID 是否合法。
func IsValidFileSecretRefId(id string) bool {
	return len(id) > 0 && id[0] == '/'
}

// ResolveSecretRefString 解析密钥引用为实际值。
func ResolveSecretRefString(ref types.SecretRef, ctx interface{}) (string, error) {
	return "", nil
}

// EncodeJsonPointerToken 编码 JSON Pointer Token。
func EncodeJsonPointerToken(s string) string {
	return s
}

// ──────────────────────────────────────────────
// AuthProfile 操作 stub
// ──────────────────────────────────────────────

// EnsureAuthProfileStore 确保 AuthProfile 存储。
func EnsureAuthProfileStore(agentDir string, opts ...interface{}) *types.AuthProfileStore {
	return &types.AuthProfileStore{Profiles: map[string]map[string]interface{}{}}
}

// ListProfilesForProvider 列出指定 Provider 的 Profile。
func ListProfilesForProvider(store *types.AuthProfileStore, provider string) []string {
	return nil
}

// ResolveAuthProfileOrder 解析 Profile 排序。
func ResolveAuthProfileOrder(cfg OpenClawConfig, store *types.AuthProfileStore, provider string) []string {
	return nil
}

// UpsertAuthProfile 创建或更新 AuthProfile。
func UpsertAuthProfile(profileID string, credential interface{}) {
}

// GetCustomProviderApiKey 获取自定义 Provider API Key。
func GetCustomProviderApiKey(config OpenClawConfig, provider string) string {
	return ""
}

// ──────────────────────────────────────────────
// Model 相关 stub
// ──────────────────────────────────────────────

// ModelCatalogEntry 模型目录条目。
type ModelCatalogEntry struct {
	Provider string
	ID       string
}

// LoadModelCatalog 加载模型目录。
func LoadModelCatalog(config OpenClawConfig) []ModelCatalogEntry {
	return nil
}

// ModelRef 模型引用。
type ModelRef struct {
	Provider string
	Model    string
}

// ResolveDefaultModelForAgent 解析代理的默认模型。
func ResolveDefaultModelForAgent(cfg OpenClawConfig, agentID string) ModelRef {
	return ModelRef{}
}

// NormalizeProviderId 规范化 Provider ID。
func NormalizeProviderId(raw string) string {
	return raw
}

// ──────────────────────────────────────────────
// Token 相关 stub
// ──────────────────────────────────────────────

// BuildTokenProfileId 构建令牌 Profile ID。
func BuildTokenProfileId(provider, name string) string {
	return provider + ":token"
}

// ValidateAnthropicSetupToken 验证 Anthropic setup token。
func ValidateAnthropicSetupToken(token string) string {
	return ""
}

// ParseDurationMs 解析持续时间为毫秒。
func ParseDurationMs(raw string, defaultUnit string) (int64, error) {
	return 0, nil
}

// NormalizeSecretInput 规范化密钥输入。
func NormalizeSecretInput(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// ──────────────────────────────────────────────
// 非交互式 API Key 解析 stub
// ──────────────────────────────────────────────

// ResolvedNonInteractiveApiKey 非交互式解析的 API Key 结果。
type ResolvedNonInteractiveApiKey struct {
	Key        string
	Source     string // "env" | "flag" | "profile"
	EnvVarName string
}

// ResolveNonInteractiveApiKeyParams 非交互式 API Key 解析参数。
type ResolveNonInteractiveApiKeyParams struct {
	Provider        string
	Cfg             OpenClawConfig
	FlagValue       *string
	FlagName        string
	EnvVar          string
	EnvVarName      string
	Runtime         RuntimeEnv
	Required        bool
	SecretInputMode types.SecretInputMode
}

// ResolveNonInteractiveApiKey 解析非交互式 API Key。
func ResolveNonInteractiveApiKey(params ResolveNonInteractiveApiKeyParams) *ResolvedNonInteractiveApiKey {
	return nil
}

// ──────────────────────────────────────────────
// 自定义 Provider stub
// ──────────────────────────────────────────────

// CustomApiError 自定义 API 错误。
type CustomApiError struct {
	Code    string
	Message string
}

func (e *CustomApiError) Error() string { return e.Message }

// ParsedCustomApiFlags 解析后的自定义 API 标志。
type ParsedCustomApiFlags struct {
	BaseURL       string
	ModelID       string
	Compatibility string
	ApiKey        string
	ProviderID    string
}

// ParseNonInteractiveCustomApiFlags 解析非交互式自定义 API 标志。
func ParseNonInteractiveCustomApiFlags(params map[string]interface{}) ParsedCustomApiFlags {
	return ParsedCustomApiFlags{}
}

// ResolvedCustomProviderId 解析后的自定义 Provider ID。
type ResolvedCustomProviderId struct {
	ProviderID string
}

// ResolveCustomProviderId 解析自定义 Provider ID。
func ResolveCustomProviderId(config OpenClawConfig, baseURL, providerID string) ResolvedCustomProviderId {
	return ResolvedCustomProviderId{ProviderID: providerID}
}

// ApplyCustomApiConfigResult 应用自定义 API 配置结果。
type ApplyCustomApiConfigResult struct {
	Config                OpenClawConfig
	ProviderID            string
	ProviderIdRenamedFrom string
}

// ApplyCustomApiConfig 应用自定义 API 配置。
func ApplyCustomApiConfig(params map[string]interface{}) ApplyCustomApiConfigResult {
	return ApplyCustomApiConfigResult{}
}
