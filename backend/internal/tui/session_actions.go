// session_actions.go — TUI 会话管理操作
//
// 对齐 TS: src/tui/tui-session-actions.ts(413L)
// 差异 S-01/S-02/S-03: agents 管理 + model 优先级 + history 加载。
//
// W5 产出文件 #1。
package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Acosmi/ClawAcosmi/internal/routing"
)

// ---------- 消息类型 ----------

// AgentsResultMsg agents 列表异步结果。
type AgentsResultMsg struct {
	Result *GatewayAgentsList
	Err    error
}

// SessionInfoResultMsg session info 刷新结果（增强版，替代 SessionInfoRefreshMsg）。
type SessionInfoResultMsg struct {
	Sessions *GatewaySessionList
	Err      error
}

// HistoryResultMsg 历史加载结果。
type HistoryResultMsg struct {
	Record map[string]interface{}
	Err    error
}

// SessionDefaults 会话默认值。
type SessionDefaults struct {
	Model         string
	ModelProvider string
	ContextTokens *int
}

// ---------- 串行化用 mutex ----------

var sessionRefreshMu sync.Mutex

// ---------- applyAgentsResult（差异 S-01）----------

// applyAgentsResult 应用 agents 列表结果到 Model。
// TS 参考: tui-session-actions.ts L73-107
func (m *Model) applyAgentsResult(result *GatewayAgentsList) {
	if result == nil {
		return
	}
	m.agentDefaultID = routing.NormalizeAgentID(result.DefaultID)
	m.sessionMainKey = routing.NormalizeMainKey(result.MainKey)
	if result.Scope != "" {
		m.sessionScope = SessionScope(result.Scope)
	}

	// 更新 agents 列表
	m.agents = nil
	for _, a := range result.Agents {
		m.agents = append(m.agents, AgentSummary{
			ID:   routing.NormalizeAgentID(a.ID),
			Name: strings.TrimSpace(a.Name),
		})
	}

	// 更新 agentNames map
	m.agentNames = make(map[string]string)
	for _, a := range m.agents {
		if a.Name != "" {
			m.agentNames[a.ID] = a.Name
		}
	}

	// 初始 session 应用
	if !m.initialSessionApplied {
		if m.initialSessionAgentID != "" {
			if m.agentExists(m.initialSessionAgentID) {
				m.currentAgentID = m.initialSessionAgentID
			}
		} else if !m.agentExists(m.currentAgentID) {
			m.currentAgentID = m.firstAgentID(result.DefaultID)
		}
		nextKey := m.resolveSessionKey(m.initialSessionInput)
		if nextKey != m.currentSessionKey {
			m.currentSessionKey = nextKey
		}
		m.initialSessionApplied = true
	} else if !m.agentExists(m.currentAgentID) {
		m.currentAgentID = m.firstAgentID(result.DefaultID)
	}
}

// agentExists 检查 agent 是否存在。
func (m *Model) agentExists(id string) bool {
	for _, a := range m.agents {
		if a.ID == id {
			return true
		}
	}
	return false
}

// firstAgentID 返回第一个 agent ID，或 fallback。
func (m *Model) firstAgentID(fallback string) string {
	if len(m.agents) > 0 {
		return m.agents[0].ID
	}
	return routing.NormalizeAgentID(fallback)
}

// ---------- refreshAgents ----------

// refreshAgentsCmd 异步刷新 agents 列表。
// TS 参考: tui-session-actions.ts L109-116
func (m *Model) refreshAgentsCmd() tea.Cmd {
	return func() tea.Msg {
		result, err := m.client.ListAgents()
		return AgentsResultMsg{Result: result, Err: err}
	}
}

// ---------- updateAgentFromSessionKey ----------

