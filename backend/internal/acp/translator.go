package acp

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/google/uuid"
)

// ---------- AcpGatewayAgent ----------

// AcpGatewayAgent ACP ⇔ Gateway 协议翻译器。
// 实现 AcpServerHandler 接口。
// 对应 TS: acp/translator.ts AcpGatewayAgent
type AcpGatewayAgent struct {
	conn    *AgentSideConnection
	gateway GatewayRequester
	opts    *AcpServerOptions
	store   AcpSessionStore
	verbose bool

	// pendingPrompts 管理待处理的 prompt 请求（sessionID → promptCtx）。
	pendingMu      sync.Mutex
	pendingPrompts map[string]*pendingPrompt
}

// pendingPrompt 表示一个待处理的 prompt 请求。
type pendingPrompt struct {
	sessionID      string
	sessionKey     string
	idempotencyKey string
	resultCh       chan promptResult
	cancel         context.CancelFunc

	// mu 保护可变字段 sentTextLength 和 toolCalls 的并发访问。
	// findPendingBySessionKey() 返回指针后，多个事件回调 goroutine 可能并发修改这些字段。
	mu             sync.Mutex
	sentTextLength int
	toolCalls      map[string]struct{} // 工具调用去重 Set
}

// promptResult prompt 请求的结果。
type promptResult struct {
	stopReason StopReason
	err        error
}

// NewAcpGatewayAgent 创建 ACP Gateway Agent 翻译器。
func NewAcpGatewayAgent(conn *AgentSideConnection, gateway GatewayRequester, opts *AcpServerOptions) *AcpGatewayAgent {
	return &AcpGatewayAgent{
		conn:           conn,
		gateway:        gateway,
		opts:           opts,
		store:          DefaultSessionStore,
		verbose:        opts.Verbose,
		pendingPrompts: make(map[string]*pendingPrompt),
	}
}

// ---------- AcpServerHandler 接口实现 ----------

// Start 启动 Agent（初始化后调用）。
func (a *AcpGatewayAgent) Start() {
	if a.verbose {
		log.Println("[acp-translator] agent started")
	}
}

// HandleGatewayReconnect Gateway 重连后调用。
func (a *AcpGatewayAgent) HandleGatewayReconnect() {
	if a.verbose {
		log.Println("[acp-translator] gateway reconnected")
	}
}

// HandleGatewayDisconnect Gateway 断连时调用，reject 所有 pending prompt。
// 修复: 先收集再解锁再发送 — 持锁发送 channel 可能永久阻塞 (已知 Go 并发反模式)。
func (a *AcpGatewayAgent) HandleGatewayDisconnect(reason string) {
	if a.verbose {
		log.Printf("[acp-translator] gateway disconnected: %s", reason)
	}

	// 1. 持锁收集并清理 map
	a.pendingMu.Lock()
	collected := make([]*pendingPrompt, 0, len(a.pendingPrompts))
	for sid, pp := range a.pendingPrompts {
		collected = append(collected, pp)
		a.store.CancelActiveRun(sid)
		delete(a.pendingPrompts, sid)
	}
	a.pendingMu.Unlock()

	// 2. 解锁后发送 channel (避免持锁阻塞)
	for _, pp := range collected {
		pp.resultCh <- promptResult{
			err: fmt.Errorf("Gateway disconnected: %s", reason),
		}
	}
}

// Initialize 处理初始化请求，返回 agent 能力声明。
func (a *AcpGatewayAgent) Initialize(_ InitializeRequest) (*InitializeResponse, error) {
	agentInfo := ACPAgentInfo
	return &InitializeResponse{
		ProtocolVersion: ACPProtocolVersion,
		AgentCapabilities: &AgentCapabilities{
			LoadSession: true,
			PromptCapabilities: &PromptCapabilities{
				Image:           true,
				EmbeddedContext: true,
			},
			SessionCapabilities: &SessionCapabilities{
				List: struct{}{},
			},
		},
		AgentInfo:   &agentInfo,
		AuthMethods: []interface{}{},
	}, nil
}

