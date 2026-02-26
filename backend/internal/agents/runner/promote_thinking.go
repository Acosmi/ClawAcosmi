package runner

// ============================================================================
// promoteThinkingTagsToBlocks — 文本 <thinking> 标签 → 结构化 thinking content blocks
// TS 对照: pi-embedded-utils.ts → promoteThinkingTagsToBlocks() + splitThinkingTaggedText()
// ============================================================================

import (
	"regexp"
	"strings"

	"github.com/anthropic/open-acosmi/internal/agents/llmclient"
)

// ThinkTaggedBlock splitThinkingTaggedText 的拆分结果。
type ThinkTaggedBlock struct {
	Type     string // "text" | "thinking"
	Text     string // type=text 时
	Thinking string // type=thinking 时
}

// promoteThinkOpenRE 检测开标签（支持多种变体）。
var promoteThinkOpenRE = regexp.MustCompile(`(?i)<\s*(?:think(?:ing)?|thought|antthinking)\s*>`)

// promoteThinkCloseRE 检测闭标签。
var promoteThinkCloseRE = regexp.MustCompile(`(?i)<\s*/\s*(?:think(?:ing)?|thought|antthinking)\s*>`)

// promoteThinkScanRE 扫描所有开/闭标签。
var promoteThinkScanRE = regexp.MustCompile(`(?i)<\s*(/?)\s*(?:think(?:ing)?|thought|antthinking)\s*>`)

// SplitThinkingTaggedText 将含有 <thinking>/<think>/<thought>/<antthinking> 标签的文本
// 拆分为 text + thinking 块。如果文本不含有效标签对，返回 nil。
// TS 对照: pi-embedded-utils.ts → splitThinkingTaggedText()
func SplitThinkingTaggedText(text string) []ThinkTaggedBlock {
	trimmedStart := strings.TrimLeft(text, " \t\n\r")

	// 只处理以 < 开头的文本
	if !strings.HasPrefix(trimmedStart, "<") {
		return nil
	}

	// 必须同时存在开标签和闭标签
	if !promoteThinkOpenRE.MatchString(trimmedStart) {
		return nil
	}
	if !promoteThinkCloseRE.MatchString(text) {
		return nil
	}

	matches := promoteThinkScanRE.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return nil
	}

	var (
		blocks        []ThinkTaggedBlock
		inThinking    bool
		cursor        int
		thinkingStart int
	)

	pushText := func(value string) {
		if value == "" {
			return
		}
		blocks = append(blocks, ThinkTaggedBlock{Type: "text", Text: value})
	}

	pushThinking := func(value string) {
		cleaned := strings.TrimSpace(value)
		if cleaned == "" {
			return
		}
		blocks = append(blocks, ThinkTaggedBlock{Type: "thinking", Thinking: cleaned})
	}

	for _, match := range matches {
		// match[0..1]: 完整匹配范围, match[2..3]: 第一个捕获组 (/ or "")
		fullStart, fullEnd := match[0], match[1]
		slashGroup := text[match[2]:match[3]]
		isClose := strings.Contains(slashGroup, "/")

		if !inThinking && !isClose {
			// 开标签: 将之前的文本推入
			pushText(text[cursor:fullStart])
			thinkingStart = fullEnd
			inThinking = true
			continue
		}

		if inThinking && isClose {
			// 闭标签: 将 thinking 内容推入
			pushThinking(text[thinkingStart:fullStart])
			cursor = fullEnd
			inThinking = false
		}
	}

	// 未闭合的 thinking 块 → 无效
	if inThinking {
		return nil
	}

	// 推入尾部文本
	pushText(text[cursor:])

	// 至少包含一个 thinking 块才有效
	hasThinking := false
	for _, b := range blocks {
		if b.Type == "thinking" {
			hasThinking = true
			break
		}
	}
	if !hasThinking {
		return nil
	}

	return blocks
}

// PromoteThinkingTagsToBlocks 遍历 content blocks，将含有 <thinking> 标签的 text block
// 拆分为结构化的 thinking + text blocks。返回新的 content 列表和是否有变更。
// TS 对照: pi-embedded-utils.ts → promoteThinkingTagsToBlocks()
func PromoteThinkingTagsToBlocks(content []llmclient.ContentBlock) ([]llmclient.ContentBlock, bool) {
	if len(content) == 0 {
		return content, false
	}

	// AUDIT-7: TS L334-336 — 已有 thinking block 则跳过
	for _, block := range content {
		if block.Type == "thinking" {
			return content, false
		}
	}

	changed := false
	var next []llmclient.ContentBlock

	for _, block := range content {
		if block.Type != "text" || block.Text == "" {
			next = append(next, block)
			continue
		}

		splits := SplitThinkingTaggedText(block.Text)
		if splits == nil {
			next = append(next, block)
			continue
		}

		changed = true
		for _, s := range splits {
			switch s.Type {
			case "thinking":
				next = append(next, llmclient.ContentBlock{
					Type:     "thinking",
					Thinking: s.Thinking,
				})
			case "text":
				// AUDIT-7: TS L357 — trimStart
				cleaned := strings.TrimLeft(s.Text, " \t\n\r")
				if cleaned != "" {
					next = append(next, llmclient.ContentBlock{
						Type: "text",
						Text: cleaned,
					})
				}
			}
		}
	}

	if !changed {
		return content, false
	}
	return next, true
}
