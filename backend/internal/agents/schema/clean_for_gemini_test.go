package schema

import (
	"encoding/json"
	"reflect"
	"testing"
)

// ---------- typebox.go ----------

func TestStringEnum(t *testing.T) {
	s := StringEnum([]string{"a", "b", "c"})
	if s["type"] != "string" {
		t.Errorf("type = %v", s["type"])
	}
	enums := s["enum"].([]any)
	if len(enums) != 3 || enums[0] != "a" {
		t.Errorf("enum = %v", enums)
	}
}

func TestStringEnumWithDescription(t *testing.T) {
	s := StringEnum([]string{"x"}, map[string]any{"description": "test"})
	if s["description"] != "test" {
		t.Errorf("description = %v", s["description"])
	}
}

func TestChannelTargetSchema(t *testing.T) {
	s := ChannelTargetSchema("")
	if s["type"] != "string" {
		t.Errorf("type = %v", s["type"])
	}
	if s["description"].(string) == "" {
		t.Error("description should have default")
	}
}

func TestTypeObject(t *testing.T) {
	s := TypeObject(map[string]any{
		"name": TypeString(nil),
	}, []string{"name"})
	if s["type"] != "object" {
		t.Errorf("type = %v", s["type"])
	}
}

// ---------- clean_for_gemini.go ----------

func TestCleanSchemaForGemini_RemovesUnsupportedKeywords(t *testing.T) {
	schema := map[string]any{
		"type":      "string",
		"minLength": 1,
		"maxLength": 100,
		"pattern":   "^[a-z]+$",
		"format":    "email",
	}
	cleaned := CleanSchemaForGemini(schema)
	obj := cleaned.(map[string]any)
	if _, ok := obj["minLength"]; ok {
		t.Error("minLength should be removed")
	}
	if _, ok := obj["maxLength"]; ok {
		t.Error("maxLength should be removed")
	}
	if _, ok := obj["pattern"]; ok {
		t.Error("pattern should be removed")
	}
	if _, ok := obj["format"]; ok {
		t.Error("format should be removed")
	}
	if obj["type"] != "string" {
		t.Errorf("type should be preserved, got %v", obj["type"])
	}
}

func TestCleanSchemaForGemini_Const(t *testing.T) {
	schema := map[string]any{
		"const": "literal",
	}
	cleaned := CleanSchemaForGemini(schema)
	obj := cleaned.(map[string]any)
	enums, ok := obj["enum"].([]any)
	if !ok || len(enums) != 1 || enums[0] != "literal" {
		t.Errorf("const should become enum, got %v", obj)
	}
}

func TestCleanSchemaForGemini_NullableType(t *testing.T) {
	schema := map[string]any{
		"type": []any{"string", "null"},
	}
	cleaned := CleanSchemaForGemini(schema)
	obj := cleaned.(map[string]any)
	if obj["type"] != "string" {
		t.Errorf("type should be 'string', got %v", obj["type"])
	}
}

func TestCleanSchemaForGemini_AnyOfLiterals(t *testing.T) {
	schema := map[string]any{
		"anyOf": []any{
			map[string]any{"type": "string", "const": "a"},
			map[string]any{"type": "string", "const": "b"},
		},
	}
	cleaned := CleanSchemaForGemini(schema)
	obj := cleaned.(map[string]any)
	if obj["type"] != "string" {
		t.Errorf("type should be 'string', got %v", obj["type"])
	}
	enums := obj["enum"].([]any)
	if len(enums) != 2 || enums[0] != "a" || enums[1] != "b" {
		t.Errorf("enum should be [a, b], got %v", enums)
	}
}

func TestCleanSchemaForGemini_NullVariantStrip(t *testing.T) {
	schema := map[string]any{
		"anyOf": []any{
			map[string]any{"type": "string"},
			map[string]any{"type": "null"},
		},
	}
	cleaned := CleanSchemaForGemini(schema)
	obj := cleaned.(map[string]any)
	// 剥离 null 后只剩 1 个 → 展平
	if obj["type"] != "string" {
		t.Errorf("expected type=string after null strip, got %v", obj)
	}
}

