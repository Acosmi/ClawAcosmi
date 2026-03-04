// authprofile/config.go — OpenClaw 配置类型（Window 4 所需子集）
// 提供 Window 4 所需的配置结构定义，避免循环依赖。
package authprofile

// OpenClawConfig 简化的 OpenClaw 配置结构（仅包含 Window 4 所需字段）。
// 对应 TS: OpenClawConfig（简化子集）
type OpenClawConfig struct {
	Auth    *AuthConfig    `json:"auth,omitempty"`
	Secrets *SecretsConfig `json:"secrets,omitempty"`
	Models  *ModelsConfig  `json:"models,omitempty"`
}

// AuthConfig 认证配置。
type AuthConfig struct {
	// Profiles 已配置的 Profile 列表
	Profiles map[string]*AuthProfileConfig `json:"profiles,omitempty"`
	// Order 每个 Provider 的 Profile 优先级顺序
	Order map[string][]string `json:"order,omitempty"`
	// Cooldowns 冷却配置
	Cooldowns *AuthCooldownsConfig `json:"cooldowns,omitempty"`
}

// AuthProfileConfig 单个 Profile 的配置。
// 对应 TS: AuthProfileConfig
type AuthProfileConfig struct {
	Provider string `json:"provider"`
	Mode     string `json:"mode,omitempty"`
	Email    string `json:"email,omitempty"`
}

// AuthCooldownsConfig 冷却参数配置。
type AuthCooldownsConfig struct {
	BillingBackoffHours           *float64           `json:"billingBackoffHours,omitempty"`
	BillingMaxHours               *float64           `json:"billingMaxHours,omitempty"`
	FailureWindowHours            *float64           `json:"failureWindowHours,omitempty"`
	BillingBackoffHoursByProvider map[string]float64 `json:"billingBackoffHoursByProvider,omitempty"`
}

// SecretsConfig 密钥配置。
type SecretsConfig struct {
	Defaults *SecretDefaults `json:"defaults,omitempty"`
}

// SecretDefaults 密钥默认提供者配置。
type SecretDefaults struct {
	Env  string `json:"env,omitempty"`
	File string `json:"file,omitempty"`
	Exec string `json:"exec,omitempty"`
}

// ModelsConfig 模型配置。
type ModelsConfig struct {
	Providers map[string]*ModelProviderConfig `json:"providers,omitempty"`
}

// ModelProviderConfig 模型提供者配置。
type ModelProviderConfig struct {
	ApiKey string `json:"apiKey,omitempty"`
	Auth   string `json:"auth,omitempty"`
}

// SessionEntry 会话条目。
// 对应 TS: SessionEntry（简化子集）
type SessionEntry struct {
	AuthProfileOverride                string `json:"authProfileOverride,omitempty"`
	AuthProfileOverrideSource          string `json:"authProfileOverrideSource,omitempty"`
	AuthProfileOverrideCompactionCount *int   `json:"authProfileOverrideCompactionCount,omitempty"`
	CompactionCount                    *int   `json:"compactionCount,omitempty"`
	UpdatedAt                          int64  `json:"updatedAt,omitempty"`
}
