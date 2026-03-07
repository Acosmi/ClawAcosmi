package gateway

// channel_monitor_manager.go — Monitor 频道热更新管理器
//
// 管理 Discord/Telegram/Slack Monitor 模式频道的生命周期。
// 支持 Reload(cfg)：计算配置 hash，若变化则 cancel 旧 goroutine + 启动新 monitor。

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ChannelMonitorManager 管理 Monitor 模式频道（Discord/Telegram/Slack）的生命周期。
// 支持热更新：配置变化时 cancel 旧 monitor、启动新 monitor，无需重启 Gateway。
type ChannelMonitorManager struct {
	mu          sync.Mutex
	dctx        *ChannelDepsContext
	mux         *http.ServeMux // Slack HTTP 模式路由（可选）
	cancelFn    context.CancelFunc
	currentHash string
	started     bool
}

// NewChannelMonitorManager 创建 Monitor 频道管理器。
func NewChannelMonitorManager(dctx *ChannelDepsContext, mux *http.ServeMux) *ChannelMonitorManager {
	return &ChannelMonitorManager{
		dctx: dctx,
		mux:  mux,
	}
}

// Start 初次启动 Monitor 频道。
func (m *ChannelMonitorManager) Start(cfg *types.OpenAcosmiConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return
	}

	m.currentHash = hashChannelConfig(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFn = cancel
	m.started = true

	startMonitorChannels(ctx, m.dctx, cfg, m.mux)
	slog.Info("channel-monitor: started", "hash", truncHash(m.currentHash))
}

// Reload 热更新 Monitor 频道。
// 若配置 hash 未变化则跳过；否则 cancel 旧 monitor 并启动新 monitor。
func (m *ChannelMonitorManager) Reload(cfg *types.OpenAcosmiConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	newHash := hashChannelConfig(cfg)
	if newHash == m.currentHash {
		slog.Debug("channel-monitor: config unchanged, skip reload", "hash", truncHash(newHash))
		return
	}

	slog.Info("channel-monitor: config changed, reloading monitors",
		"oldHash", truncHash(m.currentHash),
		"newHash", truncHash(newHash),
	)

	// 停止旧 monitor
	if m.cancelFn != nil {
		m.cancelFn()
	}

	// 启动新 monitor
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFn = cancel
	m.currentHash = newHash
	m.started = true

	startMonitorChannels(ctx, m.dctx, cfg, m.mux)
	slog.Info("channel-monitor: reload complete", "hash", truncHash(newHash))
}

// Stop 优雅关闭所有 Monitor 频道。
func (m *ChannelMonitorManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancelFn != nil {
		m.cancelFn()
		m.cancelFn = nil
	}
	m.started = false
	slog.Info("channel-monitor: stopped")
}

// hashChannelConfig 计算 cfg.Channels 中 Discord/Telegram/Slack 配置的 SHA256 摘要。
// 仅 hash 这三个 Monitor 模式频道的配置，其他频道不参与。
func hashChannelConfig(cfg *types.OpenAcosmiConfig) string {
	if cfg == nil || cfg.Channels == nil {
		return "empty"
	}

	// 提取 Monitor 频道子配置进行序列化
	subset := struct {
		Discord  interface{} `json:"discord,omitempty"`
		Telegram interface{} `json:"telegram,omitempty"`
		Slack    interface{} `json:"slack,omitempty"`
	}{
		Discord:  cfg.Channels.Discord,
		Telegram: cfg.Channels.Telegram,
		Slack:    cfg.Channels.Slack,
	}

	data, err := json.Marshal(subset)
	if err != nil {
		return "marshal-error"
	}

	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

// truncHash 安全截断 hash 字符串用于日志显示。
func truncHash(h string) string {
	if len(h) > 16 {
		return h[:16]
	}
	return h
}
