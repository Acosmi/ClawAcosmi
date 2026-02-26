package config

import (
	"testing"
)

// mockEnv 创建一个模拟的环境变量查找函数
func mockEnv(vars map[string]string) EnvLookupFunc {
	return func(key string) (string, bool) {
		v, ok := vars[key]
		return v, ok
	}
}

func TestSubstituteString(t *testing.T) {
	env := mockEnv(map[string]string{
		"FOO":     "hello",
		"BAR_BAZ": "world",
		"EMPTY":   "",
	})

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"no substitution", "plain text", "plain text", false},
		{"simple var", "${FOO}", "hello", false},
		{"var in middle", "pre-${FOO}-post", "pre-hello-post", false},
		{"two vars", "${FOO} ${BAR_BAZ}", "hello world", false},
		{"underscore var", "${BAR_BAZ}", "world", false},
		{"escape", "$${FOO}", "${FOO}", false},
		{"missing var", "${MISSING_VAR}", "", true},
		{"empty var treated as missing", "${EMPTY}", "", true},
		{"no dollar", "no dollars here", "no dollars here", false},
		{"lone dollar", "cost is $5", "cost is $5", false},
		{"dollar without brace", "$FOO", "$FOO", false},
		{"unclosed brace", "${FOO", "${FOO", false},
		{"lowercase var passthrough", "${foo}", "${foo}", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := substituteString(tc.input, env, "test.path")
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if _, ok := err.(*MissingEnvVarError); !ok {
					t.Fatalf("expected MissingEnvVarError, got %T", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveConfigEnvVarsWithLookup(t *testing.T) {
	env := mockEnv(map[string]string{
		"API_KEY": "sk-test-123",
		"PORT":    "8080",
	})

	t.Run("nested object", func(t *testing.T) {
		input := map[string]interface{}{
			"models": map[string]interface{}{
				"apiKey": "${API_KEY}",
				"port":   "${PORT}",
			},
			"name": "literal",
			"num":  42.0,
		}
		result, err := ResolveConfigEnvVarsWithLookup(input, env)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		m := result.(map[string]interface{})
		models := m["models"].(map[string]interface{})
		if models["apiKey"] != "sk-test-123" {
			t.Fatalf("apiKey = %v, want sk-test-123", models["apiKey"])
		}
		if models["port"] != "8080" {
			t.Fatalf("port = %v, want 8080", models["port"])
		}
		if m["name"] != "literal" {
			t.Fatalf("name = %v, want literal", m["name"])
		}
		if m["num"] != 42.0 {
			t.Fatalf("num = %v, want 42", m["num"])
		}
	})

	t.Run("array", func(t *testing.T) {
		input := []interface{}{"${API_KEY}", "literal", 123}
		result, err := ResolveConfigEnvVarsWithLookup(input, env)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		arr := result.([]interface{})
		if arr[0] != "sk-test-123" {
			t.Fatalf("arr[0] = %v, want sk-test-123", arr[0])
		}
		if arr[1] != "literal" {
			t.Fatalf("arr[1] = %v, want literal", arr[1])
		}
		if arr[2] != 123 {
			t.Fatalf("arr[2] = %v, want 123", arr[2])
		}
	})

	t.Run("error propagates from nested", func(t *testing.T) {
		input := map[string]interface{}{
			"deep": map[string]interface{}{
				"key": "${NONEXISTENT}",
			},
		}
		_, err := ResolveConfigEnvVarsWithLookup(input, env)
		if err == nil {
			t.Fatal("expected error")
		}
		mErr, ok := err.(*MissingEnvVarError)
		if !ok {
			t.Fatalf("expected MissingEnvVarError, got %T", err)
		}
		if mErr.VarName != "NONEXISTENT" {
			t.Fatalf("VarName = %q, want NONEXISTENT", mErr.VarName)
		}
		if mErr.ConfigPath != "deep.key" {
			t.Fatalf("ConfigPath = %q, want deep.key", mErr.ConfigPath)
		}
	})

	t.Run("nil passthrough", func(t *testing.T) {
		result, err := ResolveConfigEnvVarsWithLookup(nil, env)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil {
			t.Fatalf("got %v, want nil", result)
		}
	})
}

func TestMissingEnvVarError(t *testing.T) {
	err := &MissingEnvVarError{VarName: "MY_VAR", ConfigPath: "models.apiKey"}
	expected := `Missing env var "MY_VAR" referenced at config path: models.apiKey`
	if err.Error() != expected {
		t.Fatalf("got %q, want %q", err.Error(), expected)
	}
}
