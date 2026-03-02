package reply

import (
	"strings"

	"github.com/openacosmi/claw-acismi/internal/autoreply"
)

// TS 对照: auto-reply/reply/streaming-directives.ts (129L)
//
// 流式 [[...]] directive 的增量解析逻辑。
// 在 SSE/stream 模式下，directive 标签可能跨多个 chunk 到达，
// 本模块负责将不完整的 "[[ ... " 尾片段暂存，待下一个 chunk 到达后合并解析。

// StreamingDirectiveChunk 流式 directive 解析结果。
// TS 对照: streaming-directives.ts ParsedChunk (L12-14)
type StreamingDirectiveChunk struct {
	autoreply.ReplyPayload
	// IsSilent 标记当前 chunk 是否为静默回复（已被去除）
	IsSilent bool
}

// pendingReplyState 保存跨 chunk 的 reply 标签状态。
// TS 对照: streaming-directives.ts PendingReplyState (L6-10)
type pendingReplyState struct {
	explicitID string
	sawCurrent bool
	hasTag     bool
}

// StreamingDirectiveAccumulator 流式 directive 累加器。
// TS 对照: streaming-directives.ts createStreamingDirectiveAccumulator (L74-128)
type StreamingDirectiveAccumulator struct {
	pendingTail  string
	pendingReply pendingReplyState
	silentToken  string
}

// NewStreamingDirectiveAccumulator 创建新的流式 directive 累加器。
// TS 对照: streaming-directives.ts createStreamingDirectiveAccumulator (L74-128)
func NewStreamingDirectiveAccumulator(silentToken string) *StreamingDirectiveAccumulator {
	tok := silentToken
	if tok == "" {
		tok = autoreply.SilentReplyToken
	}
	return &StreamingDirectiveAccumulator{silentToken: tok}
}

// Reset 重置累加器状态。
// TS 对照: streaming-directives.ts reset (L78-81)
func (a *StreamingDirectiveAccumulator) Reset() {
	a.pendingTail = ""
	a.pendingReply = pendingReplyState{}
}

// splitTrailingDirective 拆分末尾不完整的 [[...]] 标签。
// 若文本末尾有未闭合的 [[，则将其分离为 tail；否则 tail 为空。
// TS 对照: streaming-directives.ts splitTrailingDirective (L21-34)
func splitTrailingDirective(text string) (body, tail string) {
	openIdx := strings.LastIndex(text, "[[")
	if openIdx < 0 {
		return text, ""
	}
	closeIdx := strings.Index(text[openIdx+2:], "]]")
	if closeIdx >= 0 {
		// 已闭合，不需要暂存
		return text, ""
	}
	return text[:openIdx], text[openIdx:]
}

// parseStreamingChunk 解析单个 chunk 的内容。
// TS 对照: streaming-directives.ts parseChunk (L36-66)
func parseStreamingChunk(raw string, silentToken string) StreamingDirectiveChunk {
	// 提取媒体 URL（inline [[media:...]] 等）
	text, mediaURL, mediaURLs, audioAsVoice := extractStreamingMedia(raw)

	// 提取 [[reply:...]] 标签
	replyToID, replyToExplicitID, replyToCurrent, hasReplyTag, cleanedText := extractStreamingReplyTags(text)
	text = cleanedText

	// 检查静默标记
	isSilent := autoreply.IsSilentReplyText(text, silentToken)
	if isSilent {
		text = ""
	}

	chunk := StreamingDirectiveChunk{
		ReplyPayload: autoreply.ReplyPayload{
			Text:           text,
			MediaURL:       mediaURL,
			MediaURLs:      mediaURLs,
			ReplyToID:      replyToID,
			ReplyToTag:     hasReplyTag,
			ReplyToCurrent: replyToCurrent,
			AudioAsVoice:   audioAsVoice,
		},
		IsSilent: isSilent,
	}
	_ = replyToExplicitID // used below in Consume via explicit accumulation
	return chunk
}

// hasRenderableContent 判断 ReplyPayload 是否有可渲染内容。
// TS 对照: streaming-directives.ts hasRenderableContent (L68-73)
func hasRenderableContent(p autoreply.ReplyPayload) bool {
	return p.Text != "" ||
		p.MediaURL != "" ||
		len(p.MediaURLs) > 0 ||
		p.AudioAsVoice
}

// ConsumeOptions 消费选项。
// TS 对照: streaming-directives.ts ConsumeOptions (L16-19)
type ConsumeOptions struct {
	// Final 若为 true，则不暂存末尾不完整 directive，直接解析所有内容
	Final bool
	// SilentToken 覆盖默认静默 token
	SilentToken string
}

