package reply

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/anthropic/open-acosmi/internal/autoreply"
	"github.com/anthropic/open-acosmi/internal/session"
)

// TS 对照: auto-reply/reply/session.ts (394L)
// 会话初始化与会话分叉逻辑。

// ---------- 类型 ----------

// SessionScope 会话作用域。
type SessionScope string

const (
	ScopeConversation SessionScope = "conversation"
	ScopeThread       SessionScope = "thread"
	ScopeGlobal       SessionScope = "global"
)

// SessionInitResult 会话初始化结果。
// TS 对照: session.ts SessionInitResult
type SessionInitResult struct {
	Entry       *SessionEntry
	IsNew       bool
	SessionKey  string
	SessionID   string
	StorePath   string
	Scope       SessionScope
	ParentEntry *SessionEntry
	ForkedFrom  string
}

// SessionStoreAccessor 会话存储访问器（DI 接口）。
// 避免 reply 包直接引用 gateway 包。
type SessionStoreAccessor interface {
	LoadSessionEntry(sessionKey string) *session.SessionEntry
	Save(entry *session.SessionEntry)
	ResolveMainSessionKey(sessionKey string) string
}

// SessionInitParams 会话初始化参数。
type SessionInitParams struct {
	Ctx               *autoreply.MsgContext
	CommandAuthorized bool
	Store             SessionStoreAccessor
	StorePath         string
	DefaultScope      SessionScope
}

// ---------- 核心函数 ----------

// InitSessionState 初始化会话状态。
// TS 对照: session.ts initSessionState (L14-220)
// 流程: 解析 session key → 查找/创建条目 → 检查分叉 → 返回结果
func InitSessionState(params SessionInitParams) (*SessionInitResult, error) {
	if params.Ctx == nil {
		return nil, fmt.Errorf("session: nil MsgContext")
	}
	if params.Store == nil {
		return nil, fmt.Errorf("session: nil SessionStoreAccessor")
	}

	// 1. 解析 session key
	sessionKey := ResolveSessionKey(params.Ctx, params.DefaultScope)
	if sessionKey == "" {
		return nil, fmt.Errorf("session: empty session key")
	}

	// 2. 解析主 session key（支持子会话）
	mainKey := params.Store.ResolveMainSessionKey(sessionKey)

	// 3. 加载已有条目
	existing := params.Store.LoadSessionEntry(mainKey)

	if existing != nil {
		// 更新 last access
		existing.UpdatedAt = time.Now().UnixMilli()
		params.Store.Save(existing)

		slog.Debug("session: loaded existing",
			"sessionKey", mainKey,
			"label", existing.Label,
		)

		return &SessionInitResult{
			Entry:      existing,
			IsNew:      false,
			SessionKey: mainKey,
			SessionID:  existing.SessionId,
			StorePath:  params.StorePath,
			Scope:      params.DefaultScope,
		}, nil
	}

	// 4. 创建新条目
	now := time.Now().UnixMilli()
	newEntry := &SessionEntry{
		SessionKey: mainKey,
		CreatedAt:  now,
		UpdatedAt:  now,
		Channel:    params.Ctx.ChannelType,
		ChatType:   params.Ctx.ChatType,
	}

	// 设置 origin
	newEntry.Origin = &session.SessionOrigin{
		Label:    ResolveConversationLabel(params.Ctx),
		ChatType: params.Ctx.ChatType,
		From:     params.Ctx.From,
	}

	// 设置 label
	newEntry.Label = newEntry.Origin.Label

	params.Store.Save(newEntry)

	slog.Info("session: created new",
		"sessionKey", mainKey,
		"label", newEntry.Label,
	)

	return &SessionInitResult{
		Entry:      newEntry,
		IsNew:      true,
		SessionKey: mainKey,
		StorePath:  params.StorePath,
		Scope:      params.DefaultScope,
	}, nil
}

// ResolveSessionKey 解析会话 key。
// TS 对照: session.ts resolveSessionKey
func ResolveSessionKey(ctx *autoreply.MsgContext, scope SessionScope) string {
	if ctx == nil {
		return ""
	}

	// 线程模式 — 以 threadId 为 key
	if scope == ScopeThread && ctx.MessageThreadID != "" {
		return fmt.Sprintf("thread:%s", ctx.MessageThreadID)
	}

	// 全局模式
	if scope == ScopeGlobal {
		return "global"
	}

	// 默认: conversation scope
	label := ctx.ConversationLabel
	if label == "" {
		label = ctx.From
	}
	if label == "" {
		return ""
	}
	return fmt.Sprintf("conv:%s", label)
}

// ForkSessionFromParent 从父会话分叉创建子会话。
// TS 对照: session.ts forkSessionFromParent
func ForkSessionFromParent(parent *SessionEntry, childKey string, store SessionStoreAccessor) *SessionEntry {
	if parent == nil || childKey == "" {
		return nil
	}

	now := time.Now().UnixMilli()
	child := &SessionEntry{
		SessionKey:    childKey,
		MainKey:       parent.SessionKey,
		CreatedAt:     now,
		UpdatedAt:     now,
		Channel:       parent.Channel,
		ChatType:      parent.ChatType,
		Label:         parent.Label,
		ThinkingLevel: parent.ThinkingLevel,
		Origin:        parent.Origin,
	}

	// 继承模型覆盖
	if parent.ModelOverride != "" {
		child.ModelOverride = parent.ModelOverride
		child.ProviderOverride = parent.ProviderOverride
	}

	if store != nil {
		store.Save(child)
	}
	return child
}
