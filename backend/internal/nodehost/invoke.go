package nodehost

// invoke.go — node.invoke.request 命令分派与响应构建
// 对应 TS: runner.ts L653-1308 (handleInvoke + 辅助函数)

import (
	"encoding/json"
	"strings"
)

// CoerceNodeInvokePayload 从任意 map 提取 NodeInvokeRequest。
func CoerceNodeInvokePayload(payload interface{}) *NodeInvokeRequest {
	obj, ok := payload.(map[string]interface{})
	if !ok || obj == nil {
		return nil
	}
	id, _ := obj["id"].(string)
	nodeID, _ := obj["nodeId"].(string)
	command, _ := obj["command"].(string)
	id = strings.TrimSpace(id)
	nodeID = strings.TrimSpace(nodeID)
	command = strings.TrimSpace(command)
	if id == "" || nodeID == "" || command == "" {
		return nil
	}

	req := &NodeInvokeRequest{
		ID:      id,
		NodeID:  nodeID,
		Command: command,
	}

	// paramsJSON: 直接字符串或序列化 params 对象
	switch v := obj["paramsJSON"].(type) {
	case string:
		req.ParamsJSON = v
	default:
		if p, exists := obj["params"]; exists && p != nil {
			if data, err := json.Marshal(p); err == nil {
				req.ParamsJSON = string(data)
			}
		}
	}

	if v, ok := obj["timeoutMs"].(float64); ok {
		ms := int(v)
		req.TimeoutMs = &ms
	}
	if v, ok := obj["idempotencyKey"].(string); ok {
		req.IdempotencyKey = v
	}
	return req
}

// DecodeParams 从 JSON 字符串解码参数。
func DecodeParams(raw string, target interface{}) error {
	if raw == "" {
		return &InvokeError{Code: "INVALID_REQUEST", Msg: "paramsJSON required"}
	}
	return json.Unmarshal([]byte(raw), target)
}

// BuildInvokeResult 构建 node.invoke.result 响应参数。
func BuildInvokeResult(frame *NodeInvokeRequest, ok bool, payloadJSON string, invokeErr *InvokeErrorShape) *InvokeResult {
	r := &InvokeResult{
		ID:     frame.ID,
		NodeID: frame.NodeID,
		OK:     ok,
	}
	if payloadJSON != "" {
		r.PayloadJSON = payloadJSON
	}
	if invokeErr != nil {
		r.Error = invokeErr
	}
	return r
}

// BuildInvokeResultWithPayload 构建带 payload 对象的响应。
func BuildInvokeResultWithPayload(frame *NodeInvokeRequest, ok bool, payload interface{}, invokeErr *InvokeErrorShape) *InvokeResult {
	r := &InvokeResult{
		ID:     frame.ID,
		NodeID: frame.NodeID,
		OK:     ok,
	}
	if payload != nil {
		r.Payload = payload
	}
	if invokeErr != nil {
		r.Error = invokeErr
	}
	return r
}

// InvokeError 调用过程中的错误。
type InvokeError struct {
	Code string
	Msg  string
}

func (e *InvokeError) Error() string {
	return e.Code + ": " + e.Msg
}

// ToShape 转换为 InvokeErrorShape。
func (e *InvokeError) ToShape() *InvokeErrorShape {
	return &InvokeErrorShape{Code: e.Code, Message: e.Msg}
}

// NewInvokeError 创建调用错误。
func NewInvokeError(code, msg string) *InvokeError {
	return &InvokeError{Code: code, Msg: msg}
}

// IsCmdExeInvocation 检查 argv 是否为 cmd.exe 调用。
func IsCmdExeInvocation(argv []string) bool {
	if len(argv) == 0 {
		return false
	}
	token := strings.TrimSpace(argv[0])
	if token == "" {
		return false
	}
	// 用 win32 basename 逻辑
	base := token
	if idx := strings.LastIndexAny(token, `/\`); idx >= 0 {
		base = token[idx+1:]
	}
	lower := strings.ToLower(base)
	return lower == "cmd.exe" || lower == "cmd"
}

// RedactExecApprovalsFile 移除敏感字段（与 infra.RedactExecApprovals 一致但用于 map）。
func RedactExecApprovalsFile(file map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range file {
		result[k] = v
	}
	if socket, ok := result["socket"].(map[string]interface{}); ok {
		path, _ := socket["path"].(string)
		path = strings.TrimSpace(path)
		if path != "" {
			result["socket"] = map[string]interface{}{"path": path}
		} else {
			delete(result, "socket")
		}
	}
	return result
}

// ResolveExecSecurity 解析执行安全级别。
func ResolveExecSecurity(value string) string {
	switch value {
	case "deny", "allowlist", "full":
		return value
	default:
		return "allowlist"
	}
}

// ResolveExecAsk 解析询问策略。
func ResolveExecAsk(value string) string {
	switch value {
	case "off", "on-miss", "always":
		return value
	default:
		return "on-miss"
	}
}

// StringOrDefault 从指针取值，为 nil 则返回默认值。
func StringOrDefault(ptr *string, def string) string {
	if ptr != nil {
		v := strings.TrimSpace(*ptr)
		if v != "" {
			return v
		}
	}
	return def
}
