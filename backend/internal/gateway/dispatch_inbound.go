package gateway

// dispatch_inbound.go — gateway → autoreply 管线桥接
// 对应 TS src/auto-reply/dispatch.ts dispatchInboundMessage
//
// 在 chat.send 中调用，将用户消息路由到 autoreply 管线。
// 使用 DI 回调模式（PipelineDispatcher）避免 gateway ↔ autoreply/reply 的循环导入。
// 回调在 server.go 启动时由外层注入。

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/anthropic/open-acosmi/internal/autoreply"
)

// PipelineDispatcher 管线分发接口。
// 由外层（如 server.go）注入，打破 gateway ↔ autoreply/reply 循环导入。
type PipelineDispatcher func(ctx context.Context, msgCtx *autoreply.MsgContext, opts *autoreply.GetReplyOptions) ([]autoreply.ReplyPayload, error)

// DispatchInboundParams 入站消息分发参数。
type DispatchInboundParams struct {
	// MsgContext 消息上下文
	MsgCtx *autoreply.MsgContext

	// 会话信息
	SessionKey string
	SessionID  string
	StorePath  string
	AgentID    string

	// abort 信号
	Ctx context.Context

	// 事件处理
	RunID string

	// 管线分发器（DI 注入）
	Dispatcher PipelineDispatcher
}

// DispatchInboundResult 分发结果。
type DispatchInboundResult struct {
	Replies []autoreply.ReplyPayload
	Error   error
}

// DispatchInboundMessage 将入站消息路由到 autoreply 管线。
// TS 对照: auto-reply/dispatch.ts dispatchInboundMessage
//
// 流程：
// 1. 构建 GetReplyOptions
// 2. 通过 PipelineDispatcher 分发到 autoreply 管线
// 3. 返回回复结果
func DispatchInboundMessage(ctx context.Context, params DispatchInboundParams) *DispatchInboundResult {
	if params.MsgCtx == nil {
		return &DispatchInboundResult{Error: fmt.Errorf("MsgCtx is required")}
	}

	// 使用 stub 分发器（如果未注入管线分发器）
	if params.Dispatcher == nil {
		slog.Info("dispatch_inbound: no pipeline dispatcher injected, using stub",
			"sessionKey", params.SessionKey,
			"runId", params.RunID,
		)
		return &DispatchInboundResult{
			Replies: []autoreply.ReplyPayload{},
		}
	}

	// 构建 GetReplyOptions
	getReplyOpts := &autoreply.GetReplyOptions{
		RunID:       params.RunID,
		IsHeartbeat: false,
	}

	slog.Info("dispatch_inbound: starting pipeline",
		"sessionKey", params.SessionKey,
		"agentId", params.AgentID,
		"runId", params.RunID,
		"body", truncateForLog(params.MsgCtx.Body, 80),
	)

	// 调用管线分发器
	replies, err := params.Dispatcher(ctx, params.MsgCtx, getReplyOpts)
	if err != nil {
		slog.Error("dispatch_inbound: pipeline error",
			"error", err,
			"sessionKey", params.SessionKey,
			"runId", params.RunID,
		)
		return &DispatchInboundResult{Error: err}
	}

	slog.Info("dispatch_inbound: pipeline complete",
		"sessionKey", params.SessionKey,
		"runId", params.RunID,
		"replyCount", len(replies),
	)

	return &DispatchInboundResult{Replies: replies}
}

// CombineReplyPayloads 合并回复载荷文本。
// TS 对照: chat.ts .then 块中的 combinedReply 逻辑
func CombineReplyPayloads(replies []autoreply.ReplyPayload) string {
	parts := make([]string, 0, len(replies))
	for _, r := range replies {
		text := strings.TrimSpace(r.Text)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

// truncateForLog 截断字符串用于日志。
func truncateForLog(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
}
