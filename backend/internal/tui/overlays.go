// overlays.go — TUI 模态选择列表 + overlay 管理
//
// 对齐 TS: src/tui/tui-overlays.ts(19L) + components/selectors.ts(31L)
//   - searchable-select-list.ts(311L) + filterable-select-list.ts(143L)
//
// 差异 SS-01 (P1): Go 使用 bubbles/list 替代 TS 自研组件。
//
// W5 产出文件 #2。
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/openacosmi/claw-acismi/internal/autoreply"
)

// ---------- Overlay 状态 ----------

// OverlayKind overlay 类型枚举。
type OverlayKind int

const (
	OverlayNone OverlayKind = iota
	OverlayAgentSelect
	OverlaySessionSelect
	OverlayModelSelect
	OverlaySettings
)

// ---------- SelectItem ----------

// selectItem 选择列表项（实现 list.Item 接口）。
type selectItem struct {
	value       string
	label       string
	description string
}

func (i selectItem) Title() string       { return i.label }
func (i selectItem) Description() string { return i.description }
func (i selectItem) FilterValue() string { return i.label + " " + i.description }

// ---------- Overlay 消息 ----------

// OverlaySelectMsg overlay 选择结果。
type OverlaySelectMsg struct {
	Kind  OverlayKind
	Value string
}

// OverlayCancelMsg overlay 取消。
type OverlayCancelMsg struct{}

// OverlayItemsMsg overlay 异步数据加载结果。
type OverlayItemsMsg struct {
	Kind  OverlayKind
	Items []selectItem
	Err   error
}

// ---------- Overlay 管理 ----------

// overlayState 当前 overlay 状态。
type overlayState struct {
	kind      OverlayKind
	listModel list.Model
	active    bool
}

// newOverlayList 创建 overlay 选择列表。
func newOverlayList(title string, items []selectItem, width, height int) list.Model {
	// 转为 list.Item 切片
	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = item
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Foreground(Palette.Accent).Bold(true)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(Palette.AccentSoft)
	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(Palette.Text)
	delegate.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(Palette.Dim)

	w := width - 4
	if w < 40 {
		w = 40
	}
	h := height - 6
	if h < 5 {
		h = 5
	}
	if h > 15 {
		h = 15
	}

	l := list.New(listItems, delegate, w, h)
	l.Title = title
	l.Styles.Title = lipgloss.NewStyle().
		Foreground(Palette.Accent).Bold(true).
		MarginBottom(1)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()

	return l
}

// ---------- Model overlay 方法 ----------

// openOverlay 打开 overlay。
func (m *Model) openOverlay(kind OverlayKind, title string, items []selectItem) {
	m.overlay.kind = kind
	m.overlay.listModel = newOverlayList(title, items, m.width, m.height)
	m.overlay.active = true
}

// closeOverlay 关闭 overlay。
func (m *Model) closeOverlay() {
	m.overlay.active = false
	m.overlay.kind = OverlayNone
}

// hasOverlay 检查是否有活跃 overlay。
func (m *Model) hasOverlay() bool {
	return m.overlay.active
}

// updateOverlay 处理 overlay 事件。
func (m *Model) updateOverlay(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.overlay.active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.closeOverlay()
			return m, nil
		case tea.KeyEnter:
			selected := m.overlay.listModel.SelectedItem()
			if selected != nil {
				if item, ok := selected.(selectItem); ok {
					return m.handleOverlaySelect(item)
				}
			}
			m.closeOverlay()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.overlay.listModel, cmd = m.overlay.listModel.Update(msg)
	return m, cmd
}

// handleOverlaySelect 处理 overlay 选择。
func (m *Model) handleOverlaySelect(item selectItem) (tea.Model, tea.Cmd) {
	kind := m.overlay.kind
	m.closeOverlay()

	switch kind {
	case OverlayAgentSelect:
		return m, m.handleSetAgent(item.value)

	case OverlaySessionSelect:
		return m, m.setSessionCmd(item.value)

	case OverlayModelSelect:
		return m, m.handlePatchSession("model", item.value)

	case OverlaySettings:
		return m, m.handleSettingsSelection(item.value)
	}

	return m, nil
}

