package tui

// 对应 TS src/terminal/table.ts — 表格渲染
// 简单的终端表格（非 bubbletea，直接字符串输出）

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// TableColumn 列定义。
type TableColumn struct {
	Header string
	Width  int
}

// RenderTable 渲染简单表格。
func RenderTable(columns []TableColumn, rows [][]string) string {
	if len(columns) == 0 {
		return ""
	}

	// 计算列宽
	widths := make([]int, len(columns))
	for i, col := range columns {
		widths[i] = col.Width
		if widths[i] == 0 {
			widths[i] = len(col.Header)
		}
		for _, row := range rows {
			if i < len(row) && len(row[i]) > widths[i] {
				widths[i] = len(row[i])
			}
		}
		widths[i] += 2 // padding
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A78BFA"))
	cellStyle := lipgloss.NewStyle()
	sepStyle := MutedStyle

	var b strings.Builder

	// 表头
	for i, col := range columns {
		b.WriteString(headerStyle.Width(widths[i]).Render(col.Header))
	}
	b.WriteString("\n")

	// 分隔线
	for i := range columns {
		b.WriteString(sepStyle.Render(strings.Repeat("─", widths[i])))
	}
	b.WriteString("\n")

	// 数据行
	for _, row := range rows {
		for i := range columns {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			b.WriteString(cellStyle.Width(widths[i]).Render(cell))
		}
		b.WriteString("\n")
	}

	if len(rows) == 0 {
		b.WriteString(MutedStyle.Render("  (no data)") + "\n")
	}

	b.WriteString(fmt.Sprintf("\n%s\n", MutedStyle.Render(fmt.Sprintf("Total: %d", len(rows)))))
	return b.String()
}
