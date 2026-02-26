package config

// 配置路径规范化 — 对应 src/config/normalize-paths.ts (74 行)
//
// 遍历整个配置对象，对 path 类字段自动展开 "~" 前缀。
// 匹配键名规则:
//   - 键名以 dir/path/paths/file/root/workspace 结尾 (不区分大小写)
//   - 键名为 "paths" 或 "pathPrepend" 时子元素也展开
//
// 依赖: paths.go 中的 resolveUserPath (已实现)

import (
	"regexp"
)

// pathValueRE 检测以 ~ 开头的路径值
var pathValueRE = regexp.MustCompile(`^~($|[/\\])`)

// pathKeyRE 匹配路径相关的键名
var pathKeyRE = regexp.MustCompile(`(?i)(dir|path|paths|file|root|workspace)$`)

// pathListKeys 其子元素也需要展开的键名集合
var pathListKeys = map[string]bool{
	"paths":       true,
	"pathPrepend": true,
}

// normalizeStringValue 对单个字符串值进行 ~ 展开
func normalizeStringValue(key string, value string) string {
	trimmed := value
	if !pathValueRE.MatchString(trimmed) {
		return value
	}
	if key == "" {
		return value
	}
	if pathKeyRE.MatchString(key) || pathListKeys[key] {
		return resolveUserPath(value)
	}
	return value
}

// normalizeAny 递归遍历配置值并规范化路径
func normalizeAny(key string, value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		return normalizeStringValue(key, v)

	case []interface{}:
		normalizeChildren := key != "" && pathListKeys[key]
		result := make([]interface{}, len(v))
		for i, entry := range v {
			switch e := entry.(type) {
			case string:
				if normalizeChildren {
					result[i] = normalizeStringValue(key, e)
				} else {
					result[i] = e
				}
			case []interface{}:
				result[i] = normalizeAny("", entry)
			case map[string]interface{}:
				result[i] = normalizeAny("", entry)
			default:
				result[i] = entry
			}
		}
		return result

	case map[string]interface{}:
		for childKey, childValue := range v {
			// 对字符串做快速判断，避免无谓的赋值；
			// 对 map 类型原地修改；对 slice 需要赋值返回值。
			switch cv := childValue.(type) {
			case string:
				normalized := normalizeStringValue(childKey, cv)
				if normalized != cv {
					v[childKey] = normalized
				}
			default:
				v[childKey] = normalizeAny(childKey, childValue)
			}
		}
		return v

	default:
		return value
	}
}

// NormalizeConfigPaths 规范化配置中的 ~ 路径
// 在 env 替换之后、应用默认值之前调用。
// 对应 TS: normalizeConfigPaths(cfg)
func NormalizeConfigPaths(cfg map[string]interface{}) map[string]interface{} {
	if cfg == nil {
		return cfg
	}
	normalizeAny("", cfg)
	return cfg
}
