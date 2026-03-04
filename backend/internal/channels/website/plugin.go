package website

// ============================================================================
// website/plugin.go — 自有网站频道插件
// 实现 channels.Plugin 接口，参照 wechat_mp/plugin.go 模式。
//
// Design doc: docs/xinshenji/impl-tracking-media-subagent.md §P4-2
// ============================================================================

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/Acosmi/ClawAcosmi/internal/channels"
)

// ChannelWebsite 自有网站频道 ID。
const ChannelWebsite channels.ChannelID = "website"

// WebsitePlugin 自有网站频道插件。
type WebsitePlugin struct {
	mu      sync.Mutex
	clients map[string]*WebsiteClient
}

// NewWebsitePlugin 创建网站插件。
func NewWebsitePlugin() *WebsitePlugin {
	return &WebsitePlugin{
		clients: make(map[string]*WebsiteClient),
	}
}

// ID 返回频道标识。
func (p *WebsitePlugin) ID() channels.ChannelID {
	return ChannelWebsite
}

// Start 启动网站频道。
func (p *WebsitePlugin) Start(accountID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if accountID == "" {
		accountID = channels.DefaultAccountID
	}

	if _, ok := p.clients[accountID]; ok {
		slog.Info("website account already started",
			"account", accountID)
		return nil
	}

	slog.Warn("website: no client configured for account, "+
		"use ConfigureAccount() before Start()",
		"account", accountID)
	return nil
}

// ConfigureAccount 配置并初始化指定账号。
func (p *WebsitePlugin) ConfigureAccount(
	accountID string,
	cfg *WebsiteConfig,
) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if accountID == "" {
		accountID = channels.DefaultAccountID
	}

	client := NewWebsiteClient(cfg)
	p.clients[accountID] = client

	slog.Info("website account configured",
		"account", accountID, "api_url", cfg.APIURL)
	return nil
}

// Stop 停止网站频道。
func (p *WebsitePlugin) Stop(accountID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if accountID == "" {
		accountID = channels.DefaultAccountID
	}

	delete(p.clients, accountID)

	slog.Info("website channel stopped", "account", accountID)
	return nil
}

// GetClient 获取指定账号的客户端。
func (p *WebsitePlugin) GetClient(accountID string) *WebsiteClient {
	p.mu.Lock()
	defer p.mu.Unlock()

	if accountID == "" {
		accountID = channels.DefaultAccountID
	}
	return p.clients[accountID]
}
