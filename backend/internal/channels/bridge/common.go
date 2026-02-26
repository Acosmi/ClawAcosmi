package bridge

import (
	"fmt"
	"strings"
)

// 工具桥接通用辅助 — 继承自 src/agents/tools/common.ts

// ActionGate 动作门控函数
type ActionGate func(actionName string) bool

// CreateActionGate 创建动作门控（基于配置的白/黑名单）
func CreateActionGate(actions interface{}) ActionGate {
	if actions == nil {
		return func(string) bool { return true }
	}
	m, ok := actions.(map[string]interface{})
	if !ok {
		return func(string) bool { return true }
	}
	return func(actionName string) bool {
		if v, ok := m[actionName].(bool); ok {
			return v
		}
		if v, ok := m["*"].(bool); ok {
			return v
		}
		return true // 默认允许
	}
}

// ReadStringParam 读取字符串参数
func ReadStringParam(params map[string]interface{}, key string, required bool) (string, error) {
	v, ok := params[key]
	if !ok || v == nil {
		if required {
			return "", fmt.Errorf("required parameter '%s' is missing", key)
		}
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Sprintf("%v", v), nil
	}
	return strings.TrimSpace(s), nil
}

// ReadNumberParam 读取数字参数
func ReadNumberParam(params map[string]interface{}, key string) (float64, bool) {
	v, ok := params[key]
	if !ok || v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

// ToolResult 工具调用结果
type ToolResult struct {
	Data    interface{} `json:"data,omitempty"`
	IsError bool        `json:"isError,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// OkResult 成功结果
func OkResult(data interface{}) ToolResult {
	return ToolResult{Data: data}
}

// ErrorResult 错误结果
func ErrorResult(err error) ToolResult {
	return ToolResult{IsError: true, Error: err.Error()}
}

// ReactionParams 反应操作参数
type ReactionParams struct {
	Emoji   string
	Remove  bool
	IsEmpty bool
}

// ReadReactionParams 读取反应参数
func ReadReactionParams(params map[string]interface{}) ReactionParams {
	emoji, _ := ReadStringParam(params, "emoji", false)
	removeRaw, _ := params["remove"].(bool)
	return ReactionParams{
		Emoji:   emoji,
		Remove:  removeRaw,
		IsEmpty: emoji == "",
	}
}

// ReadBoolParam 读取布尔参数
func ReadBoolParam(params map[string]interface{}, key string, defaultVal bool) bool {
	v, ok := params[key]
	if !ok || v == nil {
		return defaultVal
	}
	b, ok := v.(bool)
	if !ok {
		return defaultVal
	}
	return b
}

// ReadIntParam 读取整型参数（截断浮点）
func ReadIntParam(params map[string]interface{}, key string) (int, bool) {
	v, ok := ReadNumberParam(params, key)
	if !ok {
		return 0, false
	}
	return int(v), true
}

// ReadStringArrayParam 读取字符串数组参数
func ReadStringArrayParam(params map[string]interface{}, key string) []string {
	v, ok := params[key]
	if !ok || v == nil {
		return nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	var result []string
	for _, item := range arr {
		if s, ok := item.(string); ok && s != "" {
			result = append(result, s)
		}
	}
	return result
}

// ReadNumberParamRequired 读取必填数字参数
func ReadNumberParamRequired(params map[string]interface{}, key string) (float64, error) {
	v, ok := ReadNumberParam(params, key)
	if !ok {
		return 0, fmt.Errorf("required parameter '%s' is missing or not a number", key)
	}
	return v, nil
}
