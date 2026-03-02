// gateway_ws.go — TUI 专用 Gateway WebSocket 客户端
//
// 对齐 TS: src/tui/gateway-chat.ts(267L) + src/gateway/call.ts(313L)
// 在低层 gateway.GatewayWsClient 之上构建 RPC 请求/响应层，
// 提供 TUI 所需的 SendChat / AbortChat / ListSessions 等方法。
//
// DEP-01 修复: resolveGatewayConnection() 实现完整 6 层 token/password fallback。
package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/openacosmi/claw-acismi/internal/config"
	"github.com/openacosmi/claw-acismi/internal/gateway"
	"github.com/openacosmi/claw-acismi/pkg/types"
)

// ---------- 类型定义 ----------

// GatewayConnectionOptions Gateway 连接选项（对标 TS GatewayConnectionOptions）。
type GatewayConnectionOptions struct {
	URL      string
	Token    string
	Password string
}

// GatewayConnectionInfo 已解析的连接信息。
type GatewayConnectionInfo struct {
	URL      string
	Token    string
	Password string
}

// GatewayConfigSource 配置来源接口（TUI-D1 DI 注入）。
// 允许测试传入 mock 配置，避免依赖真实文件系统。
type GatewayConfigSource interface {
	LoadConfig() (*types.OpenAcosmiConfig, error)
}

// defaultConfigSource 默认实现（生产使用）。
type defaultConfigSource struct{}

func (d defaultConfigSource) LoadConfig() (*types.OpenAcosmiConfig, error) {
	return config.NewConfigLoader().LoadConfig()
}

// ChatSendOptions 发送聊天消息选项（对标 TS ChatSendOptions）。
type ChatSendOptions struct {
	SessionKey string
	Message    string
	Thinking   string
	Deliver    bool
	TimeoutMs  int
	RunID      string
}

// GatewaySessionList 会话列表响应（对标 TS GatewaySessionList）。
type GatewaySessionList struct {
	TS       int64  `json:"ts"`
	Path     string `json:"path"`
	Count    int    `json:"count"`
	Defaults *struct {
		Model         *string `json:"model,omitempty"`
		ModelProvider *string `json:"modelProvider,omitempty"`
		ContextTokens *int    `json:"contextTokens,omitempty"`
	} `json:"defaults,omitempty"`
	Sessions []GatewaySessionEntry `json:"sessions"`
}

// GatewaySessionEntry 会话条目。
type GatewaySessionEntry struct {
	Key                string `json:"key"`
	SessionID          string `json:"sessionId,omitempty"`
	UpdatedAt          *int64 `json:"updatedAt,omitempty"`
	ThinkingLevel      string `json:"thinkingLevel,omitempty"`
	VerboseLevel       string `json:"verboseLevel,omitempty"`
	ReasoningLevel     string `json:"reasoningLevel,omitempty"`
	SendPolicy         string `json:"sendPolicy,omitempty"`
	Model              string `json:"model,omitempty"`
	ContextTokens      *int   `json:"contextTokens,omitempty"`
	InputTokens        *int   `json:"inputTokens,omitempty"`
	OutputTokens       *int   `json:"outputTokens,omitempty"`
	TotalTokens        *int   `json:"totalTokens,omitempty"`
	ResponseUsage      string `json:"responseUsage,omitempty"`
	ModelProvider      string `json:"modelProvider,omitempty"`
	Label              string `json:"label,omitempty"`
	DisplayName        string `json:"displayName,omitempty"`
	DerivedTitle       string `json:"derivedTitle,omitempty"`
	LastMessagePreview string `json:"lastMessagePreview,omitempty"`
}

// GatewayAgentsList Agent 列表响应（对标 TS GatewayAgentsList）。
type GatewayAgentsList struct {
	DefaultID string `json:"defaultId"`
	MainKey   string `json:"mainKey"`
	Scope     string `json:"scope"` // "per-sender" | "global"
	Agents    []struct {
		ID   string `json:"id"`
		Name string `json:"name,omitempty"`
	} `json:"agents"`
}

