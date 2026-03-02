package telegram

import (
	"context"

	"github.com/openacosmi/claw-acismi/internal/autoreply"
)

// Telegram 入站管线 DI 依赖 — 定义外部模块注入点。
// 对齐 iMessage/Signal/Slack/Discord 的 MonitorDeps 模式。
// 各函数可为 nil，nil 时对应功能跳过或使用降级行为。

// TelegramAgentRouteParams Agent 路由参数
type TelegramAgentRouteParams struct {
	Channel   string // "telegram"
	AccountID string
	PeerKind  string // "group" | "direct"
	PeerID    string // chatID 字符串
	ThreadID  string // forum topic thread ID
}

// TelegramAgentRoute Agent 路由结果
type TelegramAgentRoute struct {
	AgentID        string
	AccountID      string
	SessionKey     string
	MainSessionKey string
}

// TelegramDispatchParams 入站消息分发参数
type TelegramDispatchParams struct {
	Ctx                   *autoreply.MsgContext
	OnModelSelected       func(model string)
	DisableBlockStreaming *bool
}

// TelegramDispatchResult 入站消息分发结果
type TelegramDispatchResult struct {
	Replies     []autoreply.ReplyPayload
	QueuedFinal bool
}

// TelegramRecordSessionParams 入站会话记录参数
type TelegramRecordSessionParams struct {
	StorePath  string
	SessionKey string
	Ctx        *autoreply.MsgContext
	// UpdateLastRoute 仅对 DM 消息设置
	UpdateLastRoute *TelegramLastRouteUpdate
}

// TelegramLastRouteUpdate 最近路由更新
type TelegramLastRouteUpdate struct {
	SessionKey string
	Channel    string
	To         string
	AccountID  string
}

// TelegramPairingParams 配对请求参数
type TelegramPairingParams struct {
	Channel string
	ID      string
	Meta    map[string]string
}

// TelegramPairingResult 配对请求结果
type TelegramPairingResult struct {
	Code    string
	Created bool
}

// TelegramMonitorDeps 入站管线依赖接口。
// 各函数均可为 nil，nil 时对应功能将被跳过或使用默认行为。
type TelegramMonitorDeps struct {
	// ResolveAgentRoute 解析 Agent 路由（频道 + peer → session key）
	// TS 对照: routing/resolve-route.ts resolveAgentRoute()
	ResolveAgentRoute func(params TelegramAgentRouteParams) (*TelegramAgentRoute, error)

	// DispatchInboundMessage 分发入站消息到 auto-reply 管线
	// TS 对照: auto-reply/dispatch.ts dispatchInboundMessage()
	DispatchInboundMessage func(ctx context.Context, params TelegramDispatchParams) (*TelegramDispatchResult, error)

	// RecordInboundSession 记录入站会话元数据
	// TS 对照: channels/session.ts recordInboundSession()
	RecordInboundSession func(params TelegramRecordSessionParams) error

	// UpsertPairingRequest 创建或更新配对请求
	// TS 对照: pairing/pairing-store.ts upsertChannelPairingRequest()
	UpsertPairingRequest func(params TelegramPairingParams) (*TelegramPairingResult, error)

	// ReadAllowFromStore 从 pairing store 读取动态允许列表
	// TS 对照: pairing/pairing-store.ts readChannelAllowFromStore()
	ReadAllowFromStore func(channel string) ([]string, error)

	// ResolveStorePath 解析会话存储路径
	// TS 对照: config/sessions.ts resolveStorePath()
	ResolveStorePath func(agentID string) string

	// ResetSession 重置会话（/reset 命令）
	// TS 对照: auto-reply/commands-handler-session.ts handleResetCommand()
	ResetSession func(ctx context.Context, sessionKey, storePath string) error

	// SwitchModel 切换模型（/model 命令回调）
	// TS 对照: auto-reply/commands-handler-models.ts handleModelCommand()
	SwitchModel func(ctx context.Context, sessionKey, storePath, modelRef string) error

	// DescribeImage 描述图片内容（贴纸视觉理解）
	// TS 对照: media-understanding/understand.ts describeImage()
	DescribeImage func(ctx context.Context, imageData []byte, contentType string) (string, error)

	// EnqueueSystemEvent 入队系统事件（反应、成员变更等）
	// TS 对照: infra/agent-events.ts emitAgentEvent()
	EnqueueSystemEvent func(eventType string, data map[string]interface{})

	// DY-012: LoadSessionEntry 读取会话存储条目
	// TS 对照: store.get(sessionKey) — 返回 session entry 中的 key-value 对。
	// 用于 resolveGroupActivation 读取 groupActivation 设置。
	LoadSessionEntry func(sessionKey string) (map[string]string, error)
}
