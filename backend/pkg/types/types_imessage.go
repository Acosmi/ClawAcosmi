package types

// iMessage 频道配置类型 — 继承自 src/config/types.imessage.ts (82 行)

// IMessageGroupConfig iMessage 群组配置
type IMessageGroupConfig struct {
	RequireMention *bool                         `json:"requireMention,omitempty"`
	Tools          *GroupToolPolicyConfig        `json:"tools,omitempty"`
	ToolsBySender  GroupToolPolicyBySenderConfig `json:"toolsBySender,omitempty"`
}

// IMessageAccountConfig iMessage 账号配置
type IMessageAccountConfig struct {
	Name                   string                            `json:"name,omitempty"`
	Capabilities           []string                          `json:"capabilities,omitempty"`
	Markdown               *MarkdownConfig                   `json:"markdown,omitempty"`
	ConfigWrites           *bool                             `json:"configWrites,omitempty"`
	Enabled                *bool                             `json:"enabled,omitempty"`
	CliPath                string                            `json:"cliPath,omitempty"`
	DbPath                 string                            `json:"dbPath,omitempty"`
	RemoteHost             string                            `json:"remoteHost,omitempty"`
	Service                string                            `json:"service,omitempty"` // "imessage"|"sms"|"auto"
	Region                 string                            `json:"region,omitempty"`
	DmPolicy               DmPolicy                          `json:"dmPolicy,omitempty"`
	AllowFrom              []interface{}                     `json:"allowFrom,omitempty"`
	GroupAllowFrom         []interface{}                     `json:"groupAllowFrom,omitempty"`
	GroupPolicy            GroupPolicy                       `json:"groupPolicy,omitempty"`
	HistoryLimit           *int                              `json:"historyLimit,omitempty"`
	DmHistoryLimit         *int                              `json:"dmHistoryLimit,omitempty"`
	Dms                    map[string]*DmConfig              `json:"dms,omitempty"`
	IncludeAttachments     *bool                             `json:"includeAttachments,omitempty"`
	MediaMaxMB             *int                              `json:"mediaMaxMb,omitempty"`
	ProbeTimeoutMs         *int                              `json:"probeTimeoutMs,omitempty"`
	TextChunkLimit         *int                              `json:"textChunkLimit,omitempty"`
	ChunkMode              string                            `json:"chunkMode,omitempty"`
	BlockStreaming         *bool                             `json:"blockStreaming,omitempty"`
	BlockStreamingCoalesce *BlockStreamingCoalesceConfig     `json:"blockStreamingCoalesce,omitempty"`
	Groups                 map[string]*IMessageGroupConfig   `json:"groups,omitempty"`
	Heartbeat              *ChannelHeartbeatVisibilityConfig `json:"heartbeat,omitempty"`
	ResponsePrefix         string                            `json:"responsePrefix,omitempty"`
}

// IMessageConfig iMessage 总配置
type IMessageConfig struct {
	IMessageAccountConfig
	Accounts map[string]*IMessageAccountConfig `json:"accounts,omitempty"`
}
