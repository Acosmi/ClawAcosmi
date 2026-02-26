package tui

// 对应 TS src/wizard/prompts.ts WizardPrompter 接口
// 提供 setup wizard 所需的交互原语

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------- 通用接口 ----------

// PromptOption select/multiselect 选项。
type PromptOption struct {
	Value string
	Label string
	Hint  string
}

// WizardCancelledError 向导取消错误。
type WizardCancelledError struct {
	Message string
}

func (e *WizardCancelledError) Error() string {
	if e.Message == "" {
		return "wizard cancelled"
	}
	return e.Message
}

// WizardPrompter 向导交互接口（可 mock 测试）。
// TS 对照: src/wizard/prompts.ts WizardPrompter
type WizardPrompter interface {
	Intro(title string)
	Outro(message string)
	Note(message, title string)
	Select(message string, options []PromptOption, initialValue string) (string, error)
	MultiSelect(message string, options []PromptOption, initialValues []string) ([]string, error)
	TextInput(message, placeholder, initial string, validate func(string) string) (string, error)
	Confirm(message string, initial bool) (bool, error)
}

// ---------- Select Model ----------

type selectModel struct {
	message  string
	options  []PromptOption
	cursor   int
	selected string
	done     bool
	aborted  bool
}

func newSelectModel(message string, options []PromptOption, initial string) selectModel {
	cursor := 0
	for i, opt := range options {
		if opt.Value == initial {
			cursor = i
			break
		}
	}
	return selectModel{message: message, options: options, cursor: cursor}
}

func (m selectModel) Init() tea.Cmd { return nil }

func (m selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.aborted = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "enter":
			m.selected = m.options[m.cursor].Value
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m selectModel) View() string {
	if m.done {
		return SuccessStyle.Render("✓ ") + m.message + ": " + AccentStyle.Render(m.selected) + "\n"
	}
	var b strings.Builder
	b.WriteString(AccentStyle.Render("? ") + m.message + "\n")
	for i, opt := range m.options {
		cursor := "  "
		if i == m.cursor {
			cursor = AccentStyle.Render("> ")
		}
		label := opt.Label
		if opt.Hint != "" {
			label += MutedStyle.Render(" (" + opt.Hint + ")")
		}
		if i == m.cursor {
			label = lipgloss.NewStyle().Bold(true).Render(label)
		}
		b.WriteString(fmt.Sprintf("%s%s\n", cursor, label))
	}
	b.WriteString(MutedStyle.Render("\n  ↑/↓ navigate • enter select • q cancel"))
	return b.String()
}

// RunSelect 运行交互式选择。
func RunSelect(message string, options []PromptOption, initial string) (string, error) {
	m := newSelectModel(message, options, initial)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return "", err
	}
	sm := result.(selectModel)
	if sm.aborted {
		return "", fmt.Errorf("cancelled")
	}
	return sm.selected, nil
}

// ---------- Text Input Model ----------

type textInputModel struct {
	input      textinput.Model
	message    string
	validateFn func(string) string
	validErr   string
	done       bool
	aborted    bool
	finalValue string
}

func newTextInputModel(message, placeholder, initial string, validate func(string) string) textInputModel {
	ti := textinput.New()
	ti.Focus()
	ti.Placeholder = placeholder
	ti.SetValue(initial)
	ti.Width = 60
	return textInputModel{input: ti, message: message, validateFn: validate}
}

func (m textInputModel) Init() tea.Cmd { return textinput.Blink }

func (m textInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.aborted = true
			return m, tea.Quit
		case "enter":
			val := m.input.Value()
			if m.validateFn != nil {
				if errMsg := m.validateFn(val); errMsg != "" {
					m.validErr = errMsg
					return m, nil
				}
			}
			m.done = true
			m.finalValue = val
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.validErr = ""
	return m, cmd
}

func (m textInputModel) View() string {
	if m.done {
		return SuccessStyle.Render("✓ ") + m.message + ": " + AccentStyle.Render(m.finalValue) + "\n"
	}
	out := AccentStyle.Render("? ") + m.message + "\n" + m.input.View() + "\n"
	if m.validErr != "" {
		out += ErrorStyle.Render("  ✗ "+m.validErr) + "\n"
	}
	return out
}