// handleSettingsSelection 处理 settings overlay 选项选择。
// TS 参考: tui-command-handlers.ts openSettings L203-238
func (m *Model) handleSettingsSelection(value string) tea.Cmd {
	switch value {
	case "thinking":
		levels := autoreply.FormatThinkingLevels(
			m.sessionInfo.ModelProvider,
			m.sessionInfo.Model,
			"|",
		)
		m.chatLog.AddSystem(fmt.Sprintf("usage: /think <%s>", levels))
	case "verbose":
		m.chatLog.AddSystem("usage: /verbose <on|off>")
	case "reasoning":
		m.chatLog.AddSystem("usage: /reasoning <on|off>")
	case "usage":
		return m.handleUsageCommand("")
	case "elevated":
		m.chatLog.AddSystem("usage: /elevated <on|off|ask|full>")
	case "activation":
		m.chatLog.AddSystem("usage: /activation <mention|always>")
	}
	return nil
}

// renderOverlay 渲染 overlay 视图。
func (m Model) renderOverlay() string {
	if !m.overlay.active {
		return ""
	}

	content := m.overlay.listModel.View()

	// 添加边框和居中
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Palette.Border).
		Padding(1, 2).
		Width(m.width - 8)

	box := boxStyle.Render(content)

	// 居中
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		box,
		lipgloss.WithWhitespaceChars(" "),
	)
}

// ---------- Overlay 创建命令 ----------

// openAgentSelectorCmd 打开 agent 选择器。
func (m *Model) openAgentSelectorCmd() tea.Cmd {
	return func() tea.Msg {
		result, err := m.client.ListAgents()
		if err != nil {
			return OverlayItemsMsg{Kind: OverlayAgentSelect, Err: err}
		}
		var items []selectItem
		for _, a := range result.Agents {
			name := strings.TrimSpace(a.Name)
			desc := ""
			if a.ID == result.DefaultID {
				desc = "(default)"
			}
			label := a.ID
			if name != "" {
				label = fmt.Sprintf("%s (%s)", a.ID, name)
			}
			items = append(items, selectItem{
				value:       a.ID,
				label:       label,
				description: desc,
			})
		}
		return OverlayItemsMsg{Kind: OverlayAgentSelect, Items: items}
	}
}

// openSessionSelectorCmd 打开 session 选择器。
func (m *Model) openSessionSelectorCmd() tea.Cmd {
	return func() tea.Msg {
		trueVal := true
		result, err := m.client.ListSessions(&SessionsListParams{
			IncludeDerivedTitles: &trueVal,
			IncludeLastMessage:   &trueVal,
		})
		if err != nil {
			return OverlayItemsMsg{Kind: OverlaySessionSelect, Err: err}
		}
		var items []selectItem
		for _, s := range result.Sessions {
			label := s.Key
			desc := ""
			if s.DisplayName != "" {
				desc = s.DisplayName
			} else if s.DerivedTitle != "" {
				desc = s.DerivedTitle
			} else if s.LastMessagePreview != "" {
				preview := s.LastMessagePreview
				if len(preview) > 60 {
					preview = preview[:60] + "…"
				}
				desc = preview
			}
			items = append(items, selectItem{
				value:       s.Key,
				label:       label,
				description: desc,
			})
		}
		return OverlayItemsMsg{Kind: OverlaySessionSelect, Items: items}
	}
}

// openModelSelectorCmd 打开 model 选择器。
func (m *Model) openModelSelectorCmd() tea.Cmd {
	return func() tea.Msg {
		models, err := m.client.ListModels()
		if err != nil {
			return OverlayItemsMsg{Kind: OverlayModelSelect, Err: err}
		}
		var items []selectItem
		for _, model := range models {
			label := model.Name
			if label == "" {
				label = model.ID
			}
			desc := model.Provider
			if model.ContextWindow != nil {
				desc += fmt.Sprintf(" (%dk ctx)", *model.ContextWindow/1000)
			}
			items = append(items, selectItem{
				value:       fmt.Sprintf("%s/%s", model.Provider, model.ID),
				label:       label,
				description: desc,
			})
		}
		return OverlayItemsMsg{Kind: OverlayModelSelect, Items: items}
	}
}

// openSettingsSelectorCmd 打开 settings 选择器。
// TS 参考: tui-command-handlers.ts openSettings L203-238
func (m *Model) openSettingsSelectorCmd() tea.Cmd {
	items := []selectItem{
		{value: "thinking", label: "Thinking", description: "Set thinking level"},
		{value: "verbose", label: "Verbose", description: "Toggle verbose mode"},
		{value: "reasoning", label: "Reasoning", description: "Toggle reasoning mode"},
		{value: "usage", label: "Usage Footer", description: "Cycle usage display"},
		{value: "elevated", label: "Elevated", description: "Set elevated permissions"},
		{value: "activation", label: "Activation", description: "Set group activation mode"},
	}
	return func() tea.Msg {
		return OverlayItemsMsg{Kind: OverlaySettings, Items: items}
	}
}