// NewSession 创建新 ACP 会话。
func (a *AcpGatewayAgent) NewSession(req NewSessionRequest) (*NewSessionResponse, error) {
	meta := ParseSessionMeta(req.Meta)

	sessionKey, err := ResolveSessionKey(ResolveSessionKeyParams{
		Meta:        meta,
		FallbackKey: "acp:main",
		Gateway:     a.gateway,
		Opts:        a.opts,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve session key: %w", err)
	}

	// 重置 session（如果需要）
	if err := ResetSessionIfNeeded(a.gateway, sessionKey, meta, a.opts); err != nil {
		if a.verbose {
			log.Printf("[acp-translator] session reset warning: %v", err)
		}
	}

	cwd := req.Cwd
	if cwd == "" {
		cwd = "."
	}

	session := a.store.CreateSession(CreateSessionOpts{
		SessionKey: sessionKey,
		Cwd:        cwd,
	})

	if a.verbose {
		log.Printf("[acp-translator] new session: %s (key=%s)", session.SessionID, sessionKey)
	}

	return &NewSessionResponse{
		SessionID: session.SessionID,
	}, nil
}

// LoadSession 加载已有 ACP 会话。
// 对应 TS: translator.ts loadSession() — 重新解析 sessionKey + reset。
func (a *AcpGatewayAgent) LoadSession(req LoadSessionRequest) (*LoadSessionResponse, error) {
	meta := ParseSessionMeta(req.Meta)

	sessionKey, err := ResolveSessionKey(ResolveSessionKeyParams{
		Meta:        meta,
		FallbackKey: req.SessionID,
		Gateway:     a.gateway,
		Opts:        a.opts,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve session key: %w", err)
	}

	if err := ResetSessionIfNeeded(a.gateway, sessionKey, meta, a.opts); err != nil {
		if a.verbose {
			log.Printf("[acp-translator] session reset warning: %v", err)
		}
	}

	cwd := req.Cwd
	if cwd == "" {
		cwd = "."
	}

	session := a.store.CreateSession(CreateSessionOpts{
		SessionKey: sessionKey,
		Cwd:        cwd,
	})

	if a.verbose {
		log.Printf("[acp-translator] loaded session: %s -> %s", session.SessionID, sessionKey)
	}

	a.sendCommandsUpdate(session.SessionID)
	return &LoadSessionResponse{}, nil
}

// ListSessions 列出会话（代理到 Gateway sessions.list）。
// ACP-P2-2: 从 req.Meta 读取 limit 参数，默认 100。
func (a *AcpGatewayAgent) ListSessions(req ListSessionsRequest) (*ListSessionsResponse, error) {
	// 从 _meta.limit 读取分页限制，默认 100
	limit := 100
	if v := ReadNumber(req.Meta, []string{"limit"}); v != nil && int(*v) > 0 {
		limit = int(*v)
	}

	// 尝试从 Gateway 获取会话列表
	var result struct {
		Sessions []struct {
			Key          string `json:"key"`
			Label        string `json:"label,omitempty"`
			DerivedTitle string `json:"derivedTitle,omitempty"`
			UpdatedAt    *int64 `json:"updatedAt"`
		} `json:"sessions"`
	}

	err := a.gateway.Request("sessions.list", map[string]interface{}{
		"includeDerivedTitles": true,
		"limit":                limit,
	}, &result)
	if err != nil {
		if a.verbose {
			log.Printf("[acp-translator] sessions.list error: %v", err)
		}
		// 返回空列表而不是错误
		return &ListSessionsResponse{
			Sessions:   []ListSessionEntry{},
			NextCursor: nil,
		}, nil
	}

	entries := make([]ListSessionEntry, 0, len(result.Sessions))
	for _, s := range result.Sessions {
		title := s.DerivedTitle
		if title == "" {
			title = s.Label
		}
		entry := ListSessionEntry{
			SessionID: s.Key,
			Title:     title,
		}
		if s.UpdatedAt != nil {
			entry.UpdatedAt = fmt.Sprintf("%d", *s.UpdatedAt)
		}
		entries = append(entries, entry)
	}

	return &ListSessionsResponse{
		Sessions:   entries,
		NextCursor: nil,
	}, nil
}

// Prompt 发送提示词到 Gateway 并等待结果。
func (a *AcpGatewayAgent) Prompt(ctx context.Context, req PromptRequest) (*PromptResponse, error) {
	session := a.store.GetSession(req.SessionID)
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", req.SessionID)
	}

	// 如果有前一个活跃 run，先取消
	a.store.CancelActiveRun(req.SessionID)

	// 提取文本和附件
	promptText := ExtractTextFromPrompt(req.Prompt)
	attachments := ExtractAttachmentsFromPrompt(req.Prompt)

	if promptText == "" {
		return nil, fmt.Errorf("empty prompt text")
	}

	// P0-1: prefixCwd 逻辑
	meta := ParseSessionMeta(req.Meta)
	prefixCwd := true // 默认为 true
	if meta.PrefixCwd != nil {
		prefixCwd = *meta.PrefixCwd
	} else if a.opts != nil {
		prefixCwd = a.opts.PrefixCwdEnabled()
	}
	message := promptText
	if prefixCwd {
		message = fmt.Sprintf("[Working directory: %s]\n\n%s", session.Cwd, promptText)
	}

	// P0-4: idempotencyKey
	runID := uuid.New().String()

	// 创建可取消的 context
	promptCtx, promptCancel := context.WithCancel(ctx)

	// 注册活跃 run
	a.store.SetActiveRun(req.SessionID, runID, promptCancel)

	// P0-5: pending 按 sessionId 索引
	resultCh := make(chan promptResult, 1)
	pp := &pendingPrompt{
		sessionID:      req.SessionID,
		sessionKey:     session.SessionKey,
		idempotencyKey: runID,
		resultCh:       resultCh,
		cancel:         promptCancel,
	}
	a.pendingMu.Lock()
	a.pendingPrompts[req.SessionID] = pp
	a.pendingMu.Unlock()

	// 向 Gateway 发送 chat.send
	sendParams := map[string]interface{}{
		"sessionKey":     session.SessionKey,
		"message":        message,
		"idempotencyKey": runID,
	}
	if len(attachments) > 0 {
		sendParams["attachments"] = attachments
	}

	// 从 meta 读取可选参数
	if req.Meta != nil {
		if v := ReadString(req.Meta, []string{"thinking", "thinkingLevel"}); v != "" {
			sendParams["thinking"] = v
		}
		if v := ReadBool(req.Meta, []string{"deliver"}); v != nil {
			sendParams["deliver"] = *v
		}
		if v := ReadNumber(req.Meta, []string{"timeoutMs"}); v != nil {
			sendParams["timeoutMs"] = *v
		}
	}

	var sendResult map[string]interface{}
	if err := a.gateway.Request("chat.send", sendParams, &sendResult); err != nil {
		a.pendingMu.Lock()
		delete(a.pendingPrompts, req.SessionID)
		a.pendingMu.Unlock()
		a.store.CancelActiveRun(req.SessionID)
		promptCancel()
		return nil, fmt.Errorf("gateway chat.send: %w", err)
	}

	// 发送可用命令更新
	a.sendCommandsUpdate(session.SessionID)

	// 等待结果
	select {
	case result := <-resultCh:
		if result.err != nil {
			return nil, result.err
		}
		return &PromptResponse{
			StopReason: result.stopReason,
		}, nil
	case <-promptCtx.Done():
		return &PromptResponse{
			StopReason: StopReasonCancelled,
		}, nil
	}
}

// Cancel 取消活跃运行。
func (a *AcpGatewayAgent) Cancel(notif CancelNotification) {
	session := a.store.GetSession(notif.SessionID)
	if session == nil {
		return
	}

	a.store.CancelActiveRun(notif.SessionID)

	// 向 Gateway 发送 abort
	var result map[string]interface{}
	_ = a.gateway.Request("chat.abort", map[string]interface{}{
		"sessionKey": session.SessionKey,
	}, &result)

	// resolve pending with cancelled
	a.pendingMu.Lock()
	pp, ok := a.pendingPrompts[notif.SessionID]
	if ok {
		delete(a.pendingPrompts, notif.SessionID)
	}
	a.pendingMu.Unlock()
	if ok && pp.resultCh != nil {
		pp.resultCh <- promptResult{stopReason: StopReasonCancelled}
	}

	if a.verbose {
		log.Printf("[acp-translator] cancelled session: %s", notif.SessionID)
	}
}

// SetSessionMode 设置会话模式。
// P0-7: 使用 thinkingLevel 而非 mode。
func (a *AcpGatewayAgent) SetSessionMode(req SetSessionModeRequest) (*SetSessionModeResponse, error) {
	session := a.store.GetSession(req.SessionID)
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", req.SessionID)
	}

	if req.ModeID != "" {
		var result map[string]interface{}
		err := a.gateway.Request("sessions.patch", map[string]interface{}{
			"key":           session.SessionKey,
			"thinkingLevel": req.ModeID,
		}, &result)
		if err != nil {
			if a.verbose {
				log.Printf("[acp-translator] setSessionMode error: %v", err)
			}
		}
	}

	return &SetSessionModeResponse{}, nil
}

