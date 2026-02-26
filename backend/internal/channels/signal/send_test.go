package signal

// send 测试 — 对齐 src/signal/send.ts 相关逻辑

import (
	"testing"

	"github.com/anthropic/open-acosmi/pkg/types"
)

func TestParseTarget_Recipient(t *testing.T) {
	tests := []struct {
		input    string
		wantKind SignalTargetKind
		wantVal  string
	}{
		{"+15550001111", TargetRecipient, "+15550001111"},
		{"signal:+15550001111", TargetRecipient, "+15550001111"},
		{"123e4567-e89b-12d3-a456-426614174000", TargetRecipient, "123e4567-e89b-12d3-a456-426614174000"},
	}
	for _, tt := range tests {
		target := ParseTarget(tt.input)
		if target == nil {
			t.Fatalf("ParseTarget(%q) = nil", tt.input)
		}
		if target.Kind != tt.wantKind {
			t.Errorf("ParseTarget(%q).Kind = %s, want %s", tt.input, target.Kind, tt.wantKind)
		}
		if target.Value != tt.wantVal {
			t.Errorf("ParseTarget(%q).Value = %q, want %q", tt.input, target.Value, tt.wantVal)
		}
	}
}

func TestParseTarget_Group(t *testing.T) {
	target := ParseTarget("group:my-group-id")
	if target == nil {
		t.Fatal("expected non-nil target")
	}
	if target.Kind != TargetGroup {
		t.Errorf("kind = %s, want group", target.Kind)
	}
	if target.Value != "my-group-id" {
		t.Errorf("value = %q, want %q", target.Value, "my-group-id")
	}
}

func TestParseTarget_Username(t *testing.T) {
	tests := []struct {
		input string
		value string
	}{
		{"username:alice", "alice"},
		{"u:bob", "bob"},
	}
	for _, tt := range tests {
		target := ParseTarget(tt.input)
		if target == nil {
			t.Fatalf("ParseTarget(%q) = nil", tt.input)
		}
		if target.Kind != TargetUsername {
			t.Errorf("kind = %s, want username", target.Kind)
		}
		if target.Value != tt.value {
			t.Errorf("value = %q, want %q", target.Value, tt.value)
		}
	}
}

func TestParseTarget_SignalPrefix(t *testing.T) {
	// "signal:" 前缀应被去除
	target := ParseTarget("signal:group:abc")
	if target == nil {
		t.Fatal("expected non-nil target")
	}
	if target.Kind != TargetGroup {
		t.Errorf("kind = %s, want group", target.Kind)
	}
	if target.Value != "abc" {
		t.Errorf("value = %q, want abc", target.Value)
	}
}

func TestParseTarget_Empty(t *testing.T) {
	if ParseTarget("") != nil {
		t.Error("empty should return nil")
	}
	if ParseTarget("  ") != nil {
		t.Error("whitespace should return nil")
	}
}

func TestParseTarget_EmptyGroup(t *testing.T) {
	if ParseTarget("group:") != nil {
		t.Error("empty group id should return nil")
	}
	if ParseTarget("username:") != nil {
		t.Error("empty username should return nil")
	}
}

func TestBuildTargetParams(t *testing.T) {
	// recipient 使用数组格式
	params := buildTargetParams(&SignalTarget{Kind: TargetRecipient, Value: "+15550001111"})
	recip, ok := params["recipient"].([]string)
	if !ok || len(recip) != 1 || recip[0] != "+15550001111" {
		t.Errorf("recipient params = %v", params)
	}

	// group 使用字符串
	params2 := buildTargetParams(&SignalTarget{Kind: TargetGroup, Value: "gid"})
	if params2["groupId"] != "gid" {
		t.Errorf("group params = %v", params2)
	}

	// username 使用数组格式
	params3 := buildTargetParams(&SignalTarget{Kind: TargetUsername, Value: "alice"})
	uname, ok := params3["username"].([]string)
	if !ok || len(uname) != 1 || uname[0] != "alice" {
		t.Errorf("username params = %v", params3)
	}
}

func TestFormatTextStyles(t *testing.T) {
	styles := []SignalTextStyleRange{
		{Start: 0, Length: 4, Style: StyleBold},
		{Start: 5, Length: 6, Style: StyleItalic},
	}
	result := formatTextStyles(styles)
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	if result[0] != "0:4:BOLD" {
		t.Errorf("result[0] = %q, want 0:4:BOLD", result[0])
	}
	if result[1] != "5:6:ITALIC" {
		t.Errorf("result[1] = %q, want 5:6:ITALIC", result[1])
	}
}

func TestResolveMaxBytes(t *testing.T) {
	// 对齐 TS: opts → account → global → 8MB 级联
	defaultMB := 8 * 1024 * 1024

	// opts 优先
	got := ResolveMaxBytes(&types.OpenAcosmiConfig{}, 5*1024*1024, "default")
	if got != 5*1024*1024 {
		t.Errorf("opts max = %d, want %d", got, 5*1024*1024)
	}

	// 无配置时默认 8MB
	got2 := ResolveMaxBytes(&types.OpenAcosmiConfig{}, 0, "default")
	if got2 != defaultMB {
		t.Errorf("default = %d, want %d", got2, defaultMB)
	}

	// account 级配置
	mb10 := 10
	got3 := ResolveMaxBytes(&types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Signal: &types.SignalConfig{
				SignalAccountConfig: types.SignalAccountConfig{
					MediaMaxMB: &mb10,
				},
			},
		},
	}, 0, "default")
	if got3 != 10*1024*1024 {
		t.Errorf("account max = %d, want %d", got3, 10*1024*1024)
	}
}
