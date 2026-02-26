package tui

import (
	"os"
	"testing"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// ---------- Mock ConfigSource ----------

// nilConfigSource 返回 nil 配置（模拟无配置文件）。
type nilConfigSource struct{}

func (n nilConfigSource) LoadConfig() (*types.OpenAcosmiConfig, error) { return nil, nil }

// mockConfigSource 返回预设配置。
type mockConfigSource struct {
	cfg *types.OpenAcosmiConfig
}

func (m mockConfigSource) LoadConfig() (*types.OpenAcosmiConfig, error) { return m.cfg, nil }

// ---------- 基础测试 ----------

func TestEnvTrimmed(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		want  string
	}{
		{"normal", "TEST_TUI_ENV_1", "hello", "hello"},
		{"with spaces", "TEST_TUI_ENV_2", "  hello  ", "hello"},
		{"empty", "TEST_TUI_ENV_3", "", ""},
		{"whitespace only", "TEST_TUI_ENV_4", "   ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(tt.key, tt.value)
			defer os.Unsetenv(tt.key)

			got := envTrimmed(tt.key)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEnvTrimmedUnset(t *testing.T) {
	os.Unsetenv("TEST_TUI_NONEXISTENT")
	got := envTrimmed("TEST_TUI_NONEXISTENT")
	if got != "" {
		t.Errorf("unset env: got %q, want empty", got)
	}
}

// ---------- CLI 优先级测试 ----------

func TestResolveGatewayConnectionCLIPriority(t *testing.T) {
	info := resolveGatewayConnection(GatewayConnectionOptions{
		URL:      "ws://cli-host:9999",
		Token:    "cli-token",
		Password: "cli-password",
	}, nilConfigSource{})

	if info.URL != "ws://cli-host:9999" {
		t.Errorf("URL: got %q, want %q", info.URL, "ws://cli-host:9999")
	}
	if info.Token != "cli-token" {
		t.Errorf("Token: got %q, want %q", info.Token, "cli-token")
	}
	if info.Password != "cli-password" {
		t.Errorf("Password: got %q, want %q", info.Password, "cli-password")
	}
}

// ---------- 环境变量 Fallback 测试 ----------

func TestResolveGatewayConnectionEnvFallback(t *testing.T) {
	os.Setenv("OPENACOSMI_GATEWAY_TOKEN", "env-token-1")
	os.Setenv("OPENACOSMI_GATEWAY_PASSWORD", "env-password-1")
	defer os.Unsetenv("OPENACOSMI_GATEWAY_TOKEN")
	defer os.Unsetenv("OPENACOSMI_GATEWAY_PASSWORD")

	info := resolveGatewayConnection(GatewayConnectionOptions{}, nilConfigSource{})

	if info.Token != "env-token-1" {
		t.Errorf("Token: got %q, want %q", info.Token, "env-token-1")
	}
	if info.Password != "env-password-1" {
		t.Errorf("Password: got %q, want %q", info.Password, "env-password-1")
	}
}

func TestResolveGatewayConnectionClawdbotEnvFallback(t *testing.T) {
	os.Unsetenv("OPENACOSMI_GATEWAY_TOKEN")
	os.Unsetenv("OPENACOSMI_GATEWAY_PASSWORD")
	os.Setenv("CLAWDBOT_GATEWAY_TOKEN", "clawdbot-token")
	os.Setenv("CLAWDBOT_GATEWAY_PASSWORD", "clawdbot-password")
	defer os.Unsetenv("CLAWDBOT_GATEWAY_TOKEN")
	defer os.Unsetenv("CLAWDBOT_GATEWAY_PASSWORD")

	info := resolveGatewayConnection(GatewayConnectionOptions{}, nilConfigSource{})

	if info.Token != "clawdbot-token" {
		t.Errorf("Token: got %q, want %q", info.Token, "clawdbot-token")
	}
	if info.Password != "clawdbot-password" {
		t.Errorf("Password: got %q, want %q", info.Password, "clawdbot-password")
	}
}

func TestResolveGatewayConnectionCLIOverridesEnv(t *testing.T) {
	os.Setenv("OPENACOSMI_GATEWAY_TOKEN", "env-token")
	os.Setenv("OPENACOSMI_GATEWAY_PASSWORD", "env-password")
	defer os.Unsetenv("OPENACOSMI_GATEWAY_TOKEN")
	defer os.Unsetenv("OPENACOSMI_GATEWAY_PASSWORD")

	info := resolveGatewayConnection(GatewayConnectionOptions{
		Token:    "cli-token",
		Password: "cli-password",
	}, nilConfigSource{})

	if info.Token != "cli-token" {
		t.Errorf("Token: got %q, want cli-token", info.Token)
	}
	if info.Password != "cli-password" {
		t.Errorf("Password: got %q, want cli-password", info.Password)
	}
}

func TestResolveGatewayConnectionURLOverride(t *testing.T) {
	os.Setenv("OPENACOSMI_GATEWAY_TOKEN", "env-token")
	os.Setenv("OPENACOSMI_GATEWAY_PASSWORD", "env-password")
	defer os.Unsetenv("OPENACOSMI_GATEWAY_TOKEN")
	defer os.Unsetenv("OPENACOSMI_GATEWAY_PASSWORD")

	info := resolveGatewayConnection(GatewayConnectionOptions{
		URL: "ws://custom-host:8080",
	}, nilConfigSource{})

	if info.URL != "ws://custom-host:8080" {
		t.Errorf("URL: got %q", info.URL)
	}
	if info.Token != "" {
		t.Errorf("Token should be empty with URL override, got %q", info.Token)
	}
	if info.Password != "" {
		t.Errorf("Password should be empty with URL override, got %q", info.Password)
	}
}

func TestResolveGatewayConnectionDefaultURL(t *testing.T) {
	os.Unsetenv("OPENACOSMI_GATEWAY_TOKEN")
	os.Unsetenv("OPENACOSMI_GATEWAY_PASSWORD")
	os.Unsetenv("CLAWDBOT_GATEWAY_TOKEN")
	os.Unsetenv("CLAWDBOT_GATEWAY_PASSWORD")

	info := resolveGatewayConnection(GatewayConnectionOptions{}, nilConfigSource{})

	if info.URL == "" {
		t.Error("URL should not be empty")
	}
	if len(info.URL) < 15 {
		t.Errorf("URL too short: %q", info.URL)
	}
}

func TestResolveGatewayConnectionTrimming(t *testing.T) {
	info := resolveGatewayConnection(GatewayConnectionOptions{
		URL:      "  ws://host:1234  ",
		Token:    "  token  ",
		Password: "  pass  ",
	}, nilConfigSource{})

	if info.URL != "ws://host:1234" {
		t.Errorf("URL not trimmed: got %q", info.URL)
	}
	if info.Token != "token" {
		t.Errorf("Token not trimmed: got %q", info.Token)
	}
	if info.Password != "pass" {
		t.Errorf("Password not trimmed: got %q", info.Password)
	}
}

// ---------- TUI-D1: Config 文件层 mock 测试 ----------

func TestResolveGatewayConnectionConfigToken(t *testing.T) {
	// 无 CLI / 无环境变量 → config 文件层 token
	os.Unsetenv("OPENACOSMI_GATEWAY_TOKEN")
	os.Unsetenv("OPENACOSMI_GATEWAY_PASSWORD")
	os.Unsetenv("CLAWDBOT_GATEWAY_TOKEN")
	os.Unsetenv("CLAWDBOT_GATEWAY_PASSWORD")

	cfg := &types.OpenAcosmiConfig{
		Gateway: &types.GatewayConfig{
			Auth: &types.GatewayAuthConfig{
				Token:    "config-token-123",
				Password: "config-password-456",
			},
		},
	}

	info := resolveGatewayConnection(GatewayConnectionOptions{}, mockConfigSource{cfg: cfg})

	if info.Token != "config-token-123" {
		t.Errorf("Token: got %q, want config-token-123", info.Token)
	}
	if info.Password != "config-password-456" {
		t.Errorf("Password: got %q, want config-password-456", info.Password)
	}
}

func TestResolveGatewayConnectionRemoteMode(t *testing.T) {
	os.Unsetenv("OPENACOSMI_GATEWAY_TOKEN")
	os.Unsetenv("OPENACOSMI_GATEWAY_PASSWORD")
	os.Unsetenv("CLAWDBOT_GATEWAY_TOKEN")
	os.Unsetenv("CLAWDBOT_GATEWAY_PASSWORD")

	cfg := &types.OpenAcosmiConfig{
		Gateway: &types.GatewayConfig{
			Mode: "remote",
			Remote: &types.GatewayRemoteConfig{
				URL:      "ws://remote-host:8888",
				Token:    "remote-token",
				Password: "remote-pass",
			},
		},
	}

	info := resolveGatewayConnection(GatewayConnectionOptions{}, mockConfigSource{cfg: cfg})

	if info.URL != "ws://remote-host:8888" {
		t.Errorf("URL: got %q, want ws://remote-host:8888", info.URL)
	}
	if info.Token != "remote-token" {
		t.Errorf("Token: got %q, want remote-token", info.Token)
	}
	// remote password 优先级低于 env password，但高于 config auth password
	if info.Password != "remote-pass" {
		t.Errorf("Password: got %q, want remote-pass", info.Password)
	}
}

func TestResolveGatewayConnectionConfigPort(t *testing.T) {
	os.Unsetenv("OPENACOSMI_GATEWAY_TOKEN")
	os.Unsetenv("OPENACOSMI_GATEWAY_PASSWORD")
	os.Unsetenv("CLAWDBOT_GATEWAY_TOKEN")
	os.Unsetenv("CLAWDBOT_GATEWAY_PASSWORD")

	port := 12345
	cfg := &types.OpenAcosmiConfig{
		Gateway: &types.GatewayConfig{
			Port: &port,
		},
	}

	info := resolveGatewayConnection(GatewayConnectionOptions{}, mockConfigSource{cfg: cfg})

	expected := "ws://127.0.0.1:12345"
	if info.URL != expected {
		t.Errorf("URL: got %q, want %q", info.URL, expected)
	}
}

func TestResolveGatewayConnectionEnvOverridesConfig(t *testing.T) {
	// env 优先于 config
	os.Setenv("OPENACOSMI_GATEWAY_TOKEN", "env-over-config")
	defer os.Unsetenv("OPENACOSMI_GATEWAY_TOKEN")
	os.Unsetenv("OPENACOSMI_GATEWAY_PASSWORD")
	os.Unsetenv("CLAWDBOT_GATEWAY_TOKEN")
	os.Unsetenv("CLAWDBOT_GATEWAY_PASSWORD")

	cfg := &types.OpenAcosmiConfig{
		Gateway: &types.GatewayConfig{
			Auth: &types.GatewayAuthConfig{
				Token: "config-token-should-lose",
			},
		},
	}

	info := resolveGatewayConnection(GatewayConnectionOptions{}, mockConfigSource{cfg: cfg})

	if info.Token != "env-over-config" {
		t.Errorf("Token: got %q, want env-over-config", info.Token)
	}
}

// ---------- TUI-2: ensureExplicitGatewayAuth 测试 ----------

func TestEnsureExplicitGatewayAuthNoURL(t *testing.T) {
	// 无 URL 覆盖 → 不校验
	err := ensureExplicitGatewayAuth("", "", "")
	if err != nil {
		t.Errorf("no URL: expected nil error, got %v", err)
	}
}

func TestEnsureExplicitGatewayAuthURLWithToken(t *testing.T) {
	err := ensureExplicitGatewayAuth("ws://remote:9999", "my-token", "")
	if err != nil {
		t.Errorf("URL + token: expected nil error, got %v", err)
	}
}

func TestEnsureExplicitGatewayAuthURLWithPassword(t *testing.T) {
	err := ensureExplicitGatewayAuth("ws://remote:9999", "", "my-pass")
	if err != nil {
		t.Errorf("URL + password: expected nil error, got %v", err)
	}
}

func TestEnsureExplicitGatewayAuthURLWithBoth(t *testing.T) {
	err := ensureExplicitGatewayAuth("ws://remote:9999", "tok", "pass")
	if err != nil {
		t.Errorf("URL + both: expected nil error, got %v", err)
	}
}

func TestEnsureExplicitGatewayAuthURLWithoutCreds(t *testing.T) {
	err := ensureExplicitGatewayAuth("ws://remote:9999", "", "")
	if err == nil {
		t.Error("URL without creds: expected error, got nil")
	}
}

func TestEnsureExplicitGatewayAuthEmptyURL(t *testing.T) {
	err := ensureExplicitGatewayAuth("", "tok", "pass")
	if err != nil {
		t.Errorf("empty URL: expected nil error, got %v", err)
	}
}