// ---------- Gateway 事件处理 ----------

// HandleGatewayEvent 处理来自 Gateway 的 EventFrame。
// 对应 TS: translator.ts handleGatewayEvent(evt: EventFrame)
// P0-3: 使用 evt.event + payload.state/stream 路由，与 Gateway EventFrame 一致。
func (a *AcpGatewayAgent) HandleGatewayEvent(event string, payload map[string]interface{}) {
	switch event {
	case "chat":
		a.handleChatEvent(payload)
	case "agent":
		a.handleAgentEvent(payload)
	default:
		if a.verbose {
			log.Printf("[acp-translator] unhandled gateway event: %s", event)
		}
	}
}

// handleChatEvent 处理 chat 事件（按 payload.state 分发）。
func (a *AcpGatewayAgent) handleChatEvent(payload map[string]interface{}) {
	sessionKey, _ := payload["sessionKey"].(string)
	state, _ := payload["state"].(string)
	runID, _ := payload["runId"].(string)
	if sessionKey == "" || state == "" {
		return
	}

	pending := a.findPendingBySessionKey(sessionKey)
	if pending == nil {
		return
	}
	// P0-4: runId/idempotencyKey 匹配
	if runID != "" && pending.idempotencyKey != runID {
		return
	}

	switch state {
	case "delta":
		if messageData, ok := payload["message"].(map[string]interface{}); ok {
			a.handleDeltaEvent(pending, messageData)
		}
	case "final":
		a.finishPrompt(pending.sessionID, StopReasonEndTurn, nil)
	case "aborted":
		a.finishPrompt(pending.sessionID, StopReasonCancelled, nil)
	case "error":
		// P1-4: chat.error → refusal
		a.finishPrompt(pending.sessionID, StopReasonRefusal, nil)
	}
}

