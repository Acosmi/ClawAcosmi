package types

// Slack 频道配置类型 — 继承自 src/config/types.slack.ts (153 行)

// SlackDmConfig Slack 私聊配置
type SlackDmConfig struct {
	Enabled       *bool         `json:"enabled,omitempty"`
	Policy        DmPolicy      `json:"policy,omitempty"`
	AllowFrom     []interface{} `json:"allowFrom,omitempty"`
	GroupEnabled  *bool         `json:"groupEnabled,omitempty"`
	GroupChannels []interface{} `json:"groupChannels,omitempty"`
	ReplyToMode   ReplyToMode   `json:"replyToMode,omitempty"` // @deprecated
}

// SlackChannelConfig Slack 频道级别配置
type SlackChannelConfig struct {
	Enabled        *bool                         `json:"enabled,omitempty"`
	Allow          *bool                         `json:"allow,omitempty"`
	RequireMention *bool                         `json:"requireMention,omitempty"`
	Tools          *GroupToolPolicyConfig        `json:"tools,omitempty"`
	ToolsBySender  GroupToolPolicyBySenderConfig `json:"toolsBySender,omitempty"`
	AllowBots      *bool                         `json:"allowBots,omitempty"`
	Users          []interface{}                 `json:"users,omitempty"`
	Skills         []string                      `json:"skills,omitempty"`
	SystemPrompt   string                        `json:"systemPrompt,omitempty"`
}

// SlackReactionNotificationMode Slack 反应通知模式
type SlackReactionNotificationMode string

const (
	SlackReactionOff       SlackReactionNotificationMode = "off"
	SlackReactionOwn       SlackReactionNotificationMode = "own"
	SlackReactionAll       SlackReactionNotificationMode = "all"
	SlackReactionAllowlist SlackReactionNotificationMode = "allowlist"
)

// SlackActionConfig Slack 操作权限配置
type SlackActionConfig struct {
	Reactions   *bool `json:"reactions,omitempty"`
	Messages    *bool `json:"messages,omitempty"`
	Pins        *bool `json:"pins,omitempty"`
	Search      *bool `json:"search,omitempty"`
	Permissions *bool `json:"permissions,omitempty"`
	MemberInfo  *bool `json:"memberInfo,omitempty"`
	ChannelInfo *bool `json:"channelInfo,omitempty"`
	EmojiList   *bool `json:"emojiList,omitempty"`
}

// SlackSlashCommandConfig Slack 斜杠命令配置
type SlackSlashCommandConfig struct {
	Enabled       *bool  `json:"enabled,omitempty"`
	Name          string `json:"name,omitempty"`
	SessionPrefix string `json:"sessionPrefix,omitempty"`
	Ephemeral     *bool  `json:"ephemeral,omitempty"`
}

// SlackThreadConfig Slack 线程配置
type SlackThreadConfig struct {
	HistoryScope  string `json:"historyScope,omitempty"` // "thread"|"channel"
	InheritParent *bool  `json:"inheritParent,omitempty"`
}

// SlackReplyToModeByChatType 按聊天类型的回复线程模式
type SlackReplyToModeByChatType struct {
	Direct  ReplyToMode `json:"direct,omitempty"`
	Group   ReplyToMode `json:"group,omitempty"`
	Channel ReplyToMode `json:"channel,omitempty"`
}

// SlackAccountConfig Slack 账号配置
type SlackAccountConfig struct {
	Name                   string                            `json:"name,omitempty"`
	Mode                   string                            `json:"mode,omitempty"` // "socket"|"http"
	SigningSecret          string                            `json:"signingSecret,omitempty"`
	WebhookPath            string                            `json:"webhookPath,omitempty"`
	Capabilities           []string                          `json:"capabilities,omitempty"`
	Markdown               *MarkdownConfig                   `json:"markdown,omitempty"`
	Commands               *ProviderCommandsConfig           `json:"commands,omitempty"`
	ConfigWrites           *bool                             `json:"configWrites,omitempty"`
	Enabled                *bool                             `json:"enabled,omitempty"`
	BotToken               string                            `json:"botToken,omitempty"`
	AppToken               string                            `json:"appToken,omitempty"`
	UserToken              string                            `json:"userToken,omitempty"`
	UserTokenReadOnly      *bool                             `json:"userTokenReadOnly,omitempty"`
	AllowBots              *bool                             `json:"allowBots,omitempty"`
	RequireMention         *bool                             `json:"requireMention,omitempty"`
	GroupPolicy            GroupPolicy                       `json:"groupPolicy,omitempty"`
	HistoryLimit           *int                              `json:"historyLimit,omitempty"`
	DmHistoryLimit         *int                              `json:"dmHistoryLimit,omitempty"`
	Dms                    map[string]*DmConfig              `json:"dms,omitempty"`
	TextChunkLimit         *int                              `json:"textChunkLimit,omitempty"`
	ChunkMode              string                            `json:"chunkMode,omitempty"`
	BlockStreaming         *bool                             `json:"blockStreaming,omitempty"`
	BlockStreamingCoalesce *BlockStreamingCoalesceConfig     `json:"blockStreamingCoalesce,omitempty"`
	MediaMaxMB             *int                              `json:"mediaMaxMb,omitempty"`
	ReactionNotifications  SlackReactionNotificationMode     `json:"reactionNotifications,omitempty"`
	ReactionAllowlist      []interface{}                     `json:"reactionAllowlist,omitempty"`
	ReplyToMode            ReplyToMode                       `json:"replyToMode,omitempty"`
	ReplyToModeByChatType  *SlackReplyToModeByChatType       `json:"replyToModeByChatType,omitempty"`
	Thread                 *SlackThreadConfig                `json:"thread,omitempty"`
	Actions                *SlackActionConfig                `json:"actions,omitempty"`
	SlashCommand           *SlackSlashCommandConfig          `json:"slashCommand,omitempty"`
	DM                     *SlackDmConfig                    `json:"dm,omitempty"`
	Channels               map[string]*SlackChannelConfig    `json:"channels,omitempty"`
	Heartbeat              *ChannelHeartbeatVisibilityConfig `json:"heartbeat,omitempty"`
	ResponsePrefix         string                            `json:"responsePrefix,omitempty"`
}

// SlackConfig Slack 总配置
type SlackConfig struct {
	SlackAccountConfig
	Accounts map[string]*SlackAccountConfig `json:"accounts,omitempty"`
}
