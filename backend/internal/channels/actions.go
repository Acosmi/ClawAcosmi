package channels

import (
	"fmt"
	"strings"
)

// 频道消息动作适配器 — 继承自 src/channels/plugins/actions/ (telegram/discord/signal)
// 定义各频道支持的消息动作集合和动作门控

// MessageActionAdapter 消息动作适配器
type MessageActionAdapter struct {
	ChannelID       ChannelID
	ListActions     func(actionGate ActionGateFunc) []MessageActionName
	SupportsButtons func() bool
	SupportsCards   func() bool
	ExtractToolSend func(action string, args map[string]interface{}) *ToolSendTarget
}

// ToolSendTarget 工具发送目标
type ToolSendTarget struct {
	To        string
	AccountID string
}

// ActionGateFunc 创建动作门控函数
type ActionGateFunc func(actionName string, defaultEnabled ...bool) bool

// NewActionGate 从配置创建门控
func NewActionGate(actions interface{}) ActionGateFunc {
	if actions == nil {
		return func(name string, defaults ...bool) bool {
			if len(defaults) > 0 {
				return defaults[0]
			}
			return true
		}
	}
	m, ok := actions.(map[string]interface{})
	if !ok {
		return func(name string, defaults ...bool) bool {
			if len(defaults) > 0 {
				return defaults[0]
			}
			return true
		}
	}
	return func(name string, defaults ...bool) bool {
		if v, ok := m[name].(bool); ok {
			return v
		}
		if v, ok := m["*"].(bool); ok {
			return v
		}
		if len(defaults) > 0 {
			return defaults[0]
		}
		return true
	}
}

// ── Telegram 动作集 ──

// TelegramListActions Telegram 可用动作列表
func TelegramListActions(gate ActionGateFunc) []MessageActionName {
	actions := []MessageActionName{ActionSend}
	if gate("reactions") {
		actions = append(actions, "react")
	}
	if gate("deleteMessage") {
		actions = append(actions, "delete")
	}
	if gate("editMessage") {
		actions = append(actions, "edit")
	}
	if gate("sticker", false) {
		actions = append(actions, "sticker", "sticker-search")
	}
	return actions
}

// TelegramExtractToolSend 从 Telegram 工具参数提取发送目标
func TelegramExtractToolSend(action string, args map[string]interface{}) *ToolSendTarget {
	if action != "sendMessage" {
		return nil
	}
	to, ok := args["to"].(string)
	if !ok || to == "" {
		return nil
	}
	accountID, _ := args["accountId"].(string)
	return &ToolSendTarget{To: to, AccountID: strings.TrimSpace(accountID)}
}

// ── Discord 动作集 ──

// DiscordListActions Discord 可用动作列表
func DiscordListActions(gate ActionGateFunc) []MessageActionName {
	actions := []MessageActionName{ActionSend}
	if gate("polls") {
		actions = append(actions, "poll")
	}
	if gate("reactions") {
		actions = append(actions, "react", "reactions", "emoji-list")
	}
	if gate("messages") {
		actions = append(actions, "read", "edit", "delete")
	}
	if gate("pins") {
		actions = append(actions, "pin", "unpin", "list-pins")
	}
	if gate("permissions") {
		actions = append(actions, "permissions")
	}
	if gate("threads") {
		actions = append(actions, "thread-create", "thread-list", "thread-reply")
	}
	if gate("search") {
		actions = append(actions, "search")
	}
	if gate("stickers") {
		actions = append(actions, "sticker")
	}
	if gate("memberInfo") {
		actions = append(actions, "member-info")
	}
	if gate("roleInfo") {
		actions = append(actions, "role-info")
	}
	if gate("emojiUploads") {
		actions = append(actions, "emoji-upload")
	}
	if gate("stickerUploads") {
		actions = append(actions, "sticker-upload")
	}
	if gate("roles", false) {
		actions = append(actions, "role-add", "role-remove")
	}
	if gate("channelInfo") {
		actions = append(actions, "channel-info", "channel-list")
	}
	if gate("channels") {
		actions = append(actions, "channel-create", "channel-edit", "channel-delete",
			"channel-move", "category-create", "category-edit", "category-delete")
	}
	if gate("voiceStatus") {
		actions = append(actions, "voice-status")
	}
	if gate("events") {
		actions = append(actions, "event-list", "event-create")
	}
	if gate("moderation", false) {
		actions = append(actions, "timeout", "kick", "ban")
	}
	if gate("presence", false) {
		actions = append(actions, "set-presence")
	}
	return actions
}

