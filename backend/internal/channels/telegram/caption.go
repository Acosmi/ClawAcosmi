package telegram

import "strings"

// TelegramMaxCaptionLength 是 Telegram 标题的最大长度限制。
const TelegramMaxCaptionLength = 1024

// CaptionSplit 表示标题分割结果。
type CaptionSplit struct {
	Caption     string // 适合作为媒体标题的文本（空表示不用标题）
	FollowUp    string // 超长文本，需作为后续消息发送
	HasCaption  bool
	HasFollowUp bool
}

// SplitTelegramCaption 将文本分割为媒体标题和后续消息。
// 如果文本超过 TelegramMaxCaptionLength，全部作为后续消息发送。
func SplitTelegramCaption(text string) CaptionSplit {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return CaptionSplit{}
	}
	if len([]rune(trimmed)) > TelegramMaxCaptionLength {
		return CaptionSplit{FollowUp: trimmed, HasFollowUp: true}
	}
	return CaptionSplit{Caption: trimmed, HasCaption: true}
}
