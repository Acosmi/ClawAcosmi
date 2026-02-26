// Package markdown 提供 Markdown 内容处理工具。
//
// TS 对照: markdown/tables.ts + markdown/ir.ts (表格部分)
//
// 本模块实现 Markdown 表格转换，支持三种模式:
//   - off:      原样保留
//   - bullets:  转为项目列表
//   - code:     保留表格格式但对齐列宽
//
// 与 TS 版不同，本实现不依赖 markdown-it IR 管线，
// 直接使用正则在原始 Markdown 上解析表格。
package markdown

import (
	"regexp"
	"strings"
)

// TableMode 表格转换模式。
// TS 对照: config/types.base.ts → MarkdownTableMode
type TableMode string

const (
	// TableModeOff 原样保留表格。
	TableModeOff TableMode = "off"
	// TableModeBullets 将表格转为项目列表。
	TableModeBullets TableMode = "bullets"
	// TableModeCode 保留格式但对齐列宽（代码块样式）。
	TableModeCode TableMode = "code"
)

// 匹配 Markdown 表格行: | cell1 | cell2 | ...
var tableRowPattern = regexp.MustCompile(`^\s*\|(.+)\|\s*$`)

// 匹配分隔行: | --- | :---: | ---: | ...
var tableSepPattern = regexp.MustCompile(`^\s*\|(\s*:?-{1,}:?\s*\|)+\s*$`)

// parsedTable 解析后的表格结构。
type parsedTable struct {
	headers []string
	rows    [][]string
}

// parseTableRow 解析表格行的单元格内容。
func parseTableRow(line string) []string {
	m := tableRowPattern.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	raw := m[1]
	cells := strings.Split(raw, "|")
	result := make([]string, len(cells))
	for i, cell := range cells {
		result[i] = strings.TrimSpace(cell)
	}
	return result
}

// ConvertMarkdownTables 将 Markdown 中的表格按指定模式转换。
// TS 对照: markdown/tables.ts convertMarkdownTables()
func ConvertMarkdownTables(markdown string, mode TableMode) string {
	if markdown == "" || mode == TableModeOff {
		return markdown
	}

	lines := strings.Split(markdown, "\n")
	var result []string
	i := 0

	for i < len(lines) {
		// 检测表格起始: header行 + 分隔行
		table := tryParseTable(lines, i)
		if table == nil {
			result = append(result, lines[i])
			i++
			continue
		}

		// 计算跳过的行数
		tableLineCount := 1 + 1 + len(table.rows) // header + sep + data rows
		i += tableLineCount

		// 转换表格
		switch mode {
		case TableModeBullets:
			result = append(result, renderAsBullets(table)...)
		case TableModeCode:
			result = append(result, renderAsAlignedTable(table)...)
		default:
			// 未知模式，原样保留
			for j := i - tableLineCount; j < i; j++ {
				result = append(result, lines[j])
			}
		}
	}

	return strings.Join(result, "\n")
}

// tryParseTable 尝试从 lines[start] 开始解析一个完整的 Markdown 表格。
// 要求: lines[start] 是 header 行, lines[start+1] 是分隔行。
func tryParseTable(lines []string, start int) *parsedTable {
	if start+1 >= len(lines) {
		return nil
	}

	// header 行
	headers := parseTableRow(lines[start])
	if headers == nil {
		return nil
	}

	// 分隔行
	if !tableSepPattern.MatchString(lines[start+1]) {
		return nil
	}

	// 数据行
	var rows [][]string
	for j := start + 2; j < len(lines); j++ {
		cells := parseTableRow(lines[j])
		if cells == nil {
			break
		}
		// 对齐列数
		for len(cells) < len(headers) {
			cells = append(cells, "")
		}
		if len(cells) > len(headers) {
			cells = cells[:len(headers)]
		}
		rows = append(rows, cells)
	}

	return &parsedTable{
		headers: headers,
		rows:    rows,
	}
}