// handleDeltaEvent 处理增量文本更新。
// P0-2: 使用增量切片（追踪 sentTextLength），与 TS 一致。
func (a *AcpGatewayAgent) handleDeltaEvent(pending *pendingPrompt, messageData map[string]interface{}) {
	// Gateway delta 发累积全文，需要切片出增量部分
	contentArr, ok := messageData["content"].([]interface{})
	if !ok {
		return
	}
	var fullText string
	for _, item := range contentArr {
		if m, ok := item.(map[string]interface{}); ok {
			if t, _ := m["type"].(string); t == "text" {
				if text, ok := m["text"].(string); ok {
					fullText = text
					break
				}
			}
		}
	}

	pending.mu.Lock()
	sentSoFar := pending.sentTextLength
	if len(fullText) <= sentSoFar {
		pending.mu.Unlock()
		return
	}
	newText := fullText[sentSoFar:]
	pending.sentTextLength = len(fullText)
	pending.mu.Unlock()

	a.conn.SendSessionUpdate(SessionNotification{
		SessionID: pending.sessionID,
		Update: SessionUpdate{
			SessionUpdate: "agent_message_chunk",
			Content: &ContentBlock{
				Type: "text",
				Text: newText,
			},
		},
	})
}

// handleAgentEvent 处理 agent 事件（按 payload.stream + data.phase 分发）。
func (a *AcpGatewayAgent) handleAgentEvent(payload map[string]interface{}) {
	stream, _ := payload["stream"].(string)
	data, ok := payload["data"].(map[string]interface{})
	sessionKey, _ := payload["sessionKey"].(string)
	if stream != "tool" || !ok || sessionKey == "" {
		return
	}

	toolCallID, _ := data["toolCallId"].(string)
	if toolCallID == "" {
		return
	}

	pending := a.findPendingBySessionKey(sessionKey)
	if pending == nil {
		return
	}

	phase, _ := data["phase"].(string)
	name, _ := data["name"].(string)

	switch phase {
	case "start":
		// P1-8: 工具调用去重
		pending.mu.Lock()
		if pending.toolCalls == nil {
			pending.toolCalls = make(map[string]struct{})
		}
		if _, exists := pending.toolCalls[toolCallID]; exists {
			pending.mu.Unlock()
			return
		}
		pending.toolCalls[toolCallID] = struct{}{}
		pending.mu.Unlock()

		args, _ := data["args"].(map[string]interface{})
		a.conn.SendSessionUpdate(SessionNotification{
			SessionID: pending.sessionID,
			Update: SessionUpdate{
				SessionUpdate: "tool_call",
				ToolCallID:    toolCallID,
				ToolTitle:     FormatToolTitle(name, args),
				Status:        "in_progress",
				RawInput:      args,
				Kind:          InferToolKind(name),
			},
		})

	case "result":
		isError := false
		if v, ok := data["isError"].(bool); ok {
			isError = v
		}
		status := "completed"
		if isError {
			status = "failed"
		}
		// P1-5: tool result 使用 tool_call_update
		a.conn.SendSessionUpdate(SessionNotification{
			SessionID: pending.sessionID,
			Update: SessionUpdate{
				SessionUpdate: "tool_call_update",
				ToolCallID:    toolCallID,
				Status:        status,
				RawOutput:     data["result"],
			},
		})
	}
}