// GatewayModelChoice 模型选择项。
type GatewayModelChoice struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Provider      string `json:"provider"`
	ContextWindow *int   `json:"contextWindow,omitempty"`
	Reasoning     *bool  `json:"reasoning,omitempty"`
}

// SessionsPatchParams 会话补丁参数。
type SessionsPatchParams struct {
	Key            string  `json:"key"`
	ThinkingLevel  *string `json:"thinkingLevel,omitempty"`
	VerboseLevel   *string `json:"verboseLevel,omitempty"`
	ReasoningLevel *string `json:"reasoningLevel,omitempty"`
	Model          *string `json:"model,omitempty"`
	ResponseUsage  *string `json:"responseUsage,omitempty"`
	SendPolicy     *string `json:"sendPolicy,omitempty"`
	Label          *string `json:"label,omitempty"`
	DisplayName    *string `json:"displayName,omitempty"`
}

// SessionsPatchResult 会话补丁结果。
type SessionsPatchResult struct {
	OK      bool        `json:"ok"`
	Payload interface{} `json:"payload,omitempty"`
}

// SessionsListParams 会话列表查询参数。
type SessionsListParams struct {
	Limit                *int   `json:"limit,omitempty"`
	ActiveMinutes        *int   `json:"activeMinutes,omitempty"`
	IncludeGlobal        *bool  `json:"includeGlobal,omitempty"`
	IncludeUnknown       *bool  `json:"includeUnknown,omitempty"`
	IncludeDerivedTitles *bool  `json:"includeDerivedTitles,omitempty"`
	IncludeLastMessage   *bool  `json:"includeLastMessage,omitempty"`
	AgentID              string `json:"agentId,omitempty"`
}

// ---------- RPC 请求/响应 ----------

// pendingRequest 挂起的 RPC 请求。
type pendingRequest struct {
	resultCh chan rpcResult
}

// rpcResult RPC 结果。
type rpcResult struct {
	Payload json.RawMessage
	Err     error
}

// ---------- GatewayChatClient ----------

// GatewayChatClient TUI 专用 Gateway 客户端。
// 在低层 GatewayWsClient 之上提供 RPC 层。
type GatewayChatClient struct {
	wsClient   *gateway.GatewayWsClient
	connection GatewayConnectionInfo
	hello      *gateway.HelloOk
	readyCh    chan struct{}

	// RPC 挂起请求
	mu      sync.Mutex
	pending map[string]*pendingRequest
	lastSeq *int64

	// 回调 — 由 Model 注册
	OnEvent        func(GatewayEventMsg)
	OnConnected    func()
	OnDisconnected func(reason string)
	OnGap          func(expected, received int64)
}

// NewGatewayChatClient 创建 TUI Gateway 客户端。
// 对标 TS GatewayChatClient constructor + resolveGatewayConnection。
func NewGatewayChatClient(opts GatewayConnectionOptions) *GatewayChatClient {
	return NewGatewayChatClientWithConfig(opts, defaultConfigSource{})
}

// NewGatewayChatClientWithConfig 创建 TUI Gateway 客户端（可注入配置来源）。
func NewGatewayChatClientWithConfig(opts GatewayConnectionOptions, cfgSrc GatewayConfigSource) *GatewayChatClient {
	resolved := resolveGatewayConnection(opts, cfgSrc)

	// TUI-2 修复: URL 覆盖时强制要求显式 auth。
	// TS 参考: gateway/call.ts ensureExplicitGatewayAuth (L70-90)
	urlOverride := strings.TrimSpace(opts.URL)
	if err := ensureExplicitGatewayAuth(urlOverride, resolved.Token, resolved.Password); err != nil {
		// 连接前报错 → 返回一个不可用客户端，Start() 时会触发 OnDisconnected
		c := &GatewayChatClient{
			connection: resolved,
			readyCh:    make(chan struct{}),
			pending:    make(map[string]*pendingRequest),
		}
		c.OnDisconnected = func(reason string) {}
		go func() {
			select {
			case <-c.readyCh:
			default:
				close(c.readyCh)
			}
		}()
		return c
	}

	c := &GatewayChatClient{
		connection: resolved,
		readyCh:    make(chan struct{}),
		pending:    make(map[string]*pendingRequest),
	}

	// 创建底层 WS 客户端
	// 差异 G-01: 传递完整的 12 字段 ConnectParams
	c.wsClient = gateway.NewGatewayWsClient(
		gateway.WsClientConfig{
			URL:      resolved.URL,
			Token:    resolved.Token,
			Password: resolved.Password,
			ConnectParams: gateway.ConnectParams{
				Role:   "operator",
				Scopes: []string{"operator.admin", "operator.approvals"},
			},
		},
		gateway.WsClientHandler{
			OnOpen: func() {
				// 连接建立后发送 connect 帧
				c.sendConnect()
			},
			OnClose: func(code int, reason string) {
				// 清空挂起请求
				c.flushPending(fmt.Errorf("gateway closed (%d): %s", code, reason))
				if c.OnDisconnected != nil {
					c.OnDisconnected(reason)
				}
			},
			OnError: func(err error) {
				// 日志记录
				_ = err
			},
			OnMessage: func(msgType int, data []byte) {
				c.handleMessage(data)
			},
		},
	)

	return c
}