// DiscordExtractToolSend 从 Discord 工具参数提取发送目标
func DiscordExtractToolSend(action string, args map[string]interface{}) *ToolSendTarget {
	if action == "sendMessage" {
		to, ok := args["to"].(string)
		if ok && to != "" {
			return &ToolSendTarget{To: to}
		}
		return nil
	}
	if action == "threadReply" {
		channelID, ok := args["channelId"].(string)
		if ok && strings.TrimSpace(channelID) != "" {
			return &ToolSendTarget{To: "channel:" + strings.TrimSpace(channelID)}
		}
		return nil
	}
	return nil
}

// ── Signal 动作集 ──

// SignalListActions Signal 可用动作列表
func SignalListActions(gate ActionGateFunc) []MessageActionName {
	actions := []MessageActionName{ActionSend}
	if gate("reactions") {
		actions = append(actions, "react")
	}
	return actions
}

// NormalizeSignalReactionRecipient 规范化 Signal 反应接收者
func NormalizeSignalReactionRecipient(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	s := trimmed
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "signal:") {
		s = strings.TrimSpace(s[len("signal:"):])
		if s == "" {
			return ""
		}
	}
	lower = strings.ToLower(s)
	if strings.HasPrefix(lower, "uuid:") {
		return strings.TrimSpace(s[len("uuid:"):])
	}
	return s
}

// ResolveSignalReactionTarget 解析 Signal 反应目标
func ResolveSignalReactionTarget(raw string) (recipient, groupID string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ""
	}
	s := trimmed
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "signal:") {
		s = strings.TrimSpace(s[len("signal:"):])
		if s == "" {
			return "", ""
		}
	}
	lower = strings.ToLower(s)
	if strings.HasPrefix(lower, "group:") {
		id := strings.TrimSpace(s[len("group:"):])
		if id != "" {
			return "", id
		}
		return "", ""
	}
	return NormalizeSignalReactionRecipient(s), ""
}

// ── 统一注册表 ──

// GetChannelMessageActionAdapter 获取频道消息动作适配器
func GetChannelMessageActionAdapter(channelID ChannelID) *MessageActionAdapter {
	switch channelID {
	case ChannelTelegram:
		return &MessageActionAdapter{
			ChannelID:       ChannelTelegram,
			ListActions:     TelegramListActions,
			ExtractToolSend: TelegramExtractToolSend,
		}
	case ChannelDiscord:
		return &MessageActionAdapter{
			ChannelID:       ChannelDiscord,
			ListActions:     DiscordListActions,
			ExtractToolSend: DiscordExtractToolSend,
		}
	case ChannelSignal:
		return &MessageActionAdapter{
			ChannelID:   ChannelSignal,
			ListActions: SignalListActions,
		}
	default:
		return nil
	}
}

// ListAllChannelMessageActions 列出所有频道的消息动作
func ListAllChannelMessageActions() map[ChannelID][]MessageActionName {
	result := make(map[ChannelID][]MessageActionName)
	for _, ch := range []ChannelID{ChannelTelegram, ChannelDiscord, ChannelSignal} {
		adapter := GetChannelMessageActionAdapter(ch)
		if adapter != nil {
			// 默认全部启用
			gate := NewActionGate(nil)
			result[ch] = adapter.ListActions(gate)
		}
	}
	return result
}

// UnsupportedActionError 不支持的动作错误
func UnsupportedActionError(channelID ChannelID, action MessageActionName) error {
	return fmt.Errorf("action %s is not supported for channel %s", action, channelID)
}
