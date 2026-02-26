// Package telegram 实现 Telegram 频道适配器。
// 继承自 src/telegram/ 的完整逻辑。
package telegram

import (
	"regexp"
	"strconv"
	"strings"
)

// TelegramTarget 表示解析后的 Telegram 投递目标。
type TelegramTarget struct {
	ChatID          string
	MessageThreadID *int // nil 表示无 topic/thread
}

var (
	telegramPrefixRe = regexp.MustCompile(`(?i)^(telegram|tg):`)
	groupPrefixRe    = regexp.MustCompile(`(?i)^group:`)
	topicFormatRe    = regexp.MustCompile(`^(.+?):topic:(\d+)$`)
	colonFormatRe    = regexp.MustCompile(`^(.+):(\d+)$`)
)

// StripTelegramInternalPrefixes 移除内部前缀（telegram:, tg:, group:）。
func StripTelegramInternalPrefixes(to string) string {
	trimmed := strings.TrimSpace(to)
	strippedTelegramPrefix := false
	for {
		next := trimmed
		if telegramPrefixRe.MatchString(trimmed) {
			strippedTelegramPrefix = true
			next = strings.TrimSpace(telegramPrefixRe.ReplaceAllString(trimmed, ""))
		} else if strippedTelegramPrefix && groupPrefixRe.MatchString(trimmed) {
			// Legacy: `telegram:group:<id>` — 仍由 session keys 产生
			next = strings.TrimSpace(groupPrefixRe.ReplaceAllString(trimmed, ""))
		}
		if next == trimmed {
			return trimmed
		}
		trimmed = next
	}
}

// ParseTelegramTarget 解析 Telegram 投递目标为 chatId + 可选 topicId。
//
// 支持格式：
//   - `chatId`（纯 chatId、t.me 链接、@username 或 telegram: 前缀）
//   - `chatId:topicId`（数字 topic/thread ID）
//   - `chatId:topic:topicId`（显式 topic 标记，推荐格式）
func ParseTelegramTarget(to string) TelegramTarget {
	normalized := StripTelegramInternalPrefixes(to)

	// 优先匹配 chatId:topic:topicId
	if m := topicFormatRe.FindStringSubmatch(normalized); m != nil {
		threadID, _ := strconv.Atoi(m[2])
		return TelegramTarget{ChatID: m[1], MessageThreadID: &threadID}
	}

	// 尝试 chatId:topicId
	if m := colonFormatRe.FindStringSubmatch(normalized); m != nil {
		threadID, _ := strconv.Atoi(m[2])
		return TelegramTarget{ChatID: m[1], MessageThreadID: &threadID}
	}

	return TelegramTarget{ChatID: normalized}
}
