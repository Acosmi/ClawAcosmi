package reply

import (
	"strings"

	"github.com/anthropic/open-acosmi/internal/autoreply"
	"github.com/anthropic/open-acosmi/pkg/markdown"
)

// TS 对照: auto-reply/reply/ 中分散的 response-body 逻辑。
// 完整指令处理链（skill-commands、directive-handling 等）延迟到 Phase 8。

// ThinkingTagPattern 匹配 <thinking>...</thinking> 标签。
var thinkingTagPattern = strings.NewReplacer(
	"<thinking>", "",
	"</thinking>", "",
)

// StripThinkingTags 剥离 thinking 标签。
// TS 对照: reply/ 中多处使用的 thinking 标签处理。
func StripThinkingTags(text string) string {
	return thinkingTagPattern.Replace(text)
}

// BuildResponseBodyOptions 回复体构建选项。
type BuildResponseBodyOptions struct {
	// StripThinking 是否剥离 thinking 标签。
	StripThinking bool
	// ChunkLimit 分块大小限制。0 = 使用默认值。
	ChunkLimit int
	// ChunkMode 分块模式。
	ChunkMode autoreply.ChunkMode
	// ResponsePrefix 响应前缀。
	ResponsePrefix string
	// ResponsePrefixContext 响应前缀模板上下文。
	ResponsePrefixContext *ResponsePrefixContext
}

// BuildResponseBody 构建回复体。
// 1. 可选剥离 thinking 标签
// 2. 应用响应前缀
// 3. 按指定模式分块
//
// TS 对照: reply/ 中分散的后处理逻辑整合。
func BuildResponseBody(text string, opts *BuildResponseBodyOptions) []string {
	if text == "" {
		return nil
	}
	if opts == nil {
		opts = &BuildResponseBodyOptions{}
	}

	processed := text
	if opts.StripThinking {
		processed = StripThinkingTags(processed)
	}

	processed = strings.TrimSpace(processed)
	if processed == "" {
		return nil
	}

	// 应用响应前缀
	if opts.ResponsePrefix != "" {
		processed = ApplyResponsePrefix(processed, opts.ResponsePrefix, opts.ResponsePrefixContext)
	}

	// 分块
	limit := opts.ChunkLimit
	if limit <= 0 {
		limit = autoreply.DefaultChunkLimit
	}

	return autoreply.ChunkMarkdownTextWithMode(processed, limit, opts.ChunkMode)
}

// BuildResponseBodyIR 构建回复体并返回 IR 分块。
// 用于需要样式/链接信息的输出渠道（如 Discord、Slack）。
// TS 对照: ir.ts chunkMarkdownIR 的调用方。
func BuildResponseBodyIR(text string, opts *BuildResponseBodyOptions) []markdown.MarkdownIR {
	if text == "" {
		return nil
	}
	if opts == nil {
		opts = &BuildResponseBodyOptions{}
	}

	processed := text
	if opts.StripThinking {
		processed = StripThinkingTags(processed)
	}

	processed = strings.TrimSpace(processed)
	if processed == "" {
		return nil
	}

	if opts.ResponsePrefix != "" {
		processed = ApplyResponsePrefix(processed, opts.ResponsePrefix, opts.ResponsePrefixContext)
	}

	limit := opts.ChunkLimit
	if limit <= 0 {
		limit = autoreply.DefaultChunkLimit
	}

	ir := markdown.MarkdownToIR(processed, nil)
	return markdown.ChunkMarkdownIR(ir, limit, autoreply.ChunkMarkdownText)
}
