// oauth/gemini/plugin.go — Google Gemini CLI 插件定义
// 对应 TS 文件: extensions/google-gemini-cli-auth/index.ts (76 行)
// 本文件定义 Gemini CLI OAuth 插件的元数据、常量和注册信息。
package gemini

// ──────────────────── 插件常量 ────────────────────

const (
	// ProviderID Gemini CLI 提供者标识。
	ProviderID = "google-gemini-cli"
	// ProviderLabel Gemini CLI 提供者显示名称。
	ProviderLabel = "Gemini CLI OAuth"
	// DefaultModel 默认模型标识。
	DefaultModel = "google-gemini-cli/gemini-3-pro-preview"
)

// EnvVars Gemini CLI 相关的环境变量列表。
var EnvVars = []string{
	"OPENCLAW_GEMINI_OAUTH_CLIENT_ID",
	"OPENCLAW_GEMINI_OAUTH_CLIENT_SECRET",
	"GEMINI_CLI_OAUTH_CLIENT_ID",
	"GEMINI_CLI_OAUTH_CLIENT_SECRET",
}

// ──────────────────── 插件定义 ────────────────────

// GeminiCliPluginInfo Gemini CLI OAuth 插件信息。
// 对应 TS: geminiCliPlugin 对象
type GeminiCliPluginInfo struct {
	// ID 插件唯一标识
	ID string
	// Name 插件名称
	Name string
	// Description 插件描述
	Description string
}

// PluginInfo 返回 Gemini CLI OAuth 插件的元信息。
func PluginInfo() GeminiCliPluginInfo {
	return GeminiCliPluginInfo{
		ID:          "google-gemini-cli-auth",
		Name:        "Google Gemini CLI Auth",
		Description: "OAuth flow for Gemini CLI (Google Code Assist)",
	}
}

// ProviderRegistration Gemini CLI 提供者注册信息。
// 对应 TS: api.registerProvider({...})
type ProviderRegistration struct {
	// ID 提供者 ID
	ID string
	// Label 显示名称
	Label string
	// DocsPath 文档路径
	DocsPath string
	// Aliases 别名列表
	Aliases []string
	// EnvVars 环境变量列表
	EnvVars []string
	// AuthID 认证方式 ID
	AuthID string
	// AuthLabel 认证方式标签
	AuthLabel string
	// AuthHint 认证方式提示
	AuthHint string
	// AuthKind 认证方式类型
	AuthKind string
}

// ProviderRegistrationInfo 返回提供者注册配置信息。
func ProviderRegistrationInfo() ProviderRegistration {
	return ProviderRegistration{
		ID:        ProviderID,
		Label:     ProviderLabel,
		DocsPath:  "/providers/models",
		Aliases:   []string{"gemini-cli"},
		EnvVars:   EnvVars,
		AuthID:    "oauth",
		AuthLabel: "Google OAuth",
		AuthHint:  "PKCE + localhost callback",
		AuthKind:  "oauth",
	}
}
