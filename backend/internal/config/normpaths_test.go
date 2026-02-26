package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeConfigPaths(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot get home dir")
	}

	t.Run("expand dir key", func(t *testing.T) {
		cfg := map[string]interface{}{
			"agentDir": "~/my-agent",
		}
		NormalizeConfigPaths(cfg)
		got := cfg["agentDir"].(string)
		want := filepath.Join(home, "my-agent")
		if !strings.HasPrefix(got, home) {
			t.Fatalf("got %q, want path starting with %q", got, home)
		}
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("expand path key", func(t *testing.T) {
		cfg := map[string]interface{}{
			"configPath": "~/config.json",
		}
		NormalizeConfigPaths(cfg)
		got := cfg["configPath"].(string)
		if !strings.HasPrefix(got, home) {
			t.Fatalf("got %q, should start with home dir", got)
		}
	})

	t.Run("expand workspace key", func(t *testing.T) {
		cfg := map[string]interface{}{
			"workspace": "~/ws",
		}
		NormalizeConfigPaths(cfg)
		got := cfg["workspace"].(string)
		if !strings.HasPrefix(got, home) {
			t.Fatalf("got %q, should start with home dir", got)
		}
	})

	t.Run("skip non-path key", func(t *testing.T) {
		cfg := map[string]interface{}{
			"name": "~/not-a-path",
		}
		NormalizeConfigPaths(cfg)
		if cfg["name"] != "~/not-a-path" {
			t.Fatalf("non-path key should not be expanded, got %q", cfg["name"])
		}
	})

	t.Run("skip non-tilde value", func(t *testing.T) {
		cfg := map[string]interface{}{
			"agentDir": "/absolute/path",
		}
		NormalizeConfigPaths(cfg)
		if cfg["agentDir"] != "/absolute/path" {
			t.Fatalf("non-tilde value should not change, got %q", cfg["agentDir"])
		}
	})

	t.Run("nested object", func(t *testing.T) {
		cfg := map[string]interface{}{
			"agents": map[string]interface{}{
				"agentDir": "~/agents",
			},
		}
		NormalizeConfigPaths(cfg)
		got := cfg["agents"].(map[string]interface{})["agentDir"].(string)
		if !strings.HasPrefix(got, home) {
			t.Fatalf("nested path key should be expanded, got %q", got)
		}
	})

	t.Run("paths list key expands children", func(t *testing.T) {
		cfg := map[string]interface{}{
			"paths": []interface{}{"~/bin", "/usr/local/bin"},
		}
		NormalizeConfigPaths(cfg)
		arr := cfg["paths"].([]interface{})
		got := arr[0].(string)
		if !strings.HasPrefix(got, home) {
			t.Fatalf("paths list item should be expanded, got %q", got)
		}
		if arr[1] != "/usr/local/bin" {
			t.Fatalf("non-tilde list item should not change, got %q", arr[1])
		}
	})

	t.Run("nil config", func(t *testing.T) {
		result := NormalizeConfigPaths(nil)
		if result != nil {
			t.Fatal("nil input should return nil")
		}
	})
}
