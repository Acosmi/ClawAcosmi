package runner

import (
	"encoding/json"
	"testing"

	"github.com/anthropic/open-acosmi/internal/agents/llmclient"
)

func TestIsGoogleProvider(t *testing.T) {
	tests := []struct {
		provider string
		want     bool
	}{
		{"google-gemini-cli", true},
		{"google-generative-ai", true},
		{"google-antigravity", true},
		{"anthropic", false},
		{"openai", false},
		{"ollama", false},
		{"", false},
	}
	for _, tt := range tests {
		got := IsGoogleProvider(tt.provider)
		if got != tt.want {
			t.Errorf("IsGoogleProvider(%q) = %v, want %v", tt.provider, got, tt.want)
		}
	}
}

func TestCleanToolSchemaForGemini_RemovesUnsupportedKeywords(t *testing.T) {
	input := json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {"type": "string", "minLength": 1, "maxLength": 100},
			"age": {"type": "number", "minimum": 0, "maximum": 150}
		},
		"additionalProperties": false,
		"$schema": "http://json-schema.org/draft-07/schema#"
	}`)

	result := CleanToolSchemaForGemini(input)

	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	// additionalProperties, $schema should be removed
	if _, ok := parsed["additionalProperties"]; ok {
		t.Error("additionalProperties should be removed")
	}
	if _, ok := parsed["$schema"]; ok {
		t.Error("$schema should be removed")
	}

	// properties should be present
	props, ok := parsed["properties"]
	if !ok {
		t.Fatal("properties should be present")
	}
	propsMap := props.(map[string]interface{})

	// minLength, maxLength should be removed from name
	nameSchema := propsMap["name"].(map[string]interface{})
	if _, ok := nameSchema["minLength"]; ok {
		t.Error("minLength should be removed from name")
	}
	if _, ok := nameSchema["maxLength"]; ok {
		t.Error("maxLength should be removed from name")
	}

	// minimum, maximum should be removed from age
	ageSchema := propsMap["age"].(map[string]interface{})
	if _, ok := ageSchema["minimum"]; ok {
		t.Error("minimum should be removed from age")
	}
	if _, ok := ageSchema["maximum"]; ok {
		t.Error("maximum should be removed from age")
	}
}

func TestCleanToolSchemaForGemini_ConstToEnum(t *testing.T) {
	input := json.RawMessage(`{"const": "fixed_value"}`)
	result := CleanToolSchemaForGemini(input)

	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	enumVals, ok := parsed["enum"]
	if !ok {
		t.Fatal("const should be converted to enum")
	}
	arr := enumVals.([]interface{})
	if len(arr) != 1 || arr[0] != "fixed_value" {
		t.Errorf("expected enum=[fixed_value], got %v", arr)
	}
}

func TestSanitizeToolsForGoogle_NonGooglePassthrough(t *testing.T) {
	tools := []llmclient.ToolDef{
		{Name: "test", Description: "desc", InputSchema: json.RawMessage(`{"type":"object","$schema":"foo"}`)},
	}
	result := SanitizeToolsForGoogle(tools, "anthropic")
	// 非 Google provider 直接返回原始数据
	if len(result) != 1 || result[0].Name != "test" {
		t.Error("non-Google provider should pass through tools")
	}
	// schema 应未被修改
	var parsed map[string]interface{}
	json.Unmarshal(result[0].InputSchema, &parsed)
	if _, ok := parsed["$schema"]; !ok {
		t.Error("non-Google provider should not clean schema")
	}
}

func TestSanitizeToolsForGoogle_GoogleCleans(t *testing.T) {
	tools := []llmclient.ToolDef{
		{Name: "test", Description: "desc", InputSchema: json.RawMessage(`{"type":"object","$schema":"foo","minLength":1}`)},
	}
	result := SanitizeToolsForGoogle(tools, "google-gemini-cli")
	var parsed map[string]interface{}
	json.Unmarshal(result[0].InputSchema, &parsed)
	if _, ok := parsed["$schema"]; ok {
		t.Error("$schema should be removed for Google provider")
	}
	if _, ok := parsed["minLength"]; ok {
		t.Error("minLength should be removed for Google provider")
	}
}

func TestSanitizeGoogleTurnOrdering_AssistantFirst(t *testing.T) {
	messages := []llmclient.ChatMessage{
		llmclient.TextMessage("assistant", "Hello"),
		llmclient.TextMessage("user", "Hi"),
	}
	result, didPrepend := SanitizeGoogleTurnOrdering(messages)
	if !didPrepend {
		t.Fatal("should have prepended bootstrap")
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Errorf("first message should be user, got %s", result[0].Role)
	}
	if result[0].Content[0].Text != "(session bootstrap)" {
		t.Errorf("bootstrap text mismatch: %s", result[0].Content[0].Text)
	}
}

func TestSanitizeGoogleTurnOrdering_UserFirstNoChange(t *testing.T) {
	messages := []llmclient.ChatMessage{
		llmclient.TextMessage("user", "Hello"),
		llmclient.TextMessage("assistant", "Hi"),
	}
	result, didPrepend := SanitizeGoogleTurnOrdering(messages)
	if didPrepend {
		t.Error("should not have prepended bootstrap when user-first")
	}
	if len(result) != 2 {
		t.Errorf("expected 2 messages, got %d", len(result))
	}
}

func TestSanitizeGoogleTurnOrdering_AlreadyBootstrapped(t *testing.T) {
	messages := []llmclient.ChatMessage{
		llmclient.TextMessage("user", "(session bootstrap)"),
		llmclient.TextMessage("assistant", "Hi"),
	}
	result, didPrepend := SanitizeGoogleTurnOrdering(messages)
	if didPrepend {
		t.Error("should not have prepended bootstrap when already present")
	}
	if len(result) != 2 {
		t.Errorf("expected 2 messages, got %d", len(result))
	}
}

func TestIsValidAntigravitySignature(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  bool
	}{
		{"valid base64", "SGVsbG8gV29ybGQ=", true},
		{"valid base64 no padding", "AQIDBAUG", true},
		{"msg_ prefix invalid", "msg_abc123", false},
		{"empty string", "", false},
		{"non-string", 123, false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidAntigravitySignature(tt.input)
			if got != tt.want {
				t.Errorf("IsValidAntigravitySignature(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFindUnsupportedSchemaKeywords(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {"type": "string", "minLength": 1},
			"nested": {"type": "object", "additionalProperties": true}
		}
	}`)
	violations := FindUnsupportedSchemaKeywords(schema)
	if len(violations) == 0 {
		t.Error("expected violations")
	}
	found := map[string]bool{}
	for _, v := range violations {
		found[v] = true
	}
	if !found["root.properties.name.minLength"] {
		t.Errorf("expected minLength violation, got %v", violations)
	}
	if !found["root.properties.nested.additionalProperties"] {
		t.Errorf("expected additionalProperties violation, got %v", violations)
	}
}

func TestIsSameModelSnapshot(t *testing.T) {
	a := ModelSnapshotEntry{Provider: "google-antigravity", ModelAPI: "gemini", ModelID: "gemini-2.0-flash"}
	b := ModelSnapshotEntry{Provider: "google-antigravity", ModelAPI: "gemini", ModelID: "gemini-2.0-flash"}
	c := ModelSnapshotEntry{Provider: "anthropic", ModelAPI: "claude", ModelID: "claude-sonnet-4"}

	if !IsSameModelSnapshot(a, b) {
		t.Error("same snapshots should be equal")
	}
	if IsSameModelSnapshot(a, c) {
		t.Error("different snapshots should not be equal")
	}
}
