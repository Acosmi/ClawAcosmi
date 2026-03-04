// types/auth_config.go — 认证配置类型定义
// 对应 TS 文件: src/config/types.auth.ts
package types

// AuthProfileConfig 认证 Profile 的配置信息。
// 对应 TS: AuthProfileConfig
type AuthProfileConfig struct {
	// Provider 提供者标识
	Provider string `json:"provider"`
	// Mode 凭证类型：api_key / oauth / token
	//   - api_key: 静态 API 密钥
	//   - oauth: 可刷新的 OAuth 凭证（access+refresh+expires）
	//   - token: 静态 bearer 令牌（可选过期时间，不可刷新）
	Mode string `json:"mode"`
	// Email 可选的邮箱地址
	Email string `json:"email,omitempty"`
}

// AuthConfig 认证配置。
// 对应 TS: AuthConfig
type AuthConfig struct {
	// Profiles 认证 Profile 映射表
	Profiles map[string]AuthProfileConfig `json:"profiles,omitempty"`
	// Order 每个 Provider 的 Profile 优先级顺序
	Order map[string][]string `json:"order,omitempty"`
	// Cooldowns 冷却时间配置
	Cooldowns *AuthCooldownsConfig `json:"cooldowns,omitempty"`
}

// AuthCooldownsConfig 认证冷却时间配置。
type AuthCooldownsConfig struct {
	// BillingBackoffHours 默认计费退避时间（小时），默认值: 5
	BillingBackoffHours *int `json:"billingBackoffHours,omitempty"`
	// BillingBackoffHoursByProvider 各 Provider 独立的计费退避时间（小时）
	BillingBackoffHoursByProvider map[string]int `json:"billingBackoffHoursByProvider,omitempty"`
	// BillingMaxHours 计费退避上限（小时），默认值: 24
	BillingMaxHours *int `json:"billingMaxHours,omitempty"`
	// FailureWindowHours 失败窗口期（小时），默认值: 24
	FailureWindowHours *int `json:"failureWindowHours,omitempty"`
}