// Connection 返回连接信息。
func (c *GatewayChatClient) Connection() GatewayConnectionInfo {
	return c.connection
}

// Start 启动连接。
func (c *GatewayChatClient) Start() {
	if err := c.wsClient.Connect(); err != nil {
		if c.OnDisconnected != nil {
			c.OnDisconnected(fmt.Sprintf("connect failed: %v", err))
		}
	}
}

// Stop 停止连接。
func (c *GatewayChatClient) Stop() {
	c.wsClient.Close()
}

// WaitForReady 等待 hello-ok 握手完成。
func (c *GatewayChatClient) WaitForReady() {
	<-c.readyCh
}

// ---------- RPC 方法 ----------

// SendChat 发送聊天消息。返回 runId。
func (c *GatewayChatClient) SendChat(opts ChatSendOptions) (string, error) {
	runID := opts.RunID
	if runID == "" {
		runID = uuid.New().String()
	}
	params := map[string]interface{}{
		"sessionKey":     opts.SessionKey,
		"message":        opts.Message,
		"idempotencyKey": runID,
	}
	if opts.Thinking != "" {
		params["thinking"] = opts.Thinking
	}
	if opts.Deliver {
		params["deliver"] = true
	}
	if opts.TimeoutMs > 0 {
		params["timeoutMs"] = opts.TimeoutMs
	}
	_, err := c.request("chat.send", params)
	if err != nil {
		return "", fmt.Errorf("chat.send: %w", err)
	}
	return runID, nil
}

// AbortChat 中止聊天。
func (c *GatewayChatClient) AbortChat(sessionKey, runID string) error {
	_, err := c.request("chat.abort", map[string]interface{}{
		"sessionKey": sessionKey,
		"runId":      runID,
	})
	if err != nil {
		return fmt.Errorf("chat.abort: %w", err)
	}
	return nil
}

// LoadHistory 加载聊天历史。
func (c *GatewayChatClient) LoadHistory(sessionKey string, limit int) (interface{}, error) {
	params := map[string]interface{}{
		"sessionKey": sessionKey,
	}
	if limit > 0 {
		params["limit"] = limit
	}
	result, err := c.request("chat.history", params)
	if err != nil {
		return nil, fmt.Errorf("chat.history: %w", err)
	}
	return result, nil
}

// ListSessions 列出会话。
func (c *GatewayChatClient) ListSessions(opts *SessionsListParams) (*GatewaySessionList, error) {
	params := map[string]interface{}{}
	if opts != nil {
		if opts.Limit != nil {
			params["limit"] = *opts.Limit
		}
		if opts.ActiveMinutes != nil {
			params["activeMinutes"] = *opts.ActiveMinutes
		}
		if opts.IncludeGlobal != nil {
			params["includeGlobal"] = *opts.IncludeGlobal
		}
		if opts.IncludeUnknown != nil {
			params["includeUnknown"] = *opts.IncludeUnknown
		}
		if opts.IncludeDerivedTitles != nil {
			params["includeDerivedTitles"] = *opts.IncludeDerivedTitles
		}
		if opts.IncludeLastMessage != nil {
			params["includeLastMessage"] = *opts.IncludeLastMessage
		}
		if opts.AgentID != "" {
			params["agentId"] = opts.AgentID
		}
	}
	raw, err := c.request("sessions.list", params)
	if err != nil {
		return nil, fmt.Errorf("sessions.list: %w", err)
	}
	var result GatewaySessionList
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("sessions.list decode: %w", err)
	}
	return &result, nil
}

