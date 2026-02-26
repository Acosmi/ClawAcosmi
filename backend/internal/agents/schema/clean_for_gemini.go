// Package schema — Gemini JSON Schema 清洗工具。
// TS 参考：src/agents/schema/clean-for-gemini.ts (376L)
package schema

import (
	"regexp"
	"strings"
)

// GeminiUnsupportedSchemaKeywords Cloud Code Assist API 拒绝的 JSON Schema 关键字。
var GeminiUnsupportedSchemaKeywords = map[string]bool{
	"patternProperties":    true,
	"additionalProperties": true,
	"$schema":              true,
	"$id":                  true,
	"$ref":                 true,
	"$defs":                true,
	"definitions":          true,
	"examples":             true,
	"minLength":            true,
	"maxLength":            true,
	"minimum":              true,
	"maximum":              true,
	"multipleOf":           true,
	"pattern":              true,
	"format":               true,
	"minItems":             true,
	"maxItems":             true,
	"uniqueItems":          true,
	"minProperties":        true,
	"maxProperties":        true,
}

// CleanSchemaForGemini 清洗 JSON Schema 以兼容 Gemini API。
// 入口函数 — 处理 $ref、anyOf/oneOf 合并、不支持关键字剔除等。
func CleanSchemaForGemini(schema any) any {
	if schema == nil {
		return schema
	}
	obj, ok := schema.(map[string]any)
	if !ok {
		if arr, ok := schema.([]any); ok {
			return mapSlice(arr, func(item any) any {
				return CleanSchemaForGemini(item)
			})
		}
		return schema
	}

	defs := extendSchemaDefs(nil, obj)
	return cleanSchemaForGeminiWithDefs(schema, defs, nil)
}

// ---------- 内部实现 ----------

type schemaDefs = map[string]any