// RunTextInput 运行文本输入。
func RunTextInput(message, placeholder, initial string, validate func(string) string) (string, error) {
	m := newTextInputModel(message, placeholder, initial, validate)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return "", err
	}
	tm := result.(textInputModel)
	if tm.aborted {
		return "", fmt.Errorf("cancelled")
	}
	return tm.finalValue, nil
}

// ---------- Confirm Model ----------

type confirmModel struct {
	message string
	value   bool
	done    bool
	aborted bool
}

func (m confirmModel) Init() tea.Cmd { return nil }

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.aborted = true
			return m, tea.Quit
		case "y", "Y":
			m.value = true
			m.done = true
			return m, tea.Quit
		case "n", "N":
			m.value = false
			m.done = true
			return m, tea.Quit
		case "enter":
			m.done = true
			return m, tea.Quit
		case "left", "h":
			m.value = true
		case "right", "l":
			m.value = false
		}
	}
	return m, nil
}

func (m confirmModel) View() string {
	if m.done {
		label := "No"
		if m.value {
			label = "Yes"
		}
		return SuccessStyle.Render("✓ ") + m.message + ": " + AccentStyle.Render(label) + "\n"
	}
	yes := "Yes"
	no := "No"
	if m.value {
		yes = AccentStyle.Render("[Yes]")
		no = MutedStyle.Render(" No ")
	} else {
		yes = MutedStyle.Render(" Yes ")
		no = AccentStyle.Render("[No]")
	}
	return AccentStyle.Render("? ") + m.message + "  " + yes + " / " + no + "\n"
}

// RunConfirm 运行确认提示。
func RunConfirm(message string, initial bool) (bool, error) {
	m := confirmModel{message: message, value: initial}
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return false, err
	}
	cm := result.(confirmModel)
	if cm.aborted {
		return false, fmt.Errorf("cancelled")
	}
	return cm.value, nil
}

// ---------- MultiSelect Model (TS: WizardPrompter.multiselect) ----------

type multiSelectModel struct {
	message  string
	options  []PromptOption
	cursor   int
	selected map[int]bool
	done     bool
	aborted  bool
}

func newMultiSelectModel(message string, options []PromptOption, initialValues []string) multiSelectModel {
	sel := make(map[int]bool)
	for i, opt := range options {
		for _, v := range initialValues {
			if opt.Value == v {
				sel[i] = true
			}
		}
	}
	return multiSelectModel{message: message, options: options, selected: sel}
}

func (m multiSelectModel) Init() tea.Cmd { return nil }

func (m multiSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.aborted = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case " ":
			m.selected[m.cursor] = !m.selected[m.cursor]
		case "enter":
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m multiSelectModel) View() string {
	if m.done {
		var labels []string
		for i, opt := range m.options {
			if m.selected[i] {
				labels = append(labels, opt.Label)
			}
		}
		return SuccessStyle.Render("✓ ") + m.message + ": " + AccentStyle.Render(strings.Join(labels, ", ")) + "\n"
	}
	var b strings.Builder
	b.WriteString(AccentStyle.Render("? ") + m.message + "\n")
	for i, opt := range m.options {
		cursor := "  "
		if i == m.cursor {
			cursor = AccentStyle.Render("> ")
		}
		check := "[ ] "
		if m.selected[i] {
			check = AccentStyle.Render("[✓] ")
		}
		label := opt.Label
		if opt.Hint != "" {
			label += MutedStyle.Render(" (" + opt.Hint + ")")
		}
		if i == m.cursor {
			label = lipgloss.NewStyle().Bold(true).Render(label)
		}
		b.WriteString(fmt.Sprintf("%s%s%s\n", cursor, check, label))
	}
	b.WriteString(MutedStyle.Render("\n  ↑/↓ navigate • space toggle • enter confirm • q cancel"))
	return b.String()
}

// RunMultiSelect 运行交互式多选。
func RunMultiSelect(message string, options []PromptOption, initialValues []string) ([]string, error) {
	m := newMultiSelectModel(message, options, initialValues)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return nil, err
	}
	msm := result.(multiSelectModel)
	if msm.aborted {
		return nil, &WizardCancelledError{}
	}
	var values []string
	for i, opt := range msm.options {
		if msm.selected[i] {
			values = append(values, opt.Value)
		}
	}
	return values, nil
}
