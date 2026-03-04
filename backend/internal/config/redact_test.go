package config

import (
	"testing"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ============================================================
// IsSensitiveKey 测试
// ============================================================

func TestIsSensitiveKey(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"token", true},
		{"botToken", true},
		{"password", true},
		{"secret", true},
		{"apiKey", true},
		{"api_key", true},
		{"API_KEY", true},
		{"name", false},
		{"model", false},
		{"host", false},
		{"enabled", false},
	}
	for _, tt := range tests {
		if got := IsSensitiveKey(tt.key); got != tt.want {
			t.Errorf("IsSensitiveKey(%q) = %v, want %v", tt.key, got, tt.want)
		}
	}
}

// ============================================================
// RedactConfigObject 测试
// ============================================================

func TestRedactConfigObject_ReplacesTokens(t *testing.T) {
	obj := map[string]interface{}{
		"name":     "test-agent",
		"botToken": "sk-12345",
		"nested": map[string]interface{}{
			"password": "hunter2",
			"host":     "localhost",
		},
	}
	result := RedactConfigObject(obj).(map[string]interface{})

	if result["name"] != "test-agent" {
		t.Error("should not redact non-sensitive fields")
	}
	if result["botToken"] != RedactedSentinel {
		t.Errorf("expected botToken=%q, got %v", RedactedSentinel, result["botToken"])
	}
	nested := result["nested"].(map[string]interface{})
	if nested["password"] != RedactedSentinel {
		t.Error("should redact nested password")
	}
	if nested["host"] != "localhost" {
		t.Error("should not redact non-sensitive nested fields")
	}
}

func TestRedactConfigObject_Nil(t *testing.T) {
	if RedactConfigObject(nil) != nil {
		t.Error("nil input should return nil")
	}
}

func TestRedactConfigObject_Array(t *testing.T) {
	obj := []interface{}{
		map[string]interface{}{"token": "abc123"},
		map[string]interface{}{"name": "test"},
	}
	result := RedactConfigObject(obj).([]interface{})
	first := result[0].(map[string]interface{})
	if first["token"] != RedactedSentinel {
		t.Error("should redact token in array elements")
	}
	second := result[1].(map[string]interface{})
	if second["name"] != "test" {
		t.Error("should preserve non-sensitive fields in array")
	}
}

func TestRedactConfigObject_SkipsNilValues(t *testing.T) {
	obj := map[string]interface{}{
		"token": nil,
		"name":  "test",
	}
	result := RedactConfigObject(obj).(map[string]interface{})
	if result["token"] != nil {
		t.Error("nil sensitive values should remain nil")
	}
}

// ============================================================
// RestoreRedactedValues 测试
// ============================================================

func TestRestoreRedactedValues_RestoresCredentials(t *testing.T) {
	incoming := map[string]interface{}{
		"name":     "test-agent",
		"botToken": RedactedSentinel,
		"nested": map[string]interface{}{
			"password": RedactedSentinel,
			"host":     "new-host",
		},
	}
	original := map[string]interface{}{
		"name":     "test-agent",
		"botToken": "sk-12345",
		"nested": map[string]interface{}{
			"password": "hunter2",
			"host":     "localhost",
		},
	}
	result, err := RestoreRedactedValues(incoming, original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := result.(map[string]interface{})
	if m["botToken"] != "sk-12345" {
		t.Errorf("expected restored botToken, got %v", m["botToken"])
	}
	nested := m["nested"].(map[string]interface{})
	if nested["password"] != "hunter2" {
		t.Error("should restore nested password")
	}
	if nested["host"] != "new-host" {
		t.Error("should keep new non-sensitive value")
	}
}

func TestRestoreRedactedValues_RejectsUnknownRedactedKey(t *testing.T) {
	incoming := map[string]interface{}{
		"apiKey": RedactedSentinel,
	}
	original := map[string]interface{}{}
	_, err := RestoreRedactedValues(incoming, original)
	if err == nil {
		t.Fatal("should error when redacted key not in original")
	}
}

func TestRestoreRedactedValues_KeepsExplicitValue(t *testing.T) {
	incoming := map[string]interface{}{
		"token": "new-token-value",
	}
	original := map[string]interface{}{
		"token": "old-token-value",
	}
	result, err := RestoreRedactedValues(incoming, original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := result.(map[string]interface{})
	if m["token"] != "new-token-value" {
		t.Error("should keep explicit new value")
	}
}

func TestRestoreRedactedValues_Array(t *testing.T) {
	incoming := []interface{}{
		map[string]interface{}{"password": RedactedSentinel},
	}
	original := []interface{}{
		map[string]interface{}{"password": "secret123"},
	}
	result, err := RestoreRedactedValues(incoming, original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	arr := result.([]interface{})
	m := arr[0].(map[string]interface{})
	if m["password"] != "secret123" {
		t.Error("should restore password in array element")
	}
}

// ============================================================
// RedactConfigSnapshot 测试
// ============================================================

func TestRedactConfigSnapshot_RedactsParsedAndRaw(t *testing.T) {
	raw := `{"botToken": "sk-12345", "name": "test"}`
	parsed := map[string]interface{}{
		"botToken": "sk-12345",
		"name":     "test",
	}
	snapshot := &types.ConfigFileSnapshot{
		Path:   "/test/config.json5",
		Exists: true,
		Raw:    &raw,
		Parsed: parsed,
		Valid:  true,
	}
	result := RedactConfigSnapshot(snapshot)

	// Raw 中应被替换
	if result.Raw == nil || !contains(*result.Raw, RedactedSentinel) {
		t.Error("Raw text should contain redacted sentinel")
	}
	if contains(*result.Raw, "sk-12345") {
		t.Error("Raw text should not contain original token")
	}

	// Parsed 中应被替换
	p := result.Parsed.(map[string]interface{})
	if p["botToken"] != RedactedSentinel {
		t.Error("Parsed botToken should be redacted")
	}
	if p["name"] != "test" {
		t.Error("Parsed name should be preserved")
	}

	// 不应修改原快照
	origP := snapshot.Parsed.(map[string]interface{})
	if origP["botToken"] != "sk-12345" {
		t.Error("original snapshot should not be modified")
	}
}

func TestRedactConfigSnapshot_Nil(t *testing.T) {
	if RedactConfigSnapshot(nil) != nil {
		t.Error("nil snapshot should return nil")
	}
}

func TestRedactRawText_NoSensitive(t *testing.T) {
	raw := `{"name": "test", "host": "localhost"}`
	config := map[string]interface{}{"name": "test", "host": "localhost"}
	result := RedactRawText(raw, config)
	if result != raw {
		t.Error("should not modify raw text without sensitive values")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchInString(s, substr)
}

func searchInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
