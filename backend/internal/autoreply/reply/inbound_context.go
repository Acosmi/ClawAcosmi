package reply

import (
	"regexp"
	"strings"

	"github.com/openacosmi/claw-acismi/internal/autoreply"
)

// TS 对照: auto-reply/reply/inbound-context.ts (82L)
// + auto-reply/reply/inbound-text.ts
// + auto-reply/reply/inbound-sender-meta.ts

// 换行符规范化正则
var (
	carriageReturnRe = regexp.MustCompile(`\r\n`)
	multiNewlineRe   = regexp.MustCompile(`\n{3,}`)
)

// NormalizeInboundTextNewlines 规范化入站文本换行。
// TS 对照: reply/inbound-text.ts
func NormalizeInboundTextNewlines(text string) string {
	if text == "" {
		return text
	}
	// \r\n → \n
	result := carriageReturnRe.ReplaceAllString(text, "\n")
	// 3+ 连续换行 → 2 换行
	result = multiNewlineRe.ReplaceAllString(result, "\n\n")
	return result
}

// normalizeTextField 规范化文本字段。
// TS 对照: inbound-context.ts L14-19
func normalizeTextField(value string) string {
	if value == "" {
		return ""
	}
	return NormalizeInboundTextNewlines(value)
}

// FormatInboundBodyWithSenderMeta 在入站消息体中注入发送者元信息。
// TS 对照: reply/inbound-sender-meta.ts
func FormatInboundBodyWithSenderMeta(ctx *autoreply.MsgContext, body string) string {
	if ctx == nil || body == "" {
		return body
	}
	// 仅对群组消息注入发送者标签
	if !ctx.IsGroup {
		return body
	}
	senderLabel := ctx.SenderDisplayName
	if senderLabel == "" {
		senderLabel = ctx.SenderName
	}
	if senderLabel == "" {
		return body
	}
	// 已有发送者标签时跳过
	prefix := senderLabel + ": "
	if strings.HasPrefix(body, prefix) {
		return body
	}
	return prefix + body
}

// FinalizeInboundContext 最终化入站上下文。
// 规范化所有文本字段、解析聊天类型和会话标签。
// TS 对照: reply/inbound-context.ts L21-81
func FinalizeInboundContext(ctx *autoreply.MsgContext, opts *FinalizeInboundContextOptions) {
	if ctx == nil {
		return
	}
	if opts == nil {
		opts = &FinalizeInboundContextOptions{}
	}

	// 规范化 Body
	ctx.Body = NormalizeInboundTextNewlines(ctx.Body)
	ctx.RawBody = normalizeTextField(ctx.RawBody)
	ctx.CommandBody = normalizeTextField(ctx.CommandBody)
	ctx.Transcript = normalizeTextField(ctx.Transcript)
	ctx.ThreadStarterBody = normalizeTextField(ctx.ThreadStarterBody)

	// 规范化 UntrustedContext
	if len(ctx.UntrustedContext) > 0 {
		var normalized []string
		for _, entry := range ctx.UntrustedContext {
			n := NormalizeInboundTextNewlines(entry)
			if n != "" {
				normalized = append(normalized, n)
			}
		}
		ctx.UntrustedContext = normalized
	}

	// ChatType 规范化
	// TS 对照: channels/chat-type.ts normalizeChatType()
	chatType := NormalizeChatType(ctx.ChatType)
	if chatType != "" && (opts.ForceChatType || ctx.ChatType != chatType) {
		ctx.ChatType = chatType
	}

	// BodyForAgent
	if opts.ForceBodyForAgent {
		ctx.BodyForAgent = NormalizeInboundTextNewlines(ctx.Body)
	} else if ctx.BodyForAgent == "" {
		ctx.BodyForAgent = NormalizeInboundTextNewlines(ctx.Body)
	} else {
		ctx.BodyForAgent = NormalizeInboundTextNewlines(ctx.BodyForAgent)
	}

	// BodyForCommands
	if opts.ForceBodyForCommands {
		source := ctx.CommandBody
		if source == "" {
			source = ctx.RawBody
		}
		if source == "" {
			source = ctx.Body
		}
		ctx.BodyForCommands = NormalizeInboundTextNewlines(source)
	} else if ctx.BodyForCommands == "" {
		source := ctx.CommandBody
		if source == "" {
			source = ctx.RawBody
		}
		if source == "" {
			source = ctx.Body
		}
		ctx.BodyForCommands = NormalizeInboundTextNewlines(source)
	}

	// ConversationLabel 解析
	// TS 对照: channels/conversation-label.ts resolveConversationLabel()
	explicitLabel := strings.TrimSpace(ctx.ConversationLabel)
	if opts.ForceConversationLabel || explicitLabel == "" {
		resolved := strings.TrimSpace(ResolveConversationLabel(ctx))
		if resolved != "" {
			ctx.ConversationLabel = resolved
		}
	} else {
		ctx.ConversationLabel = explicitLabel
	}

	// 注入发送者元信息到 Body 和 BodyForAgent
	ctx.Body = FormatInboundBodyWithSenderMeta(ctx, ctx.Body)
	ctx.BodyForAgent = FormatInboundBodyWithSenderMeta(ctx, ctx.BodyForAgent)

	// CommandAuthorized 默认 false (Go bool 零值)
}