func cleanSchemaForGeminiWithDefs(schema any, defs schemaDefs, refStack map[string]bool) any {
	if schema == nil {
		return schema
	}
	if arr, ok := schema.([]any); ok {
		return mapSlice(arr, func(item any) any {
			return cleanSchemaForGeminiWithDefs(item, defs, refStack)
		})
	}
	obj, ok := schema.(map[string]any)
	if !ok {
		return schema
	}

	nextDefs := extendSchemaDefs(defs, obj)

	// $ref 解析
	if refValue, ok := obj["$ref"].(string); ok && refValue != "" {
		if refStack != nil && refStack[refValue] {
			return map[string]any{} // 循环引用 → 空 schema
		}

		resolved := tryResolveLocalRef(refValue, nextDefs)
		if resolved != nil {
			nextRefStack := copyStringSet(refStack)
			nextRefStack[refValue] = true
			cleaned := cleanSchemaForGeminiWithDefs(resolved, nextDefs, nextRefStack)
			if cleanedObj, ok := cleaned.(map[string]any); ok {
				result := shallowCopyMap(cleanedObj)
				copyMetaKeys(obj, result)
				return result
			}
			return cleaned
		}

		// 无法解析 → 保留 meta 信息
		result := map[string]any{}
		copyMetaKeys(obj, result)
		return result
	}

	// anyOf / oneOf 处理
	hasAnyOf := isSlice(obj["anyOf"])
	hasOneOf := isSlice(obj["oneOf"])

	var cleanedAnyOf []any
	var cleanedOneOf []any

	if hasAnyOf {
		cleanedAnyOf = mapSlice(obj["anyOf"].([]any), func(v any) any {
			return cleanSchemaForGeminiWithDefs(v, nextDefs, refStack)
		})
	}
	if hasOneOf {
		cleanedOneOf = mapSlice(obj["oneOf"].([]any), func(v any) any {
			return cleanSchemaForGeminiWithDefs(v, nextDefs, refStack)
		})
	}

	if hasAnyOf {
		nonNull, stripped := stripNullVariants(cleanedAnyOf)
		if stripped {
			cleanedAnyOf = nonNull
		}
		if flattened := tryFlattenLiteralAnyOf(nonNull); flattened != nil {
			result := map[string]any{
				"type": flattened["type"],
				"enum": flattened["enum"],
			}
			copyMetaKeys(obj, result)
			return result
		}
		if stripped && len(nonNull) == 1 {
			if lone, ok := nonNull[0].(map[string]any); ok {
				result := shallowCopyMap(lone)
				copyMetaKeys(obj, result)
				return result
			}
			return nonNull[0]
		}
	}

	if hasOneOf {
		nonNull, stripped := stripNullVariants(cleanedOneOf)
		if stripped {
			cleanedOneOf = nonNull
		}
		if flattened := tryFlattenLiteralAnyOf(nonNull); flattened != nil {
			result := map[string]any{
				"type": flattened["type"],
				"enum": flattened["enum"],
			}
			copyMetaKeys(obj, result)
			return result
		}
		if stripped && len(nonNull) == 1 {
			if lone, ok := nonNull[0].(map[string]any); ok {
				result := shallowCopyMap(lone)
				copyMetaKeys(obj, result)
				return result
			}
			return nonNull[0]
		}
	}

	// 构建已清洗输出
	cleaned := map[string]any{}

	for key, value := range obj {
		if GeminiUnsupportedSchemaKeywords[key] {
			continue
		}

		if key == "const" {
			cleaned["enum"] = []any{value}
			continue
		}

		if key == "type" && (hasAnyOf || hasOneOf) {
			continue
		}

		// type: ["string", "null"] → type: "string"
		if key == "type" {
			if arr, ok := value.([]any); ok && allStrings(arr) {
				filtered := filterStrings(arr, func(s string) bool { return s != "null" })
				if len(filtered) == 1 {
					cleaned["type"] = filtered[0]
				} else {
					cleaned["type"] = toAnySliceStr(filtered)
				}
				continue
			}
		}

		if key == "properties" {
			if props, ok := value.(map[string]any); ok {
				cleanedProps := map[string]any{}
				for k, v := range props {
					cleanedProps[k] = cleanSchemaForGeminiWithDefs(v, nextDefs, refStack)
				}
				cleaned[key] = cleanedProps
			} else {
				cleaned[key] = value
			}
		} else if key == "items" {
			if arr, ok := value.([]any); ok {
				cleaned[key] = mapSlice(arr, func(item any) any {
					return cleanSchemaForGeminiWithDefs(item, nextDefs, refStack)
				})
			} else if _, ok := value.(map[string]any); ok {
				cleaned[key] = cleanSchemaForGeminiWithDefs(value, nextDefs, refStack)
			} else {
				cleaned[key] = value
			}
		} else if key == "anyOf" && isSlice(value) {
			if cleanedAnyOf != nil {
				cleaned[key] = cleanedAnyOf
			} else {
				cleaned[key] = mapSlice(value.([]any), func(v any) any {
					return cleanSchemaForGeminiWithDefs(v, nextDefs, refStack)
				})
			}
		} else if key == "oneOf" && isSlice(value) {
			if cleanedOneOf != nil {
				cleaned[key] = cleanedOneOf
			} else {
				cleaned[key] = mapSlice(value.([]any), func(v any) any {
					return cleanSchemaForGeminiWithDefs(v, nextDefs, refStack)
				})
			}
		} else if key == "allOf" && isSlice(value) {
			cleaned[key] = mapSlice(value.([]any), func(v any) any {
				return cleanSchemaForGeminiWithDefs(v, nextDefs, refStack)
			})
		} else {
			cleaned[key] = value
		}
	}

	return cleaned
}

// tryFlattenLiteralAnyOf 尝试将 anyOf/oneOf 中的 literal 值展平为 enum。
func tryFlattenLiteralAnyOf(variants []any) map[string]any {
	if len(variants) == 0 {
		return nil
	}

	var allValues []any
	var commonType string

	for _, variant := range variants {
		v, ok := variant.(map[string]any)
		if !ok {
			return nil
		}

		var literalValue any
		if constVal, hasConst := v["const"]; hasConst {
			literalValue = constVal
		} else if enumVal, hasEnum := v["enum"]; hasEnum {
			if arr, ok := enumVal.([]any); ok && len(arr) == 1 {
				literalValue = arr[0]
			} else {
				return nil
			}
		} else {
			return nil
		}

		variantType, _ := v["type"].(string)
		if variantType == "" {
			return nil
		}
		if commonType == "" {
			commonType = variantType
		} else if commonType != variantType {
			return nil
		}

		allValues = append(allValues, literalValue)
	}

	if commonType != "" && len(allValues) > 0 {
		return map[string]any{
			"type": commonType,
			"enum": allValues,
		}
	}
	return nil
}

