package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// ---------- 工具调用类型 ----------

// ToolInvokeRequest 工具调用请求。
type ToolInvokeRequest struct {
	ToolName   string                 `json:"toolName"`
	Arguments  map[string]interface{} `json:"arguments,omitempty"`
	SessionKey string                 `json:"sessionKey,omitempty"`
	RunID      string                 `json:"runId,omitempty"`
}

// ToolInvokeResponse 工具调用响应。
type ToolInvokeResponse struct {
	OK     bool        `json:"ok"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// ToolPolicy 工具调用策略。
type ToolPolicy string

const (
	ToolPolicyAllow  ToolPolicy = "allow"
	ToolPolicyDeny   ToolPolicy = "deny"
	ToolPolicyPrompt ToolPolicy = "prompt"
)

// ToolRegistration 工具注册信息。
type ToolRegistration struct {
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Policy      ToolPolicy `json:"policy,omitempty"`
}

// ToolRegistry 工具注册表（线程安全）。
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]*ToolRegistration
}

// NewToolRegistry 创建工具注册表。
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]*ToolRegistration)}
}

// Register 注册工具。
func (r *ToolRegistry) Register(tool *ToolRegistration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name] = tool
}

// Get 获取工具注册信息。
func (r *ToolRegistry) Get(name string) *ToolRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// List 列出所有工具。
func (r *ToolRegistry) List() []*ToolRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*ToolRegistration, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

// ---------- 工具调用 HTTP 处理 ----------

// ParseToolInvokeRequest 从 HTTP 请求解析工具调用。
func ParseToolInvokeRequest(r *http.Request) (*ToolInvokeRequest, error) {
	if r.Method != http.MethodPost {
		return nil, fmt.Errorf("method not allowed")
	}
	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		return nil, fmt.Errorf("unsupported content type")
	}
	var req ToolInvokeRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		return nil, fmt.Errorf("invalid request body: %w", err)
	}
	req.ToolName = strings.TrimSpace(req.ToolName)
	if req.ToolName == "" {
		return nil, fmt.Errorf("toolName required")
	}
	return &req, nil
}

// ResolveToolPolicy 解析工具调用策略（allow/deny/prompt）。
func ResolveToolPolicy(tool *ToolRegistration, defaultPolicy ToolPolicy) ToolPolicy {
	if tool == nil {
		return ToolPolicyDeny
	}
	if tool.Policy != "" {
		return tool.Policy
	}
	if defaultPolicy != "" {
		return defaultPolicy
	}
	return ToolPolicyAllow
}

// SendToolResponse 发送工具调用响应。
func SendToolResponse(w http.ResponseWriter, statusCode int, result interface{}, errMsg string) {
	resp := ToolInvokeResponse{
		OK:     errMsg == "",
		Result: result,
		Error:  errMsg,
	}
	SendJSON(w, statusCode, resp)
}
