package gateway

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// ---------- 频道配置 (移植自 channels.ts / channel-config.json) ----------

// ChannelConfig 频道配置。
type ChannelConfig struct {
	Name         string            `json:"name"`
	Label        string            `json:"label,omitempty"`
	Model        string            `json:"model,omitempty"`
	SystemPrompt string            `json:"systemPrompt,omitempty"`
	MaxTokens    int               `json:"maxTokens,omitempty"`
	Temperature  *float64          `json:"temperature,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ChannelSnapshot 频道运行时快照。
type ChannelSnapshot struct {
	Channel    ChannelConfig `json:"channel"`
	ActiveRuns int           `json:"activeRuns"`
	LastUsedAt int64         `json:"lastUsedAt,omitempty"`
}

// ChannelRegistry 频道配置注册表。
type ChannelRegistry struct {
	mu       sync.RWMutex
	channels map[string]*ChannelConfig
}

// NewChannelRegistry 创建频道注册表。
func NewChannelRegistry() *ChannelRegistry {
	return &ChannelRegistry{channels: make(map[string]*ChannelConfig)}
}

// Register 注册频道配置。
func (r *ChannelRegistry) Register(cfg *ChannelConfig) error {
	if cfg == nil || cfg.Name == "" {
		return fmt.Errorf("channel name required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channels[cfg.Name] = cfg
	return nil
}

// Get 获取频道配置。
func (r *ChannelRegistry) Get(name string) *ChannelConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.channels[name]
}

// List 列出所有频道配置。
func (r *ChannelRegistry) List() []*ChannelConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*ChannelConfig, 0, len(r.channels))
	for _, c := range r.channels {
		result = append(result, c)
	}
	return result
}

// Remove 移除频道配置。
func (r *ChannelRegistry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.channels, name)
}

// Reset 清空所有频道（用于测试）。
func (r *ChannelRegistry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channels = make(map[string]*ChannelConfig)
}

// ParseChannelConfig 从 JSON 解析频道配置。
func ParseChannelConfig(data string) (*ChannelConfig, error) {
	if data == "" {
		return nil, fmt.Errorf("empty config")
	}
	var cfg ChannelConfig
	if err := json.Unmarshal([]byte(data), &cfg); err != nil {
		return nil, err
	}
	cfg.Name = strings.TrimSpace(cfg.Name)
	if cfg.Name == "" {
		return nil, fmt.Errorf("empty channel name")
	}
	return &cfg, nil
}

// ---------- 工具策略接口定义 (移植自 tool-strategy.ts) ----------

// ToolStrategy 工具执行策略接口。
type ToolStrategy interface {
	// ShouldConfirm 判断指定工具是否需要用户确认。
	ShouldConfirm(toolName string) bool
	// AllowedTools 返回当前策略允许的工具名称列表，nil 表示全部允许。
	AllowedTools() []string
	// DeniedTools 返回当前策略禁止的工具名称列表，nil 表示无限制。
	DeniedTools() []string
}

// DefaultToolStrategy 默认工具策略（允许全部，无需确认）。
type DefaultToolStrategy struct{}

// ShouldConfirm 默认无需确认。
func (s *DefaultToolStrategy) ShouldConfirm(toolName string) bool { return false }

// AllowedTools 默认允许全部。
func (s *DefaultToolStrategy) AllowedTools() []string { return nil }

// DeniedTools 默认无限制。
func (s *DefaultToolStrategy) DeniedTools() []string { return nil }

// ConfirmAllToolStrategy 需要确认所有工具调用的策略。
type ConfirmAllToolStrategy struct{}

// ShouldConfirm 全部需要确认。
func (s *ConfirmAllToolStrategy) ShouldConfirm(toolName string) bool { return true }

// AllowedTools 允许全部。
func (s *ConfirmAllToolStrategy) AllowedTools() []string { return nil }

// DeniedTools 无限制。
func (s *ConfirmAllToolStrategy) DeniedTools() []string { return nil }

// AllowListToolStrategy 基于白名单的工具策略。
type AllowListToolStrategy struct {
	Allowed         map[string]struct{}
	ConfirmUnlisted bool
}

// ShouldConfirm 白名单外的工具需要确认。
func (s *AllowListToolStrategy) ShouldConfirm(toolName string) bool {
	if s.Allowed == nil {
		return s.ConfirmUnlisted
	}
	_, ok := s.Allowed[toolName]
	return !ok && s.ConfirmUnlisted
}

// AllowedTools 返回白名单。
func (s *AllowListToolStrategy) AllowedTools() []string {
	if s.Allowed == nil {
		return nil
	}
	result := make([]string, 0, len(s.Allowed))
	for name := range s.Allowed {
		result = append(result, name)
	}
	return result
}

// DeniedTools 白名单策略不使用黑名单。
func (s *AllowListToolStrategy) DeniedTools() []string { return nil }
