package tui

// 对应 TS src/cli/progress.ts spinner 组件
// bubbletea spinner 封装

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------- Spinner Model ----------

// SpinnerModel 旋转加载指示器。
type SpinnerModel struct {
	spinner spinner.Model
	label   string
	done    bool
	doneMsg string
	err     error
}

// SpinnerDoneMsg 完成消息。
type SpinnerDoneMsg struct {
	Message string
	Err     error
}

// NewSpinner 创建新的 spinner。
func NewSpinner(label string) SpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED"))
	return SpinnerModel{
		spinner: s,
		label:   label,
	}
}

func (m SpinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m SpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case SpinnerDoneMsg:
		m.done = true
		m.doneMsg = msg.Message
		m.err = msg.Err
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m SpinnerModel) View() string {
	if m.done {
		if m.err != nil {
			return ErrorStyle.Render("✗ ") + m.doneMsg + "\n"
		}
		return SuccessStyle.Render("✓ ") + m.doneMsg + "\n"
	}
	return fmt.Sprintf("%s %s\n", m.spinner.View(), AccentStyle.Render(m.label))
}
