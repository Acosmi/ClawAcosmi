package discord

import (
	"context"

	"github.com/openacosmi/claw-acismi/internal/autoreply"
)

// Discord 入站管线 DI 依赖 — 定义外部模块注入点
// 结构沿用 iMessage (A2) / Signal (A3) / WhatsApp (A4) / Slack (A5) 已验证的 MonitorDeps 模式。

// DiscordAgentRouteParams Agent 路由参数
type DiscordAgentRouteParams struct {
	Channel   string // "discord"
	AccountID string
	PeerKind  string // "direct" | "group" | "channel"
	PeerID    string
	GuildID   string // guild ID for guild-scoped bindings
	TeamID    string // team ID for team-scoped bindings
}

// DiscordAgentRoute Agent 路由结果
type DiscordAgentRoute struct {
	AgentID        string
	AccountID      string
	SessionKey     string
	MainSessionKey string
}

// DiscordDispatchParams 入站消息分发参数
type DiscordDispatchParams struct {
	Ctx        *autoreply.MsgContext
	Dispatcher interface{} // *reply.ReplyDispatcher — 由 DI 层构建
	// ReplyOptions
	DisableBlockStreaming *bool
	OnModelSelected       func(model string)
}

// DiscordDispatchResult 入站消息分发结果
type DiscordDispatchResult struct {
	QueuedFinal bool
}

// DiscordRecordSessionParams 入站会话记录参数
type DiscordRecordSessionParams struct {
	StorePath  string
	SessionKey string
	Ctx        *autoreply.MsgContext
	// UpdateLastRoute 仅对 DM 消息设置
	UpdateLastRoute *DiscordLastRouteUpdate
}

// DiscordLastRouteUpdate 最近路由更新
type DiscordLastRouteUpdate struct {
	SessionKey string
	Channel    string
	To         string
	AccountID  string
}

// DiscordPairingRequestParams 配对请求参数
type DiscordPairingRequestParams struct {
	Channel string
	ID      string
	Meta    map[string]string
}

// DiscordPairingResult 配对请求结果
type DiscordPairingResult struct {
	Code    string
	Created bool
}

// DiscordMonitorDeps 入站管线依赖接口
// 各函数均可为 nil，nil 时对应功能将被跳过或使用默认行为。
type DiscordMonitorDeps struct {
	// ResolveAgentRoute 解析 Agent 路由（频道 + peer → session key）
	ResolveAgentRoute func(params DiscordAgentRouteParams) (*DiscordAgentRoute, error)

	// DispatchInboundMessage 分发入站消息到 auto-reply 管线
	DispatchInboundMessage func(ctx context.Context, params DiscordDispatchParams) (*DiscordDispatchResult, error)

	// RecordInboundSession 记录入站会话元数据
	RecordInboundSession func(params DiscordRecordSessionParams) error

	// UpsertPairingRequest 创建或更新配对请求
	UpsertPairingRequest func(params DiscordPairingRequestParams) (*DiscordPairingResult, error)

	// ReadAllowFromStore 从 pairing store 读取动态允许列表
	ReadAllowFromStore func(channel string) ([]string, error)

	// ResolveStorePath 解析会话存储路径
	ResolveStorePath func(agentID string) string

	// ReadSessionUpdatedAt 读取会话最后更新时间戳
	ReadSessionUpdatedAt func(storePath, sessionKey string) *int64

	// ResolveMedia 解析媒体附件（下载+保存）
	// 返回: (localPath, contentType, error)
	ResolveMedia func(mediaURL string, maxBytes int) (string, string, error)

	// EnqueueSystemEvent 入队系统事件（反应通知等）
	EnqueueSystemEvent func(text, sessionKey, contextKey string) error

	// RecordChannelActivity 记录频道活动（出站/入站）
	RecordChannelActivity func(channel, accountID, direction string)

	// ResetSession 重置会话（/reset 命令）
	ResetSession func(ctx context.Context, accountID, channelID, senderID string) error

	// SwitchModel 切换模型（/model 命令）
	SwitchModel func(ctx context.Context, accountID, modelName string) error

	// DY-028 fix: Ack reaction 回调 — 对齐 TS message-handler.process.ts ack reaction 步骤。
	// AddReaction 添加 ack reaction（确认收到消息的表情反应）
	AddReaction func(ctx context.Context, channelID, messageID string) error

	// RemoveReaction 移除 ack reaction（回复完成后移除确认表情）
	RemoveReaction func(ctx context.Context, channelID, messageID string) error
}
