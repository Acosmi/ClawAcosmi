package types

// Discord 频道配置类型 — 继承自 src/config/types.discord.ts (168 行)

// DiscordDmConfig Discord 私聊配置
type DiscordDmConfig struct {
	Enabled       *bool         `json:"enabled,omitempty"`
	Policy        DmPolicy      `json:"policy,omitempty"`
	AllowFrom     []interface{} `json:"allowFrom,omitempty"` // string|number
	GroupEnabled  *bool         `json:"groupEnabled,omitempty"`
	GroupChannels []interface{} `json:"groupChannels,omitempty"` // string|number
}

// DiscordGuildChannelConfig Discord 频道级别配置
type DiscordGuildChannelConfig struct {
	Allow                *bool                         `json:"allow,omitempty"`
	RequireMention       *bool                         `json:"requireMention,omitempty"`
	Tools                *GroupToolPolicyConfig        `json:"tools,omitempty"`
	ToolsBySender        GroupToolPolicyBySenderConfig `json:"toolsBySender,omitempty"`
	Skills               []string                      `json:"skills,omitempty"`
	Enabled              *bool                         `json:"enabled,omitempty"`
	Users                []interface{}                 `json:"users,omitempty"` // string|number
	SystemPrompt         string                        `json:"systemPrompt,omitempty"`
	IncludeThreadStarter *bool                         `json:"includeThreadStarter,omitempty"`
}

// DiscordReactionNotificationMode 反应通知模式
type DiscordReactionNotificationMode string

const (
	DiscordReactionOff       DiscordReactionNotificationMode = "off"
	DiscordReactionOwn       DiscordReactionNotificationMode = "own"
	DiscordReactionAll       DiscordReactionNotificationMode = "all"
	DiscordReactionAllowlist DiscordReactionNotificationMode = "allowlist"
)

// DiscordGuildEntry Discord 服务器配置
type DiscordGuildEntry struct {
	Slug                  string                                `json:"slug,omitempty"`
	RequireMention        *bool                                 `json:"requireMention,omitempty"`
	Tools                 *GroupToolPolicyConfig                `json:"tools,omitempty"`
	ToolsBySender         GroupToolPolicyBySenderConfig         `json:"toolsBySender,omitempty"`
	ReactionNotifications DiscordReactionNotificationMode       `json:"reactionNotifications,omitempty"`
	Users                 []interface{}                         `json:"users,omitempty"`
	Channels              map[string]*DiscordGuildChannelConfig `json:"channels,omitempty"`
}

// DiscordActionConfig Discord 操作权限配置
type DiscordActionConfig struct {
	Reactions      *bool `json:"reactions,omitempty"`
	Stickers       *bool `json:"stickers,omitempty"`
	Polls          *bool `json:"polls,omitempty"`
	Permissions    *bool `json:"permissions,omitempty"`
	Messages       *bool `json:"messages,omitempty"`
	Threads        *bool `json:"threads,omitempty"`
	Pins           *bool `json:"pins,omitempty"`
	Search         *bool `json:"search,omitempty"`
	MemberInfo     *bool `json:"memberInfo,omitempty"`
	RoleInfo       *bool `json:"roleInfo,omitempty"`
	Roles          *bool `json:"roles,omitempty"`
	ChannelInfo    *bool `json:"channelInfo,omitempty"`
	VoiceStatus    *bool `json:"voiceStatus,omitempty"`
	Events         *bool `json:"events,omitempty"`
	Moderation     *bool `json:"moderation,omitempty"`
	EmojiUploads   *bool `json:"emojiUploads,omitempty"`
	StickerUploads *bool `json:"stickerUploads,omitempty"`
	Channels       *bool `json:"channels,omitempty"`
	Presence       *bool `json:"presence,omitempty"`
}

// DiscordIntentsConfig Discord 特权意图配置
type DiscordIntentsConfig struct {
	Presence     *bool `json:"presence,omitempty"`
	GuildMembers *bool `json:"guildMembers,omitempty"`
}

// DiscordExecApprovalConfig Discord 执行审批配置
type DiscordExecApprovalConfig struct {
	Enabled       *bool         `json:"enabled,omitempty"`
	Approvers     []interface{} `json:"approvers,omitempty"` // string|number
	AgentFilter   []string      `json:"agentFilter,omitempty"`
	SessionFilter []string      `json:"sessionFilter,omitempty"`
}