// ---------- 辅助方法 ----------

// findPendingBySessionKey 按 session key 查找 pending prompt。
// P0-5: pending 按 sessionId 索引，需要遍历。
func (a *AcpGatewayAgent) findPendingBySessionKey(sessionKey string) *pendingPrompt {
	a.pendingMu.Lock()
	defer a.pendingMu.Unlock()
	for _, pp := range a.pendingPrompts {
		if pp.sessionKey == sessionKey {
			return pp
		}
	}
	return nil
}

// sendCommandsUpdate 发送可用命令更新。
func (a *AcpGatewayAgent) sendCommandsUpdate(sessionID string) {
	commands := GetAvailableCommands()
	a.conn.SendSessionUpdate(SessionNotification{
		SessionID: sessionID,
		Update: SessionUpdate{
			SessionUpdate:     "available_commands_update",
			AvailableCommands: commands,
		},
	})
}

// finishPrompt 完成 prompt 请求，清理状态。
func (a *AcpGatewayAgent) finishPrompt(sessionID string, stopReason StopReason, err error) {
	a.pendingMu.Lock()
	pp, ok := a.pendingPrompts[sessionID]
	if ok {
		delete(a.pendingPrompts, sessionID)
	}
	a.pendingMu.Unlock()

	a.store.CancelActiveRun(sessionID)

	if ok && pp.resultCh != nil {
		pp.resultCh <- promptResult{
			stopReason: stopReason,
			err:        err,
		}
	}
}
