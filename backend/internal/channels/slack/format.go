package slack

import (
	"regexp"
	"strings"

	"github.com/openacosmi/claw-acismi/internal/autoreply"
	"github.com/openacosmi/claw-acismi/pkg/markdown"
)

// Slack mrkdwn 格式化 — 继承自 src/slack/format.ts (147L)
// 使用 markdown IR 中间表示层 + renderMarkdownWithMarkers 实现完整转换。

var slackAngleTokenRe = regexp.MustCompile(`<[^>\n]+>`)

// escapeSlackMrkdwnSegment 转义 Slack mrkdwn 特殊字符。
func escapeSlackMrkdwnSegment(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	return text
}

// isAllowedSlackAngleToken 检查 angle-bracket token 是否为 Slack 保留格式。
func isAllowedSlackAngleToken(token string) bool {
	if !strings.HasPrefix(token, "<") || !strings.HasSuffix(token, ">") {
		return false
	}
	inner := token[1 : len(token)-1]
	return strings.HasPrefix(inner, "@") ||
		strings.HasPrefix(inner, "#") ||
		strings.HasPrefix(inner, "!") ||
		strings.HasPrefix(inner, "mailto:") ||
		strings.HasPrefix(inner, "tel:") ||
		strings.HasPrefix(inner, "http://") ||
		strings.HasPrefix(inner, "https://") ||
		strings.HasPrefix(inner, "slack://")
}

// escapeSlackMrkdwnContent 转义一行内容中的特殊字符，但保留 Slack angle-bracket token。
func escapeSlackMrkdwnContent(text string) string {
	if !strings.ContainsAny(text, "&<>") {
		return text
	}

	matches := slackAngleTokenRe.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return escapeSlackMrkdwnSegment(text)
	}

	var out strings.Builder
	lastIndex := 0
	for _, loc := range matches {
		// 转义 token 之前的文本
		out.WriteString(escapeSlackMrkdwnSegment(text[lastIndex:loc[0]]))
		// 检查 token 是否为保留格式
		token := text[loc[0]:loc[1]]
		if isAllowedSlackAngleToken(token) {
			out.WriteString(token)
		} else {
			out.WriteString(escapeSlackMrkdwnSegment(token))
		}
		lastIndex = loc[1]
	}
	out.WriteString(escapeSlackMrkdwnSegment(text[lastIndex:]))
	return out.String()
}

// slackMrkdwnStyleMarkers Slack mrkdwn 的样式标记映射。
// TS 对照: slack/format.ts slackMrkdwnRenderMap
var slackMrkdwnStyleMarkers = markdown.RenderStyleMap{
	markdown.StyleBold:          {Open: "*", Close: "*"},
	markdown.StyleItalic:        {Open: "_", Close: "_"},
	markdown.StyleStrikethrough: {Open: "~", Close: "~"},
	markdown.StyleCode:          {Open: "`", Close: "`"},
	markdown.StyleCodeBlock:     {Open: "```\n", Close: "\n```"},
}

// slackBuildLink 构建 Slack mrkdwn 链接 <url|label>。
func slackBuildLink(link markdown.MarkdownLinkSpan, text string) *markdown.RenderLink {
	label := text[link.Start:link.End]
	if link.Href == "" {
		return nil
	}
	_ = label // render 内部使用 text 段
	return &markdown.RenderLink{
		Start: link.Start,
		End:   link.End,
		Open:  "<" + link.Href + "|",
		Close: ">",
	}
}

// MarkdownToSlackMrkdwn 将 Markdown 文本转换为 Slack mrkdwn 格式。
// 使用 markdown IR 中间表示层 + renderMarkdownWithMarkers。
func MarkdownToSlackMrkdwn(md string) string {
	if md == "" {
		return ""
	}
	ir := markdown.MarkdownToIR(md, &markdown.MarkdownParseOptions{
		HeadingStyle:     "bold",
		BlockquotePrefix: "> ",
	})
	return markdown.RenderMarkdownWithMarkers(ir, markdown.RenderOptions{
		StyleMarkers: slackMrkdwnStyleMarkers,
		EscapeText:   escapeSlackMrkdwnContent,
		BuildLink:    slackBuildLink,
	})
}

// MarkdownToSlackMrkdwnChunks 将 Markdown 文本转换为限定长度的 Slack mrkdwn 分块。
// 使用 ChunkMarkdownIR 进行围栏感知分块，保留代码块完整性。
func MarkdownToSlackMrkdwnChunks(md string, limit int) []string {
	if limit <= 0 {
		limit = 4000 // Slack 默认文本长度限制
	}
	if md == "" {
		return nil
	}

	ir := markdown.MarkdownToIR(md, &markdown.MarkdownParseOptions{
		HeadingStyle:     "bold",
		BlockquotePrefix: "> ",
	})

	renderOpts := markdown.RenderOptions{
		StyleMarkers: slackMrkdwnStyleMarkers,
		EscapeText:   escapeSlackMrkdwnContent,
		BuildLink:    slackBuildLink,
	}

	// 单块优化
	rendered := markdown.RenderMarkdownWithMarkers(ir, renderOpts)
	if len(rendered) <= limit {
		return []string{rendered}
	}

	// IR 分块（围栏感知）
	irChunks := markdown.ChunkMarkdownIR(ir, limit, autoreply.ChunkMarkdownText)

	results := make([]string, 0, len(irChunks))
	for _, chunk := range irChunks {
		text := markdown.RenderMarkdownWithMarkers(chunk, renderOpts)
		if text != "" {
			results = append(results, text)
		}
	}
	if len(results) == 0 && rendered != "" {
		results = []string{rendered}
	}
	return results
}