// DiscordAccountConfig Discord 账号配置
// DY-001: Name, Token, GroupPolicy, ChunkMode, ReplyToMode, ResponsePrefix 使用
// *string 指针类型，以区分 "未设置" 和 "设为空字符串"，对齐 TS `{ ...base, ...account }` 展开合并语义。
type DiscordAccountConfig struct {
	Name                   *string                           `json:"name,omitempty"`
	Capabilities           []string                          `json:"capabilities,omitempty"`
	Markdown               *MarkdownConfig                   `json:"markdown,omitempty"`
	Commands               *ProviderCommandsConfig           `json:"commands,omitempty"`
	ConfigWrites           *bool                             `json:"configWrites,omitempty"`
	Enabled                *bool                             `json:"enabled,omitempty"`
	Token                  *string                           `json:"token,omitempty"`
	AllowBots              *bool                             `json:"allowBots,omitempty"`
	GroupPolicy            *GroupPolicy                      `json:"groupPolicy,omitempty"`
	TextChunkLimit         *int                              `json:"textChunkLimit,omitempty"`
	ChunkMode              *string                           `json:"chunkMode,omitempty"` // "length"|"newline"
	BlockStreaming         *bool                             `json:"blockStreaming,omitempty"`
	BlockStreamingCoalesce *BlockStreamingCoalesceConfig     `json:"blockStreamingCoalesce,omitempty"`
	MaxLinesPerMessage     *int                              `json:"maxLinesPerMessage,omitempty"`
	MediaMaxMB             *int                              `json:"mediaMaxMb,omitempty"`
	HistoryLimit           *int                              `json:"historyLimit,omitempty"`
	DmHistoryLimit         *int                              `json:"dmHistoryLimit,omitempty"`
	Dms                    map[string]*DmConfig              `json:"dms,omitempty"`
	Retry                  *OutboundRetryConfig              `json:"retry,omitempty"`
	Actions                *DiscordActionConfig              `json:"actions,omitempty"`
	ReplyToMode            *ReplyToMode                      `json:"replyToMode,omitempty"`
	DM                     *DiscordDmConfig                  `json:"dm,omitempty"`
	Guilds                 map[string]*DiscordGuildEntry     `json:"guilds,omitempty"`
	Heartbeat              *ChannelHeartbeatVisibilityConfig `json:"heartbeat,omitempty"`
	ExecApprovals          *DiscordExecApprovalConfig        `json:"execApprovals,omitempty"`
	Intents                *DiscordIntentsConfig             `json:"intents,omitempty"`
	Pluralkit              *DiscordPluralKitConfig           `json:"pluralkit,omitempty"`
	ResponsePrefix         *string                           `json:"responsePrefix,omitempty"`
}

// DiscordAccountConfigName 安全读取 Name 字段，nil 返回空字符串。
func (c *DiscordAccountConfig) DiscordAccountConfigName() string {
	if c == nil || c.Name == nil {
		return ""
	}
	return *c.Name
}

// DiscordAccountConfigToken 安全读取 Token 字段，nil 返回空字符串。
func (c *DiscordAccountConfig) DiscordAccountConfigToken() string {
	if c == nil || c.Token == nil {
		return ""
	}
	return *c.Token
}

// DiscordAccountConfigGroupPolicy 安全读取 GroupPolicy 字段，nil 返回零值。
func (c *DiscordAccountConfig) DiscordAccountConfigGroupPolicy() GroupPolicy {
	if c == nil || c.GroupPolicy == nil {
		return ""
	}
	return *c.GroupPolicy
}

// DiscordAccountConfigChunkMode 安全读取 ChunkMode 字段，nil 返回空字符串。
func (c *DiscordAccountConfig) DiscordAccountConfigChunkMode() string {
	if c == nil || c.ChunkMode == nil {
		return ""
	}
	return *c.ChunkMode
}

// DiscordAccountConfigReplyToMode 安全读取 ReplyToMode 字段，nil 返回零值。
func (c *DiscordAccountConfig) DiscordAccountConfigReplyToMode() ReplyToMode {
	if c == nil || c.ReplyToMode == nil {
		return ""
	}
	return *c.ReplyToMode
}

// DiscordAccountConfigResponsePrefix 安全读取 ResponsePrefix 字段，nil 返回空字符串。
func (c *DiscordAccountConfig) DiscordAccountConfigResponsePrefix() string {
	if c == nil || c.ResponsePrefix == nil {
		return ""
	}
	return *c.ResponsePrefix
}

// StringPtr 返回字符串指针的辅助函数。
func StringPtr(s string) *string {
	return &s
}

// GroupPolicyPtr 返回 GroupPolicy 指针的辅助函数。
func GroupPolicyPtr(g GroupPolicy) *GroupPolicy {
	return &g
}

// ReplyToModePtr 返回 ReplyToMode 指针的辅助函数。
func ReplyToModePtr(r ReplyToMode) *ReplyToMode {
	return &r
}

// DiscordConfig Discord 总配置（嵌入 DiscordAccountConfig 实现 TS 的 & 交叉类型）
type DiscordConfig struct {
	DiscordAccountConfig
	Accounts map[string]*DiscordAccountConfig `json:"accounts,omitempty"`
}
