// view_tool.go — TUI 工具执行渲染组件
//
// 对齐 TS: components/tool-execution.ts(137L) — 差异 TE-01 (P0)
// 工具执行状态机: pending → running → success / error
//
// W3 产出文件 #2（审计新增）。
package tui

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/openacosmi/claw-acismi/internal/agents/tools"
)

// ---------- 常量 ----------

const previewLines = 12

// ---------- 类型 ----------

// ToolResultContent 工具结果内容条目。
// TS 参考: tool-execution.ts ToolResultContent
type ToolResultContent struct {
	Type     string `json:"type,omitempty"`
	Text     string `json:"text,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Bytes    int    `json:"bytes,omitempty"`
	Omitted  bool   `json:"omitted,omitempty"`
}

// ToolResult 工具执行结果。
// TS 参考: tool-execution.ts ToolResult
type ToolResult struct {
	Content []ToolResultContent    `json:"content,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// ToolExecState 工具执行状态。
type ToolExecState int

const (
	ToolStatePending ToolExecState = iota
	ToolStateRunning
	ToolStateSuccess
	ToolStateError
)

// ---------- 样式 ----------

var (
	toolPendingBg = lipgloss.NewStyle().
			Background(lipgloss.Color("#854D0E")).
			Foreground(lipgloss.Color("#FEF9C3"))

	toolSuccessBg = lipgloss.NewStyle().
			Background(lipgloss.Color("#166534")).
			Foreground(lipgloss.Color("#DCFCE7"))

	toolErrorBg = lipgloss.NewStyle().
			Background(lipgloss.Color("#991B1B")).
			Foreground(lipgloss.Color("#FEE2E2"))

	toolTitleStyle = lipgloss.NewStyle().Bold(true)

	toolDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF"))

	toolOutputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A1A1AA"))
)

// ---------- ToolExecView ----------

// ToolExecView 工具执行渲染组件。
// TS 参考: tool-execution.ts ToolExecutionComponent
type ToolExecView struct {
	ToolName  string
	Args      interface{}
	Result    *ToolResult
	State     ToolExecState
	Expanded  bool
	IsError   bool
	IsPartial bool
}

// NewToolExecView 创建工具执行视图。
func NewToolExecView(toolName string, args interface{}) *ToolExecView {
	return &ToolExecView{
		ToolName:  toolName,
		Args:      args,
		State:     ToolStatePending,
		IsPartial: true,
	}
}

// SetArgs 更新工具参数。
func (v *ToolExecView) SetArgs(args interface{}) {
	v.Args = args
}

// SetExpanded 设置展开/收起。
func (v *ToolExecView) SetExpanded(expanded bool) {
	v.Expanded = expanded
}

// SetResult 设置最终结果。
func (v *ToolExecView) SetResult(result *ToolResult, isError bool) {
	v.Result = result
	v.IsPartial = false
	v.IsError = isError
	if isError {
		v.State = ToolStateError
	} else {
		v.State = ToolStateSuccess
	}
}

// SetPartialResult 设置部分结果（running 中）。
func (v *ToolExecView) SetPartialResult(result *ToolResult) {
	v.Result = result
	v.IsPartial = true
	v.State = ToolStateRunning
}

// View 渲染工具执行视图。
// TS 参考: tool-execution.ts refresh()
func (v *ToolExecView) View(width int) string {
	if width < 20 {
		width = 20
	}
	innerWidth := width - 4

	// 选择背景样式
	bgStyle := toolPendingBg
	switch v.State {
	case ToolStateSuccess:
		bgStyle = toolSuccessBg
	case ToolStateError:
		bgStyle = toolErrorBg
	}

	// 标题行
	display := tools.ResolveToolDisplay(v.ToolName, castArgs(v.Args))
	titleText := fmt.Sprintf("%s %s", display.Emoji, display.Label)
	if v.IsPartial {
		titleText += " (running)"
	}
	titleLine := toolTitleStyle.Render(titleText)

	// 参数行
	argLine := formatToolArgs(v.ToolName, v.Args)
	if argLine != "" {
		argLine = toolDimStyle.Render(argLine)
	} else {
		argLine = toolDimStyle.Render(" ")
	}

	// 输出内容
	raw := extractToolText(v.Result)
	text := raw
	if text == "" && v.IsPartial {
		text = "…"
	}

	outputLine := ""
	if text != "" {
		if !v.Expanded {
			lines := strings.Split(text, "\n")
			if len(lines) > previewLines {
				text = strings.Join(lines[:previewLines], "\n") + "\n…"
			}
		}
		outputLine = toolOutputStyle.Render(text)
	}

	// 组合
	var content strings.Builder
	content.WriteString(titleLine)
	content.WriteString("\n")
	content.WriteString(argLine)
	if outputLine != "" {
		content.WriteString("\n")
		content.WriteString(outputLine)
	}

	box := bgStyle.
		Width(innerWidth).
		Padding(0, 1).
		Render(content.String())

	return "\n" + box
}

// ---------- 辅助函数 ----------

// castArgs 将 interface{} 转为 map[string]any。
func castArgs(args interface{}) map[string]any {
	if args == nil {
		return nil
	}
	if m, ok := args.(map[string]any); ok {
		return m
	}
	if m, ok := args.(map[string]interface{}); ok {
		return m
	}
	return nil
}

// formatToolArgs 格式化工具参数为显示文本。
// TS 参考: tool-execution.ts formatArgs
func formatToolArgs(toolName string, args interface{}) string {
	display := tools.ResolveToolDisplay(toolName, castArgs(args))
	detail := tools.FormatToolDetail(display)
	if detail != "" {
		return detail
	}
	if args == nil {
		return ""
	}
	data, err := json.Marshal(args)
	if err != nil {
		return ""
	}
	s := string(data)
	if s == "{}" || s == "null" {
		return ""
	}
	// 截断过长 JSON
	if len(s) > 200 {
		return s[:197] + "…"
	}
	return s
}

// extractToolText 从 ToolResult 中提取可显示文本。
// TS 参考: tool-execution.ts extractText
func extractToolText(result *ToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	var lines []string
	for _, entry := range result.Content {
		switch entry.Type {
		case "text":
			if entry.Text != "" {
				lines = append(lines, entry.Text)
			}
		case "image":
			mime := entry.MimeType
			if mime == "" {
				mime = "image"
			}
			size := ""
			if entry.Bytes > 0 {
				kb := math.Round(float64(entry.Bytes) / 1024)
				size = fmt.Sprintf(" %dkb", int(kb))
			}
			omitted := ""
			if entry.Omitted {
				omitted = " (omitted)"
			}
			lines = append(lines, fmt.Sprintf("[%s%s%s]", mime, size, omitted))
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
