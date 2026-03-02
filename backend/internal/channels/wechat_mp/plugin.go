package wechat_mp

// ============================================================================
// wechat_mp/plugin.go — 微信公众号频道插件
// 实现 channels.Plugin 接口。
//
// Design doc: docs/xinshenji/impl-tracking-media-subagent.md §P2-4
// ============================================================================

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/openacosmi/claw-acismi/internal/channels"
	"github.com/openacosmi/claw-acismi/internal/media"
)

// WeChatMPPlugin 微信公众号频道插件。
type WeChatMPPlugin struct {
	mu         sync.Mutex
	clients    map[string]*WeChatMPClient
	publishers map[string]*Publisher
}

// NewWeChatMPPlugin 创建微信公众号插件。
func NewWeChatMPPlugin() *WeChatMPPlugin {
	return &WeChatMPPlugin{
		clients:    make(map[string]*WeChatMPClient),
		publishers: make(map[string]*Publisher),
	}
}

// ID 返回频道标识。
func (p *WeChatMPPlugin) ID() channels.ChannelID {
	return media.ChannelWeChatMP
}

// Start 启动微信公众号频道。
// cfg 应在外部预校验后注入。
func (p *WeChatMPPlugin) Start(accountID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if accountID == "" {
		accountID = channels.DefaultAccountID
	}

	// 插件使用外部注入的配置启动，此处简单记录。
	slog.Info("wechat_mp plugin start requested",
		"account", accountID)

	// 如果已有 client 则跳过。
	if _, ok := p.clients[accountID]; ok {
		slog.Info("wechat_mp account already started", "account", accountID)
		return nil
	}

	slog.Warn("wechat_mp: no client configured for account, "+
		"use ConfigureAccount() before Start()", "account", accountID)
	return nil
}

// ConfigureAccount 配置并启动指定账号。
func (p *WeChatMPPlugin) ConfigureAccount(
	accountID string,
	cfg *WeChatMPConfig,
) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if accountID == "" {
		accountID = channels.DefaultAccountID
	}

	client := NewWeChatMPClient(cfg)
	p.clients[accountID] = client
	p.publishers[accountID] = NewPublisher(client)

	slog.Info("wechat_mp account configured",
		"account", accountID, "app_id", cfg.AppID)
	return nil
}

// Stop 停止微信公众号频道。
func (p *WeChatMPPlugin) Stop(accountID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if accountID == "" {
		accountID = channels.DefaultAccountID
	}

	delete(p.clients, accountID)
	delete(p.publishers, accountID)

	slog.Info("wechat_mp channel stopped", "account", accountID)
	return nil
}

// GetPublisher 获取指定账号的发布器。
func (p *WeChatMPPlugin) GetPublisher(accountID string) *Publisher {
	p.mu.Lock()
	defer p.mu.Unlock()

	if accountID == "" {
		accountID = channels.DefaultAccountID
	}
	return p.publishers[accountID]
}

// GetClient 获取指定账号的客户端。
func (p *WeChatMPPlugin) GetClient(accountID string) *WeChatMPClient {
	p.mu.Lock()
	defer p.mu.Unlock()

	if accountID == "" {
		accountID = channels.DefaultAccountID
	}
	return p.clients[accountID]
}
