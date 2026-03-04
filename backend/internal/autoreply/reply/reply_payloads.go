package reply

import (
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
)

// TS 对照: auto-reply/reply/reply-payloads.ts (124L)
//
// 回复载荷的后处理逻辑：
//   - 应用 [[reply:...]] 标签到 payload
//   - 过滤不可渲染 payload
//   - 应用 reply threading 模式
//   - 过滤 messaging tool 重复投递
//   - 判断是否应抑制 messaging tool 回复

// OriginatingChannelType 消息来源通道类型。
// TS 对照: templating.ts OriginatingChannelType
type OriginatingChannelType string

// MessagingToolSend messaging tool 发送记录。
// TS 对照: agents/pi-embedded-runner.ts MessagingToolSend
type MessagingToolSend struct {
	Provider  string `json:"provider,omitempty"`
	To        string `json:"to,omitempty"`
	Text      string `json:"text,omitempty"`
	AccountID string `json:"accountId,omitempty"`
}

// ReplyToMode 回复引用模式。
// TS 对照: config/types.ts ReplyToMode
type ReplyToMode string

const (
	ReplyToModeOff   ReplyToMode = "off"
	ReplyToModeFirst ReplyToMode = "first"
	ReplyToModeAll   ReplyToMode = "all"
)

// ApplyReplyTagsToPayload 应用 [[reply:...]] 标签到 payload。
// TS 对照: reply-payloads.ts applyReplyTagsToPayload (L10-45)
//
// 逻辑：
//   - 若 payload.Text 为 "" 且 replyToCurrent=true 但无 replyToId → 用 currentMessageId 填充
//   - 若 payload.Text 含 "[[" → 解析 [[reply:...]] 标签并回写到 payload 字段
//   - 否则保持原样
func ApplyReplyTagsToPayload(payload autoreply.ReplyPayload, currentMessageID string) autoreply.ReplyPayload {
	if payload.Text == "" {
		// 无文本：仅处理 replyToCurrent → replyToId 填充
		if !payload.ReplyToCurrent || payload.ReplyToID != "" {
			return payload
		}
		p := payload
		p.ReplyToID = strings.TrimSpace(currentMessageID)
		return p
	}

	// 检查是否需要解析标签
	if !strings.Contains(payload.Text, "[[") {
		// 无标签但 replyToCurrent=true：填充 replyToId
		if !payload.ReplyToCurrent || payload.ReplyToID != "" {
			return payload
		}
		p := payload
		p.ReplyToID = strings.TrimSpace(currentMessageID)
		p.ReplyToTag = p.ReplyToTag || true
		return p
	}

	// 解析 [[reply:...]] 标签
	result := ExtractReplyToTag(payload.Text, currentMessageID)
	p := payload
	if result.Cleaned != "" {
		p.Text = result.Cleaned
	} else {
		p.Text = ""
	}
	if result.ReplyToID != "" {
		p.ReplyToID = result.ReplyToID
	}
	p.ReplyToTag = result.HasTag || payload.ReplyToTag
	p.ReplyToCurrent = result.ReplyToCurrent || payload.ReplyToCurrent
	return p
}

// IsRenderablePayload 判断 payload 是否有可渲染内容。
// TS 对照: reply-payloads.ts isRenderablePayload (L47-55)
func IsRenderablePayload(payload autoreply.ReplyPayload) bool {
	return payload.Text != "" ||
		payload.MediaURL != "" ||
		len(payload.MediaURLs) > 0 ||
		len(payload.MediaItems) > 0 ||
		payload.MediaBase64 != "" ||
		payload.AudioAsVoice ||
		len(payload.ChannelData) > 0
}

// ApplyReplyThreadingParams applyReplyThreading 参数。
type ApplyReplyThreadingParams struct {
	Payloads         []autoreply.ReplyPayload
	ReplyToMode      ReplyToMode
	ReplyToChannel   OriginatingChannelType
	CurrentMessageID string
}

// ApplyReplyThreading 应用 reply threading 模式过滤。
// TS 对照: reply-payloads.ts applyReplyThreading (L57-69)
//
// 逻辑：
//  1. 对每个 payload 应用 reply 标签解析
//  2. 过滤掉不可渲染的 payload
//  3. 应用 replyToMode 过滤策略
func ApplyReplyThreading(params ApplyReplyThreadingParams) []autoreply.ReplyPayload {
	var result []autoreply.ReplyPayload
	isFirst := true

	for _, payload := range params.Payloads {
		// 1. 应用标签
		p := ApplyReplyTagsToPayload(payload, params.CurrentMessageID)

		// 2. 过滤不可渲染
		if !IsRenderablePayload(p) {
			continue
		}

		// 3. 应用 replyToMode 策略
		p = applyReplyToMode(p, params.ReplyToMode, params.ReplyToChannel, isFirst)
		isFirst = false
		result = append(result, p)
	}
	return result
}

