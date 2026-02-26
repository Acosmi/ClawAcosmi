package contracts

// 频道核心类型 — 继承自 src/channels/plugins/types.core.ts (332 行)

// ChannelID 频道标识（核心频道为预定义值，插件频道为任意字符串）
type ChannelID = string

// ChannelOutboundTargetMode 出站目标模式
type ChannelOutboundTargetMode string

const (
	OutboundModeExplicit  ChannelOutboundTargetMode = "explicit"
	OutboundModeImplicit  ChannelOutboundTargetMode = "implicit"
	OutboundModeHeartbeat ChannelOutboundTargetMode = "heartbeat"
)

// ChannelMeta 频道元数据 — UI 展示和文档索引
type ChannelMeta struct {
	ID                                   ChannelID `json:"id"`
	Label                                string    `json:"label"`
	SelectionLabel                       string    `json:"selectionLabel"`
	DocsPath                             string    `json:"docsPath"`
	DocsLabel                            string    `json:"docsLabel,omitempty"`
	Blurb                                string    `json:"blurb"`
	Order                                *int      `json:"order,omitempty"`
	Aliases                              []string  `json:"aliases,omitempty"`
	SelectionDocsPrefix                  string    `json:"selectionDocsPrefix,omitempty"`
	SelectionDocsOmitLabel               bool      `json:"selectionDocsOmitLabel,omitempty"`
	SelectionExtras                      []string  `json:"selectionExtras,omitempty"`
	DetailLabel                          string    `json:"detailLabel,omitempty"`
	SystemImage                          string    `json:"systemImage,omitempty"`
	ShowConfigured                       bool      `json:"showConfigured,omitempty"`
	QuickstartAllowFrom                  bool      `json:"quickstartAllowFrom,omitempty"`
	ForceAccountBinding                  bool      `json:"forceAccountBinding,omitempty"`
	PreferSessionLookupForAnnounceTarget bool      `json:"preferSessionLookupForAnnounceTarget,omitempty"`
	PreferOver                           []string  `json:"preferOver,omitempty"`
}

// ChannelCapabilities 频道能力描述
type ChannelCapabilities struct {
	ChatTypes       []string `json:"chatTypes"` // "direct", "group", "channel", "thread"
	Polls           bool     `json:"polls,omitempty"`
	Reactions       bool     `json:"reactions,omitempty"`
	Edit            bool     `json:"edit,omitempty"`
	Unsend          bool     `json:"unsend,omitempty"`
	Reply           bool     `json:"reply,omitempty"`
	Effects         bool     `json:"effects,omitempty"`
	GroupManagement bool     `json:"groupManagement,omitempty"`
	Threads         bool     `json:"threads,omitempty"`
	Media           bool     `json:"media,omitempty"`
	NativeCommands  bool     `json:"nativeCommands,omitempty"`
	BlockStreaming  bool     `json:"blockStreaming,omitempty"`
}

// ChannelAccountState 频道账户状态
type ChannelAccountState string

const (
	AccountStateLinked        ChannelAccountState = "linked"
	AccountStateNotLinked     ChannelAccountState = "not linked"
	AccountStateConfigured    ChannelAccountState = "configured"
	AccountStateNotConfigured ChannelAccountState = "not configured"
	AccountStateEnabled       ChannelAccountState = "enabled"
	AccountStateDisabled      ChannelAccountState = "disabled"
)

