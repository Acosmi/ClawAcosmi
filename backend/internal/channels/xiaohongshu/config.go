package xiaohongshu

// ============================================================================
// xiaohongshu/config.go — 小红书频道配置
//
// Design doc: docs/xinshenji/impl-tracking-media-subagent.md §P3-5
// ============================================================================

import "fmt"

// XiaohongshuConfig 小红书频道配置。
type XiaohongshuConfig struct {
	// Enabled 是否启用。
	Enabled bool `yaml:"enabled"`

	// CookiePath Cookie 文件路径。
	CookiePath string `yaml:"cookie_path"`

	// AutoInteractInterval 自动互动检查间隔（分钟），0 表示禁用。
	AutoInteractInterval int `yaml:"auto_interact_interval"`

	// RateLimitSeconds 操作最小间隔（秒），默认 5。
	RateLimitSeconds int `yaml:"rate_limit_seconds"`

	// ErrorScreenshotDir 错误截图保存目录。
	ErrorScreenshotDir string `yaml:"error_screenshot_dir"`
}

// DefaultConfig 返回默认配置。
func DefaultConfig() *XiaohongshuConfig {
	return &XiaohongshuConfig{
		RateLimitSeconds:     5,
		AutoInteractInterval: 30,
		ErrorScreenshotDir:   "_media/xhs/errors",
	}
}

// Validate 校验配置有效性。
func (c *XiaohongshuConfig) Validate() error {
	if !c.Enabled {
		return nil // 未启用不做校验
	}
	if c.CookiePath == "" {
		return fmt.Errorf("xiaohongshu: cookie_path required")
	}
	if c.RateLimitSeconds < 3 {
		return fmt.Errorf(
			"xiaohongshu: rate_limit_seconds must be >= 3 (got %d)",
			c.RateLimitSeconds)
	}
	return nil
}
