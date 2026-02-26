package types

// MS Teams 频道配置类型 — 继承自 src/config/types.msteams.ts (112 行)

// MSTeamsWebhookConfig MS Teams Webhook 配置
type MSTeamsWebhookConfig struct {
	Port *int   `json:"port,omitempty"` // 默认 3978
	Path string `json:"path,omitempty"` // 默认 /api/messages
}

// MSTeamsReplyStyle MS Teams 回复风格
type MSTeamsReplyStyle string

const (
	MSTeamsReplyThread   MSTeamsReplyStyle = "thread"
	MSTeamsReplyTopLevel MSTeamsReplyStyle = "top-level"
)

// MSTeamsChannelConfig MS Teams 频道配置
type MSTeamsChannelConfig struct {
	RequireMention *bool                         `json:"requireMention,omitempty"`
	Tools          *GroupToolPolicyConfig        `json:"tools,omitempty"`
	ToolsBySender  GroupToolPolicyBySenderConfig `json:"toolsBySender,omitempty"`
	ReplyStyle     MSTeamsReplyStyle             `json:"replyStyle,omitempty"`
}

// MSTeamsTeamConfig MS Teams 团队配置
type MSTeamsTeamConfig struct {
	RequireMention *bool                            `json:"requireMention,omitempty"`
	Tools          *GroupToolPolicyConfig           `json:"tools,omitempty"`
	ToolsBySender  GroupToolPolicyBySenderConfig    `json:"toolsBySender,omitempty"`
	ReplyStyle     MSTeamsReplyStyle                `json:"replyStyle,omitempty"`
	Channels       map[string]*MSTeamsChannelConfig `json:"channels,omitempty"`
}

// MSTeamsConfig MS Teams 总配置
type MSTeamsConfig struct {
	Enabled                *bool                             `json:"enabled,omitempty"`
	Capabilities           []string                          `json:"capabilities,omitempty"`
	Markdown               *MarkdownConfig                   `json:"markdown,omitempty"`
	ConfigWrites           *bool                             `json:"configWrites,omitempty"`
	AppID                  string                            `json:"appId,omitempty"`
	AppPassword            string                            `json:"appPassword,omitempty"`
	TenantID               string                            `json:"tenantId,omitempty"`
	Webhook                *MSTeamsWebhookConfig             `json:"webhook,omitempty"`
	DmPolicy               DmPolicy                          `json:"dmPolicy,omitempty"`
	AllowFrom              []string                          `json:"allowFrom,omitempty"`
	GroupAllowFrom         []string                          `json:"groupAllowFrom,omitempty"`
	GroupPolicy            GroupPolicy                       `json:"groupPolicy,omitempty"`
	TextChunkLimit         *int                              `json:"textChunkLimit,omitempty"`
	ChunkMode              string                            `json:"chunkMode,omitempty"`
	BlockStreamingCoalesce *BlockStreamingCoalesceConfig     `json:"blockStreamingCoalesce,omitempty"`
	MediaAllowHosts        []string                          `json:"mediaAllowHosts,omitempty"`
	MediaAuthAllowHosts    []string                          `json:"mediaAuthAllowHosts,omitempty"`
	RequireMention         *bool                             `json:"requireMention,omitempty"`
	HistoryLimit           *int                              `json:"historyLimit,omitempty"`
	DmHistoryLimit         *int                              `json:"dmHistoryLimit,omitempty"`
	Dms                    map[string]*DmConfig              `json:"dms,omitempty"`
	ReplyStyle             MSTeamsReplyStyle                 `json:"replyStyle,omitempty"`
	Teams                  map[string]*MSTeamsTeamConfig     `json:"teams,omitempty"`
	MediaMaxMB             *int                              `json:"mediaMaxMb,omitempty"`
	SharePointSiteID       string                            `json:"sharePointSiteId,omitempty"`
	Heartbeat              *ChannelHeartbeatVisibilityConfig `json:"heartbeat,omitempty"`
	ResponsePrefix         string                            `json:"responsePrefix,omitempty"`
}
