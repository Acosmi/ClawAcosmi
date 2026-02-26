package line

// TS 对照: src/line/markdown-to-line.ts (452L)
// Markdown → LINE Flex Message 转换

import (
	"regexp"
	"strings"
)

// ---------- 正则 ----------

var (
	reMarkdownTable     = regexp.MustCompile(`(?m)^\|(.+)\|[\r\n]+\|[-:\s|]+\|[\r\n]+((?:\|.+\|[\r\n]*)+)`)
	reMarkdownCodeBlock = regexp.MustCompile("(?s)```(\\w*)\\n([\\s\\S]*?)```")
	reMarkdownLink      = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	reBold              = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reItalic            = regexp.MustCompile(`\*(.+?)\*`)
	reStrikethrough     = regexp.MustCompile(`~~(.+?)~~`)
	reHeader            = regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)
	reBlockquote        = regexp.MustCompile(`(?m)^>\s*(.*)$`)
	reHR                = regexp.MustCompile(`(?m)^---+$`)
	reInlineCode        = regexp.MustCompile("`([^`]+)`")
)

// ---------- 处理结果 ----------

// ProcessedLineMessage 处理后的消息。
type ProcessedLineMessage struct {
	Text         string
	FlexMessages []FlexMessage
}

// ---------- Markdown 表格 ----------

// MarkdownTable 表格。
type MarkdownTable struct {
	Headers []string
	Rows    [][]string
}

func parseTableRow(row string) []string {
	row = strings.TrimSpace(row)
	row = strings.Trim(row, "|")
	parts := strings.Split(row, "|")
	result := make([]string, len(parts))
	for i, p := range parts {
		result[i] = strings.TrimSpace(p)
	}
	return result
}

func extractMarkdownTables(text string) ([]MarkdownTable, string) {
	var tables []MarkdownTable
	cleaned := reMarkdownTable.ReplaceAllStringFunc(text, func(match string) string {
		lines := strings.Split(strings.TrimSpace(match), "\n")
		if len(lines) < 3 {
			return match
		}
		headers := parseTableRow(lines[0])
		var rows [][]string
		for _, line := range lines[2:] {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			rows = append(rows, parseTableRow(line))
		}
		tables = append(tables, MarkdownTable{Headers: headers, Rows: rows})
		return ""
	})
	return tables, cleaned
}

func convertTableToFlexBubble(table MarkdownTable) FlexBubble {
	bubble := NewFlexBubble()

	// 表头
	headerContents := make([]FlexComponent, len(table.Headers))
	for i, h := range table.Headers {
		headerContents[i] = NewFlexText(h, "xs", "bold", "#1DB446")
	}
	headerBox := NewFlexBox("horizontal", headerContents...)
	headerBox.Spacing = "sm"
	bubble.Header = &headerBox

	// 数据行：修复列布局 — 将每行渲染为 horizontal FlexBox。
	bodyContents := make([]FlexComponent, 0, len(table.Rows)*2)
	for i, row := range table.Rows {
		if i > 0 {
			bodyContents = append(bodyContents, NewFlexSeparator())
		}
		rowContents := make([]FlexComponent, len(row))
		for j, cell := range row {
			// 检测粗体
			weight := "regular"
			cellText := cell
			if reBold.MatchString(cell) {
				weight = "bold"
				cellText = reBold.ReplaceAllString(cell, "$1")
			}
			col := NewFlexText(cellText, "xs", weight, "")
			col.Flex = 1 // 等宽列布局
			rowContents[j] = col
		}
		rowBox := NewFlexBox("horizontal", rowContents...)
		rowBox.Spacing = "sm"
		// 将 rowBox 的内容序列化为该行的组件（嵌入为 horizontal box）
		bodyContents = append(bodyContents, newFlexBoxComponent(rowBox))
	}
	bodyBox := NewFlexBox("vertical", bodyContents...)
	bodyBox.Spacing = "sm"
	bubble.Body = &bodyBox
	return bubble
}

// newFlexBoxComponent 将 FlexBox 转换为嵌入式 FlexComponent（type="box"）。
// 修复表格行列布局：rowBox 不再被丢弃，而是作为 box 组件加入 bodyContents。
func newFlexBoxComponent(box FlexBox) FlexComponent {
	return FlexComponent{
		Type:     "box",
		Layout:   box.Layout,
		Contents: box.Contents,
		Spacing:  box.Spacing,
	}
}

// ---------- 代码块 ----------

// CodeBlock 代码块。
type CodeBlock struct {
	Language string
	Code     string
}

