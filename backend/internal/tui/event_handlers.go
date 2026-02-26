// event_handlers.go — TUI 事件处理器
//
// 对齐 TS: src/tui/tui-event-handlers.ts(248L) — 差异 E-01/E-02 (P2)
// handleChatEvent: delta/final/aborted/error 四状态处理
// handleAgentEvent: tool(start/update/result) + lifecycle(start/end/error)
//
// W4 产出文件 #2。
package tui

import (
	"encoding/json"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ---------- ChatEvent / AgentEvent 结构 ----------

// ChatEvent 聊天事件。
// TS 参考: tui-types.ts ChatEvent
type ChatEvent struct {
	SessionKey   string      `json:"sessionKey"`
	RunID        string      `json:"runId"`
	State        string      `json:"state"` // "delta" | "final" | "aborted" | "error"
	Message      interface{} `json:"message,omitempty"`
	ErrorMessage string      `json:"errorMessage,omitempty"`
}

// AgentEvent Agent 事件。
// TS 参考: tui-types.ts AgentEvent
type AgentEvent struct {
	RunID  string                 `json:"runId"`
	Stream string                 `json:"stream"` // "tool" | "lifecycle"
	Data   map[string]interface{} `json:"data,omitempty"`
}

// ---------- Run Map 管理 ----------

// pruneRunMap 裁剪 run Map。
// >200 条时先删除 10 分钟过期条目，若仍 >200 强制删除至 150。
// TS 参考: tui-event-handlers.ts L36-57 — 差异 E-01
func pruneRunMap(runs map[string]int64) {
	if len(runs) <= 200 {
		return
	}
	keepUntil := time.Now().UnixMilli() - 10*60*1000
	for key, ts := range runs {
		if len(runs) <= 150 {
			break
		}
		if ts < keepUntil {
			delete(runs, key)
		}
	}
	// 仍超限则强制删除最旧
	if len(runs) > 200 {
		for key := range runs {
			delete(runs, key)
			if len(runs) <= 150 {
				break
			}
		}
	}
}

// ---------- Session 同步 ----------

// syncSessionKey 检测 session 切换，清空 run 追踪。
// TS 参考: tui-event-handlers.ts L59-68
func (m *Model) syncSessionKey() {
	if m.currentSessionKey == m.lastSessionKey {
		return
	}
	m.lastSessionKey = m.currentSessionKey
	m.finalizedRuns = make(map[string]int64)
	m.sessionRuns = make(map[string]int64)
	m.streamAssembler.Reset()
	m.clearLocalRunIDs()
}

// noteSessionRun 记录活跃 run。
func (m *Model) noteSessionRun(runID string) {
	m.sessionRuns[runID] = time.Now().UnixMilli()
	pruneRunMap(m.sessionRuns)
}

// noteFinalizedRun 标记 run 已完成。
func (m *Model) noteFinalizedRun(runID string) {
	m.finalizedRuns[runID] = time.Now().UnixMilli()
	delete(m.sessionRuns, runID)
	m.streamAssembler.Drop(runID)
	pruneRunMap(m.finalizedRuns)
}

// ---------- 聊天事件处理 ----------

// handleChatEvent 处理 chat 事件。
// TS 参考: tui-event-handlers.ts L82-177
func (m *Model) handleChatEvent(payload interface{}) tea.Cmd {
	if payload == nil {
		return nil
	}
	record, ok := payload.(map[string]interface{})
	if !ok {
		return nil
	}

	evt := parseChatEvent(record)
	m.syncSessionKey()

	// 过滤非当前 session 的事件
	if evt.SessionKey != m.currentSessionKey {
		return nil
	}

	// 过滤已完成的 run 的 delta/final
	if _, finalized := m.finalizedRuns[evt.RunID]; finalized {
		if evt.State == "delta" || evt.State == "final" {
			return nil
		}
	}

	m.noteSessionRun(evt.RunID)
	if m.activeChatRunID == "" {
		m.activeChatRunID = evt.RunID
	}

	switch evt.State {
	case "delta":
		displayText := m.streamAssembler.IngestDelta(evt.RunID, evt.Message, m.showThinking)
		if displayText == "" {
			return nil
		}
		m.chatLog.UpdateAssistant(displayText, evt.RunID)
		m.activityStatus = "streaming"

	case "final":
		return m.handleChatFinal(evt)

	case "aborted":
		m.chatLog.AddSystem("run aborted")
		m.streamAssembler.Drop(evt.RunID)
		delete(m.sessionRuns, evt.RunID)
		m.activeChatRunID = ""
		m.activityStatus = "aborted"
		if m.isLocalRunID(evt.RunID) {
			m.forgetLocalRunID(evt.RunID)
		}
		return m.refreshSessionInfoCmd()

	case "error":
		errMsg := evt.ErrorMessage
		if errMsg == "" {
			errMsg = "unknown"
		}
		m.chatLog.AddSystem(fmt.Sprintf("run error: %s", errMsg))
		m.streamAssembler.Drop(evt.RunID)
		delete(m.sessionRuns, evt.RunID)
		m.activeChatRunID = ""
		m.activityStatus = "error"
		if m.isLocalRunID(evt.RunID) {
			m.forgetLocalRunID(evt.RunID)
		}
		return m.refreshSessionInfoCmd()
	}

	return nil
}

// handleChatFinal 处理 chat final 事件。
func (m *Model) handleChatFinal(evt ChatEvent) tea.Cmd {
	// command 消息特殊处理
	if IsCommandMessage(evt.Message) {
		if m.isLocalRunID(evt.RunID) {
			m.forgetLocalRunID(evt.RunID)
		}
		text := ExtractTextFromMessage(evt.Message, false)
		if text != "" {
			m.chatLog.AddSystem(text)
		}
		m.streamAssembler.Drop(evt.RunID)
		m.noteFinalizedRun(evt.RunID)
		m.activeChatRunID = ""
		m.activityStatus = "idle"
		return m.refreshSessionInfoCmd()
	}

	// 普通 final 消息
	isLocal := m.isLocalRunID(evt.RunID)
	if isLocal {
		m.forgetLocalRunID(evt.RunID)
	}

	// 检查 stopReason
	stopReason := ""
	if record, ok := evt.Message.(map[string]interface{}); ok {
		if sr, ok := record["stopReason"].(string); ok {
			stopReason = sr
		}
	}

	finalText := m.streamAssembler.Finalize(evt.RunID, evt.Message, m.showThinking)
	m.chatLog.FinalizeAssistant(finalText, evt.RunID)
	m.noteFinalizedRun(evt.RunID)
	m.activeChatRunID = ""
	if stopReason == "error" {
		m.activityStatus = "error"
	} else {
		m.activityStatus = "idle"
	}

	return m.refreshSessionInfoCmd()
}

// ---------- Agent 事件处理 ----------

// handleAgentEvent 处理 agent 事件。
// TS 参考: tui-event-handlers.ts L179-244 — 差异 E-02
func (m *Model) handleAgentEvent(payload interface{}) tea.Cmd {
	if payload == nil {
		return nil
	}
	record, ok := payload.(map[string]interface{})
	if !ok {
		return nil
	}

	evt := parseAgentEvent(record)
	m.syncSessionKey()

	// 过滤非活跃/非已知 run
	isActiveRun := evt.RunID == m.activeChatRunID
	_, isSessionRun := m.sessionRuns[evt.RunID]
	_, isFinalizedRun := m.finalizedRuns[evt.RunID]
	isKnownRun := isActiveRun || isSessionRun || isFinalizedRun
	if !isKnownRun {
		return nil
	}

	switch evt.Stream {
	case "tool":
		return m.handleToolEvent(evt)
	case "lifecycle":
		if !isActiveRun {
			return nil
		}
		return m.handleLifecycleEvent(evt)
	}

	return nil
}

// handleToolEvent 处理 tool 流事件。
func (m *Model) handleToolEvent(evt AgentEvent) tea.Cmd {
	verbose := m.sessionInfo.VerboseLevel
	if verbose == "" {
		verbose = "off"
	}
	if verbose == "off" {
		return nil
	}
	allowToolOutput := verbose == "full"

	data := evt.Data
	if data == nil {
		data = make(map[string]interface{})
	}

	phase := AsString(data["phase"], "")
	toolCallID := AsString(data["toolCallId"], "")
	toolName := AsString(data["name"], "tool")

	if toolCallID == "" {
		return nil
	}

	switch phase {
	case "start":
		m.chatLog.StartTool(toolCallID, toolName, data["args"])
	case "update":
		if !allowToolOutput {
			return nil
		}
		m.chatLog.UpdateToolResult(toolCallID, data["partialResult"], ToolResultOpts{Partial: true})
	case "result":
		isError := false
		if ie, ok := data["isError"].(bool); ok {
			isError = ie
		}
		if allowToolOutput {
			m.chatLog.UpdateToolResult(toolCallID, data["result"], ToolResultOpts{IsError: isError})
		} else {
			// 无输出权限时传空 content
			emptyResult := map[string]interface{}{"content": []interface{}{}}
			m.chatLog.UpdateToolResult(toolCallID, emptyResult, ToolResultOpts{IsError: isError})
		}
	}

	return nil
}

// handleLifecycleEvent 处理 lifecycle 流事件。
func (m *Model) handleLifecycleEvent(evt AgentEvent) tea.Cmd {
	data := evt.Data
	if data == nil {
		return nil
	}
	phase, _ := data["phase"].(string)

	switch phase {
	case "start":
		m.activityStatus = "running"
	case "end":
		m.activityStatus = "idle"
	case "error":
		m.activityStatus = "error"
	}

	return nil
}

// ---------- 解析辅助 ----------

// parseChatEvent 从 map 解析 ChatEvent。
func parseChatEvent(record map[string]interface{}) ChatEvent {
	evt := ChatEvent{}
	if v, ok := record["sessionKey"].(string); ok {
		evt.SessionKey = v
	}
	if v, ok := record["runId"].(string); ok {
		evt.RunID = v
	}
	if v, ok := record["state"].(string); ok {
		evt.State = v
	}
	evt.Message = record["message"]
	if v, ok := record["errorMessage"].(string); ok {
		evt.ErrorMessage = v
	}
	return evt
}

// parseAgentEvent 从 map 解析 AgentEvent。
func parseAgentEvent(record map[string]interface{}) AgentEvent {
	evt := AgentEvent{}
	if v, ok := record["runId"].(string); ok {
		evt.RunID = v
	}
	if v, ok := record["stream"].(string); ok {
		evt.Stream = v
	}
	if v, ok := record["data"].(map[string]interface{}); ok {
		evt.Data = v
	}
	return evt
}

// ---------- Session Info 刷新 ----------

// SessionInfoRefreshMsg session info 刷新结果。
type SessionInfoRefreshMsg struct {
	Info *SessionInfo
	Err  error
}

// refreshSessionInfoCmd 异步刷新 session info（token 计数等）。
// TS 参考: tui-event-handlers.ts L148,156,169 refreshSessionInfo
func (m *Model) refreshSessionInfoCmd() tea.Cmd {
	sessionKey := m.currentSessionKey
	return func() tea.Msg {
		raw, err := m.client.LoadHistory(sessionKey, 0)
		if err != nil {
			return SessionInfoRefreshMsg{Err: err}
		}
		// 尝试从 history response 中提取 session info
		if rawMap, ok := raw.(map[string]interface{}); ok {
			if sessionRaw, ok := rawMap["session"]; ok {
				data, _ := json.Marshal(sessionRaw)
				var info SessionInfo
				if json.Unmarshal(data, &info) == nil {
					return SessionInfoRefreshMsg{Info: &info}
				}
			}
		}
		return SessionInfoRefreshMsg{}
	}
}
