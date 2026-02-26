package channels

import (
	"fmt"
	"strings"
)

// 动作参数读取辅助函数 — 继承自 src/agents/tools/common.ts

// ReadStringParam 从参数 map 中读取字符串参数
func ReadStringParam(params map[string]interface{}, key string) string {
	v, ok := params[key]
	if !ok || v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", v))
}

// ReadStringArrayParam 从参数 map 中读取字符串数组参数
func ReadStringArrayParam(params map[string]interface{}, key string) []string {
	v, ok := params[key]
	if !ok || v == nil {
		return nil
	}
	if arr, ok := v.([]interface{}); ok {
		var result []string
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			} else {
				result = append(result, fmt.Sprintf("%v", item))
			}
		}
		return result
	}
	if arr, ok := v.([]string); ok {
		return arr
	}
	// 单个值也当数组处理
	s := fmt.Sprintf("%v", v)
	if s != "" {
		return []string{s}
	}
	return nil
}

// ReadIntParam 从参数 map 中读取整数参数
func ReadIntParam(params map[string]interface{}, key string) *int {
	v, ok := params[key]
	if !ok || v == nil {
		return nil
	}
	switch n := v.(type) {
	case int:
		return &n
	case int64:
		i := int(n)
		return &i
	case float64:
		i := int(n)
		return &i
	}
	return nil
}

// ReadStringParamRequired 读取必填字符串参数，空值时返回 error
func ReadStringParamRequired(params map[string]interface{}, key string) (string, error) {
	v := ReadStringParam(params, key)
	if v == "" {
		return "", fmt.Errorf("parameter %q is required", key)
	}
	return v, nil
}

// ReadStringParamRaw 读取字符串参数，不 trim（用于 media URL 等）
func ReadStringParamRaw(params map[string]interface{}, key string) string {
	v, ok := params[key]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// ReadBoolParam 从参数 map 中读取布尔参数
func ReadBoolParam(params map[string]interface{}, key string) *bool {
	v, ok := params[key]
	if !ok || v == nil {
		return nil
	}
	if b, ok := v.(bool); ok {
		return &b
	}
	return nil
}
