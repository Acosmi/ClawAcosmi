package config

import (
	"testing"
)

func TestParseConfigPath(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantParts []string
		wantErr   bool
	}{
		{"simple", "foo.bar", []string{"foo", "bar"}, false},
		{"single", "foo", []string{"foo"}, false},
		{"deep", "a.b.c.d", []string{"a", "b", "c", "d"}, false},
		{"with spaces", " foo . bar ", []string{"foo", "bar"}, false},
		{"empty string", "", nil, true},
		{"just spaces", "   ", nil, true},
		{"empty segment", "foo..bar", nil, true},
		{"trailing dot", "foo.bar.", nil, true},
		{"leading dot", ".foo.bar", nil, true},
		{"__proto__", "foo.__proto__", nil, true},
		{"prototype", "prototype.bar", nil, true},
		{"constructor", "foo.constructor", nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parts, errMsg := ParseConfigPath(tc.input)
			if tc.wantErr {
				if errMsg == "" {
					t.Fatal("expected error, got none")
				}
				if parts != nil {
					t.Fatalf("expected nil parts on error, got %v", parts)
				}
				return
			}
			if errMsg != "" {
				t.Fatalf("unexpected error: %s", errMsg)
			}
			if len(parts) != len(tc.wantParts) {
				t.Fatalf("got %v, want %v", parts, tc.wantParts)
			}
			for i, p := range parts {
				if p != tc.wantParts[i] {
					t.Fatalf("parts[%d] = %q, want %q", i, p, tc.wantParts[i])
				}
			}
		})
	}
}

func TestSetConfigValueAtPath(t *testing.T) {
	t.Run("simple set", func(t *testing.T) {
		root := map[string]interface{}{}
		SetConfigValueAtPath(root, []string{"foo", "bar"}, 42)
		v, ok := GetConfigValueAtPath(root, []string{"foo", "bar"})
		if !ok || v != 42 {
			t.Fatalf("got %v (ok=%v), want 42", v, ok)
		}
	})

	t.Run("overwrite non-map", func(t *testing.T) {
		root := map[string]interface{}{"foo": "string-value"}
		SetConfigValueAtPath(root, []string{"foo", "bar"}, 99)
		v, ok := GetConfigValueAtPath(root, []string{"foo", "bar"})
		if !ok || v != 99 {
			t.Fatalf("got %v (ok=%v), want 99", v, ok)
		}
	})

	t.Run("deep create", func(t *testing.T) {
		root := map[string]interface{}{}
		SetConfigValueAtPath(root, []string{"a", "b", "c"}, "deep")
		v, ok := GetConfigValueAtPath(root, []string{"a", "b", "c"})
		if !ok || v != "deep" {
			t.Fatalf("got %v (ok=%v), want deep", v, ok)
		}
	})
}

func TestUnsetConfigValueAtPath(t *testing.T) {
	t.Run("unset leaf", func(t *testing.T) {
		root := map[string]interface{}{
			"foo": map[string]interface{}{
				"bar": 42,
				"baz": 99,
			},
		}
		ok := UnsetConfigValueAtPath(root, []string{"foo", "bar"})
		if !ok {
			t.Fatal("expected true")
		}
		if _, exists := root["foo"].(map[string]interface{})["bar"]; exists {
			t.Fatal("bar should be deleted")
		}
		// baz should still exist
		if _, exists := root["foo"].(map[string]interface{})["baz"]; !exists {
			t.Fatal("baz should still exist")
		}
	})

	t.Run("cleanup empty parents", func(t *testing.T) {
		root := map[string]interface{}{
			"a": map[string]interface{}{
				"b": map[string]interface{}{
					"c": 1,
				},
			},
		}
		ok := UnsetConfigValueAtPath(root, []string{"a", "b", "c"})
		if !ok {
			t.Fatal("expected true")
		}
		if _, exists := root["a"]; exists {
			t.Fatal("empty parent chain should be cleaned up")
		}
	})

	t.Run("missing key returns false", func(t *testing.T) {
		root := map[string]interface{}{}
		ok := UnsetConfigValueAtPath(root, []string{"nonexistent"})
		if ok {
			t.Fatal("expected false for missing key")
		}
	})

	t.Run("non-map intermediate returns false", func(t *testing.T) {
		root := map[string]interface{}{"foo": "string"}
		ok := UnsetConfigValueAtPath(root, []string{"foo", "bar"})
		if ok {
			t.Fatal("expected false for non-map intermediate")
		}
	})
}

func TestGetConfigValueAtPath(t *testing.T) {
	root := map[string]interface{}{
		"a": map[string]interface{}{
			"b": 42,
		},
	}

	t.Run("found", func(t *testing.T) {
		v, ok := GetConfigValueAtPath(root, []string{"a", "b"})
		if !ok || v != 42 {
			t.Fatalf("got %v (ok=%v), want 42", v, ok)
		}
	})

	t.Run("missing", func(t *testing.T) {
		_, ok := GetConfigValueAtPath(root, []string{"a", "c"})
		if ok {
			t.Fatal("expected not found")
		}
	})

	t.Run("non-map intermediate", func(t *testing.T) {
		_, ok := GetConfigValueAtPath(root, []string{"a", "b", "c"})
		if ok {
			t.Fatal("expected not found through non-map")
		}
	})
}
