// Package tui 提供终端用户界面组件。
// 对应 TS src/wizard/clack-prompter.ts + src/cli/progress.ts
// 使用 bubbletea (Elm Architecture) 实现。
package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------- 主题样式 ----------

var (
	// AccentStyle 强调色。
	AccentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7C3AED")).
			Bold(true)

	// HeadingStyle 标题。
	HeadingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A78BFA")).
			Bold(true).
			MarginBottom(1)

	// MutedStyle 静音。
	MutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))

	// SuccessStyle 成功。
	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981"))

	// ErrorStyle 错误。
	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444"))

	// WarningStyle 警告。
	WarningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B"))
)

// ---------- 应用入口 ----------

// Run 启动 TUI 程序。
func Run(model tea.Model) error {
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		return err
	}
	return nil
}

// RunInline 启动内联 TUI（不接管全屏）。
func RunInline(model tea.Model) error {
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
