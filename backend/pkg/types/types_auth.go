package types

// 认证配置类型 — 继承自 src/config/types.auth.ts (30 行)

// AuthProfileMode 认证凭证类型
type AuthProfileMode string

const (
	AuthModeAPIKey AuthProfileMode = "api_key"
	AuthModeOAuth  AuthProfileMode = "oauth"
	AuthModeToken  AuthProfileMode = "token"
)

// AuthProfileConfig 认证配置文件条目
type AuthProfileConfig struct {
	Provider string          `json:"provider"`
	Mode     AuthProfileMode `json:"mode"`
	Email    string          `json:"email,omitempty"`
}

// AuthCooldownsConfig 认证冷却配置
type AuthCooldownsConfig struct {
	BillingBackoffHours           *int           `json:"billingBackoffHours,omitempty"`
	BillingBackoffHoursByProvider map[string]int `json:"billingBackoffHoursByProvider,omitempty"`
	BillingMaxHours               *int           `json:"billingMaxHours,omitempty"`
	FailureWindowHours            *int           `json:"failureWindowHours,omitempty"`
}

// AuthConfig 认证总配置
type AuthConfig struct {
	Profiles  map[string]*AuthProfileConfig `json:"profiles,omitempty"`
	Order     map[string][]string           `json:"order,omitempty"`
	Cooldowns *AuthCooldownsConfig          `json:"cooldowns,omitempty"`
}
