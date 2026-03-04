package runner

// ============================================================================
// normalizeToolParameters — 工具参数 schema 规范化
// TS 对照: pi-tools.schema.ts → normalizeToolParameters()
//
// 职责:
// 1) 将 anyOf/oneOf union schema 扁平化为单一 object schema
// 2) 强制顶层 type:"object" 兼容 OpenAI
// 3) 调用 CleanToolSchemaForGemini 清洗不支持的关键字
// ============================================================================

import (
	"encoding/json"
	"sort"

	"github.com/Acosmi/ClawAcosmi/internal/agents/llmclient"
)

// NormalizeToolParameters 规范化工具参数 JSON Schema。
// 合并 anyOf/oneOf union 变体为单一 object schema，确保顶层 type:"object"。
// TS 对照: pi-tools.schema.ts → normalizeToolParameters()
func NormalizeToolParameters(tool llmclient.ToolDef) llmclient.ToolDef {
	if len(tool.InputSchema) == 0 {
		return tool
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
		return tool
	}

	// 分支 1: 已有 type + properties 且无顶层 anyOf → 仅清洗
	// TS 对照: L83 — "type" in schema && "properties" in schema && !Array.isArray(schema.anyOf)
	if _, hasType := schema["type"]; hasType {
		if _, hasProps := schema["properties"]; hasProps {
			if !isArray(schema["anyOf"]) {
				return rebuildTool(tool, CleanToolSchemaForGemini(tool.InputSchema))
			}
		}
	}

	// 分支 2: 缺 type 但有 properties 或 required（且无 anyOf/oneOf）→ 补 type:"object"
	// TS 对照: L92-102
	_, hasProps := schema["properties"]
	_, hasRequired := schema["required"]
	if hasProps || hasRequired {
		if !isArray(schema["anyOf"]) && !isArray(schema["oneOf"]) {
			schema["type"] = "object"
			result, err := json.Marshal(schema)
			if err != nil {
				return tool
			}
			return rebuildTool(tool, CleanToolSchemaForGemini(result))
		}
	}

	// 分支 3: anyOf/oneOf → 合并 properties + required
	// TS 对照: L104-174
	var variantKey string
	var variants []interface{}
	if v, ok := schema["anyOf"]; ok {
		if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
			variantKey = "anyOf"
			variants = arr
		}
	}
	if variantKey == "" {
		if v, ok := schema["oneOf"]; ok {
			if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
				variantKey = "oneOf"
				variants = arr
			}
		}
	}

	if variantKey == "" {
		// TS 对照: L109 — !variantKey → return tool (不做额外处理)
		return tool
	}

	mergedProps, mergedRequired, additionalProps := mergeUnionVariants(variants, schema)

	// 构建结果 schema
	nextSchema := make(map[string]interface{})
	nextSchema["type"] = "object"

	// 保留原 schema 的 title/description
	if v, ok := schema["title"]; ok {
		if s, ok := v.(string); ok {
			nextSchema["title"] = s
		}
	}
	if v, ok := schema["description"]; ok {
		if s, ok := v.(string); ok {
			nextSchema["description"] = s
		}
	}

	// TS 对照: L169-170 — 合并结果为空时回退到 schema.properties
	if len(mergedProps) > 0 {
		nextSchema["properties"] = mergedProps
	} else if origProps, ok := schema["properties"]; ok {
		nextSchema["properties"] = origProps
	} else {
		nextSchema["properties"] = map[string]interface{}{}
	}

	if len(mergedRequired) > 0 {
		nextSchema["required"] = mergedRequired
	}

	// TS 对照: L172 — 保留 additionalProperties
	nextSchema["additionalProperties"] = additionalProps

	result, err := json.Marshal(nextSchema)
	if err != nil {
		return tool
	}
	return rebuildTool(tool, CleanToolSchemaForGemini(result))
}

