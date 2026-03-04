package reply

import (
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/agents/runner"
	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
)

// TS 对照: auto-reply/reply/agent-runner-payloads.ts (122L)

// BuildReplyPayloadsParams 构建回复载荷参数。
type BuildReplyPayloadsParams struct {
	Payloads                 []autoreply.ReplyPayload
	IsHeartbeat              bool
	DidLogHeartbeatStrip     bool
	BlockStreamingEnabled    bool
	BlockStreamingAborted    bool // 块流式是否中断
	BlockStreamingDone       bool // 块流式是否完成
	ReplyToMode              string
	ReplyToChannel           string
	CurrentMessageID         string
	MessageProvider          string
	MessagingToolSentTargets []runner.MessagingToolSend
	OriginatingTo            string
	AccountID                string
}

// BuildReplyPayloadsResult 构建回复载荷结果。
type BuildReplyPayloadsResult struct {
	ReplyPayloads        []autoreply.ReplyPayload
	DidLogHeartbeatStrip bool
}

// BuildReplyPayloads 构建最终回复载荷。
// TS 对照: agent-runner-payloads.ts L17-121
// 执行：心跳剥离、Bun 错误格式化、回复线程化、指令解析、去重、块流式过滤。
func BuildReplyPayloads(params BuildReplyPayloadsParams) BuildReplyPayloadsResult {
	didLogHeartbeatStrip := params.DidLogHeartbeatStrip

	// 1. 清洗载荷：Bun 错误 + 心跳剥离
	sanitized := sanitizePayloads(params.Payloads, params.IsHeartbeat, &didLogHeartbeatStrip)

	// 2. 过滤空载荷
	var renderable []autoreply.ReplyPayload
	for _, p := range sanitized {
		if isRenderablePayload(p) {
			renderable = append(renderable, p)
		}
	}

	// 3. 块流式过滤
	filtered := renderable
	if params.BlockStreamingEnabled && params.BlockStreamingDone && !params.BlockStreamingAborted {
		// 块流式正常完成 → 丢弃最终载荷（已通过流发送）
		filtered = nil
	}

	return BuildReplyPayloadsResult{
		ReplyPayloads:        filtered,
		DidLogHeartbeatStrip: didLogHeartbeatStrip,
	}
}

// sanitizePayloads 清洗载荷列表。
func sanitizePayloads(payloads []autoreply.ReplyPayload, isHeartbeat bool, didLog *bool) []autoreply.ReplyPayload {
	if isHeartbeat {
		return payloads
	}

	var result []autoreply.ReplyPayload
	for _, p := range payloads {
		text := p.Text

		// Bun fetch socket 错误格式化
		if p.IsError && text != "" && IsBunFetchSocketError(text) {
			text = FormatBunFetchSocketError(text)
		}

		// 心跳令牌剥离
		if text == "" || !strings.Contains(text, "HEARTBEAT_OK") {
			out := p
			out.Text = text
			result = append(result, out)
			continue
		}

		stripped := autoreply.StripHeartbeatToken(text, &autoreply.StripHeartbeatOpts{Mode: "message"})
		if stripped.DidStrip && !*didLog {
			*didLog = true
		}

		hasMedia := p.MediaURL != "" ||
			len(p.MediaURLs) > 0 ||
			len(p.MediaItems) > 0 ||
			p.MediaBase64 != ""
		if stripped.ShouldSkip && !hasMedia {
			continue
		}

		out := p
		out.Text = stripped.Text
		result = append(result, out)
	}
	return result
}

// isRenderablePayload 判断载荷是否可渲染。
func isRenderablePayload(p autoreply.ReplyPayload) bool {
	if strings.TrimSpace(p.Text) != "" {
		return true
	}
	if p.MediaURL != "" {
		return true
	}
	if len(p.MediaURLs) > 0 {
		return true
	}
	if p.MediaBase64 != "" {
		return true
	}
	if len(p.MediaItems) > 0 {
		return true
	}
	return false
}
