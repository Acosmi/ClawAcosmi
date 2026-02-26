package autoreply

import "context"

// TS 对照: auto-reply/dispatch.ts (78L)
// 顶层分发入口。
// 为避免 autoreply ↔ autoreply/reply 循环导入，
// 通过 DI 函数签名注入 reply 包的具体实现。

// DispatchOutcome 分发结果。
type DispatchOutcome string

const (
	DispatchCompleted DispatchOutcome = "completed"
	DispatchSkipped   DispatchOutcome = "skipped"
	DispatchError     DispatchOutcome = "error"
)

// DispatchRecord 分发记录。
type DispatchRecord struct {
	Outcome DispatchOutcome
	Reason  string
	Error   string
}

// NewDispatchRecord 创建分发记录。
func NewDispatchRecord(outcome DispatchOutcome, reason string) DispatchRecord {
	return DispatchRecord{Outcome: outcome, Reason: reason}
}

// ---------- 统一分发入口 ----------

// DispatchInboundResult 入站分发结果。
// TS 对照: dispatch.ts L15 — DispatchFromConfigResult 的别名
type DispatchInboundResult struct {
	QueuedFinal bool
	Counts      map[string]int
}

// DispatcherHandle 分发器句柄（DI 接口）。
// 由 reply.ReplyDispatcher 实现。
type DispatcherHandle interface {
	WaitForIdle()
}

// DispatcherWithTypingResult 带打字指示符的分发器创建结果。
type DispatcherWithTypingResult struct {
	Dispatcher       DispatcherHandle
	MarkDispatchIdle func()
	ReplyOptions     *GetReplyOptions
}

// DispatchInboundParams 入站分发参数。
// TS 对照: dispatch.ts L17-22 — dispatchInboundMessage 参数
type DispatchInboundParams struct {
	Ctx          *MsgContext
	ReplyOptions *GetReplyOptions

	// DI — 由上层注入 reply 包实现
	FinalizeContextFn    func(ctx *MsgContext)
	DispatchFromConfigFn func(ctx context.Context, msgCtx *MsgContext, dispatcher DispatcherHandle, replyOpts *GetReplyOptions) (*DispatchInboundResult, error)
	Dispatcher           DispatcherHandle
}

// DispatchInboundMessage 入站消息统一分发入口。
// TS 对照: dispatch.ts L17-32 — dispatchInboundMessage()
//
// 流程: finalizeInboundContext → dispatchReplyFromConfig
func DispatchInboundMessage(ctx context.Context, params DispatchInboundParams) (*DispatchInboundResult, error) {
	// 1. finalize context
	if params.FinalizeContextFn != nil {
		params.FinalizeContextFn(params.Ctx)
	}

	// 2. dispatch
	if params.DispatchFromConfigFn == nil {
		return &DispatchInboundResult{}, nil
	}
	return params.DispatchFromConfigFn(ctx, params.Ctx, params.Dispatcher, params.ReplyOptions)
}

// DispatchInboundWithDispatcherParams 带分发器创建的分发参数。
// TS 对照: dispatch.ts L60-77 — dispatchInboundMessageWithDispatcher()
type DispatchInboundWithDispatcherParams struct {
	Ctx          *MsgContext
	ReplyOptions *GetReplyOptions

	// DI — 由上层注入
	FinalizeContextFn    func(ctx *MsgContext)
	DispatchFromConfigFn func(ctx context.Context, msgCtx *MsgContext, dispatcher DispatcherHandle, replyOpts *GetReplyOptions) (*DispatchInboundResult, error)
	CreateDispatcherFn   func() DispatcherHandle
}

// DispatchInboundMessageWithDispatcher 创建普通分发器并执行入站分发。
// TS 对照: dispatch.ts L60-77 — dispatchInboundMessageWithDispatcher()
func DispatchInboundMessageWithDispatcher(ctx context.Context, params DispatchInboundWithDispatcherParams) (*DispatchInboundResult, error) {
	var dispatcher DispatcherHandle
	if params.CreateDispatcherFn != nil {
		dispatcher = params.CreateDispatcherFn()
	}

	result, err := DispatchInboundMessage(ctx, DispatchInboundParams{
		Ctx:                  params.Ctx,
		ReplyOptions:         params.ReplyOptions,
		FinalizeContextFn:    params.FinalizeContextFn,
		DispatchFromConfigFn: params.DispatchFromConfigFn,
		Dispatcher:           dispatcher,
	})

	// 等待分发器空闲
	if dispatcher != nil {
		dispatcher.WaitForIdle()
	}
	return result, err
}

// DispatchInboundWithBufferedDispatcherParams 带缓冲分发器的分发参数。
// TS 对照: dispatch.ts L34-58 — dispatchInboundMessageWithBufferedDispatcher()
type DispatchInboundWithBufferedDispatcherParams struct {
	Ctx          *MsgContext
	ReplyOptions *GetReplyOptions

	// DI — 由上层注入
	FinalizeContextFn            func(ctx *MsgContext)
	DispatchFromConfigFn         func(ctx context.Context, msgCtx *MsgContext, dispatcher DispatcherHandle, replyOpts *GetReplyOptions) (*DispatchInboundResult, error)
	CreateDispatcherWithTypingFn func() DispatcherWithTypingResult
}

// DispatchInboundMessageWithBufferedDispatcher 创建带打字指示符的分发器并执行入站分发。
// TS 对照: dispatch.ts L34-58 — dispatchInboundMessageWithBufferedDispatcher()
func DispatchInboundMessageWithBufferedDispatcher(ctx context.Context, params DispatchInboundWithBufferedDispatcherParams) (*DispatchInboundResult, error) {
	if params.CreateDispatcherWithTypingFn == nil {
		return DispatchInboundMessage(ctx, DispatchInboundParams{
			Ctx:                  params.Ctx,
			ReplyOptions:         params.ReplyOptions,
			FinalizeContextFn:    params.FinalizeContextFn,
			DispatchFromConfigFn: params.DispatchFromConfigFn,
		})
	}

	dtResult := params.CreateDispatcherWithTypingFn()

	// 合并 reply options
	replyOpts := params.ReplyOptions
	if dtResult.ReplyOptions != nil && replyOpts != nil {
		// 合并 typing 相关的回调
		merged := *replyOpts
		if dtResult.ReplyOptions.OnReplyStart != nil {
			merged.OnReplyStart = dtResult.ReplyOptions.OnReplyStart
		}
		if dtResult.ReplyOptions.OnTypingCleanup != nil {
			merged.OnTypingCleanup = dtResult.ReplyOptions.OnTypingCleanup
		}
		replyOpts = &merged
	} else if dtResult.ReplyOptions != nil {
		replyOpts = dtResult.ReplyOptions
	}

	result, err := DispatchInboundMessage(ctx, DispatchInboundParams{
		Ctx:                  params.Ctx,
		ReplyOptions:         replyOpts,
		FinalizeContextFn:    params.FinalizeContextFn,
		DispatchFromConfigFn: params.DispatchFromConfigFn,
		Dispatcher:           dtResult.Dispatcher,
	})

	// 标记分发空闲
	if dtResult.MarkDispatchIdle != nil {
		dtResult.MarkDispatchIdle()
	}
	return result, err
}
