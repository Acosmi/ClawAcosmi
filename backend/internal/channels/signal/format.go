package signal

import (
	"regexp"
	"strings"
	"unicode/utf16"

	"github.com/Acosmi/ClawAcosmi/pkg/markdown"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// Markdown → Signal 文本样式转换 — 继承自 src/signal/format.ts (239L)

// SignalTextStyle Signal 支持的文本样式名
type SignalTextStyle string

const (
	StyleBold          SignalTextStyle = "BOLD"
	StyleItalic        SignalTextStyle = "ITALIC"
	StyleStrikethrough SignalTextStyle = "STRIKETHROUGH"
	StyleMonospace     SignalTextStyle = "MONOSPACE"
	StyleSpoiler       SignalTextStyle = "SPOILER"
)

// SignalTextStyleRange 文本样式区间（start/length 按 UTF-16 编码计算）
type SignalTextStyleRange struct {
	Start  int             `json:"start"`
	Length int             `json:"length"`
	Style  SignalTextStyle `json:"style"`
}

// SignalFormattedText 格式化后的 Signal 文本
type SignalFormattedText struct {
	Text       string                 `json:"text"`
	TextStyles []SignalTextStyleRange `json:"textStyles,omitempty"`
}

// markdownStylePattern 匹配模式到 Signal 样式的映射
type markdownStylePattern struct {
	re    *regexp.Regexp
	style SignalTextStyle
}

// markdownLinkRE 匹配 Markdown 链接 [label](url)
var markdownLinkRE = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)

// expandMarkdownLinks 将 Markdown 链接扩展为 "label (url)" 格式。
// 对齐 TS renderSignalText: 如果 label 为空则直接用 url；
// 如果 label == url 或 label == url(去掉 mailto:)，不追加 url。
func expandMarkdownLinks(text string) string {
	return markdownLinkRE.ReplaceAllStringFunc(text, func(match string) string {
		sub := markdownLinkRE.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		label := strings.TrimSpace(sub[1])
		href := strings.TrimSpace(sub[2])
		if href == "" {
			return label
		}
		if label == "" {
			return href
		}
		comparable := href
		if strings.HasPrefix(comparable, "mailto:") {
			comparable = comparable[len("mailto:"):]
		}
		if label == href || label == comparable {
			return label
		}
		return label + " (" + href + ")"
	})
}

var patterns = []markdownStylePattern{
	// 粗体 **text** 或 __text__
	{re: regexp.MustCompile(`\*\*(.+?)\*\*`), style: StyleBold},
	{re: regexp.MustCompile(`__(.+?)__`), style: StyleBold},
	// 斜体 *text* 或 _text_（排除已匹配的 **）
	{re: regexp.MustCompile(`(?:^|[^*])\*([^*]+?)\*(?:[^*]|$)`), style: StyleItalic},
	{re: regexp.MustCompile(`(?:^|[^_])_([^_]+?)_(?:[^_]|$)`), style: StyleItalic},
	// 删除线 ~~text~~
	{re: regexp.MustCompile(`~~(.+?)~~`), style: StyleStrikethrough},
	// 代码 `text`
	{re: regexp.MustCompile("`([^`]+)`"), style: StyleMonospace},
	// 剧透 ||text||
	{re: regexp.MustCompile(`\|\|(.+?)\|\|`), style: StyleSpoiler},
}

// utf16Len 计算字符串的 UTF-16 长度
func utf16Len(s string) int {
	return len(utf16.Encode([]rune(s)))
}

// utf16Index 将字节偏移转换为 UTF-16 偏移
func utf16Index(s string, byteOffset int) int {
	if byteOffset <= 0 {
		return 0
	}
	if byteOffset >= len(s) {
		return utf16Len(s)
	}
	prefix := s[:byteOffset]
	return utf16Len(prefix)
}

// SignalFormatOpts 格式化选项
type SignalFormatOpts struct {
	TableMode types.MarkdownTableMode
}

// ResolveSignalTableMode 从 config 解析 Signal 表格模式。
// TS 对照: config/markdown-tables.ts resolveMarkdownTableMode({cfg, channel: "signal", accountId})
func ResolveSignalTableMode(cfg *types.OpenAcosmiConfig, accountID string) types.MarkdownTableMode {
	account := ResolveSignalAccount(cfg, accountID)
	if account.Config.Markdown != nil && account.Config.Markdown.Tables != "" {
		return account.Config.Markdown.Tables
	}
	if cfg != nil && cfg.Markdown != nil && cfg.Markdown.Tables != "" {
		return cfg.Markdown.Tables
	}
	return ""
}

