package channels

import (
	"regexp"
	"strings"
)

// 对话标签解析 — 继承自 src/channels/conversation-label.ts (70 行)

// MsgContext 消息上下文（所需字段子集）
// 完整定义在 auto-reply/templating 中，此处仅声明所需字段避免循环依赖
type MsgContext struct {
	ConversationLabel string
	ThreadLabel       string
	ChatType          string
	SenderName        string
	From              string
	GroupChannel      string
	GroupSubject      string
	GroupSpace        string
}

var (
	numericRe = regexp.MustCompile(`^[0-9]+$`)
)

// extractConversationID 从 From 字段提取对话 ID（冒号分隔取最后一段）
func extractConversationID(from string) string {
	trimmed := strings.TrimSpace(from)
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, ":")
	var nonEmpty []string
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	if len(nonEmpty) == 0 {
		return trimmed
	}
	return nonEmpty[len(nonEmpty)-1]
}

// shouldAppendID 判断是否应追加 ID 到标签
func shouldAppendID(id string) bool {
	if numericRe.MatchString(id) {
		return true
	}
	if strings.Contains(id, "@g.us") {
		return true
	}
	return false
}

// ResolveConversationLabel 解析对话标签
func ResolveConversationLabel(ctx MsgContext) string {
	// 优先使用显式标签
	if label := strings.TrimSpace(ctx.ConversationLabel); label != "" {
		return label
	}
	if label := strings.TrimSpace(ctx.ThreadLabel); label != "" {
		return label
	}

	// 直接消息：使用发送者名称
	chatType := NormalizeChatType(ctx.ChatType)
	if chatType == ChatTypeDirect {
		if name := strings.TrimSpace(ctx.SenderName); name != "" {
			return name
		}
		if from := strings.TrimSpace(ctx.From); from != "" {
			return from
		}
		return ""
	}

	// 群组/频道：按优先级取基础名称
	base := ""
	for _, candidate := range []string{ctx.GroupChannel, ctx.GroupSubject, ctx.GroupSpace, ctx.From} {
		if v := strings.TrimSpace(candidate); v != "" {
			base = v
			break
		}
	}
	if base == "" {
		return ""
	}

	// 条件追加 ID
	id := extractConversationID(ctx.From)
	if id == "" || !shouldAppendID(id) || base == id || strings.Contains(base, id) {
		return base
	}
	if strings.Contains(strings.ToLower(base), " id:") {
		return base
	}
	if strings.HasPrefix(base, "#") || strings.HasPrefix(base, "@") {
		return base
	}
	return base + " id:" + id
}
