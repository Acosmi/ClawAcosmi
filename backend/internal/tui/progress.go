package tui

// 对应 TS src/cli/progress.ts 进度条
// bubbletea progress bar 封装

import (
	"fmt"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
)

// ---------- 消息类型 ----------

// ProgressUpdateMsg 更新进度。
type ProgressUpdateMsg struct {
	Percent float64 // 0.0 ~ 1.0
	Label   string
}

// ProgressDoneMsg 进度完成。
type ProgressDoneMsg struct {
	Label string
}

// ---------- Model ----------

// ProgressModel 进度条模型。
type ProgressModel struct {
	progress progress.Model
	label    string
	percent  float64
	done     bool
	doneMsg  string
}

// NewProgress 创建进度条。
func NewProgress(label string) ProgressModel {
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
	)
	return ProgressModel{
		progress: p,
		label:    label,
	}
}

func (m ProgressModel) Init() tea.Cmd { return nil }

func (m ProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case ProgressUpdateMsg:
		m.percent = msg.Percent
		if msg.Label != "" {
			m.label = msg.Label
		}
	case ProgressDoneMsg:
		m.done = true
		m.doneMsg = msg.Label
		return m, tea.Quit
	case tea.WindowSizeMsg:
		m.progress.Width = msg.Width - 10
		if m.progress.Width > 80 {
			m.progress.Width = 80
		}
	case progress.FrameMsg:
		pm, cmd := m.progress.Update(msg)
		m.progress = pm.(progress.Model)
		return m, cmd
	}
	return m, nil
}

func (m ProgressModel) View() string {
	if m.done {
		return SuccessStyle.Render("✓ ") + m.doneMsg + "\n"
	}
	return fmt.Sprintf("%s %s\n%s",
		AccentStyle.Render(m.label),
		MutedStyle.Render(fmt.Sprintf("%.0f%%", m.percent*100)),
		m.progress.ViewAs(m.percent),
	) + "\n"
}
