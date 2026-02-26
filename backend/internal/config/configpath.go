package config

// 配置路径操作 — 对应 src/config/config-paths.ts (91 行)
//
// 提供配置对象的 dot-path 导航能力，用于 runtime-overrides 等场景。
// 例: "agents.defaults.contextTokens" → ["agents", "defaults", "contextTokens"]
// 依赖: 无 npm 依赖。

import "strings"

// blockedKeys 禁止在配置路径中使用的键（防止原型污染）
var blockedKeys = map[string]bool{
	"__proto__":   true,
	"prototype":   true,
	"constructor": true,
}

// ParseConfigPath 解析 dot-notation 配置路径
// 返回路径分段切片和可选的错误信息。
// 对应 TS: parseConfigPath(raw)
func ParseConfigPath(raw string) ([]string, string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, "Invalid path. Use dot notation (e.g. foo.bar)."
	}

	parts := strings.Split(trimmed, ".")
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}

	for _, part := range parts {
		if part == "" {
			return nil, "Invalid path. Use dot notation (e.g. foo.bar)."
		}
		if blockedKeys[part] {
			return nil, "Invalid path segment."
		}
	}

	return parts, ""
}

// SetConfigValueAtPath 在配置树中按路径设置值
// 对应 TS: setConfigValueAtPath(root, path, value)
func SetConfigValueAtPath(root map[string]interface{}, path []string, value interface{}) {
	cursor := root
	for i := 0; i < len(path)-1; i++ {
		key := path[i]
		next, ok := cursor[key]
		if !ok {
			cursor[key] = map[string]interface{}{}
		} else if _, isMap := next.(map[string]interface{}); !isMap {
			cursor[key] = map[string]interface{}{}
		}
		cursor = cursor[key].(map[string]interface{})
	}
	cursor[path[len(path)-1]] = value
}

// UnsetConfigValueAtPath 在配置树中按路径删除值
// 如果删除后父节点为空则自动清理空父节点。
// 返回是否成功删除。
// 对应 TS: unsetConfigValueAtPath(root, path)
func UnsetConfigValueAtPath(root map[string]interface{}, path []string) bool {
	type stackEntry struct {
		node map[string]interface{}
		key  string
	}

	var stack []stackEntry
	cursor := root

	for i := 0; i < len(path)-1; i++ {
		key := path[i]
		next, ok := cursor[key]
		if !ok {
			return false
		}
		nextMap, isMap := next.(map[string]interface{})
		if !isMap {
			return false
		}
		stack = append(stack, stackEntry{node: cursor, key: key})
		cursor = nextMap
	}

	leafKey := path[len(path)-1]
	if _, exists := cursor[leafKey]; !exists {
		return false
	}
	delete(cursor, leafKey)

	// 清理空父节点
	for i := len(stack) - 1; i >= 0; i-- {
		entry := stack[i]
		child, ok := entry.node[entry.key]
		if !ok {
			break
		}
		childMap, isMap := child.(map[string]interface{})
		if isMap && len(childMap) == 0 {
			delete(entry.node, entry.key)
		} else {
			break
		}
	}

	return true
}

// GetConfigValueAtPath 在配置树中按路径获取值
// 对应 TS: getConfigValueAtPath(root, path)
func GetConfigValueAtPath(root map[string]interface{}, path []string) (interface{}, bool) {
	var cursor interface{} = root
	for _, key := range path {
		m, ok := cursor.(map[string]interface{})
		if !ok {
			return nil, false
		}
		cursor, ok = m[key]
		if !ok {
			return nil, false
		}
	}
	return cursor, true
}
