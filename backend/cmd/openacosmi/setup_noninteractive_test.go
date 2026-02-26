package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// ---------- InferAuthChoiceFromFlags 测试 ----------

func TestInferAuthChoice_NoFlags(t *testing.T) {
	result := InferAuthChoiceFromFlags(NonInteractiveOptions{})
	if result.Choice != "" {
		t.Errorf("expected empty, got %q", result.Choice)
	}
	if len(result.Matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(result.Matches))
	}
}

func TestInferAuthChoice_SingleApiKey(t *testing.T) {
	result := InferAuthChoiceFromFlags(NonInteractiveOptions{
		AnthropicApiKey: "sk-ant-test",
	})
	if result.Choice != AuthChoiceApiKey {
		t.Errorf("expected %q, got %q", AuthChoiceApiKey, result.Choice)
	}
	if len(result.Matches) != 1 {
		t.Errorf("expected 1 match, got %d", len(result.Matches))
	}
}

func TestInferAuthChoice_MultipleApiKeys(t *testing.T) {
	result := InferAuthChoiceFromFlags(NonInteractiveOptions{
		AnthropicApiKey: "sk-ant-test",
		GeminiApiKey:    "AIza-test",
	})
	if len(result.Matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(result.Matches))
	}
}

func TestInferAuthChoice_EmptyKeyIgnored(t *testing.T) {
	result := InferAuthChoiceFromFlags(NonInteractiveOptions{
		AnthropicApiKey: "  ",
	})
	if len(result.Matches) != 0 {
		t.Errorf("whitespace-only should be ignored, got %d matches", len(result.Matches))
	}
}

// ---------- RunNonInteractiveOnboarding 测试 ----------

func TestRunNonInteractive_InvalidMode(t *testing.T) {
	err := RunNonInteractiveOnboarding(NonInteractiveOptions{
		Mode: "invalid",
	})
	if err == nil {
		t.Error("expected error for invalid mode")
	}
}

func TestRunNonInteractive_RemoteMissingURL(t *testing.T) {
	err := RunNonInteractiveOnboarding(NonInteractiveOptions{
		Mode: "remote",
	})
	if err == nil {
		t.Error("expected error for missing remote URL")
	}
}