// updateAgentFromSessionKey 从 session key 解析并更新 agent。
// TS 参考: tui-session-actions.ts L118-127
func (m *Model) updateAgentFromSessionKey(key string) {
	parsed := routing.ParseAgentSessionKey(key)
	if parsed == nil {
		return
	}
	next := routing.NormalizeAgentID(parsed.AgentID)
	if next != m.currentAgentID {
		m.currentAgentID = next
	}
}

// ---------- resolveModelSelection（差异 S-02）----------

// resolveModelSelection 解析 model 选择，3 层优先级。
// TS 参考: tui-session-actions.ts L129-145
func (m *Model) resolveModelSelection(entry *GatewaySessionEntry) (provider, model string) {
	if entry != nil && (entry.ModelProvider != "" || entry.Model != "") {
		provider = entry.ModelProvider
		if provider == "" {
			provider = m.sessionInfo.ModelProvider
		}
		model = entry.Model
		if model == "" {
			model = m.sessionInfo.Model
		}
		return
	}
	// modelOverride 层（GatewaySessionEntry 无此字段，跳过）
	return m.sessionInfo.ModelProvider, m.sessionInfo.Model
}

// ---------- applySessionInfo（差异 S-02）----------

// applySessionInfo 应用 session info 更新。
// TS 参考: tui-session-actions.ts L147-226
func (m *Model) applySessionInfo(entry *GatewaySessionEntry, defaults *SessionDefaults, force bool) {
	if entry == nil && defaults == nil && !force {
		return
	}

	// updatedAt 幂等检查
	if !force && entry != nil && entry.UpdatedAt != nil && m.sessionInfo.UpdatedAt != nil {
		if *entry.UpdatedAt < *m.sessionInfo.UpdatedAt {
			// defaults 或 model 变更时仍需更新
			hasModelChange := (entry.ModelProvider != "" && entry.ModelProvider != m.sessionInfo.ModelProvider) ||
				(entry.Model != "" && entry.Model != m.sessionInfo.Model)
			if !hasModelChange {
				return
			}
		}
	}

	next := m.sessionInfo

	if entry != nil {
		if entry.ThinkingLevel != "" {
			next.ThinkingLevel = entry.ThinkingLevel
		}
		if entry.VerboseLevel != "" {
			next.VerboseLevel = entry.VerboseLevel
		}
		if entry.ReasoningLevel != "" {
			next.ReasoningLevel = entry.ReasoningLevel
		}
		if entry.ResponseUsage != "" {
			next.ResponseUsage = entry.ResponseUsage
		}
		if entry.InputTokens != nil {
			next.InputTokens = entry.InputTokens
		}
		if entry.OutputTokens != nil {
			next.OutputTokens = entry.OutputTokens
		}
		if entry.TotalTokens != nil {
			next.TotalTokens = entry.TotalTokens
		}
		if entry.ContextTokens != nil {
			next.ContextTokens = entry.ContextTokens
		} else if defaults != nil && defaults.ContextTokens != nil {
			next.ContextTokens = defaults.ContextTokens
		}
		if entry.DisplayName != "" {
			next.DisplayName = entry.DisplayName
		}
		if entry.UpdatedAt != nil {
			next.UpdatedAt = entry.UpdatedAt
		}
	}

	// model 优先级
	provider, model := m.resolveModelSelection(entry)
	if provider != "" {
		next.ModelProvider = provider
	}
	if model != "" {
		next.Model = model
	}

	m.sessionInfo = next
}

// ---------- refreshSessionInfo（增强版）----------