func extractCodeBlocks(text string) ([]CodeBlock, string) {
	var blocks []CodeBlock
	cleaned := reMarkdownCodeBlock.ReplaceAllStringFunc(text, func(match string) string {
		subs := reMarkdownCodeBlock.FindStringSubmatch(match)
		if len(subs) < 3 {
			return match
		}
		blocks = append(blocks, CodeBlock{Language: subs[1], Code: subs[2]})
		return ""
	})
	return blocks, cleaned
}

func convertCodeBlockToFlexBubble(block CodeBlock) FlexBubble {
	bubble := NewFlexBubble()
	bubble.Size = "mega"

	// 标题
	lang := block.Language
	if lang == "" {
		lang = "code"
	}
	headerBox := NewFlexBox("vertical",
		NewFlexText("📝 "+lang, "sm", "bold", "#1DB446"),
	)
	bubble.Header = &headerBox

	// 代码内容
	code := block.Code
	if len(code) > 2000 {
		code = code[:2000] + "\n…(truncated)"
	}
	bodyBox := NewFlexBox("vertical",
		NewFlexText(code, "xxs", "regular", "#666666"),
	)
	bodyBox.Padding = "md"
	bubble.Body = &bodyBox
	return bubble
}

// ---------- 链接 ----------

// MarkdownLink 链接。
type MarkdownLink struct {
	Text string
	URL  string
}

func extractLinks(text string) ([]MarkdownLink, string) {
	var links []MarkdownLink
	cleaned := reMarkdownLink.ReplaceAllStringFunc(text, func(match string) string {
		subs := reMarkdownLink.FindStringSubmatch(match)
		if len(subs) < 3 {
			return match
		}
		links = append(links, MarkdownLink{Text: subs[1], URL: subs[2]})
		return subs[1] // 保留链接文字
	})
	return links, cleaned
}

func convertLinksToFlexBubble(links []MarkdownLink) FlexBubble {
	bubble := NewFlexBubble()
	contents := make([]FlexComponent, 0, len(links))
	for _, link := range links {
		contents = append(contents, NewFlexButton(link.Text, link.URL))
	}
	bodyBox := NewFlexBox("vertical", contents...)
	bodyBox.Spacing = "sm"
	bubble.Body = &bodyBox
	return bubble
}

// ---------- Markdown 清洗 ----------

// StripMarkdown 清除 markdown 标记。
func StripMarkdown(text string) string {
	text = reBold.ReplaceAllString(text, "$1")
	text = reItalic.ReplaceAllString(text, "$1")
	text = reStrikethrough.ReplaceAllString(text, "$1")
	text = reHeader.ReplaceAllString(text, "$1")
	text = reBlockquote.ReplaceAllString(text, "$1")
	text = reHR.ReplaceAllString(text, "")
	text = reInlineCode.ReplaceAllString(text, "$1")
	return strings.TrimSpace(text)
}

// ---------- 主函数 ----------

// ProcessLineMessage 将 markdown 文本转换为 LINE 格式。
// 提取表格、代码块为 Flex Message，剩余文本清除 markdown。
func ProcessLineMessage(text string) ProcessedLineMessage {
	var flexMessages []FlexMessage

	// 1. 提取表格
	tables, text := extractMarkdownTables(text)
	for _, table := range tables {
		bubble := convertTableToFlexBubble(table)
		flexMessages = append(flexMessages, ToFlexMessage("📊 Table", bubble))
	}

	// 2. 提取代码块
	codeBlocks, text := extractCodeBlocks(text)
	for _, block := range codeBlocks {
		bubble := convertCodeBlockToFlexBubble(block)
		flexMessages = append(flexMessages, ToFlexMessage("📝 Code", bubble))
	}

	// 3. 提取链接
	links, text := extractLinks(text)
	if len(links) > 0 {
		bubble := convertLinksToFlexBubble(links)
		flexMessages = append(flexMessages, ToFlexMessage("🔗 Links", bubble))
	}

	// 4. 清除剩余 markdown
	text = StripMarkdown(text)

	// 清理多余空行
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}

	return ProcessedLineMessage{
		Text:         strings.TrimSpace(text),
		FlexMessages: flexMessages,
	}
}

// HasMarkdownToConvert 检测文本是否包含需要转换的 markdown。
func HasMarkdownToConvert(text string) bool {
	return reMarkdownTable.MatchString(text) ||
		reMarkdownCodeBlock.MatchString(text) ||
		reMarkdownLink.MatchString(text) ||
		reBold.MatchString(text) ||
		reHeader.MatchString(text)
}