// ListAgents 列出 Agent。
func (c *GatewayChatClient) ListAgents() (*GatewayAgentsList, error) {
	raw, err := c.request("agents.list", map[string]interface{}{})
	if err != nil {
		return nil, fmt.Errorf("agents.list: %w", err)
	}
	var result GatewayAgentsList
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("agents.list decode: %w", err)
	}
	return &result, nil
}

// PatchSession 补丁更新会话。
func (c *GatewayChatClient) PatchSession(opts SessionsPatchParams) (*SessionsPatchResult, error) {
	data, err := json.Marshal(opts)
	if err != nil {
		return nil, fmt.Errorf("sessions.patch marshal: %w", err)
	}
	var params map[string]interface{}
	if err := json.Unmarshal(data, &params); err != nil {
		return nil, fmt.Errorf("sessions.patch unmarshal: %w", err)
	}
	raw, err := c.request("sessions.patch", params)
	if err != nil {
		return nil, fmt.Errorf("sessions.patch: %w", err)
	}
	var result SessionsPatchResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("sessions.patch decode: %w", err)
	}
	return &result, nil
}

// ResetSession 重置会话。
func (c *GatewayChatClient) ResetSession(key string) error {
	_, err := c.request("sessions.reset", map[string]interface{}{"key": key})
	if err != nil {
		return fmt.Errorf("sessions.reset: %w", err)
	}
	return nil
}

// GetStatus 获取状态。
func (c *GatewayChatClient) GetStatus() (interface{}, error) {
	result, err := c.request("status", nil)
	if err != nil {
		return nil, fmt.Errorf("status: %w", err)
	}
	return result, nil
}

