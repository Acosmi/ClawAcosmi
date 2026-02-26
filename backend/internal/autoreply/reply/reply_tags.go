package reply

// TS 对照: auto-reply/reply/reply-tags.ts (23L)
//
// 从文本中提取 [[reply:...]] 标签。
// reply-tags.ts 的 extractReplyToTag 是 utils/directive-tags.ts parseInlineDirectives 的薄封装。
// Go 端复用 streaming_directives.go 中的 extractStreamingReplyTags。

// ExtractReplyToTagResult 提取 reply-to 标签的结果。
// TS 对照: reply-tags.ts extractReplyToTag 返回值 (L5-10)
type ExtractReplyToTagResult struct {
	Cleaned        string
	ReplyToID      string
	ReplyToCurrent bool
	HasTag         bool
}

// ExtractReplyToTag 从文本中提取 [[reply:...]] 标签。
// TS 对照: reply-tags.ts extractReplyToTag (L3-22)
//
// 解析 [[reply:ID]] 或 [[reply:current]] 标签：
//   - [[reply:current]] → ReplyToCurrent=true
//   - [[reply:SOME_ID]] → ReplyToID="SOME_ID"
//   - 标签文本从 Cleaned 中剥离
func ExtractReplyToTag(text string, currentMessageID string) ExtractReplyToTagResult {
	if text == "" {
		return ExtractReplyToTagResult{Cleaned: ""}
	}

	replyToID, _, replyToCurrent, hasTag, cleaned := extractStreamingReplyTags(text)

	// 与 TS 一致：如果 replyToCurrent 为 true 且 currentMessageID 非空，
	// 则将 replyToID 设置为 currentMessageID
	if replyToCurrent && currentMessageID != "" && replyToID == "" {
		replyToID = currentMessageID
	}

	return ExtractReplyToTagResult{
		Cleaned:        cleaned,
		ReplyToID:      replyToID,
		ReplyToCurrent: replyToCurrent,
		HasTag:         hasTag,
	}
}
