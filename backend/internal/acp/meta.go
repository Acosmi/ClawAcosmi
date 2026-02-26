package acp

import "math"

// ReadString 从 meta map 中按优先级序列读取字符串值。
// 对应 TS: acp/meta.ts readString()
func ReadString(meta map[string]interface{}, keys []string) string {
	if meta == nil {
		return ""
	}
	for _, key := range keys {
		v, ok := meta[key]
		if !ok {
			continue
		}
		s, ok := v.(string)
		if !ok {
			continue
		}
		trimmed := trimSpace(s)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// ReadBool 从 meta map 中按优先级序列读取布尔值。
// 对应 TS: acp/meta.ts readBool()
func ReadBool(meta map[string]interface{}, keys []string) *bool {
	if meta == nil {
		return nil
	}
	for _, key := range keys {
		v, ok := meta[key]
		if !ok {
			continue
		}
		b, ok := v.(bool)
		if ok {
			return &b
		}
	}
	return nil
}

// ReadNumber 从 meta map 中按优先级序列读取数值。
// 对应 TS: acp/meta.ts readNumber()
func ReadNumber(meta map[string]interface{}, keys []string) *float64 {
	if meta == nil {
		return nil
	}
	for _, key := range keys {
		v, ok := meta[key]
		if !ok {
			continue
		}
		switch n := v.(type) {
		case float64:
			if !math.IsInf(n, 0) && !math.IsNaN(n) {
				return &n
			}
		case int:
			f := float64(n)
			return &f
		case int64:
			f := float64(n)
			return &f
		}
	}
	return nil
}

// ReadInt 从 meta map 中按优先级序列读取整数值（便捷函数）。
func ReadInt(meta map[string]interface{}, keys []string, fallback int) int {
	n := ReadNumber(meta, keys)
	if n == nil {
		return fallback
	}
	return int(*n)
}

// trimSpace 去除首尾空白（避免 import strings 仅为此一用）。
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
