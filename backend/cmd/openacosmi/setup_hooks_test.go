package main

import (
	"testing"

	"github.com/openacosmi/claw-acismi/internal/tui"
	"github.com/openacosmi/claw-acismi/pkg/types"
)

func TestSetupInternalHooks_EmptyWorkspace(t *testing.T) {
	// 使用空 workspace（无 hooks 目录），应返回原始 config
	cfg := &types.OpenAcosmiConfig{}
	tempDir := t.TempDir()

	// 创建一个 mock prompter 记录 Note 调用
	p := &mockHooksPrompter{}

	result, err := SetupInternalHooks(cfg, tempDir, p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 空 workspace → no eligible hooks → 返回原始 config
	if result != cfg {
		t.Error("expected same config pointer when no hooks found")
	}

	// 应该至少调用了 2 次 Note（说明 + 无 hooks 提示）
	if p.noteCalls < 2 {
		t.Errorf("expected at least 2 Note calls, got %d", p.noteCalls)
	}
}

func TestOnboardOptionsComplete(t *testing.T) {
	// 验证 OnboardOptions 包含所有预期字段
	opts := OnboardOptions{
		Mode:           OnboardModeLocal,
		Flow:           "quickstart",
		Workspace:      "/tmp/workspace",
		NonInteractive: true,
		AcceptRisk:     true,
		Reset:          false,
		AuthChoice:     AuthChoiceGeminiApiKey,

		TokenProvider:  "anthropic",
		Token:          "sk-xxx",
		TokenProfileID: "prof-1",
		TokenExpiresIn: "1h",

		AnthropicApiKey: "sk-ant-xxx",
		OpenAIApiKey:    "sk-xxx",
		GeminiApiKey:    "AIza-xxx",

		GatewayPort:     19001,
		GatewayBind:     GatewayBindLoopback,
		GatewayAuth:     GatewayAuthChoiceToken,
		GatewayToken:    "tok-xxx",
		GatewayPassword: "",

		Tailscale:            TailscaleModeOff,
		TailscaleResetOnExit: false,

		InstallDaemon: false,
		SkipChannels:  true,
		SkipSkills:    false,
		SkipHealth:    false,
		SkipUI:        false,
		NodeManager:   NodeManagerNpm,
		JSON:          false,

		RemoteURL:   "ws://127.0.0.1:19001",
		RemoteToken: "",
	}

	// 基本验证
	if opts.Mode != "local" {
		t.Errorf("Mode = %q, want local", opts.Mode)
	}
	if opts.AuthChoice != "gemini-api-key" {
		t.Errorf("AuthChoice = %q, want gemini-api-key", opts.AuthChoice)
	}
	if opts.GatewayBind != "loopback" {
		t.Errorf("GatewayBind = %q, want loopback", opts.GatewayBind)
	}
	if opts.NodeManager != "npm" {
		t.Errorf("NodeManager = %q, want npm", opts.NodeManager)
	}
}

// ---------- Mock Prompter ----------

type mockHooksPrompter struct {
	noteCalls int
}

func (m *mockHooksPrompter) Intro(title string)         {}
func (m *mockHooksPrompter) Outro(message string)       {}
func (m *mockHooksPrompter) Note(message, title string) { m.noteCalls++ }
func (m *mockHooksPrompter) Select(message string, options []tui.PromptOption, initial string) (string, error) {
	return "", nil
}
func (m *mockHooksPrompter) MultiSelect(message string, options []tui.PromptOption, initial []string) ([]string, error) {
	return nil, nil
}
func (m *mockHooksPrompter) TextInput(message, placeholder, initial string, validate func(string) string) (string, error) {
	return "", nil
}
func (m *mockHooksPrompter) Confirm(message string, initial bool) (bool, error) {
	return false, nil
}
