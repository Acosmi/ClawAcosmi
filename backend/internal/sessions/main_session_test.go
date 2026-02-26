package sessions

import (
	"testing"
)

func TestResolveMainSessionKey(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *SessionScopeConfig
		agents []AgentListEntry
		want   string
	}{
		{"global scope", &SessionScopeConfig{Scope: "global"}, nil, "global"},
		{"no agents", nil, nil, "agent:main:main"},
		{"with default agent", nil, []AgentListEntry{{ID: "bot1", Default: true}}, "agent:bot1:main"},
		{"first agent", nil, []AgentListEntry{{ID: "bot2"}, {ID: "bot3"}}, "agent:bot2:main"},
		{"custom mainKey", &SessionScopeConfig{MainKey: "chat"}, nil, "agent:main:chat"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveMainSessionKey(tt.cfg, tt.agents)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCanonicalizeMainSessionAlias(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *SessionScopeConfig
		agentID string
		key     string
		want    string
	}{
		{"empty key", nil, "main", "", ""},
		{"main alias", nil, "main", "main", "agent:main:main"},
		{"global scope main alias", &SessionScopeConfig{Scope: "global"}, "main", "main", "global"},
		{"non-alias", nil, "main", "custom-key", "custom-key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanonicalizeMainSessionAlias(tt.cfg, tt.agentID, tt.key)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDeriveSessionKey(t *testing.T) {
	tests := []struct {
		name  string
		scope string
		ctx   MsgContextForGroup
		want  string
	}{
		{"global", "global", MsgContextForGroup{}, "global"},
		{"group", "per-sender", MsgContextForGroup{From: "telegram:group:mygrp", ChatType: "group"}, "telegram:group:mygrp"},
		{"direct with from", "per-sender", MsgContextForGroup{From: "+1234567890"}, "+1234567890"},
		{"unknown", "per-sender", MsgContextForGroup{}, "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeriveSessionKey(tt.scope, tt.ctx)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveSessionKeyFull(t *testing.T) {
	tests := []struct {
		name     string
		scope    string
		ctx      MsgContextForGroup
		explicit string
		mainKey  string
		want     string
	}{
		{"explicit override", "per-sender", MsgContextForGroup{}, "Custom-Key", "", "custom-key"},
		{"global", "global", MsgContextForGroup{}, "", "", "global"},
		{"direct defaults to canonical", "per-sender", MsgContextForGroup{From: "+1234"}, "", "", "agent:main:main"},
		{"group key", "per-sender", MsgContextForGroup{From: "telegram:group:test", ChatType: "group"}, "", "", "agent:main:telegram:group:test"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveSessionKeyFull(tt.scope, tt.ctx, tt.explicit, tt.mainKey)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
