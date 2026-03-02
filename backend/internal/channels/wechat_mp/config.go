package wechat_mp

// ============================================================================
// wechat_mp/config.go — 微信公众号频道配置
// 定义 WeChatMPConfig 结构体及校验逻辑。
//
// Design doc: docs/xinshenji/impl-tracking-media-subagent.md §P2-3
// ============================================================================

import "fmt"

// WeChatMPConfig 微信公众号频道配置。
type WeChatMPConfig struct {
	Enabled        bool   `yaml:"enabled" json:"enabled"`
	AppID          string `yaml:"app_id" json:"app_id"`
	AppSecret      string `yaml:"app_secret" json:"app_secret"`
	TokenCachePath string `yaml:"token_cache_path" json:"token_cache_path,omitempty"`
}

// Validate 校验微信公众号配置必填字段。
func (c *WeChatMPConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("wechat_mp config is nil")
	}
	if c.AppID == "" {
		return fmt.Errorf("wechat_mp app_id is required")
	}
	if c.AppSecret == "" {
		return fmt.Errorf("wechat_mp app_secret is required")
	}
	return nil
}
