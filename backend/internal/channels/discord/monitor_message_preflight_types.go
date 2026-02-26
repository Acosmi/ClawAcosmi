package discord

// Discord 消息预检类型 — 继承自 src/discord/monitor/message-handler.preflight.types.ts (104L)
// 定义预检输入参数和输出上下文的完整类型。

import (
	"github.com/bwmarrin/discordgo"
)

// DiscordMessagePreflightContext 预检通过后的完整上下文。
// TS ref: DiscordMessagePreflightContext
type DiscordMessagePreflightContext struct {
	// 配置
	AccountID string
	Token     string
	BotUserID string

	// Discord 配置
	DMPolicy    string // "open" | "allowlist" | "pairing" | "disabled"
	GroupPolicy string // "open" | "disabled" | "allowlist"

	// 原始数据
	Message *discordgo.MessageCreate
	Session *discordgo.Session

	// 发送者
	Sender DiscordSenderIdentity

	// 频道信息
	ChannelName string
	ChannelType int // discordgo channel type

	// 消息分类
	IsGuildMessage  bool
	IsDirectMessage bool
	IsGroupDm       bool

	// 授权
	CommandAuthorized bool

	// 文本
	BaseText    string // mention tag 清理后的文本
	MessageText string // 完整消息文本（包含转发）

	// Mention
	WasMentioned          bool
	EffectiveWasMentioned bool
	HasAnyMention         bool
	ShouldRequireMention  bool
	ShouldBypassMention   bool
	CanDetectMention      bool

	// 路由
	Route *DiscordAgentRoute

	// Guild 信息
	GuildInfo *DiscordGuildEntryResolved
	GuildSlug string

	// 线程信息
	ThreadChannel    *DiscordThreadChannel
	ThreadParentID   string
	ThreadParentName string
	ThreadParentType *int
	ThreadName       string

	// 频道配置
	ConfigChannelName          string
	ConfigChannelSlug          string
	DisplayChannelName         string
	DisplayChannelSlug         string
	BaseSessionKey             string
	ChannelConfig              *DiscordChannelConfigResolved
	ChannelAllowlistConfigured bool
	ChannelAllowed             bool

	// 文本命令
	AllowTextCommands bool

	// 历史
	HistoryLimit   int
	GuildHistories map[string][]DiscordHistoryEntry

	// 媒体
	MediaMaxBytes int
	TextLimit     int

	// 回复模式
	ReplyToMode      string // "always" | "first" | "never"
	AckReactionScope string // "all" | "direct" | "group-all" | "group-mentions"

	// DI 依赖
	Deps *DiscordMonitorDeps
}

// DiscordHistoryEntry 历史条目
type DiscordHistoryEntry struct {
	Sender    string `json:"sender"`
	Body      string `json:"body"`
	Timestamp int64  `json:"timestamp"`
	MessageID string `json:"messageId,omitempty"`
}

// DiscordChannelConfigResolved is defined in monitor_allow_list.go — not redeclared here.

// DiscordMessagePreflightParams 预检输入参数。
// TS ref: DiscordMessagePreflightParams
type DiscordMessagePreflightParams struct {
	AccountID        string
	Token            string
	BotUserID        string
	DMEnabled        bool
	GroupDmEnabled   bool
	DMPolicy         string
	GroupPolicy      string
	AllowFrom        []string
	GroupDmChannels  []string
	GuildEntries     map[string]DiscordGuildEntryResolved
	HistoryLimit     int
	MediaMaxBytes    int
	TextLimit        int
	ReplyToMode      string
	AckReactionScope string
	RequireMention   bool
	AllowBots        bool

	// DI
	Deps    *DiscordMonitorDeps
	Session *discordgo.Session
}