// mergeUnionVariants 合并 anyOf/oneOf 变体的 properties 和 required。
// 返回合并后的 properties、required 列表和 additionalProperties 值。
// TS 对照: pi-tools.schema.ts L113-172
func mergeUnionVariants(variants []interface{}, topSchema map[string]interface{}) (
	map[string]interface{}, []string, interface{},
) {
	mergedProps := make(map[string]interface{})
	requiredCounts := make(map[string]int)
	objectVariants := 0

	for _, v := range variants {
		obj, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		props, ok := obj["properties"].(map[string]interface{})
		if !ok {
			continue
		}
		objectVariants++

		// 合并 properties
		for key, val := range props {
			if existing, exists := mergedProps[key]; !exists {
				mergedProps[key] = val
			} else {
				mergedProps[key] = mergePropertySchemas(existing, val)
			}
		}

		// 计数 required
		if req, ok := obj["required"].([]interface{}); ok {
			for _, r := range req {
				if s, ok := r.(string); ok {
					requiredCounts[s]++
				}
			}
		}
	}

	// TS 对照: L144-154 — 确定 required 列表
	// 优先使用顶层 schema.required；否则仅保留所有 variants 都要求的字段
	var mergedRequired []string
	if baseReq, ok := topSchema["required"].([]interface{}); ok && len(baseReq) > 0 {
		for _, r := range baseReq {
			if s, ok := r.(string); ok {
				mergedRequired = append(mergedRequired, s)
			}
		}
	} else if objectVariants > 0 {
		for key, count := range requiredCounts {
			if count == objectVariants {
				mergedRequired = append(mergedRequired, key)
			}
		}
	}

	// 排序保证确定性输出
	sort.Strings(mergedRequired)

	// TS 对照: L172 — additionalProperties
	additionalProps := interface{}(true)
	if v, ok := topSchema["additionalProperties"]; ok {
		additionalProps = v
	}

	return mergedProps, mergedRequired, additionalProps
}

// mergePropertySchemas 合并两个属性 schema（特别处理 enum 合并）。
// TS 对照: pi-tools.schema.ts → mergePropertySchemas()
func mergePropertySchemas(existing, incoming interface{}) interface{} {
	if existing == nil {
		return incoming
	}
	if incoming == nil {
		return existing
	}

	existingEnum := extractEnumValues(existing)
	incomingEnum := extractEnumValues(incoming)

	if existingEnum != nil || incomingEnum != nil {
		// 合并 enum 值（去重）
		seen := make(map[interface{}]bool)
		var values []interface{}
		for _, v := range existingEnum {
			if !seen[v] {
				seen[v] = true
				values = append(values, v)
			}
		}
		for _, v := range incomingEnum {
			if !seen[v] {
				seen[v] = true
				values = append(values, v)
			}
		}

		merged := make(map[string]interface{})
		// 保留 title/description/default
		for _, source := range []interface{}{existing, incoming} {
			if obj, ok := source.(map[string]interface{}); ok {
				for _, key := range []string{"title", "description", "default"} {
					if _, exists := merged[key]; !exists {
						if v, ok := obj[key]; ok {
							merged[key] = v
						}
					}
				}
			}
		}

		// 推断 type
		typeSet := make(map[string]bool)
		for _, v := range values {
			switch v.(type) {
			case string:
				typeSet["string"] = true
			case float64:
				typeSet["number"] = true
			case bool:
				typeSet["boolean"] = true
			}
		}
		if len(typeSet) == 1 {
			for t := range typeSet {
				merged["type"] = t
			}
		}

		merged["enum"] = values
		return merged
	}

	return existing
}

// extractEnumValues 从 schema 中提取 enum 值。
// TS 对照: pi-tools.schema.ts → extractEnumValues()
// 支持: enum 直接提取、const 转 [const]、递归处理嵌套 anyOf/oneOf
func extractEnumValues(schema interface{}) []interface{} {
	obj, ok := schema.(map[string]interface{})
	if !ok {
		return nil
	}

	// 直接 enum 数组
	if enumVal, ok := obj["enum"]; ok {
		if arr, ok := enumVal.([]interface{}); ok {
			return arr
		}
	}

	// AUDIT-1: const → [const]
	if constVal, ok := obj["const"]; ok {
		return []interface{}{constVal}
	}

	// AUDIT-2: 递归处理嵌套 anyOf/oneOf
	var variantArr []interface{}
	if v, ok := obj["anyOf"]; ok {
		if arr, ok := v.([]interface{}); ok {
			variantArr = arr
		}
	}
	if variantArr == nil {
		if v, ok := obj["oneOf"]; ok {
			if arr, ok := v.([]interface{}); ok {
				variantArr = arr
			}
		}
	}
	if variantArr != nil {
		var values []interface{}
		for _, variant := range variantArr {
			extracted := extractEnumValues(variant)
			values = append(values, extracted...)
		}
		if len(values) > 0 {
			return values
		}
	}

	return nil
}

// isArray 检查值是否为 []interface{} 类型。
func isArray(v interface{}) bool {
	if v == nil {
		return false
	}
	_, ok := v.([]interface{})
	return ok
}

// rebuildTool 用新的 InputSchema 构建 ToolDef。
func rebuildTool(tool llmclient.ToolDef, schema json.RawMessage) llmclient.ToolDef {
	return llmclient.ToolDef{
		Name:        tool.Name,
		Description: tool.Description,
		InputSchema: schema,
	}
}
