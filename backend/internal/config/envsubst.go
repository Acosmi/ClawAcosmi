package config

// 环境变量替换 — 对应 src/config/env-substitution.ts (135 行)
//
// 配置值中的 ${VAR_NAME} 语法会在加载时替换为对应环境变量。
// - 仅匹配大写环境变量名: [A-Z_][A-Z0-9_]*
// - 使用 $${VAR} 转义为字面量 ${VAR}
// - 缺失的环境变量返回 MissingEnvVarError（含路径上下文）
//
// 依赖: 无 npm 依赖，使用 os.Getenv。

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// envVarNamePattern 合法的大写环境变量名模式
var envVarNamePattern = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

// MissingEnvVarError 表示配置中引用了未设置的环境变量
type MissingEnvVarError struct {
	VarName    string
	ConfigPath string
}

func (e *MissingEnvVarError) Error() string {
	return fmt.Sprintf("Missing env var %q referenced at config path: %s", e.VarName, e.ConfigPath)
}

// EnvLookupFunc 环境变量查找函数类型（方便测试注入）
type EnvLookupFunc func(key string) (string, bool)

// substituteString 在字符串中执行 ${VAR} 替换
// 对应 TS: substituteString(value, env, configPath)
func substituteString(value string, lookup EnvLookupFunc, configPath string) (string, error) {
	if !strings.Contains(value, "$") {
		return value, nil
	}

	var buf strings.Builder
	i := 0
	for i < len(value) {
		ch := value[i]
		if ch != '$' {
			buf.WriteByte(ch)
			i++
			continue
		}

		// 检查 $${...} 转义序列
		if i+2 < len(value) && value[i+1] == '$' && value[i+2] == '{' {
			start := i + 3
			end := strings.IndexByte(value[start:], '}')
			if end != -1 {
				name := value[start : start+end]
				if envVarNamePattern.MatchString(name) {
					buf.WriteString("${")
					buf.WriteString(name)
					buf.WriteByte('}')
					i = start + end + 1
					continue
				}
			}
		}

		// 检查 ${...} 替换序列
		if i+1 < len(value) && value[i+1] == '{' {
			start := i + 2
			end := strings.IndexByte(value[start:], '}')
			if end != -1 {
				name := value[start : start+end]
				if envVarNamePattern.MatchString(name) {
					envVal, exists := lookup(name)
					if !exists || envVal == "" {
						return "", &MissingEnvVarError{VarName: name, ConfigPath: configPath}
					}
					buf.WriteString(envVal)
					i = start + end + 1
					continue
				}
			}
		}

		// 不是可识别的模式，原样输出
		buf.WriteByte(ch)
		i++
	}

	return buf.String(), nil
}

// substituteAny 递归遍历配置树，替换所有字符串值中的 ${VAR}
// 对应 TS: substituteAny(value, env, path)
func substituteAny(value interface{}, lookup EnvLookupFunc, path string) (interface{}, error) {
	switch v := value.(type) {
	case string:
		return substituteString(v, lookup, path)

	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			childPath := fmt.Sprintf("%s[%d]", path, i)
			r, err := substituteAny(item, lookup, childPath)
			if err != nil {
				return nil, err
			}
			result[i] = r
		}
		return result, nil

	case map[string]interface{}:
		result := make(map[string]interface{}, len(v))
		for key, val := range v {
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			r, err := substituteAny(val, lookup, childPath)
			if err != nil {
				return nil, err
			}
			result[key] = r
		}
		return result, nil

	default:
		// number, bool, nil 等原样返回
		return value, nil
	}
}

// ResolveConfigEnvVars 解析配置对象中所有 ${VAR_NAME} 环境变量引用。
// 在 JSON5 解析和 $include 解析之后调用。
// 对应 TS: resolveConfigEnvVars(obj, env)
func ResolveConfigEnvVars(obj interface{}) (interface{}, error) {
	return ResolveConfigEnvVarsWithLookup(obj, defaultEnvLookup)
}

// ResolveConfigEnvVarsWithLookup 使用自定义查找函数解析环境变量
func ResolveConfigEnvVarsWithLookup(obj interface{}, lookup EnvLookupFunc) (interface{}, error) {
	return substituteAny(obj, lookup, "")
}

// defaultEnvLookup 默认使用 os.LookupEnv
func defaultEnvLookup(key string) (string, bool) {
	return os.LookupEnv(key)
}
