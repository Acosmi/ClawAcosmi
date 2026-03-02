package runner

import (
	"encoding/json"
	"testing"

	"github.com/openacosmi/claw-acismi/internal/agents/llmclient"
)

// helper: 构建 ToolDef
func makeTool(name string, schema map[string]interface{}) llmclient.ToolDef {
	data, _ := json.Marshal(schema)
	return llmclient.ToolDef{Name: name, InputSchema: data}
}

// helper: 解析结果 schema
func parseSchema(t *testing.T, tool llmclient.ToolDef) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal(tool.InputSchema, &result); err != nil {
		t.Fatalf("unmarshal result schema: %v", err)
	}
	return result
}

// ---------- AUDIT-1: extractEnumValues 处理 const ----------

func TestExtractEnumValues_Const(t *testing.T) {
	schema := map[string]interface{}{"const": "fixed_value"}
	values := extractEnumValues(schema)
	if len(values) != 1 || values[0] != "fixed_value" {
		t.Errorf("expected [fixed_value], got %v", values)
	}
}

// ---------- AUDIT-2: extractEnumValues 递归嵌套 anyOf/oneOf ----------

func TestExtractEnumValues_RecursiveAnyOf(t *testing.T) {
	schema := map[string]interface{}{
		"anyOf": []interface{}{
			map[string]interface{}{"const": "a"},
			map[string]interface{}{"const": "b"},
			map[string]interface{}{
				"oneOf": []interface{}{
					map[string]interface{}{"enum": []interface{}{"c", "d"}},
				},
			},
		},
	}
	values := extractEnumValues(schema)
	expected := map[string]bool{"a": true, "b": true, "c": true, "d": true}
	if len(values) != 4 {
		t.Fatalf("expected 4 values, got %d: %v", len(values), values)
	}
	for _, v := range values {
		s, ok := v.(string)
		if !ok {
			t.Errorf("expected string value, got %T", v)
			continue
		}
		if !expected[s] {
			t.Errorf("unexpected value: %s", s)
		}
	}
}

// ---------- AUDIT-3: required 合并基于 count 判断 ----------

func TestNormalize_RequiredCountBased(t *testing.T) {
	// 两个 variants：都要求 "action"，只有一个要求 "value"
	schema := map[string]interface{}{
		"anyOf": []interface{}{
			map[string]interface{}{
				"properties": map[string]interface{}{
					"action": map[string]interface{}{"type": "string"},
					"value":  map[string]interface{}{"type": "string"},
				},
				"required": []interface{}{"action", "value"},
			},
			map[string]interface{}{
				"properties": map[string]interface{}{
					"action": map[string]interface{}{"type": "string"},
					"label":  map[string]interface{}{"type": "string"},
				},
				"required": []interface{}{"action"},
			},
		},
	}
	tool := makeTool("test", schema)
	result := NormalizeToolParameters(tool)
	resultSchema := parseSchema(t, result)

	required, ok := resultSchema["required"].([]interface{})
	if !ok {
		t.Fatalf("expected required array, got %T", resultSchema["required"])
	}

	// 只有 "action" 在两个 variants 中都被要求
	if len(required) != 1 {
		t.Errorf("expected 1 required field, got %d: %v", len(required), required)
	}
	if len(required) > 0 && required[0] != "action" {
		t.Errorf("expected required[0]='action', got %v", required[0])
	}
}

// ---------- AUDIT-4: 保留 additionalProperties ----------

func TestNormalize_PreservesAdditionalProperties(t *testing.T) {
	// CleanToolSchemaForGemini strips additionalProperties from final output,
	// so test mergeUnionVariants directly to verify the logic.
	variants := []interface{}{
		map[string]interface{}{
			"properties": map[string]interface{}{
				"name": map[string]interface{}{"type": "string"},
			},
		},
	}
	topSchema := map[string]interface{}{"additionalProperties": false}
	_, _, ap := mergeUnionVariants(variants, topSchema)
	if ap != false {
		t.Errorf("expected additionalProperties=false, got %v", ap)
	}
}

func TestNormalize_DefaultAdditionalProperties(t *testing.T) {
	// No explicit additionalProperties → default true.
	variants := []interface{}{
		map[string]interface{}{
			"properties": map[string]interface{}{
				"name": map[string]interface{}{"type": "string"},
			},
		},
	}
	topSchema := map[string]interface{}{}
	_, _, ap := mergeUnionVariants(variants, topSchema)
	if ap != true {
		t.Errorf("expected additionalProperties=true (default), got %v", ap)
	}
}

// ---------- AUDIT-5: early-return 和 fallback ----------

