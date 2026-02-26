package channels

// 消息动作分发 — 继承自 src/channels/plugins/message-actions.ts (50L)

// MessageActionName 消息动作名称
type MessageActionName string

const (
	ActionSend      MessageActionName = "send"
	ActionBroadcast MessageActionName = "broadcast"
)

// MessageActionContext 消息动作上下文
type MessageActionContext struct {
	Channel   ChannelID
	Action    MessageActionName
	Params    map[string]interface{}
	AccountID string
}

// PluginActionProvider 插件动作提供器（DI 注入）。
// 返回额外的消息动作名称列表。
var PluginActionProvider func() []MessageActionName

// PluginMessageButtonsProvider 插件消息按钮能力检测（DI 注入）。
var PluginMessageButtonsProvider func() bool

// PluginMessageCardsProvider 插件消息卡片能力检测（DI 注入）。
var PluginMessageCardsProvider func() bool

// ListChannelMessageActions 列出可用消息动作（基础 + 插件扩展）
func ListChannelMessageActions() []MessageActionName {
	actions := []MessageActionName{ActionSend, ActionBroadcast}
	if PluginActionProvider != nil {
		actions = append(actions, PluginActionProvider()...)
	}
	return actions
}

// SupportsChannelMessageButtons 检查是否有插件支持消息按钮
func SupportsChannelMessageButtons() bool {
	if PluginMessageButtonsProvider != nil {
		return PluginMessageButtonsProvider()
	}
	return false
}

// SupportsChannelMessageCards 检查是否有插件支持消息卡片
func SupportsChannelMessageCards() bool {
	if PluginMessageCardsProvider != nil {
		return PluginMessageCardsProvider()
	}
	return false
}