// Consume 消费一个 raw chunk，返回可投递的解析结果（若为 nil 则需继续等待）。
// TS 对照: streaming-directives.ts consume (L83-122)
func (a *StreamingDirectiveAccumulator) Consume(raw string, opts ConsumeOptions) *autoreply.ReplyPayload {
	silentToken := opts.SilentToken
	if silentToken == "" {
		silentToken = a.silentToken
	}

	combined := a.pendingTail + raw
	a.pendingTail = ""

	if !opts.Final {
		body, tail := splitTrailingDirective(combined)
		combined = body
		a.pendingTail = tail
	}

	if combined == "" {
		return nil
	}

	parsed := parseStreamingChunk(combined, silentToken)

	hasTag := a.pendingReply.hasTag || parsed.ReplyToTag
	sawCurrent := a.pendingReply.sawCurrent || parsed.ReplyToCurrent
	explicitID := parsed.ReplyToID
	if explicitID == "" {
		explicitID = a.pendingReply.explicitID
	}

	combined_result := autoreply.ReplyPayload{
		Text:           parsed.Text,
		MediaURL:       parsed.MediaURL,
		MediaURLs:      parsed.MediaURLs,
		ReplyToID:      explicitID,
		ReplyToCurrent: sawCurrent,
		ReplyToTag:     hasTag,
		AudioAsVoice:   parsed.AudioAsVoice,
	}

	if !hasRenderableContent(combined_result) {
		if hasTag {
			a.pendingReply = pendingReplyState{
				explicitID: explicitID,
				sawCurrent: sawCurrent,
				hasTag:     hasTag,
			}
		}
		return nil
	}

	// 有可渲染内容，重置 pending reply 状态
	a.pendingReply = pendingReplyState{}
	return &combined_result
}

// extractStreamingMedia 从 raw 文本中提取媒体 URL（stub，具体逻辑依赖 media/parse）。
// TS 对照: streaming-directives.ts splitMediaFromOutput (外部依赖)
// Go 端轻量实现：暂不解析内联媒体 URL，直接透传文本。
func extractStreamingMedia(raw string) (text, mediaURL string, mediaURLs []string, audioAsVoice bool) {
	// 此处保持与 TS splitMediaFromOutput 的接口对等。
	// 完整媒体解析逻辑在 backend/internal/media/ 包实现。
	return raw, "", nil, false
}

// extractStreamingReplyTags 从文本中提取 [[reply:...]] 标签。
// TS 对照: streaming-directives.ts parseInlineDirectives → replyToId/replyToCurrent/hasReplyTag
// 轻量实现：识别 [[reply:ID]]、[[reply:current]] 标签并从文本中剥离。
func extractStreamingReplyTags(text string) (replyToID, replyToExplicitID string, replyToCurrent, hasReplyTag bool, cleaned string) {
	cleaned = text
	result := text
	searchFrom := 0
	for {
		idx := strings.Index(result[searchFrom:], "[[")
		if idx < 0 {
			break
		}
		openIdx := searchFrom + idx
		closeIdx := strings.Index(result[openIdx+2:], "]]")
		if closeIdx < 0 {
			break
		}
		closeIdx += openIdx + 2
		inner := strings.TrimSpace(result[openIdx+2 : closeIdx])
		lowerInner := strings.ToLower(inner)
		if strings.HasPrefix(lowerInner, "reply:") {
			hasReplyTag = true
			val := strings.TrimSpace(inner[6:])
			if strings.ToLower(val) == "current" {
				replyToCurrent = true
			} else if val != "" {
				replyToID = val
				replyToExplicitID = val
			}
			result = result[:openIdx] + result[closeIdx+2:]
		} else if lowerInner == "reply_to_current" {
			hasReplyTag = true
			replyToCurrent = true
			result = result[:openIdx] + result[closeIdx+2:]
		} else if strings.HasPrefix(lowerInner, "reply_to:") {
			hasReplyTag = true
			val := strings.TrimSpace(inner[9:])
			if val != "" {
				replyToID = val
				replyToExplicitID = val
			}
			result = result[:openIdx] + result[closeIdx+2:]
		} else {
			// 非 reply 标签，跳过继续扫描
			searchFrom = closeIdx + 2
		}
	}
	cleaned = result
	return replyToID, replyToExplicitID, replyToCurrent, hasReplyTag, cleaned
}
