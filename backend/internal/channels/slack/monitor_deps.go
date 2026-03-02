package slack

import (
	"context"

	"github.com/openacosmi/claw-acismi/internal/autoreply"
)

// Slack 入站管线 DI 依赖 — 定义外部模块注入点
// 结构沿用 iMessage (A2) / Signal (A3) / WhatsApp (A4) 已验证的 MonitorDeps 模式。
// 缺失的跨频道共享模块（routing, dispatch, session, pairing-store）
// 使用 DI 接口注入，避免硬依赖未实现的模块。

// SlackAgentRouteParams Agent 路由参数
type SlackAgentRouteParams struct {
	Channel   string // "slack"
	AccountID string
	PeerKind  string // "direct" | "group" | "channel"
	PeerID    string
}

// SlackAgentRoute Agent 路由结果
type SlackAgentRoute struct {
	AgentID        string
	AccountID      string
	SessionKey     string
	MainSessionKey string
}

// SlackDispatchParams 入站消息分发参数
type SlackDispatchParams struct {
	Ctx        *autoreply.MsgContext
	Dispatcher interface{} // *reply.ReplyDispatcher — 由 DI 层构建
	// ReplyOptions
	DisableBlockStreaming *bool
	OnModelSelected       func(model string)
}

// SlackDispatchResult 入站消息分发结果
type SlackDispatchResult struct {
	QueuedFinal bool
}

// SlackRecordSessionParams 入站会话记录参数
type SlackRecordSessionParams struct {
	StorePath  string
	SessionKey string
	Ctx        *autoreply.MsgContext
	// UpdateLastRoute 仅对 DM 消息设置
	UpdateLastRoute *SlackLastRouteUpdate
}

// SlackLastRouteUpdate 最近路由更新
type SlackLastRouteUpdate struct {
	SessionKey string
	Channel    string
	To         string
	AccountID  string
}

// SlackPairingRequestParams 配对请求参数
type SlackPairingRequestParams struct {
	Channel string
	ID      string
	Meta    map[string]string
}

// SlackPairingResult 配对请求结果
type SlackPairingResult struct {
	Code    string
	Created bool
}

// SlackMonitorDeps 入站管线依赖接口
// 各函数均可为 nil，nil 时对应功能将被跳过或使用默认行为。
type SlackMonitorDeps struct {
	// ResolveAgentRoute 解析 Agent 路由（频道 + peer → session key）
	// TS 对照: routing/resolve-route.ts resolveAgentRoute()
	ResolveAgentRoute func(params SlackAgentRouteParams) (*SlackAgentRoute, error)

	// DispatchInboundMessage 分发入站消息到 auto-reply 管线
	// TS 对照: auto-reply/dispatch.ts dispatchInboundMessage()
	DispatchInboundMessage func(ctx context.Context, params SlackDispatchParams) (*SlackDispatchResult, error)

	// RecordInboundSession 记录入站会话元数据
	// TS 对照: channels/session.ts recordInboundSession()
	RecordInboundSession func(params SlackRecordSessionParams) error

	// UpsertPairingRequest 创建或更新配对请求
	// TS 对照: pairing/pairing-store.ts upsertChannelPairingRequest()
	UpsertPairingRequest func(params SlackPairingRequestParams) (*SlackPairingResult, error)

	// ReadAllowFromStore 从 pairing store 读取动态允许列表
	// TS 对照: pairing/pairing-store.ts readChannelAllowFromStore()
	ReadAllowFromStore func(channel string) ([]string, error)

	// ResolveStorePath 解析会话存储路径
	// TS 对照: config/sessions.ts resolveStorePath()
	ResolveStorePath func(agentID string) string

	// ReadSessionUpdatedAt 读取会话最后更新时间戳
	// TS 对照: config/sessions.ts readSessionUpdatedAt()
	ReadSessionUpdatedAt func(storePath, sessionKey string) *int64

	// ResolveMedia 解析媒体附件（下载+保存）
	// TS 对照: send.ts resolveAttachment → loadWebMedia + saveMediaBuffer
	// 返回: (localPath, contentType, error)
	ResolveMedia func(mediaURL string, maxBytes int) (string, string, error)

	// EnqueueSystemEvent 入队系统事件（反应通知等）
	// TS 对照: auto-reply/queue.ts enqueueSystemEvent()
	EnqueueSystemEvent func(text, sessionKey, contextKey string) error
}