// ChannelAccountSnapshot 频道账户快照 — 运行时状态
type ChannelAccountSnapshot struct {
	AccountID         string      `json:"accountId"`
	Name              string      `json:"name,omitempty"`
	Enabled           *bool       `json:"enabled,omitempty"`
	Configured        *bool       `json:"configured,omitempty"`
	Linked            *bool       `json:"linked,omitempty"`
	Running           *bool       `json:"running,omitempty"`
	Connected         *bool       `json:"connected,omitempty"`
	ReconnectAttempts *int        `json:"reconnectAttempts,omitempty"`
	LastConnectedAt   *int64      `json:"lastConnectedAt,omitempty"`
	LastMessageAt     *int64      `json:"lastMessageAt,omitempty"`
	LastEventAt       *int64      `json:"lastEventAt,omitempty"`
	LastError         string      `json:"lastError,omitempty"`
	LastStartAt       *int64      `json:"lastStartAt,omitempty"`
	LastStopAt        *int64      `json:"lastStopAt,omitempty"`
	LastInboundAt     *int64      `json:"lastInboundAt,omitempty"`
	LastOutboundAt    *int64      `json:"lastOutboundAt,omitempty"`
	Mode              string      `json:"mode,omitempty"`
	DmPolicy          string      `json:"dmPolicy,omitempty"`
	AllowFrom         []string    `json:"allowFrom,omitempty"`
	TokenSource       string      `json:"tokenSource,omitempty"`
	BotTokenSource    string      `json:"botTokenSource,omitempty"`
	AppTokenSource    string      `json:"appTokenSource,omitempty"`
	CredentialSource  string      `json:"credentialSource,omitempty"`
	AudienceType      string      `json:"audienceType,omitempty"`
	Audience          string      `json:"audience,omitempty"`
	WebhookPath       string      `json:"webhookPath,omitempty"`
	WebhookURL        string      `json:"webhookUrl,omitempty"`
	BaseURL           string      `json:"baseUrl,omitempty"`
	CliPath           string      `json:"cliPath,omitempty"`
	DbPath            string      `json:"dbPath,omitempty"`
	Port              *int        `json:"port,omitempty"`
	Probe             interface{} `json:"probe,omitempty"`
	LastProbeAt       *int64      `json:"lastProbeAt,omitempty"`
	Audit             interface{} `json:"audit,omitempty"`
	Application       interface{} `json:"application,omitempty"`
	Bot               interface{} `json:"bot,omitempty"`
}

// ChannelGroupContext 群组上下文
type ChannelGroupContext struct {
	GroupID        string `json:"groupId,omitempty"`
	GroupChannel   string `json:"groupChannel,omitempty"`
	GroupSpace     string `json:"groupSpace,omitempty"`
	AccountID      string `json:"accountId,omitempty"`
	SenderID       string `json:"senderId,omitempty"`
	SenderName     string `json:"senderName,omitempty"`
	SenderUsername string `json:"senderUsername,omitempty"`
	SenderE164     string `json:"senderE164,omitempty"`
}

// ChannelSecurityDmPolicy DM 安全策略
type ChannelSecurityDmPolicy struct {
	Policy        string        `json:"policy"`
	AllowFrom     []interface{} `json:"allowFrom,omitempty"` // string | number
	PolicyPath    string        `json:"policyPath,omitempty"`
	AllowFromPath string        `json:"allowFromPath"`
	ApproveHint   string        `json:"approveHint"`
}

// ChannelStatusIssue 频道状态问题
type ChannelStatusIssue struct {
	Channel   ChannelID `json:"channel"`
	AccountID string    `json:"accountId"`
	Kind      string    `json:"kind"` // "intent" | "permissions" | "config" | "auth" | "runtime"
	Message   string    `json:"message"`
	Fix       string    `json:"fix,omitempty"`
}

// ChannelDirectoryEntryKind 目录条目类型
type ChannelDirectoryEntryKind string

const (
	DirectoryEntryUser    ChannelDirectoryEntryKind = "user"
	DirectoryEntryGroup   ChannelDirectoryEntryKind = "group"
	DirectoryEntryChannel ChannelDirectoryEntryKind = "channel"
)

// ChannelDirectoryEntry 目录条目
type ChannelDirectoryEntry struct {
	Kind      ChannelDirectoryEntryKind `json:"kind"`
	ID        string                    `json:"id"`
	Name      string                    `json:"name,omitempty"`
	Handle    string                    `json:"handle,omitempty"`
	AvatarURL string                    `json:"avatarUrl,omitempty"`
	Rank      *int                      `json:"rank,omitempty"`
	Raw       interface{}               `json:"raw,omitempty"`
}

// ChannelThreadingContext 线程上下文
type ChannelThreadingContext struct {
	Channel         string      `json:"Channel,omitempty"`
	From            string      `json:"From,omitempty"`
	To              string      `json:"To,omitempty"`
	ChatType        string      `json:"ChatType,omitempty"`
	ReplyToID       string      `json:"ReplyToId,omitempty"`
	ReplyToIDFull   string      `json:"ReplyToIdFull,omitempty"`
	ThreadLabel     string      `json:"ThreadLabel,omitempty"`
	MessageThreadID interface{} `json:"MessageThreadId,omitempty"` // string | number
}

// ChannelThreadingToolContext 线程工具上下文
type ChannelThreadingToolContext struct {
	CurrentChannelID           string `json:"currentChannelId,omitempty"`
	CurrentChannelProvider     string `json:"currentChannelProvider,omitempty"`
	CurrentThreadTs            string `json:"currentThreadTs,omitempty"`
	ReplyToMode                string `json:"replyToMode,omitempty"` // "off" | "first" | "all"
	SkipCrossContextDecoration bool   `json:"skipCrossContextDecoration,omitempty"`
}
