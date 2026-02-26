// view_chat_log.go — TUI 消息列表渲染组件
//
// 对齐 TS: src/tui/components/chat-log.ts(105L) — 差异 CL-01 (P1)
// 消息列表 + viewport 滚动 + 工具追踪 Map。
package tui

import (
	"strings"
)

// MessageType 消息类型枚举。
type MessageType int

const (
	MessageTypeUser MessageType = iota
	MessageTypeAssistant
	MessageTypeSystem
)

// ChatMessage 聊天消息条目。
type ChatMessage struct {
	Type        MessageType
	Text        string
	RunID       string // 仅 assistant 类型使用
	IsStreaming bool   // 流式中
}

// ToolExecEntry 工具执行追踪条目。
// W3 view_tool.go 实现完整渲染，此处为 stub。
type ToolExecEntry struct {
	ToolName string
	Args     interface{}
	Result   interface{}
	IsError  bool
	Expanded bool
	view     *ToolExecView // W3: 渲染视图
}

// ToolResultOpts 工具结果选项。
type ToolResultOpts struct {
	IsError bool
	Partial bool
}

// ChatLog 消息列表组件。
// 管理消息列表 + 工具追踪 Map + 流式 run 映射。
type ChatLog struct {
	messages      []ChatMessage
	toolById      map[string]*ToolExecEntry
	toolOrder     []string       // 工具 ID 有序列表
	streamingRuns map[string]int // runId → messages 索引
	toolsExpanded bool
	scrollOffset  int
}

// NewChatLog 创建 ChatLog 实例。
func NewChatLog() *ChatLog {
	return &ChatLog{
		toolById:      make(map[string]*ToolExecEntry),
		streamingRuns: make(map[string]int),
	}
}

// ClearAll 清空所有消息和工具追踪。
// TS 参考: chat-log.ts L12-16
func (cl *ChatLog) ClearAll() {
	cl.messages = nil
	cl.toolById = make(map[string]*ToolExecEntry)
	cl.toolOrder = nil
	cl.streamingRuns = make(map[string]int)
	cl.scrollOffset = 0
}

// ---------- 消息管理 ----------

// AddSystem 添加系统消息。
// TS 参考: chat-log.ts L18-21
func (cl *ChatLog) AddSystem(text string) {
	cl.messages = append(cl.messages, ChatMessage{
		Type: MessageTypeSystem,
		Text: text,
	})
}

// AddUser 添加用户消息。
// TS 参考: chat-log.ts L23-25
func (cl *ChatLog) AddUser(text string) {
	cl.messages = append(cl.messages, ChatMessage{
		Type: MessageTypeUser,
		Text: text,
	})
}

// resolveRunID 解析 runId，空串默认 "default"。
// TS 参考: chat-log.ts L27-29
func resolveRunID(runID string) string {
	if runID == "" {
		return "default"
	}
	return runID
}

// startAssistant 开始一条新的助手消息。
// TS 参考: chat-log.ts L31-36
func (cl *ChatLog) startAssistant(text string, runID string) {
	effectiveRunID := resolveRunID(runID)
	idx := len(cl.messages)
	cl.messages = append(cl.messages, ChatMessage{
		Type:        MessageTypeAssistant,
		Text:        text,
		RunID:       effectiveRunID,
		IsStreaming: true,
	})
	cl.streamingRuns[effectiveRunID] = idx
}

// UpdateAssistant 流式更新助手消息。
// 如果 runId 对应的消息不存在，创建新消息。
// TS 参考: chat-log.ts L38-46
func (cl *ChatLog) UpdateAssistant(text string, runID string) {
	effectiveRunID := resolveRunID(runID)
	idx, ok := cl.streamingRuns[effectiveRunID]
	if !ok || idx >= len(cl.messages) {
		cl.startAssistant(text, runID)
		return
	}
	cl.messages[idx].Text = text
}

// FinalizeAssistant 最终化助手消息。
// TS 参考: chat-log.ts L48-57
func (cl *ChatLog) FinalizeAssistant(text string, runID string) {
	effectiveRunID := resolveRunID(runID)
	idx, ok := cl.streamingRuns[effectiveRunID]
	if ok && idx < len(cl.messages) {
		cl.messages[idx].Text = text
		cl.messages[idx].IsStreaming = false
		delete(cl.streamingRuns, effectiveRunID)
		return
	}
	cl.messages = append(cl.messages, ChatMessage{
		Type:  MessageTypeAssistant,
		Text:  text,
		RunID: effectiveRunID,
	})
}

// ---------- 工具追踪（差异 CL-01）----------

