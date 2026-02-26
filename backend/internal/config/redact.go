package config

// 配置脱敏模块 — 对应 src/config/redact-snapshot.ts (169 行)
//
// 为配置快照中的敏感字段（token/password/secret/apiKey 等）提供脱敏和还原功能。
// 网关 config.get 返回脱敏后的配置，config.set/apply/patch 写入前调用 RestoreRedactedValues
// 还原 sentinel 占位符为原始值，确保凭据不因 Web UI 往返而丢失。

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// RedactedSentinel 敏感字段替换的哨兵值
// 对应 TS redact-snapshot.ts:L9
const RedactedSentinel = "__OPENACOSMI_REDACTED__"

// sensitiveKeyPatterns 敏感字段名匹配模式
// 对应 TS redact-snapshot.ts:L15
var sensitiveKeyPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)token`),
	regexp.MustCompile(`(?i)password`),
	regexp.MustCompile(`(?i)secret`),
	regexp.MustCompile(`(?i)api.?key`),
}

// IsSensitiveKey 检查字段名是否为敏感字段
func IsSensitiveKey(key string) bool {
	for _, p := range sensitiveKeyPatterns {
		if p.MatchString(key) {
			return true
		}
	}
	return false
}

// RedactConfigObject 深度遍历对象并替换敏感字段值为哨兵
// 对应 TS redact-snapshot.ts:L25-L50
func RedactConfigObject(obj interface{}) interface{} {
	return redactObject(obj)
}

func redactObject(obj interface{}) interface{} {
	if obj == nil {
		return nil
	}
	switch v := obj.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(v))
		for key, value := range v {
			if IsSensitiveKey(key) && value != nil {
				result[key] = RedactedSentinel
			} else if sub, ok := value.(map[string]interface{}); ok {
				result[key] = redactObject(sub)
			} else if arr, ok := value.([]interface{}); ok {
				result[key] = redactObject(arr)
			} else {
				result[key] = value
			}
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = redactObject(item)
		}
		return result
	default:
		return obj
	}
}

// collectSensitiveValues 收集对象中所有敏感字符串值
// 对应 TS redact-snapshot.ts:L56-L75
func collectSensitiveValues(obj interface{}) []string {
	var values []string
	if obj == nil {
		return values
	}
	switch v := obj.(type) {
	case map[string]interface{}:
		for key, value := range v {
			if IsSensitiveKey(key) {
				if s, ok := value.(string); ok && len(s) > 0 {
					values = append(values, s)
				}
			} else {
				values = append(values, collectSensitiveValues(value)...)
			}
		}
	case []interface{}:
		for _, item := range v {
			values = append(values, collectSensitiveValues(item)...)
		}
	}
	return values
}

// RedactRawText 在原始 JSON5 文本中替换敏感值为哨兵
// 对应 TS redact-snapshot.ts:L81-L107
// values 按长度降序替换以避免部分匹配
func RedactRawText(raw string, config interface{}) string {
	sensitiveValues := collectSensitiveValues(config)
	if len(sensitiveValues) == 0 {
		return raw
	}
	// 按长度降序排序
	sort.Slice(sensitiveValues, func(i, j int) bool {
		return len(sensitiveValues[i]) > len(sensitiveValues[j])
	})

	result := raw
	for _, value := range sensitiveValues {
		// D2 优化: 使用 strings.ReplaceAll 替代 regexp.MustCompile
		result = strings.ReplaceAll(result, value, RedactedSentinel)
	}

	// key-value 模式: 匹配 JSON/JSON5 中的 key: "value" 对
	kvPattern := regexp.MustCompile(`(?m)(^|[{\s,])("([^"]+)"|'([^']+)'|([A-Za-z0-9_.$-]+))(\s*:\s*)("([^"]*)"|'([^']*)')`)
	result = kvPattern.ReplaceAllStringFunc(result, func(match string) string {
		sub := kvPattern.FindStringSubmatch(match)
		if sub == nil {
			return match
		}
		// 提取 key: sub[3] 双引号内, sub[4] 单引号内, sub[5] 裸 key
		key := sub[3]
		if key == "" {
			key = sub[4]
		}
		if key == "" {
			key = sub[5]
		}
		if key == "" || !IsSensitiveKey(key) {
			return match
		}
		// 提取 value: sub[8] 双引号内, sub[9] 单引号内
		val := sub[8]
		if val == "" {
			val = sub[9]
		}
		if val == RedactedSentinel {
			return match
		}
		// 替换 value 为 sentinel
		prefix := sub[1]
		keyExpr := sub[2]
		sep := sub[6]
		valQuote := string(sub[7][0]) // 引号字符
		return prefix + keyExpr + sep + valQuote + RedactedSentinel + valQuote
	})

	return result
}