// isNullSchema 判断是否为 null schema。
func isNullSchema(variant any) bool {
	if variant == nil {
		return false
	}
	record, ok := variant.(map[string]any)
	if !ok {
		return false
	}
	if constVal, hasConst := record["const"]; hasConst && constVal == nil {
		return true
	}
	if enumVal, hasEnum := record["enum"]; hasEnum {
		if arr, ok := enumVal.([]any); ok && len(arr) == 1 && arr[0] == nil {
			return true
		}
	}
	if typeVal, hasType := record["type"]; hasType {
		if s, ok := typeVal.(string); ok && s == "null" {
			return true
		}
		if arr, ok := typeVal.([]any); ok && len(arr) == 1 {
			if s, ok := arr[0].(string); ok && s == "null" {
				return true
			}
		}
	}
	return false
}

// stripNullVariants 从 variants 中移除 null schema。
func stripNullVariants(variants []any) ([]any, bool) {
	if len(variants) == 0 {
		return variants, false
	}
	var nonNull []any
	for _, v := range variants {
		if !isNullSchema(v) {
			nonNull = append(nonNull, v)
		}
	}
	stripped := len(nonNull) != len(variants)
	return nonNull, stripped
}

// extendSchemaDefs 收集 $defs 和 definitions。
func extendSchemaDefs(defs schemaDefs, schema map[string]any) schemaDefs {
	defsEntry := extractObjectField(schema, "$defs")
	legacyDefsEntry := extractObjectField(schema, "definitions")

	if defsEntry == nil && legacyDefsEntry == nil {
		return defs
	}

	next := make(schemaDefs)
	for k, v := range defs {
		next[k] = v
	}
	if defsEntry != nil {
		for k, v := range defsEntry {
			next[k] = v
		}
	}
	if legacyDefsEntry != nil {
		for k, v := range legacyDefsEntry {
			next[k] = v
		}
	}
	return next
}

var localRefPattern = regexp.MustCompile(`^#/(?:\$defs|definitions)/(.+)$`)

// tryResolveLocalRef 解析本地 $ref 引用。
func tryResolveLocalRef(ref string, defs schemaDefs) any {
	if defs == nil {
		return nil
	}
	matches := localRefPattern.FindStringSubmatch(ref)
	if matches == nil || len(matches) < 2 {
		return nil
	}
	name := decodeJsonPointerSegment(matches[1])
	if name == "" {
		return nil
	}
	return defs[name]
}

// decodeJsonPointerSegment 解码 JSON Pointer 段。
func decodeJsonPointerSegment(segment string) string {
	s := strings.ReplaceAll(segment, "~1", "/")
	s = strings.ReplaceAll(s, "~0", "~")
	return s
}

// ---------- 辅助函数 ----------

func extractObjectField(m map[string]any, key string) map[string]any {
	v, ok := m[key]
	if !ok {
		return nil
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	return obj
}

func copyMetaKeys(src, dst map[string]any) {
	for _, key := range []string{"description", "title", "default"} {
		if v, ok := src[key]; ok && v != nil {
			dst[key] = v
		}
	}
}

func shallowCopyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func copyStringSet(s map[string]bool) map[string]bool {
	out := make(map[string]bool, len(s)+1)
	for k, v := range s {
		out[k] = v
	}
	return out
}

func isSlice(v any) bool {
	_, ok := v.([]any)
	return ok
}

func allStrings(arr []any) bool {
	for _, item := range arr {
		if _, ok := item.(string); !ok {
			return false
		}
	}
	return true
}

func filterStrings(arr []any, pred func(string) bool) []string {
	var out []string
	for _, item := range arr {
		if s, ok := item.(string); ok && pred(s) {
			out = append(out, s)
		}
	}
	return out
}

func toAnySliceStr(s []string) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}

func mapSlice(arr []any, fn func(any) any) []any {
	out := make([]any, len(arr))
	for i, v := range arr {
		out[i] = fn(v)
	}
	return out
}