// refreshSessionInfoFull 完整的 session info 刷新。
// 替代 event_handlers.go 中的简化版 refreshSessionInfoCmd。
// TS 参考: tui-session-actions.ts L228-265
func (m *Model) refreshSessionInfoFull() tea.Cmd {
	sessionKey := m.currentSessionKey
	agentID := m.currentAgentID
	return func() tea.Msg {
		sessionRefreshMu.Lock()
		defer sessionRefreshMu.Unlock()

		// 解析 listAgentId
		listAgentID := agentID
		if sessionKey == "global" || sessionKey == "unknown" {
			listAgentID = ""
		} else {
			parsed := routing.ParseAgentSessionKey(sessionKey)
			if parsed != nil && parsed.AgentID != "" {
				listAgentID = routing.NormalizeAgentID(parsed.AgentID)
			}
		}

		falseVal := false
		params := &SessionsListParams{
			IncludeGlobal:  &falseVal,
			IncludeUnknown: &falseVal,
		}
		if listAgentID != "" {
			params.AgentID = listAgentID
		}

		result, err := m.client.ListSessions(params)
		return SessionInfoResultMsg{Sessions: result, Err: err}
	}
}

// handleSessionInfoResult 处理 session info 刷新结果。
func (m *Model) handleSessionInfoResult(msg SessionInfoResultMsg) {
	if msg.Err != nil {
		m.chatLog.AddSystem(fmt.Sprintf("sessions list failed: %s", msg.Err))
		return
	}
	if msg.Sessions == nil {
		return
	}

	// 查找匹配当前 session key 的条目
	normalizeMatchKey := func(key string) string {
		parsed := routing.ParseAgentSessionKey(key)
		if parsed != nil {
			return parsed.Rest
		}
		return key
	}
	currentMatchKey := normalizeMatchKey(m.currentSessionKey)
	var matched *GatewaySessionEntry
	for i := range msg.Sessions.Sessions {
		row := &msg.Sessions.Sessions[i]
		if row.Key == m.currentSessionKey {
			matched = row
			break
		}
		if normalizeMatchKey(row.Key) == currentMatchKey {
			matched = row
			break
		}
	}

	// 更新 session key（如果匹配到更规范的 key）
	if matched != nil && matched.Key != "" && matched.Key != m.currentSessionKey {
		m.updateAgentFromSessionKey(matched.Key)
		m.currentSessionKey = matched.Key
	}

	// 构建 defaults
	var defaults *SessionDefaults
	if msg.Sessions.Defaults != nil {
		d := msg.Sessions.Defaults
		defaults = &SessionDefaults{}
		if d.Model != nil {
			defaults.Model = *d.Model
		}
		if d.ModelProvider != nil {
			defaults.ModelProvider = *d.ModelProvider
		}
		defaults.ContextTokens = d.ContextTokens
	}

	m.applySessionInfo(matched, defaults, false)
}

// ---------- applySessionInfoFromPatch ----------

// applySessionInfoFromPatch 从 patch 结果局部更新 session info。
// TS 参考: tui-session-actions.ts L275-294
func (m *Model) applySessionInfoFromPatch(result *SessionsPatchResult) {
	if result == nil || result.Payload == nil {
		return
	}
	// 从 payload 中解析 entry
	data, err := json.Marshal(result.Payload)
	if err != nil {
		return
	}
	var parsed struct {
		Key      string               `json:"key,omitempty"`
		Entry    *GatewaySessionEntry `json:"entry,omitempty"`
		Resolved *struct {
			ModelProvider string `json:"modelProvider,omitempty"`
			Model         string `json:"model,omitempty"`
		} `json:"resolved,omitempty"`
	}
	if json.Unmarshal(data, &parsed) != nil {
		return
	}
	if parsed.Entry == nil {
		return
	}

	if parsed.Key != "" && parsed.Key != m.currentSessionKey {
		m.updateAgentFromSessionKey(parsed.Key)
		m.currentSessionKey = parsed.Key
	}

	// 合并 resolved model
	entry := parsed.Entry
	if parsed.Resolved != nil && (parsed.Resolved.ModelProvider != "" || parsed.Resolved.Model != "") {
		if parsed.Resolved.ModelProvider != "" {
			entry.ModelProvider = parsed.Resolved.ModelProvider
		}
		if parsed.Resolved.Model != "" {
			entry.Model = parsed.Resolved.Model
		}
	}

	m.applySessionInfo(entry, nil, true)
}

