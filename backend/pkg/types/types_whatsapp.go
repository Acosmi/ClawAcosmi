package types

// WhatsApp 频道配置类型 — 继承自 src/config/types.whatsapp.ts (172 行)

// WhatsAppActionConfig WhatsApp 操作权限配置
type WhatsAppActionConfig struct {
	Reactions   *bool `json:"reactions,omitempty"`
	SendMessage *bool `json:"sendMessage,omitempty"`
	Polls       *bool `json:"polls,omitempty"`
}

// WhatsAppGroupConfig WhatsApp 群组配置
type WhatsAppGroupConfig struct {
	RequireMention *bool                         `json:"requireMention,omitempty"`
	SilentToken    string                        `json:"silentToken,omitempty"` // silent_token 模式激活词
	Tools          *GroupToolPolicyConfig        `json:"tools,omitempty"`
	ToolsBySender  GroupToolPolicyBySenderConfig `json:"toolsBySender,omitempty"`
}

// WhatsAppAckReactionConfig WhatsApp 确认反应配置
type WhatsAppAckReactionConfig struct {
	Emoji  string `json:"emoji,omitempty"`
	Direct *bool  `json:"direct,omitempty"`
	Group  string `json:"group,omitempty"` // "always"|"mentions"|"never"
}

// WhatsAppAccountConfig WhatsApp 账号配置
type WhatsAppAccountConfig struct {
	Name                   string                            `json:"name,omitempty"`
	Capabilities           []string                          `json:"capabilities,omitempty"`
	Markdown               *MarkdownConfig                   `json:"markdown,omitempty"`
	ConfigWrites           *bool                             `json:"configWrites,omitempty"`
	Enabled                *bool                             `json:"enabled,omitempty"`
	SendReadReceipts       *bool                             `json:"sendReadReceipts,omitempty"`
	MessagePrefix          string                            `json:"messagePrefix,omitempty"`
	ResponsePrefix         string                            `json:"responsePrefix,omitempty"`
	AuthDir                string                            `json:"authDir,omitempty"`
	DmPolicy               DmPolicy                          `json:"dmPolicy,omitempty"`
	SelfChatMode           *bool                             `json:"selfChatMode,omitempty"`
	AllowFrom              []string                          `json:"allowFrom,omitempty"`
	GroupAllowFrom         []string                          `json:"groupAllowFrom,omitempty"`
	GroupPolicy            GroupPolicy                       `json:"groupPolicy,omitempty"`
	GroupSilentToken       string                            `json:"groupSilentToken,omitempty"` // silent_token 模式的全局激活词
	HistoryLimit           *int                              `json:"historyLimit,omitempty"`
	DmHistoryLimit         *int                              `json:"dmHistoryLimit,omitempty"`
	Dms                    map[string]*DmConfig              `json:"dms,omitempty"`
	TextChunkLimit         *int                              `json:"textChunkLimit,omitempty"`
	ChunkMode              string                            `json:"chunkMode,omitempty"`
	MediaMaxMB             *int                              `json:"mediaMaxMb,omitempty"`
	BlockStreaming         *bool                             `json:"blockStreaming,omitempty"`
	BlockStreamingCoalesce *BlockStreamingCoalesceConfig     `json:"blockStreamingCoalesce,omitempty"`
	Groups                 map[string]*WhatsAppGroupConfig   `json:"groups,omitempty"`
	AckReaction            *WhatsAppAckReactionConfig        `json:"ackReaction,omitempty"`
	Actions                *WhatsAppActionConfig             `json:"actions,omitempty"`
	DebounceMs             *int                              `json:"debounceMs,omitempty"`
	Heartbeat              *ChannelHeartbeatVisibilityConfig `json:"heartbeat,omitempty"`
}

// WhatsAppConfig WhatsApp 总配置
type WhatsAppConfig struct {
	WhatsAppAccountConfig
	Accounts map[string]*WhatsAppAccountConfig `json:"accounts,omitempty"`
}
