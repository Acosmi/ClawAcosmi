package signal

import (
	"context"

	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
)

// 入站管线 DI 依赖 — 定义外部模块注入点
// 缺失的跨频道共享模块（routing, dispatch, session, pairing-store）
// 使用 DI 接口注入，避免硬依赖未实现的模块。
// 结构沿用 iMessage A2 已验证的 MonitorDeps 模式。

// AgentRouteParams Agent 路由参数
type AgentRouteParams struct {
	Channel   string
	AccountID string
	PeerKind  string // "group" | "direct"
	PeerID    string
}

// AgentRoute Agent 路由结果
type AgentRoute struct {
	AgentID        string
	AccountID      string
	SessionKey     string
	MainSessionKey string
}

// DispatchParams 入站消息分发参数
type DispatchParams struct {
	Ctx        *autoreply.MsgContext
	Dispatcher interface{} // *reply.ReplyDispatcher — 由 DI 层构建
	// ReplyOptions
	DisableBlockStreaming *bool
	OnModelSelected       func(model string)
}

// DispatchResult 入站消息分发结果
type DispatchResult struct {
	QueuedFinal bool
}

// RecordSessionParams 入站会话记录参数
type RecordSessionParams struct {
	StorePath  string
	SessionKey string
	Ctx        *autoreply.MsgContext
	// UpdateLastRoute 仅对 DM 消息设置
	UpdateLastRoute *LastRouteUpdate
}

// LastRouteUpdate 最近路由更新
type LastRouteUpdate struct {
	SessionKey string
	Channel    string
	To         string
	AccountID  string
}

// PairingRequestParams 配对请求参数
type PairingRequestParams struct {
	Channel string
	ID      string
	Meta    map[string]string
}

// PairingResult 配对请求结果
type PairingResult struct {
	Code    string
	Created bool
}

// SignalMonitorDeps 入站管线依赖接口
// 各函数均可为 nil，nil 时对应功能将被跳过或使用默认行为。
type SignalMonitorDeps struct {
	// ResolveAgentRoute 解析 Agent 路由（频道 + peer → session key）
	// TS 对照: routing/resolve-route.ts resolveAgentRoute()
	ResolveAgentRoute func(params AgentRouteParams) (*AgentRoute, error)

	// DispatchInboundMessage 分发入站消息到 auto-reply 管线
	// TS 对照: auto-reply/dispatch.ts dispatchInboundMessage()
	DispatchInboundMessage func(ctx context.Context, params DispatchParams) (*DispatchResult, error)

	// RecordInboundSession 记录入站会话元数据
	// TS 对照: channels/session.ts recordInboundSession()
	RecordInboundSession func(params RecordSessionParams) error

	// UpsertPairingRequest 创建或更新配对请求
	// TS 对照: pairing/pairing-store.ts upsertChannelPairingRequest()
	UpsertPairingRequest func(params PairingRequestParams) (*PairingResult, error)

	// ReadAllowFromStore 从 pairing store 读取动态允许列表
	// TS 对照: pairing/pairing-store.ts readChannelAllowFromStore()
	ReadAllowFromStore func(channel string) ([]string, error)

	// ResolveStorePath 解析会话存储路径
	// TS 对照: config/sessions.ts resolveStorePath()
	ResolveStorePath func(agentID string) string

	// EnqueueSystemEvent 入队系统事件（反应通知等）
	// TS 对照: auto-reply/queue.ts enqueueSystemEvent()
	EnqueueSystemEvent func(text, sessionKey, contextKey string) error
}
