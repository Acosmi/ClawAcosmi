package reply

import (
	"context"

	"github.com/anthropic/open-acosmi/internal/autoreply"
)

// TS 对照: auto-reply/reply/get-reply.ts (336L)

// GetReplyOptions get-reply 顶层选项。
type GetReplyOptions struct {
	AgentID      string
	AgentDir     string
	WorkspaceDir string
	StorePath    string
	SessionKey   string
	SessionID    string

	// 会话初始化
	SessionEntry *SessionEntry
	IsNewSession bool

	// 模型配置
	DefaultProvider string
	DefaultModel    string
	Provider        string
	Model           string
	ContextTokens   int

	// 超时
	TimeoutMs int

	// DI 接口
	MemoryFlusher MemoryFlusher
	AgentExecutor AgentExecutor

	// Typing 配置
	TypingMode TypingMode
}

// GetReplyFromConfig 从配置获取回复入口。
// TS 对照: get-reply.ts getReplyFromConfig (L43-335)
// 编排：配置加载 → 指令解析 → 指令应用 → 内联动作 → RunPreparedReply。
func GetReplyFromConfig(ctx context.Context, msgCtx *autoreply.MsgContext, opts *autoreply.GetReplyOptions, replyOpts *GetReplyOptions) ([]autoreply.ReplyPayload, error) {
	if msgCtx == nil {
		return nil, nil
	}
	if replyOpts == nil {
		replyOpts = &GetReplyOptions{}
	}
	if opts == nil {
		opts = &autoreply.GetReplyOptions{}
	}

	// 1. 创建 typing controller
	var onReplyStart func() error
	if opts.OnReplyStart != nil {
		cb := opts.OnReplyStart
		onReplyStart = func() error { cb(); return nil }
	}
	typing := NewTypingController(TypingControllerParams{
		OnReplyStart: onReplyStart,
		OnCleanup:    opts.OnTypingCleanup,
	})
	defer typing.Cleanup()

	// 2. 解析指令
	directiveResult, err := ResolveReplyDirectives(ResolveDirectivesParams{
		Ctx:                   msgCtx,
		AgentID:               replyOpts.AgentID,
		AgentDir:              replyOpts.AgentDir,
		WorkspaceDir:          replyOpts.WorkspaceDir,
		SessionEntry:          replyOpts.SessionEntry,
		SessionKey:            replyOpts.SessionKey,
		StorePath:             replyOpts.StorePath,
		IsGroup:               msgCtx.IsGroup,
		TriggerBodyNormalized: msgCtx.Body,
		CommandAuthorized:     msgCtx.CommandAuthorized,
		DefaultProvider:       replyOpts.DefaultProvider,
		DefaultModel:          replyOpts.DefaultModel,
		Provider:              replyOpts.Provider,
		Model:                 replyOpts.Model,
		Typing:                typing,
		Opts:                  opts,
		SkillFilter:           opts.SkillFilter,
	})
	if err != nil {
		return nil, err
	}

	// 指令直接返回
	if directiveResult.Kind == "reply" {
		return directiveResult.Reply, nil
	}
	cont := directiveResult.Continuation

	// 3. 应用指令覆盖
	applyResult, err := ApplyInlineDirectiveOverrides(ApplyOverrideParams{
		Ctx:               msgCtx,
		AgentID:           replyOpts.AgentID,
		SessionEntry:      replyOpts.SessionEntry,
		SessionKey:        replyOpts.SessionKey,
		StorePath:         replyOpts.StorePath,
		IsGroup:           msgCtx.IsGroup,
		AllowTextCommands: cont.AllowTextCommands,
		Command:           cont.Command,
		Directives:        cont.Directives,
		ElevatedEnabled:   cont.ElevatedEnabled,
		ElevatedAllowed:   cont.ElevatedAllowed,
		DefaultProvider:   replyOpts.DefaultProvider,
		DefaultModel:      replyOpts.DefaultModel,
		Provider:          cont.Provider,
		Model:             cont.Model,
		ContextTokens:     cont.ContextTokens,
		Typing:            typing,
	})
	if err != nil {
		return nil, err
	}
	if applyResult.Kind == "reply" {
		return applyResult.Reply, nil
	}

	// 4. 处理内联动作
	actionResult, err := HandleInlineActions(InlineActionParams{
		Ctx:                    msgCtx,
		AgentID:                replyOpts.AgentID,
		AgentDir:               replyOpts.AgentDir,
		SessionEntry:           replyOpts.SessionEntry,
		SessionKey:             replyOpts.SessionKey,
		StorePath:              replyOpts.StorePath,
		WorkspaceDir:           replyOpts.WorkspaceDir,
		IsGroup:                msgCtx.IsGroup,
		Opts:                   opts,
		Typing:                 typing,
		AllowTextCommands:      cont.AllowTextCommands,
		InlineStatusRequested:  cont.InlineStatusRequested,
		Command:                cont.Command,
		Directives:             applyResult.Directives,
		CleanedBody:            cont.CleanedBody,
		ElevatedEnabled:        cont.ElevatedEnabled,
		ElevatedAllowed:        cont.ElevatedAllowed,
		ElevatedFailures:       cont.ElevatedFailures,
		DefaultActivation:      cont.DefaultActivation,
		ResolvedThinkLevel:     cont.ResolvedThinkLevel,
		ResolvedVerboseLevel:   cont.ResolvedVerboseLevel,
		ResolvedReasoningLevel: cont.ResolvedReasoningLevel,
		ResolvedElevatedLevel:  cont.ResolvedElevatedLevel,
		Provider:               applyResult.Provider,
		Model:                  applyResult.Model,
		ContextTokens:          applyResult.ContextTokens,
		DirectiveAck:           applyResult.DirectiveAck,
		SkillFilter:            opts.SkillFilter,
	})
	if err != nil {
		return nil, err
	}
	if actionResult.Kind == "reply" {
		return actionResult.Reply, nil
	}

	// 5. 运行准备好的回复
	return RunPreparedReply(ctx, PreparedReplyParams{
		Ctx:                    msgCtx,
		AgentID:                replyOpts.AgentID,
		AgentDir:               replyOpts.AgentDir,
		CommandAuthorized:      msgCtx.CommandAuthorized,
		Command:                cont.Command,
		CommandSource:          cont.CommandSource,
		AllowTextCommands:      cont.AllowTextCommands,
		Directives:             actionResult.Directives,
		DefaultActivation:      cont.DefaultActivation,
		ResolvedThinkLevel:     cont.ResolvedThinkLevel,
		ResolvedVerboseLevel:   cont.ResolvedVerboseLevel,
		ResolvedReasoningLevel: cont.ResolvedReasoningLevel,
		ResolvedElevatedLevel:  cont.ResolvedElevatedLevel,
		ExecOverrides:          cont.ExecOverrides,
		ElevatedEnabled:        cont.ElevatedEnabled,
		ElevatedAllowed:        cont.ElevatedAllowed,
		BlockStreamingEnabled:  cont.BlockStreamingEnabled,
		BlockReplyChunking:     cont.BlockReplyChunking,
		BlockStreamingBreak:    cont.BlockStreamingBreak,
		Provider:               applyResult.Provider,
		Model:                  applyResult.Model,
		PerMessageQueueMode:    applyResult.PerMessageQueueMode,
		PerMessageQueueOptions: applyResult.PerMessageQueueOptions,
		Typing:                 typing,
		Opts:                   opts,
		DefaultProvider:        replyOpts.DefaultProvider,
		DefaultModel:           replyOpts.DefaultModel,
		TimeoutMs:              replyOpts.TimeoutMs,
		IsNewSession:           replyOpts.IsNewSession,
		SessionEntry:           replyOpts.SessionEntry,
		SessionKey:             replyOpts.SessionKey,
		SessionID:              replyOpts.SessionID,
		StorePath:              replyOpts.StorePath,
		WorkspaceDir:           replyOpts.WorkspaceDir,
		AbortedLastRun:         actionResult.AbortedLastRun,
		MemoryFlusher:          replyOpts.MemoryFlusher,
		AgentExecutor:          replyOpts.AgentExecutor,
		TypingMode:             replyOpts.TypingMode,
	})
}
