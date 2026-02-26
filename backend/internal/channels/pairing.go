package channels

import (
	"fmt"
	"strings"
)

// 配对管理 — 对齐 src/channels/plugins/pairing.ts (70L)
// 接口对齐 TS ChannelPairingAdapter (types.adapters.ts L184-192)

// PairingAdapter 配对适配器接口
type PairingAdapter interface {
	// IDLabel 返回渠道的用户 ID 标签（如 "userId"、"phone number"）
	IDLabel() string
	// NormalizeAllowEntry 渠道特定的白名单条目规范化。
	// 不需要特殊处理的渠道返回 strings.TrimSpace(entry)。
	NormalizeAllowEntry(entry string) string
}

// PairingApprovalNotifier 可选接口 — 支持通知配对批准。
// 对齐 TS ChannelPairingAdapter.notifyApproval? (可选方法)
type PairingApprovalNotifier interface {
	NotifyApproval(channelID, pairingID string) error
}

// pairingAdapters 配对适配器注册表
var pairingAdapters = make(map[ChannelID]PairingAdapter)

// RegisterPairingAdapter 注册频道配对适配器
func RegisterPairingAdapter(channelID ChannelID, adapter PairingAdapter) {
	pairingAdapters[channelID] = adapter
}

// ListPairingChannels 列出支持配对的频道
func ListPairingChannels() []ChannelID {
	var ids []ChannelID
	for id := range pairingAdapters {
		ids = append(ids, id)
	}
	return ids
}

// GetPairingAdapter 获取配对适配器
func GetPairingAdapter(channelID ChannelID) PairingAdapter {
	return pairingAdapters[channelID]
}

// RequirePairingAdapter 获取配对适配器（不存在则返回错误）
func RequirePairingAdapter(channelID ChannelID) (PairingAdapter, error) {
	adapter := GetPairingAdapter(channelID)
	if adapter == nil {
		return nil, fmt.Errorf("channel %s does not support pairing", channelID)
	}
	return adapter, nil
}

// ResolvePairingChannel 规范化并验证配对频道
func ResolvePairingChannel(raw string) (ChannelID, error) {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return "", fmt.Errorf("invalid channel: (empty)")
	}
	normalized := ResolveChannelIDByAlias(value)
	if normalized == "" {
		normalized = ChannelID(value)
	}
	channels := ListPairingChannels()
	for _, ch := range channels {
		if ch == normalized {
			return normalized, nil
		}
	}
	names := make([]string, len(channels))
	for i, ch := range channels {
		names[i] = string(ch)
	}
	return "", fmt.Errorf("invalid channel: %s (expected one of: %s)", value, strings.Join(names, ", "))
}

// NotifyPairingApproved 通知配对已批准（对齐 TS notifyPairingApproved）。
// 如果适配器未实现 PairingApprovalNotifier 则静默返回 nil。
func NotifyPairingApproved(channelID ChannelID, pairingID string) error {
	adapter := GetPairingAdapter(channelID)
	if adapter == nil {
		return fmt.Errorf("channel %s does not support pairing", channelID)
	}
	notifier, ok := adapter.(PairingApprovalNotifier)
	if !ok {
		return nil
	}
	return notifier.NotifyApproval(string(channelID), pairingID)
}