func TestCleanSchemaForGemini_RefResolution(t *testing.T) {
	schema := map[string]any{
		"$defs": map[string]any{
			"Name": map[string]any{"type": "string", "description": "name field"},
		},
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"$ref": "#/$defs/Name"},
		},
	}
	cleaned := CleanSchemaForGemini(schema)
	obj := cleaned.(map[string]any)
	props := obj["properties"].(map[string]any)
	name := props["name"].(map[string]any)
	if name["type"] != "string" {
		t.Errorf("$ref should be resolved, got %v", name)
	}
	if name["description"] != "name field" {
		t.Errorf("description from $ref should be preserved")
	}
}

func TestCleanSchemaForGemini_NestedProperties(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"age": map[string]any{
				"type":    "number",
				"minimum": 0,
				"maximum": 150,
			},
		},
	}
	cleaned := CleanSchemaForGemini(schema)
	obj := cleaned.(map[string]any)
	props := obj["properties"].(map[string]any)
	age := props["age"].(map[string]any)
	if _, ok := age["minimum"]; ok {
		t.Error("minimum should be removed from nested")
	}
	if _, ok := age["maximum"]; ok {
		t.Error("maximum should be removed from nested")
	}
}

func TestCleanSchemaForGemini_Nil(t *testing.T) {
	if CleanSchemaForGemini(nil) != nil {
		t.Error("nil should return nil")
	}
}

func TestCleanSchemaForGemini_Passthrough(t *testing.T) {
	if CleanSchemaForGemini("hello") != "hello" {
		t.Error("primitive should pass through")
	}
}

func TestTryFlattenLiteralAnyOf(t *testing.T) {
	tests := []struct {
		name     string
		variants []any
		wantNil  bool
		wantType string
		wantLen  int
	}{
		{
			name: "string consts",
			variants: []any{
				map[string]any{"type": "string", "const": "a"},
				map[string]any{"type": "string", "const": "b"},
			},
			wantType: "string",
			wantLen:  2,
		},
		{
			name: "mixed types",
			variants: []any{
				map[string]any{"type": "string", "const": "a"},
				map[string]any{"type": "number", "const": 1},
			},
			wantNil: true,
		},
		{
			name:     "empty",
			variants: []any{},
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		result := tryFlattenLiteralAnyOf(tt.variants)
		if tt.wantNil && result != nil {
			t.Errorf("%s: expected nil, got %v", tt.name, result)
		}
		if !tt.wantNil && result != nil {
			if result["type"] != tt.wantType {
				t.Errorf("%s: type = %v, want %v", tt.name, result["type"], tt.wantType)
			}
			if len(result["enum"].([]any)) != tt.wantLen {
				t.Errorf("%s: enum len = %d, want %d", tt.name, len(result["enum"].([]any)), tt.wantLen)
			}
		}
	}
}

func TestIsNullSchema(t *testing.T) {
	tests := []struct {
		name   string
		schema any
		want   bool
	}{
		{"type null", map[string]any{"type": "null"}, true},
		{"const null", map[string]any{"const": nil}, true},
		{"enum null", map[string]any{"enum": []any{nil}}, true},
		{"type string", map[string]any{"type": "string"}, false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		got := isNullSchema(tt.schema)
		if got != tt.want {
			t.Errorf("isNullSchema(%s) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// 验证 JSON Schema 清洗后仍可序列化。
func TestCleanSchemaForGemini_Serializable(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"anyOf": []any{
					map[string]any{"type": "string", "const": "get"},
					map[string]any{"type": "string", "const": "set"},
					map[string]any{"type": "null"},
				},
			},
		},
		"$defs":     map[string]any{"Foo": map[string]any{"type": "string"}},
		"minLength": 1,
	}
	cleaned := CleanSchemaForGemini(schema)
	data, err := json.Marshal(cleaned)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	// $defs & minLength should be gone
	if _, ok := result["$defs"]; ok {
		t.Error("$defs should be removed")
	}
	if _, ok := result["minLength"]; ok {
		t.Error("minLength should be removed")
	}
	// action should be flattened to enum
	props := result["properties"].(map[string]any)
	action := props["action"].(map[string]any)
	if !reflect.DeepEqual(action["enum"], []any{"get", "set"}) {
		t.Errorf("action enum = %v", action["enum"])
	}
}
