package tui

// 对应 TS src/wizard/ — 交互式设置向导
// bubbletea 全屏向导模型

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------- Wizard 步骤 ----------

// WizardStepKind 步骤类型。
type WizardStepKind int

const (
	StepSelect WizardStepKind = iota
	StepTextInput
	StepConfirm
	StepNote
)

// WizardStep 向导步骤。
type WizardStep struct {
	Kind        WizardStepKind
	ID          string
	Message     string
	Options     []PromptOption // StepSelect
	Placeholder string         // StepTextInput
	Initial     string         // StepTextInput/StepSelect
	InitialBool bool           // StepConfirm
	Validate    func(string) string
	NoteTitle   string // StepNote
}

// WizardResult 向导结果。
type WizardResult struct {
	Values  map[string]string
	Aborted bool
}

// ---------- Wizard Model ----------

// WizardModel 全屏向导。
type WizardModel struct {
	title   string
	steps   []WizardStep
	current int
	values  map[string]string
	aborted bool

	// 当前步骤内部状态
	selectCursor int
	textInput    string
	textCursor   int
	confirmVal   bool
	validErr     string
}

// NewWizard 创建向导。
func NewWizard(title string, steps []WizardStep) WizardModel {
	return WizardModel{
		title:  title,
		steps:  steps,
		values: make(map[string]string),
	}
}

func (m WizardModel) Init() tea.Cmd { return nil }

func (m WizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.current >= len(m.steps) {
		return m, tea.Quit
	}

	step := m.steps[m.current]

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.aborted = true
			return m, tea.Quit
		}

		switch step.Kind {
		case StepSelect:
			return m.updateSelect(msg)
		case StepTextInput:
			return m.updateTextInput(msg)
		case StepConfirm:
			return m.updateConfirm(msg)
		case StepNote:
			if msg.String() == "enter" {
				m.current++
				return m, nil
			}
		}
	}
	return m, nil
}

func (m WizardModel) updateSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	step := m.steps[m.current]
	switch msg.String() {
	case "up", "k":
		if m.selectCursor > 0 {
			m.selectCursor--
		}
	case "down", "j":
		if m.selectCursor < len(step.Options)-1 {
			m.selectCursor++
		}
	case "enter":
		m.values[step.ID] = step.Options[m.selectCursor].Value
		m.current++
		m.selectCursor = 0
	}
	return m, nil
}

func (m WizardModel) updateTextInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	step := m.steps[m.current]
	switch msg.String() {
	case "enter":
		val := m.textInput
		if step.Validate != nil {
			if errMsg := step.Validate(val); errMsg != "" {
				m.validErr = errMsg
				return m, nil
			}
		}
		m.values[step.ID] = val
		m.current++
		m.textInput = ""
		m.validErr = ""
	case "backspace":
		if len(m.textInput) > 0 {
			m.textInput = m.textInput[:len(m.textInput)-1]
		}
		m.validErr = ""
	default:
		if len(msg.String()) == 1 {
			m.textInput += msg.String()
			m.validErr = ""
		}
	}
	return m, nil
}

func (m WizardModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	step := m.steps[m.current]
	switch msg.String() {
	case "y", "Y":
		m.values[step.ID] = "true"
		m.current++
		m.confirmVal = false
	case "n", "N":
		m.values[step.ID] = "false"
		m.current++
		m.confirmVal = false
	case "left", "h":
		m.confirmVal = true
	case "right", "l":
		m.confirmVal = false
	case "enter":
		if m.confirmVal {
			m.values[step.ID] = "true"
		} else {
			m.values[step.ID] = "false"
		}
		m.current++
		m.confirmVal = false
	}
	return m, nil
}

func (m WizardModel) View() string {
	var b strings.Builder

	// 标题
	titleBox := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		Padding(0, 2).
		Render(HeadingStyle.Render("🧙 " + m.title))
	b.WriteString(titleBox + "\n\n")

	// 进度
	progress := fmt.Sprintf("Step %d/%d", m.current+1, len(m.steps))
	b.WriteString(MutedStyle.Render(progress) + "\n\n")

	if m.current >= len(m.steps) {
		b.WriteString(SuccessStyle.Render("✓ Setup complete!") + "\n")
		return b.String()
	}

	step := m.steps[m.current]

	switch step.Kind {
	case StepSelect:
		b.WriteString(AccentStyle.Render("? ") + step.Message + "\n\n")
		for i, opt := range step.Options {
			cursor := "  "
			if i == m.selectCursor {
				cursor = AccentStyle.Render("> ")
			}
			label := opt.Label
			if opt.Hint != "" {
				label += MutedStyle.Render(" — " + opt.Hint)
			}
			if i == m.selectCursor {
				label = lipgloss.NewStyle().Bold(true).Render(label)
			}
			b.WriteString(fmt.Sprintf("%s%s\n", cursor, label))
		}

	case StepTextInput:
		b.WriteString(AccentStyle.Render("? ") + step.Message + "\n")
		display := m.textInput
		if display == "" && step.Placeholder != "" {
			display = MutedStyle.Render(step.Placeholder)
		}
		b.WriteString("  > " + display + "█\n")
		if m.validErr != "" {
			b.WriteString(ErrorStyle.Render("  ✗ "+m.validErr) + "\n")
		}

	case StepConfirm:
		yes, no := " Yes ", "[No]"
		if m.confirmVal {
			yes = "[Yes]"
			no = " No "
		}
		b.WriteString(AccentStyle.Render("? ") + step.Message + "  ")
		b.WriteString(AccentStyle.Render(yes) + " / " + MutedStyle.Render(no) + "\n")

	case StepNote:
		noteBox := lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7C3AED")).
			Padding(0, 2).
			Render(step.Message)
		if step.NoteTitle != "" {
			b.WriteString(AccentStyle.Render("ℹ "+step.NoteTitle) + "\n")
		}
		b.WriteString(noteBox + "\n")
		b.WriteString(MutedStyle.Render("  Press enter to continue..."))
	}

	b.WriteString("\n" + MutedStyle.Render("  ctrl+c cancel"))
	return b.String()
}

// GetResult 获取最终结果。
func (m WizardModel) GetResult() WizardResult {
	return WizardResult{
		Values:  m.values,
		Aborted: m.aborted,
	}
}

// RunWizard 运行完整向导。
func RunWizard(title string, steps []WizardStep) (WizardResult, error) {
	m := NewWizard(title, steps)
	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return WizardResult{Aborted: true}, err
	}
	wm := result.(WizardModel)
	return wm.GetResult(), nil
}