// ListModels 列出可用模型。
func (c *GatewayChatClient) ListModels() ([]GatewayModelChoice, error) {
	raw, err := c.request("models.list", nil)
	if err != nil {
		return nil, fmt.Errorf("models.list: %w", err)
	}
	var result struct {
		Models []GatewayModelChoice `json:"models"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("models.list decode: %w", err)
	}
	return result.Models, nil
}

// ---------- WS 协议层 ----------

// sendConnect 发送 connect 帧。
func (c *GatewayChatClient) sendConnect() {
	instanceID := uuid.New().String()
	connectFrame := map[string]interface{}{
		"type":        "connect",
		"minProtocol": gateway.ProtocolVersion,
		"maxProtocol": gateway.ProtocolVersion,
		"client": map[string]interface{}{
			"id":          "tui",
			"displayName": "openacosmi-tui",
			"version":     config.BuildVersion,
			"platform":    runtime.GOOS,
			"mode":        "ui",
			"instanceId":  instanceID,
		},
		"caps":   []string{"tool_events"},
		"role":   "operator",
		"scopes": []string{"operator.admin", "operator.approvals"},
	}
	if c.connection.Token != "" {
		connectFrame["auth"] = map[string]interface{}{
			"token": c.connection.Token,
		}
	}
	if c.connection.Password != "" {
		if auth, ok := connectFrame["auth"].(map[string]interface{}); ok {
			auth["password"] = c.connection.Password
		} else {
			connectFrame["auth"] = map[string]interface{}{
				"password": c.connection.Password,
			}
		}
	}
	_ = c.wsClient.SendJSON(connectFrame)
}

// handleMessage 处理收到的 WS 消息。
func (c *GatewayChatClient) handleMessage(data []byte) {
	var frame map[string]interface{}
	if err := json.Unmarshal(data, &frame); err != nil {
		return
	}

	frameType, _ := frame["type"].(string)

	switch frameType {
	case "event":
		c.handleEventFrame(frame)

	case "res":
		c.handleResponseFrame(frame)

	case "hello-ok":
		c.handleHelloOk(data)
	}
}

// handleHelloOk 处理 hello-ok 握手响应。
func (c *GatewayChatClient) handleHelloOk(data []byte) {
	var hello gateway.HelloOk
	if err := json.Unmarshal(data, &hello); err != nil {
		return
	}
	c.hello = &hello

	// 通知就绪
	select {
	case <-c.readyCh:
		// 已经关闭
	default:
		close(c.readyCh)
	}

	if c.OnConnected != nil {
		c.OnConnected()
	}
}

// handleEventFrame 处理事件帧。
func (c *GatewayChatClient) handleEventFrame(frame map[string]interface{}) {
	event, _ := frame["event"].(string)
	payload := frame["payload"]

	// 序号检查
	if seqVal, ok := frame["seq"].(float64); ok {
		seq := int64(seqVal)
		c.mu.Lock()
		if c.lastSeq != nil && seq > *c.lastSeq+1 {
			expected := *c.lastSeq + 1
			c.mu.Unlock()
			if c.OnGap != nil {
				c.OnGap(expected, seq)
			}
			c.mu.Lock()
		}
		c.lastSeq = &seq
		c.mu.Unlock()
	}

	if c.OnEvent != nil {
		var seqPtr *int64
		if seqVal, ok := frame["seq"].(float64); ok {
			s := int64(seqVal)
			seqPtr = &s
		}
		c.OnEvent(GatewayEventMsg{
			Event:   event,
			Payload: payload,
			Seq:     seqPtr,
		})
	}
}

// handleResponseFrame 处理响应帧。
func (c *GatewayChatClient) handleResponseFrame(frame map[string]interface{}) {
	id, _ := frame["id"].(string)
	if id == "" {
		return
	}

	c.mu.Lock()
	p, ok := c.pending[id]
	if ok {
		delete(c.pending, id)
	}
	c.mu.Unlock()

	if !ok || p == nil {
		return
	}

	isOK, _ := frame["ok"].(bool)
	if isOK {
		payload, _ := json.Marshal(frame["payload"])
		p.resultCh <- rpcResult{Payload: payload}
	} else {
		errObj, _ := frame["error"].(map[string]interface{})
		errMsg := "unknown error"
		if errObj != nil {
			if msg, ok := errObj["message"].(string); ok && msg != "" {
				errMsg = msg
			}
		}
		p.resultCh <- rpcResult{Err: fmt.Errorf("%s", errMsg)}
	}
}

// request 发送 RPC 请求并等待响应。
func (c *GatewayChatClient) request(method string, params interface{}) (json.RawMessage, error) {
	id := uuid.New().String()
	frame := map[string]interface{}{
		"type":   "req",
		"id":     id,
		"method": method,
	}
	if params != nil {
		frame["params"] = params
	}

	resultCh := make(chan rpcResult, 1)
	c.mu.Lock()
	c.pending[id] = &pendingRequest{resultCh: resultCh}
	c.mu.Unlock()

	if err := c.wsClient.SendJSON(frame); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("send %s: %w", method, err)
	}

	// 等待响应，超时 30 秒
	select {
	case result := <-resultCh:
		return result.Payload, result.Err
	case <-time.After(30 * time.Second):
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("%s: timeout after 30s", method)
	}
}

// flushPending 清空所有挂起请求。
func (c *GatewayChatClient) flushPending(err error) {
	c.mu.Lock()
	pending := c.pending
	c.pending = make(map[string]*pendingRequest)
	c.mu.Unlock()

	for _, p := range pending {
		p.resultCh <- rpcResult{Err: err}
	}
}

// ---------- resolveGatewayConnection ----------

// resolveGatewayConnection 解析 Gateway 连接参数。
// DEP-01 修复: 完整 6 层 token/password fallback 逻辑。
//
// Token 优先级:
//  1. CLI opts.Token
//  2. remote.token (remote mode)
//  3. env OPENACOSMI_GATEWAY_TOKEN
//  4. env CLAWDBOT_GATEWAY_TOKEN
//  5. config.gateway.auth.token
//
// Password 优先级:
//  1. CLI opts.Password
//  2. env OPENACOSMI_GATEWAY_PASSWORD
//  3. env CLAWDBOT_GATEWAY_PASSWORD
//  4. remote.password (remote mode)
//  5. config.gateway.auth.password
//
// URL 优先级:
//  1. CLI opts.URL
//  2. remote.url (remote mode)
//  3. ws://127.0.0.1:{port}
func resolveGatewayConnection(opts GatewayConnectionOptions, cfgSrc GatewayConfigSource) GatewayConnectionInfo {
	cfg, _ := cfgSrc.LoadConfig()

	isRemoteMode := false
	var remoteURL, remoteToken, remotePassword string
	var authToken, authPassword string
	var cfgPort *int

	if cfg != nil && cfg.Gateway != nil {
		isRemoteMode = string(cfg.Gateway.Mode) == "remote"

		if isRemoteMode && cfg.Gateway.Remote != nil {
			remoteURL = strings.TrimSpace(cfg.Gateway.Remote.URL)
			remoteToken = strings.TrimSpace(cfg.Gateway.Remote.Token)
			remotePassword = strings.TrimSpace(cfg.Gateway.Remote.Password)
		}

		if cfg.Gateway.Auth != nil {
			authToken = strings.TrimSpace(cfg.Gateway.Auth.Token)
			authPassword = strings.TrimSpace(cfg.Gateway.Auth.Password)
		}

		cfgPort = cfg.Gateway.Port
	}

	// URL 解析
	urlOverride := strings.TrimSpace(opts.URL)
	var resolvedURL string
	if urlOverride != "" {
		resolvedURL = urlOverride
	} else if remoteURL != "" {
		resolvedURL = remoteURL
	} else {
		port := config.ResolveGatewayPort(cfgPort)
		resolvedURL = fmt.Sprintf("ws://127.0.0.1:%d", port)
	}

	// Token 解析 — CLI 显式传入优先
	cliToken := strings.TrimSpace(opts.Token)
	cliPassword := strings.TrimSpace(opts.Password)

	var resolvedToken string
	if cliToken != "" {
		resolvedToken = cliToken
	} else if urlOverride == "" {
		// 非 URL 覆盖时，按优先级 fallback
		if isRemoteMode {
			if remoteToken != "" {
				resolvedToken = remoteToken
			}
		} else {
			// 本地模式: env → config
			if v := envTrimmed("OPENACOSMI_GATEWAY_TOKEN"); v != "" {
				resolvedToken = v
			} else if v := envTrimmed("CLAWDBOT_GATEWAY_TOKEN"); v != "" {
				resolvedToken = v
			} else if authToken != "" {
				resolvedToken = authToken
			}
		}
	}

	// Password 解析
	var resolvedPassword string
	if cliPassword != "" {
		resolvedPassword = cliPassword
	} else if urlOverride == "" {
		if v := envTrimmed("OPENACOSMI_GATEWAY_PASSWORD"); v != "" {
			resolvedPassword = v
		} else if v := envTrimmed("CLAWDBOT_GATEWAY_PASSWORD"); v != "" {
			resolvedPassword = v
		} else if isRemoteMode && remotePassword != "" {
			resolvedPassword = remotePassword
		} else if authPassword != "" {
			resolvedPassword = authPassword
		}
	}

	return GatewayConnectionInfo{
		URL:      resolvedURL,
		Token:    resolvedToken,
		Password: resolvedPassword,
	}
}

// ensureExplicitGatewayAuth URL 覆盖时校验显式 auth。
// TS 参考: gateway/call.ts ensureExplicitGatewayAuth (L70-90)
// URL 覆盖 (--url) 时，必须同时提供 --token 或 --password。
func ensureExplicitGatewayAuth(urlOverride, token, password string) error {
	if urlOverride == "" {
		return nil
	}
	if token != "" || password != "" {
		return nil
	}
	return fmt.Errorf("gateway url override requires explicit credentials\nFix: pass --token or --password when using --url")
}

// envTrimmed 读取环境变量并去除空白。
func envTrimmed(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}
