package signal

// accounts 测试 — 对齐 src/signal/accounts.ts 相关逻辑

import (
	"testing"

	"github.com/anthropic/open-acosmi/pkg/types"
)

func TestNormalizeAccountID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "default"},
		{"  ", "default"},
		{"Default", "default"},
		{"MY_ACCT", "my_acct"},
		{"  Test  ", "test"},
	}
	for _, tt := range tests {
		got := NormalizeAccountID(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeAccountID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestListSignalAccountIds_NoCfg(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	ids := ListSignalAccountIds(cfg)
	if len(ids) != 1 || ids[0] != "default" {
		t.Errorf("got %v, want [default]", ids)
	}
}

func TestListSignalAccountIds_WithAccounts(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Signal: &types.SignalConfig{
				Accounts: map[string]*types.SignalAccountConfig{
					"alpha": {Account: "+1"},
					"beta":  {Account: "+2"},
				},
			},
		},
	}
	ids := ListSignalAccountIds(cfg)
	if len(ids) != 2 {
		t.Fatalf("got %d ids, want 2", len(ids))
	}
	// 结果已排序
	if ids[0] != "alpha" || ids[1] != "beta" {
		t.Errorf("ids = %v, want [alpha, beta]", ids)
	}
}

func TestResolveSignalAccount_DefaultBaseURL(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	acct := ResolveSignalAccount(cfg, "default")
	if acct.BaseURL != "http://127.0.0.1:8080" {
		t.Errorf("baseURL = %q, want http://127.0.0.1:8080", acct.BaseURL)
	}
	if !acct.Enabled {
		t.Error("default account should be enabled")
	}
}

func TestResolveSignalAccount_CustomPort(t *testing.T) {
	port := 9090
	cfg := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Signal: &types.SignalConfig{
				SignalAccountConfig: types.SignalAccountConfig{
					HttpPort: &port,
				},
			},
		},
	}
	acct := ResolveSignalAccount(cfg, "default")
	if acct.BaseURL != "http://127.0.0.1:9090" {
		t.Errorf("baseURL = %q, want http://127.0.0.1:9090", acct.BaseURL)
	}
}

func TestResolveSignalAccount_Disabled(t *testing.T) {
	disabled := false
	cfg := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Signal: &types.SignalConfig{
				SignalAccountConfig: types.SignalAccountConfig{
					Enabled: &disabled,
				},
			},
		},
	}
	acct := ResolveSignalAccount(cfg, "default")
	if acct.Enabled {
		t.Error("account should be disabled when root enabled=false")
	}
}

func TestResolveSignalAccount_AccountOverride(t *testing.T) {
	port := 9999
	cfg := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Signal: &types.SignalConfig{
				SignalAccountConfig: types.SignalAccountConfig{
					Account: "+1000",
				},
				Accounts: map[string]*types.SignalAccountConfig{
					"work": {
						Account:  "+2000",
						HttpPort: &port,
					},
				},
			},
		},
	}
	acct := ResolveSignalAccount(cfg, "work")
	if acct.Config.Account != "+2000" {
		t.Errorf("account = %q, want +2000", acct.Config.Account)
	}
	if acct.BaseURL != "http://127.0.0.1:9999" {
		t.Errorf("baseURL = %q, want http://127.0.0.1:9999", acct.BaseURL)
	}
}

func TestResolveSignalAccount_HttpURLOverride(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Signal: &types.SignalConfig{
				SignalAccountConfig: types.SignalAccountConfig{
					HttpURL: "http://custom:5000",
				},
			},
		},
	}
	acct := ResolveSignalAccount(cfg, "default")
	if acct.BaseURL != "http://custom:5000" {
		t.Errorf("baseURL = %q, want http://custom:5000", acct.BaseURL)
	}
}

func TestResolveSignalAccount_Configured(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Signal: &types.SignalConfig{
				SignalAccountConfig: types.SignalAccountConfig{
					Account: "+15550001111",
				},
			},
		},
	}
	acct := ResolveSignalAccount(cfg, "default")
	if !acct.Configured {
		t.Error("account with Account set should be configured")
	}

	empty := &types.OpenAcosmiConfig{}
	acct2 := ResolveSignalAccount(empty, "default")
	if acct2.Configured {
		t.Error("empty config should not be configured")
	}
}

func TestResolveDefaultSignalAccountId(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	id := ResolveDefaultSignalAccountId(cfg)
	if id != "default" {
		t.Errorf("got %q, want default", id)
	}
}
