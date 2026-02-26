package contracts

import "context"

// ---------- 频道发送契约 (agents → channels) ----------

// ChannelSender — agents 模块需要的频道发送能力。
// 由 channels 模块的具体频道实现，通过 DI 注入到 agents。
type ChannelSender interface {
	// SendMessage 发送消息到指定频道。
	SendMessage(ctx context.Context, req *OutboundMessageRequest) error
	// SendReaction 发送表情回应。
	SendReaction(ctx context.Context, channel, messageID, emoji string) error
}

// OutboundMessageRequest 出站消息请求。
type OutboundMessageRequest struct {
	Channel   string   `json:"channel"`
	AccountID string   `json:"accountId,omitempty"`
	To        string   `json:"to"`
	Content   string   `json:"content"`
	MediaURLs []string `json:"mediaUrls,omitempty"`
	ThreadID  string   `json:"threadId,omitempty"`
	ReplyToID string   `json:"replyToId,omitempty"`
}

// ---------- 频道注册表契约 (agents → channels) ----------

// ChannelRegistry — agents 查询可用频道。
type ChannelRegistry interface {
	// GetChannel 获取指定类型的频道发送器。
	GetChannel(channelType string) (ChannelSender, bool)
	// ListActiveChannels 列出所有活跃频道。
	ListActiveChannels() []string
}

// ---------- Agent 触发契约 (channels → agents) ----------

// AgentTrigger — channels 模块触发 Agent 执行。
// 由 agents 引擎实现，通过 DI 注入到 channels。
type AgentTrigger interface {
	// TriggerAgent 触发 Agent 执行。
	TriggerAgent(ctx context.Context, req *AgentTriggerRequest) (*AgentTriggerResult, error)
}

// AgentTriggerRequest Agent 触发请求。
type AgentTriggerRequest struct {
	AgentID    string `json:"agentId"`
	SessionKey string `json:"sessionKey"`
	Prompt     string `json:"prompt"`
	Channel    string `json:"channel"`
	SenderID   string `json:"senderId,omitempty"`
}

// AgentTriggerResult Agent 触发结果。
type AgentTriggerResult struct {
	Payloads []AgentPayload `json:"payloads"`
	Meta     AgentMeta      `json:"meta"`
}

// AgentPayload Agent 输出载荷。
type AgentPayload struct {
	Text    string   `json:"text"`
	IsError bool     `json:"isError,omitempty"`
	Images  []string `json:"images,omitempty"`
}

// AgentMeta Agent 执行元数据。
type AgentMeta struct {
	DurationMs int64  `json:"durationMs"`
	Error      string `json:"error,omitempty"`
}

// ---------- 插件注册表契约 (跨模块) ----------

// PluginRegistry — 运行时插件注册表查询。
// 见审计报告 §4.13: message-channel.ts 依赖运行时已加载的插件注册表。
type PluginRegistry interface {
	// IsPluginActive 检查插件是否已激活。
	IsPluginActive(pluginID string) bool
	// GetPluginChannelIDs 获取插件注册的频道 ID 列表。
	GetPluginChannelIDs() []string
}