// applyReplyToMode 根据 replyToMode 策略调整 payload 的 replyToId。
// TS 对照: reply-threading.ts createReplyToModeFilterForChannel
func applyReplyToMode(p autoreply.ReplyPayload, mode ReplyToMode, _ OriginatingChannelType, isFirst bool) autoreply.ReplyPayload {
	switch mode {
	case ReplyToModeOff:
		// 关闭 reply：清除所有 reply 标记（除非 payload 显式通过标签设置了 replyToId）
		if !p.ReplyToTag {
			p.ReplyToID = ""
			p.ReplyToCurrent = false
		}
	case ReplyToModeFirst:
		// 仅第一条消息回复原消息
		if !isFirst && !p.ReplyToTag {
			p.ReplyToID = ""
			p.ReplyToCurrent = false
		}
	case ReplyToModeAll:
		// 所有消息都回复原消息（保持原样）
	}
	return p
}

// FilterMessagingToolDuplicates 过滤 messaging tool 已发送的重复 payload。
// TS 对照: reply-payloads.ts filterMessagingToolDuplicates (L71-80)
func FilterMessagingToolDuplicates(payloads []autoreply.ReplyPayload, sentTexts []string) []autoreply.ReplyPayload {
	if len(sentTexts) == 0 {
		return payloads
	}
	var result []autoreply.ReplyPayload
	for _, p := range payloads {
		if !isMessagingToolDuplicate(p.Text, sentTexts) {
			result = append(result, p)
		}
	}
	return result
}

// isMessagingToolDuplicate 检查文本是否与已发送文本重复。
// TS 对照: agents/pi-embedded-helpers.ts isMessagingToolDuplicate
//
// 使用前缀匹配：若 payload 文本以某条已发送文本为前缀（允许 whitespace trim），
// 则视为重复。
func isMessagingToolDuplicate(text string, sentTexts []string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	for _, sent := range sentTexts {
		sentTrimmed := strings.TrimSpace(sent)
		if sentTrimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, sentTrimmed) || strings.HasPrefix(sentTrimmed, trimmed) {
			return true
		}
	}
	return false
}

// ShouldSuppressMessagingToolRepliesParams 参数。
type ShouldSuppressMessagingToolRepliesParams struct {
	MessageProvider          string
	MessagingToolSentTargets []MessagingToolSend
	OriginatingTo            string
	AccountID                string
}

// ShouldSuppressMessagingToolReplies 判断是否应抑制 messaging tool 回复。
// TS 对照: reply-payloads.ts shouldSuppressMessagingToolReplies (L87-123)
//
// 逻辑：若 messaging tool 已经向同一 provider + to 目标发送了消息，
// 则原始通道的回复应被抑制（避免重复投递）。
func ShouldSuppressMessagingToolReplies(params ShouldSuppressMessagingToolRepliesParams) bool {
	provider := strings.TrimSpace(strings.ToLower(params.MessageProvider))
	if provider == "" {
		return false
	}
	originTarget := normalizeTargetForSuppression(provider, params.OriginatingTo)
	if originTarget == "" {
		return false
	}
	originAccount := normalizeAccountID(params.AccountID)

	if len(params.MessagingToolSentTargets) == 0 {
		return false
	}

	for _, target := range params.MessagingToolSentTargets {
		if strings.TrimSpace(strings.ToLower(target.Provider)) != provider {
			continue
		}
		targetKey := normalizeTargetForSuppression(provider, target.To)
		if targetKey == "" {
			continue
		}
		targetAccount := normalizeAccountID(target.AccountID)
		if originAccount != "" && targetAccount != "" && originAccount != targetAccount {
			continue
		}
		if targetKey == originTarget {
			return true
		}
	}
	return false
}

// normalizeAccountID 规范化 account ID（小写+trim）。
func normalizeAccountID(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return strings.ToLower(trimmed)
}

// normalizeTargetForSuppression 规范化 target 标识符用于抑制判断。
// TS 对照: infra/outbound/target-normalization.ts normalizeTargetForProvider
func normalizeTargetForSuppression(provider, to string) string {
	trimmed := strings.TrimSpace(to)
	if trimmed == "" {
		return ""
	}
	return strings.ToLower(trimmed)
}
