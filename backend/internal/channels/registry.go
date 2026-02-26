package channels

import "strings"

// 频道注册表 — 继承自 src/channels/registry.ts (180 行)
// 元数据 + 别名 + 排序 + 规范化

// chatChannelOrder 频道展示排序
var chatChannelOrder = []ChannelID{
	ChannelTelegram,
	ChannelWhatsApp,
	ChannelDiscord,
	ChannelSlack,
	ChannelSignal,
	ChannelIMessage,
	ChannelGoogleChat,
	ChannelMSTeams,
	ChannelWeb,
	ChannelFeishu,
	ChannelDingTalk,
	ChannelWeCom,
}

// channelAliases 频道别名映射
var channelAliases = map[string]ChannelID{
	"tg":     ChannelTelegram,
	"wa":     ChannelWhatsApp,
	"dc":     ChannelDiscord,
	"gchat":  ChannelGoogleChat,
	"google": ChannelGoogleChat,
	"imsg":   ChannelIMessage,
	"teams":  ChannelMSTeams,
	"lark":   ChannelFeishu,
	"ding":   ChannelDingTalk,
	"wechat": ChannelWeCom,
	"wxwork": ChannelWeCom,
}

// ChannelMetaEntry 频道元数据条目
type ChannelMetaEntry struct {
	ID             ChannelID
	Label          string
	SelectionLabel string
	DetailLabel    string
	DocsPath       string
	DocsLabel      string
	Blurb          string
	SystemImage    string
	Order          int
}

// chatChannelMeta 核心频道元数据
var chatChannelMeta = map[ChannelID]ChannelMetaEntry{
	ChannelTelegram: {
		ID: ChannelTelegram, Label: "Telegram", SelectionLabel: "Telegram (Bot API)",
		DetailLabel: "Telegram Bot", DocsPath: "/channels/telegram", DocsLabel: "telegram",
		Blurb: "simplest way to get started.", SystemImage: "paperplane", Order: 0,
	},
	ChannelWhatsApp: {
		ID: ChannelWhatsApp, Label: "WhatsApp", SelectionLabel: "WhatsApp",
		DocsPath: "/channels/whatsapp", DocsLabel: "whatsapp",
		Blurb: "connect to WhatsApp Web.", SystemImage: "phone", Order: 1,
	},
	ChannelDiscord: {
		ID: ChannelDiscord, Label: "Discord", SelectionLabel: "Discord",
		DocsPath: "/channels/discord", DocsLabel: "discord",
		Blurb: "add your bot to Discord servers.", SystemImage: "gamecontroller", Order: 2,
	},
	ChannelSlack: {
		ID: ChannelSlack, Label: "Slack", SelectionLabel: "Slack",
		DocsPath: "/channels/slack", DocsLabel: "slack",
		Blurb: "integrate with Slack workspaces.", SystemImage: "number", Order: 3,
	},
	ChannelSignal: {
		ID: ChannelSignal, Label: "Signal", SelectionLabel: "Signal",
		DocsPath: "/channels/signal", DocsLabel: "signal",
		Blurb: "connect via signal-cli.", SystemImage: "lock.shield", Order: 4,
	},
	ChannelIMessage: {
		ID: ChannelIMessage, Label: "iMessage", SelectionLabel: "iMessage (BlueBubbles)",
		DocsPath: "/channels/imessage", DocsLabel: "imessage",
		Blurb: "connect via BlueBubbles.", SystemImage: "message", Order: 5,
	},
	ChannelGoogleChat: {
		ID: ChannelGoogleChat, Label: "Google Chat", SelectionLabel: "Google Chat",
		DocsPath: "/channels/googlechat", DocsLabel: "googlechat",
		Blurb: "integrate with Google Chat.", SystemImage: "bubble.left.and.bubble.right", Order: 6,
	},
	ChannelMSTeams: {
		ID: ChannelMSTeams, Label: "Microsoft Teams", SelectionLabel: "Microsoft Teams",
		DocsPath: "/channels/msteams", DocsLabel: "msteams",
		Blurb: "connect to Microsoft Teams.", SystemImage: "person.2", Order: 7,
	},
	ChannelWeb: {
		ID: ChannelWeb, Label: "Web", SelectionLabel: "Web Dashboard",
		DocsPath: "/channels/web", DocsLabel: "web",
		Blurb: "built-in web chat interface.", SystemImage: "globe", Order: 8,
	},
	ChannelFeishu: {
		ID: ChannelFeishu, Label: "飞书", SelectionLabel: "飞书 / Feishu (Lark)",
		DetailLabel: "Feishu Bot", DocsPath: "/channels/feishu", DocsLabel: "feishu",
		Blurb: "connect to Feishu/Lark workspace.", SystemImage: "bubble.left.and.text.bubble.right", Order: 9,
	},
	ChannelDingTalk: {
		ID: ChannelDingTalk, Label: "钉钉", SelectionLabel: "钉钉 / DingTalk",
		DetailLabel: "DingTalk Bot", DocsPath: "/channels/dingtalk", DocsLabel: "dingtalk",
		Blurb: "connect to DingTalk workspace.", SystemImage: "bell", Order: 10,
	},
	ChannelWeCom: {
		ID: ChannelWeCom, Label: "企业微信", SelectionLabel: "企业微信 / WeCom",
		DetailLabel: "WeCom App", DocsPath: "/channels/wecom", DocsLabel: "wecom",
		Blurb: "connect to WeCom (WeChat Work).", SystemImage: "person.crop.circle.badge.checkmark", Order: 11,
	},
}

// ResolveChannelIDByAlias 通过别名解析频道 ID
func ResolveChannelIDByAlias(raw string) ChannelID {
	lower := strings.TrimSpace(strings.ToLower(raw))
	if _, ok := chatChannelMeta[ChannelID(lower)]; ok {
		return ChannelID(lower)
	}
	if mapped, ok := channelAliases[lower]; ok {
		return mapped
	}
	return ""
}

// ListChatChannelIDs 按排序返回核心频道 ID 列表
func ListChatChannelIDs() []ChannelID {
	return chatChannelOrder
}

// GetChannelMeta 获取频道元数据
func GetChannelMeta(id ChannelID) (ChannelMetaEntry, bool) {
	m, ok := chatChannelMeta[id]
	return m, ok
}

// IsCoreChannel 判断是否为核心频道
func IsCoreChannel(id string) bool {
	resolved := ResolveChannelIDByAlias(id)
	_, ok := chatChannelMeta[resolved]
	return ok
}
