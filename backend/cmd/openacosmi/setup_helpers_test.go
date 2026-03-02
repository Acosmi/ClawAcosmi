package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

// ---------- HandleReset 测试 ----------

func TestHandleReset_Config(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.json")
	os.WriteFile(configFile, []byte(`{}`), 0o644)

	// 需要临时覆盖 resolveConfigPath
	origEnv := os.Getenv("OPENACOSMI_CONFIG")
	os.Setenv("OPENACOSMI_CONFIG", configFile)
	defer os.Setenv("OPENACOSMI_CONFIG", origEnv)

	if err := HandleReset(ResetConfig, dir); err != nil {
		t.Fatalf("HandleReset config: %v", err)
	}
	if _, err := os.Stat(configFile); !os.IsNotExist(err) {
		t.Error("config file should be deleted")
	}
}

func TestHandleReset_Full(t *testing.T) {
	dir := t.TempDir()

	configFile := filepath.Join(dir, "config.json")
	os.WriteFile(configFile, []byte(`{}`), 0o644)

	wsDir := filepath.Join(dir, "workspace")
	os.MkdirAll(wsDir, 0o755)
	os.WriteFile(filepath.Join(wsDir, "test.txt"), []byte("test"), 0o644)

	origEnv := os.Getenv("OPENACOSMI_CONFIG")
	os.Setenv("OPENACOSMI_CONFIG", configFile)
	defer os.Setenv("OPENACOSMI_CONFIG", origEnv)

	if err := HandleReset(ResetFull, wsDir); err != nil {
		t.Fatalf("HandleReset full: %v", err)
	}
	if _, err := os.Stat(configFile); !os.IsNotExist(err) {
		t.Error("config file should be deleted")
	}
	if _, err := os.Stat(wsDir); !os.IsNotExist(err) {
		t.Error("workspace dir should be deleted")
	}
}

// ---------- DetectBinary 测试 ----------

func TestDetectBinary_Go(t *testing.T) {
	// "go" 应该在 PATH 中
	if !DetectBinary("go") {
		t.Skip("go not in PATH")
	}
}

func TestDetectBinary_Nonexistent(t *testing.T) {
	if DetectBinary("this_binary_does_not_exist_zzzz") {
		t.Error("should not detect non-existent binary")
	}
}

func TestDetectBinary_EmptyString(t *testing.T) {
	if DetectBinary("") {
		t.Error("empty string should return false")
	}
	if DetectBinary("  ") {
		t.Error("whitespace-only should return false")
	}
}

func TestDetectBinary_AbsolutePath(t *testing.T) {
	// 创建临时可执行文件
	dir := t.TempDir()
	binPath := filepath.Join(dir, "mybin")
	os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755)

	if !DetectBinary(binPath) {
		t.Error("should detect existing absolute path")
	}
	if DetectBinary(filepath.Join(dir, "nonexistent")) {
		t.Error("should not detect non-existent absolute path")
	}
}

// ---------- MoveToTrash 测试 ----------

func TestMoveToTrash_EmptyPath(t *testing.T) {
	if err := MoveToTrash(""); err != nil {
		t.Errorf("empty path should be no-op: %v", err)
	}
}

func TestMoveToTrash_Nonexistent(t *testing.T) {
	if err := MoveToTrash("/tmp/this_does_not_exist_zzzz"); err != nil {
		t.Errorf("nonexistent path should be no-op: %v", err)
	}
}

func TestMoveToTrash_FallbackRemove(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "to_delete.txt")
	os.WriteFile(testFile, []byte("delete me"), 0o644)

	if err := MoveToTrash(testFile); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("file should be removed")
	}
}

// ---------- SummarizeExistingConfig 测试 ----------

func TestSummarizeExistingConfig_Nil(t *testing.T) {
	result := SummarizeExistingConfig(nil)
	if result != "No key settings detected." {
		t.Errorf("expected nil message, got %q", result)
	}
}

func TestSummarizeExistingConfig_Empty(t *testing.T) {
	result := SummarizeExistingConfig(&types.OpenAcosmiConfig{})
	if result != "No key settings detected." {
		t.Errorf("expected empty message, got %q", result)
	}
}

func TestSummarizeExistingConfig_WithData(t *testing.T) {
	port := 8080
	cfg := &types.OpenAcosmiConfig{
		Gateway: &types.GatewayConfig{
			Mode: "local",
			Port: &port,
		},
	}
	result := SummarizeExistingConfig(cfg)
	if result == "No key settings detected." {
		t.Error("should have some content")
	}
	if result == "" {
		t.Error("should not be empty")
	}
}

// ---------- ApplyWizardMetadata 测试 ----------

func TestApplyWizardMetadata(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	ApplyWizardMetadata(cfg, WizardMetadata{Command: "onboard", Mode: "local"})

	if cfg.Wizard == nil {
		t.Fatal("wizard should be set")
	}
	if cfg.Wizard.LastRunCommand != "onboard" {
		t.Errorf("expected command onboard, got %s", cfg.Wizard.LastRunCommand)
	}
	if cfg.Wizard.LastRunMode != "local" {
		t.Errorf("expected mode local, got %s", cfg.Wizard.LastRunMode)
	}
	if cfg.Wizard.LastRunAt == "" {
		t.Error("lastRunAt should be set")
	}
}

func TestApplyWizardMetadata_Nil(t *testing.T) {
	// Should not panic
	ApplyWizardMetadata(nil, WizardMetadata{Command: "test", Mode: "local"})
}

// ---------- GuardCancel 测试 ----------

func TestGuardCancel_NotCancelled(t *testing.T) {
	if err := GuardCancel(false); err != nil {
		t.Errorf("should not return error: %v", err)
	}
}

func TestGuardCancel_Cancelled(t *testing.T) {
	err := GuardCancel(true)
	if err == nil {
		t.Error("should return error when cancelled")
	}
}

// ---------- SummarizeError 测试 ----------

func TestSummarizeError_Nil(t *testing.T) {
	if got := SummarizeError(nil); got != "unknown error" {
		t.Errorf("expected 'unknown error', got %q", got)
	}
}

// ---------- DefaultWorkspace 测试 ----------

func TestDefaultWorkspace(t *testing.T) {
	if DefaultWorkspace != "agents" {
		t.Errorf("expected 'agents', got %q", DefaultWorkspace)
	}
}

// ---------- EnsureWorkspaceAndSessions 测试 ----------

func TestEnsureWorkspaceAndSessions(t *testing.T) {
	dir := t.TempDir()
	wsDir := filepath.Join(dir, "ws")
	if err := EnsureWorkspaceAndSessions(wsDir, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------- ResolveNodeManagerOptions 测试 ----------

func TestResolveNodeManagerOptions(t *testing.T) {
	opts := ResolveNodeManagerOptions()
	if len(opts) != 3 {
		t.Errorf("expected 3 options, got %d", len(opts))
	}
	expected := []string{"npm", "pnpm", "bun"}
	for i, opt := range opts {
		if opt.Value != expected[i] {
			t.Errorf("opts[%d] value=%s, want %s", i, opt.Value, expected[i])
		}
	}
}