func TestNormalize_TypePropsNoAnyOf_CleanOnly(t *testing.T) {
	// 分支 1: 有 type + properties + 无 anyOf → 仅清洗
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string"},
		},
	}
	tool := makeTool("test", schema)
	result := NormalizeToolParameters(tool)
	resultSchema := parseSchema(t, result)
	if resultSchema["type"] != "object" {
		t.Errorf("expected type=object, got %v", resultSchema["type"])
	}
}

func TestNormalize_TypePropsWithAnyOf_MergesVariants(t *testing.T) {
	// 有 type + properties + 有 anyOf → 不 early-return，走合并逻辑
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"base": map[string]interface{}{"type": "string"},
		},
		"anyOf": []interface{}{
			map[string]interface{}{
				"properties": map[string]interface{}{
					"action": map[string]interface{}{"type": "string"},
				},
			},
		},
	}
	tool := makeTool("test", schema)
	result := NormalizeToolParameters(tool)
	resultSchema := parseSchema(t, result)
	props, _ := resultSchema["properties"].(map[string]interface{})
	if props == nil {
		t.Fatal("expected properties in result")
	}
	if _, ok := props["action"]; !ok {
		t.Error("expected 'action' in merged properties")
	}
}

func TestNormalize_EmptyMergedProperties_Fallback(t *testing.T) {
	// anyOf variants 无 properties → fallback 到 schema.properties
	schema := map[string]interface{}{
		"properties": map[string]interface{}{
			"fallback_field": map[string]interface{}{"type": "string"},
		},
		"anyOf": []interface{}{
			map[string]interface{}{"type": "string"}, // 无 properties
		},
	}
	tool := makeTool("test", schema)
	result := NormalizeToolParameters(tool)
	resultSchema := parseSchema(t, result)

	props, _ := resultSchema["properties"].(map[string]interface{})
	if props == nil {
		t.Fatal("expected properties in result")
	}
	if _, ok := props["fallback_field"]; !ok {
		t.Error("expected fallback to schema.properties when merged is empty")
	}
}

// ---------- 基线测试 ----------

func TestNormalize_NoSchema(t *testing.T) {
	tool := llmclient.ToolDef{Name: "empty"}
	result := NormalizeToolParameters(tool)
	if result.Name != "empty" {
		t.Errorf("expected name=empty, got %v", result.Name)
	}
}

func TestNormalize_NoVariantKey(t *testing.T) {
	// 没有 anyOf/oneOf/type/properties → 直接返回
	schema := map[string]interface{}{
		"$ref": "#/definitions/Foo",
	}
	tool := makeTool("test", schema)
	result := NormalizeToolParameters(tool)
	resultSchema := parseSchema(t, result)
	if _, ok := resultSchema["$ref"]; !ok {
		t.Error("expected $ref preserved when no variant key")
	}
}

func TestNormalize_TopLevelRequired_Preserved(t *testing.T) {
	// 顶层 schema.required 优先于 variant-level
	schema := map[string]interface{}{
		"required": []interface{}{"must_have"},
		"anyOf": []interface{}{
			map[string]interface{}{
				"properties": map[string]interface{}{
					"must_have": map[string]interface{}{"type": "string"},
					"optional":  map[string]interface{}{"type": "string"},
				},
				"required": []interface{}{"optional"},
			},
		},
	}
	tool := makeTool("test", schema)
	result := NormalizeToolParameters(tool)
	resultSchema := parseSchema(t, result)

	required, ok := resultSchema["required"].([]interface{})
	if !ok {
		t.Fatalf("expected required as array, got %T", resultSchema["required"])
	}
	found := false
	for _, v := range required {
		if v == "must_have" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'must_have' in required from top-level, got %v", required)
	}
}

func TestExtractEnumValues_DirectEnum(t *testing.T) {
	schema := map[string]interface{}{
		"enum": []interface{}{"a", "b", "c"},
	}
	values := extractEnumValues(schema)
	if len(values) != 3 {
		t.Errorf("expected 3 values, got %d", len(values))
	}
}

func TestExtractEnumValues_Nil(t *testing.T) {
	values := extractEnumValues(nil)
	if values != nil {
		t.Errorf("expected nil for nil input, got %v", values)
	}

	values = extractEnumValues("not an object")
	if values != nil {
		t.Errorf("expected nil for string input, got %v", values)
	}
}

func TestMergePropertySchemas_EnumMerge_Dedup(t *testing.T) {
	a := map[string]interface{}{"enum": []interface{}{"x", "y"}, "type": "string"}
	b := map[string]interface{}{"enum": []interface{}{"y", "z"}, "type": "string"}
	result := mergePropertySchemas(a, b)
	obj, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	enum, ok := obj["enum"].([]interface{})
	if !ok {
		t.Fatal("expected enum in merged result")
	}
	if len(enum) != 3 {
		t.Errorf("expected 3 deduplicated enum values, got %d: %v", len(enum), enum)
	}
}
