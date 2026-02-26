package config

import (
	"testing"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// ============================================================
// normalizeSenderKey 测试
// ============================================================

func TestNormalizeSenderKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"alice", "alice"},
		{"@Alice", "alice"},
		{"  @Bob  ", "bob"},
		{"", ""},
		{"  ", ""},
		{"+1234567890", "+1234567890"},
	}
	for _, tt := range tests {
		got := normalizeSenderKey(tt.input)
		if got != tt.want {
			t.Errorf("normalizeSenderKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ============================================================
// ResolveToolsBySender 测试
// ============================================================

func TestResolveToolsBySender_MatchesByID(t *testing.T) {
	policy := &types.GroupToolPolicyConfig{
		Allow: []string{"web_search"},
	}
	bySender := types.GroupToolPolicyBySenderConfig{
		"user123": policy,
	}
	result := ResolveToolsBySender(bySender, GroupToolPolicySender{
		SenderID: "user123",
	})
	if result == nil {
		t.Fatal("should match by sender ID")
	}
	if len(result.Allow) != 1 || result.Allow[0] != "web_search" {
		t.Error("should return matching policy")
	}
}

func TestResolveToolsBySender_MatchesByUsername(t *testing.T) {
	policy := &types.GroupToolPolicyConfig{
		Allow: []string{"exec"},
	}
	bySender := types.GroupToolPolicyBySenderConfig{
		"@alice": policy,
	}
	result := ResolveToolsBySender(bySender, GroupToolPolicySender{
		SenderUsername: "Alice",
	})
	if result == nil {
		t.Fatal("should match by normalized username")
	}
}

func TestResolveToolsBySender_Wildcard(t *testing.T) {
	policy := &types.GroupToolPolicyConfig{
		Deny: []string{"exec"},
	}
	bySender := types.GroupToolPolicyBySenderConfig{
		"*": policy,
	}
	result := ResolveToolsBySender(bySender, GroupToolPolicySender{
		SenderID: "unknown-user",
	})
	if result == nil {
		t.Fatal("should fall back to wildcard")
	}
	if len(result.Deny) != 1 || result.Deny[0] != "exec" {
		t.Error("should return wildcard policy")
	}
}

func TestResolveToolsBySender_NoMatch(t *testing.T) {
	bySender := types.GroupToolPolicyBySenderConfig{
		"specific-user": {Allow: []string{"exec"}},
	}
	result := ResolveToolsBySender(bySender, GroupToolPolicySender{
		SenderID: "other-user",
	})
	if result != nil {
		t.Error("should return nil when no match and no wildcard")
	}
}

func TestResolveToolsBySender_Empty(t *testing.T) {
	result := ResolveToolsBySender(nil, GroupToolPolicySender{})
	if result != nil {
		t.Error("should return nil for nil input")
	}
}

// ============================================================
// ResolveChannelGroupPolicy 测试 (当前 stub 实现)
// ============================================================

func TestResolveChannelGroupPolicy_NoGroups(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	policy := ResolveChannelGroupPolicy(cfg, "telegram", "group123", "")

	if policy.AllowlistEnabled {
		t.Error("should not have allowlist with no groups")
	}
	if !policy.Allowed {
		t.Error("should be allowed when no allowlist")
	}
}

// ============================================================
// ResolveChannelGroupRequireMention 测试
// ============================================================

func TestResolveChannelGroupRequireMention_DefaultTrue(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	result := ResolveChannelGroupRequireMention(cfg, "telegram", "", "", nil, "")
	if !result {
		t.Error("default should require mention")
	}
}

func TestResolveChannelGroupRequireMention_OverrideBeforeConfig(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	override := false
	result := ResolveChannelGroupRequireMention(cfg, "telegram", "", "", &override, "before-config")
	if result {
		t.Error("before-config override should be used")
	}
}

func TestResolveChannelGroupRequireMention_OverrideAfterConfig(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	override := false
	result := ResolveChannelGroupRequireMention(cfg, "telegram", "", "", &override, "after-config")
	if result {
		t.Error("after-config override should be used when no config value")
	}
}

// ============================================================
// ResolveChannelGroupToolsPolicy 测试
// ============================================================

func TestResolveChannelGroupToolsPolicy_NoGroups(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	result := ResolveChannelGroupToolsPolicy(cfg, "telegram", "", "", GroupToolPolicySender{})
	if result != nil {
		t.Error("should return nil with no groups configured")
	}
}
