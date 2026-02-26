package argus

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropic/open-acosmi/internal/mcpclient"
)

// ---------- IsAvailable 测试 ----------

func TestIsAvailable_EmptyPath(t *testing.T) {
	if IsAvailable("") {
		t.Error("expected false for empty path")
	}
}

func TestIsAvailable_NonExistent(t *testing.T) {
	if IsAvailable("/nonexistent/argus-sensory-xyz-12345") {
		t.Error("expected false for non-existent path")
	}
}

func TestIsAvailable_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "argus-sensory")
	if err := os.WriteFile(tmpFile, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !IsAvailable(tmpFile) {
		t.Error("expected true for existing file")
	}
}

// ---------- NewBridge + 状态 测试 ----------

func TestNewBridge_InitialState(t *testing.T) {
	cfg := DefaultBridgeConfig()
	b := NewBridge(cfg)

	if b.State() != BridgeStateInit {
		t.Errorf("expected init state, got %s", b.State())
	}
	if b.PID() != 0 {
		t.Errorf("expected PID 0, got %d", b.PID())
	}
	if len(b.Tools()) != 0 {
		t.Errorf("expected 0 tools, got %d", len(b.Tools()))
	}
}

func TestDefaultBridgeConfig(t *testing.T) {
	cfg := DefaultBridgeConfig()
	if cfg.BinaryPath != "argus-sensory" {
		t.Errorf("expected binary path 'argus-sensory', got %q", cfg.BinaryPath)
	}
	if len(cfg.Args) != 1 || cfg.Args[0] != "-mcp" {
		t.Errorf("expected args [-mcp], got %v", cfg.Args)
	}
	if cfg.HealthInterval != defaultHealthInterval {
		t.Errorf("expected health interval %v, got %v", defaultHealthInterval, cfg.HealthInterval)
	}
}

func TestBridge_StartWithInvalidBinary(t *testing.T) {
	cfg := DefaultBridgeConfig()
	cfg.BinaryPath = "/nonexistent/argus-sensory-xyz-12345"
	b := NewBridge(cfg)

	err := b.Start()
	if err == nil {
		t.Fatal("expected error for non-existent binary")
	}

	// 启动失败后应回到 stopped 状态
	if b.State() != BridgeStateStopped {
		t.Errorf("expected stopped state after failed start, got %s", b.State())
	}
}

func TestBridge_StopIdempotent(t *testing.T) {
	cfg := DefaultBridgeConfig()
	b := NewBridge(cfg)

	// 从未启动的 bridge 调用 Stop 不应 panic
	b.mu.Lock()
	b.state = BridgeStateStopped
	b.mu.Unlock()

	b.Stop() // 第一次
	b.Stop() // 第二次 — 幂等
}

func TestBridge_CallToolNotAvailable(t *testing.T) {
	cfg := DefaultBridgeConfig()
	b := NewBridge(cfg)

	// 未启动时调用 CallTool 应报错
	_, err := b.CallTool(nil, "capture_screen", nil, 0)
	if err == nil {
		t.Fatal("expected error when bridge not started")
	}
}

// ---------- BuildArgusSkillEntries 测试 ----------

func TestBuildArgusSkillEntries_AllKnownTools(t *testing.T) {
	tools := []mcpclient.MCPToolDef{
		{Name: "capture_screen", Description: "Capture screenshot"},
		{Name: "click", Description: "Click at position"},
		{Name: "run_shell", Description: "Run shell command"},
		{Name: "macos_shortcut", Description: "macOS shortcut"},
	}

	entries := BuildArgusSkillEntries(tools)

	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}

	tests := []struct {
		idx      int
		name     string
		category string
		risk     string
		emoji    string
	}{
		{0, "argus.capture_screen", "perception", "low", "eye"},
		{1, "argus.click", "action", "medium", "pointer"},
		{2, "argus.run_shell", "shell", "high", "terminal"},
		{3, "argus.macos_shortcut", "macos", "medium", "apple"},
	}

	for _, tc := range tests {
		e := entries[tc.idx]
		if e.Name != tc.name {
			t.Errorf("[%d] name: expected %q, got %q", tc.idx, tc.name, e.Name)
		}
		if e.Category != tc.category {
			t.Errorf("[%d] category: expected %q, got %q", tc.idx, tc.category, e.Category)
		}
		if e.Risk != tc.risk {
			t.Errorf("[%d] risk: expected %q, got %q", tc.idx, tc.risk, e.Risk)
		}
		if e.Emoji != tc.emoji {
			t.Errorf("[%d] emoji: expected %q, got %q", tc.idx, tc.emoji, e.Emoji)
		}
		if e.Source != "argus" {
			t.Errorf("[%d] source: expected 'argus', got %q", tc.idx, e.Source)
		}
		if !e.Eligible {
			t.Errorf("[%d] expected eligible=true", tc.idx)
		}
		if e.SkillKey != tc.name {
			t.Errorf("[%d] skillKey: expected %q, got %q", tc.idx, tc.name, e.SkillKey)
		}
	}
}

func TestBuildArgusSkillEntries_UnknownTool(t *testing.T) {
	tools := []mcpclient.MCPToolDef{
		{Name: "future_tool", Description: "Some new tool"},
	}

	entries := BuildArgusSkillEntries(tools)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e.Category != "unknown" {
		t.Errorf("expected category 'unknown' for unrecognized tool, got %q", e.Category)
	}
	if e.Risk != "medium" {
		t.Errorf("expected risk 'medium' for unrecognized tool, got %q", e.Risk)
	}
	if e.Emoji != "tool" {
		t.Errorf("expected emoji 'tool' for unknown category, got %q", e.Emoji)
	}
}

func TestBuildArgusSkillEntries_Empty(t *testing.T) {
	entries := BuildArgusSkillEntries(nil)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for nil tools, got %d", len(entries))
	}
}

func TestBuildArgusSkillEntries_JSONSerialization(t *testing.T) {
	tools := []mcpclient.MCPToolDef{
		{Name: "capture_screen", Description: "Capture screenshot"},
	}

	entries := BuildArgusSkillEntries(tools)
	data, err := json.Marshal(entries[0])
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// 验证关键字段存在
	for _, key := range []string{"name", "description", "source", "skillKey", "category", "risk", "eligible", "requirements", "missing"} {
		if _, ok := m[key]; !ok {
			t.Errorf("missing JSON field %q", key)
		}
	}
}

// ---------- emojiForCategory 测试 ----------

func TestEmojiForCategory(t *testing.T) {
	tests := []struct {
		category string
		expected string
	}{
		{"perception", "eye"},
		{"Perception", "eye"},
		{"action", "pointer"},
		{"shell", "terminal"},
		{"macos", "apple"},
		{"unknown", "tool"},
		{"", "tool"},
	}

	for _, tc := range tests {
		got := emojiForCategory(tc.category)
		if got != tc.expected {
			t.Errorf("emojiForCategory(%q): expected %q, got %q", tc.category, tc.expected, got)
		}
	}
}

// ---------- slogWriter 测试 ----------

func TestSlogWriter(t *testing.T) {
	w := &slogWriter{prefix: "test"}

	// 不应 panic
	n, err := w.Write([]byte("hello world\n"))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != 12 {
		t.Errorf("expected 12 bytes written, got %d", n)
	}

	// 空行
	n, err = w.Write([]byte("\n"))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 byte written, got %d", n)
	}
}