// MarkdownToSignalText 将 Markdown 文本转换为 Signal 格式化文本
// 处理流程：表格预转换 → 移除标记字符 → 计算 UTF-16 范围的样式区间
func MarkdownToSignalText(md string, opts ...SignalFormatOpts) SignalFormattedText {
	if md == "" {
		return SignalFormattedText{Text: md}
	}

	// 对齐 TS: 表格预转换
	text := md
	if len(opts) > 0 && opts[0].TableMode != "" && opts[0].TableMode != types.MarkdownTableOff {
		text = markdown.ConvertMarkdownTables(text, markdown.TableMode(opts[0].TableMode))
	}
	// 对齐 TS renderSignalText: 链接扩展 [label](url) → label (url)
	text = expandMarkdownLinks(text)
	md = text

	type pendingStyle struct {
		byteStart int // 内容在原文中的字节偏移
		byteEnd   int
		style     SignalTextStyle
		content   string
		fullMatch string
	}

	// 收集所有匹配
	var allMatches []pendingStyle
	for _, p := range patterns {
		matches := p.re.FindAllStringSubmatchIndex(md, -1)
		for _, loc := range matches {
			if len(loc) < 4 {
				continue
			}
			matchStart := loc[0]
			matchEnd := loc[1]
			contentStart := loc[2]
			contentEnd := loc[3]
			allMatches = append(allMatches, pendingStyle{
				byteStart: contentStart,
				byteEnd:   contentEnd,
				style:     p.style,
				content:   md[contentStart:contentEnd],
				fullMatch: md[matchStart:matchEnd],
			})
		}
	}

	if len(allMatches) == 0 {
		return SignalFormattedText{Text: md}
	}

	// 按匹配位置排序（优先处理靠前的匹配）
	for i := 0; i < len(allMatches); i++ {
		for j := i + 1; j < len(allMatches); j++ {
			if allMatches[j].byteStart < allMatches[i].byteStart {
				allMatches[i], allMatches[j] = allMatches[j], allMatches[i]
			}
		}
	}

	// 去除重叠（后面的匹配在前面匹配内部时跳过）
	var filtered []pendingStyle
	lastEnd := -1
	for _, m := range allMatches {
		if m.byteStart >= lastEnd {
			filtered = append(filtered, m)
			lastEnd = m.byteEnd
		}
	}

	// 构建纯文本 + 样式区间
	var result strings.Builder
	var styles []SignalTextStyleRange
	cursor := 0
	for _, m := range filtered {
		// 添加标记前的纯文本
		if m.byteStart > cursor {
			// 需要找到 fullMatch 在原文中的起始位置
			matchIdx := strings.Index(md[cursor:], m.fullMatch)
			if matchIdx >= 0 {
				result.WriteString(md[cursor : cursor+matchIdx])
				cursor = cursor + matchIdx
			}
		}

		// 记录当前输出位置（UTF-16）
		start := utf16Len(result.String())
		result.WriteString(m.content)
		length := utf16Len(m.content)

		styles = append(styles, SignalTextStyleRange{
			Start:  start,
			Length: length,
			Style:  m.style,
		})

		cursor += len(m.fullMatch)
	}

	// 添加剩余文本
	if cursor < len(md) {
		result.WriteString(md[cursor:])
	}

	return SignalFormattedText{
		Text:       result.String(),
		TextStyles: styles,
	}
}

// MarkdownToSignalTextChunks 将 Markdown 分块转换为 Signal 格式化文本
// splitLimit 为每块的最大 UTF-16 长度，0 或负数表示不分块
func MarkdownToSignalTextChunks(md string, splitLimit int) []SignalFormattedText {
	ft := MarkdownToSignalText(md)
	if splitLimit <= 0 || utf16Len(ft.Text) <= splitLimit {
		return []SignalFormattedText{ft}
	}

	// 按行分块
	lines := strings.Split(ft.Text, "\n")
	var chunks []SignalFormattedText
	var currentLines []string
	currentLen := 0

	for _, line := range lines {
		lineLen := utf16Len(line)
		newLen := currentLen + lineLen
		if len(currentLines) > 0 {
			newLen++ // \n 分隔符
		}
		if newLen > splitLimit && len(currentLines) > 0 {
			chunkText := strings.Join(currentLines, "\n")
			chunks = append(chunks, MarkdownToSignalText(chunkText))
			currentLines = nil
			currentLen = 0
		}
		currentLines = append(currentLines, line)
		currentLen = currentLen + lineLen
		if len(currentLines) > 1 {
			currentLen++
		}
	}
	if len(currentLines) > 0 {
		chunkText := strings.Join(currentLines, "\n")
		chunks = append(chunks, MarkdownToSignalText(chunkText))
	}
	return chunks
}
