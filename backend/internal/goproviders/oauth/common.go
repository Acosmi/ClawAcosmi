// oauth/common.go — OAuth 公共类型和工具函数
// 对应 TS 文件: src/plugin-sdk/provider-auth-result.ts 和各 extension 的公共类型。
// 本文件定义所有 OAuth 扩展共享的类型、接口和辅助函数。
package oauth

import (
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

// ProgressReporter 进度报告接口。
// 对应 TS: { update: (msg: string) => void; stop: (msg?: string) => void }
type ProgressReporter interface {
	// Update 更新进度消息
	Update(msg string)
	// Stop 停止进度报告，可附带最终消息
	Stop(msg string)
}

// OAuthContext OAuth 流程通用上下文接口。
// 对应 TS: 各 extension 中的上下文参数（ctx.openUrl, ctx.prompter.note 等）
type OAuthContext struct {
	// IsRemote 是否运行在远程/VPS 环境
	IsRemote bool
	// OpenURL 在浏览器中打开 URL
	OpenURL func(url string) error
	// Log 输出日志信息
	Log func(msg string)
	// Note 显示通知消息
	Note func(message string, title string) error
	// Prompt 提示用户输入
	Prompt func(message string) (string, error)
	// Progress 进度报告器
	Progress ProgressReporter
}

// ProviderAuthProfile 认证结果中的 profile 条目。
// 对应 TS: ProviderAuthResult.profiles[i]
type ProviderAuthProfile struct {
	// ProfileID 认证配置文件标识
	ProfileID string `json:"profileId"`
	// Credential 凭证数据（interface{} 兼容各凭证类型的 JSON 序列化）
	Credential map[string]interface{} `json:"credential"`
}

// ProviderAuthResult OAuth 认证结果。
// 对应 TS: ProviderAuthResult
type ProviderAuthResult struct {
	// Profiles 认证配置文件列表
	Profiles []ProviderAuthProfile `json:"profiles"`
	// ConfigPatch 配置补丁（部分 OpenClawConfig）
	ConfigPatch map[string]interface{} `json:"configPatch,omitempty"`
	// DefaultModel 默认模型标识
	DefaultModel string `json:"defaultModel"`
	// Notes 附加说明
	Notes []string `json:"notes,omitempty"`
}

// BuildOAuthProviderAuthResultParams 构建 OAuth 认证结果的参数。
type BuildOAuthProviderAuthResultParams struct {
	ProviderID      string
	DefaultModel    string
	Access          string
	Refresh         string
	Expires         int64
	Email           string
	ProfilePrefix   string
	CredentialExtra map[string]interface{}
	ConfigPatch     map[string]interface{}
	Notes           []string
}

// BuildOAuthProviderAuthResult 构建标准 OAuth 认证结果。
// 对应 TS: buildOauthProviderAuthResult()
func BuildOAuthProviderAuthResult(params BuildOAuthProviderAuthResultParams) ProviderAuthResult {
	profilePrefix := params.ProfilePrefix
	if profilePrefix == "" {
		profilePrefix = params.ProviderID
	}
	email := params.Email
	profileSuffix := "default"
	if email != "" {
		profileSuffix = email
	}
	profileID := fmt.Sprintf("%s:%s", profilePrefix, profileSuffix)

	// 构建凭证
	credential := map[string]interface{}{
		"type":     "oauth",
		"provider": params.ProviderID,
		"access":   params.Access,
	}
	if params.Refresh != "" {
		credential["refresh"] = params.Refresh
	}
	if params.Expires > 0 {
		credential["expires"] = params.Expires
	}
	if email != "" {
		credential["email"] = email
	}
	// 合并额外凭证字段
	for k, v := range params.CredentialExtra {
		credential[k] = v
	}

	// 配置补丁
	configPatch := params.ConfigPatch
	if configPatch == nil {
		configPatch = map[string]interface{}{
			"agents": map[string]interface{}{
				"defaults": map[string]interface{}{
					"models": map[string]interface{}{
						params.DefaultModel: map[string]interface{}{},
					},
				},
			},
		}
	}

	return ProviderAuthResult{
		Profiles:     []ProviderAuthProfile{{ProfileID: profileID, Credential: credential}},
		ConfigPatch:  configPatch,
		DefaultModel: params.DefaultModel,
		Notes:        params.Notes,
	}
}

// IsWSL2 检测当前是否运行在 WSL2 环境中。
// 对应 TS: isWSL2Sync()
func IsWSL2() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(data))
	return strings.Contains(lower, "microsoft") && strings.Contains(lower, "wsl2")
}

// ResolvePlatform 解析当前操作系统平台标识。
// 对应 TS: resolvePlatform()
func ResolvePlatform() string {
	switch runtime.GOOS {
	case "windows":
		return "WINDOWS"
	case "darwin":
		return "MACOS"
	default:
		// Google 的 loadCodeAssist API 不接受 "LINUX"。
		// 使用 "PLATFORM_UNSPECIFIED" 匹配 pi-ai 运行时行为。
		return "PLATFORM_UNSPECIFIED"
	}
}

// GenerateUUIDv4 生成符合 RFC 4122 的 UUID v4。
// 对应 TS: randomUUID()
func GenerateUUIDv4() string {
	uuid := make([]byte, 16)
	_, _ = io.ReadFull(rand.Reader, uuid)
	// 设置版本4
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	// 设置变体
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

// NewHTTPClientWithTimeout 创建带超时的 HTTP 客户端。
// 对应 TS: fetchWithTimeout() 的超时控制
func NewHTTPClientWithTimeout(timeoutMs int) *http.Client {
	return &http.Client{
		Timeout: time.Duration(timeoutMs) * time.Millisecond,
	}
}

// DefaultHTTPClient 默认 HTTP 客户端（10秒超时）。
var DefaultHTTPClient = NewHTTPClientWithTimeout(10000)

// GenerateRandomState 生成随机状态参数（base64url 编码的 16 字节随机数）。
func GenerateRandomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("生成随机状态失败: %w", err)
	}
	// 使用 base64url 编码（手动实现，避免额外导入）
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	result := make([]byte, 22) // 16 字节 ≈ 22 base64url 字符
	for i := range result {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[n.Int64()]
	}
	return string(result), nil
}