// RedactConfigSnapshot 对配置快照进行脱敏
// 对应 TS redact-snapshot.ts:L117-L128
func RedactConfigSnapshot(snapshot *types.ConfigFileSnapshot) *types.ConfigFileSnapshot {
	if snapshot == nil {
		return nil
	}
	// 深拷贝快照
	result := *snapshot

	// D1 修复: 对 Config struct 进行脱敏
	// 通过 json Marshal → redact → Unmarshal 实现
	{
		data, err := json.Marshal(snapshot.Config)
		if err == nil {
			var configMap map[string]interface{}
			if json.Unmarshal(data, &configMap) == nil {
				redacted := redactObject(configMap)
				if redactedMap, ok := redacted.(map[string]interface{}); ok {
					redactedData, err := json.Marshal(redactedMap)
					if err == nil {
						var redactedCfg types.OpenAcosmiConfig
						if json.Unmarshal(redactedData, &redactedCfg) == nil {
							result.Config = redactedCfg
						}
					}
				}
			}
		}
	}

	// 脱敏 Raw
	if snapshot.Raw != nil {
		redacted := RedactRawText(*snapshot.Raw, snapshot.Parsed)
		result.Raw = &redacted
	}

	// 脱敏 Parsed
	if snapshot.Parsed != nil {
		result.Parsed = RedactConfigObject(snapshot.Parsed)
	}

	return &result
}

// RestoreRedactedValues 将 incoming 中的哨兵值还原为 original 中的原始值
// 对应 TS redact-snapshot.ts:L137-L168
// 在 config.set / config.apply / config.patch 写入前调用
func RestoreRedactedValues(incoming, original interface{}) (interface{}, error) {
	if incoming == nil {
		return nil, nil
	}

	switch inc := incoming.(type) {
	case map[string]interface{}:
		orig, _ := original.(map[string]interface{})
		if orig == nil {
			orig = map[string]interface{}{}
		}
		result := make(map[string]interface{}, len(inc))
		for key, value := range inc {
			if IsSensitiveKey(key) {
				if s, ok := value.(string); ok && s == RedactedSentinel {
					origVal, exists := orig[key]
					if !exists {
						return nil, fmt.Errorf(
							"config write rejected: %q is redacted; set an explicit value instead of %s",
							key, RedactedSentinel,
						)
					}
					result[key] = origVal
					continue
				}
			}
			if sub, ok := value.(map[string]interface{}); ok {
				restored, err := RestoreRedactedValues(sub, orig[key])
				if err != nil {
					return nil, err
				}
				result[key] = restored
			} else if arr, ok := value.([]interface{}); ok {
				restored, err := RestoreRedactedValues(arr, orig[key])
				if err != nil {
					return nil, err
				}
				result[key] = restored
			} else {
				result[key] = value
			}
		}
		return result, nil

	case []interface{}:
		origArr, _ := original.([]interface{})
		result := make([]interface{}, len(inc))
		for i, item := range inc {
			var origItem interface{}
			if i < len(origArr) {
				origItem = origArr[i]
			}
			restored, err := RestoreRedactedValues(item, origItem)
			if err != nil {
				return nil, err
			}
			result[i] = restored
		}
		return result, nil

	default:
		return incoming, nil
	}
}