func TestRunNonInteractive_RemoteWithURL(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	origEnv := os.Getenv("OPENACOSMI_CONFIG")
	os.Setenv("OPENACOSMI_CONFIG", configPath)
	defer os.Setenv("OPENACOSMI_CONFIG", origEnv)

	err := RunNonInteractiveOnboarding(NonInteractiveOptions{
		Mode:      "remote",
		RemoteURL: "https://example.com:18789",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证配置文件写入
	cfg, exists := readConfigFileJSON(configPath)
	if !exists {
		t.Fatal("config file should exist")
	}
	if cfg.Gateway == nil || cfg.Gateway.Mode != "remote" {
		t.Error("gateway mode should be remote")
	}
	if cfg.Gateway.Remote == nil || cfg.Gateway.Remote.URL != "https://example.com:18789" {
		t.Error("remote URL should be set")
	}
}

// ---------- applyNonInteractiveGatewayConfig 测试 ----------

func TestGatewayConfig_DefaultPort(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	result, err := applyNonInteractiveGatewayConfig(NonInteractiveOptions{}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.port != 18789 {
		t.Errorf("expected default port 18789, got %d", result.port)
	}
	if result.bind != "loopback" {
		t.Errorf("expected loopback, got %s", result.bind)
	}
	if result.authMode != "token" {
		t.Errorf("expected token, got %s", result.authMode)
	}
	if result.gatewayToken == "" {
		t.Error("should generate random token")
	}
}

func TestGatewayConfig_ExplicitPort(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	result, err := applyNonInteractiveGatewayConfig(NonInteractiveOptions{
		GatewayPort: 9999,
	}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.port != 9999 {
		t.Errorf("expected 9999, got %d", result.port)
	}
}

func TestGatewayConfig_PasswordAuth(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	result, err := applyNonInteractiveGatewayConfig(NonInteractiveOptions{
		GatewayAuth:     "password",
		GatewayPassword: "secret123",
	}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.authMode != "password" {
		t.Errorf("expected password, got %s", result.authMode)
	}
}

func TestGatewayConfig_PasswordMissing(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	_, err := applyNonInteractiveGatewayConfig(NonInteractiveOptions{
		GatewayAuth: "password",
	}, cfg)
	if err == nil {
		t.Error("expected error for missing password")
	}
}

func TestGatewayConfig_TailscaleSecurityConstraints(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	result, err := applyNonInteractiveGatewayConfig(NonInteractiveOptions{
		Tailscale:       "funnel",
		GatewayBind:     "lan",
		GatewayAuth:     "token",
		GatewayPassword: "pw",
	}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Tailscale funnel → force loopback + password
	if result.bind != "loopback" {
		t.Errorf("tailscale should force loopback, got %s", result.bind)
	}
	if result.authMode != "password" {
		t.Errorf("tailscale funnel should force password, got %s", result.authMode)
	}
}

func TestGatewayConfig_InvalidAuth(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	_, err := applyNonInteractiveGatewayConfig(NonInteractiveOptions{
		GatewayAuth: "invalid",
	}, cfg)
	if err == nil {
		t.Error("expected error for invalid auth mode")
	}
}

// ---------- randomHexToken 测试 ----------

func TestRandomHexToken(t *testing.T) {
	tok := randomHexToken()
	if len(tok) != 48 { // 24 bytes = 48 hex chars
		t.Errorf("expected 48 char hex, got %d chars", len(tok))
	}
	tok2 := randomHexToken()
	if tok == tok2 {
		t.Error("tokens should be unique")
	}
}

// ---------- ApplyNonInteractiveSkillsConfig 测试 ----------

func TestSkillsConfig_Default(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	result := ApplyNonInteractiveSkillsConfig(cfg, NonInteractiveOptions{})
	if result.Skills == nil || result.Skills.Install == nil {
		t.Fatal("skills install config should be set")
	}
	if result.Skills.Install.NodeManager != "npm" {
		t.Errorf("expected npm, got %s", result.Skills.Install.NodeManager)
	}
}

func TestSkillsConfig_Pnpm(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	result := ApplyNonInteractiveSkillsConfig(cfg, NonInteractiveOptions{
		NodeManager: "pnpm",
	})
	if result.Skills.Install.NodeManager != "pnpm" {
		t.Errorf("expected pnpm, got %s", result.Skills.Install.NodeManager)
	}
}

func TestSkillsConfig_InvalidFallback(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	result := ApplyNonInteractiveSkillsConfig(cfg, NonInteractiveOptions{
		NodeManager: "yarn",
	})
	if result.Skills.Install.NodeManager != "npm" {
		t.Errorf("invalid manager should fallback to npm, got %s", result.Skills.Install.NodeManager)
	}
}

func TestSkillsConfig_SkipSkills(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	result := ApplyNonInteractiveSkillsConfig(cfg, NonInteractiveOptions{
		SkipSkills: true,
	})
	if result.Skills != nil {
		t.Error("skills should be nil when skipped")
	}
}

// ---------- InstallGatewayDaemonNonInteractive 测试 ----------

func TestDaemonInstall_NotRequested(t *testing.T) {
	// Should not panic
	InstallGatewayDaemonNonInteractive(NonInteractiveOptions{
		InstallDaemon: false,
	}, &nonInteractiveGatewayResult{port: 18789})
}

func TestDaemonInstall_Requested(t *testing.T) {
	// Should not panic
	InstallGatewayDaemonNonInteractive(NonInteractiveOptions{
		InstallDaemon: true,
	}, &nonInteractiveGatewayResult{port: 18789, bind: "loopback", authMode: "token"})
}

// ---------- LogNonInteractiveOnboardingJson 测试 ----------

func TestJsonOutput_Disabled(t *testing.T) {
	// Should not panic or produce output
	LogNonInteractiveOnboardingJson(NonInteractiveOptions{JSON: false}, NonInteractiveOnboardingOutput{
		Mode:      "local",
		Workspace: "agents",
	})
}
