package contracts

import "context"

// 频道适配器接口 — 继承自 src/channels/plugins/types.adapters.ts (313 行)
// 每个适配器对应 ChannelPlugin 的一个能力槽位

// ChannelConfigAdapter 频道配置适配器 — 账户管理
type ChannelConfigAdapter interface {
	// ListAccountIDs 列出已配置的所有账户 ID
	ListAccountIDs() []string
	// ResolveAccount 解析账户配置
	ResolveAccount(accountID string) interface{}
	// DefaultAccountID 返回默认账户 ID
	DefaultAccountID() string
	// IsEnabled 判断账户是否启用
	IsEnabled(account interface{}) bool
	// IsConfigured 判断账户是否已配置
	IsConfigured(account interface{}) bool
}

// ChannelSetupAdapter 频道初始化适配器 — CLI 向导
type ChannelSetupAdapter interface {
	// ResolveAccountID 解析/生成账户 ID
	ResolveAccountID(accountID string) string
	// ValidateInput 验证输入参数
	ValidateInput(accountID string, input map[string]interface{}) error
}

// ChannelGroupAdapter 群组适配器 — 群组行为控制
type ChannelGroupAdapter interface {
	// ResolveRequireMention 解析群组是否需要 @提及
	ResolveRequireMention(ctx ChannelGroupContext) *bool
	// ResolveGroupIntroHint 解析群组介绍提示
	ResolveGroupIntroHint(ctx ChannelGroupContext) string
	// ResolveToolPolicy 解析群组工具策略
	ResolveToolPolicy(ctx ChannelGroupContext) interface{}
}

// ChannelMentionAdapter @提及适配器 — 提及检测与清理
type ChannelMentionAdapter interface {
	// StripPatterns 返回用于清理 @提及的正则模式
	StripPatterns(agentID string) []string
}

// ChannelStreamingAdapter 流式输出适配器
type ChannelStreamingAdapter interface {
	// BlockStreamingCoalesceDefaults 返回块流式合并默认值
	BlockStreamingCoalesceDefaults() (minChars int, idleMs int)
}

// ChannelThreadingAdapter 线程适配器
type ChannelThreadingAdapter interface {
	// ResolveReplyToMode 解析回复模式
	ResolveReplyToMode(accountID, chatType string) string // "off" | "first" | "all"
	// AllowTagsWhenOff 当回复模式为 off 时是否允许标签
	AllowTagsWhenOff() bool
}

// ChannelOutboundAdapter 出站消息适配器
type ChannelOutboundAdapter interface {
	// DeliveryMode 投递模式
	DeliveryMode() string // "direct" | "gateway" | "hybrid"
	// TextChunkLimit 文本分块限制
	TextChunkLimit() int
	// SendText 发送文本消息
	SendText(ctx context.Context, to, text string, opts OutboundOptions) error
}

// OutboundOptions 出站选项
type OutboundOptions struct {
	AccountID string
	ReplyToID string
	ThreadID  string
	MediaURL  string
}

// ChannelStatusAdapter 频道状态适配器
type ChannelStatusAdapter interface {
	// BuildAccountSnapshot 构建账户快照
	BuildAccountSnapshot(account interface{}) ChannelAccountSnapshot
	// CollectStatusIssues 收集状态问题
	CollectStatusIssues(accounts []ChannelAccountSnapshot) []ChannelStatusIssue
}

// ChannelGatewayAdapter 网关适配器 — 长连接管理
type ChannelGatewayAdapter interface {
	// StartAccount 启动频道账户连接
	StartAccount(ctx context.Context, accountID string) error
	// StopAccount 停止频道账户连接
	StopAccount(ctx context.Context, accountID string) error
}

// ChannelPairingAdapter 配对适配器 — 白名单管理
type ChannelPairingAdapter interface {
	// IDLabel 配对 ID 的标签（如 "Telegram ID", "Phone"）
	IDLabel() string
	// NormalizeAllowEntry 规范化白名单条目
	NormalizeAllowEntry(entry string) string
}

// ChannelSecurityAdapter 安全适配器
type ChannelSecurityAdapter interface {
	// ResolveDmPolicy 解析 DM 策略
	ResolveDmPolicy(accountID string) *ChannelSecurityDmPolicy
}

// ChannelDirectoryAdapter 目录适配器 — 联系人/群组查询
type ChannelDirectoryAdapter interface {
	// ListPeers 列出联系人
	ListPeers(ctx context.Context, query string, limit int) ([]ChannelDirectoryEntry, error)
	// ListGroups 列出群组
	ListGroups(ctx context.Context, query string, limit int) ([]ChannelDirectoryEntry, error)
}

// ChannelElevatedAdapter 提权适配器
type ChannelElevatedAdapter interface {
	// AllowFromFallback 未配置 allowFrom 时的回退值
	AllowFromFallback(accountID string) []interface{}
}

// ChannelCommandAdapter 命令适配器
type ChannelCommandAdapter struct {
	EnforceOwnerForCommands bool `json:"enforceOwnerForCommands,omitempty"`
	SkipWhenConfigEmpty     bool `json:"skipWhenConfigEmpty,omitempty"`
}

// ChannelAgentPromptAdapter Agent 提示适配器
type ChannelAgentPromptAdapter interface {
	// MessageToolHints 返回消息工具提示文本
	MessageToolHints(accountID string) []string
}
