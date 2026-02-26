// view_message.go — TUI 消息渲染组件
//
// 对齐 TS: components/user-message.ts(21L) + assistant-message.ts(20L)
// 使用 glamour 进行 Markdown 渲染，替代 view_chat_log.go 中的简单渲染。
//
// W3 产出文件 #1。
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// ---------- 样式定义 ----------

var (
	// userPrefixStyle 用户消息前缀。
	userPrefixStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#60A5FA")).
			Bold(true)

	// userBgStyle 用户消息背景。
	userBgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E0E7FF"))

	// assistantStyle 助手消息默认样式。
	assistantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D1D5DB"))

	// streamingCursorStyle 流式光标样式。
	streamingCursorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7C3AED")).
				Bold(true)
)

// ---------- glamour 渲染器 ----------

// markdownRenderer 全局 Markdown 渲染器（惰性初始化）。
var markdownRenderer *glamour.TermRenderer

// getMarkdownRenderer 返回或创建 Markdown 渲染器。
func getMarkdownRenderer(width int) *glamour.TermRenderer {
	if markdownRenderer != nil {
		return markdownRenderer
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil
	}
	markdownRenderer = r
	return r
}

// renderMarkdown 渲染 Markdown 文本。降级为纯文本。
func renderMarkdown(text string, width int) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	r := getMarkdownRenderer(width)
	if r == nil {
		return text
	}
	out, err := r.Render(text)
	if err != nil {
		return text
	}
	return strings.TrimRight(out, "\n")
}

// ---------- 消息渲染函数 ----------

// renderUserMessage 渲染用户消息。
// TS 参考: components/user-message.ts UserMessageComponent
func renderUserMessage(text string, width int) string {
	rendered := renderMarkdown(text, width-4)
	if rendered == "" {
		rendered = text
	}
	return fmt.Sprintf("\n%s %s",
		userPrefixStyle.Render("❯"),
		userBgStyle.Render(rendered),
	)
}

// renderAssistantMessage 渲染助手消息。
// TS 参考: components/assistant-message.ts AssistantMessageComponent
func renderAssistantMessage(text string, isStreaming bool, width int) string {
	rendered := renderMarkdown(text, width-2)
	if rendered == "" {
		rendered = text
	}

	if isStreaming {
		rendered += streamingCursorStyle.Render(" ▍")
	}

	return fmt.Sprintf("\n%s", rendered)
}

// renderSystemMessage 渲染系统消息。
func renderSystemMessage(text string, _ int) string {
	return fmt.Sprintf("\n%s", MutedStyle.Render(text))
}
