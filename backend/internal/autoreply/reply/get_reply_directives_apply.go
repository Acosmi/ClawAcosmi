package reply

import (
	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
)

// TS 对照: auto-reply/reply/get-reply-directives-apply.ts (315L)

// ApplyDirectiveResult 指令应用结果。
type ApplyDirectiveResult struct {
	Kind                   string // "reply" | "continue"
	Reply                  []autoreply.ReplyPayload
	Directives             InlineDirectives
	Provider               string
	Model                  string
	ContextTokens          int
	DirectiveAck           *autoreply.ReplyPayload
	PerMessageQueueMode    QueueMode
	PerMessageQueueOptions *QueueOptions
}

// ApplyOverrideParams 指令覆盖应用参数。
type ApplyOverrideParams struct {
	Ctx               *autoreply.MsgContext
	AgentID           string
	SessionEntry      *SessionEntry
	SessionKey        string
	StorePath         string
	IsGroup           bool
	AllowTextCommands bool
	Command           CommandContext
	Directives        InlineDirectives
	ElevatedEnabled   bool
	ElevatedAllowed   bool
	ElevatedFailures  []ElevatedFailure
	DefaultProvider   string
	DefaultModel      string
	Provider          string
	Model             string
	ContextTokens     int
	Typing            *TypingController

	// DI 回调：纯指令消息处理。返回回复文本。
	HandleDirectiveOnlyFn func(directives InlineDirectives) *autoreply.ReplyPayload

	// DI 回调：快速通道指令应用。返回 (ack, provider, model)。
	ApplyFastLaneFn func(directives InlineDirectives) (ack *autoreply.ReplyPayload, provider, model string)

	// DI 回调：保存 session entry。
	SaveSessionFn func(entry *SessionEntry)
}

// ApplyInlineDirectiveOverrides 应用内联指令覆盖。
// TS 对照: get-reply-directives-apply.ts applyInlineDirectiveOverrides (L36-314)
// 处理：指令唯一消息检测、指令持久化、模型切换、队列模式设置。
func ApplyInlineDirectiveOverrides(params ApplyOverrideParams) (*ApplyDirectiveResult, error) {
	directives := params.Directives
	provider := params.Provider
	model := params.Model
	contextTokens := params.ContextTokens

	// 1. 非授权发送者 → 清除所有权限指令
	if !params.Command.IsAuthorizedSender {
		directives = clearPrivilegedDirectives(directives)
	}

	// 2. 指令唯一检测（纯指令消息 → 直接响应）
	if isDirectiveOnlyMessage(directives, params.Ctx, params.AgentID, params.IsGroup) {
		if !params.Command.IsAuthorizedSender {
			if params.Typing != nil {
				params.Typing.Cleanup()
			}
			return &ApplyDirectiveResult{Kind: "reply"}, nil
		}
		// 纯指令消息处理。
		var directiveReply *autoreply.ReplyPayload
		if params.HandleDirectiveOnlyFn != nil {
			directiveReply = params.HandleDirectiveOnlyFn(directives)
		}
		// 持久化指令到 session。
		PersistInlineDirectives(PersistDirectiveParams{
			Directives:      directives,
			SessionEntry:    params.SessionEntry,
			SessionKey:      params.SessionKey,
			StorePath:       params.StorePath,
			ElevatedEnabled: params.ElevatedEnabled,
			ElevatedAllowed: params.ElevatedAllowed,
			DefaultProvider: params.DefaultProvider,
			DefaultModel:    params.DefaultModel,
			Provider:        provider,
			Model:           model,
			ContextTokens:   contextTokens,
			SaveSessionFn:   params.SaveSessionFn,
		})
		if params.Typing != nil {
			params.Typing.Cleanup()
		}
		var replies []autoreply.ReplyPayload
		if directiveReply != nil {
			replies = []autoreply.ReplyPayload{*directiveReply}
		}
		return &ApplyDirectiveResult{Kind: "reply", Reply: replies}, nil
	}

	// 3. 有指令且授权 → 快速通道处理
	hasAnyDirective := directives.HasThinkDirective ||
		directives.HasVerboseDirective ||
		directives.HasReasoningDirective ||
		directives.HasElevatedDirective ||
		directives.HasExecDirective ||
		directives.HasModelDirective ||
		directives.HasQueueDirective ||
		directives.HasStatusDirective

	var directiveAck *autoreply.ReplyPayload
	if hasAnyDirective && params.Command.IsAuthorizedSender {
		// 快速通道指令应用。
		if params.ApplyFastLaneFn != nil {
			ack, newProvider, newModel := params.ApplyFastLaneFn(directives)
			directiveAck = ack
			if newProvider != "" {
				provider = newProvider
			}
			if newModel != "" {
				model = newModel
			}
		}

		// 持久化指令到 session。
		persisted := PersistInlineDirectives(PersistDirectiveParams{
			Directives:      directives,
			SessionEntry:    params.SessionEntry,
			SessionKey:      params.SessionKey,
			StorePath:       params.StorePath,
			ElevatedEnabled: params.ElevatedEnabled,
			ElevatedAllowed: params.ElevatedAllowed,
			DefaultProvider: params.DefaultProvider,
			DefaultModel:    params.DefaultModel,
			Provider:        provider,
			Model:           model,
			ContextTokens:   contextTokens,
			SaveSessionFn:   params.SaveSessionFn,
		})
		provider = persisted.Provider
		model = persisted.Model
		contextTokens = persisted.ContextTokens
	}

	// 4. 队列模式
	var perMessageQueueMode QueueMode
	var perMessageQueueOptions *QueueOptions
	if directives.HasQueueDirective && !directives.QueueReset {
		perMessageQueueMode = directives.QueueMode
		perMessageQueueOptions = &QueueOptions{
			DebounceMs: directives.DebounceMs,
			Cap:        directives.Cap,
			DropPolicy: directives.DropPolicy,
		}
	}

	return &ApplyDirectiveResult{
		Kind:                   "continue",
		Directives:             directives,
		Provider:               provider,
		Model:                  model,
		ContextTokens:          contextTokens,
		DirectiveAck:           directiveAck,
		PerMessageQueueMode:    perMessageQueueMode,
		PerMessageQueueOptions: perMessageQueueOptions,
	}, nil
}

// clearPrivilegedDirectives 清除需要授权的指令。
func clearPrivilegedDirectives(d InlineDirectives) InlineDirectives {
	d.HasThinkDirective = false
	d.HasVerboseDirective = false
	d.HasReasoningDirective = false
	d.HasElevatedDirective = false
	d.HasExecDirective = false
	d.ExecHost = ""
	d.ExecSecurity = ""
	d.ExecAsk = ""
	d.ExecNode = ""
	d.HasStatusDirective = false
	d.HasModelDirective = false
	d.HasQueueDirective = false
	d.QueueReset = false
	return d
}

// isDirectiveOnlyMessage 检查消息是否仅含指令。
func isDirectiveOnlyMessage(d InlineDirectives, ctx *autoreply.MsgContext, agentID string, isGroup bool) bool {
	return IsDirectiveOnly(d, d.Cleaned, isGroup, nil)
}
