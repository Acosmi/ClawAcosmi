package types

import "encoding/json"

// Telegram 频道配置类型 — 继承自 src/config/types.telegram.ts (180 行)

// TelegramActionConfig Telegram 操作权限配置
type TelegramActionConfig struct {
	Reactions     *bool `json:"reactions,omitempty"`
	SendMessage   *bool `json:"sendMessage,omitempty"`
	DeleteMessage *bool `json:"deleteMessage,omitempty"`
	EditMessage   *bool `json:"editMessage,omitempty"`
	Sticker       *bool `json:"sticker,omitempty"`
}

// TelegramNetworkConfig Telegram 网络配置
type TelegramNetworkConfig struct {
	AutoSelectFamily *bool `json:"autoSelectFamily,omitempty"`
}

// TelegramInlineButtonsScope Telegram 内联按钮范围
type TelegramInlineButtonsScope string

const (
	TelegramInlineOff       TelegramInlineButtonsScope = "off"
	TelegramInlineDM        TelegramInlineButtonsScope = "dm"
	TelegramInlineGroup     TelegramInlineButtonsScope = "group"
	TelegramInlineAll       TelegramInlineButtonsScope = "all"
	TelegramInlineAllowlist TelegramInlineButtonsScope = "allowlist"
)

// TelegramCapabilitiesConfig Telegram 能力配置
// 对齐 TS 联合类型: string[] | { inlineButtons?: TelegramInlineButtonsScope }
type TelegramCapabilitiesConfig struct {
	Tags          []string                   `json:"tags,omitempty"`
	InlineButtons TelegramInlineButtonsScope `json:"inlineButtons,omitempty"`
}

// UnmarshalJSON 自定义反序列化，处理 TS 联合类型 string[] | object。
func (c *TelegramCapabilitiesConfig) UnmarshalJSON(data []byte) error {
	// 先尝试数组形式 ["inlinebuttons", ...]
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		c.Tags = arr
		c.InlineButtons = ""
		return nil
	}
	// 再尝试对象形式 {"inlineButtons": "all"}
	type Alias TelegramCapabilitiesConfig
	var obj Alias
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	*c = TelegramCapabilitiesConfig(obj)
	return nil
}

// TelegramCustomCommand Telegram 自定义命令
type TelegramCustomCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// TelegramAccountConfig Telegram 账号配置
type TelegramAccountConfig struct {
	Name                   string                            `json:"name,omitempty"`
	Capabilities           *TelegramCapabilitiesConfig       `json:"capabilities,omitempty"`
	Markdown               *MarkdownConfig                   `json:"markdown,omitempty"`
	Commands               *ProviderCommandsConfig           `json:"commands,omitempty"`
	CustomCommands         []TelegramCustomCommand           `json:"customCommands,omitempty"`
	ConfigWrites           *bool                             `json:"configWrites,omitempty"`
	DmPolicy               DmPolicy                          `json:"dmPolicy,omitempty"`
	Enabled                *bool                             `json:"enabled,omitempty"`
	BotToken               string                            `json:"botToken,omitempty"`
	TokenFile              string                            `json:"tokenFile,omitempty"`
	ReplyToMode            ReplyToMode                       `json:"replyToMode,omitempty"`
	Groups                 map[string]*TelegramGroupConfig   `json:"groups,omitempty"`
	AllowFrom              []interface{}                     `json:"allowFrom,omitempty"`
	GroupAllowFrom         []interface{}                     `json:"groupAllowFrom,omitempty"`
	GroupPolicy            GroupPolicy                       `json:"groupPolicy,omitempty"`
	HistoryLimit           *int                              `json:"historyLimit,omitempty"`
	DmHistoryLimit         *int                              `json:"dmHistoryLimit,omitempty"`
	Dms                    map[string]*DmConfig              `json:"dms,omitempty"`
	TextChunkLimit         *int                              `json:"textChunkLimit,omitempty"`
	ChunkMode              string                            `json:"chunkMode,omitempty"`
	BlockStreaming         *bool                             `json:"blockStreaming,omitempty"`
	DraftChunk             *BlockStreamingChunkConfig        `json:"draftChunk,omitempty"`
	BlockStreamingCoalesce *BlockStreamingCoalesceConfig     `json:"blockStreamingCoalesce,omitempty"`
	StreamMode             string                            `json:"streamMode,omitempty"` // "off"|"partial"|"block"
	MediaMaxMB             *int                              `json:"mediaMaxMb,omitempty"`
	TimeoutSeconds         *int                              `json:"timeoutSeconds,omitempty"`
	Retry                  *OutboundRetryConfig              `json:"retry,omitempty"`
	Network                *TelegramNetworkConfig            `json:"network,omitempty"`
	Proxy                  string                            `json:"proxy,omitempty"`
	WebhookURL             string                            `json:"webhookUrl,omitempty"`
	WebhookSecret          string                            `json:"webhookSecret,omitempty"`
	WebhookPath            string                            `json:"webhookPath,omitempty"`
	Actions                *TelegramActionConfig             `json:"actions,omitempty"`
	ReactionNotifications  string                            `json:"reactionNotifications,omitempty"` // "off"|"own"|"all"
	ReactionLevel          string                            `json:"reactionLevel,omitempty"`         // "off"|"ack"|"minimal"|"extensive"
	Heartbeat              *ChannelHeartbeatVisibilityConfig `json:"heartbeat,omitempty"`
	LinkPreview            *bool                             `json:"linkPreview,omitempty"`
	ResponsePrefix         string                            `json:"responsePrefix,omitempty"`
}

// TelegramTopicConfig Telegram 话题配置
type TelegramTopicConfig struct {
	RequireMention *bool         `json:"requireMention,omitempty"`
	GroupPolicy    GroupPolicy   `json:"groupPolicy,omitempty"`
	Skills         []string      `json:"skills,omitempty"`
	Enabled        *bool         `json:"enabled,omitempty"`
	AllowFrom      []interface{} `json:"allowFrom,omitempty"`
	SystemPrompt   string        `json:"systemPrompt,omitempty"`
}

// TelegramGroupConfig Telegram 群组配置
type TelegramGroupConfig struct {
	RequireMention *bool                           `json:"requireMention,omitempty"`
	GroupPolicy    GroupPolicy                     `json:"groupPolicy,omitempty"`
	Tools          *GroupToolPolicyConfig          `json:"tools,omitempty"`
	ToolsBySender  GroupToolPolicyBySenderConfig   `json:"toolsBySender,omitempty"`
	Skills         []string                        `json:"skills,omitempty"`
	Topics         map[string]*TelegramTopicConfig `json:"topics,omitempty"`
	Enabled        *bool                           `json:"enabled,omitempty"`
	AllowFrom      []interface{}                   `json:"allowFrom,omitempty"`
	SystemPrompt   string                          `json:"systemPrompt,omitempty"`
}

// TelegramConfig Telegram 总配置
type TelegramConfig struct {
	TelegramAccountConfig
	Accounts map[string]*TelegramAccountConfig `json:"accounts,omitempty"`
}
