// tools/schema.go — 工具 Schema 标准化。
// TS 参考：src/agents/pi-tools.schema.ts (179L)
package tools

import (
	"github.com/anthropic/open-acosmi/internal/agents/schema"
)

// NormalizeToolParameters 规范化工具参数 schema。
// 处理 anyOf/oneOf union 合并 + Gemini 清洗。
// TS 参考: pi-tools.schema.ts normalizeToolParameters
func NormalizeToolParameters(tool *AgentTool) *AgentTool {
	if tool == nil {
		return nil
	}
	params, ok := tool.Parameters.(map[string]any)
	if !ok || params == nil {
		return tool
	}

	// 已有 type + properties 且无 anyOf → 直接清洗
	_, hasType := params["type"]
	_, hasProps := params["properties"]
	_, hasAnyOf := params["anyOf"]
	_, hasOneOf := params["oneOf"]

	if hasType && hasProps && !hasAnyOf {
		out := copyTool(tool)
		out.Parameters = schema.CleanSchemaForGemini(params)
		return out
	}

	// 无 type 但有 object-ish 属性 → 强制 type=object
	if !hasType && (hasProps || hasRequired(params)) && !hasAnyOf && !hasOneOf {
		forced := shallowCopy(params)
		forced["type"] = "object"
		out := copyTool(tool)
		out.Parameters = schema.CleanSchemaForGemini(forced)
		return out
	}

	// anyOf / oneOf 合并
	variantKey := ""
	if hasAnyOf {
		variantKey = "anyOf"
	} else if hasOneOf {
		variantKey = "oneOf"
	}
	if variantKey == "" {
		return tool
	}

	variants, ok := params[variantKey].([]any)
	if !ok || len(variants) == 0 {
		return tool
	}

	mergedProperties := map[string]any{}
	requiredCounts := map[string]int{}
	objectVariants := 0

	for _, entry := range variants {
		obj, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		props, ok := obj["properties"].(map[string]any)
		if !ok {
			continue
		}
		objectVariants++

		for key, value := range props {
			existing, exists := mergedProperties[key]
			if !exists {
				mergedProperties[key] = value
			} else {
				mergedProperties[key] = mergePropertySchemas(existing, value)
			}
		}

		if req, ok := obj["required"].([]any); ok {
			for _, r := range req {
				if s, ok := r.(string); ok {
					requiredCounts[s]++
				}
			}
		}
	}

	// 计算 required
	baseRequired := extractStringArray(params, "required")
	var mergedRequired []string
	if len(baseRequired) > 0 {
		mergedRequired = baseRequired
	} else if objectVariants > 0 {
		for key, count := range requiredCounts {
			if count == objectVariants {
				mergedRequired = append(mergedRequired, key)
			}
		}
	}

	// 构造标准化 schema
	normalized := map[string]any{
		"type": "object",
	}
	if title, ok := params["title"].(string); ok {
		normalized["title"] = title
	}
	if desc, ok := params["description"].(string); ok {
		normalized["description"] = desc
	}
	if len(mergedProperties) > 0 {
		normalized["properties"] = mergedProperties
	} else if props, ok := params["properties"]; ok {
		normalized["properties"] = props
	}
	if len(mergedRequired) > 0 {
		normalized["required"] = toAnySlice(mergedRequired)
	}
	if ap, ok := params["additionalProperties"]; ok {
		normalized["additionalProperties"] = ap
	} else {
		normalized["additionalProperties"] = true
	}

	out := copyTool(tool)
	out.Parameters = schema.CleanSchemaForGemini(normalized)
	return out
}

// mergePropertySchemas 合并两个属性 schema。
// TS 参考: pi-tools.schema.ts mergePropertySchemas
func mergePropertySchemas(a, b any) any {
	aObj, aOk := a.(map[string]any)
	bObj, bOk := b.(map[string]any)
	if !aOk || !bOk {
		return a
	}

	// 如果两个 schema 都有 enum，合并 enum 值
	aEnum, aHasEnum := aObj["enum"].([]any)
	bEnum, bHasEnum := bObj["enum"].([]any)
	if aHasEnum && bHasEnum {
		merged := shallowCopy(aObj)
		merged["enum"] = mergeEnums(aEnum, bEnum)
		return merged
	}

	return a
}

// mergeEnums 合并两个 enum 列表（去重）。
func mergeEnums(a, b []any) []any {
	seen := map[string]bool{}
	var result []any
	for _, v := range a {
		key := anyToString(v)
		if !seen[key] {
			seen[key] = true
			result = append(result, v)
		}
	}
	for _, v := range b {
		key := anyToString(v)
		if !seen[key] {
			seen[key] = true
			result = append(result, v)
		}
	}
	return result
}

// ---------- 辅助 ----------

func copyTool(t *AgentTool) *AgentTool {
	out := *t
	return &out
}

func shallowCopy(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func hasRequired(params map[string]any) bool {
	v, ok := params["required"]
	if !ok {
		return false
	}
	arr, ok := v.([]any)
	return ok && len(arr) > 0
}

func extractStringArray(params map[string]any, key string) []string {
	arr, ok := params[key].([]any)
	if !ok {
		return nil
	}
	var result []string
	for _, v := range arr {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

func toAnySlice[T any](s []T) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}

func anyToString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
