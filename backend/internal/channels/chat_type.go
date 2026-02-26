package channels

// ChatType 聊天类型 — 继承自 src/channels/chat-type.ts
type ChatType string

const (
	ChatTypeDirect  ChatType = "direct"
	ChatTypeGroup   ChatType = "group"
	ChatTypeChannel ChatType = "channel"
)

// NormalizeChatType 对原始字符串进行规范化，匹配 TS 中 "dm" → "direct" 的映射
func NormalizeChatType(raw string) ChatType {
	switch trimLower(raw) {
	case "direct", "dm":
		return ChatTypeDirect
	case "group":
		return ChatTypeGroup
	case "channel":
		return ChatTypeChannel
	default:
		return ""
	}
}

// trimLower 去除首尾空白并转小写
func trimLower(s string) string {
	// 避免 import strings 依赖——手动实现简单的 trim+lower
	start, end := 0, len(s)
	for start < end && s[start] == ' ' {
		start++
	}
	for end > start && s[end-1] == ' ' {
		end--
	}
	b := make([]byte, end-start)
	for i := start; i < end; i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i-start] = c
	}
	return string(b)
}