// StartTool 开始工具追踪。
// TS 参考: chat-log.ts L59-70
func (cl *ChatLog) StartTool(toolCallID, toolName string, args interface{}) *ToolExecEntry {
	existing, ok := cl.toolById[toolCallID]
	if ok {
		existing.Args = args
		if existing.view != nil {
			existing.view.SetArgs(args)
		}
		return existing
	}
	view := NewToolExecView(toolName, args)
	view.SetExpanded(cl.toolsExpanded)
	entry := &ToolExecEntry{
		ToolName: toolName,
		Args:     args,
		Expanded: cl.toolsExpanded,
		view:     view,
	}
	cl.toolById[toolCallID] = entry
	// 将工具追踪 ID 附加到有序列表
	cl.toolOrder = append(cl.toolOrder, toolCallID)
	return entry
}

// UpdateToolArgs 更新工具参数。
// TS 参考: chat-log.ts L72-78
func (cl *ChatLog) UpdateToolArgs(toolCallID string, args interface{}) {
	existing, ok := cl.toolById[toolCallID]
	if !ok {
		return
	}
	existing.Args = args
	if existing.view != nil {
		existing.view.SetArgs(args)
	}
}

// UpdateToolResult 更新工具结果。
// TS 参考: chat-log.ts L80-96
func (cl *ChatLog) UpdateToolResult(toolCallID string, result interface{}, opts ToolResultOpts) {
	existing, ok := cl.toolById[toolCallID]
	if !ok {
		return
	}
	existing.Result = result
	existing.IsError = opts.IsError

	if existing.view != nil {
		tr := castToToolResult(result)
		if opts.Partial {
			existing.view.SetPartialResult(tr)
		} else {
			existing.view.SetResult(tr, opts.IsError)
		}
	}
}

// SetToolsExpanded 切换工具展开/收起。
// TS 参考: chat-log.ts L98-103
func (cl *ChatLog) SetToolsExpanded(expanded bool) {
	cl.toolsExpanded = expanded
	for _, tool := range cl.toolById {
		tool.Expanded = expanded
		if tool.view != nil {
			tool.view.SetExpanded(expanded)
		}
	}
}

// ---------- 渲染 ----------

// View 渲染消息列表。
// W2 阶段使用简单文本渲染，W3 升级为 glamour Markdown。
func (cl *ChatLog) View(width, height int) string {
	if len(cl.messages) == 0 && len(cl.toolById) == 0 {
		return MutedStyle.Width(width).Render("(no messages)")
	}

	maxWidth := width - 4
	if maxWidth < 20 {
		maxWidth = 20
	}

	var parts []string

	// 渲染消息
	for _, msg := range cl.messages {
		switch msg.Type {
		case MessageTypeUser:
			parts = append(parts, renderUserMessage(msg.Text, maxWidth))
		case MessageTypeAssistant:
			parts = append(parts, renderAssistantMessage(msg.Text, msg.IsStreaming, maxWidth))
		case MessageTypeSystem:
			parts = append(parts, renderSystemMessage(msg.Text, maxWidth))
		default:
			parts = append(parts, msg.Text)
		}
	}

	// 渲染工具条目（W3 ToolExecView）
	for _, toolID := range cl.toolOrder {
		entry, ok := cl.toolById[toolID]
		if !ok || entry.view == nil {
			continue
		}
		parts = append(parts, entry.view.View(maxWidth))
	}

	content := strings.Join(parts, "\n")

	// 简单截断到 viewport 高度（自动滚动到底部）
	allLines := strings.Split(content, "\n")
	if len(allLines) > height {
		allLines = allLines[len(allLines)-height:]
	}

	return strings.Join(allLines, "\n")
}

// castToToolResult 将 interface{} 尝试转为 *ToolResult。
func castToToolResult(result interface{}) *ToolResult {
	if result == nil {
		return nil
	}
	if tr, ok := result.(*ToolResult); ok {
		return tr
	}
	// 尝试从 map 转换
	if m, ok := result.(map[string]interface{}); ok {
		tr := &ToolResult{}
		if content, ok := m["content"].([]interface{}); ok {
			for _, c := range content {
				if cm, ok := c.(map[string]interface{}); ok {
					trc := ToolResultContent{}
					if t, ok := cm["type"].(string); ok {
						trc.Type = t
					}
					if t, ok := cm["text"].(string); ok {
						trc.Text = t
					}
					if t, ok := cm["mimeType"].(string); ok {
						trc.MimeType = t
					}
					tr.Content = append(tr.Content, trc)
				}
			}
		}
		return tr
	}
	return nil
}

// MessageCount 返回消息数量。
func (cl *ChatLog) MessageCount() int {
	return len(cl.messages)
}

// ToolCount 返回工具追踪数量。
func (cl *ChatLog) ToolCount() int {
	return len(cl.toolById)
}
