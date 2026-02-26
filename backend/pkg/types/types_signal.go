package types

// Signal 频道配置类型 — 继承自 src/config/types.signal.ts (95 行)

// SignalReactionNotificationMode Signal 反应通知模式
type SignalReactionNotificationMode string

const (
	SignalReactionOff       SignalReactionNotificationMode = "off"
	SignalReactionOwn       SignalReactionNotificationMode = "own"
	SignalReactionAll       SignalReactionNotificationMode = "all"
	SignalReactionAllowlist SignalReactionNotificationMode = "allowlist"
)

// SignalReactionLevel Signal 反应级别
type SignalReactionLevel string

const (
	SignalReactionLevelOff       SignalReactionLevel = "off"
	SignalReactionLevelAck       SignalReactionLevel = "ack"
	SignalReactionLevelMinimal   SignalReactionLevel = "minimal"
	SignalReactionLevelExtensive SignalReactionLevel = "extensive"
)

// SignalActionConfig Signal 操作配置
type SignalActionConfig struct {
	Reactions *bool `json:"reactions,omitempty"`
}

// SignalAccountConfig Signal 账号配置
type SignalAccountConfig struct {
	Name                   string                            `json:"name,omitempty"`
	Capabilities           []string                          `json:"capabilities,omitempty"`
	Markdown               *MarkdownConfig                   `json:"markdown,omitempty"`
	ConfigWrites           *bool                             `json:"configWrites,omitempty"`
	Enabled                *bool                             `json:"enabled,omitempty"`
	Account                string                            `json:"account,omitempty"`
	HttpURL                string                            `json:"httpUrl,omitempty"`
	HttpHost               string                            `json:"httpHost,omitempty"`
	HttpPort               *int                              `json:"httpPort,omitempty"`
	CliPath                string                            `json:"cliPath,omitempty"`
	AutoStart              *bool                             `json:"autoStart,omitempty"`
	StartupTimeoutMs       *int                              `json:"startupTimeoutMs,omitempty"`
	ReceiveMode            string                            `json:"receiveMode,omitempty"` // "on-start"|"manual"
	IgnoreAttachments      *bool                             `json:"ignoreAttachments,omitempty"`
	IgnoreStories          *bool                             `json:"ignoreStories,omitempty"`
	SendReadReceipts       *bool                             `json:"sendReadReceipts,omitempty"`
	DmPolicy               DmPolicy                          `json:"dmPolicy,omitempty"`
	AllowFrom              []interface{}                     `json:"allowFrom,omitempty"`
	GroupAllowFrom         []interface{}                     `json:"groupAllowFrom,omitempty"`
	GroupPolicy            GroupPolicy                       `json:"groupPolicy,omitempty"`
	HistoryLimit           *int                              `json:"historyLimit,omitempty"`
	DmHistoryLimit         *int                              `json:"dmHistoryLimit,omitempty"`
	Dms                    map[string]*DmConfig              `json:"dms,omitempty"`
	TextChunkLimit         *int                              `json:"textChunkLimit,omitempty"`
	ChunkMode              string                            `json:"chunkMode,omitempty"`
	BlockStreaming         *bool                             `json:"blockStreaming,omitempty"`
	BlockStreamingCoalesce *BlockStreamingCoalesceConfig     `json:"blockStreamingCoalesce,omitempty"`
	MediaMaxMB             *int                              `json:"mediaMaxMb,omitempty"`
	ReactionNotifications  SignalReactionNotificationMode    `json:"reactionNotifications,omitempty"`
	ReactionAllowlist      []interface{}                     `json:"reactionAllowlist,omitempty"`
	Actions                *SignalActionConfig               `json:"actions,omitempty"`
	ReactionLevel          SignalReactionLevel               `json:"reactionLevel,omitempty"`
	Heartbeat              *ChannelHeartbeatVisibilityConfig `json:"heartbeat,omitempty"`
	ResponsePrefix         string                            `json:"responsePrefix,omitempty"`
}

// SignalConfig Signal 总配置
type SignalConfig struct {
	SignalAccountConfig
	Accounts map[string]*SignalAccountConfig `json:"accounts,omitempty"`
}
