package main

import (
	"testing"

	"github.com/Acosmi/ClawAcosmi/internal/infra"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

func TestPickHost(t *testing.T) {
	tests := []struct {
		name     string
		beacon   *infra.GatewayBonjourBeacon
		expected string
	}{
		{
			"tailnet preferred",
			&infra.GatewayBonjourBeacon{TailnetDNS: "my.ts.net", LanHost: "192.168.1.1", Host: "host.local"},
			"my.ts.net",
		},
		{
			"lan fallback",
			&infra.GatewayBonjourBeacon{LanHost: "192.168.1.1", Host: "host.local"},
			"192.168.1.1",
		},
		{
			"host fallback",
			&infra.GatewayBonjourBeacon{Host: "host.local"},
			"host.local",
		},
		{
			"all empty",
			&infra.GatewayBonjourBeacon{},
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pickHost(tt.beacon)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBuildBeaconLabel(t *testing.T) {
	tests := []struct {
		name     string
		beacon   *infra.GatewayBonjourBeacon
		expected string
	}{
		{
			"full info",
			&infra.GatewayBonjourBeacon{
				DisplayName: "My Gateway",
				Host:        "192.168.1.1",
				GatewayPort: 19001,
			},
			"My Gateway (192.168.1.1:19001)",
		},
		{
			"instance name fallback",
			&infra.GatewayBonjourBeacon{
				InstanceName: "openacosmi-gw",
				Host:         "10.0.0.1",
				Port:         18790,
			},
			"openacosmi-gw (10.0.0.1:18790)",
		},
		{
			"no host",
			&infra.GatewayBonjourBeacon{
				DisplayName: "Unknown GW",
			},
			"Unknown GW (host unknown)",
		},
		{
			"default port",
			&infra.GatewayBonjourBeacon{
				DisplayName: "GW",
				Host:        "localhost",
			},
			"GW (localhost:19001)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildBeaconLabel(tt.beacon)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestEnsureWsUrl(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", defaultGatewayURL},
		{"  ", defaultGatewayURL},
		{"ws://example.com:19001", "ws://example.com:19001"},
		{"wss://secure.example.com", "wss://secure.example.com"},
	}

	for _, tt := range tests {
		result := ensureWsUrl(tt.input)
		if result != tt.expected {
			t.Errorf("ensureWsUrl(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestPromptRemoteGatewayConfig_DefaultURL(t *testing.T) {
	// 测试默认 URL
	cfg := &types.OpenAcosmiConfig{}
	if cfg.Gateway == nil || cfg.Gateway.Remote == nil {
		// 期望 suggestedURL 为 defaultGatewayURL
		expected := "ws://127.0.0.1:19001"
		if defaultGatewayURL != expected {
			t.Errorf("defaultGatewayURL = %q, want %q", defaultGatewayURL, expected)
		}
	}
}

func TestPromptRemoteGatewayConfig_ExistingURL(t *testing.T) {
	// 测试已有远程配置时使用 existing URL
	cfg := &types.OpenAcosmiConfig{
		Gateway: &types.GatewayConfig{
			Remote: &types.GatewayRemoteConfig{
				URL: "wss://my-server.com:19001",
			},
		},
	}

	// 验证读取现有 URL
	if cfg.Gateway.Remote.URL != "wss://my-server.com:19001" {
		t.Errorf("unexpected URL: %s", cfg.Gateway.Remote.URL)
	}
}
