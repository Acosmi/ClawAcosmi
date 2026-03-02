package website

// ============================================================================
// website/config.go — 自有网站频道配置
// 定义 WebsiteConfig 结构体及校验逻辑。
//
// Design doc: docs/xinshenji/impl-tracking-media-subagent.md §P4-2
// ============================================================================

import "fmt"

// AuthType 认证方式。
type AuthType string

const (
	AuthBearer AuthType = "bearer"
	AuthAPIKey AuthType = "api_key"
	AuthBasic  AuthType = "basic"
)

// WebsiteConfig 自有网站频道配置。
type WebsiteConfig struct {
	// Enabled 是否启用。
	Enabled bool `yaml:"enabled" json:"enabled"`

	// APIURL 发布 API 的完整 URL，例如 https://example.com/api/posts。
	APIURL string `yaml:"api_url" json:"api_url"`

	// AuthType_ 认证方式: bearer / api_key / basic。
	AuthType_ AuthType `yaml:"auth_type" json:"auth_type"`

	// AuthToken 认证令牌 / API Key / Basic Auth 格式 "user:pass"。
	AuthToken string `yaml:"auth_token" json:"auth_token"`

	// ImageUploadURL 图片上传 API URL（可选）。
	// 若为空则在 POST body 中内联 base64 图片。
	ImageUploadURL string `yaml:"image_upload_url" json:"image_upload_url,omitempty"`

	// TimeoutSeconds 请求超时（秒），默认 30。
	TimeoutSeconds int `yaml:"timeout_seconds" json:"timeout_seconds,omitempty"`

	// MaxRetries 最大重试次数，默认 3。
	MaxRetries int `yaml:"max_retries" json:"max_retries,omitempty"`
}

// DefaultConfig 返回默认配置。
func DefaultConfig() *WebsiteConfig {
	return &WebsiteConfig{
		AuthType_:      AuthBearer,
		TimeoutSeconds: 30,
		MaxRetries:     3,
	}
}

// Validate 校验配置有效性。
func (c *WebsiteConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("website config is nil")
	}
	if !c.Enabled {
		return nil // 未启用不做校验
	}
	if c.APIURL == "" {
		return fmt.Errorf("website: api_url required")
	}
	switch c.AuthType_ {
	case AuthBearer, AuthAPIKey, AuthBasic:
		// ok
	case "":
		return fmt.Errorf("website: auth_type required")
	default:
		return fmt.Errorf(
			"website: unknown auth_type %q (use bearer/api_key/basic)",
			c.AuthType_)
	}
	if c.AuthToken == "" {
		return fmt.Errorf("website: auth_token required")
	}
	if c.TimeoutSeconds <= 0 {
		c.TimeoutSeconds = 30
	}
	if c.MaxRetries < 0 {
		c.MaxRetries = 0
	}
	return nil
}
