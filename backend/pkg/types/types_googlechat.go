package types

// Google Chat 频道配置类型 — 继承自 src/config/types.googlechat.ts (111 行)

// GoogleChatDmConfig Google Chat 私聊配置
type GoogleChatDmConfig struct {
	Enabled   *bool         `json:"enabled,omitempty"`
	Policy    DmPolicy      `json:"policy,omitempty"`
	AllowFrom []interface{} `json:"allowFrom,omitempty"`
}

// GoogleChatGroupConfig Google Chat 群组配置
type GoogleChatGroupConfig struct {
	Enabled        *bool         `json:"enabled,omitempty"`
	Allow          *bool         `json:"allow,omitempty"`
	RequireMention *bool         `json:"requireMention,omitempty"`
	Users          []interface{} `json:"users,omitempty"`
	SystemPrompt   string        `json:"systemPrompt,omitempty"`
}

// GoogleChatActionConfig Google Chat 操作配置
type GoogleChatActionConfig struct {
	Reactions *bool `json:"reactions,omitempty"`
}

// GoogleChatAccountConfig Google Chat 账号配置
type GoogleChatAccountConfig struct {
	Name                   string                            `json:"name,omitempty"`
	Capabilities           []string                          `json:"capabilities,omitempty"`
	ConfigWrites           *bool                             `json:"configWrites,omitempty"`
	Enabled                *bool                             `json:"enabled,omitempty"`
	AllowBots              *bool                             `json:"allowBots,omitempty"`
	RequireMention         *bool                             `json:"requireMention,omitempty"`
	GroupPolicy            GroupPolicy                       `json:"groupPolicy,omitempty"`
	GroupAllowFrom         []interface{}                     `json:"groupAllowFrom,omitempty"`
	Groups                 map[string]*GoogleChatGroupConfig `json:"groups,omitempty"`
	ServiceAccount         interface{}                       `json:"serviceAccount,omitempty"` // string 或 object
	ServiceAccountFile     string                            `json:"serviceAccountFile,omitempty"`
	AudienceType           string                            `json:"audienceType,omitempty"` // "app-url"|"project-number"
	Audience               string                            `json:"audience,omitempty"`
	WebhookPath            string                            `json:"webhookPath,omitempty"`
	WebhookURL             string                            `json:"webhookUrl,omitempty"`
	BotUser                string                            `json:"botUser,omitempty"`
	HistoryLimit           *int                              `json:"historyLimit,omitempty"`
	DmHistoryLimit         *int                              `json:"dmHistoryLimit,omitempty"`
	Dms                    map[string]*DmConfig              `json:"dms,omitempty"`
	TextChunkLimit         *int                              `json:"textChunkLimit,omitempty"`
	ChunkMode              string                            `json:"chunkMode,omitempty"`
	BlockStreaming         *bool                             `json:"blockStreaming,omitempty"`
	BlockStreamingCoalesce *BlockStreamingCoalesceConfig     `json:"blockStreamingCoalesce,omitempty"`
	MediaMaxMB             *int                              `json:"mediaMaxMb,omitempty"`
	ReplyToMode            ReplyToMode                       `json:"replyToMode,omitempty"`
	Actions                *GoogleChatActionConfig           `json:"actions,omitempty"`
	DM                     *GoogleChatDmConfig               `json:"dm,omitempty"`
	TypingIndicator        string                            `json:"typingIndicator,omitempty"` // "none"|"message"|"reaction"
	ResponsePrefix         string                            `json:"responsePrefix,omitempty"`
}

// GoogleChatConfig Google Chat 总配置
type GoogleChatConfig struct {
	GoogleChatAccountConfig
	Accounts       map[string]*GoogleChatAccountConfig `json:"accounts,omitempty"`
	DefaultAccount string                              `json:"defaultAccount,omitempty"`
}
