// view_input.go — TUI 输入框组件
//
// 对齐 TS: components/custom-editor.ts(61L) — 差异 IN-01 (P1)
// 使用 bubbles/textarea 实现输入框 + 自定义快捷键 + 输入历史。
//
// W3 产出文件 #3。
package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------- 常量 ----------

const maxHistorySize = 50

// ---------- 消息类型 ----------

// InputSubmitMsg 用户提交输入。
type InputSubmitMsg struct {
	Text string
}

// ---------- InputBox ----------

// InputBox TUI 输入框组件。
// TS 参考: components/custom-editor.ts CustomEditor
type InputBox struct {
	textarea textarea.Model
	history  []string
	histIdx  int    // -1 = 当前输入, 0..n = 历史索引
	saved    string // 保存当前输入（浏览历史时）
	width    int
	focused  bool
}

// NewInputBox 创建输入框。
func NewInputBox(width int) InputBox {
	ta := textarea.New()
	ta.Placeholder = "Type a message... (/help for commands)"
	ta.ShowLineNumbers = false
	ta.SetWidth(width - 4)
	ta.SetHeight(3)
	ta.CharLimit = 0 // 无限制
	ta.Focus()

	// 自定义样式
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Base = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		Padding(0, 1)
	ta.BlurredStyle.Base = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#4B5563")).
		Padding(0, 1)

	return InputBox{
		textarea: ta,
		histIdx:  -1,
		width:    width,
		focused:  true,
	}
}

// SetWidth 更新宽度。
func (ib *InputBox) SetWidth(width int) {
	ib.width = width
	ib.textarea.SetWidth(width - 4)
}

// Focus 获取焦点。
func (ib *InputBox) Focus() {
	ib.textarea.Focus()
	ib.focused = true
}

// Blur 失去焦点。
func (ib *InputBox) Blur() {
	ib.textarea.Blur()
	ib.focused = false
}

// Value 获取当前输入值。
func (ib *InputBox) Value() string {
	return ib.textarea.Value()
}

// Reset 清空输入。
func (ib *InputBox) Reset() {
	ib.textarea.Reset()
	ib.histIdx = -1
	ib.saved = ""
}

// SetValue 设置输入值。
func (ib *InputBox) SetValue(text string) {
	ib.textarea.SetValue(text)
}

// Update 处理事件。返回 (model, cmd, handled)。
// handled=true 表示事件已被输入框处理，外层不应再处理。
func (ib *InputBox) Update(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return ib.handleKey(msg)
	}

	var cmd tea.Cmd
	ib.textarea, cmd = ib.textarea.Update(msg)
	return cmd, false
}

// handleKey 处理键盘事件。
// TS 参考: custom-editor.ts handleInput
func (ib *InputBox) handleKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.Type {
	case tea.KeyEnter:
		// Enter 提交（如果有内容）
		text := strings.TrimSpace(ib.textarea.Value())
		if text == "" {
			return nil, true
		}
		ib.addToHistory(text)
		ib.textarea.Reset()
		ib.histIdx = -1
		ib.saved = ""
		return func() tea.Msg {
			return InputSubmitMsg{Text: text}
		}, true

	case tea.KeyUp:
		// 上箭头 — 浏览历史
		if len(ib.history) == 0 {
			return nil, true
		}
		if ib.histIdx == -1 {
			ib.saved = ib.textarea.Value()
			ib.histIdx = len(ib.history) - 1
		} else if ib.histIdx > 0 {
			ib.histIdx--
		}
		ib.textarea.SetValue(ib.history[ib.histIdx])
		return nil, true

	case tea.KeyDown:
		// 下箭头 — 浏览历史
		if ib.histIdx == -1 {
			return nil, true
		}
		if ib.histIdx < len(ib.history)-1 {
			ib.histIdx++
			ib.textarea.SetValue(ib.history[ib.histIdx])
		} else {
			ib.histIdx = -1
			ib.textarea.SetValue(ib.saved)
		}
		return nil, true
	}

	// 其他键交给 textarea 处理
	var cmd tea.Cmd
	ib.textarea, cmd = ib.textarea.Update(msg)
	return cmd, false
}

// addToHistory 添加输入到历史。
func (ib *InputBox) addToHistory(text string) {
	// 去重：移除相同的旧条目
	for i, h := range ib.history {
		if h == text {
			ib.history = append(ib.history[:i], ib.history[i+1:]...)
			break
		}
	}
	ib.history = append(ib.history, text)
	if len(ib.history) > maxHistorySize {
		ib.history = ib.history[len(ib.history)-maxHistorySize:]
	}
}

// HasSlashPrefix 检查当前输入是否以 / 开头。
// 用于触发 slash 命令补全（W4 完善）。
func (ib *InputBox) HasSlashPrefix() bool {
	return strings.HasPrefix(ib.textarea.Value(), "/")
}

// HasBangPrefix 检查当前输入是否以 ! 开头。
// 用于触发本地 shell 命令（W4 完善）。
func (ib *InputBox) HasBangPrefix() bool {
	return strings.HasPrefix(ib.textarea.Value(), "!")
}

// View 渲染输入框。
func (ib *InputBox) View() string {
	return ib.textarea.View()
}