// ---------- loadHistory（差异 S-03）----------

// loadHistoryCmd 异步加载聊天历史。
// TS 参考: tui-session-actions.ts L296-369
func (m *Model) loadHistoryCmd() tea.Cmd {
	sessionKey := m.currentSessionKey
	limit := m.opts.HistoryLimit
	if limit <= 0 {
		limit = 200
	}
	return func() tea.Msg {
		raw, err := m.client.LoadHistory(sessionKey, limit)
		if err != nil {
			return HistoryResultMsg{Err: err}
		}
		if rawMap, ok := raw.(map[string]interface{}); ok {
			return HistoryResultMsg{Record: rawMap}
		}
		// 尝试 json.RawMessage → map
		if rawBytes, ok := raw.(json.RawMessage); ok {
			var m map[string]interface{}
			if json.Unmarshal(rawBytes, &m) == nil {
				return HistoryResultMsg{Record: m}
			}
		}
		return HistoryResultMsg{Record: make(map[string]interface{})}
	}
}

// handleHistoryResult 处理历史加载结果。
func (m *Model) handleHistoryResult(msg HistoryResultMsg) tea.Cmd {
	if msg.Err != nil {
		m.chatLog.AddSystem(fmt.Sprintf("history failed: %s", msg.Err))
		return nil
	}

	record := msg.Record
	// 提取 sessionId
	if sid, ok := record["sessionId"].(string); ok {
		m.currentSessionID = sid
	}
	// 提取 thinkingLevel/verboseLevel
	if tl, ok := record["thinkingLevel"].(string); ok && tl != "" {
		m.sessionInfo.ThinkingLevel = tl
	}
	if vl, ok := record["verboseLevel"].(string); ok && vl != "" {
		m.sessionInfo.VerboseLevel = vl
	}

	showTools := m.sessionInfo.VerboseLevel != "" && m.sessionInfo.VerboseLevel != "off"

	// 清空并重建
	m.chatLog.ClearAll()
	m.chatLog.AddSystem(fmt.Sprintf("session %s", m.currentSessionKey))

	// 遍历 messages
	messages, _ := record["messages"].([]interface{})
	for _, entry := range messages {
		message, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}

		// command 消息
		if IsCommandMessage(message) {
			text := ExtractTextFromMessage(message, false)
			if text != "" {
				m.chatLog.AddSystem(text)
			}
			continue
		}

		role, _ := message["role"].(string)
		switch role {
		case "user":
			text := ExtractTextFromMessage(message, false)
			if text != "" {
				m.chatLog.AddUser(text)
			}
		case "assistant":
			text := ExtractTextFromMessage(message, m.showThinking)
			if text != "" {
				m.chatLog.FinalizeAssistant(text, "")
			}
		case "toolResult":
			if !showTools {
				continue
			}
			toolCallID := AsString(message["toolCallId"], "")
			toolName := AsString(message["toolName"], "tool")
			toolEntry := m.chatLog.StartTool(toolCallID, toolName, map[string]interface{}{})
			// 构建 result
			isError := false
			if ie, ok := message["isError"].(bool); ok {
				isError = ie
			}
			tr := castToToolResult(message)
			if toolEntry.view != nil {
				toolEntry.view.SetResult(tr, isError)
			}
			toolEntry.IsError = isError
		}
	}

	m.historyLoaded = true
	return m.refreshSessionInfoFull()
}

// ---------- setSession ----------

// setSessionCmd 切换 session。
// TS 参考: tui-session-actions.ts L371-382
func (m *Model) setSessionCmd(rawKey string) tea.Cmd {
	nextKey := m.resolveSessionKey(rawKey)
	m.updateAgentFromSessionKey(nextKey)
	m.currentSessionKey = nextKey
	m.activeChatRunID = ""
	m.currentSessionID = ""
	m.historyLoaded = false
	m.clearLocalRunIDs()
	return m.loadHistoryCmd()
}