// renderAsBullets 将表格转为项目列表格式。
// TS 对照: ir.ts renderTableAsBullets()
//
// 两种格式:
//   - 多列: 第一列作为行标签，其余列为 key:value 对
//   - 单列: 简单列表
func renderAsBullets(t *parsedTable) []string {
	var out []string

	if len(t.headers) == 0 && len(t.rows) == 0 {
		return out
	}

	useFirstColAsLabel := len(t.headers) > 1 && len(t.rows) > 0

	if useFirstColAsLabel {
		for _, row := range t.rows {
			if len(row) == 0 {
				continue
			}
			// 行标签（加粗）
			if row[0] != "" {
				out = append(out, "**"+row[0]+"**")
			}
			// 每列作为 bullet
			for ci := 1; ci < len(row); ci++ {
				if row[ci] == "" {
					continue
				}
				header := ""
				if ci < len(t.headers) && t.headers[ci] != "" {
					header = t.headers[ci]
				}
				if header != "" {
					out = append(out, "• "+header+": "+row[ci])
				} else {
					out = append(out, "• "+row[ci])
				}
			}
			out = append(out, "")
		}
	} else {
		for _, row := range t.rows {
			for ci := 0; ci < len(row); ci++ {
				if row[ci] == "" {
					continue
				}
				header := ""
				if ci < len(t.headers) && t.headers[ci] != "" {
					header = t.headers[ci]
				}
				if header != "" {
					out = append(out, "• "+header+": "+row[ci])
				} else {
					out = append(out, "• "+row[ci])
				}
			}
			out = append(out, "")
		}
	}

	return out
}

// renderAsAlignedTable 将表格渲染为列对齐的代码风格表格。
// TS 对照: ir.ts renderTableAsCode()
func renderAsAlignedTable(t *parsedTable) []string {
	colCount := len(t.headers)
	for _, row := range t.rows {
		if len(row) > colCount {
			colCount = len(row)
		}
	}
	if colCount == 0 {
		return nil
	}

	// 计算各列最大宽度
	widths := make([]int, colCount)
	for ci := 0; ci < colCount; ci++ {
		if ci < len(t.headers) {
			w := runeWidth(t.headers[ci])
			if w > widths[ci] {
				widths[ci] = w
			}
		}
	}
	for _, row := range t.rows {
		for ci := 0; ci < colCount && ci < len(row); ci++ {
			w := runeWidth(row[ci])
			if w > widths[ci] {
				widths[ci] = w
			}
		}
	}
	// 最小宽度 3（分隔线 ---）
	for ci := range widths {
		if widths[ci] < 3 {
			widths[ci] = 3
		}
	}

	var out []string

	// header 行
	out = append(out, buildAlignedRow(t.headers, widths, colCount))

	// 分隔行
	var sep strings.Builder
	sep.WriteByte('|')
	for ci := 0; ci < colCount; ci++ {
		sep.WriteByte(' ')
		sep.WriteString(strings.Repeat("-", widths[ci]))
		sep.WriteString(" |")
	}
	out = append(out, sep.String())

	// 数据行
	for _, row := range t.rows {
		out = append(out, buildAlignedRow(row, widths, colCount))
	}

	out = append(out, "")
	return out
}

// buildAlignedRow 构建对齐的表格行。
func buildAlignedRow(cells []string, widths []int, colCount int) string {
	var b strings.Builder
	b.WriteByte('|')
	for ci := 0; ci < colCount; ci++ {
		b.WriteByte(' ')
		cell := ""
		if ci < len(cells) {
			cell = cells[ci]
		}
		b.WriteString(cell)
		pad := widths[ci] - runeWidth(cell)
		if pad > 0 {
			b.WriteString(strings.Repeat(" ", pad))
		}
		b.WriteString(" |")
	}
	return b.String()
}

// runeWidth 返回字符串的显示宽度（rune 数）。
func runeWidth(s string) int {
	return len([]rune(s))
}