// NormalizeChatType 规范化聊天类型。
// TS 对照: channels/chat-type.ts normalizeChatType()
func NormalizeChatType(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return ""
	}
	switch value {
	case "direct", "dm":
		return "direct"
	case "group":
		return "group"
	case "channel":
		return "channel"
	default:
		return ""
	}
}

// ResolveConversationLabel 推断会话标签。
// TS 对照: channels/conversation-label.ts resolveConversationLabel()
func ResolveConversationLabel(ctx *autoreply.MsgContext) string {
	if ctx == nil {
		return ""
	}

	// 1. 已有显式标签
	explicit := strings.TrimSpace(ctx.ConversationLabel)
	if explicit != "" {
		return explicit
	}

	// 2. 线程标签
	threadLabel := strings.TrimSpace(ctx.ThreadLabel)
	if threadLabel != "" {
		return threadLabel
	}

	// 3. 按聊天类型推断
	chatType := NormalizeChatType(ctx.ChatType)
	if chatType == "direct" {
		name := strings.TrimSpace(ctx.SenderName)
		if name != "" {
			return name
		}
		from := strings.TrimSpace(ctx.From)
		if from != "" {
			return from
		}
		return ""
	}

	// 群组/频道: 使用 GroupChannel > GroupSubject > From
	base := strings.TrimSpace(ctx.GroupChannel)
	if base == "" {
		base = strings.TrimSpace(ctx.GroupSubject)
	}
	if base == "" {
		base = strings.TrimSpace(ctx.From)
	}
	if base == "" {
		return ""
	}

	// 追加 ID（仅纯数字或 WhatsApp 群组 ID）
	id := extractConversationID(ctx.From)
	if id == "" || id == base || strings.Contains(base, id) {
		return base
	}
	if shouldAppendID(id) {
		return base + " id:" + id
	}
	return base
}

// extractConversationID 从 From 字段提取会话 ID。
func extractConversationID(from string) string {
	trimmed := strings.TrimSpace(from)
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, ":")
	for i := len(parts) - 1; i >= 0; i-- {
		p := strings.TrimSpace(parts[i])
		if p != "" {
			return p
		}
	}
	return trimmed
}

// shouldAppendID 判断是否应追加 ID 到标签。
func shouldAppendID(id string) bool {
	// 纯数字 ID
	for _, r := range id {
		if r < '0' || r > '9' {
			// 检查 WhatsApp 群组 ID
			return strings.Contains(id, "@g.us")
		}
	}
	return true
}
