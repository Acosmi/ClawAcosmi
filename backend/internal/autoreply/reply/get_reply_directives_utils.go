package reply

// TS 对照: auto-reply/reply/get-reply-directives-utils.ts (48L)

// ClearInlineDirectives 清除所有内联指令，保留 cleaned 文本。
// TS 对照: get-reply-directives-utils.ts L3-47
func ClearInlineDirectives(cleaned string) InlineDirectives {
	return InlineDirectives{
		Cleaned: cleaned,
		// 所有指令标志默认 false / nil
	}
}
