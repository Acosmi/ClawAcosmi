package security

import (
	"testing"
)

func TestParseJSONC_StandardJSON(t *testing.T) {
	data := []byte(`{"key": "value", "count": 42}`)
	var result map[string]interface{}
	if err := ParseJSONC(data, &result); err != nil {
		t.Fatalf("ParseJSONC should parse standard JSON: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("expected key=value, got %v", result["key"])
	}
	if result["count"] != float64(42) {
		t.Errorf("expected count=42, got %v", result["count"])
	}
}

func TestParseJSONC_LineComments(t *testing.T) {
	data := []byte(`{
		// This is a line comment
		"gateway": {
			"port": 18789 // inline comment
		}
	}`)
	var result map[string]interface{}
	if err := ParseJSONC(data, &result); err != nil {
		t.Fatalf("ParseJSONC should handle // comments: %v", err)
	}
	gw, ok := result["gateway"].(map[string]interface{})
	if !ok {
		t.Fatal("expected gateway to be a map")
	}
	if gw["port"] != float64(18789) {
		t.Errorf("expected port=18789, got %v", gw["port"])
	}
}

func TestParseJSONC_BlockComments(t *testing.T) {
	data := []byte(`{
		/* block comment */
		"auth": {
			"token": "secret123"
		}
	}`)
	var result map[string]interface{}
	if err := ParseJSONC(data, &result); err != nil {
		t.Fatalf("ParseJSONC should handle /* */ comments: %v", err)
	}
	auth, ok := result["auth"].(map[string]interface{})
	if !ok {
		t.Fatal("expected auth to be a map")
	}
	if auth["token"] != "secret123" {
		t.Errorf("expected token=secret123, got %v", auth["token"])
	}
}

func TestParseJSONC_TrailingCommas(t *testing.T) {
	data := []byte(`{
		"items": [
			"a",
			"b",
			"c",
		],
		"nested": {
			"x": 1,
		},
	}`)
	var result map[string]interface{}
	if err := ParseJSONC(data, &result); err != nil {
		t.Fatalf("ParseJSONC should handle trailing commas: %v", err)
	}
	items, ok := result["items"].([]interface{})
	if !ok {
		t.Fatal("expected items to be an array")
	}
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}
}

func TestParseJSONC_InvalidJSON(t *testing.T) {
	data := []byte(`{invalid json content`)
	var result map[string]interface{}
	if err := ParseJSONC(data, &result); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseJSONC_EmptyObject(t *testing.T) {
	data := []byte(`{}`)
	var result map[string]interface{}
	if err := ParseJSONC(data, &result); err != nil {
		t.Fatalf("ParseJSONC should handle empty object: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestParseJSONC_MixedCommentsAndTrailingComma(t *testing.T) {
	// 模拟真实 OpenAcosmi 配置文件场景
	data := []byte(`{
		// Gateway configuration
		"gateway": {
			"port": 18789,
			"auth": {
				/* Use env var for production */
				"password": "${OPENACOSMI_GATEWAY_PASSWORD}",
				"token": "dev-token-123", // dev only
			},
		},
		// Channels
		"channels": {
			"telegram": {
				"groupPolicy": "open", // should be "allowlist"
			},
		},
	}`)
	var result map[string]interface{}
	if err := ParseJSONC(data, &result); err != nil {
		t.Fatalf("ParseJSONC should handle real-world JSONC config: %v", err)
	}
	gw := result["gateway"].(map[string]interface{})
	auth := gw["auth"].(map[string]interface{})
	if auth["password"] != "${OPENACOSMI_GATEWAY_PASSWORD}" {
		t.Errorf("unexpected password: %v", auth["password"])
	}
	channels := result["channels"].(map[string]interface{})
	tg := channels["telegram"].(map[string]interface{})
	if tg["groupPolicy"] != "open" {
		t.Errorf("unexpected groupPolicy: %v", tg["groupPolicy"])
	}
}
