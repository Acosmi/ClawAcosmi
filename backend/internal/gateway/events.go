package gateway

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// ---------- 节点事件类型 ----------

// NodeEvent 节点发送到网关的事件。
type NodeEvent struct {
	Event       string `json:"event"`
	PayloadJSON string `json:"payloadJSON,omitempty"`
}

// NodeEventContext 节点事件处理上下文。
type NodeEventContext struct {
	ChatState *ChatRunState
	Logger    func(format string, args ...interface{})
}

// AgentDeepLink 代理深度链接请求。
type AgentDeepLink struct {
	Message        string  `json:"message,omitempty"`
	SessionKey     *string `json:"sessionKey,omitempty"`
	Thinking       *string `json:"thinking,omitempty"`
	Deliver        *bool   `json:"deliver,omitempty"`
	To             *string `json:"to,omitempty"`
	Channel        *string `json:"channel,omitempty"`
	TimeoutSeconds *int    `json:"timeoutSeconds,omitempty"`
	Key            *string `json:"key,omitempty"`
}

const maxNodeEventTextLen = 20_000

// ---------- 事件解析 ----------

// ParseVoiceTranscript 解析 voice.transcript 的 payload。
func ParseVoiceTranscript(payloadJSON string) (text string, sessionKey string, err error) {
	if payloadJSON == "" {
		return "", "", fmt.Errorf("empty payload")
	}
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(payloadJSON), &raw); err != nil {
		return "", "", err
	}
	t, _ := raw["text"].(string)
	text = strings.TrimSpace(t)
	if text == "" {
		return "", "", fmt.Errorf("empty text")
	}
	if len(text) > maxNodeEventTextLen {
		return "", "", fmt.Errorf("text too long: %d", len(text))
	}
	sk, _ := raw["sessionKey"].(string)
	sessionKey = strings.TrimSpace(sk)
	return text, sessionKey, nil
}

// ParseAgentRequest 解析 agent.request 的 payload。
func ParseAgentRequest(payloadJSON string) (*AgentDeepLink, error) {
	if payloadJSON == "" {
		return nil, fmt.Errorf("empty payload")
	}
	var link AgentDeepLink
	if err := json.Unmarshal([]byte(payloadJSON), &link); err != nil {
		return nil, err
	}
	link.Message = strings.TrimSpace(link.Message)
	if link.Message == "" {
		return nil, fmt.Errorf("empty message")
	}
	if len(link.Message) > maxNodeEventTextLen {
		return nil, fmt.Errorf("message too long")
	}
	return &link, nil
}

// ---------- 3 个缺失事件解析 (移植自 node-events.ts) ----------

// SystemContextPayload 系统上下文变更事件。
type SystemContextPayload struct {
	SessionKey string `json:"sessionKey"`
	ContextKey string `json:"contextKey"`
	Text       string `json:"text"`
}

// ParseSystemContext 解析 system.context 事件。
func ParseSystemContext(payloadJSON string) (*SystemContextPayload, error) {
	if payloadJSON == "" {
		return nil, fmt.Errorf("empty payload")
	}
	var p SystemContextPayload
	if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
		return nil, err
	}
	p.SessionKey = strings.TrimSpace(p.SessionKey)
	if p.SessionKey == "" {
		return nil, fmt.Errorf("empty sessionKey")
	}
	p.Text = strings.TrimSpace(p.Text)
	p.ContextKey = strings.TrimSpace(p.ContextKey)
	return &p, nil
}

// AgentUpdatePayload agent 状态更新事件。
type AgentUpdatePayload struct {
	RunID  string `json:"runId"`
	Status string `json:"status"` // "started" | "finished" | "error"
	Error  string `json:"error,omitempty"`
}

// ParseAgentUpdate 解析 agent.update 事件。
func ParseAgentUpdate(payloadJSON string) (*AgentUpdatePayload, error) {
	if payloadJSON == "" {
		return nil, fmt.Errorf("empty payload")
	}
	var p AgentUpdatePayload
	if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
		return nil, err
	}
	p.RunID = strings.TrimSpace(p.RunID)
	if p.RunID == "" {
		return nil, fmt.Errorf("empty runId")
	}
	p.Status = strings.TrimSpace(p.Status)
	if p.Status == "" {
		return nil, fmt.Errorf("empty status")
	}
	return &p, nil
}

// HealthPingPayload 健康检查 ping 事件。
type HealthPingPayload struct {
	NodeID    string `json:"nodeId"`
	Timestamp int64  `json:"timestamp"`
	Version   string `json:"version,omitempty"`
}

// ParseHealthPing 解析 health.ping 事件。
func ParseHealthPing(payloadJSON string) (*HealthPingPayload, error) {
	if payloadJSON == "" {
		return nil, fmt.Errorf("empty payload")
	}
	var p HealthPingPayload
	if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
		return nil, err
	}
	p.NodeID = strings.TrimSpace(p.NodeID)
	if p.NodeID == "" {
		return nil, fmt.Errorf("empty nodeId")
	}
	return &p, nil
}

// ---------- 事件分发器 ----------

// NodeEventHandler 节点事件处理函数签名。
type NodeEventHandler func(ctx *NodeEventContext, nodeID string, evt *NodeEvent) error

// NodeEventDispatcher 注册与分发节点事件。
type NodeEventDispatcher struct {
	mu       sync.RWMutex
	handlers map[string][]NodeEventHandler
}

// NewNodeEventDispatcher 创建分发器。
func NewNodeEventDispatcher() *NodeEventDispatcher {
	return &NodeEventDispatcher{
		handlers: make(map[string][]NodeEventHandler),
	}
}

// Register 注册事件处理器。
func (d *NodeEventDispatcher) Register(event string, handler NodeEventHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[event] = append(d.handlers[event], handler)
}

// Dispatch 分发事件到注册的处理器。
func (d *NodeEventDispatcher) Dispatch(ctx *NodeEventContext, nodeID string, evt *NodeEvent) error {
	d.mu.RLock()
	handlers := d.handlers[evt.Event]
	d.mu.RUnlock()

	for _, h := range handlers {
		if err := h(ctx, nodeID, evt); err != nil {
			return err
		}
	}
	return nil
}
