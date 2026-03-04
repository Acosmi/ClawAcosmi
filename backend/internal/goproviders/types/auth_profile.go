// types/auth_profile.go — 认证 Profile 凭证与存储类型
// 对应 TS 文件: src/agents/auth-profiles/types.ts
package types

// ApiKeyCredential API 密钥凭证。
// 对应 TS: ApiKeyCredential
type ApiKeyCredential struct {
	// Type 凭证类型标识，固定为 "api_key"
	Type string `json:"type"`
	// Provider 提供者标识
	Provider string `json:"provider"`
	// Key API 密钥明文（可选，可能使用 KeyRef 代替）
	Key string `json:"key,omitempty"`
	// KeyRef 密钥引用（可选，用于安全存储）
	KeyRef *SecretRef `json:"keyRef,omitempty"`
	// Email 可选邮箱
	Email string `json:"email,omitempty"`
	// Metadata 可选的提供者特定元数据（如 account ID、gateway ID 等）
	Metadata map[string]string `json:"metadata,omitempty"`
}

// CredentialType 返回凭证类型标识。
func (c *ApiKeyCredential) CredentialType() string { return "api_key" }

// TokenCredential 静态令牌凭证。
// 对应 TS: TokenCredential
// 静态 bearer 风格令牌（通常是 OAuth access token / PAT）。
// 不可由 OpenClaw 自动刷新（不同于 OAuthCredential）。
type TokenCredential struct {
	// Type 凭证类型标识，固定为 "token"
	Type string `json:"type"`
	// Provider 提供者标识
	Provider string `json:"provider"`
	// Token 令牌值
	Token string `json:"token"`
	// TokenRef 令牌引用（可选）
	TokenRef *SecretRef `json:"tokenRef,omitempty"`
	// Expires 可选的过期时间戳（毫秒级 Unix 时间）
	Expires *int64 `json:"expires,omitempty"`
	// Email 可选邮箱
	Email string `json:"email,omitempty"`
}

// CredentialType 返回凭证类型标识。
func (c *TokenCredential) CredentialType() string { return "token" }

// OAuthCredential OAuth 凭证，继承 OAuthCredentials 并扩展。
// 对应 TS: OAuthCredential = OAuthCredentials & { type: "oauth"; provider: string; ... }
type OAuthCredential struct {
	OAuthCredentials
	// Type 凭证类型标识，固定为 "oauth"
	Type string `json:"type"`
	// Provider 提供者标识
	Provider string `json:"provider"`
	// ClientID 可选的 OAuth 客户端 ID
	ClientID string `json:"clientId,omitempty"`
	// Email 可选邮箱
	Email string `json:"email,omitempty"`
}

// CredentialType 返回凭证类型标识。
func (c *OAuthCredential) CredentialType() string { return "oauth" }

// AuthProfileCredential 认证 Profile 凭证接口。
// 对应 TS 联合类型: ApiKeyCredential | TokenCredential | OAuthCredential
type AuthProfileCredential interface {
	CredentialType() string
}

// AuthProfileFailureReason 认证 Profile 失败原因。
// 对应 TS: AuthProfileFailureReason
type AuthProfileFailureReason string

const (
	FailureReasonAuth           AuthProfileFailureReason = "auth"
	FailureReasonAuthPermanent  AuthProfileFailureReason = "auth_permanent"
	FailureReasonFormat         AuthProfileFailureReason = "format"
	FailureReasonRateLimit      AuthProfileFailureReason = "rate_limit"
	FailureReasonBilling        AuthProfileFailureReason = "billing"
	FailureReasonTimeout        AuthProfileFailureReason = "timeout"
	FailureReasonModelNotFound  AuthProfileFailureReason = "model_not_found"
	FailureReasonSessionExpired AuthProfileFailureReason = "session_expired"
	FailureReasonUnknown        AuthProfileFailureReason = "unknown"
)

// ProfileUsageStats 每个 Profile 的使用统计信息，用于轮询和冷却追踪。
// 对应 TS: ProfileUsageStats
type ProfileUsageStats struct {
	// LastUsed 最近使用时间戳
	LastUsed *int64 `json:"lastUsed,omitempty"`
	// CooldownUntil 冷却截止时间戳
	CooldownUntil *int64 `json:"cooldownUntil,omitempty"`
	// DisabledUntil 禁用截止时间戳
	DisabledUntil *int64 `json:"disabledUntil,omitempty"`
	// DisabledReason 禁用原因
	DisabledReason AuthProfileFailureReason `json:"disabledReason,omitempty"`
	// ErrorCount 错误计数
	ErrorCount *int `json:"errorCount,omitempty"`
	// FailureCounts 按失败原因分类的计数
	FailureCounts map[AuthProfileFailureReason]int `json:"failureCounts,omitempty"`
	// LastFailureAt 最近失败时间戳
	LastFailureAt *int64 `json:"lastFailureAt,omitempty"`
}

// AuthProfileStore 认证 Profile 存储结构。
// 对应 TS: AuthProfileStore
type AuthProfileStore struct {
	// Version 存储版本号
	Version int `json:"version"`
	// Profiles Profile 凭证映射
	Profiles map[string]map[string]interface{} `json:"profiles"`
	// Order 可选的每个代理的 Profile 优先级顺序覆盖
	Order map[string][]string `json:"order,omitempty"`
	// LastGood 最近成功的 Profile ID 映射
	LastGood map[string]string `json:"lastGood,omitempty"`
	// UsageStats 每个 Profile 的使用统计
	UsageStats map[string]ProfileUsageStats `json:"usageStats,omitempty"`
}

// AuthProfileIDRepairResult Profile ID 修复结果。
// 对应 TS: AuthProfileIdRepairResult
type AuthProfileIDRepairResult struct {
	// Changes 修改描述列表
	Changes []string `json:"changes"`
	// Migrated 是否发生了迁移
	Migrated bool `json:"migrated"`
	// FromProfileID 迁移前的 Profile ID
	FromProfileID string `json:"fromProfileId,omitempty"`
	// ToProfileID 迁移后的 Profile ID
	ToProfileID string `json:"toProfileId,omitempty"`
}
